package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
	"github.com/medici-finance/assay/loopadmin/workspace"
	"github.com/medici-finance/assay/loopadmin/workspace/fake"
)

// deskPreparation derives a prepared-environment request from a standing-desk
// launch request: its invocation, its authority as the owner, and its role
// packet's role in the cell, with the fixture's repositories, environment and
// policy.
func deskPreparation(t *testing.T, launch runner.LaunchRequest, cell string, gen uint64) workspace.Request {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "workspace", "ready-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := workspace.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Inputs
	req.Invocation = workspace.Invocation{Caller: launch.Caller, ID: launch.ID}
	req.Owner = workspace.Owner{Key: launch.Authority.Key, Generation: launch.Authority.Generation,
		Incarnation: "inc-" + launch.ID, ValidatedBy: launch.Authority.ValidatedBy}
	req.Instance = workspace.Instance{Cell: cell, Role: launch.Packet.Role, ID: launch.ID, Generation: gen}
	return req
}

// TestDeskWorkspaceBinding: two instances of one role in one cell reuse the
// cell's compatible preparation and never share a writable workspace; one
// instance's readiness is not admitted for the other; a successor generation
// gets its own binding, and a restoration records its source subject and the
// continuity it carried; a continuity need the environment does not support,
// or cannot say it supports, is refused with no fresh fallback.
// regression: agent-execution/02
func TestDeskWorkspaceBinding(t *testing.T) {
	ctx := context.Background()
	env := fake.New()
	p := env.Port(workspace.Port{Continuity: runner.Preservation{
		Files: runner.SupportYes, Conversation: runner.SupportNo, ProcessMemory: runner.SupportUnknown,
		Descendants: runner.SupportNo, PendingInteractions: runner.SupportUnknown,
	}})

	reqA := deskPreparation(t, conformance.StandingRequest("w1"), "cell-a", 1)
	reqB := deskPreparation(t, conformance.StandingRequest("w2"), "cell-a", 1)
	if reqA.Instance.Role != reqB.Instance.Role {
		t.Fatal("the two instances must share a role")
	}
	a, err := p.Prepare(ctx, reqA)
	if err != nil {
		t.Fatalf("instance w1: %v", err)
	}
	b, err := p.Prepare(ctx, reqB)
	if err != nil {
		t.Fatalf("instance w2: %v", err)
	}
	if a.Allocation.Preparation != b.Allocation.Preparation {
		t.Fatal("same-role instances with one environment did not reuse the cell preparation")
	}
	if a.Allocation.Workspace == b.Allocation.Workspace {
		t.Fatal("two instances share one writable workspace")
	}

	// Swapped readiness: neither receipt admits the other instance.
	if err := a.Admit(reqB.Invocation, reqB.Owner, reqB.Instance, b.Allocation.Workspace); !errors.Is(err, workspace.ErrMismatch) {
		t.Fatalf("w1's receipt admitted for w2: %v", err)
	}
	if err := b.Admit(reqB.Invocation, reqB.Owner, reqB.Instance, a.Allocation.Workspace); !errors.Is(err, workspace.ErrMismatch) {
		t.Fatalf("w2's receipt admitted over w1's workspace: %v", err)
	}

	// A successor generation has its own binding and workspace.
	succ := reqA
	succ.Instance.Generation = 2
	s, err := p.Prepare(ctx, succ)
	if err != nil {
		t.Fatalf("successor: %v", err)
	}
	if s.Allocation.Workspace == a.Allocation.Workspace || workspace.BindingKey(succ.Instance) == workspace.BindingKey(reqA.Instance) {
		t.Fatal("a successor reused its predecessor's binding")
	}
	if err := a.Admit(reqA.Invocation, reqA.Owner, succ.Instance, a.Allocation.Workspace); !errors.Is(err, workspace.ErrMismatch) {
		t.Fatalf("predecessor's receipt admitted for the successor: %v", err)
	}

	// Restoration from w1: files carried, the source subject recorded.
	rest := reqA
	rest.Instance.Generation = 3
	rest.Restore = &workspace.Restore{Source: reqA.Instance, Needs: runner.PreservationNeed{Files: true}}
	r, err := p.Prepare(ctx, rest)
	if err != nil {
		t.Fatalf("restoration: %v", err)
	}
	if r.Restored == nil || r.Restored.Source != reqA.Instance || !r.Restored.Preserved.Files || env.Restores != 1 {
		t.Fatalf("restoration did not record its source and preserved files: %+v", r.Restored)
	}
	if r.Allocation.Workspace == a.Allocation.Workspace {
		t.Fatal("a restoration wrote into the source's workspace")
	}

	// Continuity the environment cannot give: refused, nothing prepared.
	for need, want := range map[string]struct {
		n   runner.PreservationNeed
		err error
	}{
		"conversation":   {runner.PreservationNeed{Files: true, Conversation: true}, runner.ErrUnsupportedMandatory},
		"process memory": {runner.PreservationNeed{ProcessMemory: true}, runner.ErrCapabilityUnknown},
	} {
		c := reqA
		c.Instance.Generation = 4
		c.Restore = &workspace.Restore{Source: reqA.Instance, Needs: want.n}
		if _, err := p.Prepare(ctx, c); !errors.Is(err, want.err) {
			t.Fatalf("%s continuity: got %v, want %v", need, err, want.err)
		}
		if _, ok := env.Receipt(workspace.BindingKey(c.Instance)); ok {
			t.Fatalf("%s continuity: a refused restoration left a receipt", need)
		}
	}
}
