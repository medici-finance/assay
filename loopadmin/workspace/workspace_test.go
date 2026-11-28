package workspace_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/workspace"
	"github.com/medici-finance/assay/loopadmin/workspace/fake"
)

// The prepared-environment/v1 tests. Each named test is a Verify row of
// agent-execution/02; its executable mutations are the entries for the same
// test name in testdata/mutations.json:
//
//	cd loopadmin && GOWORK=off go run ./mutate -map testdata/mutations.json -test <TestName>

var ctx = context.Background()

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "workspace", "ready-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// baseRequest is the fixture receipt's inputs: the request it records.
func baseRequest(t *testing.T) workspace.Request {
	t.Helper()
	r, err := workspace.Decode(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	return r.Inputs
}

func checkOf(r workspace.Receipt, input string) workspace.Result {
	for _, c := range r.Validation.Checks {
		if c.Input == input {
			return c.Result
		}
	}
	return ""
}

func holdReason(t *testing.T, err error) workspace.HoldReason {
	t.Helper()
	var h *workspace.Hold
	if !errors.As(err, &h) || !errors.Is(err, workspace.ErrHold) {
		t.Fatalf("got %v, want a typed hold", err)
	}
	return h.Reason
}

const otherRevision = "3f79bb7b435b05321651daefd374cdc681dc06fa"

// TestPreparedEnvironmentReadyFlow: a workspace is ready only when the exact
// repository revisions, the recipe, tools, skills, harness bundle, image,
// writable paths and effective policy all match; a mismatch of any one is
// not-ready with that check recorded, and a not-ready receipt is not
// admitted. Requests are validated and the owner confirmed current before
// any adapter call; an interrupted preparation leaves an unknown receipt that
// the next attempt reconciles.
// regression: agent-execution/02
func TestPreparedEnvironmentReadyFlow(t *testing.T) {
	req := baseRequest(t)
	env := fake.New()
	p := env.Port(workspace.Port{})

	r, err := p.Prepare(ctx, req)
	if err != nil || r.Outcome != workspace.OutcomeReady {
		t.Fatalf("exact inputs: got %v %v, want ready", r.Outcome, err)
	}
	if err := r.Admit(req.Invocation, req.Owner, req.Instance, r.Allocation.Workspace); err != nil {
		t.Fatalf("ready receipt not admitted: %v", err)
	}
	stored, ok := env.Receipt(workspace.BindingKey(req.Instance))
	if !ok || !reflect.DeepEqual(stored, r) {
		t.Fatal("the store does not hold the receipt Prepare returned")
	}
	b, _ := json.Marshal(r)
	if back, err := workspace.Decode(b); err != nil || !reflect.DeepEqual(back, r) {
		t.Fatalf("ready receipt does not round-trip: %v", err)
	}

	// The checkout lands on another revision (a ref moved): not ready.
	moved := fake.New()
	moved.Moved[req.Repositories[0].Name] = otherRevision
	r, err = moved.Port(workspace.Port{}).Prepare(ctx, req)
	if !errors.Is(err, workspace.ErrNotReady) || r.Outcome != workspace.OutcomeNotReady ||
		checkOf(r, "repositories") != workspace.ResultMismatch {
		t.Fatalf("wrong effective revision: got %v %v, want not-ready on repositories", r.Outcome, err)
	}
	if err := r.Admit(req.Invocation, req.Owner, req.Instance, r.Allocation.Workspace); !errors.Is(err, workspace.ErrNotReady) {
		t.Fatalf("not-ready receipt admitted: %v", err)
	}

	// Drift after ready: each mandatory input is checked on its own.
	for input, drift := range map[string]func(*workspace.Effective){
		"recipe":         func(e *workspace.Effective) { e.Environment.Recipe.Version = "4" },
		"tools":          func(e *workspace.Effective) { e.Environment.Tools[0].Version = "1.26.0" },
		"skills":         func(e *workspace.Effective) { e.Environment.Skills = nil },
		"harness_bundle": func(e *workspace.Effective) { e.Environment.HarnessBundle = e.Environment.Image },
		"image":          func(e *workspace.Effective) { e.Environment.Image = "" },
		"writable":       func(e *workspace.Effective) { e.Writable = append(e.Writable, "etc") },
		"policy":         func(e *workspace.Effective) { e.Policy.Network = "policies/network/open" },
	} {
		env := fake.New()
		p := env.Port(workspace.Port{})
		r, err := p.Prepare(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		env.Drift(r.Allocation.Workspace, drift)
		r, err = p.Prepare(ctx, req)
		if !errors.Is(err, workspace.ErrNotReady) || checkOf(r, input) != workspace.ResultMismatch {
			t.Fatalf("%s drift: got %v %v, want not-ready on %s", input, r.Outcome, err, input)
		}
		for _, c := range r.Validation.Checks {
			if c.Input != input && c.Result != workspace.ResultMatch {
				t.Fatalf("%s drift also mismatched %s", input, c.Input)
			}
		}
	}

	// An inexact revision is refused before any adapter call.
	bad := req
	bad.Repositories = []workspace.Repository{{Name: "example/app", Revision: "main"}}
	env = fake.New()
	if _, err := env.Port(workspace.Port{}).Prepare(ctx, bad); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("branch name as revision: got %v, want ErrInvalid", err)
	}
	if _, ok := env.Receipt(workspace.BindingKey(req.Instance)); ok || env.Applies != 0 {
		t.Fatal("an invalid request reached an adapter")
	}

	// A revoked owner prepares nothing.
	env = fake.New()
	env.Revoke(req.Owner.Key)
	if _, err := env.Port(workspace.Port{}).Prepare(ctx, req); !errors.Is(err, workspace.ErrUnauthorized) {
		t.Fatalf("revoked owner: got %v, want ErrUnauthorized", err)
	}
	if _, ok := env.Receipt(workspace.BindingKey(req.Instance)); ok {
		t.Fatal("a revoked owner left a receipt")
	}

	// Interrupted after allocation: the stored receipt is unknown, never
	// ready, and the retry reconciles it to ready.
	env = fake.New()
	env.FailWorkspace = true
	p = env.Port(workspace.Port{})
	if _, err := p.Prepare(ctx, req); !errors.Is(err, workspace.ErrAdapter) {
		t.Fatalf("adapter fault: got %v, want ErrAdapter", err)
	}
	if s, _ := env.Receipt(workspace.BindingKey(req.Instance)); s.Outcome != workspace.OutcomeUnknown {
		t.Fatalf("interrupted preparation stored %q, want unknown", s.Outcome)
	}
	if r, err := p.Prepare(ctx, req); err != nil || r.Outcome != workspace.OutcomeReady {
		t.Fatalf("reconciled retry: got %v %v, want ready", r.Outcome, err)
	}
}

// TestPreparedEnvironmentReuseAndPartialFailure: a ready receipt is reused
// only for the same invocation and the same current policy; a changed policy
// or invocation rebinds (current authority, policy applied, revisions
// re-read). Partial preparation never checks out over unpublished or unknown
// files, an unknown allocation holds, and release holds for a missing
// receipt, an expired allocation, an unknown allocation or unpublished files,
// delegating only a clean one to the retirement owner.
// regression: agent-execution/02
func TestPreparedEnvironmentReuseAndPartialFailure(t *testing.T) {
	req := baseRequest(t)
	env := fake.New()
	p := env.Port(workspace.Port{})
	first, err := p.Prepare(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	ws := first.Allocation.Workspace

	// Same inputs: reused, re-observed, nothing re-applied.
	if r, err := p.Prepare(ctx, req); err != nil || r.Outcome != workspace.OutcomeReady || env.Applies != 1 || env.Checkouts != 1 {
		t.Fatalf("reuse: got %v %v applies=%d checkouts=%d", r.Outcome, err, env.Applies, env.Checkouts)
	}

	// Changed policy: rebound with the new policy applied.
	changed := req
	changed.Policy.Network = "policies/network/egress-forge"
	r, err := p.Prepare(ctx, changed)
	if err != nil || r.Outcome != workspace.OutcomeReady || r.Inputs.Policy != changed.Policy || env.Applies != 2 {
		t.Fatalf("changed policy: got %v %v applies=%d, want a rebind to ready", r.Outcome, err, env.Applies)
	}
	if r.Allocation.Workspace != ws {
		t.Fatal("a rebind moved the instance to another workspace")
	}

	// Changed invocation: rebound; the earlier invocation's receipt is no
	// longer what the store admits.
	again := changed
	again.Invocation.ID = "inv-2"
	r, err = p.Prepare(ctx, again)
	if err != nil || r.Inputs.Invocation != again.Invocation || env.Applies != 3 {
		t.Fatalf("changed invocation: got %v applies=%d, want a rebind", err, env.Applies)
	}
	if err := r.Admit(req.Invocation, req.Owner, req.Instance, ws); !errors.Is(err, workspace.ErrMismatch) {
		t.Fatalf("receipt admitted for the earlier invocation: %v", err)
	}

	// A rebind needs current authority.
	env.Revoke(req.Owner.Key)
	third := again
	third.Invocation.ID = "inv-3"
	if _, err := p.Prepare(ctx, third); !errors.Is(err, workspace.ErrUnauthorized) {
		t.Fatalf("rebind under revoked authority: got %v", err)
	}
	if s, _ := env.Receipt(workspace.BindingKey(req.Instance)); s.Inputs.Invocation != again.Invocation {
		t.Fatal("a refused rebind changed the stored receipt")
	}

	// Partial: a new revision over unpublished files holds, never re-clones,
	// and leaves a partial receipt; unknown presence holds the same way.
	env = fake.New()
	p = env.Port(workspace.Port{})
	if _, err := p.Prepare(ctx, req); err != nil {
		t.Fatal(err)
	}
	moved := req
	moved.Repositories = []workspace.Repository{{Name: "example/app", Revision: otherRevision}}
	for _, pres := range []workspace.Presence{workspace.PresencePresent, workspace.PresenceUnknown} {
		env.SetUnpublished(ws, pres)
		if _, err := p.Prepare(ctx, moved); holdReason(t, err) != workspace.HoldUnpublished {
			t.Fatalf("%s files: want HoldUnpublished", pres)
		}
		if env.Checkouts != 1 {
			t.Fatalf("%s files: checked out over them", pres)
		}
		if s, _ := env.Receipt(workspace.BindingKey(req.Instance)); s.Outcome != workspace.OutcomePartial {
			t.Fatalf("%s files: stored %q, want partial", pres, s.Outcome)
		}
	}
	// Partial with an allocation that can no longer be found: hold.
	env.Forget(ws)
	if _, err := p.Prepare(ctx, moved); holdReason(t, err) != workspace.HoldUnknownAllocation {
		t.Fatal("partial over an unknown allocation: want HoldUnknownAllocation")
	}

	// Release.
	env = fake.New()
	p = env.Port(workspace.Port{})
	if err := p.Release(ctx, req.Instance, false); holdReason(t, err) != workspace.HoldNoReceipt {
		t.Fatal("release without a receipt: want HoldNoReceipt")
	}
	if _, err := p.Prepare(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := p.Release(ctx, req.Instance, true); holdReason(t, err) != workspace.HoldExpired {
		t.Fatal("expired release: want HoldExpired")
	}
	env.SetUnpublished(ws, workspace.PresencePresent)
	if err := p.Release(ctx, req.Instance, false); holdReason(t, err) != workspace.HoldUnpublished {
		t.Fatal("release over unpublished files: want HoldUnpublished")
	}
	env.SetUnpublished(ws, workspace.PresenceNone)
	env.RetireUnknown = true
	if err := p.Release(ctx, req.Instance, false); !errors.Is(err, workspace.ErrAdapter) {
		t.Fatalf("uncertain retirement: got %v, want ErrAdapter", err)
	}
	env.RetireUnknown = false
	if err := p.Release(ctx, req.Instance, false); err != nil || !slices.Equal(env.Retired, []string{ws}) {
		t.Fatalf("clean release: got %v retired=%v, want delegated", err, env.Retired)
	}
	env.Forget(ws)
	if err := p.Release(ctx, req.Instance, false); holdReason(t, err) != workspace.HoldUnknownAllocation {
		t.Fatal("release of an unknown allocation: want HoldUnknownAllocation")
	}
}

// TestPreparedEnvironmentSchemaAndSecrets: the schema matches the Go types (a
// planted drift is caught); decoding is strict; a receipt is admitted only
// for its own invocation, owner, instance and workspace; and credential
// material at any depth is refused, by both the receipt and the request, with
// errors that name the field and never the value.
// regression: agent-execution/02
func TestPreparedEnvironmentSchemaAndSecrets(t *testing.T) {
	raw := fixture(t)
	good, err := workspace.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	edit := func(f func(m map[string]any)) []byte {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		f(m)
		b, _ := json.Marshal(m)
		return b
	}
	obj := func(m map[string]any, keys ...string) map[string]any {
		for _, k := range keys {
			m = m[k].(map[string]any)
		}
		return m
	}
	for what, b := range map[string][]byte{
		"unknown root key":   edit(func(m map[string]any) { m["fallback"] = "fresh" }),
		"unknown nested key": edit(func(m map[string]any) { obj(m, "inputs", "policy")["inline"] = "allow" }),
		"null field":         edit(func(m map[string]any) { obj(m, "inputs")["writable"] = nil }),
		"wrong version":      edit(func(m map[string]any) { m["version"] = "prepared-environment/v2" }),
		"ready without check": edit(func(m map[string]any) {
			v := obj(m, "validation")
			v["checks"] = v["checks"].([]any)[1:]
		}),
		"ready without workspace": edit(func(m map[string]any) { delete(obj(m, "allocation"), "workspace") }),
		"repeated key":            []byte(strings.Replace(string(raw), `"outcome": "ready"`, `"outcome": "ready", "outcome": "ready"`, 1)),
		"trailing data":           append(slices.Clone(raw), []byte(" {}")...),
	} {
		if _, err := workspace.Decode(b); !errors.Is(err, workspace.ErrInvalid) {
			t.Fatalf("%s: got %v, want ErrInvalid", what, err)
		}
	}

	// Wrong invocation, owner, instance or allocation.
	in := good.Inputs
	otherInv, otherOwner, otherInst := in.Invocation, in.Owner, in.Instance
	otherInv.ID = "inv-9"
	otherOwner.Generation = 2
	otherInst.ID = "w9"
	for what, err := range map[string]error{
		"invocation": good.Admit(otherInv, in.Owner, in.Instance, good.Allocation.Workspace),
		"owner":      good.Admit(in.Invocation, otherOwner, in.Instance, good.Allocation.Workspace),
		"instance":   good.Admit(in.Invocation, in.Owner, otherInst, good.Allocation.Workspace),
		"allocation": good.Admit(in.Invocation, in.Owner, in.Instance, "ws-9"),
	} {
		if !errors.Is(err, workspace.ErrMismatch) {
			t.Fatalf("wrong %s: got %v, want ErrMismatch", what, err)
		}
	}

	// Credential material, nested at every kind of depth. The plant is built
	// at run time so no credential-shaped literal sits in the source.
	secret := "AKIA" + strings.Repeat("Q", 16)
	source := map[string]any{"cell": "cell-a", "role": "worker", "id": secret, "generation": 1}
	for what, b := range map[string][]byte{
		"inputs.restore.source.id": edit(func(m map[string]any) {
			obj(m, "inputs")["restore"] = map[string]any{"source": source, "needs": map[string]any{"files": true}}
		}),
		"restored.source.id": edit(func(m map[string]any) {
			m["restored"] = map[string]any{"source": source, "preserved": map[string]any{"files": true}}
		}),
		"inputs.environment.tools[0].version": edit(func(m map[string]any) {
			obj(m, "inputs", "environment")["tools"].([]any)[0].(map[string]any)["version"] = secret
		}),
		"artifacts[0].ref": edit(func(m map[string]any) {
			m["artifacts"].([]any)[0].(map[string]any)["ref"] = secret
		}),
		"validation.observer": edit(func(m map[string]any) { obj(m, "validation")["observer"] = secret }),
	} {
		_, err := workspace.Decode(b)
		if !errors.Is(err, runner.ErrCredential) {
			t.Fatalf("%s: got %v, want ErrCredential", what, err)
		}
		if !strings.Contains(err.Error(), what) || strings.Contains(err.Error(), secret) {
			t.Fatalf("%s: error does not name the field or carries the value", what)
		}
	}

	// The request side: refused before any adapter call, sanitized.
	req := good.Inputs
	src := req.Instance
	src.ID = secret
	req.Restore = &workspace.Restore{Source: src, Needs: runner.PreservationNeed{Files: true}}
	env := fake.New()
	_, err = env.Port(workspace.Port{}).Prepare(ctx, req)
	if !errors.Is(err, runner.ErrCredential) || strings.Contains(err.Error(), secret) {
		t.Fatalf("credential in a nested request field: got %v", err)
	}
	if _, ok := env.Receipt(workspace.BindingKey(req.Instance)); ok {
		t.Fatal("a credential-bearing request reached the store")
	}
	// Other refusals are sanitized too: no planted value in the text.
	planted := "planted-value-xyz"
	bad := good.Inputs
	bad.Repositories = []workspace.Repository{{Name: planted, Revision: planted}}
	if err := bad.Validate(); err == nil || strings.Contains(err.Error(), planted) {
		t.Fatalf("invalid revision: got %v, want a refusal without the value", err)
	}

	// Schema to source.
	sraw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "prepared-environment-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(sraw, &s); err != nil {
		t.Fatal(err)
	}
	if d := schemaDrift(s); len(d) != 0 {
		t.Fatalf("schema drifted from the Go types:\n%s", strings.Join(d, "\n"))
	}
	for what, plant := range map[string]func(map[string]any){
		"dropped policy":    func(s map[string]any) { delete(props(def(s, "request")), "policy") },
		"optional revision": func(s map[string]any) { def(s, "repository")["required"] = []any{"name"} },
		"open environment":  func(s map[string]any) { def(s, "environment")["additionalProperties"] = true },
		"outcome collapsed": func(s map[string]any) { def(s, "outcome")["enum"] = []any{"ready", "unknown"} },
		"inline secret":     func(s map[string]any) { props(def(s, "policy"))["token"] = map[string]any{"type": "string"} },
	} {
		var planted map[string]any
		_ = json.Unmarshal(sraw, &planted)
		plant(planted)
		if d := schemaDrift(planted); len(d) == 0 {
			t.Fatalf("a planted schema drift (%s) went undetected", what)
		}
	}
}

func def(s map[string]any, name string) map[string]any {
	return s["$defs"].(map[string]any)[name].(map[string]any)
}

func props(n map[string]any) map[string]any {
	p, _ := n["properties"].(map[string]any)
	return p
}

func strs(v any) []string {
	var out []string
	l, _ := v.([]any)
	for _, x := range l {
		s, _ := x.(string)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// schemaDrift compares the receipt schema with the Go types: each object's
// properties are the type's json tags, its required list is the tags without
// omitempty, every object is closed, and the outcome and result enums are the
// Go constants.
func schemaDrift(s map[string]any) []string {
	var drift []string
	for _, p := range []struct {
		name   string
		schema map[string]any
		typ    reflect.Type
	}{
		{"receipt", s, reflect.TypeOf(workspace.Receipt{})},
		{"request", def(s, "request"), reflect.TypeOf(workspace.Request{})},
		{"invocation", def(s, "invocation"), reflect.TypeOf(workspace.Invocation{})},
		{"owner", def(s, "owner"), reflect.TypeOf(workspace.Owner{})},
		{"instance", def(s, "instance"), reflect.TypeOf(workspace.Instance{})},
		{"repository", def(s, "repository"), reflect.TypeOf(workspace.Repository{})},
		{"component", def(s, "component"), reflect.TypeOf(workspace.Component{})},
		{"recipe", def(s, "recipe"), reflect.TypeOf(workspace.Recipe{})},
		{"environment", def(s, "environment"), reflect.TypeOf(workspace.Environment{})},
		{"policy", def(s, "policy"), reflect.TypeOf(workspace.Policy{})},
		{"preservationNeed", def(s, "preservationNeed"), reflect.TypeOf(runner.PreservationNeed{})},
		{"restore", def(s, "restore"), reflect.TypeOf(workspace.Restore{})},
		{"allocation", def(s, "allocation"), reflect.TypeOf(workspace.Allocation{})},
		{"check", def(s, "check"), reflect.TypeOf(workspace.Check{})},
		{"validation", def(s, "validation"), reflect.TypeOf(workspace.Validation{})},
		{"artifact", def(s, "artifact"), reflect.TypeOf(workspace.Artifact{})},
		{"restored", def(s, "restored"), reflect.TypeOf(workspace.Restored{})},
	} {
		var ps, tags, required []string
		for k := range props(p.schema) {
			ps = append(ps, k)
		}
		for i := 0; i < p.typ.NumField(); i++ {
			tag, opts, _ := strings.Cut(p.typ.Field(i).Tag.Get("json"), ",")
			tags = append(tags, tag)
			if !strings.Contains(opts, "omitempty") {
				required = append(required, tag)
			}
		}
		sort.Strings(ps)
		sort.Strings(tags)
		sort.Strings(required)
		if !reflect.DeepEqual(ps, tags) {
			drift = append(drift, p.name+": properties "+strings.Join(ps, ",")+" != Go tags "+strings.Join(tags, ","))
		}
		if got := strs(p.schema["required"]); !slices.Equal(got, required) {
			drift = append(drift, p.name+": required "+strings.Join(got, ",")+" != Go non-omitempty tags "+strings.Join(required, ","))
		}
		if p.schema["additionalProperties"] != false {
			drift = append(drift, p.name+": additionalProperties is not false")
		}
	}
	for name, want := range map[string][]string{
		"outcome": {string(workspace.OutcomeUnknown), string(workspace.OutcomePartial), string(workspace.OutcomeNotReady), string(workspace.OutcomeReady)},
		"result":  {string(workspace.ResultMatch), string(workspace.ResultMismatch)},
	} {
		sort.Strings(want)
		if got := strs(def(s, name)["enum"]); !slices.Equal(got, want) {
			drift = append(drift, name+": enum "+strings.Join(got, ",")+" != Go "+strings.Join(want, ","))
		}
	}
	if c, _ := props(s)["version"].(map[string]any); c["const"] != workspace.Version {
		drift = append(drift, "receipt: version const is not "+workspace.Version)
	}
	return drift
}
