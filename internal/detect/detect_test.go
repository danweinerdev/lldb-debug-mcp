package detect

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeDummyBin creates an empty executable file in dir with the given name
// and returns its full path.
func makeDummyBin(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindLLDBDAP_EnvVar(t *testing.T) {
	tmp := t.TempDir()
	binPath := makeDummyBin(t, tmp, "my-custom-lldb-dap")

	t.Setenv("LLDB_DAP_PATH", binPath)
	// Set PATH to empty so nothing else is found.
	t.Setenv("PATH", "")

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != binPath {
		t.Errorf("path = %q, want %q", path, binPath)
	}
	if !isLLDBDAP {
		t.Error("isLLDBDAP = false, want true (basename contains lldb-dap)")
	}
}

func TestFindLLDBDAP_EnvVarLLDBVscode(t *testing.T) {
	tmp := t.TempDir()
	binPath := makeDummyBin(t, tmp, "lldb-vscode")

	t.Setenv("LLDB_DAP_PATH", binPath)
	t.Setenv("PATH", "")

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != binPath {
		t.Errorf("path = %q, want %q", path, binPath)
	}
	if isLLDBDAP {
		t.Error("isLLDBDAP = true, want false (basename is lldb-vscode)")
	}
}

func TestFindLLDBDAP_EnvVarPriority(t *testing.T) {
	// Both LLDB_DAP_PATH and lldb-dap in PATH exist.
	// LLDB_DAP_PATH must win.
	envDir := t.TempDir()
	envBin := makeDummyBin(t, envDir, "custom-lldb-dap-binary")

	pathDir := t.TempDir()
	makeDummyBin(t, pathDir, "lldb-dap")

	t.Setenv("LLDB_DAP_PATH", envBin)
	t.Setenv("PATH", pathDir)

	path, _, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != envBin {
		t.Errorf("path = %q, want %q (env var should take priority)", path, envBin)
	}
}

func TestFindLLDBDAP_InPath(t *testing.T) {
	tmp := t.TempDir()
	makeDummyBin(t, tmp, "lldb-dap")

	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", tmp)

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "lldb-dap")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
	if !isLLDBDAP {
		t.Error("isLLDBDAP = false, want true")
	}
}

func TestFindLLDBDAP_VersionedInPath(t *testing.T) {
	tmp := t.TempDir()
	// Only lldb-dap-18 is present.
	makeDummyBin(t, tmp, "lldb-dap-18")

	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", tmp)

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "lldb-dap-18")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
	if !isLLDBDAP {
		t.Error("isLLDBDAP = false, want true")
	}
}

func TestFindLLDBDAP_VersionedPrefersHigher(t *testing.T) {
	tmp := t.TempDir()
	// Both lldb-dap-15 and lldb-dap-19 are present; 19 should win
	// because we search from 20 down.
	makeDummyBin(t, tmp, "lldb-dap-15")
	makeDummyBin(t, tmp, "lldb-dap-19")

	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", tmp)

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "lldb-dap-19")
	if path != expected {
		t.Errorf("path = %q, want %q (should prefer higher version)", path, expected)
	}
	if !isLLDBDAP {
		t.Error("isLLDBDAP = false, want true")
	}
}

func TestFindLLDBDAP_FallbackToLLDBVscode(t *testing.T) {
	tmp := t.TempDir()
	makeDummyBin(t, tmp, "lldb-vscode")

	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", tmp)

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "lldb-vscode")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
	if isLLDBDAP {
		t.Error("isLLDBDAP = true, want false")
	}
}

func TestFindLLDBDAP_NothingFound(t *testing.T) {
	// Empty PATH, no env var set.
	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", t.TempDir()) // empty directory

	_, _, err := FindLLDBDAP()
	if err == nil {
		t.Fatal("expected error when nothing is found, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "lldb-dap binary not found") {
		t.Errorf("error should mention 'lldb-dap binary not found', got: %s", errMsg)
	}
	// Should mention the names that were searched.
	for _, name := range []string{"lldb-dap", "lldb-dap-18", "lldb-vscode"} {
		if !strings.Contains(errMsg, name) {
			t.Errorf("error should mention %q, got: %s", name, errMsg)
		}
	}
	// On macOS, should also mention xcrun.
	if runtime.GOOS == "darwin" {
		if !strings.Contains(errMsg, "xcrun") {
			t.Errorf("error on macOS should mention xcrun, got: %s", errMsg)
		}
	}
}

func TestFindLLDBDAP_LLDBDAPBeforeLLDBVscode(t *testing.T) {
	// When both lldb-dap and lldb-vscode are in PATH, lldb-dap wins.
	tmp := t.TempDir()
	makeDummyBin(t, tmp, "lldb-dap")
	makeDummyBin(t, tmp, "lldb-vscode")

	t.Setenv("LLDB_DAP_PATH", "")
	t.Setenv("PATH", tmp)

	path, isLLDBDAP, err := FindLLDBDAP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "lldb-dap")
	if path != expected {
		t.Errorf("path = %q, want %q (lldb-dap should be preferred)", path, expected)
	}
	if !isLLDBDAP {
		t.Error("isLLDBDAP = false, want true")
	}
}
