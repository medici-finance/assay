package main

// deskdigest_test.go — the suite is organised around the three invariants the tool
// exists for, because each of them is a property that a plausible-looking generator
// silently violates:
//
//	CLASSIFICATION  no safe default; blessing-authority comment > human-only label > trusted body
//	                override > R-3 criteria > unclassified. Nothing reaches the
//	                desk-decides lane by exhaustion.
//	EMPTY WEEK      a week with nothing in it renders AND posts a digest saying so.
//	COULD-NOT-READ  a third state everywhere: unread threads, unanswered repos, and
//	                labels that do not exist yet.
//
// Plus the boundary: the read paths construct no mutating gh argv at all, and the write
// path constructs exactly one — create or edit, on the tool's own weekly issue.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestMain(m *testing.M) {
	cleanup, err := installFixtureRoster()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot install the test-fixture roster:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// blessLogin/blessID are the fixture roster's blessing authority — the identity whose
// comment outranks every other classification input.
const (
	blessLogin = "ada"
	blessID    = int64(2001)
)

var fixedNow = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

// mkItem builds a read item (CommentsRead=true) authored by a trusted login.
func mkItem(title, body string, labels ...string) *item {
	return &item{
		Repo: "example-org/tracker", Number: 101, Title: title, Body: body,
		URL: "https://example.invalid/tracker/issues/101", Labels: labels,
		AuthorLogin: blessLogin, AuthorID: blessID, AuthorType: "User",
		CreatedAt: fixedNow.AddDate(0, 0, -10), CommentsRead: true,
	}
}

// blessComment is a comment by the roster's blessing authority.
func blessComment(body string, at time.Time) comment {
	return comment{AuthorLogin: blessLogin, AuthorID: blessID, AuthorType: "User", Body: body, CreatedAt: at}
}

// ---------------------------------------------------------------------------
// classification
// ---------------------------------------------------------------------------

// TestClassificationReversible — R-3's own examples of a one-commit reversal reach the
// reversible lane, and do so by a NAMED signal. The provenance matters as much as the
// verdict: a lane entered without a stated reason is a lane nobody can audit.
func TestClassificationReversible(t *testing.T) {
	for _, c := range []struct{ name, title string }{
		{"docs wording", "Fix the docs wording in the intake README"},
		{"lint level", "Should the unrun lint level be a notice or error?"},
		{"port or drop", "port-or-drop the legacy sweep script"},
		{"tool default", "change the tool default for --sla-days"},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := classifyItem(mkItem(c.title, "no other signal here"))
			if v.Class != classReversible {
				t.Fatalf("class = %q, want reversible (why: %s)", v.Class, v.Why)
			}
			if v.Source != srcHeuristic {
				t.Errorf("source = %q, want %q", v.Source, srcHeuristic)
			}
			if strings.TrimSpace(v.Why) == "" {
				t.Error("a reversible verdict rendered no reason; an unauditable lane is the defect")
			}
		})
	}
}

// TestClassificationHumanOnly — R-3's four human-only categories, and the asymmetry: an item
// that is BOTH a docs-wording change and a security change is a security change. The
// tie-break is not a preference, it is the only direction whose failure is recoverable.
func TestClassificationHumanOnly(t *testing.T) {
	for _, c := range []struct{ name, title string }{
		{"irreversible", "publish the corpus to a public repo"},
		{"mechanism", "who may flip a PR ready — delegate to the desk?"},
		{"security", "rotate the App token scope"},
		{"spend", "approve the subscription cost for the eval harness"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if v := classifyItem(mkItem(c.title, "")); v.Class != classHumanOnly {
				t.Fatalf("class = %q, want human-only (why: %s)", v.Class, v.Why)
			}
		})
	}
	t.Run("both signals resolve human-only", func(t *testing.T) {
		v := classifyItem(mkItem("Fix the docs wording on the token rotation runbook", ""))
		if v.Class != classHumanOnly {
			t.Fatalf("class = %q, want human-only: an item that is both cheap-to-revert and "+
				"security-shaped is security-shaped (why: %s)", v.Class, v.Why)
		}
	})
}

// TestClassificationBodyOverride — a `decision-class:` line in the body overrides the
// R-3 criteria for a TRUSTED author, and is REFUSED for an untrusted one.
//
// The second half is the security property. The body of an issue is written by whoever
// opened it; honouring an untrusted body would let an outside author move their own item
// into the lane the desk decides without asking. That is not an override, it is a
// self-service authority grant, and the refusal lands on `unclassified` — a request for a
// human look — rather than on the heuristic's answer.
func TestClassificationBodyOverride(t *testing.T) {
	t.Run("trusted author overrides the heuristic", func(t *testing.T) {
		it := mkItem("rotate the App token scope", "decision-class: reversible\nit is a one-line revert")
		v := classifyItem(it)
		if v.Class != classReversible || v.Source != srcBodyOverride {
			t.Fatalf("got %q/%q, want reversible/body-override (why: %s)", v.Class, v.Source, v.Why)
		}
	})
	t.Run("trusted author can override the other way", func(t *testing.T) {
		it := mkItem("fix the docs wording", "decision-class: human-only")
		if v := classifyItem(it); v.Class != classHumanOnly || v.Source != srcBodyOverride {
			t.Fatalf("got %q/%q, want human-only/body-override", v.Class, v.Source)
		}
	})
	t.Run("untrusted author is refused, not obeyed", func(t *testing.T) {
		it := mkItem("some question", "decision-class: reversible")
		it.AuthorLogin, it.AuthorID = "drive-by", 909090
		v := classifyItem(it)
		if v.Class != classUnclassified {
			t.Fatalf("class = %q, want unclassified — an untrusted body must not reach the "+
				"desk-decides lane (why: %s)", v.Class, v.Why)
		}
		if !strings.Contains(v.Why, "trust roster") {
			t.Errorf("the refusal must say WHY it was refused; got %q", v.Why)
		}
	})
	t.Run("an unknown override value is reported, not rounded", func(t *testing.T) {
		v := classifyItem(mkItem("q", "decision-class: probably-fine"))
		if v.Class != classUnclassified {
			t.Fatalf("class = %q, want unclassified for an unrecognised override value", v.Class)
		}
	})
}

// TestClassificationBlessWins — the precedence rule the brief calls load-bearing: a
// blessing-authority comment always wins, over the heuristic AND over a trusted body override, in
// both directions, and the LATEST blessing-authority comment wins over an earlier one.
func TestClassificationBlessWins(t *testing.T) {
	t.Run("beats a trusted body override", func(t *testing.T) {
		it := mkItem("fix the docs wording", "decision-class: reversible")
		it.Comments = []comment{blessComment("decision-class: human-only\nI want this one.", fixedNow)}
		v := classifyItem(it)
		if v.Class != classHumanOnly || v.Source != srcBlessComment {
			t.Fatalf("got %q/%q, want human-only/bless-comment (why: %s)", v.Class, v.Source, v.Why)
		}
	})
	t.Run("beats the human-only label", func(t *testing.T) {
		it := mkItem("rotate a token", "", humanOnlyLabel)
		it.Comments = []comment{blessComment("decision-class: reversible", fixedNow)}
		if v := classifyItem(it); v.Class != classReversible || v.Source != srcBlessComment {
			t.Fatalf("got %q/%q, want reversible/bless-comment", v.Class, v.Source)
		}
	})
	t.Run("the latest blessing-authority comment wins", func(t *testing.T) {
		it := mkItem("q", "")
		it.Comments = []comment{
			blessComment("decision-class: reversible", fixedNow.AddDate(0, 0, -3)),
			blessComment("decision-class: human-only", fixedNow),
		}
		if v := classifyItem(it); v.Class != classHumanOnly {
			t.Fatalf("class = %q, want the LATEST the blessing authority word (human-only)", v.Class)
		}
	})
	t.Run("a lookalike comment does not win", func(t *testing.T) {
		// Same login, wrong numeric id: the strict id-pinned check is what stops a
		// recycled or squatted login from ruling on the human's behalf.
		it := mkItem("fix the docs wording", "")
		it.Comments = []comment{{AuthorLogin: blessLogin, AuthorID: 999, AuthorType: "User",
			Body: "decision-class: human-only", CreatedAt: fixedNow}}
		if v := classifyItem(it); v.Source == srcBlessComment {
			t.Fatalf("a login match with the wrong id was accepted as the blessing authority (why: %s)", v.Why)
		}
	})
	t.Run("a bot comment does not win", func(t *testing.T) {
		it := mkItem("fix the docs wording", "")
		it.Comments = []comment{{AuthorLogin: blessLogin, AuthorID: blessID, AuthorType: "Bot",
			Body: "decision-class: human-only", CreatedAt: fixedNow}}
		if v := classifyItem(it); v.Source == srcBlessComment {
			t.Fatalf("an App/Bot artifact was accepted as a human ruling (why: %s)", v.Why)
		}
	})
}

// TestClassificationHumanLabel — the human-only label is human-only by construction and
// a body line cannot argue it out of that lane.
func TestClassificationHumanLabel(t *testing.T) {
	it := mkItem("fix the docs wording", "decision-class: reversible", humanOnlyLabel)
	v := classifyItem(it)
	if v.Class != classHumanOnly || v.Source != srcHumanLabel {
		t.Fatalf("got %q/%q, want human-only/human-only-label (why: %s)", v.Class, v.Source, v.Why)
	}
}

// TestClassificationNoDefault — THE no-safe-default invariant, stated as the property
// rather than as an example: over a corpus of items carrying no R-3 signal at all,
// NOTHING may come back reversible. An engine with a safe default passes every other test
// in this file and fails this one.
func TestClassificationNoDefault(t *testing.T) {
	corpus := []string{
		"Question about the thing",
		"What should we do here?",
		"follow-up from the last pass",
		"needs a call on the approach",
		"unclear — please advise",
		"",
	}
	for _, title := range corpus {
		v := classifyItem(mkItem(title, "body with nothing decisive in it"))
		if v.Class == classReversible {
			t.Fatalf("%q classified reversible with no signal — that is a safe default, and there "+
				"must not be one (why: %s)", title, v.Why)
		}
		if v.Class != classUnclassified || v.Source != srcNoSignal {
			t.Fatalf("%q → %q/%q, want unclassified/no-signal", title, v.Class, v.Source)
		}
	}
}

// TestNoDefaultControlCanFail is the POSITIVE CONTROL for the invariant above. A check
// never observed to both pass and fail is a comment that happens to compile — so the same
// predicate TestClassificationNoDefault uses is run here against a classifier that DOES
// have a safe default, and is asserted to catch it.
//
// Without this, "no item classified reversible" is satisfied just as well by a corpus
// that happens to contain nothing, by a predicate reading the wrong field, or by a
// classifier that returns the zero value for everything.
func TestNoDefaultControlCanFail(t *testing.T) {
	// The defect being controlled for: an engine whose last arm falls back to the cheap
	// lane instead of asking. This is the one-line change that would make the real
	// classifier wrong, written out so the test can be seen catching it.
	withSafeDefault := func(*item) verdict {
		return verdict{classReversible, srcNoSignal, "nothing fired, assuming it is cheap"}
	}
	caught := false
	for _, title := range []string{"Question about the thing", "unclear — please advise"} {
		if withSafeDefault(mkItem(title, "")).Class == classReversible {
			caught = true
		}
	}
	if !caught {
		t.Fatal("the no-safe-default predicate did not flag a classifier that defaults to reversible — " +
			"the assertion in TestClassificationNoDefault is vacuous and proves nothing")
	}
	if classifyItem(mkItem("Question about the thing", "")).Class == classReversible {
		t.Fatal("the shipped classifier has a safe default")
	}
}

// TestEmptyWeekControlCanFail is the positive control for the empty-week assertions: a
// generator that OMITTED the empty week (rendered nothing) must fail the same checks
// TestZeroItemWeekIsAReport applies. If it did not, that test would pass for a tool that
// silently drops empty weeks — the exact failure this brief exists to prevent.
func TestEmptyWeekControlCanFail(t *testing.T) {
	omitted := "" // what a "nothing to report, so report nothing" generator emits
	if strings.Contains(omitted, "**0 items.**") || len(strings.TrimSpace(omitted)) >= 400 {
		t.Fatal("a digest that omitted the empty week satisfied the empty-week assertions; " +
			"those assertions cannot tell a report from a silence")
	}
	real := build(emptyColl(), signOff{State: signOffUnsigned}, fixedNow, "2026-W33").Render()
	if !strings.Contains(real, "**0 items.**") || len(strings.TrimSpace(real)) < 400 {
		t.Fatal("the shipped renderer produced a silence-shaped empty week")
	}
}

// TestClassificationUnread — could-not-read outranks every other arm, including a
// human-only label and a trusted body override, because the input that outranks
// everything (a blessing-authority comment) was not consulted. A verdict reached without the
// highest-precedence input is not a verdict.
func TestClassificationUnread(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"plain", ""},
		{"with a body override", "decision-class: reversible"},
		{"with docs wording", "fix the docs wording"},
	} {
		t.Run(c.name, func(t *testing.T) {
			it := mkItem("fix the docs wording", c.body, humanOnlyLabel)
			it.CommentsRead, it.CommentsErr = false, "HTTP 403"
			v := classifyItem(it)
			if v.Class != classCouldNotRead || v.Source != srcUnread {
				t.Fatalf("got %q/%q, want could-not-read/unread (why: %s)", v.Class, v.Source, v.Why)
			}
			if !strings.Contains(v.Why, "HTTP 403") {
				t.Errorf("the could-not-read state must carry its cause; got %q", v.Why)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rendering invariants
// ---------------------------------------------------------------------------

// emptyColl is a collection that read three repos cleanly and found nothing.
func emptyColl() *collection {
	scope := []string{"example-org/tracker", "example-org/agents", "medici-finance/assay"}
	c := &collection{Scope: scope}
	for _, r := range scope {
		c.Repos = append(c.Repos, repoRead{Repo: r, LabelSetRead: true, MissingLabels: []string{humanOnlyLabel}})
	}
	return c
}

// TestZeroItemWeekIsAReport — the empty-week invariant at the rendering layer. A digest
// for a week with nothing in it must SAY it is empty, name what it read, and say how the
// reader tells this apart from a run that never happened.
func TestZeroItemWeekIsAReport(t *testing.T) {
	d := build(emptyColl(), signOff{State: signOffUnsigned, Note: "unsigned"}, fixedNow, "2026-W33")
	body := d.Render()
	for _, want := range []string{
		"**0 items.**",
		"failed to run",
		"example-org/tracker",
		"medici-finance/assay",
		"## Queue",
		"## Coverage",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a zero-item digest must contain %q; body:\n%s", want, body)
		}
	}
	if len(strings.TrimSpace(body)) < 400 {
		t.Errorf("the zero-item digest is %d chars — an empty week is a REPORT, not a stub", len(body))
	}
}

// TestNoSafeDefaultRendering — the no-safe-default invariant at the rendering layer.
// An unclassified item must render as unclassified, must NOT render a 14-day default
// date (that horizon belongs to items with a stated safe default), and the table must
// tell the reader that unclassified is not a soft form of reversible.
func TestNoSafeDefaultRendering(t *testing.T) {
	unread := mkItem("something opaque", "")
	unread.Number = 202
	unread.CommentsRead, unread.CommentsErr = false, "HTTP 500"
	c := &collection{
		Scope: []string{"example-org/tracker"},
		Items: []*item{mkItem("a question with no signal", "nothing decisive"), unread},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true}},
	}
	d := build(c, signOff{State: signOffUnsigned, Note: "unsigned"}, fixedNow, "2026-W33")
	body := d.Render()

	if strings.Contains(body, "| `reversible` |") {
		t.Errorf("no item here carries a reversible signal, yet a row rendered reversible:\n%s", body)
	}
	for _, want := range []string{
		"`unclassified`",
		"`could-not-read`",
		"no default — needs classification first",
		"unknown — the item could not be read",
		"NOT soft forms of `reversible`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	// The 14-day horizon must not appear: it applies only to an human-only item that has
	// a stated safe default, and neither row here is one.
	if strings.Contains(body, fixedNow.AddDate(0, 0, defaultAnswerDays).Format("2006-01-02")) {
		t.Errorf("a default-if-no-answer DATE rendered for items with no stated default:\n%s", body)
	}
}

// TestNoDefaultBlocksUntilAnswered — an human-only item with a stated default gets the
// 14-day date; one without gets the brief's literal `no default — blocks until answered`.
func TestNoDefaultBlocksUntilAnswered(t *testing.T) {
	withDef := mkItem("rotate the App token scope",
		"desk-recommendation: rotate on the next release\ndefault-if-no-answer: rotate anyway")
	without := mkItem("approve the subscription cost", "")
	without.Number = 303
	c := &collection{
		Scope: []string{"example-org/tracker"},
		Items: []*item{withDef, without},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true}},
	}
	body := build(c, signOff{State: signOffUnsigned}, fixedNow, "2026-W33").Render()
	if !strings.Contains(body, "no default — blocks until answered") {
		t.Errorf("an human-only item with no stated default must render the blocking literal:\n%s", body)
	}
	if !strings.Contains(body, "rotate anyway — applies 2026-08-27") {
		t.Errorf("a stated default must render with the 14-day date (2026-08-27):\n%s", body)
	}
	if !strings.Contains(body, "none on file — the desk owes one") {
		t.Errorf("an absent recommendation must render as a debt, not a blank cell:\n%s", body)
	}
}

// TestReadGapsAreRendered — the third state at repo granularity. A repo that did not
// answer and a label that does not exist are DIFFERENT findings, and neither may read as
// "zero items".
func TestReadGapsAreRendered(t *testing.T) {
	c := &collection{
		Scope: []string{"example-org/tracker", "example-org/agents"},
		Repos: []repoRead{
			{Repo: "example-org/tracker", LabelSetRead: true, MissingLabels: []string{humanOnlyLabel}},
			{Repo: "example-org/agents", Err: "issue list failed for needs-decision: HTTP 403"},
		},
	}
	body := build(c, signOff{State: signOffUnsigned}, fixedNow, "2026-W33").Render()
	if !strings.Contains(body, "does not exist in this repo") {
		t.Errorf("a missing label must be reported as missing, not as an empty queue:\n%s", body)
	}
	if !strings.Contains(body, "**did not answer**") || !strings.Contains(body, "are UNKNOWN") {
		t.Errorf("an unanswered repo must be reported as unknown, not as zero:\n%s", body)
	}
	if !c.Partial() {
		t.Error("a collection with an unanswered repo must report Partial()")
	}
}

// TestCoverageAccountingIsStated — the review gate's property ("no item can silently
// drop out of the queue") made checkable from the artifact: collected and rendered counts
// are printed and must agree.
func TestCoverageAccountingIsStated(t *testing.T) {
	items := []*item{mkItem("a", ""), mkItem("b", ""), mkItem("c", "")}
	for i, it := range items {
		it.Number = 400 + i
	}
	c := &collection{Scope: []string{"example-org/tracker"}, Items: items,
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true, Items: 3}}}
	d := build(c, signOff{State: signOffUnsigned}, fixedNow, "2026-W33")
	body := d.Render()
	if !strings.Contains(body, "items collected: 3 · rows rendered: 3") {
		t.Errorf("the coverage line must state both counts:\n%s", body)
	}
	if strings.Contains(body, "COVERAGE DEFECT") {
		t.Errorf("a matched collection reported a coverage defect:\n%s", body)
	}
	for i := range items {
		if !strings.Contains(body, fmt.Sprintf("#%d", 400+i)) {
			t.Errorf("collected item #%d did not render", 400+i)
		}
	}
}

// TestR3UnsignedIsStated — R-3's standing is reported from the register's CLAIM, and an
// unsigned R-3 means the reversible column is a proposal, not a delegation. Reporting it
// the other way would put a false authority statement in front of the one person the
// digest exists to inform.
func TestR3UnsignedIsStated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rulings.md")
	if err := os.WriteFile(path, []byte("## R-3 Reversible-decision default\n\n**Sign-off:** _(empty)_\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	so := readR3SignOff(path)
	if so.State != signOffUnsigned {
		t.Fatalf("state = %q, want unsigned", so.State)
	}
	c := &collection{Scope: []string{"example-org/tracker"},
		Items: []*item{mkItem("fix the docs wording", "")},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true, Items: 1}}}
	body := build(c, so, fixedNow, "2026-W33").Render()
	if !strings.Contains(body, "**R-3 is UNSIGNED.**") {
		t.Errorf("the unsigned standing must be stated:\n%s", body)
	}
	if !strings.Contains(body, "no default — R-3 unsigned, so the desk cannot decide it either") {
		t.Errorf("a reversible row under an unsigned R-3 must not offer a desk decision:\n%s", body)
	}
	t.Run("missing register is could-not-read", func(t *testing.T) {
		if got := readR3SignOff(filepath.Join(dir, "nope.md")); got.State != signOffUnknown {
			t.Fatalf("state = %q, want could-not-read", got.State)
		}
	})
	t.Run("a later signed ruling is not R-3's", func(t *testing.T) {
		p2 := filepath.Join(dir, "two.md")
		body := "## R-3 Reversible-decision default\n\n**Sign-off:** _(empty)_\n\n" +
			"## R-4 Drain-weighting\n\n**Sign-off:** https://example.invalid/c#issuecomment-1\n"
		if err := os.WriteFile(p2, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readR3SignOff(p2); got.State != signOffUnsigned {
			t.Fatalf("state = %q — R-4's sign-off was read as R-3's", got.State)
		}
	})
}

// TestR3SignedOnContinuationLine — FAIL-FIRST for the live register format. The rulings
// register writes each ruling's acceptance URL on the line BENEATH the
// `**Sign-off:**` label, not on the label itself. A label-only read reports a SIGNED R-3 as
// unsigned — the inverted mirror of the false-authority failure rulings.go forbids: a
// POSITIVE "unsigned" against a register that is in fact signed. The parser must read the
// continuation-line URL as a claim, and the digest must report the artifact, not deny it.
func TestR3SignedOnContinuationLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rulings.md")
	// The exact shape of the live register: the URL is on the
	// continuation line beneath the label, and a later ruling is still unsigned.
	reg := "## R-3 Reversible-decision default\n\n" +
		"**Sign-off:** approver, 2026-08-13 — approved:\n" +
		"https://example.invalid/pull/931#issuecomment-1\n\n" +
		"## R-4 Drain-weighting\n\n**Sign-off:** _(empty)_\n"
	if err := os.WriteFile(path, []byte(reg), 0o600); err != nil {
		t.Fatal(err)
	}
	so := readR3SignOff(path)
	if so.State != signOffClaimed {
		t.Fatalf("state = %q, want claimed — the sign-off URL sits on the continuation line", so.State)
	}
	if so.URL != "https://example.invalid/pull/931#issuecomment-1" {
		t.Fatalf("url = %q, want the continuation-line URL", so.URL)
	}
	c := &collection{Scope: []string{"example-org/tracker"},
		Items: []*item{mkItem("fix the docs wording", "")},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true, Items: 1}}}
	body := build(c, so, fixedNow, "2026-W33").Render()
	if strings.Contains(body, "**R-3 is UNSIGNED.**") {
		t.Errorf("a signed R-3 must not be reported as unsigned:\n%s", body)
	}
	if !strings.Contains(body, "records a sign-off artifact") || !strings.Contains(body, so.URL) {
		t.Errorf("the sign-off claim and its URL must be reported:\n%s", body)
	}
}

// TestR3DecisionsAreReadNotWritten — the digest is R-3's veto surface, so it lists the
// desk decisions it can FIND with their deadlines. An untrusted comment claiming a desk
// decision is not one.
func TestR3DecisionsAreReadNotWritten(t *testing.T) {
	it := mkItem("fix the docs wording", "")
	taken := fixedNow.AddDate(0, 0, -2)
	it.Comments = []comment{
		{AuthorLogin: "shared-agent", AuthorID: 2002, AuthorType: "User", CreatedAt: taken,
			Body: "<!-- desk-r3-decision v1 -->\ndecision: keep the current wording"},
		{AuthorLogin: "drive-by", AuthorID: 909090, AuthorType: "User", CreatedAt: taken,
			Body: "<!-- desk-r3-decision v1 -->\ndecision: something nobody authorised"},
	}
	c := &collection{Scope: []string{"example-org/tracker"}, Items: []*item{it},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true, Items: 1}}}
	d := build(c, signOff{State: signOffUnsigned}, fixedNow, "2026-W33")
	if len(d.Decisions) != 1 {
		t.Fatalf("decisions = %d, want 1 (the untrusted marker must not count)", len(d.Decisions))
	}
	if got := d.Decisions[0].VetoEnds.Format("2006-01-02"); got != "2026-08-18" {
		t.Errorf("veto deadline = %s, want 2026-08-18 (taken + %d days)", got, vetoWindowDays)
	}
	body := d.Render()
	if !strings.Contains(body, "keep the current wording") || strings.Contains(body, "nobody authorised") {
		t.Errorf("the veto table listed the wrong decisions:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// the gh seam
// ---------------------------------------------------------------------------

// ghScript is a scripted gh. It records every argv so the boundary tests can assert on
// what was CONSTRUCTED rather than on what a mock believed.
type ghScript struct {
	calls   [][]string
	labels  []string
	issues  map[string][]ghIssue // label -> issues, per any repo
	threads map[int][]ghComment
	search  []existing
	created string
	failIn  map[string]string // substring of argv -> error message
	// bodies is what was actually staged for the remote, captured AT CALL TIME because
	// the temp body file is removed when the run returns. Asserting on the posted body
	// rather than on the renderer's return value is the difference between "the digest
	// can say it is empty" and "the digest that was published says it is empty".
	bodies map[string]string
}

func (g *ghScript) install(t *testing.T) {
	t.Helper()
	prev := runGH
	g.bodies = map[string]string{}
	runGH = func(args ...string) (string, error) {
		g.calls = append(g.calls, append([]string(nil), args...))
		for i, a := range args {
			if a == "--body-file" && i+1 < len(args) {
				if b, rerr := os.ReadFile(args[i+1]); rerr == nil {
					g.bodies[args[1]] = string(b)
				}
			}
		}
		joined := strings.Join(args, " ")
		for frag, msg := range g.failIn {
			if strings.Contains(joined, frag) {
				return "", fmt.Errorf("%s", msg)
			}
		}
		switch {
		case len(args) > 1 && args[0] == "label" && args[1] == "list":
			var out []map[string]string
			for _, l := range g.labels {
				out = append(out, map[string]string{"name": l})
			}
			return mustJSON(out), nil
		case args[0] == "api" && strings.Contains(joined, "/comments"):
			n := numAfter(joined, "/issues/")
			return mustJSON(g.threads[n]), nil
		case args[0] == "api" && strings.Contains(joined, "/issues?"):
			label := afterEquals(joined, "labels=")
			return mustJSON(g.issues[label]), nil
		case args[0] == "issue" && args[1] == "list":
			return mustJSON(g.search), nil
		case args[0] == "issue" && args[1] == "create":
			return g.created, nil
		case args[0] == "issue" && args[1] == "edit":
			return "", nil
		}
		return "[]", nil
	}
	t.Cleanup(func() { runGH = prev })
}

// argvsMatching returns the calls whose gh SUB-VERB is verb. It reads position 1 rather
// than scanning for the word anywhere, so a search string that happens to contain
// "create" cannot be counted as a create.
func (g *ghScript) argvsMatching(verb string) [][]string {
	var out [][]string
	for _, c := range g.calls {
		if len(c) >= 2 && c[1] == verb {
			out = append(out, c)
		}
	}
	return out
}

func mustJSON(v any) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func numAfter(s, sep string) int {
	i := strings.Index(s, sep)
	if i < 0 {
		return 0
	}
	rest := s[i+len(sep):]
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func afterEquals(s, key string) string {
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key):]
	for j, r := range rest {
		if r == ' ' || r == '&' {
			return rest[:j]
		}
	}
	return rest
}

// oneIssue is the canonical scripted issue.
func oneIssue(n int, title, body string) ghIssue {
	i := ghIssue{Number: n, Title: title, Body: body, HTMLURL: "https://example.invalid/i/" + fmt.Sprint(n),
		CreatedAt: fixedNow.AddDate(0, 0, -20).Format(time.RFC3339)}
	i.User.Login, i.User.ID, i.User.Type = blessLogin, blessID, "User"
	i.Labels = []struct {
		Name string `json:"name"`
	}{{Name: needsDecisionLabel}}
	return i
}

// ---------------------------------------------------------------------------
// weekly lifecycle
// ---------------------------------------------------------------------------

// TestWeeklyLifecycleUpdate — a second run in the SAME week edits the existing issue in
// place and creates nothing. Without this the human accumulates one digest per run and
// the batching the tool exists for is undone by the tool.
func TestWeeklyLifecycleUpdate(t *testing.T) {
	g := &ghScript{
		labels: []string{needsDecisionLabel, humanOnlyLabel},
		issues: map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "fix the docs wording", "")}},
		search: []existing{{Number: 77, Title: "Decision digest 2026-W33", State: "OPEN"}},
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker", "--now", fixedNow.Format(time.RFC3339), "--week", "2026-W33",
		"--rulings", writeRulings(t)}, &out, &errb)
	if err != nil {
		t.Fatalf("dispatch: %v (stderr: %s)", err, errb.String())
	}
	if n := len(g.argvsMatching("create")); n != 0 {
		t.Errorf("an in-week re-run created %d issues; it must update in place", n)
	}
	edits := g.argvsMatching("edit")
	if len(edits) != 1 {
		t.Fatalf("edits = %d, want exactly 1", len(edits))
	}
	if !strings.Contains(strings.Join(edits[0], " "), " 77 ") {
		t.Errorf("the edit did not target the existing digest #77: %v", edits[0])
	}
	if !strings.Contains(out.String(), "updated medici-finance/assay#77") {
		t.Errorf("stdout did not report the in-place update: %s", out.String())
	}
}

// TestWeeklyLifecycleNewWeek — a new ISO week is a new title, so nothing matches and a
// NEW issue is created. Last week's digest is left alone: superseding it is a close, and
// closure belongs to deskclose.
func TestWeeklyLifecycleNewWeek(t *testing.T) {
	g := &ghScript{
		labels:  []string{needsDecisionLabel, humanOnlyLabel},
		issues:  map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "fix the docs wording", "")}},
		search:  []existing{{Number: 77, Title: "Decision digest 2026-W33", State: "OPEN"}},
		created: "https://example.invalid/i/88",
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker", "--now", fixedNow.AddDate(0, 0, 7).Format(time.RFC3339),
		"--rulings", writeRulings(t)}, &out, &errb)
	if err != nil {
		t.Fatalf("dispatch: %v (stderr: %s)", err, errb.String())
	}
	creates := g.argvsMatching("create")
	if len(creates) != 1 {
		t.Fatalf("creates = %d, want 1 for a new week", len(creates))
	}
	if !strings.Contains(strings.Join(creates[0], " "), "Decision digest 2026-W34") {
		t.Errorf("the new week's title is wrong: %v", creates[0])
	}
	if n := len(g.argvsMatching("edit")); n != 0 {
		t.Errorf("a new week edited %d existing digests; last week's record is not ours to rewrite", n)
	}
	if n := len(g.argvsMatching("close")); n != 0 {
		t.Errorf("deskdigest constructed %d close argvs — closure belongs to deskclose", n)
	}
	if !strings.Contains(errb.String(), "deskclose superseded") {
		t.Errorf("the supersede hint must be PRINTED for a human to run: %s", errb.String())
	}
}

// TestWeeklyLifecycleEmpty — THE INVARIANT. A week with zero items still posts a digest,
// and the posted BODY says it is zero and names what was read. A skipped empty week is
// indistinguishable from a crashed generator, which is the failure mode this tool exists
// to remove.
func TestWeeklyLifecycleEmpty(t *testing.T) {
	g := &ghScript{
		labels:  []string{needsDecisionLabel, humanOnlyLabel},
		issues:  map[string][]ghIssue{},
		created: "https://example.invalid/i/90",
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker,example-org/agents",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb)
	if err != nil {
		t.Fatalf("an empty week must POST, not skip; got: %v (stderr %s)", err, errb.String())
	}
	if n := len(g.argvsMatching("create")); n != 1 {
		t.Fatalf("creates = %d, want 1 — an empty queue is a report, not a skip", n)
	}
	body := postedBody(t, g, "create")
	for _, want := range []string{"**0 items.**", "failed to run", "example-org/tracker", "example-org/agents"} {
		if !strings.Contains(body, want) {
			t.Errorf("the POSTED empty-week body is missing %q; body:\n%s", want, body)
		}
	}
}

// TestWeeklyLifecycleClosed — a CLOSED digest for the current week is refused, not
// reopened. Whether the week is still live is a judgement, and this tool renders reports
// rather than making them.
func TestWeeklyLifecycleClosed(t *testing.T) {
	g := &ghScript{
		labels: []string{needsDecisionLabel, humanOnlyLabel},
		search: []existing{{Number: 77, Title: "Decision digest 2026-W33", State: "CLOSED"}},
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker", "--now", fixedNow.Format(time.RFC3339),
		"--week", "2026-W33", "--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 (refused)", deskkit.ExitCodeOf(err))
	}
	if n := len(g.argvsMatching("reopen")); n != 0 {
		t.Errorf("deskdigest constructed a reopen argv")
	}
}

// TestWeeklyLifecycleAmbiguous — two issues with the identical title is a duplicate
// somebody made; picking one would silently abandon whatever is on the other.
func TestWeeklyLifecycleAmbiguous(t *testing.T) {
	g := &ghScript{
		labels: []string{needsDecisionLabel, humanOnlyLabel},
		search: []existing{
			{Number: 77, Title: "Decision digest 2026-W33", State: "OPEN"},
			{Number: 78, Title: "Decision digest 2026-W33", State: "OPEN"},
		},
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker", "--now", fixedNow.Format(time.RFC3339),
		"--week", "2026-W33", "--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 (refused)", deskkit.ExitCodeOf(err))
	}
	if n := len(g.argvsMatching("edit")) + len(g.argvsMatching("create")); n != 0 {
		t.Errorf("an ambiguous match still wrote %d times", n)
	}
}

// TestPartialReadStillPostsAndSays — a repo that did not answer produces a digest that
// NAMES the gap and an exit 6 that carries the same news. A partial digest that admits it
// is partial beats a silent week.
func TestPartialReadStillPostsAndSays(t *testing.T) {
	g := &ghScript{
		labels:  []string{needsDecisionLabel, humanOnlyLabel},
		created: "https://example.invalid/i/91",
		failIn:  map[string]string{"example-org/agents/issues?": "HTTP 403"},
	}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay",
		"--repos", "example-org/tracker,example-org/agents",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want 6 (partial read)", deskkit.ExitCodeOf(err))
	}
	if n := len(g.argvsMatching("create")); n != 1 {
		t.Fatalf("creates = %d — a partial read must still publish a digest that names the gap", n)
	}
	body := postedBody(t, g, "create")
	if !strings.Contains(body, "**did not answer**") || !strings.Contains(body, "example-org/agents") {
		t.Errorf("the posted body does not name the unreadable repo:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// boundary: the digest reports, and only reports
// ---------------------------------------------------------------------------

// TestReportOnlyNeverMutates — every argv the NON-post path constructs is inspected, and
// any mutating gh verb fails the test. This is what makes "the digest reports" a property
// of the binary rather than a claim in a comment.
func TestReportOnlyNeverMutates(t *testing.T) {
	g := &ghScript{
		labels: []string{needsDecisionLabel, humanOnlyLabel},
		issues: map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "rotate a token", "")}},
	}
	g.install(t)
	var out, errb bytes.Buffer
	if err := dispatch([]string{"--dry-run", "--repos", "example-org/tracker",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(g.calls) == 0 {
		t.Fatal("the dry run made no gh calls at all — the assertion below would be vacuous")
	}
	for _, argv := range g.calls {
		if verb, bad := ghArgvMutates(argv); bad {
			t.Errorf("a read path constructed a mutating gh call (%s): %v", verb, argv)
		}
	}
	if !strings.Contains(out.String(), "# Decision digest") {
		t.Errorf("--dry-run printed no digest: %s", out.String())
	}
}

// TestDryRunAndPostExclusive — --dry-run is a promise not to write, and a promise that
// can be paired with the write flag is not one.
func TestDryRunAndPostExclusive(t *testing.T) {
	var out, errb bytes.Buffer
	err := dispatch([]string{"--dry-run", "--post", "--digest-repo", "medici-finance/assay"}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5", deskkit.ExitCodeOf(err))
	}
}

// TestPostGatesOnTheRepoSet — the write boundary is deskkit's. deskdigest introduces no
// repo list of its own and refuses a target outside the set, and refuses to default one.
func TestPostGatesOnTheRepoSet(t *testing.T) {
	var out, errb bytes.Buffer
	if err := dispatch([]string{"--post", "--digest-repo", "example-org/not-in-the-set"}, &out, &errb); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 for a repo outside the set", deskkit.ExitCodeOf(err))
	}
	out.Reset()
	errb.Reset()
	if err := dispatch([]string{"--post"}, &out, &errb); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 for --post with no --digest-repo", deskkit.ExitCodeOf(err))
	}
}

// TestEmptyScopeIsNotAnEmptyQueue — a read scope that resolves to nothing is a
// configuration finding, not a clean week. It renders, and it exits 6.
func TestEmptyScopeIsNotAnEmptyQueue(t *testing.T) {
	g := &ghScript{}
	g.install(t)
	var out, errb bytes.Buffer
	err := dispatch([]string{"--dry-run", "--repos", " , ", "--now", fixedNow.Format(time.RFC3339),
		"--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want 6 for an empty read scope", deskkit.ExitCodeOf(err))
	}
	if !strings.Contains(out.String(), "read scope resolved EMPTY") {
		t.Errorf("the rendered digest must say the scope was empty:\n%s", out.String())
	}
}

// TestVersionAndHelpAreQuiet — --version and --help do not touch the kill switch, the
// audit log or the network.
func TestVersionAndHelpAreQuiet(t *testing.T) {
	g := &ghScript{}
	g.install(t)
	var out, errb bytes.Buffer
	if rc := run([]string{"--version"}, &out, &errb); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d", rc)
	}
	if !strings.Contains(out.String(), "deskdigest sourceSHA=") {
		t.Errorf("--version output: %s", out.String())
	}
	if rc := run([]string{"--help"}, &out, &errb); rc != deskkit.ExitOK {
		t.Fatalf("--help rc = %d", rc)
	}
	if len(g.calls) != 0 {
		t.Errorf("--version/--help made %d gh calls", len(g.calls))
	}
}

// TestReversibleUnderClaimedR3RendersDeskMayDecide — the single highest-consequence cell
// the tool prints. When R-3 records a sign-off (claimed) and a row is reversible,
// defaultCell renders "desk may decide under R-3 …" — the sentence that tells the human the
// desk may decide their item without them. Against the live register that arm was
// unreachable while R-3 was unsigned; now that the parser reads the claim it is the
// production rendering for every reversible item, so it gets a test that is RED before the
// claim is read and GREEN after. The complementary UNSIGNED control is what makes this a
// property of the claimed STATE and not of the row.
func TestReversibleUnderClaimedR3RendersDeskMayDecide(t *testing.T) {
	c := &collection{Scope: []string{"example-org/tracker"},
		Items: []*item{mkItem("Fix the docs wording in the intake README", "no other signal here")},
		Repos: []repoRead{{Repo: "example-org/tracker", LabelSetRead: true, Items: 1}}}
	want := fmt.Sprintf("desk may decide under %s; %d-day veto once recorded here", r3ID, vetoWindowDays)

	// Guard the fixture: the row must actually be reversible, or the cell below is tested
	// against the wrong class and the assertion is vacuous.
	if v := classifyItem(c.Items[0]); v.Class != classReversible {
		t.Fatalf("fixture item classified %q, want reversible — the cell under test never renders", v.Class)
	}

	claimed := signOff{State: signOffClaimed, URL: "https://example.invalid/pull/931#issuecomment-1"}
	if body := build(c, claimed, fixedNow, "2026-W33").Render(); !strings.Contains(body, want) {
		t.Fatalf("a reversible row under a claimed R-3 must render the desk-decides default %q:\n%s", want, body)
	}

	unsigned := signOff{State: signOffUnsigned}
	if body := build(c, unsigned, fixedNow, "2026-W33").Render(); strings.Contains(body, want) {
		t.Fatalf("a reversible row under an UNSIGNED R-3 must not offer a desk decision:\n%s", body)
	}
}

// TestKillSwitchDisabledMakesNoCall — the kill switch is load-bearing: it is what stands
// between a scheduled weekly job and a repo the operator has stopped. Every other command in
// tools/desk/cmd has this test. deskdigest's Guard() sits in run() ABOVE dispatch, so a test
// that only ever calls dispatch (as every behavioural test here does) never reaches it. This
// one goes through run() with the switch armed and asserts exit 3 and zero gh calls, so the
// next refactor that deletes the Guard() block reddens the suite rather than shipping green.
func TestKillSwitchDisabledMakesNoCall(t *testing.T) {
	g := &ghScript{labels: []string{needsDecisionLabel, humanOnlyLabel},
		issues: map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "fix the docs wording", "")}}}
	g.install(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	var out, errb bytes.Buffer
	// A scope IS named, so absent the guard this run would reach the network — which is
	// exactly what makes the zero-call assertion prove the guard, not the arguments.
	rc := run([]string{"--dry-run", "--repos", "example-org/tracker",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb)
	if rc != deskkit.ExitDisabled {
		t.Fatalf("armed kill switch rc = %d, want 3", rc)
	}
	if len(g.calls) != 0 {
		t.Fatalf("a gh call was made while the kill switch was armed: %v", g.calls)
	}
}

// TestWriteMeterGatesTheCreate — the outward-write meter is consulted before the one write
// this tool performs, and a refusal from it stops the create. Stubbing the meter to error
// and asserting zero `gh issue create` calls makes deletion of the gate a red test: without
// the gate the create runs despite the refusal. The complement of the kill-switch test — the
// meter is the per-repo budget the guard defers to once the switch is disarmed.
func TestWriteMeterGatesTheCreate(t *testing.T) {
	g := &ghScript{labels: []string{needsDecisionLabel, humanOnlyLabel},
		issues:  map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "fix the docs wording", "")}},
		created: "https://example.invalid/i/91"}
	g.install(t)

	prev := allowWriteRepoWide
	allowWriteRepoWide = func(string) error { return deskkit.RateLimited("refused: write budget spent") }
	t.Cleanup(func() { allowWriteRepoWide = prev })

	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay", "--repos", "example-org/tracker",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb)
	if err == nil {
		t.Fatal("a spent write meter must stop the create, but dispatch returned nil")
	}
	if n := len(g.argvsMatching("create")); n != 0 {
		t.Fatalf("creates = %d — the write meter gate was not consulted before the create", n)
	}
}

// TestBodyCheckGatesTheWrite — the body scanner runs before the digest can reach the remote,
// and a rejection stops the write. Stubbing it to error and asserting no create/edit call
// makes deletion of the BodyCheck call a red test. The stub stands in for a real rejection so
// the test needs no secret-shaped fixture of its own.
func TestBodyCheckGatesTheWrite(t *testing.T) {
	g := &ghScript{labels: []string{needsDecisionLabel, humanOnlyLabel},
		issues:  map[string][]ghIssue{needsDecisionLabel: {oneIssue(11, "fix the docs wording", "")}},
		created: "https://example.invalid/i/91"}
	g.install(t)

	prev := bodyCheck
	bodyCheck = func([]byte) error { return deskkit.Refused("refused: body failed the scan") }
	t.Cleanup(func() { bodyCheck = prev })

	var out, errb bytes.Buffer
	err := dispatch([]string{"--post", "--digest-repo", "medici-finance/assay", "--repos", "example-org/tracker",
		"--now", fixedNow.Format(time.RFC3339), "--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 — a failed body scan must refuse the write", deskkit.ExitCodeOf(err))
	}
	if n := len(g.argvsMatching("create")) + len(g.argvsMatching("edit")); n != 0 {
		t.Fatalf("write calls = %d — the body scan gate was not consulted before the write", n)
	}
}

// TestBareInvocationRefusesUnaimedScope — a run that names no read scope must not silently
// fan a live read across the roster's whole scan scope. It is refused (exit 5) BEFORE any
// network read, and --all-repos is the explicit opt-in that a caller who DOES want the whole
// scope reaches for. This is the read-side mirror of --post refusing to default its write
// target, and it closes the gap that TestReportOnlyNeverMutates left open by always passing
// an explicit --repos.
func TestBareInvocationRefusesUnaimedScope(t *testing.T) {
	g := &ghScript{}
	g.install(t)
	var out, errb bytes.Buffer

	err := dispatch([]string{"--dry-run", "--now", fixedNow.Format(time.RFC3339),
		"--rulings", writeRulings(t)}, &out, &errb)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5 for a run that named no read scope", deskkit.ExitCodeOf(err))
	}
	if len(g.calls) != 0 {
		t.Fatalf("a scope-less run reached the network before refusing: %v", g.calls)
	}

	// --all-repos is the explicit opt-in and must NOT be refused for want of a scope.
	out.Reset()
	errb.Reset()
	if err := dispatch([]string{"--dry-run", "--all-repos", "--now", fixedNow.Format(time.RFC3339),
		"--rulings", writeRulings(t)}, &out, &errb); deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
		t.Fatalf("--all-repos is an explicit scope and must not be refused for want of one: %v", err)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeRulings plants an UNSIGNED R-3 register — a fixture that exercises the unsigned
// rendering path deterministically. It is NOT the live register: on main R-3 is signed with
// the URL on the continuation line (see TestR3SignedOnContinuationLine); this fixture just
// pins the unsigned branch so the lifecycle tests do not depend on the register's real state.
func writeRulings(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rulings.md")
	if err := os.WriteFile(p, []byte("## R-3 Reversible-decision default\n\n**Sign-off:** _(empty)_\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// postedBody returns the body that was staged for a gh sub-verb ("create" / "edit"),
// captured at call time by ghScript because the temp file does not outlive the run.
func postedBody(t *testing.T, g *ghScript, verb string) string {
	t.Helper()
	b, ok := g.bodies[verb]
	if !ok {
		t.Fatalf("no body was staged for `gh issue %s`", verb)
	}
	return b
}
