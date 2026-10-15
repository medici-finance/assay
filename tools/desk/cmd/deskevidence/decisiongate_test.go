package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisiongate_test.go — the decision-gate hold's network layer on an evidence landing
// (decisiongate.go; spec/lifecycle-v1.md §4.5).

const dgeLine = "sdlc/18 REFUSED — ruling: comment author is a bot"

func stubDecisionGateRuns(t *testing.T, before, after []string, seen *[]string) {
	t.Helper()
	old := statusgenDecisionGateFn
	oldBase := decisionGateReplacedBaseFn
	t.Cleanup(func() { statusgenDecisionGateFn = old; decisionGateReplacedBaseFn = oldBase })
	decisionGateReplacedBaseFn = func(string, string, []byte, bool, []boardEdit) (string, error) { return "BASE", nil }
	calls := 0
	statusgenDecisionGateFn = func(root, _ string) ([]string, error) {
		calls++
		if seen != nil {
			b, _ := os.ReadFile(filepath.Join(root, "docs/streams/x/README.md"))
			*seen = append(*seen, string(b))
		}
		if calls == 1 {
			return before, nil
		}
		return after, nil
	}
}

// TestDecisionGateDiffJudgesAgainstTheReplacedContent: both runs are judged
// against the base built from the content the write REPLACES (never the
// worktree's own HEAD), and the "before" run sees that replaced content staged
// over a worktree that already holds the edit, the "after" run the new content.
// A worktree carrying the edit (uncommitted or committed locally) must not
// cancel it out. The worktree file is restored afterwards.
func TestDecisionGateDiffJudgesAgainstTheReplacedContent(t *testing.T) {
	const rel = "docs/streams/x/README.md"
	root := rootWithFile(t, rel, "LOCAL-EDIT\n")
	oldRun, oldBase := statusgenDecisionGateFn, decisionGateReplacedBaseFn
	t.Cleanup(func() { statusgenDecisionGateFn = oldRun; decisionGateReplacedBaseFn = oldBase })
	var builtFrom []string
	decisionGateReplacedBaseFn = func(_, target string, replaced []byte, exists bool, _ []boardEdit) (string, error) {
		builtFrom = append(builtFrom, target+"="+string(replaced))
		return "SYNTHETIC", nil
	}
	var runs []string
	statusgenDecisionGateFn = func(r, base string) ([]string, error) {
		b, _ := os.ReadFile(filepath.Join(r, rel))
		runs = append(runs, base+"|"+string(b))
		return nil, nil
	}
	if _, err := decisionGateDiffAt(root, rel, []byte("REPLACED\n"), true, []byte("NEW\n"), nil); err != nil {
		t.Fatalf("decisionGateDiffAt: %v", err)
	}
	if len(builtFrom) != 1 || builtFrom[0] != rel+"=REPLACED\n" {
		t.Errorf("base built from %v, want the replaced content of %s", builtFrom, rel)
	}
	want := []string{"SYNTHETIC|REPLACED\n", "SYNTHETIC|NEW\n"}
	if len(runs) != 2 || runs[0] != want[0] || runs[1] != want[1] {
		t.Errorf("statusgen runs (base|staged) = %q, want %q", runs, want)
	}
	if b, _ := os.ReadFile(filepath.Join(root, rel)); string(b) != "LOCAL-EDIT\n" {
		t.Errorf("worktree file after the check = %q, want it restored", b)
	}
}

func wantUnverifiable(t *testing.T, err error) {
	t.Helper()
	var de *deskkit.DeskError
	if err == nil || !asDeskError(err, &de) || de.Code != deskkit.ExitUnverifiable {
		t.Fatalf("error = %v, want an Unverifiable DeskError (exit %d)", err, deskkit.ExitUnverifiable)
	}
}

// TestDecisionGateDiffIntroducedOnly: only the REFUSED lines the staged write adds
// come back; one already present before the write is not this landing's fault.
func TestDecisionGateDiffIntroducedOnly(t *testing.T) {
	root := rootWithFile(t, "docs/streams/x/README.md", "old\n")
	var seen []string
	pre := "other/02 REFUSED — no decision issue is recorded"
	stubDecisionGateRuns(t, []string{pre}, []string{pre, dgeLine}, &seen)
	got, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"), nil)
	if err != nil {
		t.Fatalf("decisionGateDiffAt: %v", err)
	}
	if len(got) != 1 || got[0] != dgeLine {
		t.Fatalf("introduced = %v, want [%q]", got, dgeLine)
	}
	if len(seen) != 2 || seen[0] != "old\n" || seen[1] != "new\n" {
		t.Fatalf("the gate ran against %q, want the tree before then after the staged write", seen)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "docs/streams/x/README.md")); string(b) != "old\n" {
		t.Fatalf("stage not reverted: %q", b)
	}
}

// TestDecisionGateDiffRemovesNewFile: a staged file that did not exist is removed.
func TestDecisionGateDiffRemovesNewFile(t *testing.T) {
	root := t.TempDir()
	stubDecisionGateRuns(t, nil, []string{dgeLine}, nil)
	got, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"), nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v; want one introduced line", got, err)
	}
	if _, serr := os.Stat(filepath.Join(root, "docs/streams/x/README.md")); !os.IsNotExist(serr) {
		t.Fatalf("staged new file left behind: %v", serr)
	}
}

// TestDecisionGateDiffCouldNotCheckPropagates: a could-not-check on either run is
// returned, never a pass.
func TestDecisionGateDiffCouldNotCheckPropagates(t *testing.T) {
	root := rootWithFile(t, "docs/streams/x/README.md", "old\n")
	old := statusgenDecisionGateFn
	oldBase := decisionGateReplacedBaseFn
	t.Cleanup(func() { statusgenDecisionGateFn = old; decisionGateReplacedBaseFn = oldBase })
	decisionGateReplacedBaseFn = func(string, string, []byte, bool, []boardEdit) (string, error) { return "BASE", nil }
	calls := 0
	statusgenDecisionGateFn = func(string, string) ([]string, error) {
		calls++
		if calls == 2 {
			return nil, deskkit.Unverifiable("boom", nil)
		}
		return nil, nil
	}
	_, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"), nil)
	wantUnverifiable(t, err)
	if b, _ := os.ReadFile(filepath.Join(root, "docs/streams/x/README.md")); string(b) != "old\n" {
		t.Fatalf("stage not reverted after could-not-check: %q", b)
	}
}

func TestStatusgenDecisionGateAtNotOnPathIsUnverifiable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := statusgenDecisionGateAt(t.TempDir(), "HEAD")
	wantUnverifiable(t, err)
}

// TestStatusgenDecisionGateAtArgsAndLines: the shell passes --root, --decision-gate
// and --decision-gate-base HEAD, and returns exactly the REFUSED lines on exit 1.
func TestStatusgenDecisionGateAtArgsAndLines(t *testing.T) {
	root := t.TempDir()
	fakeStatusgenOnPath(t, `if [ $# -ne 5 ] || [ "$1" != "--root" ] || [ "$2" != "`+root+`" ] || [ "$3" != "--decision-gate" ] || [ "$4" != "--decision-gate-base" ] || [ "$5" != "HEAD" ]; then
echo "bad args: $*"; exit 2
fi
echo "# decision-gate hold (lifecycle-v1 §4.5)"
echo "a/01 RULED — ruled by someone (https://example.invalid)"
echo "`+dgeLine+`"
exit 1`)
	got, err := statusgenDecisionGateAt(root, "HEAD")
	if err != nil {
		t.Fatalf("statusgenDecisionGateAt: %v", err)
	}
	if len(got) != 1 || got[0] != dgeLine {
		t.Fatalf("refused = %v, want [%q]", got, dgeLine)
	}
}

func TestStatusgenDecisionGateAtCleanExit(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "no gate: human brief moved, relabelled or dropped — clean"
exit 0`)
	got, err := statusgenDecisionGateAt(t.TempDir(), "HEAD")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want clean", got, err)
	}
}

// TestStatusgenDecisionGateAtOldBinaryIsUnverifiable: a statusgen that predates the
// hold rejects the flag; that is could-not-check naming the upgrade, never a pass.
func TestStatusgenDecisionGateAtOldBinaryIsUnverifiable(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "flag provided but not defined: -decision-gate" >&2
exit 2`)
	_, err := statusgenDecisionGateAt(t.TempDir(), "HEAD")
	wantUnverifiable(t, err)
	if !strings.Contains(err.Error(), "predates the decision-gate hold") {
		t.Fatalf("old-binary error does not name the upgrade: %v", err)
	}
}

// TestStatusgenDecisionGateAtNoRefusedLineIsUnverifiable: exit 1 with nothing refused
// (a crash, a forge error) is could-not-check.
func TestStatusgenDecisionGateAtNoRefusedLineIsUnverifiable(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "statusgen: something broke" >&2
exit 1`)
	_, err := statusgenDecisionGateAt(t.TempDir(), "HEAD")
	wantUnverifiable(t, err)
}

// TestStatusgenDecisionGateAtWholeChangeIsUnverifiable: statusgen failing closed on
// the change as a whole ("(board) REFUSED") is could-not-check: before and after
// would carry the same line and cancel out in the diff.
func TestStatusgenDecisionGateAtWholeChangeIsUnverifiable(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "(board) REFUSED — the base \"HEAD\" does not resolve to a commit"
exit 1`)
	_, err := statusgenDecisionGateAt(t.TempDir(), "HEAD")
	wantUnverifiable(t, err)
}

// TestDecisionGateLandingRefused: cmdEvidence refuses (exit 5) a landing whose
// staged write the gate refuses, before any write, carrying the line verbatim.
func TestDecisionGateLandingRefused(t *testing.T) {
	f, _ := setupFake(t)
	var gotRoot, gotTarget string
	decisionGateDiffFn = func(r, target string, _ []byte, _ bool, _ []byte, _ []boardEdit) ([]string, error) {
		gotRoot, gotTarget = r, target
		return []string{dgeLine}, nil
	}
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("refused landing still wrote %d time(s)", f.putCalls)
	}
	if gotRoot != root || gotTarget != evidencePath {
		t.Fatalf("gate ran on (%q, %q), want (%q, %q)", gotRoot, gotTarget, root, evidencePath)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, dgeLine) || !strings.Contains(d, "decision-gate hold") {
		t.Fatalf("audit detail = %q, want the refused line and the hold named", d)
	}
}

// TestDecisionGateLandingCouldNotCheck: a could-not-check from the gate is exit 6,
// no write.
func TestDecisionGateLandingCouldNotCheck(t *testing.T) {
	f, _ := setupFake(t)
	decisionGateDiffFn = func(string, string, []byte, bool, []byte, []boardEdit) ([]string, error) {
		return nil, deskkit.Unverifiable("statusgen is not on PATH", nil)
	}
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("could-not-check landing still wrote %d time(s)", f.putCalls)
	}
}

// realGateFixture builds this repository's statusgen onto PATH and a real git root
// holding a gate: human brief with no decision issue, its board cell at todo and
// committed. board(status) renders the stream README with that cell (wrapped in the
// generated-table markers when marked). git runs a git command in the root.
type realGate struct {
	root, readme string
	board        func(status string) string
	git          func(args ...string)
}

const realGateReadme = "docs/streams/sdlc/README.md"

func realGateFixture(t *testing.T, marked bool) realGate {
	t.Helper()
	if testing.Short() {
		t.Skip("-short: skips compiling statusgen")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim not exercised on windows")
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "statusgen"))
	if err != nil {
		t.Fatal(err)
	}
	if _, serr := os.Stat(filepath.Join(src, "go.mod")); serr != nil {
		t.Skip("statusgen source not beside this module")
	}
	bin := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "statusgen"), ".")
	build.Dir = src
	if out, berr := build.CombinedOutput(); berr != nil {
		t.Fatalf("build statusgen: %v\n%s", berr, out)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	const brief = "docs/streams/sdlc/brief-18.md"
	board := func(status string) string {
		table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n" +
			"| 18 | [brief-18](brief-18.md) | 1 | M | " + status + " | — | — |\n"
		if marked {
			table = tableMarkerBegin + "\n" + table + tableMarkerEnd + "\n"
		}
		return "---\nstream: sdlc\nstatus: active\npriority: P1\n---\n\n" + table + "\n"
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		if out, gerr := c.CombinedOutput(); gerr != nil {
			t.Fatalf("git %v: %v\n%s", args, gerr, out)
		}
	}
	for rel, content := range map[string]string{
		realGateReadme: board("todo"),
		brief:          "---\nbrief: sdlc/18\ntitle: Brief 18\nwave: 1\neffort: M\ngate: human\n---\n\n# Brief 18\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if merr := os.MkdirAll(filepath.Dir(p), 0o755); merr != nil {
			t.Fatal(merr)
		}
		if werr := os.WriteFile(p, []byte(content), 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	return realGate{root: root, readme: realGateReadme, board: board, git: git}
}

func (g realGate) writeReadme(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(g.root, filepath.FromSlash(g.readme)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (g realGate) readReadme(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(g.root, filepath.FromSlash(g.readme)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestDecisionGateDiffRealStatusgen builds this repository's statusgen and runs the
// production shell against a real git fixture: a gate: human brief with no decision
// issue, its board cell flipped todo → implemented by the staged write. The real
// tool's REFUSED line comes back as introduced; the flag names and line shape this
// file parses are proven against the binary, not only a fake script. Skipped under
// -short (it compiles statusgen).
func TestDecisionGateDiffRealStatusgen(t *testing.T) {
	g := realGateFixture(t, false)
	got, err := decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("implemented")), nil)
	if err != nil {
		t.Fatalf("decisionGateDiffAt against the real statusgen: %v", err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "sdlc/18"+decisionGateRefusedMark) {
		t.Fatalf("introduced = %v, want one sdlc/18 REFUSED line", got)
	}
	if b := g.readReadme(t); b != g.board("todo") {
		t.Fatalf("stage not reverted: %q", b)
	}
	// The same write with the cell left alone introduces nothing.
	got, err = decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("todo")), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("unchanged board: got %v, %v; want nothing introduced", got, err)
	}
}

// TestDecisionGateDiffWorktreeAlreadyHoldsTheEdit: the documented direct-write flow
// edits the tracked README in place and then lands it, so the landing worktree
// already holds the unruled move — uncommitted, or committed locally. The write is
// judged against the content it REPLACES, so both shapes are refused, each with its
// matched control (the same shape with the cell left alone) quiet, and the worktree
// comes back exactly as it was found.
func TestDecisionGateDiffWorktreeAlreadyHoldsTheEdit(t *testing.T) {
	for _, shape := range []string{"uncommitted", "committed"} {
		t.Run(shape, func(t *testing.T) {
			g := realGateFixture(t, false)
			g.writeReadme(t, g.board("implemented"))
			if shape == "committed" {
				g.git("commit", "-q", "-am", "local edit")
			}
			got, err := decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("implemented")), nil)
			if err != nil {
				t.Fatalf("decisionGateDiffAt: %v", err)
			}
			if len(got) != 1 || !strings.HasPrefix(got[0], "sdlc/18"+decisionGateRefusedMark) {
				t.Fatalf("introduced = %v, want one sdlc/18 REFUSED line (the worktree already holding the edit must not cancel it)", got)
			}
			if b := g.readReadme(t); b != g.board("implemented") {
				t.Fatalf("worktree not restored: %q", b)
			}
			// Control: the landing leaves the cell where the remote has it.
			got, err = decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("todo")), nil)
			if err != nil || len(got) != 0 {
				t.Fatalf("control: got %v, %v; want nothing introduced", got, err)
			}
			if b := g.readReadme(t); b != g.board("implemented") {
				t.Fatalf("worktree not restored after the control: %q", b)
			}
		})
	}
}

// TestDecisionGateLandingWholeVerbRefusesInPlaceEdit drives the whole verb in
// direct-write mode with the real statusgen and the real decisionGateDiffAt: the
// README row is edited in place in the landing worktree, then landed — with and
// without the generated-table markers (--row). The landing exits 5 and writes
// nothing; the matched control (the cell untouched) lands.
func TestDecisionGateLandingWholeVerbRefusesInPlaceEdit(t *testing.T) {
	for _, marked := range []bool{false, true} {
		name := "plain table"
		if marked {
			name = "marker table with --row"
		}
		t.Run(name, func(t *testing.T) {
			g := realGateFixture(t, marked)
			f, _ := setupFake(t)
			decisionGateDiffFn = decisionGateDiffAt
			f.setFile(g.readme, g.board("todo"))
			args := []string{"example-org/tracker", "main", "--evidence-file", g.readme, "--root", g.root}
			if marked {
				args = append(args, "--row", "18")
			}

			g.writeReadme(t, g.board("implemented"))
			if code := run(args); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (an unruled gate: human move edited in place must be refused)", code, deskkit.ExitRefused)
			}
			if f.putCalls != 0 {
				t.Fatalf("refused landing still wrote %d time(s)", f.putCalls)
			}
			if d := lastAudit(t).Detail; !strings.Contains(d, "sdlc/18"+decisionGateRefusedMark) {
				t.Fatalf("audit detail = %q, want the REFUSED line", d)
			}

			// Control: the in-place edit leaves the cell where the remote has it.
			g.writeReadme(t, g.board("todo")+"\nA note.\n")
			if code := run(args); code != deskkit.ExitOK {
				t.Fatalf("control exit = %d, want %d", code, deskkit.ExitOK)
			}
		})
	}
}

// freshHome is a real git root with one commit, so the branch-board read can resolve a
// home HEAD; it returns the root and that HEAD.
func freshHome(t *testing.T) (root, head string) {
	t.Helper()
	root = rootWithFile(t, "docs/streams/sdlc/README.md", "x\n")
	run := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("add", "-A")
	run("commit", "-q", "-m", "home")
	return root, run("rev-parse", "HEAD")
}

func freshFiles(names ...string) []deskkit.ChangedFile {
	var out []deskkit.ChangedFile
	for _, n := range names {
		out = append(out, deskkit.ChangedFile{Filename: n, Status: "modified"})
	}
	return out
}

// TestDecisionGateBranchBoard pins the read of the board the write lands on: every
// board file the target branch changed since the home's HEAD, other than the target, is
// read at the branch tip and returned for the judge; a register, the target and a
// file outside the board are not. Each uncertain read (no HEAD, no compare, a compare
// that cannot be shown complete, a changed file that cannot be read) is could-not-check.
func TestDecisionGateBranchBoard(t *testing.T) {
	const target = "docs/streams/sdlc/README.md"
	const brief = "docs/streams/sdlc/brief-18.md"
	root, head := freshHome(t)
	type want struct {
		path   string
		body   string
		delete bool
	}
	cases := []struct {
		name    string
		mutate  func(f *fakeForge)
		edits   []want
		wantErr string // "" = no error
	}{
		{"nothing changed on the branch", func(f *fakeForge) {}, nil, ""},
		{"only the target changed", func(f *fakeForge) { f.cmpFiles = freshFiles(target) }, nil, ""},
		{"a register changed", func(f *fakeForge) {
			f.cmpFiles = freshFiles("docs/streams/findings/2026-10-12-x.md", "docs/streams/intake/a.md", "docs/streams/verify-outcomes/sdlc/18.json")
		}, nil, ""},
		{"a file outside the board changed", func(f *fakeForge) { f.cmpFiles = freshFiles("tools/desk/main.go", "STATUS.md") }, nil, ""},
		{"a brief changed on the branch", func(f *fakeForge) {
			f.setFile(brief, "branch brief\n")
			f.cmpFiles = freshFiles(brief)
		}, []want{{path: brief, body: "branch brief\n"}}, ""},
		{"a brief changed beside the target", func(f *fakeForge) {
			f.setFile(brief, "branch brief\n")
			f.cmpFiles = freshFiles(target, brief)
		}, []want{{path: brief, body: "branch brief\n"}}, ""},
		{"another stream index changed", func(f *fakeForge) {
			f.setFile("docs/streams/other/README.md", "other\n")
			f.cmpFiles = freshFiles("docs/streams/other/README.md")
		}, []want{{path: "docs/streams/other/README.md", body: "other\n"}}, ""},
		{"an archived record changed", func(f *fakeForge) {
			f.setFile("docs/archive/sdlc/brief-18.md", "archived\n")
			f.cmpFiles = freshFiles("docs/archive/sdlc/brief-18.md")
		}, []want{{path: "docs/archive/sdlc/brief-18.md", body: "archived\n"}}, ""},
		{"a board file removed on the branch", func(f *fakeForge) {
			f.cmpFiles = []deskkit.ChangedFile{{Filename: brief, Status: "removed"}}
		}, []want{{path: brief, delete: true}}, ""},
		{"a board file renamed away", func(f *fakeForge) {
			f.setFile("docs/archive/sdlc/brief-18.md", "archived\n")
			f.cmpFiles = []deskkit.ChangedFile{{Filename: "docs/archive/sdlc/brief-18.md", PreviousFilename: brief, Status: "renamed"}}
		}, []want{{path: brief, delete: true}, {path: "docs/archive/sdlc/brief-18.md", body: "archived\n"}}, ""},
		{"a changed board file cannot be read at the tip", func(f *fakeForge) { f.cmpFiles = freshFiles(brief) }, nil, "cannot read"},
		{"the compare errors", func(f *fakeForge) { f.cmpErr = deskkit.Unverifiable("compare failed", nil) }, nil, "cannot compare"},
		{"the commit interval is incomplete", func(f *fakeForge) { f.cmpIncomplete = true }, nil, "cannot be shown complete"},
		{"the file listing may be truncated", func(f *fakeForge) {
			for i := 0; i < decisionGateCompareFileCap; i++ {
				f.cmpFiles = append(f.cmpFiles, deskkit.ChangedFile{Filename: "tools/x" + strings.Repeat("a", i%7) + ".go"})
			}
		}, nil, "cannot be shown complete"},
		// S-2266-7: the board reads a record only from a plain file, so a changed board
		// entry the forge serves as anything else is could-not-check, never staged.
		{"a board entry is a link to a file", func(f *fakeForge) {
			f.setFile(brief, "the link target's bytes\n")
			f.fileKinds = map[string]string{brief: deskkit.FileKindSymlink}
			f.cmpFiles = freshFiles(brief)
		}, nil, "not a plain file"},
		{"a board entry is a dangling link", func(f *fakeForge) {
			f.setFile(brief, "")
			f.fileKinds = map[string]string{brief: deskkit.FileKindSymlink}
			f.cmpFiles = freshFiles(brief)
		}, nil, "not a plain file"},
		{"a board entry is a submodule", func(f *fakeForge) {
			f.setFile(brief, "")
			f.fileKinds = map[string]string{brief: deskkit.FileKindSubmodule}
			f.cmpFiles = freshFiles(brief)
		}, nil, "not a plain file"},
		{"the forge does not type a board entry", func(f *fakeForge) {
			f.setFile(brief, "branch brief\n")
			f.fileKinds = map[string]string{brief: ""}
			f.cmpFiles = freshFiles(brief)
		}, nil, "not a plain file"},
		// The compare lists the branch's changes since the merge base: only a home HEAD
		// that is an ancestor of the tip makes that the whole difference.
		{"the home is behind and ahead of the branch", func(f *fakeForge) {
			f.compareStatus = map[string]string{"example-org/tracker:" + head + "...c0ffee1": "diverged"}
			f.cmpBehind = 2
		}, nil, "not an ancestor"},
		{"the branch is behind the home", func(f *fakeForge) {
			f.compareStatus = map[string]string{"example-org/tracker:" + head + "...c0ffee1": "behind"}
			f.cmpBehind = 1
		}, nil, "not an ancestor"},
		{"the compare reports no status", func(f *fakeForge) {
			f.compareStatus = map[string]string{"example-org/tracker:" + head + "...c0ffee1": ""}
		}, nil, "not an ancestor"},
		{"a board file copied, not renamed", func(f *fakeForge) {
			f.setFile("docs/archive/sdlc/brief-18.md", "archived\n")
			f.cmpFiles = []deskkit.ChangedFile{{Filename: "docs/archive/sdlc/brief-18.md", PreviousFilename: brief, Status: "copied"}}
		}, []want{{path: "docs/archive/sdlc/brief-18.md", body: "archived\n"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The tip resolves to c0ffee1: the compare and every read are made at that
			// commit, never at the moving branch name.
			f := &fakeForge{commitSHAs: map[string]string{"main": "c0ffee1"}}
			c.mutate(f)
			edits, err := decisionGateBranchBoard(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, root, "refs/heads/main", target)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got %v, want an error containing %q", err, c.wantErr)
				}
				if !deskkit.IsUnverifiable(err) {
					t.Fatalf("got %v, want could-not-check", err)
				}
			} else if err != nil {
				t.Fatalf("got %v, want no error", err)
			}
			if len(edits) != len(c.edits) {
				t.Fatalf("edits = %+v, want %+v", edits, c.edits)
			}
			for i, w := range c.edits {
				if edits[i].Path != w.path || edits[i].Delete != w.delete || string(edits[i].Content) != w.body {
					t.Fatalf("edit %d = %+v, want %+v", i, edits[i], w)
				}
			}
			if len(f.cmpCalls) != 1 || f.cmpCalls[0] != head+"...c0ffee1" {
				t.Fatalf("compare calls = %v, want the home HEAD against the branch tip resolved once", f.cmpCalls)
			}
			for _, r := range f.reads {
				if r.Ref != "c0ffee1" {
					t.Fatalf("read %s at %q, want the resolved tip c0ffee1 (the commit the compare was made at)", r.File, r.Ref)
				}
			}
		})
	}
	t.Run("the branch tip cannot be resolved", func(t *testing.T) {
		f := &fakeForge{commitErr: deskkit.Unverifiable("no such ref", nil)}
		_, err := decisionGateBranchBoard(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, root, "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "cannot resolve the tip") {
			t.Fatalf("got %v, want could-not-check naming the tip", err)
		}
		if len(f.cmpCalls) != 0 {
			t.Fatalf("compare ran without a resolved tip: %v", f.cmpCalls)
		}
	})
	t.Run("home HEAD does not resolve", func(t *testing.T) {
		_, err := decisionGateBranchBoard(&fakeForge{}, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, t.TempDir(), "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) {
			t.Fatalf("got %v, want could-not-check", err)
		}
	})
	t.Run("a forge that serves no compare", func(t *testing.T) {
		// The GitLab backend: CompareRefs is could-not-check, so a landing there stops
		// rather than judging a board it cannot show to match (documented in
		// docs/adopting-assay.md and the changelog).
		_, err := decisionGateBranchBoard(&deskkit.GitLabForge{}, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, root, "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) {
			t.Fatalf("got %v, want could-not-check", err)
		}
	})
}

// briefOf renders the fixture's brief with the given gate and an optional trailing note.
func briefOf(gate, note string) string {
	return "---\nbrief: sdlc/18\ntitle: Brief 18\nwave: 1\neffort: M\ngate: " + gate + "\n---\n\n# Brief 18\n" + note
}

// TestDecisionGateDiffAtJudgesBranchBoard drives the real judge: the home's brief is
// gate: model, so the move of its row introduces nothing; with the branch's board (the
// brief relabelled gate: human) applied, the same move is refused. The branch edit is
// reverted from the worktree afterwards.
func TestDecisionGateDiffAtJudgesBranchBoard(t *testing.T) {
	g := realGateFixture(t, false)
	briefPath := filepath.Join(g.root, "docs/streams/sdlc/brief-18.md")
	if err := os.WriteFile(briefPath, []byte(briefOf("model", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	g.git("commit", "-q", "-am", "attested commit: brief is gate: model")
	todo, impl := []byte(g.board("todo")), []byte(g.board("implemented"))

	got, err := decisionGateDiffAt(g.root, g.readme, todo, true, impl, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("home-only judge got %v, %v; want it to see nothing", got, err)
	}
	edits := []boardEdit{{Path: "docs/streams/sdlc/brief-18.md", Content: []byte(briefOf("human", ""))}}
	got, err = decisionGateDiffAt(g.root, g.readme, todo, true, impl, edits)
	if err != nil {
		t.Fatalf("judge with the branch's board: %v", err)
	}
	if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), "sdlc/18"+decisionGateRefusedMark) {
		t.Fatalf("judge with the branch's board got %v, want a refusal of sdlc/18", got)
	}
	if b, _ := os.ReadFile(briefPath); string(b) != briefOf("model", "") {
		t.Fatalf("the branch's brief was left in the worktree: %q", b)
	}
	// A board file the branch removed is removed for the judge too, then put back.
	if _, err := decisionGateDiffAt(g.root, g.readme, todo, true, impl, []boardEdit{{Path: "docs/streams/sdlc/brief-18.md", Delete: true}}); err != nil {
		t.Fatalf("judge with a removed brief: %v", err)
	}
	if b, _ := os.ReadFile(briefPath); string(b) != briefOf("model", "") {
		t.Fatalf("the removed brief was not put back: %q", b)
	}
}

// landingStaleHome sets up the whole-verb fixture: the home's attested commit holds the
// brief as gate: model and the README row at todo; the landing in the worktree moves the
// row to implemented. branchBrief is the brief as the target branch holds it now.
func landingStaleHome(t *testing.T, homeGate, branchBrief string) (g realGate, f *fakeForge, args []string) {
	t.Helper()
	g = realGateFixture(t, false)
	briefPath := filepath.Join(g.root, "docs/streams/sdlc/brief-18.md")
	if err := os.WriteFile(briefPath, []byte(briefOf(homeGate, "")), 0o644); err != nil {
		t.Fatal(err)
	}
	g.git("commit", "-q", "--allow-empty", "-am", "attested commit: brief is gate: "+homeGate)
	f, _ = setupFake(t)
	decisionGateDiffFn = decisionGateDiffAt
	decisionGateBranchBoardFn = decisionGateBranchBoard
	f.setFile(g.readme, g.board("todo"))
	f.setFile("docs/streams/sdlc/brief-18.md", branchBrief)
	args = []string{"example-org/tracker", "main", "--evidence-file", g.readme, "--root", g.root}
	g.writeReadme(t, g.board("implemented"))
	return g, f, args
}

// TestDecisionGateLandingWholeVerbJudgesTheBranchBoard drives the whole verb with the
// real statusgen and the real judge. The home's brief is gate: model (the attested
// commit, fixed at dispatch) while the target branch has since relabelled it gate:
// human: the landing is judged against the branch, so the unruled move is refused
// (exit 5) and nothing is written. Matched controls: the branch changed no board file;
// the branch changed the brief but kept it gate: model (the second landing of a
// verify-desk pair, whose first landing wrote the brief's Evidence rows). Both land,
// from the same unmoved home.
func TestDecisionGateLandingWholeVerbJudgesTheBranchBoard(t *testing.T) {
	t.Run("branch relabelled the brief gate: human", func(t *testing.T) {
		g, f, args := landingStaleHome(t, "model", briefOf("human", ""))
		f.cmpFiles = freshFiles("docs/streams/sdlc/brief-18.md")
		if code := run(args); code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (the branch relabelled the brief gate: human after the home's commit)", code, deskkit.ExitRefused)
		}
		if f.putCalls != 0 {
			t.Fatalf("refused landing still wrote %d time(s)", f.putCalls)
		}
		if d := lastAudit(t).Detail; !strings.Contains(d, "sdlc/18"+decisionGateRefusedMark) {
			t.Fatalf("audit detail = %q, want the REFUSED line", d)
		}
		if b, _ := os.ReadFile(filepath.Join(g.root, "docs/streams/sdlc/brief-18.md")); string(b) != briefOf("model", "") {
			t.Fatalf("the home's brief was moved: %q", b)
		}
	})
	t.Run("control: the branch changed no board file", func(t *testing.T) {
		_, f, args := landingStaleHome(t, "model", briefOf("model", ""))
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("control exit = %d, want %d", code, deskkit.ExitOK)
		}
		if f.putCalls != 1 {
			t.Fatalf("control wrote %d time(s), want 1", f.putCalls)
		}
	})
	t.Run("control: second landing of the pair, brief changed by the first", func(t *testing.T) {
		_, f, args := landingStaleHome(t, "model", briefOf("model", "\n## Evidence\n\n| row | result |\n|-----|--------|\n| 1 | pass |\n"))
		f.cmpFiles = freshFiles("docs/streams/sdlc/brief-18.md")
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("second-landing exit = %d, want %d (the home must not need to move)", code, deskkit.ExitOK)
		}
		if f.putCalls != 1 {
			t.Fatalf("second landing wrote %d time(s), want 1", f.putCalls)
		}
	})
	t.Run("control: the branch relabelled the brief away from human before this landing", func(t *testing.T) {
		// The home's commit still has gate: human; the branch has since (by a ruled
		// change that already landed) made it gate: model. Judged against the branch the
		// row move is a plain gate: model move.
		_, f, args := landingStaleHome(t, "human", briefOf("model", ""))
		f.cmpFiles = freshFiles("docs/streams/sdlc/brief-18.md")
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want %d (the branch's brief is gate: model)", code, deskkit.ExitOK)
		}
		if f.putCalls != 1 {
			t.Fatalf("landing wrote %d time(s), want 1", f.putCalls)
		}
	})
	t.Run("a changed board file that cannot be read stops the landing", func(t *testing.T) {
		_, f, args := landingStaleHome(t, "model", briefOf("human", ""))
		f.cmpFiles = freshFiles("docs/streams/sdlc/other-brief.md")
		if code := run(args); code != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %d, want %d (could-not-check)", code, deskkit.ExitUnverifiable)
		}
		if f.putCalls != 0 {
			t.Fatalf("stopped landing still wrote %d time(s)", f.putCalls)
		}
	})
}

// landLinkedBoard drives the whole verb with the real judge where the target branch
// holds the brief as a non-plain entry (kind), served with content: the board reads it
// as an unreadable record, so the landing must stop as could-not-check with no write.
func landLinkedBoard(t *testing.T, kind, content string) {
	t.Helper()
	_, f, args := landingStaleHome(t, "human", content)
	f.fileKinds = map[string]string{"docs/streams/sdlc/brief-18.md": kind}
	f.cmpFiles = freshFiles("docs/streams/sdlc/brief-18.md")
	if code := run(args); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (a %s board entry is not a record the board reads)", code, deskkit.ExitUnverifiable, kind)
	}
	if f.putCalls != 0 {
		t.Fatalf("stopped landing still wrote %d time(s)", f.putCalls)
	}
}

// regression: S-2266-7 — a brief the target branch holds as a symbolic link to a file is
// served by the forge as the target's bytes (here a gate: model brief). The board reads
// the link as an unreadable record and holds it; staging those bytes as a plain file
// judged a gate: model brief and let the unruled move land.
func TestBranchBoardLinkServed(t *testing.T) {
	landLinkedBoard(t, deskkit.FileKindSymlink, briefOf("model", ""))
}

// regression: S-2266-7 — a brief the target branch holds as a dangling link or a
// submodule is served with no content; staging that as an empty plain file judged a
// board the write does not land on.
func TestBranchBoardEmptyEntry(t *testing.T) {
	t.Run("dangling link", func(t *testing.T) { landLinkedBoard(t, deskkit.FileKindSymlink, "") })
	t.Run("submodule", func(t *testing.T) { landLinkedBoard(t, deskkit.FileKindSubmodule, "") })
}

// symlinkOrSkip makes link point at target, skipping where the platform will not.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
}

// regression: S-2266-7 — the stage writes the branch's board and the target into the
// landing home; a link at (or above) one of those paths carried the staged bytes
// through to whatever it names, outside the home. Each is could-not-check before
// anything is written, and the judge never runs.
func TestStageRefusesHomeLink(t *testing.T) {
	const readme = "docs/streams/x/README.md"
	const brief = "docs/streams/x/brief-1.md"
	cases := []struct {
		name  string
		setup func(t *testing.T, root, outside string)
		board []boardEdit
	}{
		{"board file is a link", func(t *testing.T, root, outside string) {
			symlinkOrSkip(t, filepath.Join(outside, "victim.md"), filepath.Join(root, brief))
		}, []boardEdit{{Path: brief, Content: []byte("STAGED\n")}}},
		{"board directory is a link", func(t *testing.T, root, outside string) {
			symlinkOrSkip(t, outside, filepath.Join(root, "docs/streams/y"))
		}, []boardEdit{{Path: "docs/streams/y/victim.md", Content: []byte("STAGED\n")}}},
		{"target is a link", func(t *testing.T, root, outside string) {
			if err := os.Remove(filepath.Join(root, readme)); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, filepath.Join(outside, "victim.md"), filepath.Join(root, readme))
		}, nil},
		{"board path is a directory", func(t *testing.T, root, _ string) {
			if err := os.MkdirAll(filepath.Join(root, brief), 0o755); err != nil {
				t.Fatal(err)
			}
		}, []boardEdit{{Path: brief, Content: []byte("STAGED\n")}}},
		{"board path leaves the home", func(t *testing.T, _, _ string) {}, []boardEdit{{Path: "../victim.md", Content: []byte("STAGED\n")}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := rootWithFile(t, readme, "LOCAL\n")
			outside := t.TempDir()
			victim := filepath.Join(outside, "victim.md")
			if err := os.WriteFile(victim, []byte("OUTSIDE\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			c.setup(t, root, outside)
			var seen []string
			stubDecisionGateRuns(t, nil, nil, nil)
			old := statusgenDecisionGateFn
			statusgenDecisionGateFn = func(r, b string) ([]string, error) {
				v, _ := os.ReadFile(victim)
				seen = append(seen, string(v))
				return old(r, b)
			}
			_, err := decisionGateDiffAt(root, readme, []byte("R\n"), true, []byte("N\n"), c.board)
			if err == nil || !deskkit.IsUnverifiable(err) {
				t.Fatalf("got %v, want could-not-check", err)
			}
			if len(seen) != 0 {
				t.Fatalf("the judge ran %d time(s) over a linked home (outside file read as %q)", len(seen), seen)
			}
			if v, _ := os.ReadFile(victim); string(v) != "OUTSIDE\n" {
				t.Fatalf("the file outside the home was changed: %q", v)
			}
		})
	}
}

// regression: S-2266-7 — the base replaces each staged path with a plain blob; a home
// HEAD that holds the path, or a directory above it, as a link is not the tree the board
// reads, so building the base is could-not-check. A plain path builds.
func TestHomeHeadModeRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	const readme = "docs/streams/x/README.md"
	build := func(t *testing.T, link, target, staged string) error {
		t.Helper()
		root := rootWithFile(t, readme, "x\n")
		if link != "" {
			symlinkOrSkip(t, target, filepath.Join(root, link))
		}
		git := func(args ...string) {
			c := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		git("init", "-q")
		git("add", "-A")
		git("commit", "-q", "-m", "home")
		_, err := replacedBaseCommit(root, readme, []byte("r\n"), true, []boardEdit{{Path: staged, Content: []byte("b\n")}})
		return err
	}
	t.Run("staged file is a link in HEAD", func(t *testing.T) {
		err := build(t, "docs/streams/x/brief-1.md", "README.md", "docs/streams/x/brief-1.md")
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "mode 120000") {
			t.Fatalf("got %v, want could-not-check naming the link", err)
		}
	})
	t.Run("directory above is a link in HEAD", func(t *testing.T) {
		err := build(t, "docs/streams/y", "x", "docs/streams/y/brief-1.md")
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "mode 120000") {
			t.Fatalf("got %v, want could-not-check naming the link", err)
		}
	})
	t.Run("control: plain and new paths build", func(t *testing.T) {
		if err := build(t, "", "", "docs/streams/z/brief-1.md"); err != nil {
			t.Fatalf("got %v, want a base", err)
		}
	})
}

// regression: S-2266-7 advisory — staging a board file or target under a directory the
// home lacks created it and left it behind, empty, after the revert.
func TestStageRemovesCreatedDirs(t *testing.T) {
	const readme = "docs/streams/x/README.md"
	root := rootWithFile(t, readme, "LOCAL\n")
	stubDecisionGateRuns(t, nil, nil, nil)
	board := []boardEdit{{Path: "docs/streams/new/deep/brief-1.md", Content: []byte("B\n")}}
	if _, err := decisionGateDiffAt(root, "docs/archive/x/README.md", nil, false, []byte("N\n"), board); err != nil {
		t.Fatalf("judge: %v", err)
	}
	for _, d := range []string{"docs/streams/new", "docs/archive"} {
		if _, err := os.Lstat(filepath.Join(root, d)); !os.IsNotExist(err) {
			t.Fatalf("%s was left in the home after the revert (lstat err %v)", d, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, readme)); string(b) != "LOCAL\n" {
		t.Fatalf("the home's own file changed: %q", b)
	}
}
