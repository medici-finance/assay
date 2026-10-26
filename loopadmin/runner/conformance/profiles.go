package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// QualifiedProfile builds a well-formed qualified profile for the runner
// profile and role the request builders use, with its digest computed. Its
// resume preserves files only: the conversation and process memory are
// declared unsupported, descendants and pending interactions unknown. A
// restart preserves files and nothing else is known.
func QualifiedProfile() runner.QualifiedProfile {
	p := runner.QualifiedProfile{
		RunnerProfile: "profile-a",
		Roles:         []string{"worker"},
		Harness: runner.Harness{
			Implementation: "harness-x", Version: "1.4.2",
			BundleDigest:     "sha256:" + digestOf("harness-x bundle 1.4.2"),
			BundleProvenance: "attestations/harness-x-1.4.2",
		},
		Model:   runner.ModelBinding{Profile: "model-profile-a", Model: "model-a"},
		Backend: runner.Backend{Profile: "backend-local", Platform: "linux-arm64"},
		Preservation: runner.Operations{
			Resume: runner.Preservation{
				Files: runner.SupportYes, Conversation: runner.SupportNo, ProcessMemory: runner.SupportNo,
				Descendants: runner.SupportUnknown, PendingInteractions: runner.SupportUnknown,
			},
			Restart: runner.Preservation{
				Files: runner.SupportYes, Conversation: runner.SupportUnknown, ProcessMemory: runner.SupportUnknown,
				Descendants: runner.SupportUnknown, PendingInteractions: runner.SupportUnknown,
			},
		},
		Cancellation:  runner.SupportYes,
		BudgetScopes:  []runner.BudgetScope{runner.BudgetLaunch, runner.BudgetRequest},
		Telemetry:     runner.CoverageComplete,
		Qualification: runner.Qualification{Evidence: []string{"qualification/harness-x-1.4.2"}, Limits: []string{"linux-arm64 only"}},
	}
	return Seal(p)
}

func digestOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Seal recomputes a profile's digest over its content.
func Seal(p runner.QualifiedProfile) runner.QualifiedProfile {
	p.Digest = runner.ProfileDigest(p)
	return p
}

// Requirement is the requirement that pins a qualified profile: its digest and
// the identities it binds, with the given needs.
func Requirement(p runner.QualifiedProfile, needs runner.Needs) runner.ProfileRequirement {
	return runner.ProfileRequirement{Digest: p.Digest, Harness: p.Harness, Model: p.Model, Backend: p.Backend, Needs: needs}
}

// ProfileRequest makes a request carry a requirement and require the
// execution-profile extension, so an adapter that does not know it refuses.
func ProfileRequest(req runner.LaunchRequest, need runner.ProfileRequirement) runner.LaunchRequest {
	b, _ := json.Marshal(need) // strings, bools and nested structs always marshal
	ext := map[string]json.RawMessage{}
	for k, v := range req.Extensions {
		ext[k] = v
	}
	ext[runner.ExecutionProfileExt] = b
	req.Extensions = ext
	req.Require = append(append([]string(nil), req.Require...), runner.RequireExecutionProfile)
	return req
}

// ForgedProfiles is the fixture that bypasses the client's execution-profile
// check: it declares the extension and a qualified-profile list of its own to
// the Client, whatever the wrapped adapter really qualifies, like a client
// that skipped or mis-read the declaration. Everything else is forwarded, so
// the wrapped adapter's own admission check is the layer left to refuse.
type ForgedProfiles struct {
	runner.Adapter
	Declared []runner.QualifiedProfile
}

// Capabilities implements runner.Adapter: the wrapped adapter's capabilities,
// declaring the extension.
func (f ForgedProfiles) Capabilities() runner.Capabilities {
	c := f.Adapter.Capabilities()
	c.Extensions = append(append([]string(nil), c.Extensions...), runner.ExecutionProfileExt)
	return c
}

// QualifiedProfiles implements runner.ProfileDeclarer with the forged list.
func (f ForgedProfiles) QualifiedProfiles() []runner.QualifiedProfile { return f.Declared }

// ErrStaleProfile is the reference caller's own refusal of an accepted result
// whose qualified profile is not the one it currently holds for the role.
var ErrStaleProfile = errors.New("conformance: caller's qualified profile is not the one the result ran under")

// CurrentProfiles is a caller's own record of the qualified profile digest each
// role currently runs on: what a desk binding or a controller's node table is
// to the real caller. It is independent of the runner contract and of
// anything the adapter reports.
type CurrentProfiles struct {
	mu      sync.Mutex
	digests map[string]string
}

// NewCurrentProfiles returns an empty record.
func NewCurrentProfiles() *CurrentProfiles {
	return &CurrentProfiles{digests: map[string]string{}}
}

// Set records the current qualified profile digest for a role.
func (c *CurrentProfiles) Set(role, digest string) {
	c.mu.Lock()
	c.digests[role] = digest
	c.mu.Unlock()
}

func (c *CurrentProfiles) get(role string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.digests[role]
	return d, ok
}

// ProfileCaller is the caller-side layer of the extension: whatever the
// runner contract accepted, the caller checks the execution report against
// its OWN current qualified profile for the role before it commits. It fails
// closed when it has no record for the role or the result carries no report.
type ProfileCaller struct{ Profiles *CurrentProfiles }

// Commit refuses an accepted result that did not run on the role's current
// qualified profile.
func (c ProfileCaller) Commit(_ context.Context, role string, acc runner.Accepted) error {
	cur, ok := c.Profiles.get(role)
	if !ok {
		return fmt.Errorf("%w: no current profile is recorded for the role", ErrStaleProfile)
	}
	if acc.Execution == nil || acc.Execution.Digest != cur {
		return fmt.Errorf("%w: the result does not report the role's current profile", ErrStaleProfile)
	}
	return nil
}

// CaseProfileRefusedBeforeStart: a request that requires a qualified profile
// the adapter does not declare is refused before Start, in both modes,
// whether the adapter does not know the extension at all or declares other
// profiles: nothing is launched and no invocation is recorded. The same
// launch without the extension still runs.
func CaseProfileRefusedBeforeStart(t *testing.T, f Factory) {
	ctx := context.Background()
	p := QualifiedProfile()
	p.Qualification.Limits = append(p.Qualification.Limits, "conformance kit only: declared by no adapter")
	p = Seal(p)
	for _, base := range []runner.LaunchRequest{StandingRequest("profile-refused"), WorkflowRequest("profile-refused")} {
		s, claims, c := setup(t, f)
		req := ProfileRequest(base, Requirement(p, runner.Needs{}))
		claims.Set(req.Authority.Key, req.Authority.Generation)
		_, _, err := c.Start(ctx, req)
		wantErr(t, string(base.Mode)+": undeclared qualified profile", err, runner.ErrUnsupportedMandatory)
		if errors.Is(err, runner.ErrDefiniteFailure) {
			t.Fatalf("%s: the refusal came from the adapter, not before Start: %v", base.Mode, err)
		}
		if _, err := c.State(req.Ref()); !errors.Is(err, runner.ErrUnknownInvocation) {
			t.Fatalf("%s: a refused launch left an invocation: %v", base.Mode, err)
		}
		if obs, err := s.Adapter.Observe(ctx, req.Ref()); err == nil && obs.State != runner.StateAbsent {
			t.Fatalf("%s: the adapter launched a refused request: %q", base.Mode, obs.State)
		}
		if _, _, err := c.Start(ctx, base); err != nil {
			t.Fatalf("%s: the same launch without the extension must still run: %v", base.Mode, err)
		}
	}
}
