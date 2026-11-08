package main

import (
	"os"
	"path/filepath"
	"sort"
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
	return vgLandingAt(t, status, "", vgPriorRun, vgPriorRun+"\n2026-11-01 re-run.\n\nVERIFY: "+word+"\n")
}

// vgLandingAt lands after's Evidence over a remote brief holding before's, on a
// home board reading status; a non-empty tip stubs the target branch tip's board
// to read vg/01 at tip.
func vgLandingAt(t *testing.T, status, tip, before, after string) (int, *fakeForge, string) {
	t.Helper()
	f, errBuf := setupFake(t)
	realVerdictCheck(t)
	root := rootWithFile(t, vgBrief, vgBriefText(after))
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "vg", "README.md"), []byte(vgReadme(status)), 0o644); err != nil {
		t.Fatal(err)
	}
	if tip != "" {
		tipBoard(t, tip)
	}
	f.setFile(vgBrief, vgBriefText(before))
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

// regression: #5759
//
// TestTipVerifiedBlockedRefused: the home (pinned at dispatch) reads implemented,
// the target branch tip reads verified. The landing lands on the tip, so its
// BLOCKED append is refused.
func TestTipVerifiedBlockedRefused(t *testing.T) {
	code, f, stderr := vgLandingAt(t, "implemented", "verified", vgPriorRun, vgPriorRun+"\n2026-11-01 re-run.\n\nVERIFY: BLOCKED\n")
	if code != deskkit.ExitRefused || len(f.writes) != 0 {
		t.Fatalf("home implemented, tip verified, BLOCKED: exit = %d writes = %d, want refused; stderr %q", code, len(f.writes), stderr)
	}
	if !strings.Contains(stderr, `"verified"`) {
		t.Fatalf("refusal does not name the tip's status: %q", stderr)
	}
}

// TestUnchangedVerdictLands: a landing whose latest verdict is the one the brief
// already records (same word, same count) records no new verdict and lands, at
// the deskevidence level as in statusgen.
func TestUnchangedVerdictLands(t *testing.T) {
	prior := vgPriorRun + "\n2026-11-01 re-run.\n\nVERIFY: BLOCKED\n"
	code, f, stderr := vgLandingAt(t, "verified", "", prior, prior+"\nA note on the runner image.\n")
	if code != deskkit.ExitOK || len(f.writes) != 1 {
		t.Fatalf("unchanged latest verdict: exit = %d writes = %d, want it to land; stderr %q", code, len(f.writes), stderr)
	}
}

// TestBriefPathBlockedRefused: the --brief-path route (an Evidence fragment
// merged into the brief) is gated on the merged bytes, like a whole-file landing.
func TestBriefPathBlockedRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	realVerdictCheck(t)
	root := rootWithFile(t, vgBrief, vgBriefText(vgPriorRun))
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "vg", "README.md"), []byte(vgReadme("verified")), 0o644); err != nil {
		t.Fatal(err)
	}
	f.setFile(vgBrief, vgBriefText(vgPriorRun))
	fragment := writeRepoFile(t, "row.md", "\n2026-11-01 re-run.\n\nVERIFY: BLOCKED\n")
	code := run([]string{fixtureRepo, "main", "--root", root, "--evidence-file", fragment, "--brief-path", vgBrief})
	if code != deskkit.ExitRefused || len(f.writes) != 0 {
		t.Fatalf("--brief-path BLOCKED fragment on a verified brief: exit = %d writes = %d, want refused; stderr %q",
			code, len(f.writes), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "--outcome-only") {
		t.Fatalf("refusal is not the verdict gate's: %q", errBuf.String())
	}
}

// regression: #5759
//
// TestVerdictCheckOnlyAtTip: the verdict question is asked in exactly one place,
// evidenceVerdictAtTip, which stages the target branch tip's board first. A second
// call of evidenceVerdictCheckFn anywhere in this package's non-test sources would
// read the home's board, which is behind the tip; this census turns it red.
func TestVerdictCheckOnlyAtTip(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var sites []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "evidenceVerdictCheckFn(") {
				sites = append(sites, name+":"+strings.TrimSpace(line))
			}
		}
	}
	want := "verdictgate.go:return evidenceVerdictCheckFn(root, briefPath, before, after, hasBefore, verdict)"
	if len(sites) != 1 || sites[0] != want {
		t.Fatalf("evidenceVerdictCheckFn call sites = %q, want exactly [%q] (ask through evidenceVerdictAtTip)", sites, want)
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
