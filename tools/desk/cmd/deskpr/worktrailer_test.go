package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

// regression: #5704
func TestWorkTrailerSourceHold(t *testing.T) {
	root := t.TempDir()
	_, err := requireTrailer([]byte("Brief: stream:b-08@2"), root, root)
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("new source observation must hold before legacy reduction: %v", err)
	}
}
