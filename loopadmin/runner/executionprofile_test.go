package runner_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
	"github.com/medici-finance/assay/loopadmin/runner/fake"
)

// The execution-profile/v1 tests. Each named test is a Verify row of
// agent-execution/01; its executable mutations are the entries for the same
// test name in testdata/mutations.json:
//
//	cd loopadmin && GOWORK=off go run ./mutate -map testdata/mutations.json -test <TestName>

func profileFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "executionprofile", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeFixture(t *testing.T, dir, name string) runner.LaunchRequest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}
	req, err := runner.DecodeRequest(b)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return req
}

// oldAdapter does not know the extension; newAdapter declares the profiles.
func oldAdapter() *fake.Adapter { return fake.New(fake.FullCapabilities()) }

func newAdapter(p ...runner.QualifiedProfile) *fake.Adapter {
	a := fake.New(fake.FullCapabilities())
	a.Qualify(p...)
	return a
}

func clientFor(a runner.Adapter) (*runner.Client, *conformance.Claims) {
	claims := conformance.NewClaims()
	return runner.NewClient(a, claims), claims
}

// refusedBeforeStart asserts the client refused req with want BEFORE asking
// the adapter: no invocation recorded, no launch begun, and not a definite
// failure the adapter returned.
func refusedBeforeStart(t *testing.T, what string, a *fake.Adapter, req runner.LaunchRequest, want error) {
	t.Helper()
	c, claims := clientFor(a)
	claims.Set(req.Authority.Key, req.Authority.Generation)
	_, _, err := c.Start(context.Background(), req)
	if !errors.Is(err, want) {
		t.Fatalf("%s: got %v, want %v", what, err, want)
	}
	if errors.Is(err, runner.ErrDefiniteFailure) {
		t.Fatalf("%s: refused by the adapter, not before Start: %v", what, err)
	}
	if _, err := c.State(req.Ref()); !errors.Is(err, runner.ErrUnknownInvocation) {
		t.Fatalf("%s: a refused launch left an invocation: %v", what, err)
	}
	if a.Starts() != 0 {
		t.Fatalf("%s: the adapter began %d launches", what, a.Starts())
	}
}

// accept runs req to a finished observation and returns the client's
// acceptance verdict. A refused Start fails the test.
func accept(t *testing.T, what string, a *fake.Adapter, req runner.LaunchRequest) (runner.Accepted, error) {
	t.Helper()
	ctx := context.Background()
	c, claims := clientFor(a)
	claims.Set(req.Authority.Key, req.Authority.Generation)
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatalf("%s: start: %v", what, err)
	}
	a.Complete(ref, conformance.GoodResult(req), conformance.FullUsage())
	_, _ = c.Observe(ctx, ref) // acceptance re-judges whatever the observation recorded
	return c.AcceptResult(ctx, ref)
}

func modes(id string) []runner.LaunchRequest {
	return []runner.LaunchRequest{conformance.StandingRequest(id), conformance.WorkflowRequest(id)}
}

// TestExecutionProfileV1Compatibility: v1 requests without the extension run
// exactly as before on old and new adapters; a request that requires the
// extension is refused before Start by an adapter that does not declare it,
// and by one that declares it when the requirement is absent or names a
// profile it does not declare.
// regression: agent-execution/01
func TestExecutionProfileV1Compatibility(t *testing.T) {
	p := conformance.QualifiedProfile()
	for _, name := range []string{"standing-desk-request.json", "workflow-stage-request.json"} {
		req := decodeFixture(t, "runner", name)
		for adapter, a := range map[string]*fake.Adapter{"old adapter": oldAdapter(), "new adapter": newAdapter(p)} {
			acc, err := accept(t, name+" on the "+adapter, a, req)
			if err != nil {
				t.Fatalf("%s on the %s: a v1 request must still be accepted: %v", name, adapter, err)
			}
			if acc.Execution != nil {
				t.Fatalf("%s on the %s: a launch without the extension carries no execution report", name, adapter)
			}
		}
	}

	for _, name := range []string{"standing-desk-profile-request.json", "workflow-stage-profile-request.json"} {
		req := decodeFixture(t, "executionprofile", name)
		if _, carried, err := req.ExecutionProfile(); !carried || err != nil {
			t.Fatalf("%s: the fixture's requirement must decode: carried=%v %v", name, carried, err)
		}
		// An old adapter does not declare the extension: the v1 negotiation
		// itself refuses the mandatory requirement ...
		if err := req.CheckCapabilities(oldAdapter().Capabilities()); !errors.Is(err, runner.ErrUnsupportedMandatory) {
			t.Fatalf("%s: an old adapter's capabilities must refuse the required extension: %v", name, err)
		}
		// ... and the client refuses before Start.
		refusedBeforeStart(t, name+" on an old adapter", oldAdapter(), req, runner.ErrUnsupportedMandatory)

		// Carried but not required, the value is a v1 optional extension: an
		// old adapter ignores it and runs the launch.
		optional := req
		optional.Require = nil
		acc, err := accept(t, name+" optional on an old adapter", oldAdapter(), optional)
		if err != nil || acc.Execution != nil {
			t.Fatalf("%s: an old adapter must ignore an optional extension: %v %+v", name, err, acc.Execution)
		}

		// A new adapter refuses a requirement that names no value ...
		missing := req
		missing.Extensions = nil
		refusedBeforeStart(t, name+" required without a value", newAdapter(p), missing, runner.ErrInvalidRequest)
		// ... and one pinning a profile it does not declare.
		other := p
		other.Qualification.Limits = []string{"another qualification"}
		other = conformance.Seal(other)
		refusedBeforeStart(t, name+" with an undeclared profile", newAdapter(other), req, runner.ErrUnsupportedMandatory)

		// Positive control: the adapter that declares the pinned profile runs
		// it, and acceptance carries the checked report.
		acc, err = accept(t, name+" on a qualified adapter", newAdapter(p), req)
		if err != nil {
			t.Fatalf("%s: a qualified launch must be accepted: %v", name, err)
		}
		if acc.Execution == nil || acc.Execution.Digest != p.Digest || acc.Execution.Harness != p.Harness || acc.Execution.Backend != p.Backend {
			t.Fatalf("%s: the accepted report must name the pinned tuple: %+v", name, acc.Execution)
		}
		if acc.ActualModel != p.Model.Model {
			t.Fatalf("%s: the actual model is the observation's, got %q", name, acc.ActualModel)
		}
	}
}

func resumeRequest(base runner.LaunchRequest, p runner.QualifiedProfile, need runner.PreservationNeed) runner.LaunchRequest {
	base.Resume = &runner.SessionRef{SessionID: "session-1", Role: base.Packet.Role, ProfileID: base.Profile.ID}
	return conformance.ProfileRequest(base, conformance.Requirement(p, runner.Needs{Resume: &need}))
}

func fullPreservation() runner.Preservation {
	y := runner.SupportYes
	return runner.Preservation{Files: y, Conversation: y, ProcessMemory: y, Descendants: y, PendingInteractions: y}
}

// TestExecutionProfilePreservationFlow: a profile whose resume restores files
// only cannot satisfy a launch that needs the conversation or process memory,
// in either caller mode, though v1's resume and snapshot capabilities are
// declared; an unknown dimension refuses as unknown, not as unsupported; and a
// run that restored only files is refused at acceptance even when the profile
// declared more.
// regression: agent-execution/01
func TestExecutionProfilePreservationFlow(t *testing.T) {
	filesOnly := conformance.QualifiedProfile()
	declaresAll := filesOnly
	declaresAll.Preservation.Resume = fullPreservation()
	declaresAll = conformance.Seal(declaresAll)
	if caps := oldAdapter().Capabilities(); !caps.Resume || !caps.Snapshot {
		t.Fatal("the fixture adapter must declare v1 resume and snapshot, so neither is mistaken for preservation")
	}

	for _, base := range modes("resume") {
		mode := string(base.Mode)
		acc, err := accept(t, mode+": files", newAdapter(filesOnly), resumeRequest(base, filesOnly, runner.PreservationNeed{Files: true}))
		if err != nil {
			t.Fatalf("%s: a files-only need on a files-preserving profile must run: %v", mode, err)
		}
		if acc.Execution == nil || acc.Execution.Restored == nil || acc.Execution.Restored.Files != runner.SupportYes {
			t.Fatalf("%s: the accepted report must show the files restored: %+v", mode, acc.Execution)
		}

		for what, need := range map[string]runner.PreservationNeed{
			"conversation":   {Conversation: true},
			"process memory": {ProcessMemory: true},
			"everything":     {Files: true, Conversation: true, ProcessMemory: true},
		} {
			refusedBeforeStart(t, mode+": "+what+" on files-only", newAdapter(filesOnly),
				resumeRequest(base, filesOnly, need), runner.ErrUnsupportedMandatory)
		}

		a := newAdapter(filesOnly)
		req := resumeRequest(base, filesOnly, runner.PreservationNeed{Descendants: true})
		refusedBeforeStart(t, mode+": descendants", a, req, runner.ErrCapabilityUnknown)
		c, _ := clientFor(newAdapter(filesOnly))
		if _, _, err := c.Start(context.Background(), req); errors.Is(err, runner.ErrUnsupportedMandatory) {
			t.Fatalf("%s: an unknown dimension must not be reported as unsupported: %v", mode, err)
		}

		// Acceptance: the profile declares full preservation, but this run
		// restored only the files.
		need := runner.PreservationNeed{Conversation: true, ProcessMemory: true}
		partial := runner.Preservation{
			Files: runner.SupportYes, Conversation: runner.SupportNo, ProcessMemory: runner.SupportNo,
			Descendants: runner.SupportNo, PendingInteractions: runner.SupportNo,
		}
		a = newAdapter(declaresAll)
		a.ObserveRestored(partial)
		if _, err := accept(t, mode+": files-only restoration", a, resumeRequest(base, declaresAll, need)); !errors.Is(err, runner.ErrUnsupportedMandatory) {
			t.Fatalf("%s: a files-only restoration must not satisfy conversation or process preservation, got %v", mode, err)
		}
		partial.Conversation, partial.ProcessMemory = runner.SupportUnknown, runner.SupportYes
		a = newAdapter(declaresAll)
		a.ObserveRestored(partial)
		if _, err := accept(t, mode+": unknown conversation", a, resumeRequest(base, declaresAll, need)); !errors.Is(err, runner.ErrCapabilityUnknown) {
			t.Fatalf("%s: an unknown restored conversation must refuse as unknown, got %v", mode, err)
		}
		// Positive control: a run that restored everything it needed.
		a = newAdapter(declaresAll)
		if _, err := accept(t, mode+": full restoration", a, resumeRequest(base, declaresAll, need)); err != nil {
			t.Fatalf("%s: a full restoration must be accepted: %v", mode, err)
		}
	}
}

// TestExecutionProfileBindingAndSchema: a substituted harness, backend, model,
// runner profile or role, and a declaration whose content drifted from its
// digest, are refused before Start; a substitution at run time is refused at
// acceptance; with the client check bypassed the adapter still launches
// nothing unqualified, and with the contract satisfied the caller still
// refuses a result that did not run on its current profile; and the schema
// stays in step with the Go types and refuses the malformed fixtures.
// regression: agent-execution/01
func TestExecutionProfileBindingAndSchema(t *testing.T) {
	ctx := context.Background()
	p := conformance.QualifiedProfile()

	for _, base := range modes("binding") {
		mode := string(base.Mode)
		good := conformance.Requirement(p, runner.Needs{})
		subs := map[string]runner.LaunchRequest{}
		r := good
		r.Backend.Platform = "linux-amd64"
		subs["backend"] = conformance.ProfileRequest(base, r)
		r = good
		r.Harness.Version = "1.4.3"
		subs["harness"] = conformance.ProfileRequest(base, r)
		r = good
		r.Model.Profile = "model-profile-b"
		subs["model profile"] = conformance.ProfileRequest(base, r)
		q := conformance.ProfileRequest(base, good)
		q.Profile.ID = "profile-b"
		subs["runner profile"] = q
		q = conformance.ProfileRequest(base, good)
		q.Profile.Model = "model-b"
		subs["pinned model"] = q
		q = conformance.ProfileRequest(base, good)
		q.Packet.Role = "reviewer"
		subs["role"] = q
		for what, req := range subs {
			refusedBeforeStart(t, mode+": substituted "+what, newAdapter(p), req, runner.ErrProfileMismatch)
		}

		// A declaration whose content drifted under its old digest.
		drifted := p
		drifted.Backend.Platform = "linux-amd64"
		refusedBeforeStart(t, mode+": drifted declaration", newAdapter(drifted),
			conformance.ProfileRequest(base, conformance.Requirement(drifted, runner.Needs{})), runner.ErrProfileMismatch)

		// A backend substituted at run time is caught on the receipt, the
		// observation and at acceptance.
		a := newAdapter(p)
		a.SubstituteBackend(runner.Backend{Profile: "backend-remote", Platform: "linux-amd64"})
		req := conformance.ProfileRequest(base, good)
		c, claims := clientFor(a)
		claims.Set(req.Authority.Key, req.Authority.Generation)
		ref, _, err := c.Start(ctx, req)
		if !errors.Is(err, runner.ErrProfileMismatch) {
			t.Fatalf("%s: a substituted backend on the receipt: got %v", mode, err)
		}
		a.Complete(ref, conformance.GoodResult(req), conformance.FullUsage())
		if _, err := c.Observe(ctx, ref); !errors.Is(err, runner.ErrProfileMismatch) {
			t.Fatalf("%s: a substituted backend on the observation: got %v", mode, err)
		}
		if _, err := c.AcceptResult(ctx, ref); !errors.Is(err, runner.ErrProfileMismatch) {
			t.Fatalf("%s: a substituted backend must not be accepted: got %v", mode, err)
		}

		// An observation reporting another qualified profile's digest.
		a = newAdapter(p)
		h := conformance.NewHostile(a)
		c, claims = clientFor(h)
		claims.Set(req.Authority.Key, req.Authority.Generation)
		if ref, _, err = c.Start(ctx, req); err != nil {
			t.Fatalf("%s: start: %v", mode, err)
		}
		res := conformance.GoodResult(req)
		h.Report(ref, runner.Observation{
			State: runner.StateFinished, ActualModel: p.Model.Model, Usage: conformance.FullUsage(), Result: &res,
			Extensions: runner.ReportExtension(runner.ExecutionReport{Digest: "sha256:" + fake.Digest("another profile"), Harness: p.Harness, Backend: p.Backend}),
		})
		_, _ = c.Observe(ctx, ref)
		if _, err := c.AcceptResult(ctx, ref); !errors.Is(err, runner.ErrProfileMismatch) {
			t.Fatalf("%s: a report naming another profile digest must not be accepted: got %v", mode, err)
		}

		// The client check bypassed: the client is shown a forged declaration
		// of the substituted tuple, but the adapter qualifies only p, so its
		// own admission refuses and nothing is launched.
		swapped := p
		swapped.Backend = runner.Backend{Profile: "backend-remote", Platform: "linux-amd64"}
		swapped = conformance.Seal(swapped)
		inner := newAdapter(p)
		c, claims = clientFor(conformance.ForgedProfiles{Adapter: inner, Declared: []runner.QualifiedProfile{swapped}})
		req = conformance.ProfileRequest(base, conformance.Requirement(swapped, runner.Needs{}))
		claims.Set(req.Authority.Key, req.Authority.Generation)
		if _, _, err := c.Start(ctx, req); !errors.Is(err, runner.ErrDefiniteFailure) || !errors.Is(err, runner.ErrUnsupportedMandatory) {
			t.Fatalf("%s: with the client check bypassed the adapter must refuse: got %v", mode, err)
		}
		if inner.Starts() != 0 {
			t.Fatalf("%s: with the client check bypassed the adapter launched %d runs", mode, inner.Starts())
		}

		// The caller's own check: the contract accepted a result on p, but
		// the caller's current profile for the role has since rotated.
		req = conformance.ProfileRequest(base, good)
		acc, err := accept(t, mode+": caller check", newAdapter(p), req)
		if err != nil {
			t.Fatalf("%s: positive control: %v", mode, err)
		}
		current := conformance.NewCurrentProfiles()
		caller := conformance.ProfileCaller{Profiles: current}
		current.Set(req.Packet.Role, p.Digest)
		if err := caller.Commit(ctx, req.Packet.Role, acc); err != nil {
			t.Fatalf("%s: a result on the current profile must commit: %v", mode, err)
		}
		current.Set(req.Packet.Role, swapped.Digest)
		if err := caller.Commit(ctx, req.Packet.Role, acc); !errors.Is(err, conformance.ErrStaleProfile) {
			t.Fatalf("%s: a result on a rotated-out profile must not commit: %v", mode, err)
		}
		// With the contract bypassed entirely, the caller still refuses a
		// substituted report and a missing one.
		current.Set(req.Packet.Role, p.Digest)
		forged := acc
		forged.Execution = &runner.ExecutionReport{Digest: swapped.Digest, Harness: p.Harness, Backend: swapped.Backend}
		if err := caller.Commit(ctx, req.Packet.Role, forged); !errors.Is(err, conformance.ErrStaleProfile) {
			t.Fatalf("%s: a substituted report must not commit: %v", mode, err)
		}
		forged.Execution = nil
		if err := caller.Commit(ctx, req.Packet.Role, forged); !errors.Is(err, conformance.ErrStaleProfile) {
			t.Fatalf("%s: a result with no report must not commit: %v", mode, err)
		}
	}

	// The malformed and drifted fixtures.
	if got, err := runner.DecodeQualifiedProfile(profileFixture(t, "qualified-profile.json")); err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("qualified-profile.json must decode to the kit's profile: %v", err)
	}
	for name, want := range map[string]error{
		"requirement-unknown-field.json": runner.ErrInvalidRequest,
		"requirement-null-field.json":    runner.ErrInvalidRequest,
		"requirement-bad-digest.json":    runner.ErrInvalidRequest,
	} {
		if _, err := runner.DecodeProfileRequirement(profileFixture(t, name)); !errors.Is(err, want) {
			t.Fatalf("%s: got %v, want %v", name, err, want)
		}
	}
	for name, want := range map[string]error{
		"profile-undefined-support.json": runner.ErrInvalidRequest,
		"profile-drifted.json":           runner.ErrProfileMismatch,
	} {
		if _, err := runner.DecodeQualifiedProfile(profileFixture(t, name)); !errors.Is(err, want) {
			t.Fatalf("%s: got %v, want %v", name, err, want)
		}
	}

	// Schema to source: the real schema has no drift; a planted drift is
	// detected.
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "loop-admin-execution-profile-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if d := profileSchemaDrift(s); len(d) != 0 {
		t.Fatalf("schema drifted from the Go types:\n%s", strings.Join(d, "\n"))
	}
	for what, plant := range map[string]func(map[string]any){
		"dropped dimension": func(s map[string]any) {
			delete(schemaProps(schemaDef(s, "preservation")), "process_memory")
		},
		"added fallback": func(s map[string]any) {
			schemaProps(s)["fallback"] = map[string]any{"type": "string"}
		},
		"optional digest": func(s map[string]any) {
			schemaDef(s, "qualifiedProfile")["required"] = []any{"runner_profile"}
		},
		"support collapsed": func(s map[string]any) {
			schemaDef(s, "support")["enum"] = []any{"supported", "unsupported"}
		},
	} {
		var planted map[string]any
		_ = json.Unmarshal(raw, &planted)
		plant(planted)
		if d := profileSchemaDrift(planted); len(d) == 0 {
			t.Fatalf("a planted schema drift (%s) went undetected", what)
		}
	}
}

func schemaDef(s map[string]any, name string) map[string]any {
	return s["$defs"].(map[string]any)[name].(map[string]any)
}

func schemaProps(n map[string]any) map[string]any {
	p, _ := n["properties"].(map[string]any)
	return p
}

func schemaStrings(v any) []string {
	var out []string
	l, _ := v.([]any)
	for _, x := range l {
		s, _ := x.(string)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// profileSchemaDrift compares the execution-profile schema with the Go types:
// each object's properties are the type's json tags, its required list is
// the tags without omitempty, and the support and coverage enums are the Go
// constants.
func profileSchemaDrift(s map[string]any) []string {
	var drift []string
	for _, p := range []struct {
		name   string
		schema map[string]any
		typ    reflect.Type
	}{
		{"requirement", s, reflect.TypeOf(runner.ProfileRequirement{})},
		{"harness", schemaDef(s, "harness"), reflect.TypeOf(runner.Harness{})},
		{"modelBinding", schemaDef(s, "modelBinding"), reflect.TypeOf(runner.ModelBinding{})},
		{"backend", schemaDef(s, "backend"), reflect.TypeOf(runner.Backend{})},
		{"preservation", schemaDef(s, "preservation"), reflect.TypeOf(runner.Preservation{})},
		{"operations", schemaDef(s, "operations"), reflect.TypeOf(runner.Operations{})},
		{"preservationNeed", schemaDef(s, "preservationNeed"), reflect.TypeOf(runner.PreservationNeed{})},
		{"needs", schemaDef(s, "needs"), reflect.TypeOf(runner.Needs{})},
		{"qualification", schemaDef(s, "qualification"), reflect.TypeOf(runner.Qualification{})},
		{"qualifiedProfile", schemaDef(s, "qualifiedProfile"), reflect.TypeOf(runner.QualifiedProfile{})},
		{"report", schemaDef(s, "report"), reflect.TypeOf(runner.ExecutionReport{})},
	} {
		var props, tags, required []string
		for k := range schemaProps(p.schema) {
			props = append(props, k)
		}
		for i := 0; i < p.typ.NumField(); i++ {
			tag, opts, _ := strings.Cut(p.typ.Field(i).Tag.Get("json"), ",")
			tags = append(tags, tag)
			if !strings.Contains(opts, "omitempty") {
				required = append(required, tag)
			}
		}
		sort.Strings(props)
		sort.Strings(tags)
		sort.Strings(required)
		if !reflect.DeepEqual(props, tags) {
			drift = append(drift, p.name+": properties "+strings.Join(props, ",")+" != Go tags "+strings.Join(tags, ","))
		}
		if got := schemaStrings(p.schema["required"]); !reflect.DeepEqual(got, required) {
			drift = append(drift, p.name+": required "+strings.Join(got, ",")+" != Go non-omitempty tags "+strings.Join(required, ","))
		}
		if p.schema["additionalProperties"] != false {
			drift = append(drift, p.name+": additionalProperties is not false")
		}
	}
	for name, want := range map[string][]string{
		"support":  {string(runner.SupportYes), string(runner.SupportNo), string(runner.SupportUnknown)},
		"coverage": {string(runner.CoverageNone), string(runner.CoveragePartial), string(runner.CoverageComplete), string(runner.CoverageUnknown)},
	} {
		sort.Strings(want)
		if got := schemaStrings(schemaDef(s, name)["enum"]); !reflect.DeepEqual(got, want) {
			drift = append(drift, name+": enum "+strings.Join(got, ",")+" != Go "+strings.Join(want, ","))
		}
	}
	return drift
}

// digestVector is the exact byte sequence ProfileDigest hashes for one
// profile, written by hand for a non-Go adapter to check its own encoder
// against: compact, members in the spec's order, digest empty, strings as
// themselves (no HTML escaping of "&", "<" or ">"), and an empty list as [].
const digestVector = `{"digest":"","runner_profile":"profile-a","roles":["worker"],"harness":{"implementation":"harness-x","version":"1.4.2","bundle_digest":"sha256:a02a49f49507c2acda4c89a292c70db848da8684014c4491e8719ae853746128","bundle_provenance":"attestations/harness-x-1.4.2"},"model":{"profile":"model-profile-a","model":"model-a"},"backend":{"profile":"backend-local","platform":"linux-arm64"},"preservation":{"resume":{"files":"supported","conversation":"unsupported","process_memory":"unsupported","descendants":"unknown","pending_interactions":"unknown"},"restart":{"files":"supported","conversation":"unknown","process_memory":"unknown","descendants":"unknown","pending_interactions":"unknown"}},"cancellation":"supported","budget_scopes":[],"telemetry":"complete","qualification":{"evidence":["https://example.test/run?a=1&b=2"],"limits":["<linux-arm64> & later"]}}`

const digestVectorSum = "sha256:4d13fa543142fa0549f8cb3bb2e23f848a36fb2988d33f49b5f3d8d96bd4b621"

// TestExecutionProfileDigestVector: ProfileDigest hashes exactly the fixed
// vector's bytes, so "&", "<" and ">" are not HTML-escaped and an empty list is
// []; the sealed profile validates and its JSON form decodes with the same
// digest; a nil list, which would hash as null and whose JSON form decoding
// refuses, does not validate.
// regression: agent-execution/01
func TestExecutionProfileDigestVector(t *testing.T) {
	sum := sha256.Sum256([]byte(digestVector))
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != digestVectorSum {
		t.Fatalf("the vector itself: got %s, want %s", got, digestVectorSum)
	}
	var p runner.QualifiedProfile
	if err := json.Unmarshal([]byte(digestVector), &p); err != nil {
		t.Fatal(err)
	}
	if got := runner.ProfileDigest(p); got != digestVectorSum {
		t.Fatalf("ProfileDigest: got %s, want %s", got, digestVectorSum)
	}
	p.Digest = digestVectorSum
	if err := p.Validate(); err != nil {
		t.Fatalf("the sealed vector profile: %v", err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if d, err := runner.DecodeQualifiedProfile(b); err != nil || d.Digest != digestVectorSum {
		t.Fatalf("the vector profile's JSON form: %v", err)
	}
	for what, nilList := range map[string]func(*runner.QualifiedProfile){
		"nil limits":        func(q *runner.QualifiedProfile) { q.Qualification.Limits = nil },
		"nil budget scopes": func(q *runner.QualifiedProfile) { q.BudgetScopes = nil },
	} {
		q := p
		nilList(&q)
		q = conformance.Seal(q)
		if err := q.Validate(); !errors.Is(err, runner.ErrInvalidRequest) {
			t.Fatalf("%s: got %v, want %v", what, err, runner.ErrInvalidRequest)
		}
	}
}
