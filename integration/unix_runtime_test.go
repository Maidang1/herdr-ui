//go:build integration && (darwin || linux)

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Linux CI must supply its checksum-pinned runtime. Never discover the user's
// Linux installation implicitly. All callers launch a disposable daemon with
// their own socket and config; the macOS harness keeps its existing discovery.
func unixTestBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("HERDR_TEST_BINARY")
	if binary == "" {
		if runtime.GOOS == "linux" {
			t.Skip("set HERDR_TEST_BINARY to an explicit Herdr Linux executable")
		}
		var err error
		binary, err = exec.LookPath("herdr")
		if err != nil {
			t.Skip("Herdr not installed")
		}
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		t.Fatalf("invalid HERDR_TEST_BINARY %q: %v", binary, err)
	}
	return binary
}
