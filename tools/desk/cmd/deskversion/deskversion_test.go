package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The three states must map to three distinct exit codes, and each report must
// carry the words the Verify table greps for.

func TestDeskversion_Known(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--root", filepath.Join("..", "..", "internal", "deskkit", "testdata", "marker-known")}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("known exit = %d, want 0; stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "umbrella") {
		t.Errorf("known output missing 'umbrella':\n%s", out.String())
	}
}

func TestDeskversion_Inconsistent(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--root", filepath.Join("..", "..", "internal", "deskkit", "testdata", "marker-inconsistent")}, &out, &errb)
	if code == deskkit.ExitOK {
		t.Fatalf("inconsistent must be non-zero, got 0")
	}
	s := out.String()
	if !strings.Contains(s, "statusgen") || !strings.Contains(s, "umbrella") {
		t.Errorf("inconsistent output must name statusgen and umbrella:\n%s", s)
	}
}

func TestDeskversion_NoUmbrellaPin(t *testing.T) {
	tmp := t.TempDir()
	golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "deskkit", "testdata", "assay-versions-live.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".assay-versions"), golden, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	run([]string{"--root", tmp}, &out, &errb)
	if !strings.Contains(out.String(), "no umbrella pin") {
		t.Errorf("golden (no umbrella line) must report 'no umbrella pin':\n%s", out.String())
	}
}

func TestDeskversion_ThreeDistinctExitCodes(t *testing.T) {
	base := filepath.Join("..", "..", "internal", "deskkit", "testdata")
	var b1, b2, e bytes.Buffer
	known := run([]string{"--root", filepath.Join(base, "marker-known")}, &b1, &e)
	incon := run([]string{"--root", filepath.Join(base, "marker-inconsistent")}, &b2, &e)
	// could-not-determine: a root with no pin file at all.
	var b3 bytes.Buffer
	cnd := run([]string{"--root", t.TempDir()}, &b3, &e)
	if known != 0 {
		t.Errorf("known = %d, want 0", known)
	}
	if incon == 0 || cnd == 0 || incon == cnd {
		t.Errorf("need three distinct codes: known=%d inconsistent=%d could-not=%d", known, incon, cnd)
	}
}
