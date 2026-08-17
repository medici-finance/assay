package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// deskboardReadmePath is tools/desk/README.md, relative to this package
// (tools/desk/cmd/deskboard).
var deskboardReadmePath = filepath.Join("..", "..", "README.md")

// skipIfReadmeAbsent skips a README-content-integrity test when
// tools/desk/README.md is not present in this checkout.
//
// tools/desk/README.md is not part of every checkout of this repository; when it
// is absent these README-parity checks have nothing to read, so they skip. Only
// os.ErrNotExist skips — a README that exists but cannot be read still fails —
// and where the README is present the checks run in full.
func skipIfReadmeAbsent(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(deskboardReadmePath); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree", deskboardReadmePath)
	}
}
