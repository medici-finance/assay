package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
	"github.com/medici-finance/assay/loopadmin/runner/fake"
)

// The receipt and observation ext tests. Runner v1 section 7: an adapter's
// ext values are for an extension the request carried and the adapter
// declares, and they carry no credential material. The Client screens every
// ext map it is handed before it returns or records it, and an invocation
// whose adapter broke the rule is never accepted. Mutations: the entries for
// the same test names in testdata/mutations.json.

// extAdapter overlays ext values on the wrapped adapter's receipts and
// observations, whatever the request carried. Everything else is forwarded.
type extAdapter struct {
	runner.Adapter

	mu       sync.Mutex
	receipt  map[string]json.RawMessage
	observed map[string]json.RawMessage
}

func (e *extAdapter) overlay(base, extra map[string]json.RawMessage) map[string]json.RawMessage {
	if len(extra) == 0 {
		return base
	}
	out := map[string]json.RawMessage{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func (e *extAdapter) setObserved(ext map[string]json.RawMessage) {
	e.mu.Lock()
	e.observed = ext
	e.mu.Unlock()
}

func (e *extAdapter) Start(ctx context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	rec, err := e.Adapter.Start(ctx, req)
	e.mu.Lock()
	defer e.mu.Unlock()
	rec.Extensions = e.overlay(rec.Extensions, e.receipt)
	return rec, err
}

func (e *extAdapter) Observe(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	obs, err := e.Adapter.Observe(ctx, ref)
	e.mu.Lock()
	defer e.mu.Unlock()
	obs.Extensions = e.overlay(obs.Extensions, e.observed)
	return obs, err
}

func (e *extAdapter) Reconcile(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return e.Observe(ctx, ref)
}

// QualifiedProfiles forwards the wrapped adapter's declaration.
func (e *extAdapter) QualifiedProfiles() []runner.QualifiedProfile {
	if d, ok := e.Adapter.(runner.ProfileDeclarer); ok {
		return d.QualifiedProfiles()
	}
	return nil
}

// credentialShape builds a credential-shaped string at run time, so no
// literal in the tree looks like one.
func credentialShape() string { return "gh" + "p_" + strings.Repeat("a1", 20) }

const otherExt = "notes/v1"

func otherValue(s string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"note": s})
	return b
}

// screened drives req to a finished observation over e and returns the
// errors of Start, Observe and AcceptResult. It asserts that a refused
// observation hands back nothing and that no refusal echoes the ext key or
// value it refused.
func screened(t *testing.T, what string, e *extAdapter, f *fake.Adapter, req runner.LaunchRequest, payload ...string) (start, observe, accept error) {
	t.Helper()
	ctx := context.Background()
	c, claims := clientFor(e)
	claims.Set(req.Authority.Key, req.Authority.Generation)
	ref, st, start := c.Start(ctx, req)
	if start != nil && st != runner.StateRunning {
		t.Fatalf("%s: a refused receipt still means a launch: state %q", what, st)
	}
	f.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
	if ref == (runner.Ref{}) {
		ref = req.Ref()
	}
	obs, observe := c.Observe(ctx, ref)
	if observe != nil && (obs.Extensions != nil || obs.Result != nil || obs.State != "") {
		t.Fatalf("%s: a refused observation was handed back: %+v", what, obs)
	}
	_, accept = c.AcceptResult(ctx, ref)
	for _, err := range []error{start, observe, accept} {
		if err == nil {
			continue
		}
		for _, p := range append([]string{otherExt}, payload...) {
			if strings.Contains(err.Error(), p) {
				t.Fatalf("%s: a refusal echoes what it refused: %v", what, err)
			}
		}
	}
	return start, observe, accept
}

func wantIs(t *testing.T, what string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// TestRunnerAdapterExtensionsScreened: an ext value on a receipt or an
// observation is refused when the request did not carry that extension or the
// adapter does not declare it, and when any value carries credential material,
// in both modes; the refused observation is neither returned nor recorded,
// and the invocation's result is not accepted even after a later clean
// observation. An extension the request carried and the adapter declares
// still passes.
// regression: agent-execution/01
func TestRunnerAdapterExtensionsScreened(t *testing.T) {
	secret := credentialShape()
	for _, base := range modes("ext-screen") {
		mode := string(base.Mode)

		// An unlisted extension on the observation of a request that carried none.
		f := fake.New(fake.FullCapabilities())
		e := &extAdapter{Adapter: f, observed: map[string]json.RawMessage{otherExt: otherValue("benign")}}
		_, obsErr, accErr := screened(t, mode+": unlisted extension", e, f, base)
		wantIs(t, mode+": unlisted extension on the observation", obsErr, runner.ErrMalformedResult)
		wantIs(t, mode+": unlisted extension at acceptance", accErr, runner.ErrMalformedResult)

		// Extension values are screened: a credential-shaped value under an
		// unlisted key refuses as a credential first.
		f = fake.New(fake.FullCapabilities())
		e = &extAdapter{Adapter: f, observed: map[string]json.RawMessage{otherExt: otherValue(secret)}}
		_, obsErr, accErr = screened(t, mode+": screened value", e, f, base, secret)
		wantIs(t, mode+": screened value on the observation", obsErr, runner.ErrCredential)
		wantIs(t, mode+": screened value at acceptance", accErr, runner.ErrCredential)

		// Carried by the request, but the adapter does not declare it.
		carried := base
		carried.Extensions = map[string]json.RawMessage{otherExt: otherValue("asked")}
		f = fake.New(fake.FullCapabilities())
		e = &extAdapter{Adapter: f, observed: map[string]json.RawMessage{otherExt: otherValue("benign")}}
		_, obsErr, accErr = screened(t, mode+": undeclared extension", e, f, carried)
		wantIs(t, mode+": undeclared extension on the observation", obsErr, runner.ErrMalformedResult)
		wantIs(t, mode+": undeclared extension at acceptance", accErr, runner.ErrMalformedResult)

		// The receipt is screened too, and the invocation stays unacceptable
		// though every later observation is clean.
		f = fake.New(fake.FullCapabilities())
		e = &extAdapter{Adapter: f, receipt: map[string]json.RawMessage{otherExt: otherValue("benign")}}
		startErr, obsErr, accErr := screened(t, mode+": unlisted extension on the receipt", e, f, base)
		wantIs(t, mode+": unlisted extension on the receipt", startErr, runner.ErrMalformedResult)
		if obsErr != nil {
			t.Fatalf("%s: a clean observation after a refused receipt: %v", mode, obsErr)
		}
		wantIs(t, mode+": acceptance after a refused receipt", accErr, runner.ErrMalformedResult)

		// A refused observation followed by a clean one: still not accepted.
		ctx := context.Background()
		f = fake.New(fake.FullCapabilities())
		e = &extAdapter{Adapter: f, observed: map[string]json.RawMessage{otherExt: otherValue(secret)}}
		c, claims := clientFor(e)
		claims.Set(base.Authority.Key, base.Authority.Generation)
		ref, _, err := c.Start(ctx, base)
		if err != nil {
			t.Fatalf("%s: start: %v", mode, err)
		}
		f.Complete(ref, conformance.GoodResult(base), conformance.FullUsage())
		if _, err := c.Observe(ctx, ref); !errors.Is(err, runner.ErrCredential) {
			t.Fatalf("%s: screened observation: got %v", mode, err)
		}
		if st, _ := c.State(ref); st != runner.StateRunning {
			t.Fatalf("%s: a refused observation was recorded: state %q", mode, st)
		}
		e.setObserved(nil)
		if _, err := c.Observe(ctx, ref); err != nil {
			t.Fatalf("%s: clean observation: %v", mode, err)
		}
		if _, err := c.AcceptResult(ctx, ref); !errors.Is(err, runner.ErrCredential) {
			t.Fatalf("%s: an invocation whose adapter sent a screened value must not be accepted: got %v", mode, err)
		}

		// Positive control: carried by the request and declared by the adapter.
		caps := fake.FullCapabilities()
		caps.Extensions = []string{otherExt}
		f = fake.New(caps)
		e = &extAdapter{Adapter: f, observed: map[string]json.RawMessage{otherExt: otherValue("answer")}}
		startErr, obsErr, accErr = screened(t, mode+": declared extension", e, f, carried)
		if startErr != nil || obsErr != nil || accErr != nil {
			t.Fatalf("%s: a carried and declared extension must pass: %v %v %v", mode, startErr, obsErr, accErr)
		}
	}
}

// TestExecutionReportScreened: an execution report is screened like every
// other ext value, on the receipt and on the observation, including a part of
// the report acceptance does not otherwise compare. A clean report on the
// same launch is accepted.
// regression: agent-execution/01
func TestExecutionReportScreened(t *testing.T) {
	p := conformance.QualifiedProfile()
	secret := credentialShape()
	restored := runner.Preservation{
		Files: runner.Support(secret), Conversation: runner.SupportNo, ProcessMemory: runner.SupportNo,
		Descendants: runner.SupportNo, PendingInteractions: runner.SupportNo,
	}
	report := runner.ReportExtension(runner.ExecutionReport{Digest: p.Digest, Harness: p.Harness, Backend: p.Backend, Restored: &restored})
	for _, base := range modes("report-screen") {
		mode := string(base.Mode)
		req := conformance.ProfileRequest(base, conformance.Requirement(p, runner.Needs{}))

		f := newAdapter(p)
		e := &extAdapter{Adapter: f, observed: report}
		_, obsErr, accErr := screened(t, mode+": report on the observation", e, f, req, secret)
		wantIs(t, mode+": screened report on the observation", obsErr, runner.ErrCredential)
		wantIs(t, mode+": screened report at acceptance", accErr, runner.ErrCredential)

		f = newAdapter(p)
		e = &extAdapter{Adapter: f, receipt: report}
		startErr, _, accErr := screened(t, mode+": report on the receipt", e, f, req, secret)
		wantIs(t, mode+": screened report on the receipt", startErr, runner.ErrCredential)
		wantIs(t, mode+": acceptance after a screened receipt", accErr, runner.ErrCredential)

		// Positive control.
		f = newAdapter(p)
		e = &extAdapter{Adapter: f}
		startErr, obsErr, accErr = screened(t, mode+": clean report", e, f, req)
		if startErr != nil || obsErr != nil || accErr != nil {
			t.Fatalf("%s: a clean report must be accepted: %v %v %v", mode, startErr, obsErr, accErr)
		}
	}
}

// TestExecutionProfileEvidenceRequired: a qualified profile that cites no
// qualification evidence is refused, sealed or decoded, and an adapter that
// declares one admits nothing under it. A profile citing evidence still
// validates.
// regression: agent-execution/01
func TestExecutionProfileEvidenceRequired(t *testing.T) {
	p := conformance.QualifiedProfile()
	if err := p.Validate(); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	for what, evidence := range map[string][]string{"no evidence": nil, "empty evidence": {}} {
		bare := p
		bare.Qualification.Evidence = evidence
		bare = conformance.Seal(bare)
		if err := bare.Validate(); !errors.Is(err, runner.ErrInvalidRequest) {
			t.Fatalf("%s: got %v, want %v", what, err, runner.ErrInvalidRequest)
		}
		b, err := json.Marshal(bare)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runner.DecodeQualifiedProfile(b); !errors.Is(err, runner.ErrInvalidRequest) {
			t.Fatalf("%s decoded: got %v, want %v", what, err, runner.ErrInvalidRequest)
		}
		for _, base := range modes("evidence") {
			req := conformance.ProfileRequest(base, conformance.Requirement(bare, runner.Needs{}))
			refusedBeforeStart(t, string(base.Mode)+": "+what, newAdapter(bare), req, runner.ErrInvalidRequest)
		}
	}
}

// TestRunnerDocumentSizeCap: a request, a qualified profile declaration and a
// requirement longer than runner.MaxDocumentBytes are refused before they are
// parsed; the same documents at the cap still decode.
// regression: agent-execution/01
func TestRunnerDocumentSizeCap(t *testing.T) {
	pad := func(b []byte, n int) []byte {
		return append(append([]byte(nil), b...), []byte(strings.Repeat(" ", n-len(b)))...)
	}
	for name, decode := range map[string]func([]byte) error{
		"runner/standing-desk-request.json": func(b []byte) error { _, err := runner.DecodeRequest(b); return err },
		"executionprofile/qualified-profile.json": func(b []byte) error {
			_, err := runner.DecodeQualifiedProfile(b)
			return err
		},
	} {
		dir, file, _ := strings.Cut(name, "/")
		var b []byte
		if dir == "runner" {
			req := decodeFixture(t, dir, file)
			b, _ = json.Marshal(req)
		} else {
			b = profileFixture(t, file)
		}
		if err := decode(pad(b, runner.MaxDocumentBytes)); err != nil {
			t.Fatalf("%s at the cap must decode: %v", name, err)
		}
		if err := decode(pad(b, runner.MaxDocumentBytes+1)); !errors.Is(err, runner.ErrInvalidRequest) {
			t.Fatalf("%s over the cap: got %v, want %v", name, err, runner.ErrInvalidRequest)
		}
	}
	need, _ := json.Marshal(conformance.Requirement(conformance.QualifiedProfile(), runner.Needs{}))
	if _, err := runner.DecodeProfileRequirement(pad(need, runner.MaxDocumentBytes)); err != nil {
		t.Fatalf("a requirement at the cap must decode: %v", err)
	}
	if _, err := runner.DecodeProfileRequirement(pad(need, runner.MaxDocumentBytes+1)); !errors.Is(err, runner.ErrInvalidRequest) {
		t.Fatalf("a requirement over the cap: got %v", err)
	}
}
