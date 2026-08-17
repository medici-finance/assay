package deskkit

import (
	"errors"
	"os"
	"testing"
)

// skipIfDehoused skips a test whose fixture is a house-only file or tree that
// the publication manifest classifies do-not-copy, when that path
// is genuinely absent from the current checkout.
//
// It is the Option-B guard for the desk-tools suite running inside the de-housed
// public medici-finance/assay copy, where the manifest withholds .github/
// (house CI — the public repo wires its own at publication/06), .claude/
// (house-local skills/settings; the shipped adopter skills live under
// plugins/assay/skills) and go.work (the public repo ships its own subset).
//
// Fail-closed intent is preserved: ONLY os.ErrNotExist skips, so a fixture that
// exists but cannot be read still fails; and in the source repo,
// where every such fixture is present, the guard never fires and the test runs
// in full against the real tree.
func skipIfDehoused(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Skipf("house fixture %s not present in this tree — %s", path, why)
	}
}
