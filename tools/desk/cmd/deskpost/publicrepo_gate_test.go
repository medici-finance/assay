package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// the public-repo risk rule — the risk-class signal is repo-aware, and a PUBLIC repo is risk-classed
// unconditionally.
//
// Before this, riskPathTriggers was five tracker-shaped paths applied to every repo.
// example-org/example-k8s is PUBLIC, holds the Ledger/Identity manifests, and has no
// k8s/ directory at all — so the trigger list could never match a file in it and the
// public infrastructure repo got strictly LESS mandatory security scrutiny than the
// private application repo. These are the tests whose absence let that survive.
// ---------------------------------------------------------------------------

const publicInfraRepo = "example-org/example-k8s"

// TestReadyPublicRepoIdentityDiffNeedsSecurityReview — the regression the issue is
// about. Under the OLD behaviour this PR flipped with a correctness review alone.
func TestReadyPublicRepoIdentityDiffNeedsSecurityReview(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"base/ledger/identity.yaml"}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(publicInfraRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("public-repo identity diff exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a public-repo PR must not flip without a security verdict", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "risk-classed") || !strings.Contains(d, "PUBLIC repo") {
		t.Fatalf("refusal must name the reason (public repo); audit detail = %q", d)
	}
}

// TestReadyPublicRepoEveryDiffIsRiskClassed — the policy call (issue suggested-fix item
// 3, decided in the SECURE direction): visibility alone risk-classes, whatever the diff
// touches. Each of these files is outside every path trigger.
func TestReadyPublicRepoEveryDiffIsRiskClassed(t *testing.T) {
	for _, file := range []string{
		"README.md",
		"docs/architecture.md",
		"examples/demo.yaml",
		"admin/scripts/mint-token.sh",
		"deploy/scripts/rollout.sh",
		".github/workflows/validate.yml",
	} {
		t.Run(file, func(t *testing.T) {
			f, _ := setupFake(t)
			f.files = []string{file}
			f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
			f.status = greenStatus()

			if code := run(readyArgs(publicInfraRepo)); code != deskkit.ExitRefused {
				t.Fatalf("public-repo %s exit = %d, want %d", file, code, deskkit.ExitRefused)
			}
			if f.flips != 0 {
				t.Fatalf("flips = %d, want 0", f.flips)
			}
		})
	}
}

// TestReadyPublicRepoFlipsWithSecurityPass — the gate is a gate, not a wall: with the
// second verdict at head the flip goes through.
func TestReadyPublicRepoFlipsWithSecurityPass(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"base/ledger/identity.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, "## Security review\n\nNo secrets, no widened reader.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(publicInfraRepo)); code != 0 {
		t.Fatalf("public-repo flip with security pass exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyPublicCIlessRepoStillNeedsSecurityReview — example-org/proposals is CI-less
// AND public. The CI half stays green; the security half now blocks.
func TestReadyPublicCIlessRepoStillNeedsSecurityReview(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	// empty CI rollup — proposals has no workflows

	if code := run(readyArgs("example-org/proposals")); code != deskkit.ExitRefused {
		t.Fatalf("public CI-less repo exit = %d, want %d (security gate, not the CI gate)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyEmptyChangedFileListFailsClosed — a changed-file read that comes back empty
// tells us nothing about the diff, so it must not be read as "clean". A PR always
// changes at least one file.
func TestReadyEmptyChangedFileListFailsClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.files = nil // the API returned no files
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("empty changed-file list exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — an unread diff must never flip", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "no changed files") {
		t.Fatalf("refusal must name the cause; audit detail = %q", d)
	}
}

// TestReadyPrivateRepoUnchanged — the private application repo behaves exactly as before:
// a non-risk diff flips, an tracker trigger blocks. The change only ever widens.
func TestReadyPrivateRepoUnchanged(t *testing.T) {
	t.Run("clean diff flips", func(t *testing.T) {
		f, _ := setupFake(t)
		f.files = []string{"frontend/src/lib/ledger.ts"}
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		f.status = greenStatus()
		if code := run(readyArgs(exampleRepo)); code != 0 {
			t.Fatalf("clean private-repo diff exit = %d, want 0", code)
		}
		if f.flips != 1 {
			t.Fatalf("flips = %d, want 1", f.flips)
		}
	})
	t.Run("security-path diff blocks", func(t *testing.T) {
		f, _ := setupFake(t)
		f.files = []string{"secrets/prod/token.yaml"}
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		f.status = greenStatus()
		if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
			t.Fatalf("security-path diff exit = %d, want %d", code, deskkit.ExitRefused)
		}
		if f.flips != 0 {
			t.Fatalf("flips = %d, want 0", f.flips)
		}
	})
}

// ---------------------------------------------------------------------------
// The gate's INPUT: the changed-file list. Three ways it used to lie.
// ---------------------------------------------------------------------------

// TestReadyShortFileReadIsUnverifiable — listFiles walked to a short page and returned,
// believing it had seen everything. GitHub's own changed_files says otherwise, so the
// risk-class determination is unverifiable (exit 6), not "clean".
//
// The concrete attack: pad the PR with files that page ahead of secrets/evil.key, and the
// walk never reaches the trigger. The cross-check is free — GET /pulls/{n} already
// carries the count, exactly as the CI rollups carry total_count.
func TestReadyShortFileReadIsUnverifiable(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"docs/pad.md"}
	f.changedFilesCount = 3000 // GitHub says the diff is far bigger than we read
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitUnverifiable {
		t.Fatalf("short changed-file read exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a diff we could not read in full must not flip", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "could not be read in full") {
		t.Fatalf("refusal must name the cause; audit detail = %q", d)
	}
}

// TestReadyCompleteFileReadFlips — the control: counts agree, so the reconciliation does
// not block an honest PR.
func TestReadyCompleteFileReadFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"docs/a.md", "docs/b.md"}
	f.changedFilesCount = 2
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("complete clean read exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyRenameOutOfSecurityPathIsRiskClassed — `git mv secrets/auth/token.go
// config/authz/token.go` plus arbitrary
// edits. GitHub reports ONLY the new path, which matches no trigger; previous_filename
// is the only thing that keeps the gate armed. One plausibly-deniable refactor would
// otherwise de-risk-class the most security-relevant edit shape in the repo.
func TestReadyRenameOutOfSecurityPathIsRiskClassed(t *testing.T) {
	f, _ := setupFake(t)
	f.fileEntries = []prFile{{
		Filename:         "config/authz/token.go",
		PreviousFilename: "secrets/auth/token.go",
		Status:           "renamed",
	}}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("rename out of a security path exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — one git mv must not waive the security gate", f.flips)
	}
}

// TestReadyRenameIntoSecurityPathIsRiskClassed — the mirror case; the NEW path triggers.
func TestReadyRenameIntoSecurityPathIsRiskClassed(t *testing.T) {
	f, _ := setupFake(t)
	f.fileEntries = []prFile{{
		Filename:         "secrets/moved.key",
		PreviousFilename: "docs/Moved.md",
		Status:           "renamed",
	}}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("rename into a security path exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestPRFilePathsCarriesBothPaths — the flattening itself.
func TestPRFilePathsCarriesBothPaths(t *testing.T) {
	got := prFilePaths([]prFile{
		{Filename: "a.go"},
		{Filename: "new/b.go", PreviousFilename: "old/b.go", Status: "renamed"},
		{Filename: "same.go", PreviousFilename: "same.go"}, // degenerate: not duplicated
		{Filename: ""}, // malformed entry survives, so the classifier can fail closed on it
	})
	want := []string{"a.go", "new/b.go", "old/b.go", "same.go", ""}
	if len(got) != len(want) {
		t.Fatalf("prFilePaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prFilePaths = %v, want %v", got, want)
		}
	}
}
