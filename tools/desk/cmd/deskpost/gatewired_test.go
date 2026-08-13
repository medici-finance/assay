package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateWired — every write-capable command MUST refuse
// (exit 5) when the repo is PUBLIC and the target issue/PR carries NO qualifying +1.
// Each subtest drives a different entry function: runComment, runReview, runReady.
// Each must make ZERO mutating calls against the fake server.

func TestPublicRepoGateWired(t *testing.T) {
	t.Run("comment_public_no_plus_one_refused", func(t *testing.T) {
		f, _ := setupFake(t)
		f.repoVisibility = "public"
		f.repoReactions = []deskkit.Reaction{} // no +1

		bf := writeBody(t, "c.md", "Re-reviewed the delta.")
		code := run(commentArgs("example-org/tracker", "1", bf))
		if code != deskkit.ExitRefused {
			t.Fatalf("comment on public repo without +1 exit = %d, want 5 (refused)", code)
		}
		if f.postedCmt != 0 {
			t.Fatal("gate refused — must NOT post comment")
		}
		if f.flips != 0 {
			t.Fatal("gate refused — must NOT flip")
		}
	})

	t.Run("review_public_no_plus_one_refused", func(t *testing.T) {
		f, _ := setupFake(t)
		f.repoVisibility = "public"
		f.repoReactions = []deskkit.Reaction{} // no +1

		bf := writeBody(t, "r.md", "## Summary\n\nVerdict: approve\n\nLGTM.")
		code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
		if code != deskkit.ExitRefused {
			t.Fatalf("review on public repo without +1 exit = %d, want 5 (refused)", code)
		}
		if f.postedReview != 0 {
			t.Fatal("gate refused — must NOT post review")
		}
	})

	t.Run("ready_public_no_plus_one_refused", func(t *testing.T) {
		f, _ := setupFake(t)
		f.repoVisibility = "public"
		f.repoReactions = []deskkit.Reaction{} // no +1

		// Add a prior APPROVED review at the head so the ready gate passes the approval check
		f.reviews = []reviewInfo{
			appReview("APPROVED", testHead, "## Summary\n\nVerdict: approve\n\nLGTM."),
		}
		// No changed files (not risk-classed)
		f.files = nil

		code := run(readyArgs("example-org/tracker"))
		if code != deskkit.ExitRefused {
			t.Fatalf("ready on public repo without +1 exit = %d, want 5 (refused)", code)
		}
		if f.flips != 0 {
			t.Fatal("gate refused — must NOT flip")
		}
	})

	// Verify that a private repo still passes (the gate is not a blanket refusal).
	t.Run("comment_private_repo_passes", func(t *testing.T) {
		f, _ := setupFake(t)
		f.repoVisibility = "private" // default already, but explicit

		bf := writeBody(t, "c.md", "Re-reviewed the delta.")
		code := run(commentArgs("example-org/tracker", "1", bf))
		if code != deskkit.ExitOK {
			t.Fatalf("comment on private repo exit = %d, want 0", code)
		}
		if f.postedCmt != 1 {
			t.Fatalf("private repo should pass gate, postedCmt = %d, want 1", f.postedCmt)
		}
	})
}
