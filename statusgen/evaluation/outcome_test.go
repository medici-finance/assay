package evaluation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// outcome_test.go — the protocol's matrix and its class guard.

// TestOutcomeProtocolMatrix pins the shared meaning once: the outcome vocabulary as a reader
// and as a writer, the lifecycle disposition, the row reduction and the run reduction, and the
// record reader over both layouts. Every producer and reader reaches these rules through this
// package, so a row here is a row for all of them.
// regression: F-outcome-protocol-copies
func TestOutcomeProtocolMatrix(t *testing.T) {
	t.Run("vocabulary: the reader classification of every spelling", func(t *testing.T) {
		cases := []struct {
			in   string
			want OutcomeClass
		}{
			{"verified", OutcomePass},
			{"Verified", OutcomePass},
			{" VERIFIED ", OutcomePass},
			{"verify-fail", OutcomeFail},
			{"fail", OutcomeFail},
			{"FAIL", OutcomeFail},
			{"blocked", OutcomeHold},
			{"needs_context", OutcomeHold},
			{"needs-context", OutcomeHold},
			{"Needs-Context", OutcomeHold},
			{"pass", OutcomeUnrecognised},
			{"ok", OutcomeUnrecognised},
			{"done", OutcomeUnrecognised},
			{"", OutcomeUnrecognised},
			{"verified!", OutcomeUnrecognised},
		}
		for _, c := range cases {
			if got := ClassifyOutcome(c.in); got != c.want {
				t.Errorf("ClassifyOutcome(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	})

	t.Run("vocabulary: the writer accepts what the readers recognise, and nothing else", func(t *testing.T) {
		for _, w := range append(OutcomeWords(), "Verified", " verified", "verified ", "VERIFIED", "Fail", " Needs-Context ") {
			c, err := ParseOutcomeWord(w)
			if err != nil {
				t.Errorf("ParseOutcomeWord(%q) refused a word the readers recognise: %v", w, err)
			}
			if c != ClassifyOutcome(w) {
				t.Errorf("writer and reader disagree on %q: %v vs %v", w, c, ClassifyOutcome(w))
			}
		}
		// A row result ("pass"), an unknown word, or nothing at all is refused, so the writer
		// never creates a record a reader would classify as unrecognised (#6002).
		for _, bad := range []string{"pass", "ok", "", "done", "could-not-run", "future-value", "verified!"} {
			if _, err := ParseOutcomeWord(bad); err == nil {
				t.Errorf("ParseOutcomeWord(%q) accepted a word no reader recognises", bad)
			}
		}
		if got := len(OutcomeWords()); got != 6 {
			t.Errorf("vocabulary has %d words, want 6", got)
		}
	})

	t.Run("vocabulary: failed-or-held and non-decisive agree with the class", func(t *testing.T) {
		for _, w := range OutcomeWords() {
			c := ClassifyOutcome(w)
			if IsFailedOrHeld(w) != (c == OutcomeFail || c == OutcomeHold) {
				t.Errorf("IsFailedOrHeld(%q) disagrees with its class %v", w, c)
			}
			if IsNonDecisive(w) != (c == OutcomeHold) {
				t.Errorf("IsNonDecisive(%q) disagrees with its class %v", w, c)
			}
		}
		for in, want := range map[string]string{
			"blocked": "BLOCKED", "BLOCKED": "BLOCKED", "needs_context": "NEEDS-CONTEXT",
			"needs-context": "NEEDS-CONTEXT", "verified": "", "verify-fail": "", "ok": "",
		} {
			if got := NonDecisiveVerdictWord(in); got != want {
				t.Errorf("NonDecisiveVerdictWord(%q) = %q, want %q", in, got, want)
			}
		}
	})

	t.Run("lifecycle: no outcome changes an accepted delivery", func(t *testing.T) {
		for _, prior := range []Lifecycle{LifecycleOpen, LifecycleAccepted} {
			for _, w := range append(OutcomeWords(), "ok", "") {
				d := Dispose(prior, w)
				if d.Next != prior {
					t.Errorf("Dispose(%v, %q).Next = %v: an outcome moved the lifecycle", prior, w, d.Next)
				}
				switch d.Class {
				case OutcomePass:
					if d.Role != RoleClosureCandidate || !d.NeedsClosure {
						t.Errorf("a pass must be a closure candidate needing the closure check: %+v", d)
					}
				case OutcomeFail, OutcomeHold:
					if d.Role != RoleDiagnostic || d.NeedsClosure {
						t.Errorf("a fail/hold must be a diagnostic: %+v", d)
					}
				default:
					if d.Role != RoleUnreadable || d.NeedsClosure {
						t.Errorf("an unrecognised outcome must be unreadable: %+v", d)
					}
				}
			}
		}
	})

	t.Run("rows: the reduction of counts, held rows and row_results", func(t *testing.T) {
		pass := func(r string) RowResultEntry { return RowResultEntry{r, RowPass} }
		cnr := func(r string) RowResultEntry { return RowResultEntry{r, RowCouldNotRun} }
		cases := []struct {
			name   string
			in     RowSet
			fault  FaultKind
			state  RowsState
			passed []string
			nonp   []string
		}{
			{name: "empty is unavailable", in: RowSet{}, state: RowsUnavailable},
			{name: "count only", in: RowSet{HasTotal: true, Total: 3, HasPassed: true, Passed: 2, Held: []string{"3"}}, state: RowsCountOnly},
			{name: "passed over total", in: RowSet{HasTotal: true, Total: 1, HasPassed: true, Passed: 2}, fault: FaultPassedExceedsTotal, state: RowsCountOnly},
			{name: "held repeats", in: RowSet{Held: []string{"1", "1"}}, fault: FaultHeldRepeats},
			{name: "passed over non-held", in: RowSet{HasTotal: true, Total: 2, HasPassed: true, Passed: 2, Held: []string{"1"}}, fault: FaultPassedExceedsNonHeld, state: RowsCountOnly},
			{name: "results need total", in: RowSet{HasResults: true, Results: []RowResultEntry{pass("1")}}, fault: FaultResultsNeedTotal, state: RowsExact},
			{name: "results cover all rows", in: RowSet{HasResults: true, HasTotal: true, Total: 2, Results: []RowResultEntry{pass("1")}}, fault: FaultResultsCount, state: RowsExact},
			{name: "result repeats", in: RowSet{HasResults: true, HasTotal: true, Total: 2, Results: []RowResultEntry{pass("1"), pass("1")}}, fault: FaultResultRepeats, state: RowsExact},
			{name: "result vocabulary", in: RowSet{HasResults: true, HasTotal: true, Total: 1, Results: []RowResultEntry{{"1", "green"}}}, fault: FaultResultInvalid, state: RowsExact},
			{name: "held row claims pass", in: RowSet{HasResults: true, HasTotal: true, Total: 1, Held: []string{"1"}, Results: []RowResultEntry{pass("1")}}, fault: FaultHeldClaimsPass, state: RowsExact},
			{name: "pass count disagrees", in: RowSet{HasResults: true, HasTotal: true, Total: 2, HasPassed: true, Passed: 1, Results: []RowResultEntry{pass("1"), pass("2")}}, fault: FaultPassCount, state: RowsExact},
			{name: "held absent from inventory", in: RowSet{HasResults: true, HasTotal: true, Total: 1, Held: []string{"9"}, Results: []RowResultEntry{cnr("1")}}, fault: FaultHeldAbsent, state: RowsExact},
			{name: "held outside count-only inventory", in: RowSet{HasTotal: true, Total: 2, Held: []string{"6a"}}, fault: FaultHeldOutsideInventory, state: RowsCountOnly},
			{name: "exact results keep non-pass rows", in: RowSet{HasResults: true, HasTotal: true, Total: 3, HasPassed: true, Passed: 1, Held: []string{"2"},
				Results: []RowResultEntry{pass("1"), cnr("2"), {"3", RowDeferred}}}, state: RowsExact, passed: []string{"1"}, nonp: []string{"2", "3"}},
		}
		for _, c := range cases {
			red, f := ReduceRows(c.in)
			if c.fault != 0 {
				if f == nil || f.Kind != c.fault {
					t.Errorf("%s: fault = %v, want kind %v", c.name, f, c.fault)
				} else if f.Text() == "" || f.Error() != f.Text() {
					t.Errorf("%s: fault has no text", c.name)
				}
				continue
			}
			if f != nil {
				t.Errorf("%s: unexpected fault %v", c.name, f)
				continue
			}
			if red.State != c.state {
				t.Errorf("%s: state = %v, want %v", c.name, red.State, c.state)
			}
			if c.passed != nil && fmt.Sprint(red.Passed) != fmt.Sprint(c.passed) {
				t.Errorf("%s: passed = %v, want %v", c.name, red.Passed, c.passed)
			}
			if c.nonp != nil && fmt.Sprint(red.NonPass) != fmt.Sprint(c.nonp) {
				t.Errorf("%s: non-pass = %v, want %v", c.name, red.NonPass, c.nonp)
			}
		}
	})

	t.Run("rows: the empty held set never erases an observed non-pass row", func(t *testing.T) {
		red, f := ReduceRows(RowSet{HasResults: true, HasTotal: true, Total: 2, HasPassed: true, Passed: 1,
			Results: []RowResultEntry{{"1", RowPass}, {"2", RowFail}}})
		if f != nil {
			t.Fatal(f)
		}
		if len(red.NonPass) != 1 || red.NonPass[0] != "2" || red.Complete {
			t.Errorf("a failed row with no held set must stay visible and incomplete: %+v", red)
		}
	})

	t.Run("rows: bound to the current Verify table", func(t *testing.T) {
		inv := []string{"1", "2", "3"}
		bound := func(s RowSet) RowSet { s.BoundToInventory, s.Inventory = true, inv; return s }
		cases := []struct {
			name     string
			in       RowSet
			fault    FaultKind
			runnable string
		}{
			{name: "whole brief", in: bound(RowSet{}), runnable: ""},
			{name: "held rows withheld", in: bound(RowSet{Held: []string{"2"}}), runnable: "1,3"},
			{name: "unknown held row", in: bound(RowSet{Held: []string{"9"}}), fault: FaultHeldUnknown},
			{name: "exact results subtract passed rows", in: bound(RowSet{Held: []string{"3"}, HasResults: true, HasTotal: true, Total: 3, HasPassed: true, Passed: 1,
				Results: []RowResultEntry{{"1", RowPass}, {"2", RowCouldNotRun}, {"3", RowDeferred}}}), runnable: "2"},
			{name: "coverage", in: bound(RowSet{Held: []string{"3"}, HasResults: true, HasTotal: true, Total: 2, Results: []RowResultEntry{{"1", RowPass}, {"2", RowPass}}}), fault: FaultCoverage},
			{name: "unknown result row", in: bound(RowSet{Held: []string{"3"}, HasResults: true, HasTotal: true, Total: 3, HasPassed: true, Passed: 3,
				Results: []RowResultEntry{{"1", RowPass}, {"2", RowPass}, {"9", RowPass}}}), fault: FaultResultUnknown},
			{name: "count-only all non-held passed", in: bound(RowSet{Held: []string{"3"}, HasTotal: true, Total: 3, HasPassed: true, Passed: 2}), runnable: ""},
			{name: "count-only some passed", in: bound(RowSet{Held: []string{"3"}, HasTotal: true, Total: 3, HasPassed: true, Passed: 1}), runnable: "1,2"},
			{name: "count-only wrong total", in: bound(RowSet{Held: []string{"3"}, HasTotal: true, Total: 5, HasPassed: true, Passed: 1}), fault: FaultCountsDisagree},
		}
		for _, c := range cases {
			red, f := ReduceRows(c.in)
			if c.fault != 0 {
				if f == nil || f.Kind != c.fault {
					t.Errorf("%s: fault = %v, want kind %v", c.name, f, c.fault)
				}
				continue
			}
			if f != nil {
				t.Errorf("%s: unexpected fault %v", c.name, f)
				continue
			}
			if got := strings.Join(red.Runnable, ","); got != c.runnable {
				t.Errorf("%s: runnable = %q, want %q", c.name, got, c.runnable)
			}
		}
		// A label inventory cannot be proven by counts alone.
		_, f := ReduceRows(RowSet{BoundToInventory: true, Inventory: []string{"R", "S1"}, Held: []string{"S1"}, HasTotal: true, Total: 2, HasPassed: true, Passed: 1})
		if f == nil || f.Kind != FaultCountOnlyNeedsResults {
			t.Errorf("label rows with counts only must demand exact row_results, got %v", f)
		}
	})

	t.Run("run: the outcome against its rows", func(t *testing.T) {
		held := RowSet{Held: []string{"2"}, HasTotal: true, Total: 2, HasPassed: true, Passed: 1}
		allPass := RowSet{HasResults: true, HasTotal: true, Total: 2, HasPassed: true, Passed: 2, Results: []RowResultEntry{{"1", RowPass}, {"2", RowPass}}}
		failing := RowSet{HasResults: true, HasTotal: true, Total: 2, HasPassed: true, Passed: 1, Results: []RowResultEntry{{"1", RowPass}, {"2", RowFail}}}
		cases := []struct {
			outcome string
			rows    RowSet
			want    RunVerdict
		}{
			{"verified", RowSet{}, VerdictCandidate},
			{"verified", allPass, VerdictCandidate},
			{"verified", held, VerdictContradiction},
			{"verified", failing, VerdictContradiction},
			{"Verified", held, VerdictContradiction},
			{"blocked", held, VerdictRowBlocked},
			{"verify-fail", failing, VerdictRowBlocked},
			{"blocked", allPass, VerdictClosureOnly},
			{"fail", RowSet{}, VerdictRowsUnavailable},
			{"ok", RowSet{}, VerdictUnreadable},
			{"ok", held, VerdictUnreadable},
		}
		for _, c := range cases {
			run, f := ReduceRun(c.outcome, c.rows)
			if f != nil {
				t.Errorf("ReduceRun(%q): unexpected fault %v", c.outcome, f)
				continue
			}
			if run.Verdict != c.want {
				t.Errorf("ReduceRun(%q, %+v) = %v, want %v", c.outcome, c.rows, run.Verdict, c.want)
			}
		}
		if _, f := ReduceRun("verified", RowSet{Held: []string{"1", "1"}}); f == nil {
			t.Error("a row fault must surface as a fault, never fold into a verdict")
		}
	})

	t.Run("row identity grammar", func(t *testing.T) {
		for _, ok := range []string{"1", "6", "6a", "12b", "R", "S1", "r2"} {
			if id, good := ParseRowID(ok); !good || id != ok {
				t.Errorf("ParseRowID(%q) refused a valid identity", ok)
			}
		}
		for _, bad := range []string{"", "0", "06", "R-1", "6a1", "1.5", "a b", "-1", "6ab"} {
			if _, good := ParseRowID(bad); good {
				t.Errorf("ParseRowID(%q) accepted an invalid identity", bad)
			}
		}
	})
}

// --- the class guard ---------------------------------------------------------------------

// protocolRule is one shape of private copy of the outcome protocol. A file outside the owner
// that matches any rule is a second definition of the protocol.
type protocolRule struct {
	name string
	// all must match (every pattern, anywhere in the file's code lines) for the rule to fire.
	all []*regexp.Regexp
}

var protocolRules = []protocolRule{
	{"vocabulary-word", []*regexp.Regexp{regexp.MustCompile(`"(verify-fail|needs_context|needs-context)"`)}},
	{"exact-verified", []*regexp.Regexp{regexp.MustCompile(`[Oo]utcome\w*(\)|\])?\s*[!=]=\s*"(verified|verify-fail|fail|blocked)"`)}},
	{"brief-key-grammar", []*regexp.Regexp{
		regexp.MustCompile("`\\^\\[a-z0-9\\]\\[a-z0-9-\\]\\*\\$`"),
		regexp.MustCompile("`\\^\\[0-9\\]\\+\\$`"),
	}},
	{"row-result-vocabulary", []*regexp.Regexp{regexp.MustCompile(`"could-not-run"`), regexp.MustCompile(`"deferred"`)}},
}

// protocolAllowList names, module-relative, the non-test files that may match a rule, with the
// reason. A stale entry (the file no longer matches) is also a failure.
var protocolAllowList = map[string]string{
	"statusgen/evaluation/outcome.go": "the protocol's owner: the one definition",
}

// scanProtocolCopies walks root for non-test .go files (skipping testdata) and returns
// "<rel>:<rule>" for every file matching a rule. Comment-only lines are ignored so prose that
// names a spelling is not a definition.
func scanProtocolCopies(root, rel string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		var code []string
		for _, line := range strings.Split(string(raw), "\n") {
			if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
				continue
			}
			code = append(code, line)
		}
		body := strings.Join(code, "\n")
		r, _ := filepath.Rel(root, p)
		for _, rule := range protocolRules {
			all := true
			for _, re := range rule.all {
				if !re.MatchString(body) {
					all = false
					break
				}
			}
			if all {
				hits = append(hits, filepath.ToSlash(filepath.Join(rel, r))+":"+rule.name)
			}
		}
		return nil
	})
	return hits, err
}

// TestOutcomeProtocolClassGuard pins the CLASS, not the four copies that motivated it: no file
// outside the protocol's owner may carry a private outcome vocabulary, an exact-spelling
// closure comparison, a second brief-key grammar or a second row-result vocabulary. A planted
// file proves every rule is live.
// regression: F-outcome-protocol-copies
func TestOutcomeProtocolClassGuard(t *testing.T) {
	type tree struct{ root, rel string }
	var trees []tree
	for _, c := range []tree{{"..", "statusgen"}, {"../../tools/desk", "tools/desk"}} {
		if st, err := os.Stat(c.root); err == nil && st.IsDir() {
			trees = append(trees, c)
		}
	}
	if len(trees) == 0 {
		t.Fatal("no source tree reachable from the evaluation package")
	}
	hitSet := map[string]bool{}
	for _, tr := range trees {
		hits, err := scanProtocolCopies(tr.root, tr.rel)
		if err != nil {
			t.Fatalf("scan %s: %v", tr.rel, err)
		}
		for _, h := range hits {
			file := h[:strings.LastIndex(h, ":")]
			hitSet[file] = true
			if _, ok := protocolAllowList[file]; !ok {
				t.Errorf("%s: a second definition of the verification-outcome protocol outside statusgen/evaluation/outcome.go — "+
					"delegate to evaluation.ClassifyOutcome / ParseOutcomeWord / ParseRecord / ReduceRows instead of re-stating the rule", h)
			}
		}
	}
	if len(trees) == 2 {
		for f := range protocolAllowList {
			if !hitSet[f] {
				t.Errorf("allow-list entry %s no longer matches any rule — remove the stale entry", f)
			}
		}
	}

	// Positive control: the committed fixture plants every rule outside the owner.
	hits, err := scanProtocolCopies("testdata/outcome_fixture", "testdata/outcome_fixture")
	if err != nil {
		t.Fatal(err)
	}
	fired := map[string]bool{}
	for _, h := range hits {
		fired[h[strings.LastIndex(h, ":")+1:]] = true
	}
	for _, rule := range protocolRules {
		if !fired[rule.name] {
			t.Errorf("positive control: the planted fixture did not trip rule %q — the guard is not live", rule.name)
		}
	}
}
