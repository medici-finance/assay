package main

// Issue #2684 — a Verify Command cell that MENTIONS an inline heading
// (`## Coverage`) lifted that heading as the command. The shell reads a word
// starting `#` as a comment: it exits 0 with no output, and verifyrun recorded
// a pass witness (the empty-output fingerprint e3b0c44298fc) for a check that
// never looked at the review evidence the row requires.
//
// The class is "the lifted command is nothing but a shell comment". These tests
// pin it at every site that consumes the lift — the row lint, the witness a run
// writes, the audit of an old witness, the check:ci re-execution lane — and keep
// the legitimate contracts (a command with a trailing comment, a `cmd:` marker)
// unchanged. They run in an offline fixture: no brief on the real tree is
// executed.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// emptyOutputFingerprint is the 12-hex sha256 prefix of empty combined output —
// what the false pass in #2684 recorded.
const emptyOutputFingerprint = "e3b0c44298fc"

// commentOnlyCells are the Command cells whose lifted command is a shell
// comment: the reported inline-heading shape plus its siblings.
var commentOnlyCells = []struct{ name, cell string }{
	{"inline heading in prose (the #2684 cell)", "Inspect the `## Coverage` section of the review and confirm every angle is addressed"},
	{"heading first, real command later", "Read `## Coverage`, then run `go test ./x -count=1`"},
	{"single-token comment in prose", "Check the `#coverage` note in the review"},
	{"the whole cell is one comment span", "`## Coverage`"},
	{"bare cell, no code span", "# read the review"},
	{"indented comment span", "Read `  # Coverage` first"},
}

// regression: #2684
func TestCommentLedRowIsFlaggedProseLed(t *testing.T) {
	for _, c := range commentOnlyCells {
		t.Run(c.name, func(t *testing.T) {
			span, why := proseLedCommandWhy(c.cell)
			if why == "" || !strings.Contains(why, "shell comment") {
				t.Fatalf("proseLedCommandWhy(%q) = %q,%q; want a shell-comment reason", c.cell, span, why)
			}
			if fs := rowFindings(c.cell, "exit 0"); !hasRule(fs, proseLedRule) {
				t.Errorf("want a %s NOTICE for %q, got %+v", proseLedRule, c.cell, fs)
			}
		})
	}
}

// TestCommentLedLintSparesRealCommands — the contracts the fix must not move:
// a command with a trailing comment, a `#` inside a command, an explicit marker
// beside a heading mention, and a one-span command cell.
func TestCommentLedLintSparesRealCommands(t *testing.T) {
	for _, c := range []struct{ name, cell string }{
		{"trailing comment", "`go test ./x -count=1 # the real check`"},
		{"hash inside a quoted argument", "`grep -c '#' docs/x.md`"},
		{"hash mid-word", "`echo a#b`"},
		{"marker beside a heading mention", "Inspect `## Coverage`, then run `cmd: go test ./x -count=1`"},
		{"one command span", "`go test ./... -count=1`"},
		{"command then a heading mention", "`go test ./x -count=1` and read `## Coverage`"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if span, why := proseLedCommandWhy(c.cell); why != "" {
				t.Errorf("proseLedCommandWhy(%q) = %q,%q; a real command must not be flagged", c.cell, span, why)
			}
		})
	}
}

// TestCommentLedRowNotExecuted — the witness a run writes for the #2684 cell, and
// for a marked comment (review F2), is could-not-run (exit -1, no output
// fingerprint, the rule tag in the note), never the empty-output pass. The same cell with a marked command runs it and passes
// (the positive control: the executor still executes a legitimate row).
// regression: #2684
func TestCommentLedRowNotExecuted(t *testing.T) {
	rows := briefVerifyRows(verifyTable(
		"| 1 | Inspect the `## Coverage` section of the review | exit 0 |",
		"| 2 | Inspect `## Coverage`, then run `cmd: test -d .` | exit 0 |",
		"| 3 | `## Coverage` | exit 0 |",
		"| 4 | `cmd: # placeholder` | exit 0 |",
	))
	if len(rows) != 4 {
		t.Fatalf("want 4 rows, got %d", len(rows))
	}
	ws := runWitnesses(t.TempDir(), rows, "human:tester", "", "0000", "2026-10-15", 30*time.Second, false)
	if len(ws) != 4 {
		t.Fatalf("want 4 witnesses, got %d", len(ws))
	}
	if !strings.Contains(ws[3].Note, "Replace the cmd:-marked comment") || !strings.HasPrefix(rows[3].ProseLed, "marked command ") {
		t.Errorf("row 4: a marked comment's note must name the replace-it remedy, got prose-led=%q note=%q", rows[3].ProseLed, ws[3].Note)
	}
	for _, i := range []int{0, 2, 3} {
		w := ws[i]
		if w.State != stateCouldNotRun || w.Exit != -1 || w.OutHash == emptyOutputFingerprint ||
			!strings.Contains(w.Note, proseLedRule) || !strings.Contains(w.Note, "shell comment") {
			t.Errorf("row %d: a comment-only row must be could-not-run, unexecuted, with a %s shell-comment note; got state=%s exit=%d hash=%q note=%q",
				i+1, proseLedRule, w.State, w.Exit, w.OutHash, w.Note)
		}
	}
	if ws[1].State != statePass || ws[1].Exit != 0 {
		t.Errorf("the marked row must run its marked command and pass; got state=%s exit=%d note=%q", ws[1].State, ws[1].Exit, ws[1].Note)
	}
}

// TestCommentLedOldPassWitnessIsNotPass — --check must not audit the pass an
// older binary recorded for the #2684 cell (empty-output fingerprint) as pass;
// a fail witness is left as recorded.
// regression: #2684
func TestCommentLedOldPassWitnessIsNotPass(t *testing.T) {
	verify := verifyTable("| 1 | Inspect the `## Coverage` section of the review | exit 0 |")
	rows := briefVerifyRows(verify)
	if len(rows) != 1 || rows[0].ProseLed == "" {
		t.Fatalf("fixture drift: the row must be flagged prose-led, got %+v", rows)
	}
	for _, c := range []struct{ state, want string }{{statePass, stateCouldNotRun}, {stateFail, stateFail}} {
		w := witness{ID: "1", Command: rows[0].Command, State: c.state, Exit: 0, OutHash: emptyOutputFingerprint,
			Date: "2026-10-15", Runner: "human:alex", Tree: "0123456789ab"}
		fs := checkWitnesses(verify, witnessHeader+"\n"+w.row()+"\n")
		if len(fs) != 1 || fs[0].State != c.want {
			t.Errorf("comment-only row with a %s witness must check %s, got %+v", c.state, c.want, fs)
		}
	}
}

// TestCommentLedMarkedIsRefused — a MARKED command that is only a comment is
// refused like an unmarked one (#2684 review F2): the marker names the command
// to run and a comment runs nothing. statusgen agrees with the tools/desk
// executors, which refuse it too. The lint raises prose-led-command with the
// replace-it remedy (the row is already marked), not cmd-marker-vacuous, whose
// "the row passes" no longer holds; a marked command with a trailing comment
// is a real command and is left alone.
// regression: #2684
func TestCommentLedMarkedIsRefused(t *testing.T) {
	for _, cell := range []string{
		"`cmd: ## Coverage`",
		"Read `x.md`: `cmd: # placeholder`",
		"`cmd: #coverage`",
		"`cmd: # don't forget the quote`",
	} {
		span, why := proseLedCommandWhy(cell)
		if !strings.Contains(why, "`cmd:`-marked shell comment") {
			t.Errorf("proseLedCommandWhy(%q) = %q,%q; want the marked shell-comment reason", cell, span, why)
		}
		fs := rowFindings(cell, "exit 0")
		if !hasRule(fs, proseLedRule) || hasRule(fs, "cmd-marker-vacuous") {
			t.Errorf("%q must raise %s and not cmd-marker-vacuous, got %+v", cell, proseLedRule, fs)
		}
		for _, f := range fs {
			if f.rule == proseLedRule && (!strings.Contains(f.msg, "Replace the cmd:-marked comment") || strings.Contains(f.msg, "Mark the real command")) {
				t.Errorf("%q: the NOTICE must name the replace-it remedy, got %q", cell, f.msg)
			}
		}
	}
	if fs := rowFindings("`cmd: go test ./x -count=1 # the real check`", "exit 0"); hasRule(fs, "cmd-marker-vacuous") || hasRule(fs, proseLedRule) {
		t.Errorf("a command with a trailing comment must not be flagged, got %+v", fs)
	}
}

// TestTranscribeVerdictRefusesCommentOnlyCheckCI — the check:ci re-execution
// lane refuses the #2684 cell under clause-6 without reaching the runner, which
// would otherwise re-run the comment and report a pass.
// regression: #2684
func TestTranscribeVerdictRefusesCommentOnlyCheckCI(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)
	abs := filepath.Join(root, rel)
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	nb := strings.Replace(string(b), "| 1 | check:ci | true | exit 0 |", "| 1 | check:ci | Inspect the `## Coverage` section of the review | exit 0 |", 1)
	if nb == string(b) {
		t.Fatal("fixture drift: the check:ci row was not found to rewrite")
	}
	if err := os.WriteFile(abs, []byte(nb), 0o644); err != nil {
		t.Fatal(err)
	}
	body := signVerdictBody(t, key, okPayload(okEntry(rel, "| 1 | check:ci | true | 0 | PASS | 2026-10-15 | verifier |")))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 612}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{verdictTestRepo + "#612": {Author: verifierIdent, Body: body}})
	var got []string
	rec := func(_ string, command string) checkCIResult {
		got = append(got, command)
		return checkCIResult{Passed: true, Reason: "exit 0"}
	}
	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, rec, noHealthHold, blessR6Resolver)
	})
	if len(got) != 0 {
		t.Errorf("the comment-only row must never reach the check:ci runner, got %q\n%s", got, out.log)
	}
	if !strings.Contains(out.log, "REFUSE "+verdictTestRepo+"#612 — clause-6 (check:ci)") || !strings.Contains(out.log, "shell comment") {
		t.Errorf("a comment-only check:ci row must be REFUSED under clause-6 as a shell comment; log:\n%s", out.log)
	}
	if strings.Contains(out.log, "CONSUME") {
		t.Errorf("a comment-only check:ci verdict must not be consumed:\n%s", out.log)
	}
}

// TestShellCommentSharedVectors — shellCommentOnly satisfies the table the
// tools/desk executors' verifycmd.CommentOnly is held to as well.
// regression: #2684
func TestShellCommentSharedVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "shell-comment-vectors.json"))
	if err != nil {
		t.Fatalf("could-not-check: the shared comment vector table is unreadable: %v", err)
	}
	var doc struct {
		Vectors []struct {
			Name        string `json:"name"`
			Cmd         string `json:"cmd"`
			CommentOnly bool   `json:"commentOnly"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) < 8 {
		t.Fatalf("positive control: only %d vectors loaded", len(doc.Vectors))
	}
	for _, v := range doc.Vectors {
		if got := shellCommentOnly(v.Cmd); got != v.CommentOnly {
			t.Errorf("%s: shellCommentOnly(%q) = %v, want %v", v.Name, v.Cmd, got, v.CommentOnly)
		}
	}
}

// commentCellVector is one whole Command cell from the shared table's "cells"
// section (#2684 review S1, F2; #3147), which tools/desk's verifycmd is held to too.
type commentCellVector struct {
	Name      string `json:"name"`
	Cell      string `json:"cell"`
	FirstSpan string `json:"firstSpan"`
	Marked    bool   `json:"marked"`
	Lift      string `json:"lift"`
	Statusgen string `json:"statusgen"`
	Desk      string `json:"desk"`
	Passes    bool   `json:"passes"`
}

// TestCommentCellSharedVectors — statusgen's cell-to-verdict mapping, pinned at
// the CELL level so the two modules cannot apply one rule to different strings:
// the first-span lift tools/desk ports, the marker, the lifted command both
// modules run, the lint's verdict, and the witness a real run writes (a refused
// cell is could-not-run and never executed; a runs cell executes and keeps its
// own exit status). The table's own invariant is checked too: statusgen and the
// tools/desk executors reach the same verdict on every cell (#3147).
// regression: #2684, #3147
func TestCommentCellSharedVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "shell-comment-vectors.json"))
	if err != nil {
		t.Fatalf("could-not-check: the shared comment vector table is unreadable: %v", err)
	}
	var doc struct {
		Cells []commentCellVector `json:"cells"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cells) < 30 {
		t.Fatalf("positive control: only %d cell vectors loaded", len(doc.Cells))
	}
	var table []string
	for i, v := range doc.Cells {
		if v.Statusgen != v.Desk {
			t.Errorf("%s: the table lets the tools/desk executors (%s) differ from statusgen (%s)", v.Name, v.Desk, v.Statusgen)
		}
		if got := codeSpan(v.Cell); got != v.FirstSpan {
			t.Errorf("%s: codeSpan(%q) = %q, want %q", v.Name, v.Cell, got, v.FirstSpan)
		}
		if got := len(markedCommands(v.Cell)) > 0; got != v.Marked {
			t.Errorf("%s: marked = %v, want %v", v.Name, got, v.Marked)
		}
		if got := verifyCommand(v.Cell); got != v.Lift {
			t.Errorf("%s: verifyCommand(%q) = %q, want %q", v.Name, v.Cell, got, v.Lift)
		}
		_, why := proseLedCommandWhy(v.Cell)
		got := "runs"
		switch {
		case strings.Contains(why, "shell comment"):
			got = "comment-refused"
		case why != "" || strings.TrimSpace(verifyCommand(v.Cell)) == "":
			got = "refused"
		}
		if got != v.Statusgen {
			t.Errorf("%s: statusgen verdict on %q = %s (%q), want %s", v.Name, v.Cell, got, why, v.Statusgen)
		}
		table = append(table, "| "+strconv.Itoa(i+1)+" | "+v.Cell+" | exit 0 |")
	}
	rows := briefVerifyRows(verifyTable(table...))
	if len(rows) != len(doc.Cells) {
		t.Fatalf("want %d rows, got %d", len(doc.Cells), len(rows))
	}
	ws := runWitnesses(t.TempDir(), rows, "human:tester", "", "0000", "2026-10-15", 30*time.Second, false)
	for i, v := range doc.Cells {
		w := ws[i]
		refused := w.State == stateCouldNotRun && w.Exit == -1
		switch {
		case v.Statusgen != "runs" && !refused:
			t.Errorf("%s: witness state=%s exit=%d note=%q; want a could-not-run that never executed (%s)", v.Name, w.State, w.Exit, w.Note, v.Statusgen)
		case v.Statusgen == "comment-refused" && !strings.Contains(w.Note, "shell comment"):
			t.Errorf("%s: a comment-refused witness must say it is a shell comment, got %q", v.Name, w.Note)
		case v.Statusgen == "runs" && v.Passes && w.State != statePass:
			t.Errorf("%s: witness state=%s exit=%d note=%q; want pass", v.Name, w.State, w.Exit, w.Note)
		case v.Statusgen == "runs" && !v.Passes && w.State != stateFail:
			t.Errorf("%s: witness state=%s exit=%d note=%q; want fail", v.Name, w.State, w.Exit, w.Note)
		}
	}
}

// TestShellNoOpRemedy — a no-op lift (#3147 A2) is refused with its own remedy:
// replace the no-op. The marker remedy is wrong for it — a marked `cmd: :` is
// already marked, and marking an unmarked `:` changes nothing — and a marked
// no-op is reported as the marked command, not the first span.
// regression: #3147
func TestShellNoOpRemedy(t *testing.T) {
	for _, why := range []string{shellNoOpWhy, shellNoOpMarkedWhy} {
		if got := proseLedRemedy(why); !strings.HasPrefix(got, "Replace the no-op") {
			t.Errorf("proseLedRemedy(%q) = %q; want the replace-the-no-op remedy", why, got)
		}
	}
	for _, cell := range []string{"`:`", "`REM check the log`", "Read `x.md`: `cmd: :`"} {
		var msg string
		for _, f := range rowFindings(cell, "exit 0") {
			if f.rule == ruleProseLedCommand {
				msg = f.msg
			}
		}
		if !strings.Contains(msg, "Replace the no-op") || strings.Contains(msg, "explicit marker") {
			t.Errorf("%s: the prose-led finding must name the replace remedy, not the marker: %q", cell, msg)
		}
	}
	rows := briefVerifyRows(verifyTable("| 1 | Read `x.md`: `cmd: :` | exit 0 |"))
	if len(rows) != 1 || !strings.HasPrefix(rows[0].ProseLed, "marked command : is ") {
		t.Fatalf("a marked no-op must be reported as the marked command, got %+v", rows)
	}
}
