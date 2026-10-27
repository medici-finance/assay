package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

// regression: #4405
// TestDeskinstallHotfixReaders: resolvePin's tag grammar admits only a bare vX.Y.Z, so a
// hotfix tag can never reach a sha256-pinned download.
func TestDeskinstallHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("deskinstall")
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("a", 64)
	for _, c := range m.Cases {
		cm := componentManifest{ReleaseHome: "medici-finance/assay", Tag: c.Tag,
			Platforms: map[string]string{"linux-amd64": "desk-tools-linux-amd64.tar.gz " + c.Tag + " " + sum}}
		got := "accept"
		if _, err := resolvePin("desk-tools", cm, "linux-amd64"); err != nil {
			got = "refuse"
		}
		if want := c.Verdicts["deskinstall"]; got != want {
			t.Errorf("deskinstall reads %q as %s, matrix says %s", c.Tag, got, want)
		}
	}
}
