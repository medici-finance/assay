package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

// regression: #4405
// TestUpgradeAssayHotfixReaders: only a bare vX.Y.Z is a resolvable upgrade target; a
// hotfix tag is refused.
func TestUpgradeAssayHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("upgrade-assay")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Cases {
		got := "refuse"
		if bareUmbrella.MatchString(c.Tag) {
			got = "accept"
		}
		if want := c.Verdicts["upgrade-assay"]; got != want {
			t.Errorf("upgrade-assay reads %q as %s, matrix says %s", c.Tag, got, want)
		}
	}
}
