//go:build !darwin && !linux

package cellcache

import "testing"

func TestCacheUnsupported(t *testing.T) {
	if _, err := Resolve(t.TempDir(), func(string) string { return "on" }); err == nil {
		t.Fatal("unsupported platform admitted managed cache")
	}
}
