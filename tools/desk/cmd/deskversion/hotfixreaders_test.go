package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

// regression: #4405
// TestDeskversionHotfixReaders: an umbrella line carrying a hotfix tag resolves to no
// composition, so the marker is could-not-determine (exit 6) — never a known umbrella.
// Offline: no --fetch, only the pin's own composition is materialised.
func TestDeskversionHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("deskversion")
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("a", 64)
	for _, c := range m.Cases {
		root := t.TempDir()
		pins := "assay " + c.Tag + "\nstatusgen " + m.Pin + " " + sum + "\ndesk-tools " + m.Pin + " " + sum + "\n"
		comp := "umbrella: " + m.Pin + "\nartifacts:\n  - artifact: statusgen\n    tag: " + m.Pin +
			"\n  - artifact: desk-tools\n    tag: " + m.Pin + "\n"
		if err := os.MkdirAll(filepath.Join(root, deskkit.ReleasesDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, deskkit.AssayVersionsFile), []byte(pins), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, deskkit.ReleasesDir, deskkit.ManifestFileName(m.Pin)), []byte(comp), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		got := "refuse"
		if run([]string{"--root", root}, &out, &errb) == deskkit.ExitOK {
			got = "accept"
		}
		if want := c.Verdicts["deskversion"]; got != want {
			t.Errorf("deskversion reads umbrella %q as %s (%s%s), matrix says %s", c.Tag, got, out.String(), errb.String(), want)
		}
	}
}
