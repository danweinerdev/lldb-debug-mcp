package detect

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStderrBuffer_Basic(t *testing.T) {
	buf := NewStderrBuffer(4096)
	data := "hello, stderr"
	n, err := buf.Write([]byte(data))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned n=%d, want %d", n, len(data))
	}
	if got := buf.String(); got != data {
		t.Errorf("String() = %q, want %q", got, data)
	}
}

func TestStderrBuffer_Overflow(t *testing.T) {
	buf := NewStderrBuffer(10)
	data := "abcdefghijklmnop" // 16 bytes, capacity is 10
	n, err := buf.Write([]byte(data))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned n=%d, want %d", n, len(data))
	}
	// Should keep only the last 10 bytes.
	want := "ghijklmnop"
	if got := buf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStderrBuffer_MultipleWrites(t *testing.T) {
	buf := NewStderrBuffer(10)
	// Write 5 bytes three times (15 total), capacity is 10.
	writes := []string{"abcde", "fghij", "klmno"}
	for _, w := range writes {
		n, err := buf.Write([]byte(w))
		if err != nil {
			t.Fatalf("Write(%q) returned error: %v", w, err)
		}
		if n != len(w) {
			t.Errorf("Write(%q) returned n=%d, want %d", w, n, len(w))
		}
	}
	// Should keep only the last 10 bytes: "fghijklmno"
	want := "fghijklmno"
	if got := buf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStderrBuffer_Concurrent(t *testing.T) {
	buf := NewStderrBuffer(4096)
	const goroutines = 10
	const writesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_, _ = buf.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()

	got := buf.String()
	totalWritten := goroutines * writesPerGoroutine
	if len(got) != totalWritten {
		t.Errorf("len(String()) = %d, want %d", len(got), totalWritten)
	}
	// Every byte should be 'x'.
	for i, c := range got {
		if c != 'x' {
			t.Errorf("String()[%d] = %q, want 'x'", i, c)
			break
		}
	}
}

func TestStderrBuffer_DefaultSize(t *testing.T) {
	buf := NewStderrBuffer(0)
	if buf.size != 4096 {
		t.Errorf("default size = %d, want 4096", buf.size)
	}

	buf2 := NewStderrBuffer(-1)
	if buf2.size != 4096 {
		t.Errorf("default size for negative = %d, want 4096", buf2.size)
	}
}

func TestSpawnSubprocess(t *testing.T) {
	// Spawn a shell that writes to stderr then echoes stdin to stdout.
	// This avoids needing lldb-dap to be installed.
	result, err := SpawnLLDBDAP("sh", false)
	if err != nil {
		t.Fatalf("SpawnLLDBDAP failed: %v", err)
	}

	// Send a command that writes to stderr and echoes to stdout.
	_, err = result.Stdin.Write([]byte("echo hello_stderr >&2\necho hello_stdout\n"))
	if err != nil {
		t.Fatalf("writing to stdin: %v", err)
	}

	// Read the stdout line.
	line, err := result.Stdout.ReadString('\n')
	if err != nil {
		t.Fatalf("reading from stdout: %v", err)
	}
	line = strings.TrimSpace(line)
	if line != "hello_stdout" {
		t.Errorf("stdout line = %q, want %q", line, "hello_stdout")
	}

	// Close stdin to signal EOF, then wait for the process to exit.
	if err := result.Stdin.Close(); err != nil {
		t.Fatalf("closing stdin: %v", err)
	}

	if err := result.Cmd.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}

	// Give the stderr drain goroutine a moment to finish copying.
	time.Sleep(50 * time.Millisecond)

	// Verify stderr was captured.
	stderrContent := result.Stderr.String()
	if !strings.Contains(stderrContent, "hello_stderr") {
		t.Errorf("stderr = %q, want it to contain %q", stderrContent, "hello_stderr")
	}
}

func TestSpawnSubprocess_ExitDetected(t *testing.T) {
	// Spawn a process that exits immediately with code 0.
	result, err := SpawnLLDBDAP("sh", false)
	if err != nil {
		t.Fatalf("SpawnLLDBDAP failed: %v", err)
	}

	// Tell sh to exit with code 0.
	_, err = result.Stdin.Write([]byte("exit 0\n"))
	if err != nil {
		t.Fatalf("writing to stdin: %v", err)
	}

	// Wait should return nil for a clean exit.
	if err := result.Cmd.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}

	// ProcessState should be available after Wait.
	if result.Cmd.ProcessState == nil {
		t.Fatal("ProcessState is nil after Wait")
	}
	if !result.Cmd.ProcessState.Exited() {
		t.Error("ProcessState.Exited() = false, want true")
	}
}

func TestSpawnSubprocess_IsLLDBDAPFlag(t *testing.T) {
	// Verify that the IsLLDBDAP field is passed through correctly.
	// Use "echo" as a dummy binary that exits immediately (no --repl-mode needed).
	result, err := SpawnLLDBDAP("sh", false)
	if err != nil {
		t.Fatalf("SpawnLLDBDAP(sh, false) failed: %v", err)
	}
	if result.IsLLDBDAP {
		t.Error("IsLLDBDAP = true, want false")
	}
	_ = result.Stdin.Close()
	_ = result.Cmd.Wait()

	// We cannot easily test isLLDBDAP=true without an actual binary that
	// accepts --repl-mode=command, but we can verify the flag is set on the
	// result struct by checking the command args.
	// Use "true" as a binary that ignores arguments.
	result2, err := SpawnLLDBDAP("true", true)
	if err != nil {
		t.Fatalf("SpawnLLDBDAP(true, true) failed: %v", err)
	}
	if !result2.IsLLDBDAP {
		t.Error("IsLLDBDAP = false, want true")
	}
	// Verify --repl-mode=command is in the command args.
	found := false
	for _, arg := range result2.Cmd.Args {
		if arg == "--repl-mode=command" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --repl-mode=command in args, got %v", result2.Cmd.Args)
	}
	_ = result2.Cmd.Wait()
}
