package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The identity role-init stamps is now DERIVED from the roster (ASSAY_TRUSTED_BOT_SLUGS),
// not a source literal — so this expected value is the fixture roster's neutral verifier
// binding (verifier=assay-verifier-app:300000005 in fixtureRoster), never a real bot USER id.
const verifierBotEmail = "300000005+assay-verifier-app[bot]@users.noreply.github.com"

// TestRoleInitProvisionsLockedWorktreeWithScopedIdentity is the happy path AND the
// worktree-scoped-identity regression: role-init must create a session-scoped worktree on a
// uniquely-named branch tracking origin/main, lock it, and stamp the role's App identity in
// the worktree-SCOPED config (config.worktree) — never the shared .git/config that every
// linked worktree inherits, which is how concurrent worker/reviewer/verifier sessions were
// observed clobbering each other's commit identity.
func TestRoleInitProvisionsLockedWorktreeWithScopedIdentity(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "sess1"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-sess1")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("worktree dir %s not created: %v", target, err)
	}
	if !hasWorktreeVerb(*calls, "add") {
		t.Fatalf("expected a `git worktree add`; git calls: %v", gitCalls(*calls))
	}
	if !hasWorktreeVerb(*calls, "lock") {
		t.Fatalf("expected a `git worktree lock`; git calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag: %v", gitCalls(*calls))
	}

	// The branch is uniquely named and tracks origin/main (so the preflight landing probe
	// has a ref, and a linked worktree never needs to check out `main`).
	if br := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); br != "verify-desk/sess1" {
		t.Fatalf("worktree branch = %q, want verify-desk/sess1", br)
	}
	if up := mustGit(t, target, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); up != "origin/main" {
		t.Fatalf("upstream = %q, want origin/main", up)
	}

	// The identity is written to the WORKTREE-scoped config, and NOT to the shared config.
	if got := mustGit(t, target, "config", "--worktree", "--get", "user.email"); got != verifierBotEmail {
		t.Fatalf("worktree-scoped user.email = %q, want %q", got, verifierBotEmail)
	}
	sharedEmail := mustGit(t, "", "config", "--file", filepath.Join(work, ".git", "config"), "--get", "user.email")
	if sharedEmail == verifierBotEmail {
		t.Fatalf("verifier identity leaked into SHARED .git/config (%q) — it must be worktree-scoped", sharedEmail)
	}

	// The live race: another process rewrites the SHARED user.email. The worktree override
	// must still win, so this session keeps committing as the verifier.
	mustGit(t, work, "config", "user.email", "assay-worker-app[bot]@users.noreply.github.com")
	if got := mustGit(t, target, "config", "--get", "user.email"); got != verifierBotEmail {
		t.Fatalf("after a shared-config race the effective worktree user.email = %q, want %q "+
			"(the worktree-scoped override must win over shared)", got, verifierBotEmail)
	}
}

// TestPerCommitIdentityOverrideBeatsAnyRacedConfig proves the PRIMARY layer of the
// commit-identity design: a per-commit `git -c user.email=…` override produces that author even
// when BOTH the shared .git/config AND the worktree-scoped config have been rewritten to
// something else by concurrent sessions. This is why the one local-commit path (the raw
// App-push fallback in the skill) supplies the identity inline rather than trusting any
// persisted config — the worktree-scoped identity role-init sets is defense-in-depth, but the
// inline override is what makes the commit race-proof by construction.
func TestPerCommitIdentityOverrideBeatsAnyRacedConfig(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "race"}); rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-race")

	// Two concurrent sessions clobber BOTH config scopes to the wrong identity.
	mustGit(t, work, "config", "user.email", "assay-worker-app[bot]@users.noreply.github.com")                   // shared
	mustGit(t, target, "config", "--worktree", "user.email", "assay-reviewer-app[bot]@users.noreply.github.com") // worktree
	mustGit(t, work, "config", "user.name", "assay-worker-app[bot]")
	mustGit(t, target, "config", "--worktree", "user.name", "assay-reviewer-app[bot]")

	// A per-commit override still wins — the commit author is the verifier.
	writeFile(t, filepath.Join(target, "evidence.txt"), "row\n")
	mustGit(t, target, "add", "evidence.txt")
	mustGit(t, target,
		"-c", "user.name=assay-verifier-app[bot]",
		"-c", "user.email="+verifierBotEmail,
		"commit", "-m", "verify: evidence")
	if got := mustGit(t, target, "log", "-1", "--format=%ae"); got != verifierBotEmail {
		t.Fatalf("commit author email = %q, want %q — the per-commit override must beat any raced "+
			"shared OR worktree config", got, verifierBotEmail)
	}
}

// TestRoleInitIsIdempotent — a second role-init for the same role+session reuses the existing
// worktree as a NOOP rather than clobbering or erroring, and re-applies the identity/lock.
func TestRoleInitIsIdempotent(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "dup"}); rc != deskkit.ExitOK {
		t.Fatalf("first role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-dup")
	// Drop a sentinel so a clobber-and-recreate would be detectable.
	writeFile(t, filepath.Join(target, "SENTINEL"), "keep me\n")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "dup"})
	if rc != deskkit.ExitOK {
		t.Fatalf("second role-init rc = %d, want 0 (idempotent reuse); stderr: %s", rc, stderr)
	}
	if _, err := os.Stat(filepath.Join(target, "SENTINEL")); err != nil {
		t.Fatalf("idempotent reuse destroyed the existing worktree (sentinel gone): %v", err)
	}
	// The audit line for the reuse is a noop.
	entries, _ := deskkit.LoadEntries()
	if len(entries) == 0 {
		t.Fatal("expected audit entries")
	}
	if last := entries[len(entries)-1]; last.Result != deskkit.ResultNoop {
		t.Fatalf("reuse audit result = %q, want %q", last.Result, deskkit.ResultNoop)
	}
}

// TestRoleInitRefusesStrayTargetWithoutClobbering — a directory occupying the session-scoped
// path that is NOT a registered worktree of this repo (a stray, or a foreign repo's checkout)
// is refused, and nothing on disk is touched. This is the fail-closed half of the origin
// identity guard.
func TestRoleInitRefusesStrayTargetWithoutClobbering(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-stray")
	writeFile(t, filepath.Join(target, "foreign"), "not ours\n")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "stray"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("role-init over a stray dir rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	assertExists(t, filepath.Join(target, "foreign"))
}

// TestRoleCleanUnlocksAndRemoves — role-clean tears down a role worktree it provisioned,
// UNLOCKING it first so the admin entry can be pruned (a locked entry is skipped by prune,
// which would leave it registered after the directory is gone).
func TestRoleCleanUnlocksAndRemoves(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "gc"}); rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-gc")
	resetCalls(calls)

	rc, stderr := runCapErr(t, []string{"role-clean", "--role", "verifier", "--session", "gc"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-clean rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !hasWorktreeVerb(*calls, "unlock") {
		t.Fatalf("expected a `git worktree unlock` before removal; git calls: %v", gitCalls(*calls))
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still present after role-clean: %v", err)
	}
	// It is no longer registered.
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if contains(list, target) {
		t.Fatalf("worktree still registered after role-clean:\n%s", list)
	}
}

// TestRoleCleanNoopWhenAbsent — role-clean with nothing to remove is a clean noop, not an
// error (idempotent teardown).
func TestRoleCleanNoopWhenAbsent(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	rc, stderr := runCapErr(t, []string{"role-clean", "--role", "verifier", "--session", "absent"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-clean with nothing to clean rc = %d, want 0; stderr: %s", rc, stderr)
	}
}

// TestRoleInitBadArgs — an unknown role and a missing session both refuse (exit 5).
func TestRoleInitBadArgs(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// Clear BOTH session sources so no session can be resolved for the missing-session case:
	// resolveSession consults $DESK_SESSION before $CLAUDE_SESSION_ID, so an ambient
	// DESK_SESSION (a real desk running the suite) would otherwise satisfy the refusal case.
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	if rc, _ := runCapErr(t, []string{"role-init", "--role", "nobody", "--session", "s"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown role rc = %d, want 5", rc)
	}
	if rc, _ := runCapErr(t, []string{"role-init", "--role", "verifier"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing session rc = %d, want 5", rc)
	}
}

// TestRoleInitRefusesWhenRoleIdentityUnbound pins the net-new fail-closed branch: when the
// roster does not bind the role to a bot commit identity (unconfigured, role unbound, or the
// bot USER id unpinned), role-init REFUSES rather than stamping an account-unlinked identity —
// the #638 class the roster-sourced identity exists over. The refusal names the env var and
// the config path, and nothing is provisioned (the check sits before the working-dir read).
func TestRoleInitRefusesWhenRoleIdentityUnbound(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	// Re-plant a roster that binds every role EXCEPT verifier, so RoleBotCommitIdentity
	// returns not-ok for it. Same HOME withEnv already pointed the config home at.
	unbound := strings.ReplaceAll(fixtureRoster, "verifier=assay-verifier-app:300000005,", "")
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(unbound), 0o600); err != nil {
		t.Fatalf("re-plant roster: %v", err)
	}
	deskkit.ReloadConfig()

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "unbound"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("role-init with an unbound role identity rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	if !contains(stderr, "ASSAY_TRUSTED_BOT_SLUGS") {
		t.Fatalf("refusal must name the env var to pin the identity; stderr: %s", stderr)
	}
	// Nothing was provisioned: no worktree add, and the target dir does not exist.
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was created despite the fail-closed refusal; git calls: %v", gitCalls(*calls))
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-verify-desk-unbound")); !os.IsNotExist(err) {
		t.Fatalf("target dir exists after a fail-closed refusal: %v", err)
	}
}

// --- #677 part (b): role-init works for every desk role, not only the verifier -------

// TestRoleInitSupportsAllDeskRoles pins the #677 part (b) fix: roleWorktreeConfig maps
// EVERY loop deskboot can boot (the-desk / worker-desk / pr-review-desk / verify-desk /
// intake-desk), keyed on the App TOKEN role, so `deskwt role-init --role <role>` provisions
// a worktree for each — not only the verifier. Pre-fix the four non-verifier roles are
// absent from roleWorktreeConfig, so parseRoleParams refuses each with exit 5 (unknown
// role) and this test's rc==0 assertion fails. The GitHub identity for each is stamped
// worktree-scoped from the fixture roster.
func TestRoleInitSupportsAllDeskRoles(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	cases := []struct {
		role, branchPrefix, email string
	}{
		{"desk", "the-desk", "300000001+assay-desk-app[bot]@users.noreply.github.com"},
		{"worker", "worker-desk", "300000006+assay-worker-app[bot]@users.noreply.github.com"},
		{"reviewer", "pr-review-desk", "300000004+assay-reviewer-app[bot]@users.noreply.github.com"},
		{"issue-loop", "intake-desk", "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			rc, stderr := runCapErr(t, []string{"role-init", "--role", c.role, "--session", "s"})
			if rc != deskkit.ExitOK {
				t.Fatalf("role-init --role %s rc = %d, want 0; stderr: %s", c.role, rc, stderr)
			}
			target := filepath.Join(tmpBaseDir, "tracker-"+c.branchPrefix+"-s")
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("worktree dir %s not created: %v", target, err)
			}
			if br := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); br != c.branchPrefix+"/s" {
				t.Fatalf("worktree branch = %q, want %q", br, c.branchPrefix+"/s")
			}
			if got := mustGit(t, target, "config", "--worktree", "--get", "user.email"); got != c.email {
				t.Fatalf("worktree-scoped user.email = %q, want %q", got, c.email)
			}
		})
	}
}

// TestRoleWorktreeConfigMatchesLoopTokenRoles is the drift guard: roleWorktreeConfig must
// cover every loop deskboot can boot, keyed on each loop's App token role, with the
// branch prefix equal to the loop name — AND every role desktoken mints (its fixed set:
// desk / worker / reviewer / verifier / issue-loop / intake-loop), so no roster-bound role
// is left with no way to isolate. It fails pre-fix because only the verifier is mapped.
func TestRoleWorktreeConfigMatchesLoopTokenRoles(t *testing.T) {
	loopRoles := deskkit.LoopTokenRoles() // map[loop]tokenRole
	for loop, role := range loopRoles {
		cfg, ok := roleWorktreeConfig[role]
		if !ok {
			t.Fatalf("roleWorktreeConfig is missing token role %q (loop %q) — role-init cannot provision it", role, loop)
		}
		if cfg.branchPrefix != loop {
			t.Fatalf("roleWorktreeConfig[%q].branchPrefix = %q, want the loop name %q", role, cfg.branchPrefix, loop)
		}
	}
	// desktoken's fixed role set (cmd/desktoken validRoles) — restated here because the two
	// are separate main packages; a role added there must be provisionable here.
	desktokenRoles := []string{"desk", "worker", "reviewer", "verifier", "issue-loop", "intake-loop"}
	for _, role := range desktokenRoles {
		if _, ok := roleWorktreeConfig[role]; !ok {
			t.Fatalf("roleWorktreeConfig is missing desktoken role %q — a role that mints a token must be able to isolate", role)
		}
	}
	if len(roleWorktreeConfig) != len(desktokenRoles) {
		t.Fatalf("roleWorktreeConfig has %d roles, want %d (desktoken's fixed set): %v",
			len(roleWorktreeConfig), len(desktokenRoles), roleWorktreeConfig)
	}
	// Every configured role resolves to a bot commit identity under the fixture roster —
	// otherwise role-init would refuse it at the identity gate, which is a silent
	// re-introduction of "refuses every role but one".
	work := newRepo(t)
	withEnv(t, work)
	for role := range roleWorktreeConfig {
		if _, _, ok := deskkit.RoleBotCommitIdentity(role); !ok {
			t.Fatalf("fixture roster binds no commit identity for role %q", role)
		}
	}
}

// --- every desk role, every spelling: the isolate-first contract ------------------------

// runCapBoth runs deskwt with args, capturing BOTH streams (runCapOut from prune_test.go
// around runCapErr). stdout is what a launcher reads (`cd "$(deskwt role-init …)"`), so the
// tests below assert on it, not only on the rc.
func runCapBoth(t *testing.T, args []string) (rc int, stdout, stderr string) {
	t.Helper()
	stdout = runCapOut(t, func() { rc, stderr = runCapErr(t, args) })
	return rc, stdout, stderr
}

// lastLine returns the last non-empty line of s.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

// isLocked reports whether `git worktree list --porcelain` shows target as locked.
func isLocked(t *testing.T, work, target string) bool {
	t.Helper()
	porcelain := mustGit(t, work, "worktree", "list", "--porcelain")
	rt := resolvePath(target)
	blocks := strings.Split(porcelain, "\n\n")
	for _, b := range blocks {
		if strings.Contains(b, "worktree "+rt) || strings.Contains(b, "worktree "+target) {
			return strings.Contains(b, "\nlocked")
		}
	}
	return false
}

// roleSpellingCases is the role table the isolate-first contract is proven over: every
// role desktoken mints, spelled as its TOKEN role and — where deskboot boots a loop for it
// — as that LOOP name (the spelling deskboot's own refusal prints), plus the retired
// `batch-fanout` spelling the kill switch still honours for worker-desk.
var roleSpellingCases = []struct {
	spelling, branchPrefix, email string
}{
	{"desk", "the-desk", "300000001+assay-desk-app[bot]@users.noreply.github.com"},
	{"the-desk", "the-desk", "300000001+assay-desk-app[bot]@users.noreply.github.com"},
	{"worker", "worker-desk", "300000006+assay-worker-app[bot]@users.noreply.github.com"},
	{"worker-desk", "worker-desk", "300000006+assay-worker-app[bot]@users.noreply.github.com"},
	{"batch-fanout", "worker-desk", "300000006+assay-worker-app[bot]@users.noreply.github.com"},
	{"reviewer", "pr-review-desk", "300000004+assay-reviewer-app[bot]@users.noreply.github.com"},
	{"pr-review-desk", "pr-review-desk", "300000004+assay-reviewer-app[bot]@users.noreply.github.com"},
	{"verifier", "verify-desk", verifierBotEmail},
	{"verify-desk", "verify-desk", verifierBotEmail},
	{"issue-loop", "intake-desk", "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"},
	{"intake-desk", "intake-desk", "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"},
	{"intake-loop", "intake-loop", "300000002+assay-intake-loop-app[bot]@users.noreply.github.com"},
}

// TestRoleInitEveryRoleEverySpelling is the role table: for every role, spelled as its
// token role OR its loop name, given POSITIONALLY (the launcher form) and run from OUTSIDE
// the checkout via --repo-root (the shared-checkout boot case — cwd is nowhere near the
// repo), role-init (1) creates the worktree on `<loop>/<session>` tracking origin/main,
// (2) LOCKS it, (3) stamps the role's identity worktree-scoped, and (4) prints the
// worktree's ABSOLUTE path as the last stdout line. Pre-fix, every loop-name spelling and
// `intake-loop` refuse (exit 5) and there is no --repo-root, so the loop-name command
// deskboot prints is one the tool then refuses.
func TestRoleInitEveryRoleEverySpelling(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// cwd is an unrelated, non-git directory: --repo-root must carry the checkout.
	elsewhere := t.TempDir()
	getwd = func() (string, error) { return elsewhere, nil }

	for i, c := range roleSpellingCases {
		sess := "s" + string(rune('a'+i))
		t.Run(c.spelling, func(t *testing.T) {
			rc, stdout, stderr := runCapBoth(t, []string{"role-init", c.spelling, "--repo-root", work, "--session", sess})
			if rc != deskkit.ExitOK {
				t.Fatalf("role-init %s rc = %d, want 0; stderr: %s", c.spelling, rc, stderr)
			}
			target := filepath.Join(tmpBaseDir, "tracker-"+c.branchPrefix+"-"+sess)
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("worktree dir %s not created: %v", target, err)
			}
			if got := lastLine(stdout); got != target || !filepath.IsAbs(got) {
				t.Fatalf("last stdout line = %q, want the absolute worktree path %q", got, target)
			}
			if br := mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD"); br != c.branchPrefix+"/"+sess {
				t.Fatalf("worktree branch = %q, want %q", br, c.branchPrefix+"/"+sess)
			}
			if up := mustGit(t, target, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); up != "origin/main" {
				t.Fatalf("upstream = %q, want origin/main", up)
			}
			if !isLocked(t, work, target) {
				t.Fatalf("worktree %s is not locked — an unlocked live worktree is reclaimable by the prune supervisor", target)
			}
			if got := mustGit(t, target, "config", "--worktree", "--get", "user.email"); got != c.email {
				t.Fatalf("worktree-scoped user.email = %q, want %q", got, c.email)
			}
		})
	}
}

// TestRoleInitRefusesUnknownRoleAndDoubleRole — a spelling that is neither a token role
// nor a loop name refuses (exit 5) naming both accepted vocabularies; naming the role twice
// with two different values refuses rather than picking one; nothing is provisioned.
func TestRoleInitRefusesUnknownRoleAndDoubleRole(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	rc, stderr := runCapErr(t, []string{"role-init", "nobody-desk", "--session", "s"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unknown role rc = %d, want 5; stderr: %s", rc, stderr)
	}
	if !contains(stderr, "worker-desk") || !contains(stderr, "verifier") {
		t.Fatalf("refusal must name both the token roles and the loop names; stderr: %s", stderr)
	}
	rc, stderr = runCapErr(t, []string{"role-init", "worker", "--role", "verifier", "--session", "s"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("role given twice and differently rc = %d, want 5; stderr: %s", rc, stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was created on a refused invocation; git calls: %v", gitCalls(*calls))
	}
	// The same role spelled twice — positional loop name, --role token role — is NOT a
	// conflict: both resolve to `worker`, so it provisions.
	if rc, stderr := runCapErr(t, []string{"role-init", "worker-desk", "--role", "worker", "--session", "s"}); rc != deskkit.ExitOK {
		t.Fatalf("same role in two vocabularies rc = %d, want 0; stderr: %s", rc, stderr)
	}
}

// TestRoleInitLeavesSharedCheckoutConfigAndIndexUntouched pins the property the
// shared-homed boot failure class is about: provisioning a role worktree from the SHARED
// checkout must not write that checkout's index, and must not write its user.* config (a
// `git config user.*` in a linked worktree lands in the shared config and clobbers every
// session's identity). Two shared-config writes are INHERENT to the design and are the only
// ones allowed: `[extensions] worktreeConfig = true` (git requires it in the common config
// for the worktree-scoped identity to take effect; idempotent, once per checkout) and the
// new branch's own `[branch "<loop>/<session>"]` tracking section (what `git worktree add
// --track -b` records so the preflight landing probe has an upstream). With those two
// masked, the base .git/config is BYTE-EQUAL before and after — the identity strings never
// appear in it — and the index is byte-equal unmasked.
func TestRoleInitLeavesSharedCheckoutConfigAndIndexUntouched(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cfgPath := filepath.Join(work, ".git", "config")
	idxPath := filepath.Join(work, ".git", "index")

	// maskInherent drops the two inherent sections so everything else must be byte-equal.
	maskInherent := func(s string) string {
		var keep []string
		skipping := false
		for _, l := range strings.Split(s, "\n") {
			tl := strings.TrimSpace(l)
			if strings.HasPrefix(tl, "[") {
				skipping = tl == "[extensions]" || strings.HasPrefix(tl, "[branch \"")
			}
			if skipping {
				continue
			}
			keep = append(keep, l)
		}
		return strings.TrimRight(strings.Join(keep, "\n"), "\n")
	}

	for _, c := range []struct{ role, session, identity string }{
		{"worker-desk", "cfgA", "assay-worker-app"},      // first role-init: enables the extension
		{"pr-review-desk", "cfgB", "assay-reviewer-app"}, // steady state: extension already on
	} {
		cfgBefore, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		idxBefore, err := os.ReadFile(idxPath)
		if err != nil {
			t.Fatal(err)
		}
		if rc, stderr := runCapErr(t, []string{"role-init", c.role, "--session", c.session}); rc != deskkit.ExitOK {
			t.Fatalf("role-init %s rc = %d, want 0; stderr: %s", c.role, rc, stderr)
		}
		cfgAfter, _ := os.ReadFile(cfgPath)
		idxAfter, _ := os.ReadFile(idxPath)
		if string(idxBefore) != string(idxAfter) {
			t.Fatalf("role-init %s rewrote the SHARED checkout's index", c.role)
		}
		if got := mustGit(t, "", "config", "--file", cfgPath, "--get", "user.email"); got != "t@e.st" {
			t.Fatalf("shared user.email = %q after role-init %s, want the untouched fixture value t@e.st", got, c.role)
		}
		if got := mustGit(t, "", "config", "--file", cfgPath, "--get", "user.name"); got != "Test" {
			t.Fatalf("shared user.name = %q after role-init %s, want the untouched fixture value Test", got, c.role)
		}
		if strings.Contains(string(cfgAfter), c.identity) {
			t.Fatalf("the %s identity leaked into the SHARED .git/config:\n%s", c.identity, cfgAfter)
		}
		if maskInherent(string(cfgBefore)) != maskInherent(string(cfgAfter)) {
			t.Fatalf("shared .git/config changed beyond the extension enable and the new branch's own section (role-init %s):\n--- before\n%s\n--- after\n%s",
				c.role, cfgBefore, cfgAfter)
		}
		if c.session == "cfgB" && !strings.Contains(string(cfgBefore), "worktreeConfig") {
			t.Fatal("steady-state case expected extensions.worktreeConfig to be enabled already")
		}
	}
}

// TestRoleInitCutsFromFreshOriginMain — the worktree is cut from origin's CURRENT main,
// not the checkout's stale remote-tracking ref: origin advances behind the checkout's back,
// and role-init's fresh fetch picks the new tip up. With --no-fetch the stale ref is used
// as-is (the explicit, never-default opt-out).
func TestRoleInitCutsFromFreshOriginMain(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	stale := mustGit(t, work, "rev-parse", "origin/main")

	// Advance origin's main from a second clone — the fixture checkout does not see it.
	other := filepath.Join(t.TempDir(), "other")
	mustGit(t, "", "clone", "--quiet", originBare(t, work), other)
	mustGit(t, other, "config", "user.email", "o@e.st")
	mustGit(t, other, "config", "user.name", "Other")
	mustGit(t, other, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(other, "NEW.md"), "advanced\n")
	mustGit(t, other, "add", "NEW.md")
	mustGit(t, other, "commit", "-q", "-m", "advance main")
	mustGit(t, other, "push", "--quiet", "origin", "main")
	fresh := mustGit(t, other, "rev-parse", "HEAD")
	if fresh == stale {
		t.Fatal("fixture did not advance origin/main")
	}

	rc, stdout, stderr := runCapBoth(t, []string{"role-init", "worker-desk", "--session", "fresh"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := lastLine(stdout)
	if got := mustGit(t, target, "rev-parse", "HEAD"); got != fresh {
		t.Fatalf("worktree HEAD = %s, want origin's FRESH main %s (stale local ref was %s)", got, fresh, stale)
	}
	fetched := false
	for _, c := range gitCalls(*calls) {
		if len(c) > 1 && c[1] == "fetch" {
			fetched = true
		}
	}
	if !fetched {
		t.Fatalf("no `git fetch` recorded; git calls: %v", gitCalls(*calls))
	}

	// --no-fetch: a second role (its own worktree) is cut from whatever origin/main is now,
	// with no fetch issued.
	resetCalls(calls)
	rc, _, stderr = runCapBoth(t, []string{"role-init", "verifier", "--session", "nofetch", "--no-fetch"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init --no-fetch rc = %d, want 0; stderr: %s", rc, stderr)
	}
	for _, c := range gitCalls(*calls) {
		if len(c) > 1 && c[1] == "fetch" {
			t.Fatalf("--no-fetch still fetched: %v", c)
		}
	}
}

// TestRoleInitUnfetchableOriginIsUnverifiable — when the fresh fetch cannot run (origin
// unreachable), role-init is could-not-check (exit 6), never a worktree quietly cut from a
// possibly-stale base; the message names --no-fetch as the explicit opt-out.
func TestRoleInitUnfetchableOriginIsUnverifiable(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	mustGit(t, work, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone", "example-org", "tracker.git"))

	rc, stderr := runCapErr(t, []string{"role-init", "worker-desk", "--session", "unreach"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unfetchable origin rc = %d, want 6 (unverifiable); stderr: %s", rc, stderr)
	}
	if !contains(stderr, "--no-fetch") {
		t.Fatalf("the could-not-check must name --no-fetch as the opt-out; stderr: %s", stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was cut despite the failed fetch; git calls: %v", gitCalls(*calls))
	}
}

// TestRoleInitRepoRootMustBeACheckout — an explicit --repo-root is authoritative: one that
// is not a git worktree is could-not-check (exit 6), never a silent fall-back to the cwd.
func TestRoleInitRepoRootMustBeACheckout(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work) // cwd IS a valid checkout — the fall-back that must not happen
	rc, stderr := runCapErr(t, []string{"role-init", "worker-desk", "--repo-root", t.TempDir(), "--session", "rr"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("non-checkout --repo-root rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("fell back to the cwd checkout and provisioned; git calls: %v", gitCalls(*calls))
	}
}

// --- #677 part (a): GitLab worktree commit identity, never the GitHub noreply shape ----

// gitLabRoster rewrites the fixture roster so the verifier role is a GITLAB identity, and
// (when sessionEmails != "") adds the ASSAY_GITLAB_SESSION_EMAILS allowlist. It re-plants
// it under the active HOME and reloads config, mirroring TestRoleInitRefusesWhenRoleIdentityUnbound.
func gitLabRoster(t *testing.T, sessionEmails string) {
	t.Helper()
	r := strings.ReplaceAll(fixtureRoster,
		"verifier=assay-verifier-app:300000005",
		"verifier=gitlab:assay-verifier-sa:41987965")
	if sessionEmails != "" {
		r += "ASSAY_GITLAB_SESSION_EMAILS=" + sessionEmails + "\n"
	}
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(r), 0o600); err != nil {
		t.Fatalf("re-plant GitLab roster: %v", err)
	}
	deskkit.ReloadConfig()
}

// TestRoleInitStampsGitLabSessionIdentity pins the #677 part (a) fix: on a GitLab-bound
// roster with a trusted GitLab session/implementer commit address configured, role-init
// STAMPS that GitLab-shaped address (here the service-account noreply form) — it does NOT
// refuse, and it NEVER falls back to the GitHub noreply shape. Pre-fix, role-init hits the
// hard GitLab refuse (exit 5), so this test's rc==0 assertion fails.
func TestRoleInitStampsGitLabSessionIdentity(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	const glEmail = "service_account_group_9619193_ab12cd@noreply.gitlab.example.com"
	gitLabRoster(t, glEmail)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "gl"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on a GitLab roster rc = %d, want 0 (must stamp the GitLab identity, not refuse); stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-gl")
	got := mustGit(t, target, "config", "--worktree", "--get", "user.email")
	if got != glEmail {
		t.Fatalf("worktree-scoped user.email = %q, want the GitLab address %q", got, glEmail)
	}
	if strings.Contains(got, "users.noreply.github.com") {
		t.Fatalf("stamped a GitHub noreply address (%q) on a GitLab commit identity — the #677 bug", got)
	}
	if !strings.HasPrefix(got, "service_account_group_") {
		t.Fatalf("stamped email %q is not the GitLab service-account shape", got)
	}
}

// TestRoleInitRefusesGitLabWithoutSessionEmail keeps the never-fall-back-to-GitHub
// invariant: a GitLab role with NO trusted session address configured is refused (exit 5),
// the refusal names ASSAY_GITLAB_SESSION_EMAILS, and it neither emits nor stamps a GitHub
// noreply address. Nothing is provisioned.
func TestRoleInitRefusesGitLabWithoutSessionEmail(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	gitLabRoster(t, "") // GitLab identity, but no ASSAY_GITLAB_SESSION_EMAILS

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "glno"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("role-init on a GitLab roster with no session email rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	if !contains(stderr, deskkit.EnvGitLabSessionEmails) {
		t.Fatalf("refusal must name %s; stderr: %s", deskkit.EnvGitLabSessionEmails, stderr)
	}
	if contains(stderr, "users.noreply.github.com") {
		t.Fatalf("refusal leaked the GitHub noreply shape for a GitLab identity; stderr: %s", stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was created despite the refusal; git calls: %v", gitCalls(*calls))
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-verify-desk-glno")); !os.IsNotExist(err) {
		t.Fatalf("target dir exists after a refusal: %v", err)
	}
}

// contains is a tiny substring helper local to this test file.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
