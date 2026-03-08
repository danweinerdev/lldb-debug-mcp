package dap

import (
	"sync"

	godap "github.com/google/go-dap"
)

// StopResult holds the outcome of waiting for a stop-related event.
// Exactly one of Event, Exited, or Terminated will be set.
type StopResult struct {
	Event      *godap.StoppedEvent
	Exited     bool
	ExitCode   *int
	Terminated bool
	Err        error
}

// StopWaiter allows a single caller to wait for a stopped, exited, or
// terminated event. Only one waiter may be registered at a time; calling
// Register replaces any previous waiter.
type StopWaiter struct {
	mu sync.Mutex
	ch chan StopResult // nil when no waiter registered
}

// Register creates a new buffered(1) channel and stores it, replacing any
// previous waiter. The returned channel will receive exactly one StopResult
// when Deliver, DeliverExit, or Cancel is called.
func (w *StopWaiter) Register() <-chan StopResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.ch = make(chan StopResult, 1)
	return w.ch
}

// Deliver sends a StopResult containing the given StoppedEvent to the
// registered waiter, if any. After delivery, the waiter is cleared.
// If no waiter is registered, this is a no-op.
func (w *StopWaiter) Deliver(event *godap.StoppedEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ch == nil {
		return
	}

	w.ch <- StopResult{Event: event}
	w.ch = nil
}

// DeliverExit sends a StopResult indicating the debuggee exited with the
// given exit code. After delivery, the waiter is cleared.
// If no waiter is registered, this is a no-op.
func (w *StopWaiter) DeliverExit(exitCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ch == nil {
		return
	}

	w.ch <- StopResult{Exited: true, ExitCode: &exitCode}
	w.ch = nil
}

// Cancel sends a StopResult indicating the debug session was terminated
// (e.g., EOF on the connection). After delivery, the waiter is cleared.
// If no waiter is registered, this is a no-op.
func (w *StopWaiter) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ch == nil {
		return
	}

	w.ch <- StopResult{Terminated: true}
	w.ch = nil
}
