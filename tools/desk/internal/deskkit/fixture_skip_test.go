package deskkit

import (
	"errors"
	"os"
	"testing"
)

// skipIfFixtureAbsent skips a test whose fixture is a file or tree that is not
// part of every checkout of this repository, when that path is genuinely absent
// from the current checkout.
//
// It guards the desk-tools suite when it runs inside a published subset of this
// repository that does not carry .github/ (CI wiring), .claude/ (skills and
// settings; the shipped adopter skills live under plugins/assay/skills) or
// go.work (the subset ships its own).
//
// Fail-closed intent is preserved: ONLY os.ErrNotExist skips, so a fixture that
// exists but cannot be read still fails; and where every such fixture is
// present, the guard never fires and the test runs in full against the real
// tree.
func skipIfFixtureAbsent(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Skipf("fixture %s not present in this tree — %s", path, why)
	}
}
