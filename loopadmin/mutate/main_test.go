package main

import (
	"path/filepath"
	"testing"
)

// TestMutateConfinesFile: a map entry can only name a file inside the scratch
// copy of the module.
func TestMutateConfinesFile(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"", "/etc/hosts", "../outside.go", "runner/../../outside.go", ".."} {
		if got, err := confine(dir, bad); err == nil {
			t.Errorf("%q must be refused, resolved to %q", bad, got)
		}
	}
	// Positive control: a module-relative path resolves inside the copy.
	got, err := confine(dir, "runner/client.go")
	if err != nil || got != filepath.Join(dir, "runner", "client.go") {
		t.Fatalf("module path: %q %v", got, err)
	}
}
