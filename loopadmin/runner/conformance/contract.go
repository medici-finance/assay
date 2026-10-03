package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// The cases in this file drive the contract's own rules with a Hostile wrapper
// around the adapter under test: they hold whatever the adapter does, and none
// of them consults a caller's own check, so each proves the contract refuses
// the fault on its own.

func hsetup(t *testing.T, f Factory) (Subject, *Hostile, *Claims, *runner.Client) {
	t.Helper()
	s := f(t)
	h := NewHostile(s.Adapter)
	claims := NewClaims()
	return s, h, claims, runner.NewClient(h, claims)
}

// finish drives a run to finished in the adapter and records the observation.
func finish(t *testing.T, s Subject, c *runner.Client, ref runner.Ref, res runner.Result) {
	t.Helper()
	s.Control.Complete(ref, res, FullUsage())
	if _, err := c.Observe(context.Background(), ref); err != nil {
		t.Fatalf("observe finished %s: %v", ref.ID, err)
	}
}

// secretShape builds a credential-shaped string at run time, so no literal in
// the tree looks like one.
func secretShape() string { return "gh" + "p_" + strings.Repeat("a1", 20) }

// CaseTerminalAbsorbs: failed, stopped and finished are absorbing. An adapter
// that later reports a failed or stopped attempt running, or a finished one
// with another result, is refused as a contract violation; the recorded state
// and the accepted result do not move, so a revived attempt can never sit
// beside its replacement or swap the result already accepted.
func CaseTerminalAbsorbs(t *testing.T, f Factory) {
	s, h, claims, c := hsetup(t, f)
	ctx := context.Background()

	// Revival after a definite failure: the adapter launched the run but
	// answered with a definite failure, so the authority was freed.
	a := StandingRequest("revive-a")
	claims.Set(a.Authority.Key, 1)
	h.FailNextStart(errors.Join(runner.ErrDefiniteFailure, errors.New("hostile: reported failed")), true)
	aRef, st, err := c.Start(ctx, a)
	wantErr(t, "definite failure", err, runner.ErrDefiniteFailure)
	wantState(t, "definite failure", st, runner.StateFailed)
	b := StandingRequest("revive-b")
	if _, st, err := c.Start(ctx, b); err != nil || st != runner.StateRunning {
		t.Fatalf("replacement after a definite failure: state %q err %v", st, err)
	}
	_, err = c.Observe(ctx, aRef)
	wantErr(t, "failed attempt reported running", err, runner.ErrStateRegression)
	_, err = c.Reconcile(ctx, aRef)
	wantErr(t, "failed attempt reconciled running", err, runner.ErrStateRegression)
	s.Control.Complete(aRef, GoodResult(a), FullUsage())
	_, err = c.Observe(ctx, aRef)
	wantErr(t, "failed attempt reported finished", err, runner.ErrStateRegression)
	if st, _ := c.State(aRef); st != runner.StateFailed {
		t.Fatalf("a failed attempt must stay failed, got %q", st)
	}
	_, err = c.AcceptResult(ctx, aRef)
	wantErr(t, "accept a revived attempt", err, runner.ErrNotFinished)

	// Revival after an observed stop.
	stop := StandingRequest("stop-1")
	stop.Authority.Key = "claim/stop"
	claims.Set(stop.Authority.Key, 1)
	stopRef, _, err := c.Start(ctx, stop)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Cancel(ctx, stopRef); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	h.Report(stopRef, runner.Observation{State: runner.StateStopped, ActualModel: stop.Profile.Model})
	if _, err := c.Observe(ctx, stopRef); err != nil {
		t.Fatalf("observe stopped: %v", err)
	}
	wantStateOf(t, c, stopRef, runner.StateStopped)
	repl := StandingRequest("stop-2")
	repl.Authority.Key = stop.Authority.Key
	if _, st, err := c.Start(ctx, repl); err != nil || st != runner.StateRunning {
		t.Fatalf("replacement after an observed stop: state %q err %v", st, err)
	}
	h.Report(stopRef, runner.Observation{State: runner.StateRunning, ActualModel: stop.Profile.Model})
	_, err = c.Observe(ctx, stopRef)
	wantErr(t, "stopped attempt reported running", err, runner.ErrStateRegression)
	wantStateOf(t, c, stopRef, runner.StateStopped)

	// A finished, accepted result cannot be swapped or reopened.
	fin := StandingRequest("fin")
	fin.Authority.Key = "claim/fin"
	claims.Set(fin.Authority.Key, 1)
	finRef, _, err := c.Start(ctx, fin)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, s, c, finRef, GoodResult(fin))
	if _, err := c.AcceptResult(ctx, finRef); err != nil {
		t.Fatalf("accept: %v", err)
	}
	other := GoodResult(fin)
	other.Summary = "a different result"
	h.Report(finRef, runner.Observation{State: runner.StateFinished, ActualModel: fin.Profile.Model, Result: &other})
	_, err = c.Observe(ctx, finRef)
	wantErr(t, "finished attempt reported with another result", err, runner.ErrStateRegression)
	h.Report(finRef, runner.Observation{State: runner.StateRunning, ActualModel: fin.Profile.Model})
	_, err = c.Observe(ctx, finRef)
	wantErr(t, "finished attempt reported running", err, runner.ErrStateRegression)
	acc, err := c.AcceptResult(ctx, finRef)
	if err != nil || acc.Result.Summary != "done" {
		t.Fatalf("the recorded result must stand: summary %q err %v", acc.Result.Summary, err)
	}
}

func wantStateOf(t *testing.T, c *runner.Client, ref runner.Ref, want runner.State) {
	t.Helper()
	got, err := c.State(ref)
	if err != nil || got != want {
		t.Fatalf("%s: state %q err %v, want %q", ref.ID, got, err, want)
	}
}

// CaseCredentialShapes: credential exclusion reads extension values decoded,
// at every depth: a nested credential slot, an authorization or cookie key, a
// Basic credential and a JSON-escaped token are refused. Prose that merely
// says "basic" is not.
func CaseCredentialShapes(t *testing.T, f Factory) {
	_, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	secret := secretShape()
	basic := "Basic " + "dXNlcjpwYXNzd29yZA=="
	for name, ext := range map[string]map[string]string{
		"nested token key":     {"cfg": `{"retry":{"token":"abcdefgh12"}}`},
		"nested authorization": {"cfg": `{"headers":{"Authorization":"x"}}`},
		"basic credential":     {"cfg": `{"hdr":"` + basic + `"}`},
		"escaped token":        {"cfg": `"g` + secret[1:] + `"`},
		"token in an array":    {"cfg": `[{"note":"` + secret + `"}]`},
		"cookie key":           {"cookie": `"sid"`},
		"session key":          {"session": `"s"`},
	} {
		r := StandingRequest("cred-shape")
		r.Extensions = map[string]json.RawMessage{}
		for k, v := range ext {
			r.Extensions[k] = json.RawMessage(v)
		}
		claims.Set(r.Authority.Key, 1)
		_, _, err := c.Start(ctx, r)
		wantErr(t, name, err, runner.ErrCredential)
		if _, err := c.State(r.Ref()); !errors.Is(err, runner.ErrUnknownInvocation) {
			t.Fatalf("%s: a refused request must not be recorded, got %v", name, err)
		}
	}
	bad := StandingRequest("cred-undecodable")
	bad.Extensions = map[string]json.RawMessage{"cfg": json.RawMessage(`{"a":`)}
	_, _, err := c.Start(ctx, bad)
	wantErr(t, "undecodable extension", err, runner.ErrInvalidRequest)

	// Positive control: an extension with no credential in it starts.
	ok := StandingRequest("cred-clean")
	ok.Extensions = map[string]json.RawMessage{"cfg": json.RawMessage(`{"retries":3,"note":"basic functionality only"}`)}
	claims.Set(ok.Authority.Key, 1)
	if _, _, err := c.Start(ctx, ok); err != nil {
		t.Fatalf("a clean extension must start: %v", err)
	}
}

// CaseNoPayloadEcho: no refusal carries request or result content in its
// error text, and a credential anywhere in a result is refused as a credential
// before any shape check can describe it.
func CaseNoPayloadEcho(t *testing.T, f Factory) {
	ctx := context.Background()
	marker := "m4rk3r-untrusted-payload"
	secret := secretShape()
	clean := func(what string, err error, payload string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: want a refusal", what)
		}
		if strings.Contains(err.Error(), payload) {
			t.Fatalf("%s: the refusal echoes its payload", what)
		}
	}
	for name, corrupt := range map[string]func(*runner.Result, string){
		"outcome":      func(r *runner.Result, p string) { r.Outcome = runner.Outcome(p) },
		"artifact":     func(r *runner.Result, p string) { r.Artifacts[0].Name, r.Artifacts[0].Hash = p, "x" },
		"trust":        func(r *runner.Result, p string) { r.Artifacts[0].Trust = runner.Trust(p) },
		"tool":         func(r *runner.Result, p string) { r.ToolRequests = []runner.ToolRequest{{Name: p}} },
		"identity":     func(r *runner.Result, p string) { r.ID = p },
		"artifact ref": func(r *runner.Result, p string) { r.Artifacts[0].Ref, r.Artifacts[0].Hash = p, "x" },
	} {
		for _, payload := range []string{marker, secret} {
			s, _, claims, c := hsetup(t, f)
			req := StandingRequest("echo-" + strings.ReplaceAll(name, " ", "-"))
			claims.Set(req.Authority.Key, 1)
			ref, _, err := c.Start(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			res := GoodResult(req)
			corrupt(&res, payload)
			finish(t, s, c, ref, res)
			_, err = c.AcceptResult(ctx, ref)
			clean("result "+name, err, payload)
			if payload == secret && !errors.Is(err, runner.ErrCredential) {
				t.Fatalf("result %s: a credential must be refused as one first, got %v", name, err)
			}
		}
	}

	_, _, claims, c := hsetup(t, f)
	for name, corrupt := range map[string]func(*runner.LaunchRequest){
		"version":   func(r *runner.LaunchRequest) { r.Version = marker },
		"mode":      func(r *runner.LaunchRequest) { r.Mode = runner.Mode(marker) },
		"trust":     func(r *runner.LaunchRequest) { r.Packet.Trust = runner.Trust(marker) },
		"scope":     func(r *runner.LaunchRequest) { r.Budget.Scope = runner.BudgetScope(marker) },
		"require":   func(r *runner.LaunchRequest) { r.Require = []string{marker} },
		"ext name":  func(r *runner.LaunchRequest) { r.Require = []string{"ext:" + marker} },
		"ext key":   func(r *runner.LaunchRequest) { r.Extensions = map[string]json.RawMessage{secret: json.RawMessage(`1`)} },
		"workspace": func(r *runner.LaunchRequest) { r.Workspace = secret },
	} {
		r := StandingRequest("echo-request")
		corrupt(&r)
		claims.Set(r.Authority.Key, 1)
		_, _, err := c.Start(ctx, r)
		clean("request "+name, err, marker)
		clean("request "+name, err, secret)
	}
	held := StandingRequest(marker)
	held.Authority.Key = marker + "-key"
	claims.Set(held.Authority.Key, 1)
	if _, _, err := c.Start(ctx, held); err != nil {
		t.Fatal(err)
	}
	next := StandingRequest("next")
	next.Authority.Key = held.Authority.Key
	_, _, err := c.Start(ctx, next)
	wantErr(t, "held authority", err, runner.ErrReconcileRequired)
	clean("held authority", err, marker)

	good, err := json.Marshal(StandingRequest("decode"))
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.Replace(string(good), `{`, `{"`+marker+`":1,`, 1)
	_, err = runner.DecodeRequest([]byte(unknown))
	wantErr(t, "unknown key", err, runner.ErrInvalidRequest)
	clean("unknown key", err, marker)
}

// CaseStrictDecode: a request key must match a defined field exactly, no
// object may repeat a key, a defined field may not be null, and nothing may
// follow the request. A case-variant duplicate can therefore never erase a
// mandatory requirement.
func CaseStrictDecode(t *testing.T, f Factory) {
	_, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	marshal := func(r runner.LaunchRequest) string {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	good := marshal(StandingRequest("decode"))
	r, err := runner.DecodeRequest([]byte(good))
	if err != nil {
		t.Fatalf("a well-formed request must decode: %v", err)
	}
	claims.Set(r.Authority.Key, 1)
	if _, _, err := c.Start(ctx, r); err != nil {
		t.Fatalf("a decoded request must start: %v", err)
	}
	mandatory := StandingRequest("decode-require")
	mandatory.Require = []string{"quantum-teleport"}
	req := marshal(mandatory)
	open := strings.TrimSuffix(req, "}")
	for name, doc := range map[string]string{
		"case-variant key":        strings.Replace(good, `"version":`, `"VERSION":`, 1),
		"case-variant nested key": strings.Replace(good, `"role":`, `"Role":`, 1),
		"case-variant duplicate":  open + `,"Require":[]}`,
		"exact duplicate":         open + `,"require":[]}`,
		"trailing brace":          good + "}",
		"trailing value":          good + " {}",
		"null field":              strings.Replace(good, `"budget":{`, `"work":null,"budget":{`, 1),
		"null nested field":       strings.Replace(good, `"role":"worker"`, `"role":null`, 1),
	} {
		if doc == good || doc == req {
			t.Fatalf("%s: fixture edit did not apply", name)
		}
		_, err := runner.DecodeRequest([]byte(doc))
		wantErr(t, name, err, runner.ErrInvalidRequest)
	}
}

// CaseModelUnreported: a model nobody reported is unknown, never the pinned
// model. A receipt without one is not a fallback, but a finished observation
// without one is not accepted.
func CaseModelUnreported(t *testing.T, f Factory) {
	s, h, claims, c := hsetup(t, f)
	ctx := context.Background()
	h.BlankModel()
	req := StandingRequest("no-model")
	claims.Set(req.Authority.Key, 1)
	ref, st, err := c.Start(ctx, req)
	if err != nil || st != runner.StateRunning {
		t.Fatalf("a receipt without a model is unknown, not refused: state %q err %v", st, err)
	}
	finish(t, s, c, ref, GoodResult(req))
	_, err = c.AcceptResult(ctx, ref)
	wantErr(t, "finished with no model reported", err, runner.ErrModelUnreported)
}

// CaseResultIdentity: a result that echoes another invocation's identity is
// refused, even when both ran under the same generation number.
func CaseResultIdentity(t *testing.T, f Factory) {
	s, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	a := StandingRequest("ident-a")
	a.Authority.Key = "claim/ident-a"
	b := StandingRequest("ident-b")
	b.Authority.Key = "claim/ident-b"
	claims.Set(a.Authority.Key, 1)
	claims.Set(b.Authority.Key, 1)
	if _, _, err := c.Start(ctx, a); err != nil {
		t.Fatal(err)
	}
	bRef, _, err := c.Start(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, s, c, bRef, GoodResult(a)) // the adapter cross-wired a's result
	_, err = c.AcceptResult(ctx, bRef)
	wantErr(t, "cross-wired result", err, runner.ErrResultIdentity)
}

// CaseRequestRules: identity reuse, the workflow-stage desk exclusion, the
// pinned tool list, the budget limits and blank workflow references are each
// refused before any adapter is asked.
func CaseRequestRules(t *testing.T, f Factory) {
	_, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	first := StandingRequest("reuse")
	claims.Set(first.Authority.Key, 1)
	if _, _, err := c.Start(ctx, first); err != nil {
		t.Fatal(err)
	}
	reuse := first
	reuse.Workspace = "workspaces/other"
	_, _, err := c.Start(ctx, reuse)
	wantErr(t, "same (caller, id), different content", err, runner.ErrIdentityReuse)

	for name, tc := range map[string]struct {
		edit func(*runner.LaunchRequest)
		want error
	}{
		"workflow stage with a desk": {func(r *runner.LaunchRequest) {
			*r = WorkflowRequest(r.ID)
			r.Desk = &runner.DeskRef{BindingID: "binding-1"}
		}, runner.ErrModeFields},
		"blank work id": {func(r *runner.LaunchRequest) {
			*r = WorkflowRequest(r.ID)
			r.Work.WorkID = " "
		}, runner.ErrModeFields},
		"absent tool list": {func(r *runner.LaunchRequest) { r.Profile.Tools = nil }, runner.ErrInvalidRequest},
		"no budget limit":  {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = 0, 0 }, runner.ErrInvalidRequest},
		"negative tokens":  {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = -5, 1 }, runner.ErrInvalidRequest},
		"negative cost":    {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = 5, -1 }, runner.ErrInvalidRequest},
	} {
		r := StandingRequest("rule")
		r.Authority.Key = "claim/rules"
		tc.edit(&r)
		r.Authority.Key = "claim/rules"
		claims.Set(r.Authority.Key, 1)
		_, _, err := c.Start(ctx, r)
		wantErr(t, name, err, tc.want)
	}
	// Positive control: an empty tool list pins no tools and is valid.
	none := StandingRequest("no-tools")
	none.Authority.Key = "claim/rules"
	none.Profile.Tools = []string{}
	if _, _, err := c.Start(ctx, none); err != nil {
		t.Fatalf("an empty tool list must start: %v", err)
	}
}

// CaseReconcileRules: absent settles an unknown launch as failed only for an
// adapter that declares launch-dedupe, and an adapter error is never read as
// a state.
func CaseReconcileRules(t *testing.T, f Factory) {
	_, h, claims, c := hsetup(t, f)
	ctx := context.Background()
	req := StandingRequest("absent")
	claims.Set(req.Authority.Key, 1)
	h.FailNextStart(runner.ErrAckLost, false) // nothing was launched
	ref, _, err := c.Start(ctx, req)
	wantErr(t, "lost acknowledgment", err, runner.ErrReconcileRequired)
	obs, err := c.Reconcile(ctx, ref)
	if err != nil || obs.State != runner.StateAbsent {
		t.Fatalf("reconcile of a launch that never happened: state %q err %v", obs.State, err)
	}
	repl := StandingRequest("absent-replacement")
	if h.Capabilities().LaunchDedupe {
		wantStateOf(t, c, ref, runner.StateFailed)
		if _, _, err := c.Start(ctx, repl); err != nil {
			t.Fatalf("absent under launch-dedupe frees the authority: %v", err)
		}
	} else {
		wantStateOf(t, c, ref, runner.StateUnknown)
		_, _, err := c.Start(ctx, repl)
		wantErr(t, "absent without launch-dedupe still holds", err, runner.ErrReconcileRequired)
	}

	unread := StandingRequest("unread")
	unread.Authority.Key = "claim/unread"
	claims.Set(unread.Authority.Key, 1)
	h.FailNextStart(runner.ErrAckLost, true)
	uRef, _, err := c.Start(ctx, unread)
	wantErr(t, "lost acknowledgment", err, runner.ErrReconcileRequired)
	h.FailNextObserve(errors.New("hostile: adapter unreachable"))
	_, err = c.Observe(ctx, uRef)
	wantErr(t, "observe error", err, runner.ErrReconcileRequired)
	wantStateOf(t, c, uRef, runner.StateUnknown)
	next := StandingRequest("unread-replacement")
	next.Authority.Key = unread.Authority.Key
	_, _, err = c.Start(ctx, next)
	wantErr(t, "replacement after an observe error", err, runner.ErrReconcileRequired)
}

// CaseNegativeUsage: a negative usage figure is not a reading; it is unknown,
// never subtracted from a total.
func CaseNegativeUsage(t *testing.T, f Factory) {
	s, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	req := StandingRequest("negative-usage")
	claims.Set(req.Authority.Key, 1)
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	s.Control.Complete(ref, GoodResult(req), runner.Usage{InputTokens: runner.Int64(-50), OutputTokens: runner.Int64(5), CostMicros: runner.Int64(1)})
	obs, err := c.Observe(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Usage.InputTokens != nil || c.Usage(req.Caller).InputTokens != nil {
		t.Fatalf("a negative figure must read unknown, got %+v", obs.Usage)
	}
}
