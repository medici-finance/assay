package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFamilyWritesSixPNGs(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	if code := run([]string{"--org", "example-org", "--tier", "family", "--out", dir}, &errb); code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb.String())
	}
	pngs, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	if len(pngs) != 6 {
		t.Fatalf("want 6 PNGs, got %d", len(pngs))
	}
	svgs, _ := filepath.Glob(filepath.Join(dir, "*.svg"))
	if len(svgs) != 6 {
		t.Fatalf("want 6 SVGs, got %d", len(svgs))
	}
}

func TestRunTeamNames(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	if code := run([]string{"--org", "example-org", "--tier", "team", "--out", dir}, &errb); code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb.String())
	}
	for _, want := range []string{"example-org-read.png", "example-org-act.png"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing %s", want)
		}
	}
}

func TestRunDeterministic(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	var e1, e2 bytes.Buffer
	if code := run([]string{"--org", "example-org", "--tier", "family", "--out", d1}, &e1); code != 0 {
		t.Fatalf("run1 exit %d: %s", code, e1.String())
	}
	if code := run([]string{"--org", "example-org", "--tier", "family", "--out", d2}, &e2); code != 0 {
		t.Fatalf("run2 exit %d: %s", code, e2.String())
	}
	files, _ := filepath.Glob(filepath.Join(d1, "*"))
	for _, f := range files {
		a, _ := os.ReadFile(f)
		b, err := os.ReadFile(filepath.Join(d2, filepath.Base(f)))
		if err != nil {
			t.Fatalf("missing %s in second run", filepath.Base(f))
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between runs — not deterministic", filepath.Base(f))
		}
	}
}

func TestRunSizesMultiple(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	if code := run([]string{"--org", "example-org", "--tier", "team", "--out", dir, "--sizes", "200,512"}, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"example-org-read-200.png", "example-org-read-512.png", "example-org-act-200.png", "example-org-act-512.png"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing %s", want)
		}
	}
}

func TestRunRejectsOversizedSize(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	code := run([]string{"--org", "example-org", "--tier", "team", "--out", dir, "--sizes", "99999"}, &errb)
	if code != 2 {
		t.Errorf("want exit 2 for an oversized --sizes, got %d (stderr=%s)", code, errb.String())
	}
}

func TestRunRejectsBadOrg(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	// A path-traversal org must be refused before any file is written.
	code := run([]string{"--org", "../evil", "--tier", "team", "--out", dir}, &errb)
	if code == 0 {
		t.Errorf("want non-zero exit for a path-traversal --org, got 0")
	}
	if entries, _ := filepath.Glob(filepath.Join(dir, "*")); len(entries) != 0 {
		t.Errorf("no files should be written for a rejected org, found %v", entries)
	}
}

func TestRunMissingArgs(t *testing.T) {
	var errb bytes.Buffer
	if code := run([]string{"--tier", "team"}, &errb); code != 2 {
		t.Errorf("want exit 2 for missing --org/--out, got %d", code)
	}
}

func TestRunBadTierIsError(t *testing.T) {
	dir := t.TempDir()
	var errb bytes.Buffer
	code := run([]string{"--org", "example-org", "--tier", "bogus", "--out", dir}, &errb)
	if code != 1 {
		t.Errorf("want exit 1 for unknown tier, got %d (stderr=%s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "tier") {
		t.Errorf("stderr should mention the tier problem: %s", errb.String())
	}
}
