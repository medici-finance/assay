package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vcFixture lays a one-brief stream on disk — README register row (Status +
// Verified cell) plus the brief file (a two-row Verify table and whatever
// Evidence the caller supplies) — and returns the root, so runVerifyclosure can
// read it through loadStreams exactly as production does. The Verify commands are
// `true`/`false`, matching witnessgate_test.go's witnessRow* constants so those
// passing/failing witnesses drop straight into Evidence.
func vcFixture(t *testing.T, status, verifiedCell, evidence string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "vc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := `---
brief: vc/01
title: A closure fixture
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture"]
---

# Brief 01

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`true`" + ` | exit 0 |
| 2 | ` + "`false`" + ` | exit 1 |

## Evidence
` + evidence + `

## Review
Gate: model.
`
	if err := os.WriteFile(filepath.Join(dir, "brief-01-closure.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: vc\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# VC\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [t](./brief-01-closure.md) | 0 | S | " + status + " | " + verifiedCell + " | " + verifiedCell + " |\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// captureVerifyclosure runs the sub-command with stdout/stderr redirected to temp
// files and returns the exit code and both streams.
func captureVerifyclosure(t *testing.T, args []string) (code int, out, errOut string) {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out")
	errPath := filepath.Join(t.TempDir(), "err")
	of, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	ef, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	code = runVerifyclosure(args, of, ef)
	of.Sync()
	ef.Sync()
	ob, _ := os.ReadFile(outPath)
	eb, _ := os.ReadFile(errPath)
	return code, string(ob), string(eb)
}

const stamped = "2026-08-13 human:alex"

// TestVerifyclosureAcceptsAFullClosure: Status verified, a dated Verified stamp,
// and a passing execution witness for every Verify row → exit 0.
func TestVerifyclosureAcceptsAFullClosure(t *testing.T) {
	root := vcFixture(t, "verified", stamped, witnessTableFor(witnessRowOnePass, witnessRowTwoPass))
	code, out, errOut := captureVerifyclosure(t, []string{"--root", root, "--brief", "vc/01"})
	if code != verifyclosureExitAccepted {
		t.Fatalf("exit = %d, want %d (accepted); out=%q err=%q", code, verifyclosureExitAccepted, out, errOut)
	}
	if !strings.Contains(out, "accepted") {
		t.Fatalf("stdout = %q, want it to say accepted", out)
	}
}

// TestVerifyclosureRefusesStuckAtImplemented is the stuck-flip fail-first case
// (#1309): the Verify rows are fully witnessed, but the board still calls the
// brief `implemented` (no Verified stamp). A `verified` sidecar row here is the
// exact mismatch review refuses — verifyclosure must say NOT accepted (exit 1).
func TestVerifyclosureRefusesStuckAtImplemented(t *testing.T) {
	root := vcFixture(t, "implemented", "—", witnessTableFor(witnessRowOnePass, witnessRowTwoPass))
	code, out, errOut := captureVerifyclosure(t, []string{"--root", root, "--brief", "vc/01"})
	if code != verifyclosureExitNotAccepted {
		t.Fatalf("exit = %d, want %d (not accepted); out=%q err=%q", code, verifyclosureExitNotAccepted, out, errOut)
	}
	if !strings.Contains(out, "NOT accepted") || !strings.Contains(out, "implemented") {
		t.Fatalf("stdout = %q, want it to name the implemented stuck-flip shape", out)
	}
}

// TestVerifyclosureRefusesVerifiedWithoutWitness: the board IS flipped to verified
// with a stamp, but the Evidence carries NO execution witness for the Verify rows
// (filled with prose, not a verifyrun witness). That is the other half of the bug
// — a verified sidecar row without a lint-valid witness — so exit 1.
func TestVerifyclosureRefusesVerifiedWithoutWitness(t *testing.T) {
	prose := "| # | Command | Result | Output | Date | Runner |\n" +
		"|---|---------|--------|--------|------|--------|\n" +
		"| 1 | ran it | looked fine | — | 2026-08-13 | human:alex |\n"
	root := vcFixture(t, "verified", stamped, prose)
	code, out, errOut := captureVerifyclosure(t, []string{"--root", root, "--brief", "vc/01"})
	if code != verifyclosureExitNotAccepted {
		t.Fatalf("exit = %d, want %d (not accepted); out=%q err=%q", code, verifyclosureExitNotAccepted, out, errOut)
	}
	if !strings.Contains(out, "witness") {
		t.Fatalf("stdout = %q, want it to name the missing witness", out)
	}
}

// TestVerifyclosureCouldNotCheckUnknownBrief: a brief key that is on no board is
// could-not-check (exit 2) — never silently accepted, never a refusal verdict on
// a brief that could not be evaluated.
func TestVerifyclosureCouldNotCheckUnknownBrief(t *testing.T) {
	root := vcFixture(t, "verified", stamped, witnessTableFor(witnessRowOnePass, witnessRowTwoPass))
	code, _, errOut := captureVerifyclosure(t, []string{"--root", root, "--brief", "vc/99"})
	if code != verifyclosureExitCouldNotCheck {
		t.Fatalf("unknown brief exit = %d, want %d (could-not-check); err=%q", code, verifyclosureExitCouldNotCheck, errOut)
	}
	if !strings.Contains(errOut, "could-not-check") {
		t.Fatalf("stderr = %q, want a could-not-check diagnostic", errOut)
	}
}

// TestVerifyclosureUsageErrors: a missing/malformed --brief is could-not-check
// (exit 2), never a verdict.
func TestVerifyclosureUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--root", "."},         // no --brief
		{"--brief", "no-slash"}, // malformed key
		{"--brief", "vc/"},      // empty num
	} {
		if code, _, _ := captureVerifyclosure(t, args); code != verifyclosureExitCouldNotCheck {
			t.Fatalf("args %v exit = %d, want %d", args, code, verifyclosureExitCouldNotCheck)
		}
	}
}
