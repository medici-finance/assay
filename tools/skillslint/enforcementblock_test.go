package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureBlock is a minimal generated block whose first line is the real anchor,
// so region-finding behaves exactly as in production. Its rows carry all three
// statuses so a hand-edit of any one is detectable.
const fixtureBlock = enforcementAnchor + `

Regenerate with the skillslint sync.

| rule | what it checks | status |
| --- | --- | --- |
| ` + "`alpha`" + ` | checks alpha | fatal |
| ` + "`beta`" + ` | checks beta | advisory |
| ` + "`gamma`" + ` | checks gamma | not enforced |`

// writeEnforcementFixture lays out a repo whose guidance document carries block
// as its generated region, bounded by a following `## ` heading.
func writeEnforcementFixture(t *testing.T, block string) (root string) {
	t.Helper()
	root = t.TempDir()
	abs := filepath.Join(root, filepath.FromSlash(enforcementSitePath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "# Guidance\n\n## A section\n\nprose\n\n" + block + "\n\n## Before dispatch\n\nmore prose\n"
	if err := os.WriteFile(abs, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func fixtureDerive(block string) func() (string, error) {
	return func() (string, error) { return block, nil }
}

// TestGeneratedBlockDiffFailsOnHandEdit is the positive control (brief task 6): a
// hand-edited generated block reddens the gate, and a freshly generated one does
// not. Both directions are asserted so a check that always passes — the failure
// mode that makes a gate worthless — cannot slip through.
func TestGeneratedBlockDiffFailsOnHandEdit(t *testing.T) {
	derive := fixtureDerive(fixtureBlock)

	// Fresh: the copy matches the derived block → clean.
	root := writeEnforcementFixture(t, fixtureBlock)
	rep := CheckEnforcementBlock(root, derive)
	if !rep.Compared || rep.Failed != nil || rep.Unchecked != nil {
		t.Fatalf("a freshly generated block should pass: %+v (failed=%v unchecked=%v)", rep, rep.Failed, rep.Unchecked)
	}

	// Hand-edit one status cell in the copy → the gate must fail.
	edited := strings.Replace(fixtureBlock, "| checks alpha | fatal |", "| checks alpha | advisory |", 1)
	if edited == fixtureBlock {
		t.Fatal("test setup: the hand-edit did not change the block")
	}
	root2 := writeEnforcementFixture(t, edited)
	rep2 := CheckEnforcementBlock(root2, derive)
	if rep2.Failed == nil {
		t.Fatalf("a hand-edited block must red the gate, got %+v", rep2)
	}
	if rep2.Compared {
		t.Errorf("a drifted block must not report Compared-clean")
	}
}

// TestEnforcementBlockAnchorMissingIsCouldNotCheck — deleting the block (its
// anchor absent) is could-not-check, never a silent pass.
func TestEnforcementBlockAnchorMissingIsCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, filepath.FromSlash(enforcementSitePath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("# Guidance\n\nno block here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := CheckEnforcementBlock(root, fixtureDerive(fixtureBlock))
	if rep.Unchecked == nil {
		t.Fatalf("a missing anchor must be could-not-check, got %+v", rep)
	}
	if rep.Compared {
		t.Errorf("could-not-check must not report Compared-clean")
	}
}

// TestEnforcementBlockDeriveErrorIsCouldNotCheck — if the source cannot be
// derived (the emitter failed), the check is could-not-check, not a pass.
func TestEnforcementBlockDeriveErrorIsCouldNotCheck(t *testing.T) {
	root := writeEnforcementFixture(t, fixtureBlock)
	badDerive := func() (string, error) { return "", os.ErrNotExist }
	rep := CheckEnforcementBlock(root, badDerive)
	if rep.Unchecked == nil {
		t.Fatalf("a derive error must be could-not-check, got %+v", rep)
	}
}

// TestSyncEnforcementBlockRewritesStaleCopy — sync regenerates a drifted copy
// from the derived source and CheckEnforcementBlock then passes.
func TestSyncEnforcementBlockRewritesStaleCopy(t *testing.T) {
	stale := strings.Replace(fixtureBlock, "| checks beta | advisory |", "| checks beta | fatal |", 1)
	root := writeEnforcementFixture(t, stale)
	derive := fixtureDerive(fixtureBlock)

	// Precondition: the stale copy is detected as drift.
	if CheckEnforcementBlock(root, derive).Failed == nil {
		t.Fatal("test setup: stale copy should be detected as drift before sync")
	}

	changed, rep := SyncEnforcementBlock(root, derive)
	if rep.Unchecked != nil {
		t.Fatalf("sync could-not-check: %v", rep.Unchecked.Msg)
	}
	if !changed {
		t.Fatal("sync reported no change over a stale copy")
	}
	if got := CheckEnforcementBlock(root, derive); !got.Compared || got.Failed != nil {
		t.Fatalf("after sync the copy should byte-match: %+v", got)
	}
}

// TestDeriveEnforcementBlockShellsOutToStatusgen proves the production derive
// path — shelling out to `statusgen enforcement-status` — actually returns the
// block. It runs against the real repo root and SKIPS (never fails) if statusgen
// cannot be built in this environment, so `go test ./tools/skillslint/` stays
// green in isolation while the real bridge is still exercised where a toolchain
// is present (the CI skillslint job).
func TestDeriveEnforcementBlockShellsOutToStatusgen(t *testing.T) {
	root := filepath.Join("..", "..") // tools/skillslint -> repo root
	block, err := deriveEnforcementBlock(root)
	if err != nil {
		t.Skipf("skipping shell-out integration test: %v", err)
	}
	if !strings.HasPrefix(block, enforcementAnchor) {
		t.Errorf("derived block does not start with the anchor line; got first line %q", strings.SplitN(block, "\n", 2)[0])
	}
	if !strings.Contains(block, "| fatal |") {
		t.Errorf("derived block does not carry a fatal row")
	}
}
