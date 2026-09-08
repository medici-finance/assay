package desk_test

// This is the LAYOUT GUARD. It is a package-level test in the module root
// (github.com/medici-finance/assay/tools/desk), deliberately NOT inside the
// askassay package, because the fault it catches is a filesystem fact no compile
// error would surface: the ask-pane numbers-rule layer being moved back under an
// internal/ path.
//
// Why this matters, and why a compile error cannot substitute for it: an
// internal/ package is importable only from code rooted at the parent of
// internal/. Moving askassay to tools/desk/internal/askassay keeps every
// IN-MODULE import working and the whole module compiling green — which is
// exactly why an earlier such move broke a consumer in a DIFFERENT module while
// this module's own build stayed silent. The only signal is a filesystem
// assertion, so that is what this guard makes.

import (
	"os"
	"testing"
)

// TestLayout fails if the old internal path exists again. It stats relative to
// the working directory, which `go test` sets to this package's own directory
// (the module root), so it also holds for a copied tree run with `go test .`.
func TestLayout(t *testing.T) {
	const internalPath = "internal/askassay"

	info, err := os.Stat(internalPath)
	if err == nil && info.IsDir() {
		t.Fatalf("layout guard: %q exists again. Another Go module cannot import an "+
			"internal/ path, so re-homing the ask-pane numbers-rule layer under internal/ "+
			"silently breaks every external consumer of "+
			"github.com/medici-finance/assay/tools/desk/askassay while this module's own "+
			"build stays green. Keep the package at tools/desk/askassay.", internalPath)
	}
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("layout guard: could not stat %q: %v", internalPath, err)
	}

	// Positive assertion: the importable path IS present, so the guard is wired
	// to the real tree and not vacuously green in an empty checkout.
	if info, err := os.Stat("askassay"); err != nil || !info.IsDir() {
		t.Fatalf("layout guard: importable path %q is missing (err=%v) — the package a "+
			"consumer imports must live here.", "askassay", err)
	}
}
