package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestOwnedReposEqualsStatusgenScanRepos ENFORCES the invariant that board.go's
// ownedRepos and statusgen's scanRepos are the SAME owned-repo set — the read
// surface and the write surface of one intake lane.
//
// WHY THIS EXISTS: scanRepos is what actually WRITES issue placeholders; ownedRepos
// is what the board READS to report coverage. They once lived as two hand-synced
// compiled lists in separate modules with nothing but a comment holding them
// together, and they duly drifted — scanRepos went to 12 (#152/#193) while
// ownedRepos stayed at the original 3, so the board showed a clean front door while
// many issues the desk owns sat invisible to it (#784). A comment is not an
// enforcement.
//
// HOW THE COUPLING IS NOW ENFORCED. The owned-repo
// roster was externalised out of BOTH modules' source into ONE roster key,
// ASSAY_SCAN_REPOS: board.go's ownedRepos reads deskkit.ScanRepos(), and statusgen's
// scanRepos reads scanEffectiveConfig().ScanRepos — the identical key. Two lists a
// comment begged you to keep equal became one value both sides read, so they can no
// longer drift. This test proves BOTH halves of that: (1) the board's ownedRepos
// genuinely resolves through the roster (deskkit.ScanRepos), non-vacuously under the
// fixture roster; and (2) statusgen still consumes the same env key and reads it
// from config rather than re-compiling a literal list. If either half regresses —
// the board hard-codes a list again, or statusgen stops reading ASSAY_SCAN_REPOS —
// the single source of truth is broken and this fires. Re-establish the shared key;
// do NOT delete this test.
func TestOwnedReposEqualsStatusgenScanRepos(t *testing.T) {
	// (1) The board's read surface resolves through the roster, not a literal.
	owned := ownedRepos()
	if len(owned) == 0 {
		t.Fatal("ownedRepos() is empty under the fixture roster — either the fixture no longer sets " +
			"ASSAY_SCAN_REPOS or ownedRepos stopped reading it; the board would then report an empty " +
			"front door. This coupling must stay checkable — re-point it, do NOT delete it")
	}
	// ownedRepos MUST resolve through deskkit's parsed ScanRepos — the SAME roster
	// value statusgen reads. If ownedRepos were re-hardcoded, this identity breaks.
	cfg := deskkit.ScanRepos()
	if strings.Join(owned, ",") != strings.Join(cfg, ",") {
		t.Errorf("ownedRepos() does not resolve through deskkit.ScanRepos() — the board's read surface "+
			"is no longer the configured intake scope, so it can drift from the scanner again (#784).\n"+
			"  ownedRepos()        = %v\n  deskkit.ScanRepos() = %v", owned, cfg)
	}
	// No slug may repeat: a duplicate would mask a missing repo.
	seen := map[string]bool{}
	for _, r := range owned {
		if seen[r] {
			t.Errorf("ownedRepos contains %q twice — a duplicate can mask a missing repo", r)
		}
		seen[r] = true
	}

	// (2) statusgen consumes the IDENTICAL roster key, read from config. Reading a
	// sibling module's source from a test is deliberate and has precedent here
	// (internal/deskkit/scancoupling_test.go reads the same statusgen tree). The path
	// is ../../../../statusgen because statusgen lives at the repo root, not under
	// tools/.
	src := readStatusgenNonTestSource(t)
	if !strings.Contains(src, `"ASSAY_SCAN_REPOS"`) {
		t.Error("statusgen source no longer declares the ASSAY_SCAN_REPOS key — the board and the " +
			"scanner would read different roster values and could drift (#784). Re-establish the shared key")
	}
	if !strings.Contains(src, "scanEffectiveConfig().ScanRepos") {
		t.Error("statusgen's scanRepos no longer reads scanEffectiveConfig().ScanRepos — if it went back " +
			"to a compiled literal, the single-source coupling with the board is broken (#784)")
	}
}

// readStatusgenNonTestSource concatenates statusgen's non-test .go source so the
// coupling can be asserted against the actual code, not a comment.
func readStatusgenNonTestSource(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "statusgen")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s — ownedRepos is coupled to statusgen's scanRepos and that coupling "+
			"must stay checkable from here; if statusgen moved, re-point this test, do NOT delete it: %v", dir, err)
	}
	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("cannot read %s: %v", filepath.Join(dir, name), err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		t.Fatalf("no non-test .go files found in %s — re-point this test, do NOT delete it", dir)
	}
	return b.String()
}
