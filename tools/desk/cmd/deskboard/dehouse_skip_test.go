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
// The README is human-ratified to ship 100% (issue #1276) but that flip lands
// in the dist/12b manifest change; until then it is do-not-copy and absent from
// the de-housed public medici-finance/assay copy, so these README-parity checks
// have nothing to read. Only os.ErrNotExist skips — a README that exists but
// cannot be read still fails — and in the source repo the README is always
// present, so the guard never fires there and the checks run in full.
//
// NOTE: this is a stopgap; the README SHOULD ship (see #1276), at which point
// this guard stops firing and these checks run here too.
func skipIfReadmeAbsent(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(deskboardReadmePath); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree — ships via the ratified dist/12b flip (issue #1276); do-not-copy until then", deskboardReadmePath)
	}
}
