package main

import (
	"fmt"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateWired — deskpr refuses when the repo is public and the
// target issue carries no qualifying +1. Table-driven over create and update entry points.
// Each must make ZERO write calls (no git push, no gh pr create).
func TestPublicRepoGateWired(t *testing.T) {
	t.Run("create_public_refused", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)

		// Override the gate stub: deskpr create has no PR number (0),
		// which means the gate returns exit 6 (Unverifiable).
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error {
			if issueNumber <= 0 {
				return deskkit.Unverifiable("public-repo gate: no issue/PR number", nil)
			}
			return deskkit.Refused("public-repo gate: " + owner + "/" + repo + " is public with no +1")
		}

		rc := run([]string{"create", "--title", "Test PR", "--body-min", "Brief: fixture/01\nbody"})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("create on public repo rc = %d, want 6 (unverifiable)", rc)
		}

		// Assert ZERO write calls (no git push, no gh pr create).
		for _, c := range *calls {
			if len(c) >= 2 && c[0] == "git" && c[1] == "push" {
				t.Fatalf("gate refused -- must NOT make git push call; calls: %v", *calls)
			}
		}
	})

	t.Run("update_public_refused", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

		// Override the gate stub: public repo with no +1 -> exit 5.
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error {
			return deskkit.Refused("public-repo gate: " + owner + "/" + repo + " is public with no +1")
		}

		rc := run([]string{"update"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("update on public repo rc = %d, want 5 (refused)", rc)
		}

		// Assert ZERO write calls.
		for _, c := range *calls {
			if len(c) >= 2 && c[0] == "git" && c[1] == "push" {
				t.Fatalf("gate refused -- must NOT make git push call; calls: %v", *calls)
			}
		}
	})

	// Review N2: the two subtests above prove the caller PROPAGATES a refusal, but their
	// stubs ignore every argument — so they would pass identically if `update` asked the
	// gate about the wrong repo, or about PR #0. What the gate is ASKED is the whole
	// authorization question (a +1 on some other issue must not authorize this write), so
	// pin the arguments too. The fake `gh pr list` reports PR #42 on this branch.
	t.Run("update_asks_the_gate_about_the_real_target", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")

		var gotOwner, gotRepo string
		var gotIssue int
		var calledTimes int
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error {
			calledTimes++
			gotOwner, gotRepo, gotIssue = owner, repo, issueNumber
			return deskkit.Refused("public-repo gate: stub refusal")
		}

		if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
			t.Fatalf("update rc = %d, want 5 (refused)", rc)
		}
		if calledTimes != 1 {
			t.Fatalf("gate called %d times, want exactly 1", calledTimes)
		}
		if gotOwner != "example-org" || gotRepo != "tracker" {
			t.Fatalf("gate asked about %s/%s, want example-org/tracker — a gate "+
				"asked about the wrong repo reads the wrong repo's visibility", gotOwner, gotRepo)
		}
		if gotIssue != 42 {
			t.Fatalf("gate asked about issue/PR #%d, want #42 (the PR being updated) — a +1 on "+
				"any OTHER number must never authorize this write", gotIssue)
		}
	})

	// The create counterweight: create has no PR yet, so the gate must be asked about
	// number 0 — which is what makes it unperformable (exit 6) rather than something a
	// stray +1 elsewhere could satisfy.
	t.Run("create_asks_the_gate_with_no_number", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)

		gotIssue := -1
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, _, _ string, issueNumber int) error {
			gotIssue = issueNumber
			return deskkit.Unverifiable("public-repo gate: no issue/PR number", nil)
		}

		if rc := run([]string{"create", "--title", "Test PR", "--body-min", "Brief: fixture/01\nbody"}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("create rc = %d, want 6 (unverifiable)", rc)
		}
		if gotIssue != 0 {
			t.Fatalf("gate asked about issue/PR #%d on create, want 0", gotIssue)
		}
	})

	// #1707: a create carrying an `Issue: #<N>` trailer routes that number to the gate,
	// so a non-blessed public repo gains the per-issue-+1 path (a +1 on that tracking
	// issue admits the create) instead of the structural no-number hard-fail. A `Brief:`
	// trailer keeps 0 (covered by create_asks_the_gate_with_no_number above) — a brief
	// resolves to a file, not a reactions surface.
	//
	// FAIL-FIRST: revert the create-path gate call in deskpr.go from
	// `publicRepoGateFn(fetcher, owner, name, trailerIssue)` back to the pre-fix hardcoded
	// `...name, 0)` (neutralize the now-unused `trailerIssue` capture, e.g. `_ = trailerIssue`,
	// or restore requireTrailer's error-only signature) and this subtest alone goes red:
	//   gate asked about issue/PR #0 on an `Issue: #77` create, want 77
	// while create_asks_the_gate_with_no_number stays green — the two spellings differ only
	// for a create carrying an `Issue:` trailer, which is exactly what this pins.
	t.Run("create_issue_trailer_asks_the_gate_with_the_issue_number", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)

		gotIssue := -1
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, _, _ string, issueNumber int) error {
			gotIssue = issueNumber
			// Refuse so the create stops at the gate (no push/create) regardless of
			// number — this test pins the ARGUMENT, not the admission outcome.
			return deskkit.Refused("public-repo gate: stub refusal")
		}

		if rc := run([]string{"create", "--title", "Test PR", "--body-min", "body\nIssue: #77"}); rc != deskkit.ExitRefused {
			t.Fatalf("create rc = %d, want 5 (refused)", rc)
		}
		if gotIssue != 77 {
			t.Fatalf("gate asked about issue/PR #%d on an `Issue: #77` create, want 77 — the "+
				"trailer's tracking issue is the reactions surface the +1 lands on", gotIssue)
		}
	})
}

// TestGateSeamIsRealInProduction — the seam these tests replace must, in a fresh
// binary, be the real deskkit.PublicRepoGate.
//
// This closes a class the review found twice. publicRepoGateFn is stubbed by every
// other test here, so the tests above prove only that the caller PROPAGATES the gate's
// error — they would pass identically if production had never been bound to the gate
// at all. That is not hypothetical: at head 1b91e146 `deskpr create` returned exit 6
// on EVERY repo, private ones included, and no test noticed, because they all ran
// against a stub. Asserting the production binding is what makes the other assertions
// mean something.
func TestGateSeamIsRealInProduction(t *testing.T) {
	// Package-level vars are initialised before any test runs, so compare against a
	// sentinel captured at init rather than the possibly-stubbed current value.
	if productionGateFn == nil {
		t.Fatal("productionGateFn is nil — the seam has no recorded production binding")
	}
	if fmt.Sprintf("%p", productionGateFn) != fmt.Sprintf("%p", deskkit.PublicRepoGate) {
		t.Fatal("publicRepoGateFn is not bound to deskkit.PublicRepoGate at init — the gate " +
			"is stubbed out in the shipped binary, and every other gate test here is vacuous")
	}
}
