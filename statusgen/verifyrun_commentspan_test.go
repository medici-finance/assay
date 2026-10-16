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

// TestCommentLedRowNotExecuted — the witness a run writes for the #2684 cell is
// could-not-run (exit -1, no output fingerprint, the rule tag in the note), never
// the empty-output pass. The same cell with a marked command runs it and passes
// (the positive control: the executor still executes a legitimate row).
// regression: #2684
func TestCommentLedRowNotExecuted(t *testing.T) {
	rows := briefVerifyRows(verifyTable(
		"| 1 | Inspect the `## Coverage` section of the review | exit 0 |",
		"| 2 | Inspect `## Coverage`, then run `cmd: test -d .` | exit 0 |",
		"| 3 | `## Coverage` | exit 0 |",
	))
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	ws := runWitnesses(t.TempDir(), rows, "human:tester", "", "0000", "2026-10-15", 30*time.Second, false)
	if len(ws) != 3 {
		t.Fatalf("want 3 witnesses, got %d", len(ws))
	}
	for _, i := range []int{0, 2} {
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

// TestCommentLedMarkedCommandIsVacuous — a MARKED command that is only a comment
// is the explicit contract, so it still runs; the lint NOTICEs it as
// cmd-marker-vacuous so the author marks the command that performs the check.
func TestCommentLedMarkedCommandIsVacuous(t *testing.T) {
	for _, cell := range []string{
		"`cmd: ## Coverage`",
		"Read `x.md`: `cmd: # placeholder`",
		"`cmd: #coverage`",
		"`cmd: # don't forget the quote`",
	} {
		if fs := rowFindings(cell, "exit 0"); !hasRule(fs, "cmd-marker-vacuous") {
			t.Errorf("%q must raise cmd-marker-vacuous, got %+v", cell, fs)
		}
	}
	if fs := rowFindings("`cmd: go test ./x -count=1 # the real check`", "exit 0"); hasRule(fs, "cmd-marker-vacuous") {
		t.Errorf("a command with a trailing comment must not raise cmd-marker-vacuous, got %+v", fs)
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
