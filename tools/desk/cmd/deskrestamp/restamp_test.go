package main

// restamp_test.go — the verb's suite.
//
// A RECORDING fake forge drives every case, so "zero forge calls before the role check",
// "the removal is its own write ahead of the application", and "the record comment names
// both actors" are asserted against the calls the fake saw rather than inferred from
// output.

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const allowedRepo = "example-org/tracker"

// fakeForge records every call. Embedding a nil deskkit.Forge means any method the verb
// was not expected to reach PANICS — which is the point: the verb touches GetPullRequest,
// ListLabelEvents, ApplyLabels and PostCommentTyped, and nothing else.
type fakeForge struct {
	deskkit.Forge
	pr         *deskkit.PullRequest
	prErr      error
	events     []deskkit.LabelEvent
	eventsErr  error
	applyErr   error
	commentErr error
	reads      []string
	writes     []deskkit.LabelChange
	comments   []string
}

func (f *fakeForge) GetPullRequest(fr deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	f.reads = append(f.reads, "GetPullRequest")
	if f.prErr != nil {
		return nil, f.prErr
	}
	pr := *f.pr
	pr.Number = n
	return &pr, nil
}

func (f *fakeForge) ListLabelEvents(fr deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	f.reads = append(f.reads, "ListLabelEvents")
	if f.eventsErr != nil {
		return nil, f.eventsErr
	}
	return f.events, nil
}

func (f *fakeForge) ApplyLabels(fr deskkit.ForgeRepo, n int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	f.writes = append(f.writes, change)
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	out := &deskkit.LabelOutcome{}
	for _, l := range change.Add {
		out.Added = append(out.Added, l.Name)
	}
	out.Removed = append(out.Removed, change.Remove...)
	return out, nil
}

func (f *fakeForge) PostCommentTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind, body string) (*deskkit.CommentRef, error) {
	f.comments = append(f.comments, body)
	if f.commentErr != nil {
		return nil, f.commentErr
	}
	return &deskkit.CommentRef{ID: "c1"}, nil
}

func (f *fakeForge) calls() int { return len(f.reads) + len(f.writes) + len(f.comments) }

// plantWorld isolates HOME (roster fixture, audit log), pins the clock, and fixes the
// session role and the forge the verb resolves.
func plantWorld(t *testing.T, role string, fg deskkit.Forge) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskrestamp-test")
	t.Setenv("DESK_LOOP", "")

	oldRole, oldForge, oldNow := roleFn, forgeForFn, nowFunc
	oldMinted, oldToken := mintedRole, ghToken
	roleFn = func() (string, error) { return role, nil }
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return fg, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	nowFunc = func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }
	mintedRole, ghToken = "", ""
	t.Cleanup(func() {
		roleFn, forgeForFn, nowFunc = oldRole, oldForge, oldNow
		mintedRole, ghToken = oldMinted, oldToken
	})
}

func runVerb(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	err := cmdReStamp(args, &buf)
	out := buf.String()
	if err != nil {
		out += "\nERR: " + err.Error()
	}
	return deskkit.ExitCodeOf(err), out
}

func stampedPR(labels ...string) *deskkit.PullRequest {
	return &deskkit.PullRequest{State: "open", Draft: true, Labels: labels}
}

// preCutoffRFC3339 is strictly BEFORE deskkit.RestampDriverCutoff (the driver's ruling on
// PR #1727, comment 5860170351: "before 2026-09-27T00:00:00Z, the #336 legacy backlog").
// labeledBy defaults every event to this timestamp, so a pre-existing happy-path test
// (applier "ada", the fixture's ASSAY_BLESS_LOGIN) keeps meaning what it always meant —
// a legacy, pre-ruling hand-applied label — without every call site naming a date. A test
// that needs to control WHEN a label was applied (the cutoff boundary, a half-swap
// timeline) uses labeledByAt instead.
const preCutoffRFC3339 = "2026-09-01T00:00:00Z"

func labeledBy(name, who string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: who, CreatedAt: preCutoffRFC3339}
}

func labeledByAt(name, who, createdAt string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: who, CreatedAt: createdAt}
}

func unlabeledBy(name, who string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: who, Removed: true}
}

const (
	modelLabel = "dispatched-model:example-model-1"
	tierLabel  = "dispatched-tier:strong"
)

// --- The happy path: a stamp standing under the DRIVER'S OWN login ("ada", the fixture
// roster's ASSAY_BLESS_LOGIN — every other trusted login is now refused, SEC-1b round 3)
// applied BEFORE the cutoff (labeledBy's default) is removed and re-applied under the
// dispatcher, the removal is its OWN write ahead of the application, and the record
// comment names both actors. This is the #336 legacy-backlog case the driver's ruling on PR
// #1727 (comment 5860170351) names: a pre-ruling, hand-applied label under the driver's
// own login.
func TestRestampForeignAppliedPair(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "ada"),
			labeledBy(tierLabel, "ada"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	if len(fg.writes) != 2 {
		t.Fatalf("writes = %d, want 2 (remove, then apply): %+v", len(fg.writes), fg.writes)
	}
	rm, add := fg.writes[0], fg.writes[1]
	if len(rm.Remove) != 2 || len(rm.Add) != 0 {
		t.Fatalf("first write is not the pure removal of both halves: %+v", rm)
	}
	if len(add.Add) != 2 || len(add.Remove) != 0 {
		t.Fatalf("second write is not the pure application of the pair: %+v", add)
	}
	if rm.Target != deskkit.TargetChange || add.Target != deskkit.TargetChange {
		t.Fatal("a label write carried no explicit change target")
	}
	if len(fg.comments) != 1 {
		t.Fatalf("comments = %d, want exactly one record comment", len(fg.comments))
	}
	for _, want := range []string{"ada", "desk", modelLabel, tierLabel} {
		if !strings.Contains(fg.comments[0], want) {
			t.Fatalf("record comment does not name %q:\n%s", want, fg.comments[0])
		}
	}
	if !strings.Contains(out, "re-stamped") {
		t.Fatalf("report does not read as a re-stamp:\n%s", out)
	}
}

// An identical stamp already standing under the dispatcher is a no-op: no write, no
// comment, no budget charged.
func TestRestampSkipsStandingPair(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "assay-desk-app[bot]"),
			labeledBy(tierLabel, "assay-desk-app[bot]"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatalf("a no-op made %d writes and %d comments", len(fg.writes), len(fg.comments))
	}
	if !strings.Contains(out, "noop") {
		t.Fatalf("report does not read as a noop:\n%s", out)
	}
}

// A stamp already standing under a roster-allowed trusted login (#336's allowance) is
// also a no-op — the allowance the floor reads is the allowance this verb respects.
func TestRestampUnderAllowanceIsQuiet(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "shared-agent"),
			labeledBy(tierLabel, "shared-agent"),
		},
	}
	plantWorld(t, "desk", fg)
	// Arm the allowance for shared-agent (a trusted login of the fixture roster) — AFTER
	// plantWorld planted the base roster, so the append survives.
	rosterPath := os.Getenv("HOME") + "/.config/assay/roster.env"
	roster, _ := os.ReadFile(rosterPath)
	roster = append(roster, []byte("ASSAY_STAMP_TRUSTED_LOGINS=shared-agent:2002\n")...)
	if err := os.WriteFile(rosterPath, roster, 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	if len(fg.writes) != 0 {
		t.Fatalf("an allowance-standing stamp was churned: %+v", fg.writes)
	}
	if !strings.Contains(out, "noop") {
		t.Fatalf("report does not read as a noop:\n%s", out)
	}
}

// An UNREADABLE stamp is refused: this verb preserves content and never invents it — the
// corrupt-content repair is the dispatch ceremony's, which validates an explicit
// --model/--tier.
func TestRestampRefusesCorruptContent(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel, "dispatched-tier:any"),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "assay-desk-app[bot]"),
			labeledBy(tierLabel, "assay-desk-app[bot]"),
			labeledBy("dispatched-tier:any", "assay-desk-app[bot]"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "never invents") {
		t.Fatalf("refusal does not state the content rule:\n%s", out)
	}
}

// A session role that is not a bound dispatcher is refused BEFORE any forge call — a
// re-stamp under any other identity only mints a second unreadable stamp.
func TestRestampRefusesWorkerRole(t *testing.T) {
	fg := &fakeForge{pr: stampedPR(modelLabel, tierLabel)}
	plantWorld(t, "worker", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if fg.calls() != 0 {
		t.Fatalf("the role check ran after %d forge calls — it must run before any", fg.calls())
	}
	if !strings.Contains(out, "dispatching roles") {
		t.Fatalf("refusal does not name the dispatcher set:\n%s", out)
	}
}

// The reviewer role IS a dispatching role (it dispatches the review lane), so its
// re-stamp proceeds under its own App.
func TestRestampReviewerRoleProceeds(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "ada"),
			labeledBy(tierLabel, "ada"),
		},
	}
	plantWorld(t, "reviewer", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	if len(fg.writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(fg.writes))
	}
}

// --dry-run reads everything, writes nothing, and prints the plan including the previous
// applier.
func TestRestampDryRun(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "ada"),
			labeledBy(tierLabel, "ada"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a dry run wrote")
	}
	for _, want := range []string{"dry-run", "ada", modelLabel} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run plan does not name %q:\n%s", want, out)
		}
	}
}

// A forge read failure is could-not-check (exit 6), never a blind write.
func TestRestampReadFailureIsCouldNotCheck(t *testing.T) {
	fg := &fakeForge{prErr: errors.New("boom")}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 6 {
		t.Fatalf("exit = %d, want 6 (unverifiable):\n%s", code, out)
	}
	if len(fg.writes) != 0 {
		t.Fatal("a blind re-stamp wrote")
	}
}

// SEC-2: a label-HISTORY read failure (ListLabelEvents errors, distinct from the PR-read
// failure above) is ALSO could-not-check, never a blind re-stamp. With the history
// unreadable, every present label is unattributable, so proceeding would be SEC-1 with no
// applier information at all.
func TestRestampEventsReadFailureIsCouldNotCheck(t *testing.T) {
	fg := &fakeForge{
		pr:        stampedPR(modelLabel, tierLabel),
		eventsErr: errors.New("history boom"),
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 6 {
		t.Fatalf("exit = %d, want 6 (unverifiable):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a re-stamp over an unreadable label history wrote")
	}
}

// SEC-1/C4: a pair standing under an UNTRUSTED login is not this verb's to repair —
// re-attesting it under the dispatcher would be laundering, not repair. This is the
// PR's original (pre-fix) TestRestampForeignAppliedPair scenario, now pinned the other
// way: "some-human" is not the driver's own login (the ONLY login SEC-1b round 3 vouches
// for) — it is not even in the fixture roster's ASSAY_TRUSTED_LOGINS at all.
func TestRestampRefusesUntrustedForeignApplier(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "some-human"),
			labeledBy(tierLabel, "some-human"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "some-human") || !strings.Contains(out, "vouches only for") {
		t.Fatalf("refusal does not name the unvouched applier:\n%s", out)
	}
}

// SEC-1b round 3 (the driver's ruling on PR #1727): a TRUSTED login that is NOT the driver
// ("shared-agent", of the fixture roster's ASSAY_TRUSTED_LOGINS) is refused exactly like
// an untrusted one — trust alone is no longer enough; only the driver's own login
// qualifies. This is the case the earlier (reviewer-suggested, now superseded) "any
// trusted human" bar would have wrongly accepted.
func TestRestampRefusesNonDriverTrustedLogin(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "shared-agent"),
			labeledBy(tierLabel, "shared-agent"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "shared-agent") {
		t.Fatalf("refusal does not name the non-driver applier:\n%s", out)
	}
}

// SEC-1b round 3: the driver's OWN login ("ada", ASSAY_BLESS_LOGIN) applied AFTER the
// cutoff is refused too — the allowance is for the #336 legacy backlog, not a standing
// bypass for the driver's login going forward.
func TestRestampRefusesPostCutoffDriverApplication(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledByAt(modelLabel, "ada", "2026-09-27T00:00:01Z"),
			labeledByAt(tierLabel, "ada", "2026-09-27T00:00:01Z"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "ada") {
		t.Fatalf("refusal does not name the post-cutoff applier:\n%s", out)
	}
}

// A label applied exactly AT the cutoff instant is refused too — the ruling says
// "before" the cutoff, a strict bound, so the boundary instant itself does not qualify.
func TestRestampRefusesAtCutoffInstant(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledByAt(modelLabel, "ada", deskkit.RestampDriverCutoffRFC3339),
			labeledByAt(tierLabel, "ada", deskkit.RestampDriverCutoffRFC3339),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused, the bound is strict):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
}

// SEC-1a: a HALF-SWAP timeline — one half already standing under the dispatcher (so it
// needs no removal), the other re-applied by an unvouched identity — is refused on that
// one label alone. This is the mutant round 2 left surviving: a gate that only fires when
// EVERY half is foreign would wrongly pass this case, because only one of the two labels
// is in the removal set.
func TestRestampRefusesHalfSwapTimeline(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "assay-desk-app[bot]"), // already dispatcher-standing: NOT in remove
			labeledBy(tierLabel, "shared-agent"),         // foreign, non-driver: IS in remove
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused — a half-swap timeline is not this verb's to repair):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "shared-agent") {
		t.Fatalf("refusal does not name the swapped half's applier:\n%s", out)
	}
}

// A4 (round 3, C5's companion): a MIXED pair where BOTH labels are foreign to the
// dispatcher — unlike the half-swap case above, neither is already dispatcher-standing, so
// BOTH are in the removal set — and only ONE of the two vouches (blessing authority,
// pre-cutoff). remove is always sorted (modelLabel < tierLabel lexically), so this pair and
// its mirror below put the vouched half at EACH position in turn. Together they are the
// only way to distinguish "check every label in the removal set" from a mutant that checks
// only the first, only the last, or stops scanning at the first label that vouches:
//   - vouched at index 0 (this test): "only the first" and "stop at first vouched" both
//     wrongly clear the pair, because the second (unvouched) label is never reached;
//   - vouched at index 1 (the next test): "only the last" wrongly clears the pair, because
//     the first (unvouched) label is never reached.
func TestRestampRefusesMixedPairVouchedFirst(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "ada"),          // vouched: blessing authority, pre-cutoff (index 0)
			labeledBy(tierLabel, "shared-agent"),  // unvouched: trusted, but not the driver (index 1)
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused — the SECOND label in the removal set is unvouched):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "shared-agent") {
		t.Fatalf("refusal does not name the unvouched half's applier:\n%s", out)
	}
}

// The mirror of the test above: the unvouched half is now FIRST in the (sorted) removal
// set and the vouched half LAST, which is what a "check only the last label" mutant gets
// wrong.
func TestRestampRefusesMixedPairVouchedLast(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "shared-agent"), // unvouched: trusted, but not the driver (index 0)
			labeledBy(tierLabel, "ada"),            // vouched: blessing authority, pre-cutoff (index 1)
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused — the FIRST label in the removal set is unvouched):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "shared-agent") {
		t.Fatalf("refusal does not name the unvouched half's applier:\n%s", out)
	}
}

// A4 (round 3): the driver's login RE-APPLIES the same label after the cutoff, with NO
// `unlabeled` event in between — deskkit.StandingStampApplierAt must read this label's
// standing (CURRENT, latest) application as the post-cutoff one, not the pre-cutoff FIRST
// one. A mutant that keeps the first application instead of the latest would read the
// pre-cutoff event and wrongly vouch for a label whose actual standing application is
// post-cutoff.
func TestRestampRefusesReapplicationAfterCutoffOnStandingLabel(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledByAt(modelLabel, "ada", preCutoffRFC3339),       // FIRST application: pre-cutoff
			labeledByAt(modelLabel, "ada", "2026-09-27T00:00:01Z"), // RE-APPLIED, no unlabeled in between: this is now standing
			labeledBy(tierLabel, "ada"),                            // vouched on its own (pre-cutoff, unaffected)
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused — modelLabel's STANDING application is the post-cutoff re-application):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "ada") {
		t.Fatalf("refusal does not name the post-cutoff standing applier:\n%s", out)
	}
}

// A4 (round 3): the driver's login applies a label with NO timestamp at all (the forge
// reported none) — fail-closed, never fail-open. A mutant that ignores the parse error on
// an empty/unparseable CreatedAt would compare the zero time against the cutoff, and the
// zero time is before every real cutoff, so it would wrongly vouch.
func TestRestampRefusesDriverApplicationWithNoTimestamp(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			{Name: modelLabel, AppliedBy: "ada"}, // CreatedAt == "" — the forge named no timestamp
			labeledBy(tierLabel, "ada"),           // vouched on its own (pre-cutoff, unaffected)
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused — an unparseable/missing timestamp is fail-closed, not fail-open):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
}

// SEC-1: a pair standing under a bot/App identity (here, a worker App slug — not one of
// the dispatching roles) is refused the same way. Re-attesting a worker App's stamp under
// the dispatcher would let a non-reviewing identity's label clear the floor.
func TestRestampRefusesBotAppliedPair(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "assay-worker-app[bot]"),
			labeledBy(tierLabel, "assay-worker-app[bot]"),
		},
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
}

// SEC-1/SEC-2: a present WANT label the timeline carries NO standing event for at all
// (unattributable, as distinct from an events-read failure) is refused too — there is no
// applier to vouch for, so this is not a pair this verb repairs.
func TestRestampRefusesUnattributedWantLabel(t *testing.T) {
	fg := &fakeForge{
		pr:     stampedPR(modelLabel, tierLabel),
		events: nil, // no labeled event for either half: both read as unattributed
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused):\n%s", code, out)
	}
	if len(fg.writes) != 0 || len(fg.comments) != 0 {
		t.Fatal("a refused re-stamp still wrote")
	}
	if !strings.Contains(out, "does not name") {
		t.Fatalf("refusal does not name the unattributed placeholder:\n%s", out)
	}
}

// A re-stamp whose record comment fails to post is reported UNVERIFIABLE — the labels
// landed, the record did not, and the message says both rather than either alone.
func TestRestampCommentFailureIsLoud(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "ada"),
			labeledBy(tierLabel, "ada"),
		},
		commentErr: errors.New("comment boom"),
	}
	plantWorld(t, "desk", fg)
	code, out := runVerb(t, allowedRepo, "7")
	if code != 6 {
		t.Fatalf("exit = %d, want 6 (unverifiable):\n%s", code, out)
	}
	if len(fg.writes) != 2 {
		t.Fatal("the label writes should have landed before the comment failed")
	}
	if !strings.Contains(out, "LANDED") || !strings.Contains(out, "record comment") {
		t.Fatalf("the report does not state both halves (labels landed, record missing):\n%s", out)
	}
}
