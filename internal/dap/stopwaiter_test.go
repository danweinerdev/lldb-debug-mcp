package dap

import (
	"sync"
	"testing"
	"time"

	godap "github.com/google/go-dap"
)

func TestStopWaiterRegisterAndDeliver(t *testing.T) {
	w := &StopWaiter{}
	ch := w.Register()

	event := &godap.StoppedEvent{}
	event.Type = "event"
	event.Event.Event = "stopped"
	event.Body = godap.StoppedEventBody{
		Reason:   "breakpoint",
		ThreadId: 1,
	}

	w.Deliver(event)

	select {
	case result := <-ch:
		if result.Event == nil {
			t.Fatal("expected Event to be non-nil")
		}
		if result.Event.Body.Reason != "breakpoint" {
			t.Errorf("Reason: got %q, want %q", result.Event.Body.Reason, "breakpoint")
		}
		if result.Event.Body.ThreadId != 1 {
			t.Errorf("ThreadId: got %d, want 1", result.Event.Body.ThreadId)
		}
		if result.Exited {
			t.Error("expected Exited to be false")
		}
		if result.Terminated {
			t.Error("expected Terminated to be false")
		}
		if result.ExitCode != nil {
			t.Error("expected ExitCode to be nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for StopResult")
	}
}

func TestStopWaiterRegisterAndDeliverExit(t *testing.T) {
	w := &StopWaiter{}
	ch := w.Register()

	w.DeliverExit(42)

	select {
	case result := <-ch:
		if !result.Exited {
			t.Error("expected Exited to be true")
		}
		if result.ExitCode == nil {
			t.Fatal("expected ExitCode to be non-nil")
		}
		if *result.ExitCode != 42 {
			t.Errorf("ExitCode: got %d, want 42", *result.ExitCode)
		}
		if result.Event != nil {
			t.Error("expected Event to be nil")
		}
		if result.Terminated {
			t.Error("expected Terminated to be false")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for StopResult")
	}
}

func TestStopWaiterRegisterAndCancel(t *testing.T) {
	w := &StopWaiter{}
	ch := w.Register()

	w.Cancel()

	select {
	case result := <-ch:
		if !result.Terminated {
			t.Error("expected Terminated to be true")
		}
		if result.Event != nil {
			t.Error("expected Event to be nil")
		}
		if result.Exited {
			t.Error("expected Exited to be false")
		}
		if result.ExitCode != nil {
			t.Error("expected ExitCode to be nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for StopResult")
	}
}

func TestStopWaiterDeliverNoWaiter(t *testing.T) {
	w := &StopWaiter{}

	event := &godap.StoppedEvent{}
	event.Type = "event"
	event.Event.Event = "stopped"
	event.Body = godap.StoppedEventBody{Reason: "breakpoint"}

	// Should not panic or block.
	w.Deliver(event)
}

func TestStopWaiterDeliverExitNoWaiter(t *testing.T) {
	w := &StopWaiter{}

	// Should not panic or block.
	w.DeliverExit(1)
}

func TestStopWaiterCancelNoWaiter(t *testing.T) {
	w := &StopWaiter{}

	// Should not panic or block.
	w.Cancel()
}

func TestStopWaiterConcurrent(t *testing.T) {
	w := &StopWaiter{}

	var wg sync.WaitGroup
	const iterations = 100

	// Concurrent Register + Deliver from different goroutines.
	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			w.Register()
		}()

		go func() {
			defer wg.Done()
			event := &godap.StoppedEvent{}
			event.Type = "event"
			event.Event.Event = "stopped"
			event.Body = godap.StoppedEventBody{Reason: "step"}
			w.Deliver(event)
		}()
	}

	wg.Wait()

	// Also test concurrent DeliverExit and Cancel.
	for i := 0; i < iterations; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			w.Register()
		}()

		go func() {
			defer wg.Done()
			w.DeliverExit(0)
		}()

		go func() {
			defer wg.Done()
			w.Cancel()
		}()
	}

	wg.Wait()
}
