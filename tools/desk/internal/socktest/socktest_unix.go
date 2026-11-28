//go:build unix

// Package socktest provides short owned directories for _test.go files that bind a Unix
// socket. A test whose socket directory would otherwise follow TMPDIR takes it from here,
// so a long managed TMPDIR cannot push a socket path past the platform's sun_path limit.
package socktest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Dir returns a fresh owner-only directory that the test creates and removes, short
// enough that Dir/suffix fits a Unix socket address (sun_path is 104 bytes on macOS and
// 108 on Linux, NUL included). It prefers TMPDIR; a managed task scratch TMPDIR can be
// too long for that, so it then tries a fresh private directory under /tmp. If neither
// fits, the socket prerequisite is could-not-check and the test is skipped saying so,
// never reported as a pass of the socket round trip.
func Dir(t testing.TB, pattern, suffix string) string {
	t.Helper()
	limit := len(syscall.RawSockaddrUnix{}.Path) - 1
	var tried []string
	for _, parent := range []string{os.TempDir(), "/tmp"} {
		dir, err := os.MkdirTemp(parent, pattern)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", parent, err))
			continue
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			_ = os.RemoveAll(dir)
			tried = append(tried, fmt.Sprintf("%s: %v", parent, err))
			continue
		}
		if n := len(filepath.Join(resolved, suffix)); n > limit {
			_ = os.RemoveAll(resolved)
			tried = append(tried, fmt.Sprintf("%s: socket path %d bytes, limit %d", parent, n, limit))
			continue
		}
		t.Cleanup(func() { _ = os.Chmod(resolved, 0700); _ = os.RemoveAll(resolved) })
		return resolved
	}
	t.Skipf("could-not-check: no owned socket-safe directory for %s (%s)", suffix, strings.Join(tried, "; "))
	return ""
}
