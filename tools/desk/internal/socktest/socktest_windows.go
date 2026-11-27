//go:build windows

// Package socktest provides short owned directories for _test.go files that bind a Unix
// socket. On Windows the local transport is a named pipe, so the directory is only a
// scratch location with no length limit to enforce.
package socktest

import (
	"os"
	"testing"
)

// Dir returns a fresh directory that the test creates and removes.
func Dir(t testing.TB, pattern, suffix string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
