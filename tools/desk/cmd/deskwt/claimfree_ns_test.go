package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- the claim namespace (SEC-1 / F1) ---------------------------------------------

// namespaceCases are homes whose run key is absent from THIS checkout's claim listing without
// that proving a release: the claim was taken elsewhere, in another store, or nowhere recorded.
var namespaceCases = []struct {
	name, repo, store, want string
}{
	// A cross-repository verifier claims in the TRACKING repo; its home is cut from the
	// deliverable checkout, so its live claim is absent from the deliverable's listing. The
	// listing is read in the tracking repository instead (crossrepo_test.go); one that cannot
	// be read there (this fixture has no such repository) holds.
	{"other-repo", "other-org/tracking", deskkit.ClaimStoreForgeRef, "claim listing in other-org/tracking unreadable"},
	{"other-store", "", deskkit.ClaimStoreFile, "taken in the file store"},
	{"no-store", "", "", "records no claim store"},
	{"no-repo", "NOREPO", deskkit.ClaimStoreForgeRef, "records no claim repository"},
}

func namespaceRepo(t *testing.T, target, repo string) string {
	t.Helper()
	switch repo {
	case "":
		r, err := currentRepo(target)
		if err != nil {
			t.Fatal(err)
		}
		return r
	case "NOREPO":
		return ""
	}
	return repo
}

// A bare-locked home whose key is absent from this checkout's listing, but whose claim was
// taken in another repository (or store, or nowhere recorded), is HELD by the reap arm.
// regression: #6863
func TestReapClaimNamespaceHolds(t *testing.T) {
	for _, tc := range namespaceCases {
		for _, locked := range []bool{true, false} {
			name := tc.name + "-unlocked"
			if locked {
				name = tc.name + "-locked"
			}
			t.Run(name, func(t *testing.T) {
				work := newRepo(t)
				withEnv(t, work)
				target, _ := openPRWorktree(t, work, "ns"+tc.name)
				runKeyedIn(t, target, "item-ns", namespaceRepo(t, target, tc.repo), tc.store)
				if locked {
					bareLock(t, work, target)
				}

				rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
				if rc != deskkit.ExitOK {
					t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
				}
				assertExists(t, target)
				assertLocked(t, work, target, locked)
				// A locked tree is held first by the reap arm's generic gates, which print no
				// claim reason; the unlocked tree is held by the claim gate and names it.
				if !locked && (!strings.Contains(errout, "could-not-check: dispatch claim item-ns") || !strings.Contains(errout, tc.want)) {
					t.Fatalf("the hold did not say could-not-check (%q); stderr:\n%s", tc.want, errout)
				}
			})
		}
	}
}

// The same homes are could-not-check (exit 6) for `remove`, never removed under a bare lock.
// regression: #6863
func TestRemoveClaimNamespaceUnverifiable(t *testing.T) {
	for _, tc := range namespaceCases {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			target := addWorktree(t, "rmns"+tc.name)
			runKeyedIn(t, target, "item-ns", namespaceRepo(t, target, tc.repo), tc.store)
			bareLock(t, work, target)

			rc, errout := runCapErr(t, []string{"remove", target})
			if rc != deskkit.ExitUnverifiable {
				t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
			}
			assertExists(t, target)
			if !strings.Contains(errout, tc.want) {
				t.Fatalf("the refusal did not name %q; stderr:\n%s", tc.want, errout)
			}
		})
	}
}

// --- remove reads only its own admin entry (F2) -------------------------------------

// A malformed SIBLING admin entry must not change the verdict on a home with no run key.
// regression: #6863
func TestRemoveIgnoresBrokenSibling(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	sibling := addWorktree(t, "sibling")
	target := addWorktree(t, "plain")
	admin := filepath.Join(work, ".git", "worktrees", filepath.Base(sibling), "gitdir")
	writeFile(t, admin, "")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// A home's OWN admin entry that cannot be confirmed holds it, whatever the run key says.
// regression: #6863
func TestClaimForRemoveOwnEntryUnreadable(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "ownbad")
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	rt := resolvePath(target)
	if c, _ := claimForRemove(guard, work, rt, ""); c.holds() || c.key != "" {
		t.Fatalf("a readable own entry with no run key must pass untouched; got %+v", c)
	}
	writeFile(t, filepath.Join(target, ".git"), "gitdir: "+filepath.Join(t.TempDir(), "elsewhere")+"\n")
	if c, _ := claimForRemove(guard, work, rt, ""); c.state != claimUnknown || !c.holds() {
		t.Fatalf("an unconfirmable own admin entry must hold as could-not-check; got %+v", c)
	}
}

// --- fail-closed on an unreadable run key or admin registry (F3) ---------------------

// A config.worktree that cannot be parsed is a run key that could not be read: the unlocked,
// clean, pushed tree is held by the reap arm and could-not-check for remove.
// regression: #6863
func TestRunKeyUnreadableHolds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "badcfg")
	writeFile(t, filepath.Join(work, ".git", "worktrees", filepath.Base(target), "config.worktree"), "[assay\n\trunKey = x\n")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "could-not-check: run key") {
		t.Fatalf("the hold did not say could-not-check: run key; stderr:\n%s", errout)
	}

	rc, errout = runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	assertExists(t, target)
}

// A sweep whose admin registry cannot be read holds a home whose OWN admin entry cannot be
// confirmed either, as an unknown claim (F8 keeps this hold; only a confirmed own entry lifts it).
// regression: #6863
func TestSweepClaimAdminRegistryUnreadable(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "badreg")
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	// Break the target's own back-pointer: its entry no longer confirms it.
	writeFile(t, filepath.Join(work, ".git", "worktrees", filepath.Base(target), "gitdir"), "")
	sc := &sweepCtx{commonDir: guard.commonDir,
		admins: &worktreeAdminIndex{paths: map[string]string{}, ambiguous: map[string]bool{}, err: errors.New("registry unreadable")}}
	c := sc.claimFor(guard, work, resolvePath(target))
	if c.state != claimUnknown || !c.holds() || !strings.Contains(c.why, "could-not-check: run key") {
		t.Fatalf("an unreadable registry with an unconfirmable own entry must hold as could-not-check; got %+v", c)
	}
}

// --- git's initializing lock is never retired (F3) -----------------------------------

// regression: #6863
func TestReapInitLockHeldUnderReleasedClaim(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "initreap")
	runKeyed(t, target, "item-i")
	mustGit(t, work, "worktree", "lock", "--reason", gitInitLockReason, target)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
}

// regression: #6863
func TestRemoveInitLockRefusedUnderReleasedClaim(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "initrm")
	runKeyed(t, target, "item-j")
	mustGit(t, work, "worktree", "lock", "--reason", gitInitLockReason, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
}

// --- the listing reader (F3, SEC-3, F7) ---------------------------------------------

// regression: #6863
func TestParseClaimListing(t *testing.T) {
	const oid = "0123456789abcdef0123456789abcdef01234567"
	got, err := parseClaimListing(strings.Join([]string{
		oid + "\trefs/dispatch/active-one",
		oid + "\trefs/heads/dispatch/branch-shaped",
		// A nested key is a claim the claim tool accepts and lists verbatim: dropping it
		// would read a held claim as released (SEC-5).
		oid + "\trefs/dispatch/item--stream/07",
		oid + "\trefs/heads/dispatch/item--stream/08",
		oid + "\trefs/heads/other/foreign",
		"",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"active-one": true, "branch-shaped": true,
		"item--stream/07": true, "item--stream/08": true}
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want exactly %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("keys = %v, missing %q", got, k)
		}
	}
	if _, err := parseClaimListing("not-a-listing-line"); err == nil {
		t.Fatal("an unparseable line must be an error, not an empty listing")
	}
}

// The listing runs without any inherited GIT_* variable and in batch mode.
// regression: #6863
func TestClaimListEnv(t *testing.T) {
	t.Setenv("GIT_DIR", "/nonexistent")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_SSH_COMMAND", "ssh -o BatchMode=no")
	env := claimListEnv()
	seen := map[string]string{}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		if strings.HasPrefix(k, "GIT_") {
			seen[k] = v
		}
	}
	if len(seen) != 2 || seen["GIT_TERMINAL_PROMPT"] != "0" || seen["GIT_SSH_COMMAND"] != "ssh -o BatchMode=yes" {
		t.Fatalf("GIT_* in the listing env = %v, want only GIT_TERMINAL_PROMPT=0 and batch-mode ssh", seen)
	}
}

// --- the re-read before a destructive step (SEC-4 / F6) ------------------------------

// stubClaimRereads makes the FIRST listing (the sweep's) empty and every later one hold key.
func stubClaimRereads(t *testing.T, key string) {
	t.Helper()
	old := listDispatchClaims
	calls := 0
	listDispatchClaims = func(string, string) (map[string]bool, error) {
		calls++
		if calls == 1 {
			return map[string]bool{}, nil
		}
		return map[string]bool{key: true}, nil
	}
	t.Cleanup(func() { listDispatchClaims = old })
}

// A claim re-acquired after the sweep's listing keeps the home.
// regression: #6863
func TestReapClaimReacquiredAfterListing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	stubClaimRereads(t, "item-k")
	target, _ := openPRWorktree(t, work, "reacq")
	runKeyed(t, target, "item-k")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
}

// regression: #6863
func TestRemoveClaimReacquiredAfterListing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	stubClaimRereads(t, "item-l")
	target := addWorktree(t, "reacqrm")
	runKeyed(t, target, "item-l")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
}
