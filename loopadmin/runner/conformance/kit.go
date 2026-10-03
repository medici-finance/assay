// Package conformance is the offline conformance kit for runner adapters. An
// adapter (the fake here, a real harness adapter later) supplies a Factory that
// returns a fresh Subject per case; the kit drives the contract's Client over it
// and asserts the outcomes the contract promises: an unknown launch needs
// reconcile, a fenced result is refused, credentials are rejected, missing
// usage stays unknown, the two modes share one set of checks, a cancel
// request is not a confirmed stop, terminal states are absorbing, refusals
// never echo their payload, decoding is strict, an unreported model is not the
// pinned model, a result must echo its own invocation's identity, an adapter's
// or fence's error never reaches an error's text, and nothing the caller or
// the adapter keeps a handle on can rewrite the Client's record.
package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// Control lets a case script the adapter's faults. A real adapter implements
// it over its own qualification fixtures; the fake implements it directly.
type Control interface {
	// LoseNextLaunchAck: the next Start begins the run but its acknowledgment
	// never arrives.
	LoseNextLaunchAck()
	// RejectNextLaunch: the next Start fails definitely and starts nothing.
	RejectNextLaunch()
	// IgnoreCancel: Cancel is acknowledged but the run keeps running.
	IgnoreCancel()
	// Complete drives a run to a finished observation.
	Complete(ref runner.Ref, res runner.Result, usage runner.Usage)
}

// ModelSwitcher is optional: an adapter that can be scripted to run a model
// other than the pinned one lets the kit check the no-silent-fallback rule.
type ModelSwitcher interface{ RunModelInstead(model string) }

// Subject is one adapter under test plus its fault control.
type Subject struct {
	Adapter runner.Adapter
	Control Control
}

// Factory returns a fresh Subject for one case.
type Factory func(t *testing.T) Subject

// Cases is every kit case, by name, in the order RunAll runs them.
var Cases = []struct {
	Name string
	Run  func(*testing.T, Factory)
}{
	{"UnknownLaunch", CaseUnknownLaunch},
	{"LateResultDenied", CaseLateResultDenied},
	{"CredentialAndUsage", CaseCredentialAndUsage},
	{"BothModes", CaseBothModes},
	{"MalformedResult", CaseMalformedResult},
	{"UnauthorizedTool", CaseUnauthorizedTool},
	{"CancelStillRunning", CaseCancelStillRunning},
	{"ResumeUnsupported", CaseResumeUnsupported},
	{"ModelFallback", CaseModelFallback},
	{"TerminalAbsorbs", CaseTerminalAbsorbs},
	{"CredentialShapes", CaseCredentialShapes},
	{"NoPayloadEcho", CaseNoPayloadEcho},
	{"StrictDecode", CaseStrictDecode},
	{"ModelUnreported", CaseModelUnreported},
	{"ResultIdentity", CaseResultIdentity},
	{"RequestRules", CaseRequestRules},
	{"ReconcileRules", CaseReconcileRules},
	{"NegativeUsage", CaseNegativeUsage},
	{"NoAdapterEcho", CaseNoAdapterEcho},
	{"NoAliasing", CaseNoAliasing},
}

// RunAll runs every case as a subtest against fresh subjects.
func RunAll(t *testing.T, f Factory) {
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) { c.Run(t, f) })
	}
}

func setup(t *testing.T, f Factory) (Subject, *Claims, *runner.Client) {
	t.Helper()
	s := f(t)
	claims := NewClaims()
	return s, claims, runner.NewClient(s.Adapter, claims)
}

func wantErr(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: got %v, want %v", what, err, want)
	}
}

func wantState(t *testing.T, what string, got, want runner.State) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: state %q, want %q", what, got, want)
	}
}

// CaseUnknownLaunch: a lost launch acknowledgment is an UNKNOWN outcome. It
// requires reconcile, cannot be retried or replaced concurrently, and only a
// definite failure frees the authority.
func CaseUnknownLaunch(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()
	req := StandingRequest("inv-1")
	claims.Set(req.Authority.Key, req.Authority.Generation)

	s.Control.LoseNextLaunchAck()
	ref, st, err := c.Start(ctx, req)
	wantErr(t, "lost acknowledgment", err, runner.ErrReconcileRequired)
	wantState(t, "lost acknowledgment", st, runner.StateUnknown)

	_, _, err = c.Start(ctx, req)
	wantErr(t, "retry of the unknown launch", err, runner.ErrReconcileRequired)
	replacement := StandingRequest("inv-2")
	_, _, err = c.Start(ctx, replacement)
	wantErr(t, "concurrent replacement under the same authority", err, runner.ErrReconcileRequired)
	if _, err := c.State(replacement.Ref()); !errors.Is(err, runner.ErrUnknownInvocation) {
		t.Fatalf("a refused replacement must leave no invocation behind, got %v", err)
	}

	obs, err := c.Reconcile(ctx, ref)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	wantState(t, "reconciled launch", obs.State, runner.StateRunning)
	_, _, err = c.Start(ctx, replacement)
	wantErr(t, "replacement while the reconciled launch still runs", err, runner.ErrReconcileRequired)

	// A definite failure, by contrast, frees the authority for a retry.
	s2, claims2, c2 := setup(t, f)
	claims2.Set(req.Authority.Key, req.Authority.Generation)
	s2.Control.RejectNextLaunch()
	_, st, err = c2.Start(ctx, StandingRequest("inv-3"))
	wantErr(t, "definite failure", err, runner.ErrDefiniteFailure)
	wantState(t, "definite failure", st, runner.StateFailed)
	if _, st, err = c2.Start(ctx, StandingRequest("inv-4")); err != nil || st != runner.StateRunning {
		t.Fatalf("a definite failure must free the authority: state %q err %v", st, err)
	}
}

// CaseLateResultDenied: a result from an attempt whose authority generation
// has moved on cannot be accepted, even when the result is well formed. A
// current attempt's identical result is accepted, so the refusal is the fence.
func CaseLateResultDenied(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()

	live := StandingRequest("live")
	claims.Set(live.Authority.Key, 1)
	liveRef, _, err := c.Start(ctx, live)
	if err != nil {
		t.Fatalf("start live: %v", err)
	}
	s.Control.Complete(liveRef, GoodResult(live), FullUsage())
	if _, err := c.Observe(ctx, liveRef); err != nil {
		t.Fatalf("observe live: %v", err)
	}
	if _, err := c.AcceptResult(ctx, liveRef); err != nil {
		t.Fatalf("a current, well-formed result must be accepted: %v", err)
	}

	late := StandingRequest("late")
	late.Authority.Key = "claim/desk-2"
	claims.Set(late.Authority.Key, 1)
	lateRef, _, err := c.Start(ctx, late)
	if err != nil {
		t.Fatalf("start late: %v", err)
	}
	claims.Set(late.Authority.Key, 2) // ownership moved on while the attempt ran
	s.Control.Complete(lateRef, GoodResult(late), FullUsage())
	if _, err := c.Observe(ctx, lateRef); err != nil {
		t.Fatalf("observe late: %v", err)
	}
	_, err = c.AcceptResult(ctx, lateRef)
	wantErr(t, "late output from a fenced attempt", err, runner.ErrFenced)

	// An adapter echoing a different generation is refused the same way.
	echo := StandingRequest("echo")
	echo.Authority.Key = "claim/desk-3"
	claims.Set(echo.Authority.Key, 1)
	echoRef, _, err := c.Start(ctx, echo)
	if err != nil {
		t.Fatalf("start echo: %v", err)
	}
	bad := GoodResult(echo)
	bad.Generation = 7
	s.Control.Complete(echoRef, bad, FullUsage())
	if _, err := c.Observe(ctx, echoRef); err != nil {
		t.Fatalf("observe echo: %v", err)
	}
	_, err = c.AcceptResult(ctx, echoRef)
	wantErr(t, "result echoing another generation", err, runner.ErrFenced)

	// The fence fails closed when the caller's record cannot be read.
	closed := runner.NewClient(s.Adapter, runner.FenceFunc(func(context.Context, string) (uint64, error) {
		return 0, errors.New("claim store unreachable")
	}))
	unread := StandingRequest("unread")
	unread.Authority.Key = "claim/desk-4"
	unreadRef, _, err := closed.Start(ctx, unread)
	if err != nil {
		t.Fatalf("start unread: %v", err)
	}
	s.Control.Complete(unreadRef, GoodResult(unread), FullUsage())
	if _, err := closed.Observe(ctx, unreadRef); err != nil {
		t.Fatalf("observe unread: %v", err)
	}
	_, err = closed.AcceptResult(ctx, unreadRef)
	wantErr(t, "unreadable authority generation", err, runner.ErrFenceUnavailable)
}

// CaseCredentialAndUsage: credential material is rejected in a request and in
// a result, and missing usage stays unknown through acceptance and totals.
func CaseCredentialAndUsage(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()
	secret := "gh" + "p_" + strings.Repeat("a1", 20)

	bad := StandingRequest("secret-packet")
	bad.Packet.Ref = "packets/worker?token=" + secret
	claims.Set(bad.Authority.Key, 1)
	_, _, err := c.Start(ctx, bad)
	wantErr(t, "credential in the packet", err, runner.ErrCredential)
	if _, err := c.State(bad.Ref()); !errors.Is(err, runner.ErrUnknownInvocation) {
		t.Fatalf("a refused request must not be recorded, got %v", err)
	}
	ext := StandingRequest("secret-ext")
	ext.Extensions = map[string]json.RawMessage{"api_key": json.RawMessage(`"x"`)}
	_, _, err = c.Start(ctx, ext)
	wantErr(t, "credential slot in an extension", err, runner.ErrCredential)

	a := StandingRequest("usage-a")
	a.Authority.Key = "claim/a"
	claims.Set(a.Authority.Key, 1)
	aRef, _, err := c.Start(ctx, a)
	if err != nil {
		t.Fatalf("start a: %v", err)
	}
	leak := GoodResult(a)
	leak.Summary = "found " + secret
	s.Control.Complete(aRef, leak, FullUsage())
	if _, err := c.Observe(ctx, aRef); err != nil {
		t.Fatalf("observe a: %v", err)
	}
	_, err = c.AcceptResult(ctx, aRef)
	wantErr(t, "credential in a result", err, runner.ErrCredential)

	// Missing cost: a measured figure survives, the missing one stays unknown.
	b := StandingRequest("usage-b")
	b.Authority.Key = "claim/b"
	claims.Set(b.Authority.Key, 1)
	bRef, _, err := c.Start(ctx, b)
	if err != nil {
		t.Fatalf("start b: %v", err)
	}
	s.Control.Complete(bRef, GoodResult(b), runner.Usage{InputTokens: runner.Int64(20), OutputTokens: runner.Int64(7)})
	if _, err := c.Observe(ctx, bRef); err != nil {
		t.Fatalf("observe b: %v", err)
	}
	acc, err := c.AcceptResult(ctx, bRef)
	if err != nil {
		t.Fatalf("accept b: %v", err)
	}
	if acc.UsageKnown || acc.Usage.CostMicros != nil {
		t.Fatalf("missing cost must stay unknown, got %+v known=%v", acc.Usage, acc.UsageKnown)
	}
	total := c.Usage(a.Caller)
	if total.CostMicros != nil || total.Complete() {
		t.Fatalf("a total over an unknown cost must be unknown, got %+v", total)
	}
	if s.Adapter.Capabilities().Telemetry == runner.TelemetryNone {
		if total.InputTokens != nil || acc.Usage.InputTokens != nil {
			t.Fatalf("an adapter without telemetry must leave usage unknown, got %+v", total)
		}
		return
	}
	if total.InputTokens == nil || *total.InputTokens != 30 {
		t.Fatalf("measured input tokens must sum, got %+v", total)
	}
}

// CaseBothModes: a standing-desk request needs no graph field, a
// workflow-stage request needs its canonical references, and one set of
// mandatory-capability checks refuses an unsupported requirement in both.
func CaseBothModes(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()

	desk := StandingRequest("desk-1")
	claims.Set(desk.Authority.Key, 1)
	if desk.Work != nil {
		t.Fatal("the standing fixture must carry no workflow reference")
	}
	deskRef, st, err := c.Start(ctx, desk)
	if err != nil || st != runner.StateRunning {
		t.Fatalf("standing-desk request with no graph fields: state %q err %v", st, err)
	}
	s.Control.Complete(deskRef, GoodResult(desk), FullUsage())
	if _, err := c.Observe(ctx, deskRef); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AcceptResult(ctx, deskRef); err != nil {
		t.Fatalf("standing-desk result: %v", err)
	}

	wf := WorkflowRequest("wf-1")
	claims.Set(wf.Authority.Key, 1)
	if _, st, err = c.Start(ctx, wf); err != nil || st != runner.StateRunning {
		t.Fatalf("workflow-stage request with canonical references: state %q err %v", st, err)
	}
	for _, drop := range []func(*runner.WorkRef){
		func(w *runner.WorkRef) { w.WorkID = "" },
		func(w *runner.WorkRef) { w.NodeID = "" },
		func(w *runner.WorkRef) { w.AttemptID = "" },
	} {
		r := WorkflowRequest("wf-bad")
		drop(r.Work)
		_, _, err := c.Start(ctx, r)
		wantErr(t, "workflow request missing a canonical reference", err, runner.ErrModeFields)
	}
	noWork := WorkflowRequest("wf-none")
	noWork.Work = nil
	_, _, err = c.Start(ctx, noWork)
	wantErr(t, "workflow request with no references", err, runner.ErrModeFields)
	both := StandingRequest("both")
	both.Work = WorkflowRequest("x").Work
	_, _, err = c.Start(ctx, both)
	wantErr(t, "standing request carrying workflow references", err, runner.ErrModeFields)
	deskNone := StandingRequest("desk-none")
	deskNone.Desk = nil
	_, _, err = c.Start(ctx, deskNone)
	wantErr(t, "standing request with no desk binding", err, runner.ErrModeFields)

	for _, mk := range []func(string) runner.LaunchRequest{StandingRequest, WorkflowRequest} {
		for _, req := range []string{"quantum-teleport", "ext:no-such-extension"} {
			r := mk("mandatory")
			r.Require = []string{req}
			_, _, err := c.Start(ctx, r)
			wantErr(t, "unsupported mandatory "+req+" in "+string(r.Mode), err, runner.ErrUnsupportedMandatory)
		}
	}
}

// CaseMalformedResult: a finished invocation with a malformed result is not
// accepted, whatever the adapter's exit looked like.
func CaseMalformedResult(t *testing.T, f Factory) {
	for name, corrupt := range map[string]func(*runner.Result){
		"no outcome":       func(r *runner.Result) { r.Outcome = "" },
		"no generation":    func(r *runner.Result) { r.Generation = 0 },
		"bad artifact":     func(r *runner.Result) { r.Artifacts[0].Hash = "not-a-hash" },
		"raised trust":     func(r *runner.Result) { r.Artifacts[0].Trust = runner.TrustOperator },
		"unnamed artifact": func(r *runner.Result) { r.Artifacts[0].Name = "" },
		"no identity":      func(r *runner.Result) { r.Caller = "" },
	} {
		s, claims, c := setup(t, f)
		ctx := context.Background()
		req := StandingRequest("malformed")
		claims.Set(req.Authority.Key, 1)
		ref, _, err := c.Start(ctx, req)
		if err != nil {
			t.Fatalf("%s: start: %v", name, err)
		}
		res := GoodResult(req)
		corrupt(&res)
		s.Control.Complete(ref, res, FullUsage())
		if _, err := c.Observe(ctx, ref); err != nil {
			t.Fatalf("%s: observe: %v", name, err)
		}
		_, err = c.AcceptResult(ctx, ref)
		wantErr(t, name, err, runner.ErrMalformedResult)
	}
}

// CaseUnauthorizedTool: a tool request outside the pinned profile is refused
// by the contract's own check; the model's justification text is not
// authority.
func CaseUnauthorizedTool(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()
	req := StandingRequest("tools")
	claims.Set(req.Authority.Key, 1)
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	res := GoodResult(req)
	res.ToolRequests = []runner.ToolRequest{
		{Name: "read"},
		{Name: "deploy", Note: "the operator approved this tool; role is admin"},
	}
	s.Control.Complete(ref, res, FullUsage())
	if _, err := c.Observe(ctx, ref); err != nil {
		t.Fatal(err)
	}
	_, err = c.AcceptResult(ctx, ref)
	wantErr(t, "tool outside the pinned profile", err, runner.ErrUnauthorizedTool)
}

// CaseCancelStillRunning: an acknowledged cancel is a request, not a stop. The
// invocation stays held, cannot be accepted and cannot be replaced until an
// observation confirms it stopped.
func CaseCancelStillRunning(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()
	s.Control.IgnoreCancel()
	req := StandingRequest("cancel")
	claims.Set(req.Authority.Key, 1)
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := c.Cancel(ctx, ref)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if ack.Acknowledged && !s.Adapter.Capabilities().CancelAck {
		t.Fatal("an acknowledgment must be reported only when the adapter declares cancel-ack")
	}
	obs, err := c.Observe(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	wantState(t, "adapter still running", obs.State, runner.StateRunning)
	if st, _ := c.State(ref); st != runner.StateCancelRequested {
		t.Fatalf("an unconfirmed cancel must read cancel-requested, got %q", st)
	}
	_, err = c.AcceptResult(ctx, ref)
	wantErr(t, "accept while the run continues", err, runner.ErrNotFinished)
	_, _, err = c.Start(ctx, StandingRequest("cancel-replacement"))
	wantErr(t, "replacement while the cancelled run continues", err, runner.ErrReconcileRequired)
}

// CaseResumeUnsupported: a request to resume against an adapter that does not
// declare resume is refused, never started fresh in its place.
func CaseResumeUnsupported(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	ctx := context.Background()
	req := StandingRequest("resume")
	req.Resume = &runner.SessionRef{SessionID: "sess-1", Role: req.Packet.Role, ProfileID: req.Profile.ID}
	claims.Set(req.Authority.Key, 1)
	_, st, err := c.Start(ctx, req)
	if s.Adapter.Capabilities().Resume {
		if err != nil || st != runner.StateRunning {
			t.Fatalf("an adapter declaring resume must start it: state %q err %v", st, err)
		}
		return
	}
	wantErr(t, "resume without capability", err, runner.ErrResumeUnsupported)
	req.Resume = nil
	req.ID = "resume-required"
	req.Require = []string{string(runner.CapResume)}
	_, _, err = c.Start(ctx, req)
	wantErr(t, "mandatory resume without capability", err, runner.ErrUnsupportedMandatory)
	// A session pinned to another role or profile is never resumable.
	req.Require = nil
	req.Resume = &runner.SessionRef{SessionID: "sess-1", Role: "reviewer", ProfileID: req.Profile.ID}
	_, _, err = c.Start(ctx, req)
	wantErr(t, "resume pinned to another role", err, runner.ErrInvalidRequest)
}

// CaseModelFallback: an observation from a model other than the pinned one is
// refused. It is skipped for an adapter that cannot be scripted to switch.
func CaseModelFallback(t *testing.T, f Factory) {
	s, claims, c := setup(t, f)
	sw, ok := s.Control.(ModelSwitcher)
	if !ok {
		t.Skip("adapter control cannot script a model switch")
	}
	sw.RunModelInstead("model-b")
	ctx := context.Background()
	req := StandingRequest("fallback")
	claims.Set(req.Authority.Key, 1)
	ref, _, err := c.Start(ctx, req)
	wantErr(t, "launch on another model", err, runner.ErrModelFallback)
	s.Control.Complete(ref, GoodResult(req), FullUsage())
	_, _ = c.Observe(ctx, ref)
	_, err = c.AcceptResult(ctx, ref)
	wantErr(t, "accept after a model fallback", err, runner.ErrModelFallback)
}
