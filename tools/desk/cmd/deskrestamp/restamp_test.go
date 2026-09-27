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

func labeledBy(name, who string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: who}
}

func unlabeledBy(name, who string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: who, Removed: true}
}

const (
	modelLabel = "dispatched-model:example-model-1"
	tierLabel  = "dispatched-tier:strong"
)

// --- The happy path: a foreign-applied stamp is removed and re-applied under the
// dispatcher, the removal is its OWN write ahead of the application, and the record
// comment names both actors.
func TestReStampForeignAppliedPair(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "some-human"),
			labeledBy(tierLabel, "some-human"),
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
	for _, want := range []string{"some-human", "desk", modelLabel, tierLabel} {
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
func TestReStampNoopWhenAlreadyStanding(t *testing.T) {
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
func TestReStampNoopUnderAllowanceLogin(t *testing.T) {
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
func TestReStampRefusesUnreadableContent(t *testing.T) {
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
func TestReStampRefusesNonDispatcherRole(t *testing.T) {
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
func TestReStampReviewerRoleProceeds(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "some-human"),
			labeledBy(tierLabel, "some-human"),
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
func TestReStampDryRun(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "some-human"),
			labeledBy(tierLabel, "some-human"),
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
	for _, want := range []string{"dry-run", "some-human", modelLabel} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run plan does not name %q:\n%s", want, out)
		}
	}
}

// A forge read failure is could-not-check (exit 6), never a blind write.
func TestReStampUnverifiableOnReadFailure(t *testing.T) {
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

// A re-stamp whose record comment fails to post is reported UNVERIFIABLE — the labels
// landed, the record did not, and the message says both rather than either alone.
func TestReStampCommentFailureIsLoud(t *testing.T) {
	fg := &fakeForge{
		pr: stampedPR(modelLabel, tierLabel),
		events: []deskkit.LabelEvent{
			labeledBy(modelLabel, "some-human"),
			labeledBy(tierLabel, "some-human"),
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
