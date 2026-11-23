package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- the claim read in the repository that took it (#7490) ---------------------------
//
// A cross-repository verifier claims in the TRACKING repository but cuts its home from the
// deliverable checkout. The home records that repository (assay.claimRepo), and the claim is
// read there: released → the safety gates decide; held or unreadable → refused, as before.
// A home recorded before claim namespaces were recorded names its repository with
// `remove --claim-repo`.

// trackingRepo is the fixture's tracking repository: a bare repository on the same "forge" as
// the fixture's origin (beside it, under the same parent), at the address claimRepoURL derives
// for other-org/tracking.
func trackingRepo(t *testing.T, work string) string {
	t.Helper()
	origin := originBare(t, work)
	bare := filepath.Join(filepath.Dir(filepath.Dir(origin)), "other-org", "tracking.git")
	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	return bare
}

// holdClaimIn plants key's dispatch claim in the bare repository at bare.
func holdClaimIn(t *testing.T, work, bare, key string) {
	t.Helper()
	src := mustGit(t, work, "rev-parse", "origin/main")
	ref := deskkit.DispatchClaimActiveRefsPrefix + key
	mustGit(t, work, "push", "--quiet", bare, src+":"+ref)
}

// crossRepoHome is a bare-locked home whose claim was taken in other-org/tracking.
func crossRepoHome(t *testing.T, work, name, key string) string {
	t.Helper()
	target := addWorktree(t, name)
	runKeyedIn(t, target, key, "other-org/tracking", deskkit.ClaimStoreForgeRef)
	bareLock(t, work, target)
	return target
}

// A released cross-repository claim frees a clean, pushed home. Pre-fix: could-not-check
// "taken in other-org/tracking, not in this checkout's origin", exit 6, forever.
// regression: #7490
func TestRemoveCrossRepoReleased(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	tracking := trackingRepo(t, work)
	holdClaim(t, work, "unrelated-origin-claim")
	holdClaimIn(t, work, tracking, "unrelated-tracking-claim")
	target := crossRepoHome(t, work, "xrel", "item-x1")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// The key is held in the tracking repository and absent from origin: refused, kept.
// regression: #7490
func TestRemoveCrossRepoHeld(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	tracking := trackingRepo(t, work)
	target := crossRepoHome(t, work, "xheld", "item-x2")
	holdClaimIn(t, work, tracking, "item-x2")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !strings.Contains(errout, "dispatch claim item-x2 is held in other-org/tracking") {
		t.Fatalf("the refusal did not name the held claim and its repository; stderr:\n%s", errout)
	}
}

// A released cross-repository claim never relaxes the safety gates: a dirty tracked file
// still refuses, with nothing deleted.
// regression: #7490
func TestRemoveCrossRepoDirty(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	trackingRepo(t, work)
	target := crossRepoHome(t, work, "xdirty", "item-x3")
	writeFile(t, filepath.Join(target, "README.md"), "edited, not committed\n")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
}

// The reap arm reads the same repository: a released cross-repository home is reaped.
// regression: #7490
func TestReapCrossRepoReleased(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	trackingRepo(t, work)
	target, _ := openPRWorktree(t, work, "xreap")
	runKeyedIn(t, target, "item-x4", "other-org/tracking", deskkit.ClaimStoreForgeRef)
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
	if !strings.Contains(errout, "dispatch claim item-x4 is released in other-org/tracking") {
		t.Fatalf("the reap did not name the released claim and its repository; stderr:\n%s", errout)
	}
}

// A home with no recorded claim namespace still holds, and the refusal prints the sanctioned
// path: --claim-repo.
// regression: #7490
func TestRemoveLegacyNamesFlag(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "lgnames")
	runKeyedIn(t, target, "item-l1", "", "")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "deskwt remove <home> --claim-repo <owner/name>") {
		t.Fatalf("the refusal did not print the sanctioned path; stderr:\n%s", errout)
	}
}

// --claim-repo naming this checkout's origin frees a legacy home whose claim is absent there.
// regression: #7490
func TestRemoveLegacyFlagFreed(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "lgfree")
	runKeyedIn(t, target, "item-l2", "", "")
	bareLock(t, work, target)
	repo, err := currentRepo(target)
	if err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"remove", target, "--claim-repo", repo})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// --claim-repo never frees a live claim: held in the named repository, refused.
// regression: #7490
func TestRemoveLegacyFlagHeld(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "lgheld")
	runKeyedIn(t, target, "item-l3", "", "")
	bareLock(t, work, target)
	holdClaim(t, work, "item-l3")
	repo, err := currentRepo(target)
	if err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"remove", target, "--claim-repo", repo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "dispatch claim item-l3 is held") {
		t.Fatalf("the refusal did not name the held claim; stderr:\n%s", errout)
	}
}

// --claim-repo naming the tracking repository reads the claim THERE (a legacy verifier home).
// regression: #7490
func TestRemoveLegacyFlagTracking(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	tracking := trackingRepo(t, work)
	target := addWorktree(t, "lgtrack")
	runKeyedIn(t, target, "item-l4", "", "")
	bareLock(t, work, target)
	holdClaimIn(t, work, tracking, "item-l4")

	rc, errout := runCapErr(t, []string{"remove", target, "--claim-repo", "other-org/tracking"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "is held in other-org/tracking") {
		t.Fatalf("the refusal did not read the tracking repository; stderr:\n%s", errout)
	}
}

// --claim-repo that contradicts the repository the home records is refused, never preferred.
// regression: #7490
func TestRemoveClaimRepoContradicts(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	trackingRepo(t, work)
	target := addWorktree(t, "lgcontra")
	runKeyed(t, target, "item-l5")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"remove", target, "--claim-repo", "other-org/tracking"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "contradicts the claim repository the home records") {
		t.Fatalf("the refusal did not name the contradiction; stderr:\n%s", errout)
	}
}

// --claim-repo takes owner/name; anything else is a usage error that touches nothing.
// regression: #7490
func TestRemoveClaimRepoBadSlug(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "lgbad")
	runKeyedIn(t, target, "item-l6", "", "")

	for _, bad := range []string{"no-slash", "../escape", "a/b/c", "a b/c"} {
		rc, errout := runCapErr(t, []string{"remove", target, "--claim-repo=" + bad})
		if rc != 2 {
			t.Fatalf("--claim-repo %q rc = %d, want 2; stderr:\n%s", bad, rc, errout)
		}
		assertExists(t, target)
	}
}
