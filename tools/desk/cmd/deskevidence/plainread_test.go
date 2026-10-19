package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plainread_test.go — S-2266-8: a landing takes the branch's copy of a file as a record
// only when the forge shows it to be a plain file's own bytes.

// unprovenGate is the whole-verb fixture with the real statusgen, the real judge, the real
// branch-board read and the real lint: the root holds the gate: human brief-18 and its
// board row at todo, committed.
func unprovenGate(t *testing.T) (realGate, *fakeForge) {
	t.Helper()
	g := realGateFixture(t, false)
	f, _ := setupFake(t)
	decisionGateDiffFn = decisionGateDiffAt
	decisionGateBranchBoardFn = decisionGateBranchBoard
	lintDiffFn = lintDiffAt
	return g, f
}

const unprovenBrief = "docs/streams/sdlc/brief-18.md"

// regression: S-2266-8 — the Contents API serves a file over its size limit with its
// content left out, which the backend reports as unproven with an empty body. A
// --brief-path landing merged its Evidence into that empty body, so the write replaced
// the gate: human brief with an Evidence section and no frontmatter; the judge compared
// that against an empty prior and saw nothing to hold. The landing must stop as
// could-not-check with no write. Control: the same brief served in full keeps its
// frontmatter (gate: human) and lands.
func TestLandingRefusesUnprovenBriefBody(t *testing.T) {
	fragment := filepath.Join(t.TempDir(), "row.md")
	if err := os.WriteFile(fragment, []byte("| 1 | pass | the row ran |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Run("served without its content", func(t *testing.T) {
		g, f := unprovenGate(t)
		f.setFile(unprovenBrief, "")
		f.fileKinds = map[string]string{unprovenBrief: deskkit.FileKindUnproven}
		code := run([]string{"example-org/tracker", "main", "--brief-path", unprovenBrief, "--evidence-file", fragment, "--root", g.root})
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %d, want %d (an unproven brief body is not the branch's copy); wrote %q", code, deskkit.ExitUnverifiable, f.putContent)
		}
		if f.putCalls != 0 {
			t.Fatalf("stopped landing still wrote %d time(s): %q", f.putCalls, f.putContent)
		}
	})
	t.Run("control: the same brief served in full", func(t *testing.T) {
		g, f := unprovenGate(t)
		f.setFile(unprovenBrief, briefOf("human", ""))
		code := run([]string{"example-org/tracker", "main", "--brief-path", unprovenBrief, "--evidence-file", fragment, "--root", g.root})
		if code != deskkit.ExitOK {
			t.Fatalf("control exit = %d, want %d", code, deskkit.ExitOK)
		}
		if f.putCalls != 1 || !strings.HasPrefix(f.putContent, briefOf("human", "")) || !strings.Contains(f.putContent, "| 1 | pass | the row ran |") {
			t.Fatalf("control wrote %d time(s), want the brief with its frontmatter kept and the row appended:\n%s", f.putCalls, f.putContent)
		}
	})
}

// regression: S-2266-8 — a whole-file README landing over a board the forge serves with
// its content left out: the empty prior made the judge and the shrink guards compare
// against nothing, so a write that drops the gate: human brief's row landed. The landing
// must stop as could-not-check with no write. Control: the same README served in full —
// the drop of the row is refused (exit 5), nothing written.
func TestLandingRefusesUnprovenBoardBody(t *testing.T) {
	dropped := boardOfNoRows()
	t.Run("served without its content", func(t *testing.T) {
		g, f := unprovenGate(t)
		f.setFile(g.readme, "")
		f.fileKinds = map[string]string{g.readme: deskkit.FileKindUnproven}
		g.writeReadme(t, dropped)
		code := run([]string{"example-org/tracker", "main", "--evidence-file", g.readme, "--root", g.root})
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %d, want %d (an unproven board body is not the branch's copy); wrote %q", code, deskkit.ExitUnverifiable, f.putContent)
		}
		if f.putCalls != 0 {
			t.Fatalf("stopped landing still wrote %d time(s)", f.putCalls)
		}
	})
	t.Run("control: the same README served in full", func(t *testing.T) {
		g, f := unprovenGate(t)
		f.setFile(g.readme, g.board("todo"))
		g.writeReadme(t, dropped)
		code := run([]string{"example-org/tracker", "main", "--evidence-file", g.readme, "--root", g.root})
		if code != deskkit.ExitRefused {
			t.Fatalf("control exit = %d, want %d (dropping a gate: human row is refused)", code, deskkit.ExitRefused)
		}
		if f.putCalls != 0 {
			t.Fatalf("refused control still wrote %d time(s)", f.putCalls)
		}
	})
}

// boardOfNoRows is the fixture's stream README with brief 18's row dropped.
func boardOfNoRows() string {
	return "---\nstream: sdlc\nstatus: active\npriority: P1\n---\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n\n"
}

// TestPlainFileRead: only a plain file the forge proved is a record; the outcome-record
// lane alone also admits a read the forge did not type (the GitLab backend types none).
func TestPlainFileRead(t *testing.T) {
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	for _, c := range []struct {
		kind      string
		untypedOK bool
		ok        bool
	}{
		{deskkit.FileKindRegular, false, true},
		{deskkit.FileKindRegular, true, true},
		{"", true, true},
		{"", false, false},
		{deskkit.FileKindUnproven, true, false},
		{deskkit.FileKindUnproven, false, false},
		{deskkit.FileKindSymlink, true, false},
		{deskkit.FileKindSubmodule, true, false},
	} {
		err := plainFileRead(&deskkit.FileContent{Exists: true, Kind: c.kind}, fr, "f.md", "main", "the test", c.untypedOK)
		if c.ok && err != nil {
			t.Errorf("kind %q untypedOK=%v: %v, want nil", c.kind, c.untypedOK, err)
		}
		if !c.ok && (err == nil || !deskkit.IsUnverifiable(err)) {
			t.Errorf("kind %q untypedOK=%v: %v, want could-not-check", c.kind, c.untypedOK, err)
		}
	}
	if err := plainFileRead(nil, fr, "f.md", "main", "the test", true); err == nil || !deskkit.IsUnverifiable(err) {
		t.Errorf("nil read: %v, want could-not-check", err)
	}
}

// regression: S-2266-8 — the row-scope write-time re-fetch reads the table it rebases the
// named rows onto; a README served with its content left out is no table to rebase onto,
// and must stop the write as could-not-check rather than be judged as a changed table.
func TestRowScopeWriteTimeCheckRefusesUnprovenFetch(t *testing.T) {
	target := "docs/streams/example-stream/README.md"
	commit := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "verified"}, "\n")
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	for _, kind := range []string{deskkit.FileKindUnproven, deskkit.FileKindSymlink, ""} {
		f := &fakeForge{}
		f.setFile(target, commit)
		f.fileKinds = map[string]string{target: kind}
		if _, err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(commit)); !deskkit.IsUnverifiable(err) {
			t.Fatalf("kind %q: err = %v, want could-not-check", kind, err)
		}
	}
	f := &fakeForge{}
	f.setFile(target, commit)
	if _, err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(commit)); err != nil {
		t.Fatalf("control: a plain file's own bytes: %v", err)
	}
}
