package dap

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	godap "github.com/google/go-dap"
)

// makeInitializeResponse creates an InitializeResponse with the given seq and
// requestSeq values, ready for writing to a pipe.
func makeInitializeResponse(seq, requestSeq int) *godap.InitializeResponse {
	resp := &godap.InitializeResponse{}
	resp.Seq = seq
	resp.Type = "response"
	resp.Command = "initialize"
	resp.RequestSeq = requestSeq
	resp.Success = true
	return resp
}

func TestSendReceive(t *testing.T) {
	// Create two pipe pairs: one for client writes (requests), one for client reads (responses).
	clientReadPR, clientReadPW := io.Pipe()
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)
	go client.ReadLoop()

	// In a goroutine, read the request from the other end of the client's
	// write pipe, then write a matching response to the client's read pipe.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		msg, err := godap.ReadProtocolMessage(reader)
		if err != nil {
			t.Errorf("failed to read request from pipe: %v", err)
			return
		}

		req, ok := msg.(godap.RequestMessage)
		if !ok {
			t.Errorf("expected RequestMessage, got %T", msg)
			return
		}

		resp := makeInitializeResponse(1, req.GetRequest().Seq)
		if err := godap.WriteProtocolMessage(clientReadPW, resp); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	initReq := &godap.InitializeRequest{}
	initReq.Type = "request"
	initReq.Command = "initialize"
	initReq.Arguments = godap.InitializeRequestArguments{
		ClientID:  "test",
		AdapterID: "lldb-dap",
	}

	result, err := client.Send(ctx, initReq)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	resp, ok := result.(*godap.InitializeResponse)
	if !ok {
		t.Fatalf("expected *InitializeResponse, got %T", result)
	}
	if !resp.Success {
		t.Error("expected Success to be true")
	}
	if resp.Command != "initialize" {
		t.Errorf("Command: got %q, want %q", resp.Command, "initialize")
	}
}

func TestSendMultipleConcurrent(t *testing.T) {
	clientReadPR, clientReadPW := io.Pipe()
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)
	go client.ReadLoop()

	const numRequests = 3

	// Goroutine that reads all requests, collects their sequence numbers,
	// then responds in reverse order.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		seqs := make([]int, 0, numRequests)

		for i := 0; i < numRequests; i++ {
			msg, err := godap.ReadProtocolMessage(reader)
			if err != nil {
				t.Errorf("failed to read request %d: %v", i, err)
				return
			}
			req, ok := msg.(godap.RequestMessage)
			if !ok {
				t.Errorf("expected RequestMessage, got %T", msg)
				return
			}
			seqs = append(seqs, req.GetRequest().Seq)
		}

		// Respond in reverse order.
		for i := len(seqs) - 1; i >= 0; i-- {
			resp := makeInitializeResponse(100+i, seqs[i])
			if err := godap.WriteProtocolMessage(clientReadPW, resp); err != nil {
				t.Errorf("failed to write response for seq %d: %v", seqs[i], err)
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]godap.Message, numRequests)
	errs := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := &godap.InitializeRequest{}
			req.Type = "request"
			req.Command = "initialize"
			req.Arguments = godap.InitializeRequestArguments{
				ClientID:  "test",
				AdapterID: "lldb-dap",
			}
			results[idx], errs[idx] = client.Send(ctx, req)
		}(i)
	}

	wg.Wait()

	for i := 0; i < numRequests; i++ {
		if errs[i] != nil {
			t.Errorf("Send[%d] returned error: %v", i, errs[i])
			continue
		}
		resp, ok := results[i].(*godap.InitializeResponse)
		if !ok {
			t.Errorf("Send[%d]: expected *InitializeResponse, got %T", i, results[i])
			continue
		}
		if !resp.Success {
			t.Errorf("Send[%d]: expected Success=true", i)
		}
	}
}

func TestSendAsync(t *testing.T) {
	clientReadPR, clientReadPW := io.Pipe()
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)
	go client.ReadLoop()

	// Read the request and respond.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		msg, err := godap.ReadProtocolMessage(reader)
		if err != nil {
			t.Errorf("failed to read request: %v", err)
			return
		}
		req, ok := msg.(godap.RequestMessage)
		if !ok {
			t.Errorf("expected RequestMessage, got %T", msg)
			return
		}
		resp := makeInitializeResponse(1, req.GetRequest().Seq)
		if err := godap.WriteProtocolMessage(clientReadPW, resp); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &godap.InitializeRequest{}
	req.Type = "request"
	req.Command = "initialize"
	req.Arguments = godap.InitializeRequestArguments{
		ClientID:  "test",
		AdapterID: "lldb-dap",
	}

	ch, err := client.SendAsync(ctx, req)
	if err != nil {
		t.Fatalf("SendAsync returned error: %v", err)
	}

	select {
	case result := <-ch:
		if result.err != nil {
			t.Fatalf("result.err: %v", result.err)
		}
		resp, ok := result.msg.(*godap.InitializeResponse)
		if !ok {
			t.Fatalf("expected *InitializeResponse, got %T", result.msg)
		}
		if !resp.Success {
			t.Error("expected Success=true")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for async response")
	}
}

func TestSendContextCancellation(t *testing.T) {
	clientReadPR, clientReadPW := io.Pipe()
	// Use a writer that we never read from, so the write blocks are handled,
	// but we use a discard-style approach: we read and discard.
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)
	go client.ReadLoop()

	// Drain the write side so Send doesn't block on write.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		for {
			_, err := godap.ReadProtocolMessage(reader)
			if err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	req := &godap.InitializeRequest{}
	req.Type = "request"
	req.Command = "initialize"
	req.Arguments = godap.InitializeRequestArguments{
		ClientID:  "test",
		AdapterID: "lldb-dap",
	}

	// Use SendAsync so we can cancel between sending and receiving.
	ch, err := client.SendAsync(ctx, req)
	if err != nil {
		t.Fatalf("SendAsync returned error: %v", err)
	}

	// Cancel the context before any response arrives.
	cancel()

	// Now use a select simulating what Send would do.
	select {
	case result := <-ch:
		// The response channel might also receive if cancelAllPending
		// ran, but in this case we never close the reader, so the read
		// loop is still running. The channel should not receive.
		t.Fatalf("unexpected result on channel: %+v", result)
	case <-ctx.Done():
		// Expected: context was cancelled.
	}

	if ctx.Err() != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", ctx.Err())
	}

	// Verify the pending entry is cleaned up. We access it through Send
	// which cleans up on cancellation.
	seq := req.GetRequest().Seq
	client.pendingMu.Lock()
	_, found := client.pending[seq]
	client.pendingMu.Unlock()

	// The pending entry should still exist since we used SendAsync
	// (Send does the cleanup, not SendAsync). Clean it up manually.
	if found {
		client.pendingMu.Lock()
		delete(client.pending, seq)
		client.pendingMu.Unlock()
	}
}

func TestSendContextCancellationViaSend(t *testing.T) {
	clientReadPR, clientReadPW := io.Pipe()
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)
	go client.ReadLoop()

	// Drain the write side so Send doesn't block on write.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		for {
			_, err := godap.ReadProtocolMessage(reader)
			if err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	req := &godap.InitializeRequest{}
	req.Type = "request"
	req.Command = "initialize"
	req.Arguments = godap.InitializeRequestArguments{
		ClientID:  "test",
		AdapterID: "lldb-dap",
	}

	// Cancel immediately so Send returns quickly.
	cancel()

	result, err := client.Send(ctx, req)
	if err == nil {
		t.Fatalf("expected error from Send, got result: %v", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Verify the pending entry was cleaned up.
	seq := req.GetRequest().Seq
	client.pendingMu.Lock()
	_, found := client.pending[seq]
	client.pendingMu.Unlock()
	if found {
		t.Error("expected pending entry to be cleaned up after context cancellation")
	}
}

func TestCancelAllPending(t *testing.T) {
	// We don't need real pipes here; just test the pending map management.
	// Create a client with a dummy reader/writer that we won't use.
	clientReadPR, clientReadPW := io.Pipe()
	clientWritePR, clientWritePW := io.Pipe()
	t.Cleanup(func() {
		clientReadPR.Close()
		clientReadPW.Close()
		clientWritePR.Close()
		clientWritePW.Close()
	})

	client := NewClient(bufio.NewReader(clientReadPR), clientWritePW)

	// Drain the write side so SendAsync doesn't block.
	go func() {
		reader := bufio.NewReader(clientWritePR)
		for {
			_, err := godap.ReadProtocolMessage(reader)
			if err != nil {
				return
			}
		}
	}()

	ctx := context.Background()
	const numRequests = 3
	channels := make([]<-chan pendingResult, numRequests)

	for i := 0; i < numRequests; i++ {
		req := &godap.InitializeRequest{}
		req.Type = "request"
		req.Command = "initialize"
		req.Arguments = godap.InitializeRequestArguments{
			ClientID:  "test",
			AdapterID: "lldb-dap",
		}
		ch, err := client.SendAsync(ctx, req)
		if err != nil {
			t.Fatalf("SendAsync[%d] failed: %v", i, err)
		}
		channels[i] = ch
	}

	// Verify all 3 entries are in the pending map.
	client.pendingMu.Lock()
	if len(client.pending) != numRequests {
		t.Fatalf("expected %d pending entries, got %d", numRequests, len(client.pending))
	}
	client.pendingMu.Unlock()

	// Cancel all pending.
	cancelErr := errors.New("connection closed")
	client.cancelAllPending(cancelErr)

	// Verify the pending map is empty.
	client.pendingMu.Lock()
	if len(client.pending) != 0 {
		t.Errorf("expected 0 pending entries after cancelAllPending, got %d", len(client.pending))
	}
	client.pendingMu.Unlock()

	// Verify each channel received an error.
	for i, ch := range channels {
		select {
		case result := <-ch:
			if result.err == nil {
				t.Errorf("channel[%d]: expected error, got nil", i)
			} else if !errors.Is(result.err, cancelErr) {
				t.Errorf("channel[%d]: expected %v, got %v", i, cancelErr, result.err)
			}
			if result.msg != nil {
				t.Errorf("channel[%d]: expected nil msg, got %T", i, result.msg)
			}
		default:
			t.Errorf("channel[%d]: expected result to be available immediately", i)
		}
	}
}

func TestNextSeqIncrementing(t *testing.T) {
	client := NewClient(bufio.NewReader(&io.LimitedReader{R: nil, N: 0}), io.Discard)

	for i := 1; i <= 10; i++ {
		got := client.nextSeq()
		if got != i {
			t.Errorf("nextSeq() call %d: got %d, want %d", i, got, i)
		}
	}
}

func TestDispatchResponseNoWaiter(t *testing.T) {
	// dispatchResponse should not panic when there is no waiter.
	client := NewClient(bufio.NewReader(&io.LimitedReader{R: nil, N: 0}), io.Discard)

	resp := makeInitializeResponse(1, 999)
	// Should log but not panic.
	client.dispatchResponse(resp)
}
