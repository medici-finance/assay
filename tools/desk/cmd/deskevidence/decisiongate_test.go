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
	decisionGateReplacedBaseFn = func(string, string, []byte, bool) (string, error) { return "BASE", nil }
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
	decisionGateReplacedBaseFn = func(_, target string, replaced []byte, exists bool) (string, error) {
		builtFrom = append(builtFrom, target+"="+string(replaced))
		return "SYNTHETIC", nil
	}
	var runs []string
	statusgenDecisionGateFn = func(r, base string) ([]string, error) {
		b, _ := os.ReadFile(filepath.Join(r, rel))
		runs = append(runs, base+"|"+string(b))
		return nil, nil
	}
	if _, err := decisionGateDiffAt(root, rel, []byte("REPLACED\n"), true, []byte("NEW\n")); err != nil {
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
	got, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"))
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
	got, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"))
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
	decisionGateReplacedBaseFn = func(string, string, []byte, bool) (string, error) { return "BASE", nil }
	calls := 0
	statusgenDecisionGateFn = func(string, string) ([]string, error) {
		calls++
		if calls == 2 {
			return nil, deskkit.Unverifiable("boom", nil)
		}
		return nil, nil
	}
	_, err := decisionGateDiffAt(root, "docs/streams/x/README.md", []byte("old\n"), true, []byte("new\n"))
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
	decisionGateDiffFn = func(r, target string, _ []byte, _ bool, _ []byte) ([]string, error) {
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
	decisionGateDiffFn = func(string, string, []byte, bool, []byte) ([]string, error) {
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
	got, err := decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("implemented")))
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
	got, err = decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("todo")))
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
			got, err := decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("implemented")))
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
			got, err = decisionGateDiffAt(g.root, g.readme, []byte(g.board("todo")), true, []byte(g.board("todo")))
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
