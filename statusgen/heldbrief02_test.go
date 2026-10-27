package main

import (
	"os"
	"strings"
	"testing"
)

// Tests for the held scan on the real Evidence shape that kept
// graph-execution/02 from flipping (#4294). The fixture
// (testdata/heldscope-4294/brief-02.md) is that brief as committed: thirteen
// entries recording BLOCKED runs and passes, then a last entry that re-ran all
// nine Verify rows clean in verifyrun witness tables with Date and Runner and
// closes on a live strict **VERIFY: PASS**. One earlier entry carries a
// one-line prose paragraph quoting a test pattern with an unescaped "|"
// (`-run "TestPatterns|TestTopologyValuesMatchSource"`). The scan read that
// line as a table it could not key, so no earlier entry was superseded whole
// and the first entry's BLOCKED heading refused the flip.
//
// The last entry's own prose also quotes base-tree tool output that reads
// COULD-NOT-CHECK, in double quotes rather than inline code. The held
// occurrences of the last entry are always read (heldscope.go, rule 3), so
// that line still refuses the real Evidence by design; quoted tool output is
// excluded only inside a code span (#2100). b02Coded is the real Evidence with
// that one quote written as a code span, the form the scan already admits.

// b02Evidence returns the fixture's Evidence section as the flip reads it.
func b02Evidence(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/heldscope-4294/brief-02.md")
	if err != nil {
		t.Fatal(err)
	}
	_, ev := briefSectionsOf(string(raw))
	if !strings.Contains(ev, "### Non-implementer verifier run — VERIFY: BLOCKED") {
		t.Fatal("fixture Evidence lost its first BLOCKED entry")
	}
	return ev
}

// b02Last returns the index in ev where its last entry's heading starts.
func b02Last(t *testing.T, ev string) int {
	t.Helper()
	i := strings.LastIndex(ev, "\n### ")
	if i < 0 {
		t.Fatal("fixture Evidence has no entry heading")
	}
	return i + 1
}

// b02Edit applies one string replacement to ev, failing the test when old is
// not present exactly once in the part of ev it names: before the last entry
// (earlier=true) or in the last entry.
func b02Edit(t *testing.T, ev string, earlier bool, old, repl string) string {
	t.Helper()
	cut := b02Last(t, ev)
	head, tail := ev[:cut], ev[cut:]
	part := &tail
	if earlier {
		part = &head
	}
	if n := strings.Count(*part, old); n != 1 {
		t.Fatalf("edit anchor %q occurs %d times, want 1", old, n)
	}
	*part = strings.Replace(*part, old, repl, 1)
	return head + tail
}

// b02Pipe is the prose line, in the 2026-10-24 entry, whose pipe was read as
// an unkeyable table line.
const b02Pipe = "The lint reports the [gotest-run-vacuous] NOTICE on rows 1, 3, 4 and 7 (go test -run carries"

// b02Quote is the last entry's base-tree diagnostic, quoted in prose.
const b02Quote = `prints "COULD-NOT-CHECK: delivery must be an ancestor of the checkout's HEAD"`

// b02Coded is the real Evidence with b02Quote written as a code span.
func b02Coded(t *testing.T) string {
	t.Helper()
	return b02Edit(t, b02Evidence(t), false, b02Quote,
		"prints `COULD-NOT-CHECK: delivery must be an ancestor of the checkout's HEAD`")
}

// TestHeldScopeB02Flips: the last entry, a strict PASS that re-ran all nine
// rows in witness tables with Date and Runner, supersedes every earlier entry
// whole. The real Evidence then refuses only on the last entry's own quoted
// diagnostic, never on a superseded entry; with that quote in a code span it
// passes, read through both brief readers.
// regression: #4294
func TestHeldScopeB02Flips(t *testing.T) {
	ev := b02Evidence(t)
	if s := heldScanScope(ev); !s.whole || s.from < 0 {
		t.Fatalf("heldScanScope: whole=%v from=%d, want the last entry to supersede every earlier one", s.whole, s.from)
	}
	err := closeVerifyHeldRefusal("graph-execution/02", "verified", ev)
	if err == nil || !strings.Contains(err.Error(), "Fail-first, row 10:") || strings.Contains(err.Error(), "2026-09-23") {
		t.Fatalf("real Evidence: refusal %v, want one naming only the last entry's own quoted COULD-NOT-CHECK", err)
	}
	coded := b02Coded(t)
	if err := closeVerifyHeldRefusal("graph-execution/02", "verified", coded); err != nil {
		t.Fatalf("closeVerifyHeldRefusal: %v", err)
	}
	if held, why := verifyPassHeldContradiction(coded); held {
		t.Fatalf("verifyPassHeldContradiction held: %q", why)
	}
	raw, err := os.ReadFile("testdata/heldscope-4294/brief-02.md")
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), b02Quote,
		"prints `COULD-NOT-CHECK: delivery must be an ancestor of the checkout's HEAD`", 1))
	bf, _, err := parseBriefFileBytes("brief-02.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeVerifyHeldRefusal("graph-execution/02", "verified", bf.Evidence); err != nil {
		t.Fatalf("parseBriefFileBytes reader: %v", err)
	}
}

// TestHeldScopeB02Prose: a prose paragraph of one line, at column 0 and
// between blank lines, is no table on the page (GFM needs a header and a
// delimiter row on adjacent lines and ends a table at a blank line), so its
// pipe no longer turns whole supersession off. A second, planted instance in
// another earlier entry is admitted the same way (defect class).
// regression: #4294
func TestHeldScopeB02Prose(t *testing.T) {
	ev := b02Coded(t)
	planted := b02Edit(t, ev, true, "\n\nVERIFY: BLOCKED\n### Correction: 2026-09-30",
		"\n\nRe-ran `grep` by hand as grep -E \"a|b\" spec/README.md; it printed 1.\n\nVERIFY: BLOCKED\n### Correction: 2026-09-30")
	runHeldLayerCases(t, []heldScanCase{
		{"real Evidence, quote in code", ev, false},
		{"planted second prose pipe", planted, false},
		{"second pipe on the line", b02Edit(t, ev, true, b02Pipe, "Ran a | b. "+b02Pipe), false},
	})
}

// TestHeldScopeB02Refuses: the supersession the fix admits stays narrow. Each
// case is b02Coded, which passes, with one change that must keep a hold live.
// regression: #4294
func TestHeldScopeB02Refuses(t *testing.T) {
	ev := b02Coded(t)
	hold := func(heading string) string {
		return "\n### " + heading + "\n\n- Row 3: BLOCKED — could-not-check, no Linux runner online.\n\n**VERIFY: BLOCKED**\n"
	}
	row10 := ev[b02Last(t, ev):]
	row10 = row10[strings.Index(row10, "| 10 |"):]
	row10 = row10[:strings.Index(row10, "\n")+1]
	cases := []heldScanCase{
		// A later entry that is not a strict full PASS supersedes nothing.
		{"latest drops row 10", b02Edit(t, ev, false, row10, ""), true},
		{"latest row 3 held", b02Edit(t, ev, false,
			"| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0",
			"| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | could-not-check exit=-1"), true},
		{"latest PASS not strict", b02Edit(t, ev, false, "**VERIFY: PASS** — 9 of 9", "VERIFY: PASS — 9 of 9"), true},
		{"latest PASS then BLOCKED", ev + "\nVERIFY: BLOCKED\n", true},
		{"real Evidence as committed", b02Evidence(t), true},
		// A hold newer than the last full PASS stays live, whatever its heading
		// says about its date or its standing.
		{"hold after the last PASS", ev + hold("Non-implementer verifier re-run — 2026-10-26"), true},
		{"old-dated hold after PASS", ev + hold("Non-implementer verifier run — 2026-09-01 (superseded)"), true},
		{"BLOCKED heading after PASS",
			ev + "\n### Non-implementer verifier run — VERIFY: BLOCKED — 4/9 pass, 5 could-not-check, 0 fail — 2026-09-23\n", true},
		// The prose exemption covers a one-line paragraph only.
		{"delimiter row under it", b02Edit(t, ev, true, b02Pipe, "| a | b |\n|---|---|\n"+b02Pipe), true},
		{"delimiter row after it", b02Edit(t, ev, true, b02Pipe, "| a | b |\n"+b02Pipe), true},
		{"joined to the line above", b02Edit(t, ev, true, "\n\n"+b02Pipe, "\n"+b02Pipe), true},
		{"joined to the line below", b02Edit(t, ev, true, "\n\nDelivered code since the previous pass (b87d9f50)",
			"\nDelivered code since the previous pass (b87d9f50)"), true},
		{"led by a pipe", b02Edit(t, ev, true, b02Pipe, "| "+b02Pipe), true},
		{"in a blockquote", b02Edit(t, ev, true, b02Pipe, "> "+b02Pipe), true},
		{"in a list item", b02Edit(t, ev, true, b02Pipe, "- "+b02Pipe), true},
		{"indented", b02Edit(t, ev, true, b02Pipe, "  "+b02Pipe), true},
		{"stray CR splits it", b02Edit(t, ev, true, "4 and 7 (go test", "4 and 7\r|---|---|\r(go test"), true},
	}
	runHeldLayerCases(t, cases)
}

// TestHeldScopePipeProse: the exemption on synthetic Evidence beside the
// whole-supersession cases it must not change (TestHeldScopeWholeSuperseded).
// regression: #4294
func TestHeldScopePipeProse(t *testing.T) {
	blocked := "**VERIFY: BLOCKED** — could-not-check, see the note above."
	final := proseRun("Run 2", 8, allOK, "", "**VERIFY: PASS**")
	eight := func(extra string) string { return proseRun("Run 1", 8, allOK, extra, blocked) }
	runHeldLayerCases(t, []heldScanCase{
		{"one-line prose pipe", eight(`Ran go test -run "A|B" by hand.`) + final, false},
		{"two prose pipe paragraphs", eight("Ran a|b.\n\nRan c|d.") + final, false},
		// Unchanged: tables and table-like runs the scan cannot key.
		{"no-separator table", eight("| # | Result |\n| 9 | exit 2 |\n\n- row 9 could-not-check.") + final, true},
		{"pipeless table", eight("# | Result\n--- | ---\n9 | exit 2\n\n- row 9 could-not-check.") + final, true},
		{"pipeless no separator", eight("# | Result\n9 | exit 2\n\n- row 9 could-not-check.") + final, true},
		{"two-line pipe paragraph", eight("Ran a|b and\nthen c.\n\n- row 9 could-not-check.") + final, true},
		{"lone row line", eight("| 9 | exit 2 |\n\n- row 9 could-not-check.") + final, true},
		{"HTML table tag in prose", eight("Ran <td>9</td> by hand.\n\n- row 9 could-not-check.") + final, true},
		{"prose pipe, row not re-run", eight(`Ran "a|b".`) + proseRun("Run 2", 7, allOK, "", "**VERIFY: PASS**"), true},
	})
}
