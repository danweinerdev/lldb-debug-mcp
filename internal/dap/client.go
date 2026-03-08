package dap

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	godap "github.com/google/go-dap"
)

// pendingResult holds the response or error for a pending request.
type pendingResult struct {
	msg godap.Message
	err error
}

// Client is a DAP protocol client that correlates requests to responses
// using sequence numbers. It writes DAP messages to a writer and reads
// responses from a reader, dispatching them to callers via a pending map.
type Client struct {
	writer  io.Writer
	reader  *bufio.Reader
	writeMu sync.Mutex

	seq   int
	seqMu sync.Mutex

	pending   map[int]chan pendingResult
	pendingMu sync.Mutex

	closed    chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// NewClient creates a new DAP client that reads from reader and writes to
// writer. The caller must invoke ReadLoop in a goroutine to start processing
// incoming messages.
func NewClient(reader *bufio.Reader, writer io.Writer) *Client {
	return &Client{
		writer:  writer,
		reader:  reader,
		pending: make(map[int]chan pendingResult),
		closed:  make(chan struct{}),
	}
}

// nextSeq increments and returns the next sequence number.
func (c *Client) nextSeq() int {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()
	c.seq++
	return c.seq
}

// Send sends a DAP request and blocks until the corresponding response is
// received or the context is cancelled. It assigns a sequence number to the
// request, writes it to the wire, and waits for the response.
func (c *Client) Send(ctx context.Context, request godap.Message) (godap.Message, error) {
	ch, err := c.SendAsync(ctx, request)
	if err != nil {
		return nil, err
	}

	select {
	case result := <-ch:
		return result.msg, result.err
	case <-ctx.Done():
		// Remove from pending map on cancellation.
		if req, ok := request.(godap.RequestMessage); ok {
			seq := req.GetRequest().Seq
			c.pendingMu.Lock()
			delete(c.pending, seq)
			c.pendingMu.Unlock()
		}
		return nil, ctx.Err()
	}
}

// SendAsync sends a DAP request and returns a channel that will receive the
// response. This is useful when the caller needs to wait for multiple
// messages (e.g., both a response and an event) concurrently.
func (c *Client) SendAsync(ctx context.Context, request godap.Message) (<-chan pendingResult, error) {
	req, ok := request.(godap.RequestMessage)
	if !ok {
		return nil, fmt.Errorf("SendAsync: message is not a request: %T", request)
	}

	seq := c.nextSeq()
	req.GetRequest().Seq = seq

	ch := make(chan pendingResult, 1)

	c.pendingMu.Lock()
	c.pending[seq] = ch
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	err := godap.WriteProtocolMessage(c.writer, request)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, seq)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("SendAsync: write failed: %w", err)
	}

	return ch, nil
}

// dispatchResponse extracts the request_seq from a response message, looks up
// the corresponding pending channel, sends the response to it, and removes the
// entry from the pending map. If no waiter is found, it logs and discards.
func (c *Client) dispatchResponse(msg godap.Message) {
	resp, ok := msg.(godap.ResponseMessage)
	if !ok {
		log.Printf("dap.Client: dispatchResponse called with non-response: %T", msg)
		return
	}

	reqSeq := resp.GetResponse().RequestSeq

	c.pendingMu.Lock()
	ch, found := c.pending[reqSeq]
	if found {
		delete(c.pending, reqSeq)
	}
	c.pendingMu.Unlock()

	if !found {
		log.Printf("dap.Client: no waiter for response to request seq %d", reqSeq)
		return
	}

	ch <- pendingResult{msg: msg}
}

// cancelAllPending sends the given error to all pending request channels and
// clears the pending map. This is called when the read loop encounters an
// EOF or read error.
func (c *Client) cancelAllPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for seq, ch := range c.pending {
		ch <- pendingResult{err: err}
		delete(c.pending, seq)
	}
}

// ReadLoop reads DAP messages from the reader and dispatches them.
// It runs until the reader returns an error (including io.EOF), at which
// point it cancels all pending requests and closes the client.
// This method should be called in a goroutine.
func (c *Client) ReadLoop() {
	for {
		msg, err := godap.ReadProtocolMessage(c.reader)
		if err != nil {
			c.closeOnce.Do(func() {
				c.closeErr = err
				close(c.closed)
			})
			c.cancelAllPending(fmt.Errorf("dap.Client: read loop terminated: %w", err))
			return
		}

		switch msg.(type) {
		case godap.ResponseMessage:
			c.dispatchResponse(msg)
		default:
			// Events and other message types will be handled in task 1.4.
			log.Printf("dap.Client: unhandled message type: %T", msg)
		}
	}
}

// Closed returns a channel that is closed when the read loop exits.
func (c *Client) Closed() <-chan struct{} {
	return c.closed
}

// CloseErr returns the error that caused the read loop to exit, or nil if
// it has not exited yet.
func (c *Client) CloseErr() error {
	return c.closeErr
}
