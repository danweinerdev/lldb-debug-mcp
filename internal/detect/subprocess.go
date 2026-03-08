package detect

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// SubprocessResult holds the spawned subprocess and its I/O handles.
type SubprocessResult struct {
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser // write DAP requests here
	Stdout    *bufio.Reader  // read DAP responses here
	Stderr    *StderrBuffer  // captured stderr
	IsLLDBDAP bool           // true if binary is lldb-dap (supports --repl-mode)
}

// StderrBuffer is a thread-safe ring buffer that captures the last N bytes of stderr.
type StderrBuffer struct {
	mu   sync.Mutex
	buf  []byte
	size int // max capacity
}

// NewStderrBuffer creates a buffer with the given max size.
// If size is <= 0, it defaults to 4096 bytes.
func NewStderrBuffer(size int) *StderrBuffer {
	if size <= 0 {
		size = 4096
	}
	return &StderrBuffer{
		buf:  make([]byte, 0, size),
		size: size,
	}
}

// Write implements io.Writer. It is thread-safe. If total bytes exceed the
// buffer's capacity, only the last size bytes are kept (ring buffer behavior).
func (b *StderrBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := len(p)

	// If the incoming data alone exceeds capacity, keep only the tail.
	if len(p) >= b.size {
		b.buf = make([]byte, b.size)
		copy(b.buf, p[len(p)-b.size:])
		return n, nil
	}

	b.buf = append(b.buf, p...)

	// Trim from the front if we exceed capacity.
	if len(b.buf) > b.size {
		excess := len(b.buf) - b.size
		// Shift data to avoid holding onto the old backing array forever.
		copy(b.buf, b.buf[excess:])
		b.buf = b.buf[:b.size]
	}

	return n, nil
}

// String returns the captured content as a string. It is thread-safe.
func (b *StderrBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// SpawnLLDBDAP spawns the lldb-dap binary as a subprocess with piped
// stdin/stdout and a goroutine draining stderr into a StderrBuffer.
//
// If isLLDBDAP is true, --repl-mode=command is passed as a CLI argument.
func SpawnLLDBDAP(path string, isLLDBDAP bool) (*SubprocessResult, error) {
	var args []string
	if isLLDBDAP {
		args = append(args, "--repl-mode=command")
	}

	cmd := exec.Command(path, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	stderrBuf := NewStderrBuffer(4096)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting subprocess %q: %w", path, err)
	}

	// Drain stderr in background to prevent pipe buffer deadlocks.
	go func() {
		_, _ = io.Copy(stderrBuf, stderr)
	}()

	return &SubprocessResult{
		Cmd:       cmd,
		Stdin:     stdin,
		Stdout:    bufio.NewReader(stdout),
		Stderr:    stderrBuf,
		IsLLDBDAP: isLLDBDAP,
	}, nil
}
