// Package detect finds the lldb-dap (or lldb-vscode) binary on the system.
package detect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindLLDBDAP locates the lldb-dap or lldb-vscode binary using a fallback chain.
//
// It returns:
//   - path: the full path to the binary
//   - isLLDBDAP: true if the binary is named lldb-dap (LLVM 18+), false if
//     lldb-vscode (older). This flag determines whether --repl-mode=command
//     can be used.
//   - err: a descriptive error listing all searched paths on failure
//
// Detection order:
//  1. LLDB_DAP_PATH environment variable (if set, use directly)
//  2. lldb-dap in PATH
//  3. lldb-dap-{20,19,18,17,16,15} in PATH
//  4. lldb-vscode in PATH
//  5. macOS only: xcrun --find lldb-dap
func FindLLDBDAP() (path string, isLLDBDAP bool, err error) {
	var searched []string

	// 1. Check LLDB_DAP_PATH environment variable.
	if envPath := os.Getenv("LLDB_DAP_PATH"); envPath != "" {
		if _, err := exec.LookPath(envPath); err == nil {
			base := filepath.Base(envPath)
			return envPath, strings.Contains(base, "lldb-dap"), nil
		}
		// If the env var is set but the binary doesn't exist, still try to
		// stat it in case it's an absolute path not on PATH.
		if _, err := os.Stat(envPath); err == nil {
			base := filepath.Base(envPath)
			return envPath, strings.Contains(base, "lldb-dap"), nil
		}
		searched = append(searched, fmt.Sprintf("LLDB_DAP_PATH=%s", envPath))
	}

	// 2. Check lldb-dap in PATH.
	if p, err := exec.LookPath("lldb-dap"); err == nil {
		return p, true, nil
	}
	searched = append(searched, "lldb-dap")

	// 3. Check versioned lldb-dap-{20..15} in PATH.
	for v := 20; v >= 15; v-- {
		name := fmt.Sprintf("lldb-dap-%d", v)
		if p, err := exec.LookPath(name); err == nil {
			return p, true, nil
		}
		searched = append(searched, name)
	}

	// 4. Check lldb-vscode in PATH.
	if p, err := exec.LookPath("lldb-vscode"); err == nil {
		return p, false, nil
	}
	searched = append(searched, "lldb-vscode")

	// 5. macOS only: try xcrun --find lldb-dap.
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("xcrun", "--find", "lldb-dap").Output()
		if err == nil {
			p := strings.TrimSpace(string(out))
			if p != "" {
				return p, true, nil
			}
		}
		searched = append(searched, "xcrun --find lldb-dap")
	}

	return "", false, fmt.Errorf(
		"lldb-dap binary not found; searched: %s",
		strings.Join(searched, ", "),
	)
}
