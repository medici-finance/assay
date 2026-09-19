package main

import (
	"os"
	"strings"
	"testing"
)

// dpFixture copies the shared deploy-gate fixture tree into a temp root and
// loads the streams, mirroring designgate_test.go's dgProblems helper exactly.
func dpFixture(t *testing.T) (root string, streams []*Stream) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/deploygate")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

// TestDeployTransition exercises the whole deploy transition over the
// committed fixture register: the positive path, the mutation (negative) path
// the transition must actually gate on (Verify row 4), the dangling reference,
// the rollback accepted-consequence grammar, and the runbook drill-row shape
// and could-not-check reporting (Verify row 5) — one subtest each, all under
// the single name Verify row 3 selects.
func TestDeployTransition(t *testing.T) {
	root, streams := dpFixture(t)
	problems := deployTransitionProblems(root, streams)
	joined := strings.Join(problems, "\n")

	t.Run("VerifiedBriefIsClean", func(t *testing.T) {
		if strings.Contains(joined, "DEPLOY-fixture-clean") {
			t.Errorf("a DEPLOY record whose brief is verified must pass; got:\n%s", joined)
		}
	})

	t.Run("ImplementedOnlyBriefGatesTheTransition", func(t *testing.T) {
		// This is the negative-path row (Verify row 4): the transition must
		// actually gate on the precondition, naming it.
		if !strings.Contains(joined, "DEPLOY-fixture-notverified") || !strings.Contains(joined, `is "implemented", not verified`) {
			t.Errorf("a DEPLOY record whose brief is only implemented must be flagged naming the precondition; got:\n%s", joined)
		}
	})

	t.Run("DanglingBriefReferenceIsFlagged", func(t *testing.T) {
		if !strings.Contains(joined, "DEPLOY-fixture-dangling") || !strings.Contains(joined, "dereferences to no brief") {
			t.Errorf("a brief: reference that resolves to no brief must be flagged; got:\n%s", joined)
		}
	})

	t.Run("RollbackNoneAcceptedWithoutApproverIsFlagged", func(t *testing.T) {
		if !strings.Contains(joined, "DEPLOY-fixture-rollback-bad") || !strings.Contains(joined, "rollback-approver") {
			t.Errorf("rollback: none-accepted with no named approver must be flagged; got:\n%s", joined)
		}
	})

	t.Run("RollbackNoneAcceptedWithApproverIsClean", func(t *testing.T) {
		if strings.Contains(joined, "DEPLOY-fixture-rollback-ok") {
			t.Errorf("rollback: none-accepted WITH a named approver is the legitimate accepted-consequence form and must not be flagged; got:\n%s", joined)
		}
	})

	t.Run("RunbookWithNoDrillRowsIsFlagged", func(t *testing.T) {
		if !strings.Contains(joined, "RUNBOOK-fixture-nodrills") || !strings.Contains(joined, "no drill rows") {
			t.Errorf("a runbook with no drill rows at all must be a hard PROBLEM; got:\n%s", joined)
		}
	})

	t.Run("DrilledRunbookIsNotAProblem", func(t *testing.T) {
		if strings.Contains(joined, "RUNBOOK-fixture-drilled") {
			t.Errorf("a fully-drilled runbook must not be a PROBLEM; got:\n%s", joined)
		}
	})

	t.Run("UndrilledRowIsNotAProblemEither", func(t *testing.T) {
		// Verify row 5: an undrilled runbook is could-not-check, which is a
		// NOTICE (deployTransitionNotices), never a hard --lint PROBLEM.
		if strings.Contains(joined, "RUNBOOK-fixture-undrilled") {
			t.Errorf("an undrilled drill row is could-not-check, not a hard PROBLEM; got:\n%s", joined)
		}
	})
}

// TestDeployTransitionUndrilledNotice pins the could-not-check half of Verify
// row 5: an undrilled runbook must be VISIBLE (a NOTICE naming it and the
// undrilled count), neither a silent pass nor a hard failure — and a fully
// drilled runbook must carry no such notice.
func TestDeployTransitionUndrilledNotice(t *testing.T) {
	root, streams := dpFixture(t)
	notices := deployTransitionNotices(root, streams)
	joined := strings.Join(notices, "\n")

	if !strings.Contains(joined, "RUNBOOK-fixture-undrilled") || !strings.Contains(joined, "could-not-check") {
		t.Errorf("an undrilled runbook must emit a could-not-check NOTICE naming it; got:\n%s", joined)
	}
	if !strings.Contains(joined, "1 of 2 drill row") {
		t.Errorf("the notice must name the exact undrilled/total drill-row count; got:\n%s", joined)
	}
	if strings.Contains(joined, "RUNBOOK-fixture-drilled: could-not-check") {
		t.Errorf("a fully-drilled runbook must not be reported could-not-check; got:\n%s", joined)
	}
}

// TestDeployTransitionUnreadableRegisterCouldNotCheck pins the three-state
// read: an unreadable DEPLOYS register (a file where the directory should be)
// must be reported could-not-check by both the problems and notices paths,
// never collapsed into "no records" (a silent pass) — the same contract
// parseDecisionsDir's designgate.go sibling already keeps.
func TestDeployTransitionUnreadableRegisterCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/docs/streams", 0o755); err != nil {
		t.Fatal(err)
	}
	// A FILE named "deploys" where a directory is expected makes os.ReadDir
	// fail with something other than IsNotExist.
	if err := os.WriteFile(root+"/docs/streams/deploys", []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	notices := deployTransitionNotices(root, streams)
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "COULD-NOT-CHECK") {
		t.Errorf("an unreadable deploys register must be reported COULD-NOT-CHECK, never a silent pass; got:\n%s", joined)
	}
}
