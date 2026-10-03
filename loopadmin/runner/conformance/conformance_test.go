package conformance_test

import (
	"context"
	"errors"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
	"github.com/medici-finance/assay/loopadmin/runner/fake"
)

// full and minimal build a fresh fake per case, declaring every capability or
// only the mandatory floor, so the kit runs against both ends of the range.
func full(t *testing.T) conformance.Subject {
	a := fake.New(fake.FullCapabilities())
	return conformance.Subject{Adapter: a, Control: a}
}

func minimal(t *testing.T) conformance.Subject {
	a := fake.New(fake.MinimalCapabilities())
	return conformance.Subject{Adapter: a, Control: a}
}

func TestRunnerConformanceUnknownLaunch(t *testing.T) {
	conformance.CaseUnknownLaunch(t, full)
	conformance.CaseUnknownLaunch(t, minimal)
}

func TestRunnerConformanceLateResultDenied(t *testing.T) {
	conformance.CaseLateResultDenied(t, full)
	conformance.CaseLateResultDenied(t, minimal)
}

func TestRunnerConformanceCredentialAndUsage(t *testing.T) {
	conformance.CaseCredentialAndUsage(t, full)
	conformance.CaseCredentialAndUsage(t, minimal)
}

func TestRunnerBothModesWithoutGraph(t *testing.T) {
	conformance.CaseBothModes(t, full)
	conformance.CaseBothModes(t, minimal)
}

// TestRunnerCallerRecheck bypasses the runner contract's generation fence with
// a fixture that calls a fenced attempt current. The contract then accepts a
// well-formed result reported as success, and the caller's OWN claim check is
// the layer that still refuses it.
func TestRunnerCallerRecheck(t *testing.T) {
	ctx := context.Background()
	a := fake.New(fake.FullCapabilities())
	claims := conformance.NewClaims()
	req := conformance.StandingRequest("fenced-attempt")
	claims.Set(req.Authority.Key, 1)

	// The fixture fence still reports the attempt's generation as current.
	c := runner.NewClient(a, conformance.StaleFence{Generation: 1})
	ref, _, err := c.Start(ctx, req)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	claims.Set(req.Authority.Key, 2) // the caller's claim has since moved on
	a.Complete(ref, conformance.GoodResult(req), conformance.FullUsage())
	if _, err := c.Observe(ctx, ref); err != nil {
		t.Fatalf("observe: %v", err)
	}
	acc, err := c.AcceptResult(ctx, ref)
	if err != nil {
		t.Fatalf("with the contract's fence bypassed the result must pass the contract: %v", err)
	}
	if acc.Result.Outcome != runner.OutcomeSuccess {
		t.Fatalf("fixture must report success, got %q", acc.Result.Outcome)
	}

	caller := conformance.ReferenceCaller{Claims: claims}
	if err := caller.Commit(ctx, acc); !errors.Is(err, conformance.ErrStaleClaim) {
		t.Fatalf("the caller's own claim-generation check must refuse the fenced result, got %v", err)
	}

	// Positive control: with the claim still at the attempt's generation the
	// same caller commits, so the refusal above is the generation check.
	claims.Set(req.Authority.Key, 1)
	if err := caller.Commit(ctx, acc); err != nil {
		t.Fatalf("a current claim must commit: %v", err)
	}
	// And the caller fails closed when its claim record cannot be read.
	empty := conformance.ReferenceCaller{Claims: conformance.NewClaims()}
	if err := empty.Commit(ctx, acc); !errors.Is(err, conformance.ErrStaleClaim) {
		t.Fatalf("an unreadable claim must refuse, got %v", err)
	}
}

func TestRunnerConformanceMalformedResult(t *testing.T) {
	conformance.CaseMalformedResult(t, full)
}

func TestRunnerConformanceUnauthorizedTool(t *testing.T) {
	conformance.CaseUnauthorizedTool(t, full)
}

func TestRunnerConformanceCancelStillRunning(t *testing.T) {
	conformance.CaseCancelStillRunning(t, full)
	conformance.CaseCancelStillRunning(t, minimal)
}

func TestRunnerConformanceResumeUnsupported(t *testing.T) {
	conformance.CaseResumeUnsupported(t, minimal)
	conformance.CaseResumeUnsupported(t, full)
}

func TestRunnerConformanceModelFallback(t *testing.T) {
	conformance.CaseModelFallback(t, full)
}

// TestRunnerKitRunAll runs the whole kit the way an adapter package would.
func TestRunnerKitRunAll(t *testing.T) {
	t.Run("full", func(t *testing.T) { conformance.RunAll(t, full) })
	t.Run("minimal", func(t *testing.T) { conformance.RunAll(t, minimal) })
}
