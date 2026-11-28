//go:build !unix

// Package socktest provides short owned directories for _test.go files that bind a Unix
// socket. Off unix there is no sun_path limit to enforce (on Windows the local transport is
// a named pipe), so the directory is only a scratch location.
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
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
