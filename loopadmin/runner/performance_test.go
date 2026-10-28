package runner

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

type perfAdapter struct {
	identities []PerformanceIdentity
	requests   []LaunchRequest
}

func (a *perfAdapter) Capabilities() Capabilities {
	return Capabilities{Resume: true, LaunchDedupe: true, BudgetScopes: []BudgetScope{BudgetLaunch}}
}
func (a *perfAdapter) Start(ctx context.Context, r LaunchRequest) (Receipt, error) {
	identity, ok := PerformanceContext(ctx)
	if !ok {
		panic("missing launch identity")
	}
	a.identities = append(a.identities, identity)
	a.requests = append(a.requests, r)
	return Receipt{Ref: r.Ref(), SessionID: "native-" + r.ID, ActualModel: r.Profile.Model}, nil
}
func (*perfAdapter) Observe(context.Context, Ref) (Observation, error) {
	return Observation{State: StateStopped}, nil
}
func (*perfAdapter) Reconcile(context.Context, Ref) (Observation, error) {
	return Observation{State: StateStopped}, nil
}
func (*perfAdapter) Cancel(context.Context, Ref) (CancelAck, error) {
	return CancelAck{Acknowledged: true}, nil
}

// regression: desktools-v2/64
func TestPerfV3RunnerIdentityLifecycle(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "workflow")
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("ASSAY_PERF_AGENT_ID", strings.Repeat("a", 32))
	t.Setenv("ASSAY_PERF_SESSION_ID", strings.Repeat("b", 32))
	parent := perfrec.ResolveContext(perfrec.ContextInput{})
	a := &perfAdapter{}
	c := NewClient(a, FenceFunc(func(context.Context, string) (uint64, error) { return 1, nil }))
	request := func(id string) *LaunchRequest {
		return &LaunchRequest{
			Version: Version, Caller: "fixture", ID: id, Mode: ModeStandingDesk, Desk: &DeskRef{BindingID: "desk"},
			Packet: RolePacket{Role: "review", Ref: "packet", Trust: TrustOperator}, Profile: Profile{ID: "pinned", Model: "test", Tools: []string{}},
			Workspace: "workspace", Authority: Authority{Key: id, Generation: 1, ValidatedBy: "fixture"}, Budget: Reservation{ID: "budget", Scope: BudgetLaunch, MaxTokens: 1}}
	}
	first := request("first")
	work := []perfrec.WorkRef{{Kind: "issue", Forge: "github", Instance: "github.com", Repo: "example/project", Issue: 1, Primary: true}}
	ctx := WithPerformance(context.Background(), parent, "review", work)
	copyBefore := first.clone()
	ref, state, err := c.Start(ctx, *first)
	if err != nil || state != StateRunning || ref != first.Ref() {
		t.Fatal("optional diagnostics changed start")
	}
	if !reflect.DeepEqual(a.requests[0], copyBefore) {
		t.Fatal("diagnostics changed request/authority")
	}
	if _, _, err := c.Start(ctx, *first); err != nil || len(a.identities) != 1 {
		t.Fatal("retry changed deduplication")
	}
	d := a.identities[0].Context.Data()
	if d.AgentID == parent.Data().AgentID || d.SessionID == parent.Data().SessionID || d.ParentAgentID != parent.Data().AgentID || d.ParentInvocationID != parent.InvocationID() || d.Role != "review" {
		t.Fatal("fresh identity/parent flow lost")
	}
	resumed := request("resumed")
	resumed.Resume = &SessionRef{SessionID: "native-first", Role: "review", ProfileID: "pinned"}
	work[0].Issue = 2
	resumeContext := WithPerformance(context.Background(), parent, "review", work)
	if _, _, err := c.Start(resumeContext, *resumed); err != nil {
		t.Fatal(err)
	}
	rd := a.identities[1].Context.Data()
	if rd.AgentID != d.AgentID || rd.SessionID != d.SessionID || rd.InvocationID == d.InvocationID || rd.Work[0].Issue != 2 || a.identities[1].ContinuityGap {
		t.Fatal("explicit resume lost identity or retained stale work")
	}
	replacement := request("replacement")
	if _, _, err := c.Start(ctx, *replacement); err != nil {
		t.Fatal(err)
	}
	if a.identities[2].Context.Data().AgentID == d.AgentID {
		t.Fatal("replacement inherited old agent")
	}
	missing := request("missing")
	missing.Resume = &SessionRef{SessionID: "unknown-native", Role: "review", ProfileID: "pinned"}
	if _, _, err := c.Start(ctx, *missing); err != nil {
		t.Fatal(err)
	}
	if !a.identities[3].ContinuityGap || a.identities[3].Context.Data().AgentID == d.AgentID {
		t.Fatal("unknown mapping guessed continuity")
	}
	c.performance["native-first"] = performanceMap{d, performanceDay(time.Now()).AddDate(0, 0, -7)}
	expired := request("expired")
	expired.Resume = &SessionRef{SessionID: "native-first", Role: "review", ProfileID: "pinned"}
	if _, _, err := c.Start(ctx, *expired); err != nil {
		t.Fatal(err)
	}
	if !a.identities[4].ContinuityGap {
		t.Fatal("expired mapping resumed")
	}
	t.Setenv("ASSAY_TELEMETRY", "0")
	off := perfrec.ResolveContext(perfrec.ContextInput{})
	if _, _, err := c.Start(WithPerformance(context.Background(), off, "review", work), *request("off")); err != nil {
		t.Fatal("disabled diagnostics blocked launch")
	}
	if a.identities[5].Context.Profile() != "" {
		t.Fatal("launcher manufactured opt-in")
	}
	if osLoop := parent.Data().DeskRole; osLoop != "worker-desk" {
		t.Fatal("assigned role changed desk authority")
	}
}
