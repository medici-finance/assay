package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const vgBrief = "docs/streams/vg/brief-01-gate.md"

var (
	vgStatusgenOnce sync.Once
	vgStatusgenDir  string
)

// realVerdictCheck puts this tree's statusgen on PATH and restores the real
// evidenceverdict check behind evidenceVerdictCheckFn (setupFake stubs it).
func realVerdictCheck(t *testing.T) {
	t.Helper()
	vgStatusgenOnce.Do(func() {
		bin := buildStatusgen(t)
		dir, err := os.MkdirTemp("", "vg-statusgen-")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "statusgen"), raw, 0o755); err != nil {
			t.Fatal(err)
		}
		vgStatusgenDir = dir
	})
	t.Setenv("PATH", vgStatusgenDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	evidenceVerdictCheckFn = evidenceVerdictCheckAt
}

func vgBriefText(evidence string) string {
	return "---\nbrief: vg/01\ntitle: A verdict-gate fixture\nwave: 0\ndepends: []\nunblocks: []\neffort: S\n" +
		"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\n" +
		"schema: brief-v1\nauthored: 2026-08-13 by fixture\nsources: [\"fixture\"]\n---\n\n# Brief 01\n\n" +
		"## Verify\n| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |\n\n" +
		"## Evidence\n" + evidence + "\n## Review\nGate: model.\n"
}

const vgPriorRun = "2026-08-12 verifier run.\n\nVERIFY: PASS\n"

// vgLanding stages a board whose vg/01 row reads status, a remote brief whose
// Evidence holds one PASS, and a local brief appending a re-run that records
// word. It returns the run() exit code and the fake forge.
func vgLanding(t *testing.T, status, word string) (int, *fakeForge, string) {
	t.Helper()
	f, errBuf := setupFake(t)
	realVerdictCheck(t)
	after := vgBriefText(vgPriorRun + "\n2026-11-01 re-run.\n\nVERIFY: " + word + "\n")
	root := rootWithFile(t, vgBrief, after)
	readme := "---\nstream: vg\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# VG\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [t](./brief-01-gate.md) | 0 | S | " + status + " | 2026-08-13 human:alex | 2026-08-13 human:alex |\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "vg", "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	f.setFile(vgBrief, vgBriefText(vgPriorRun))
	code := run([]string{fixtureRepo, "main", "--root", root, "--evidence-file", vgBrief})
	return code, f, errBuf.String()
}

// regression: #5759
func TestVerifiedBlockedRefused(t *testing.T) {
	code, f, stderr := vgLanding(t, "verified", "BLOCKED")
	if code != deskkit.ExitRefused {
		t.Fatalf("verified + BLOCKED landing exit = %d, want %d (refused); stderr %q", code, deskkit.ExitRefused, stderr)
	}
	if len(f.writes) != 0 {
		t.Fatalf("a refused landing wrote %d file(s)", len(f.writes))
	}
	for _, want := range []string{vgBrief, "vg/01", `"verified"`, "VERIFY: BLOCKED", "--outcome-only"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refusal does not name %q: %q", want, stderr)
		}
	}
}

// regression: #5759
func TestDoneNeedsContextRefused(t *testing.T) {
	if code, _, stderr := vgLanding(t, "done", "NEEDS-CONTEXT"); code != deskkit.ExitRefused {
		t.Fatalf("done + NEEDS-CONTEXT landing exit = %d, want refused; stderr %q", code, stderr)
	}
}

// regression: #5759
func TestVerifiedFailLands(t *testing.T) {
	code, f, stderr := vgLanding(t, "verified", "FAIL")
	if code != deskkit.ExitOK || len(f.writes) != 1 {
		t.Fatalf("verified + FAIL landing exit = %d writes = %d, want it to land; stderr %q", code, len(f.writes), stderr)
	}
}

// regression: #5759
func TestVerifiedPassLands(t *testing.T) {
	code, f, stderr := vgLanding(t, "verified", "PASS")
	if code != deskkit.ExitOK || len(f.writes) != 1 {
		t.Fatalf("verified + PASS landing exit = %d writes = %d, want it to land; stderr %q", code, len(f.writes), stderr)
	}
}

// regression: #5759
func TestImplBlockedLands(t *testing.T) {
	code, f, stderr := vgLanding(t, "implemented", "BLOCKED")
	if code != deskkit.ExitOK || len(f.writes) != 1 {
		t.Fatalf("implemented + BLOCKED landing exit = %d writes = %d, want it to land unchanged; stderr %q", code, len(f.writes), stderr)
	}
}

func TestIsBriefFile(t *testing.T) {
	for p, want := range map[string]bool{
		vgBrief:                     true,
		"docs/streams/vg/README.md": false,
		"docs/streams/verify-outcomes/vg/01-a.json": false,
		"docs/other/brief-01.md":                    false,
		"docs/streams/vg/brief-01-gate.md.bak":      false,
	} {
		if got := isBriefFile(p); got != want {
			t.Errorf("isBriefFile(%q) = %v, want %v", p, got, want)
		}
	}
}
