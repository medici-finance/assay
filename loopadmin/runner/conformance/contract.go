package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

// passwordURL builds a URL whose userinfo carries a password, at run time.
func passwordURL() string {
	return "https://" + "deploy" + ":" + "x9" + "y8z7w6" + "@" + "git.example/r"
}

// CaseTerminalAbsorbs: failed, stopped and finished are absorbing. An adapter
// that later reports a failed or stopped attempt running, or a finished one
// with another result or under another model, is refused as a contract violation; the recorded state
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
	// The same result re-reported is not a regression; the same result under
	// another model is.
	same := GoodResult(fin)
	h.Report(finRef, runner.Observation{State: runner.StateFinished, ActualModel: fin.Profile.Model, Result: &same})
	if _, err := c.Observe(ctx, finRef); err != nil {
		t.Fatalf("the recorded outcome re-reported: %v", err)
	}
	h.Report(finRef, runner.Observation{State: runner.StateFinished, ActualModel: "model-b", Result: &same})
	_, err = c.Observe(ctx, finRef)
	wantErr(t, "finished attempt reported under another model", err, runner.ErrStateRegression)
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
// at every depth: a nested credential slot, an authorization, auth, bearer,
// private-key, access-key, cookie or session key, a Basic credential, a
// JSON-escaped token and a URL carrying a password are refused. Prose that
// merely says "basic", a key that merely starts with "auth" (author,
// authority) and a URL without a password are not.
func CaseCredentialShapes(t *testing.T, f Factory) {
	_, _, claims, c := hsetup(t, f)
	ctx := context.Background()
	secret := secretShape()
	basic := "Basic " + "dXNlcjpwYXNzd29yZA=="
	for name, ext := range map[string]map[string]string{
		"nested token key":     {"cfg": `{"retry":{"token":"abcdefgh12"}}`},
		"nested authorization": {"cfg": `{"headers":{"Authorization":"x"}}`},
		"basic credential":     {"cfg": `{"hdr":"` + basic + `"}`},
		"escaped token":        {"cfg": `"\u0067` + secret[1:] + `"`},
		"escaped nested token": {"cfg": `{"a":["x","\u0067` + secret[1:] + `"]}`},
		"escaped nested key":   {"cfg": `{"\u0074oken":"x"}`},
		"token in an array":    {"cfg": `[{"note":"` + secret + `"}]`},
		"cookie key":           {"cookie": `"sid"`},
		"session key":          {"session": `"s"`},
		"auth key":             {"auth": `"x"`},
		"auth word in a key":   {"cfg": `{"x-auth":"x"}`},
		"oauth key":            {"oauth": `"x"`},
		"bearer key":           {"cfg": `{"bearer":"x"}`},
		"privkey key":          {"privkey": `"x"`},
		"camelCase privkey":    {"cfg": `{"sshPrivKey":"x"}`},
		"PrivKey key":          {"PrivKey": `"x"`},
		"acronym access key":   {"cfg": `{"AWSAccessKeyId":"x"}`},
		"run-together slot":    {"cfg": `{"myaccesskey":"x","userbearer":"y"}`},
		"prefixed slot value":  {"cfg": `"userprivkey=abcdefgh12"`},
		"slot after a setting": {"cfg": `"{\"auth\":\"disabled\",\"privkey\":\"abcdefgh12\"}"`},
		"password with a hash": {"cfg": `"https://user:abc#def123@git.example"`},
		"access key id key":    {"cfg": `{"aws_access_key_id":"x"}`},
		"access key id value":  {"cfg": `"aws_access_key_id=abcdefgh12"`},
		"url with a password":  {"cfg": `{"remote":"` + passwordURL() + `"}`},
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

	ws := StandingRequest("cred-workspace")
	ws.Workspace = passwordURL()
	_, _, err = c.Start(ctx, ws)
	wantErr(t, "workspace URL with a password", err, runner.ErrCredential)

	// Positive control: an extension with no credential in it starts, and so
	// does a workspace URL with a user but no password.
	ok := StandingRequest("cred-clean")
	ok.Extensions = map[string]json.RawMessage{"cfg": json.RawMessage(`{"retries":3,"note":"basic functionality only","author":"a","authority":"b","remote":"https://git.example/r"}`)}
	ok.Workspace = "ssh://git@git.example/r"
	// Settings that merely name a slot word are not slots, and neither is a
	// port with an at-sign in the query.
	ok.Extensions["settings"] = json.RawMessage(`{"auth_method":"oidc","auth_required":true,"oauth_scopes":"read","bearer_format":"jwt","access_key_rotation_days":90,"note":"auth: disabled","pallbearer":"pallbearer=ab12cd34ef"}`)
	ok.Extensions["more"] = json.RawMessage(`{"cupbearer":"x","note":"auth: disabled","other":"fine"}`)
	ok.Extensions["contact"] = json.RawMessage(`"https://git.example:8443?owner=a@b.example"`)
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

	// A null element of a string list is a null too: encoding/json would
	// decode it to an empty string and every later check would pass it, yet
	// the schema types each element as a string. It is refused at every
	// position, in both modes, so nothing the module accepts fails the schema.
	for mode, base := range map[string]runner.LaunchRequest{
		"standing-desk":  StandingRequest("decode-null-tool"),
		"workflow-stage": WorkflowRequest("decode-null-tool"),
	} {
		base.Profile.Tools = []string{"tool-a", "tool-b", "tool-c"}
		doc := marshal(base)
		list := `"tools":["tool-a","tool-b","tool-c"]`
		if !strings.Contains(doc, list) {
			t.Fatalf("%s: fixture edit did not apply", mode)
		}
		clean, err := runner.DecodeRequest([]byte(doc))
		if err != nil {
			t.Fatalf("%s: a request with three tools must decode: %v", mode, err)
		}
		if err := clean.Validate(); err != nil {
			t.Fatalf("%s: a request with three tools must validate: %v", mode, err)
		}
		for pos, nulled := range map[string]string{
			"first":  `"tools":[null,"tool-b","tool-c"]`,
			"middle": `"tools":["tool-a",null,"tool-c"]`,
			"last":   `"tools":["tool-a","tool-b",null]`,
			"only":   `"tools":[null]`,
		} {
			_, err := runner.DecodeRequest([]byte(strings.Replace(doc, list, nulled, 1)))
			wantErr(t, mode+" null tool, "+pos, err, runner.ErrInvalidRequest)
		}
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

	// The same ID under another caller namespace is another invocation too.
	other := StandingRequest("ident-c")
	other.Authority.Key = "claim/ident-c"
	claims.Set(other.Authority.Key, 1)
	otherRef, _, err := c.Start(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	res := GoodResult(other)
	res.Caller = "another-caller"
	finish(t, s, c, otherRef, res)
	_, err = c.AcceptResult(ctx, otherRef)
	wantErr(t, "result echoing another caller", err, runner.ErrResultIdentity)
}

// CaseRequestRules: identity reuse, the workflow-stage desk exclusion, the
// pinned tool list, the budget limits, blank references (a blank is trimmed
// first, in the desk binding, each workflow reference and the resume session),
// a resume pinned to another profile and a credential in any resume field are
// each refused before any adapter is asked.
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
		"blank node id": {func(r *runner.LaunchRequest) {
			*r = WorkflowRequest(r.ID)
			r.Work.NodeID = " "
		}, runner.ErrModeFields},
		"blank attempt id": {func(r *runner.LaunchRequest) {
			*r = WorkflowRequest(r.ID)
			r.Work.AttemptID = "\t"
		}, runner.ErrModeFields},
		"blank desk binding": {func(r *runner.LaunchRequest) { r.Desk.BindingID = " " }, runner.ErrModeFields},
		"blank resume session": {func(r *runner.LaunchRequest) {
			r.Resume = &runner.SessionRef{SessionID: " ", Role: r.Packet.Role, ProfileID: r.Profile.ID}
		}, runner.ErrInvalidRequest},
		"resume pinned to another profile": {func(r *runner.LaunchRequest) {
			r.Resume = &runner.SessionRef{SessionID: "sess-1", Role: r.Packet.Role, ProfileID: "profile-b"}
		}, runner.ErrInvalidRequest},
		"credential in the resume session": {func(r *runner.LaunchRequest) {
			r.Resume = &runner.SessionRef{SessionID: secretShape(), Role: r.Packet.Role, ProfileID: r.Profile.ID}
		}, runner.ErrCredential},
		"credential in the resume role": {func(r *runner.LaunchRequest) {
			r.Resume = &runner.SessionRef{SessionID: "sess-1", Role: secretShape(), ProfileID: r.Profile.ID}
		}, runner.ErrCredential},
		"credential in the resume profile": {func(r *runner.LaunchRequest) {
			r.Resume = &runner.SessionRef{SessionID: "sess-1", Role: r.Packet.Role, ProfileID: secretShape()}
		}, runner.ErrCredential},
		"blank tool name, first":  {func(r *runner.LaunchRequest) { r.Profile.Tools = []string{"", "read"} }, runner.ErrInvalidRequest},
		"blank tool name, middle": {func(r *runner.LaunchRequest) { r.Profile.Tools = []string{"read", " ", "exec"} }, runner.ErrInvalidRequest},
		"blank tool name, last":   {func(r *runner.LaunchRequest) { r.Profile.Tools = []string{"read", "\t"} }, runner.ErrInvalidRequest},
		"absent tool list":        {func(r *runner.LaunchRequest) { r.Profile.Tools = nil }, runner.ErrInvalidRequest},
		"no budget limit":         {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = 0, 0 }, runner.ErrInvalidRequest},
		"negative tokens":         {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = -5, 1 }, runner.ErrInvalidRequest},
		"negative cost":           {func(r *runner.LaunchRequest) { r.Budget.MaxTokens, r.Budget.MaxCostMicros = 5, -1 }, runner.ErrInvalidRequest},
	} {
		r := StandingRequest("rule")
		r.Authority.Key = "claim/rules"
		tc.edit(&r)
		r.Authority.Key = "claim/rules"
		claims.Set(r.Authority.Key, 1)
		_, _, err := c.Start(ctx, r)
		wantErr(t, name, err, tc.want)
	}
	// Positive control: named tools start, and an empty tool list pins no tools.
	named := StandingRequest("named-tools")
	named.Authority.Key = "claim/named"
	claims.Set(named.Authority.Key, 1)
	named.Profile.Tools = []string{"read", "write", "exec"}
	if _, _, err := c.Start(ctx, named); err != nil {
		t.Fatalf("a request naming its tools must start: %v", err)
	}
	// An empty tool list pins no tools and is valid.
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

// CaseNoAdapterEcho: an error an adapter or a fence returns never reaches the
// text of the error the Client returns, through any verb: a definite and an
// unknown launch failure, an observe, a reconcile, a cancel and a fence read.
// The adapter's error stays in the chain, so errors.Is still matches it beside
// the contract's sentinel.
func CaseNoAdapterEcho(t *testing.T, f Factory) {
	s, h, claims, c := hsetup(t, f)
	ctx := context.Background()
	marker := "m4rk3r-adapter-text"
	cause := errors.New("hostile adapter: " + marker + " " + secretShape())
	check := func(what string, err, sentinel error) {
		t.Helper()
		if !errors.Is(err, sentinel) {
			t.Fatalf("%s: got %v, want %v", what, err, sentinel)
		}
		if !errors.Is(err, cause) {
			t.Fatalf("%s: the adapter's error must stay in the chain", what)
		}
		if msg := err.Error(); strings.Contains(msg, marker) || strings.Contains(msg, secretShape()) {
			t.Fatalf("%s: the error text echoes the adapter's error", what)
		}
	}

	definite := StandingRequest("adapter-echo-definite")
	definite.Authority.Key = "claim/adapter-echo-a"
	claims.Set(definite.Authority.Key, 1)
	h.FailNextStart(errors.Join(runner.ErrDefiniteFailure, cause), false)
	_, st, err := c.Start(ctx, definite)
	check("start, definite failure", err, runner.ErrDefiniteFailure)
	wantState(t, "start, definite failure", st, runner.StateFailed)

	unknown := StandingRequest("adapter-echo-unknown")
	unknown.Authority.Key = "claim/adapter-echo-b"
	claims.Set(unknown.Authority.Key, 1)
	h.FailNextStart(cause, true)
	ref, st, err := c.Start(ctx, unknown)
	check("start, unknown outcome", err, runner.ErrReconcileRequired)
	wantState(t, "start, unknown outcome", st, runner.StateUnknown)

	h.FailNextObserve(cause)
	_, err = c.Observe(ctx, ref)
	check("observe", err, runner.ErrReconcileRequired)
	h.FailNextObserve(cause)
	_, err = c.Reconcile(ctx, ref)
	check("reconcile", err, runner.ErrReconcileRequired)
	h.FailNextCancel(cause)
	_, err = c.Cancel(ctx, ref)
	check("cancel", err, runner.ErrCancelFailed)
	wantStateOf(t, c, ref, runner.StateUnknown)

	fenced := runner.NewClient(h, runner.FenceFunc(func(context.Context, string) (uint64, error) {
		return 0, cause
	}))
	req := StandingRequest("adapter-echo-fence")
	req.Authority.Key = "claim/adapter-echo-c"
	fRef, _, err := fenced.Start(ctx, req)
	if err != nil {
		t.Fatalf("start under the failing fence: %v", err)
	}
	s.Control.Complete(fRef, GoodResult(req), FullUsage())
	if _, err := fenced.Observe(ctx, fRef); err != nil {
		t.Fatalf("observe under the failing fence: %v", err)
	}
	_, err = fenced.AcceptResult(ctx, fRef)
	check("fence read", err, runner.ErrFenceUnavailable)
}

// rewriter is an adapter that rewrites the request it was handed after it
// returns, as a hostile or careless adapter keeping a reference might.
type rewriter struct{ runner.Adapter }

func (r rewriter) Start(ctx context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	rec, err := r.Adapter.Start(ctx, req)
	req.Profile.Tools[1] = "deploy"
	return rec, err
}

// CaseNoAliasing: the Client's record shares nothing with the request the
// caller passed or the adapter was handed, the observation the adapter
// returned, or anything the Client hands back. Rewriting any of them after the
// call leaves the record, and so every later acceptance and usage total, as it
// was.
func CaseNoAliasing(t *testing.T, f Factory) {
	s := f(t)
	h := NewHostile(s.Adapter)
	claims := NewClaims()
	c := runner.NewClient(rewriter{h}, claims)
	ctx := context.Background()
	req := StandingRequest("aliasing")
	claims.Set(req.Authority.Key, 1)
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	// The caller rewrites its request after Start: the pinned tools stand.
	req.Profile.Tools[0] = "deploy"

	res := GoodResult(req)
	res.ToolRequests = []runner.ToolRequest{{Name: "read"}, {Name: "write"}}
	usage := FullUsage()
	h.Report(ref, runner.Observation{State: runner.StateFinished, ActualModel: req.Profile.Model, Usage: usage, Result: &res})
	obs, err := c.Observe(ctx, ref)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	// The adapter rewrites what it returned; the caller rewrites what it got.
	res.Summary = "rewritten by the adapter"
	res.Artifacts[0].Trust = runner.TrustOperator
	res.ToolRequests[0].Name = "deploy"
	*usage.InputTokens = 999
	if obs.Result.Summary != "done" || obs.Result.ToolRequests[0].Name != "read" {
		t.Fatalf("the adapter rewrote what Observe returned: %+v", *obs.Result)
	}
	obs.Result.Summary = "rewritten by the caller"
	obs.Result.Artifacts[0].Name = "rewritten"
	if obs.Usage.OutputTokens != nil {
		*obs.Usage.OutputTokens = 999
	}

	first, err := c.AcceptResult(ctx, ref)
	if err != nil {
		t.Fatalf("the recorded result must still be accepted: %v", err)
	}
	// The caller rewrites the accepted result; a second acceptance is unchanged.
	first.Result.Summary = "rewritten after acceptance"
	first.Result.Artifacts[0].Ref = "rewritten"
	first.Result.ToolRequests[0].Name = "rewritten"
	if first.Usage.CostMicros != nil {
		*first.Usage.CostMicros = 999
	}
	// A terminal re-report returns the record; the caller rewrites that too.
	want := GoodResult(StandingRequest("aliasing"))
	want.ToolRequests = []runner.ToolRequest{{Name: "read"}, {Name: "write"}}
	fresh := want
	fresh.Artifacts = append([]runner.Artifact(nil), want.Artifacts...)
	fresh.ToolRequests = append([]runner.ToolRequest(nil), want.ToolRequests...)
	h.Report(ref, runner.Observation{State: runner.StateFinished, ActualModel: req.Profile.Model, Usage: FullUsage(), Result: &fresh})
	re, err := c.Observe(ctx, ref)
	if err != nil {
		t.Fatalf("terminal re-report: %v", err)
	}
	re.Result.Artifacts[0].Hash = "rewritten"
	again, err := c.AcceptResult(ctx, ref)
	if err != nil {
		t.Fatalf("second acceptance: %v", err)
	}
	if !reflect.DeepEqual(again.Result, want) {
		t.Fatalf("the recorded result moved: got %+v, want %+v", again.Result, want)
	}
	if h.Capabilities().Telemetry == runner.TelemetryNone {
		return // usage is unknown throughout; nothing to rewrite
	}
	wantUsage := FullUsage()
	if !reflect.DeepEqual(again.Usage, wantUsage) {
		t.Fatalf("the recorded usage moved: got %+v", again.Usage)
	}
	if tot := c.Usage(req.Caller); !reflect.DeepEqual(tot, wantUsage) {
		t.Fatalf("the usage total moved: got %+v", tot)
	}
}

// rewriteAdapter rewrites the request it was handed after the real Start
// returns, over every field a request shares by reference.
type rewriteAdapter struct {
	runner.Adapter
	rewrite func(*runner.LaunchRequest)
}

func (r rewriteAdapter) Start(ctx context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	rec, err := r.Adapter.Start(ctx, req)
	r.rewrite(&req)
	return rec, err
}

// capsAdapter declares capabilities the test still holds a handle on.
type capsAdapter struct {
	runner.Adapter
	caps runner.Capabilities
}

func (a capsAdapter) Capabilities() runner.Capabilities { return a.caps }

// CaseNoAliasingRecord: the record shares no slice, map or pointer of the
// request with the caller or the adapter, field by field: the required
// capabilities, each extension (the entry and its bytes), the desk, work and
// resume references, and the capabilities the adapter declared. Rewriting any
// of them after Start leaves the record as it was, so the pristine request is
// still the same request (a repeat Start is a no-op, never an identity reuse)
// and the declared capabilities still hold.
func CaseNoAliasingRecord(t *testing.T, f Factory) {
	ctx := context.Background()
	s := f(t)
	withResume := s.Adapter.Capabilities().Resume
	pristine := func(mk func(string) runner.LaunchRequest) runner.LaunchRequest {
		r := mk("alias-record")
		r.Require = []string{string(runner.CapBudgetLaunch)}
		r.Extensions = map[string]json.RawMessage{"note": json.RawMessage(`"keep"`), "bytes": json.RawMessage(`"abcd"`)}
		if withResume {
			r.Resume = &runner.SessionRef{SessionID: "sess-1", Role: r.Packet.Role, ProfileID: r.Profile.ID}
		}
		return r
	}
	rewrite := func(r *runner.LaunchRequest) {
		r.Require[0] = "rewritten"
		r.Extensions["note"] = json.RawMessage(`"rewritten"`)
		r.Extensions["bytes"][1] = 'X'
		if r.Desk != nil {
			r.Desk.BindingID = "rewritten"
		}
		if r.Work != nil {
			r.Work.NodeID = "rewritten"
		}
		if r.Resume != nil {
			r.Resume.SessionID = "rewritten"
		}
	}
	for name, mk := range map[string]func(string) runner.LaunchRequest{"standing-desk": StandingRequest, "workflow-stage": WorkflowRequest} {
		claims := NewClaims()
		c := runner.NewClient(rewriteAdapter{Adapter: NewHostile(s.Adapter), rewrite: rewrite}, claims)
		req := pristine(mk)
		claims.Set(req.Authority.Key, 1)
		if _, _, err := c.Start(ctx, req); err != nil {
			t.Fatalf("%s: start: %v", name, err)
		}
		// The adapter has rewritten its copy; now the caller rewrites its own.
		rewrite(&req)
		_, st, err := c.Start(ctx, pristine(mk))
		if err != nil || st != runner.StateRunning {
			t.Fatalf("%s: the recorded request moved: a repeat of the original start is state %q err %v", name, st, err)
		}
	}

	// The adapter's declared capabilities are copied at NewClient: the adapter
	// changing the slices it declared afterwards changes nothing.
	caps := s.Adapter.Capabilities()
	caps.BudgetScopes = []runner.BudgetScope{runner.BudgetLaunch}
	caps.Extensions = []string{"hint"}
	claims := NewClaims()
	c := runner.NewClient(capsAdapter{Adapter: s.Adapter, caps: caps}, claims)
	caps.BudgetScopes[0] = "rewritten"
	caps.Extensions[0] = "rewritten"
	req := StandingRequest("alias-caps")
	req.Require = []string{"ext:hint", string(runner.CapBudgetLaunch)}
	claims.Set(req.Authority.Key, 1)
	if _, st, err := c.Start(ctx, req); err != nil || st != runner.StateRunning {
		t.Fatalf("the declared capabilities moved after NewClient: state %q err %v", st, err)
	}
}
