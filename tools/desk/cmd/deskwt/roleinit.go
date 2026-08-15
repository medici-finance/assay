package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// roleinit.go adds two verbs that provision and tear down a DESK ROLE's own git worktree in
// one idempotent call — the discrete component behind the verify-desk boot sequence.
//
// Hand-rolling `git worktree add` in the skill text kept re-introducing the same defects a
// live session had to fix by hand: a fixed relative path such as `../<role>-main` collides
// with an unrelated worktree of that name in a sibling checkout; a linked worktree cannot
// check out `main` (the primary holds it), so the preflight write-transport probe went RED
// for want of a landing ref; the commit identity was set to the App id instead of the bot
// USER id (#638); and nothing guarded against re-pointing a path that turned out to belong to
// a foreign repo. This verb bakes every one of those correctness properties in:
//
//   - a SESSION-SCOPED path under the sanctioned /private/tmp/tracker-* prefix (never a fixed
//     relative name that can collide with a sibling worktree);
//   - a UNIQUELY-NAMED branch (<role-loop>/<session>) that TRACKS origin/main, so the
//     worktree is immediately clean AND the preflight landing probe has a ref to push to;
//   - a worktree LOCK (cooperative half of the prune liveness guard);
//   - the role's App commit identity as a PER-WORKTREE config (bot USER id, #638), scoped via
//     extensions.worktreeConfig so it never bleeds into the primary checkout;
//   - an ORIGIN identity guard: an existing target whose origin is a different repo is
//     REFUSED, never re-pointed or reset (fail closed);
//   - idempotency: a valid existing worktree is reused (noop), not clobbered or errored.
//
// role-clean is the matching teardown: it UNLOCKS first (so the admin entry can be pruned),
// then applies the same tracked-clean / pushed guards deskwt remove uses.

// roleWTConfig binds a desk role to the worktree it provisions: the branch/loop name prefix.
// The App commit identity is NOT held here — it is resolved at run time from the roster
// (deskkit.RoleBotCommitIdentity), because the bot USER id is a real, deployment-specific
// account id that must not be baked into this publicly-staged tool. The identity shape is the
// SETTLED house fact — the bot USER id prefix, NOT the App id (#638 / the app-commit-identity
// memory note): a commit authored with the App id links to no account and shows
// author.login=null. RoleBotCommitIdentity builds the `<botUserID>+<slug>[bot]@…` prefix from
// the roster, preserving that shape while keeping the id itself in config.
type roleWTConfig struct {
	branchPrefix string // branch = <branchPrefix>/<session>, dir = tracker-<branchPrefix>-<session>
}

var roleWorktreeConfig = map[string]roleWTConfig{
	"verifier": {
		branchPrefix: "verify-desk",
	},
}

// roleWorktreeRoles returns the configured roles, sorted, for error messages.
func roleWorktreeRoles() string {
	roles := make([]string, 0, len(roleWorktreeConfig))
	for r := range roleWorktreeConfig {
		roles = append(roles, r)
	}
	// tiny, deterministic sort without importing sort for one call site
	for i := 1; i < len(roles); i++ {
		for j := i; j > 0 && roles[j-1] > roles[j]; j-- {
			roles[j-1], roles[j] = roles[j], roles[j-1]
		}
	}
	return strings.Join(roles, ", ")
}

// resolveSession returns the session id: the flag, else $DESK_SESSION, else
// $CLAUDE_SESSION_ID, else "" (which the caller refuses — a role worktree MUST be
// session-scoped, never a shared fixed name).
func resolveSession(flagVal string) string {
	if s := strings.TrimSpace(flagVal); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("DESK_SESSION")); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")); s != "" {
		return s
	}
	return ""
}

// roleInitParams is the validated, derived shape shared by role-init and role-clean so the
// two verbs cannot disagree about a role's path or branch.
type roleInitParams struct {
	role    string
	cfg     roleWTConfig
	session string
	branch  string
	target  string // <tmpBaseDir>/tracker-<branchPrefix>-<session>
}

// parseRoleParams validates --role/--session and derives the path + branch. It is the single
// place the naming convention lives.
func parseRoleParams(verb string, args []string) (roleInitParams, error) {
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	role := fs.String("role", "", "desk role to provision a worktree for (e.g. verifier)")
	session := fs.String("session", "", "session id (default $DESK_SESSION, then $CLAUDE_SESSION_ID)")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return roleInitParams{}, deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if len(positionals) != 0 {
		return roleInitParams{}, deskkit.Refused("refused: " + verb + " takes no positional args; use --role and --session")
	}
	cfg, ok := roleWorktreeConfig[*role]
	if !ok {
		return roleInitParams{}, deskkit.Refused("refused: --role must be one of: " + roleWorktreeRoles())
	}
	sess := resolveSession(*session)
	if !nameRe.MatchString(sess) || strings.Contains(sess, "..") {
		return roleInitParams{}, deskkit.Refused("refused: session id must be a single safe segment " +
			"(pass --session or set $DESK_SESSION / $CLAUDE_SESSION_ID; no slashes, no leading dash/dot, no '..')")
	}
	branch := cfg.branchPrefix + "/" + sess
	if !branchRe.MatchString(branch) || strings.Contains(branch, "..") {
		return roleInitParams{}, deskkit.Refused("refused: derived branch " + branch + " is not a plain branch name")
	}
	target := filepath.Join(tmpBaseDir, "tracker-"+cfg.branchPrefix+"-"+sess)
	return roleInitParams{role: strings.ToLower(*role), cfg: cfg, session: sess, branch: branch, target: target}, nil
}

// cmdRoleInit implements `deskwt role-init --role <role> [--session <s>]`.
func cmdRoleInit(args []string) (err error) {
	ac := &auditCtx{verb: "role-init"}
	defer func() { ac.finalize(err) }()

	p, perr := parseRoleParams("role-init", args)
	if perr != nil {
		return perr
	}

	// The App commit identity comes from the roster, not a source literal: the bot USER id is
	// deployment-specific. Refuse loudly rather than stamp an empty/unlinked identity.
	botName, botEmail, ok := deskkit.RoleBotCommitIdentity(p.role)
	if !ok {
		return deskkit.Refused("refused: role " + p.role + " has no bot commit identity in the roster — " +
			"pin it with a " + deskkit.EnvTrustedBotSlugs + " entry " + p.role +
			"=<app-slug>:<bot-user-id> (the bot USER id, from `gh api /users/<app-slug>[bot]`) in " +
			deskkit.ConfigHomePath())
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	repo, rerr := currentRepo(dir)
	if rerr != nil {
		return rerr
	}
	ac.repo = repo
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused("refused: origin " + repo + " is not in the desk-tools repo set")
	}

	guard, gErr := newPathGuard(dir)
	if gErr != nil {
		return gErr
	}
	rt, cerr := guard.check(p.target)
	if cerr != nil {
		return cerr
	}
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}

	// Idempotent reuse: an existing target must be a registered worktree of THIS repo.
	if _, statErr := os.Lstat(p.target); statErr == nil {
		if !set[rt] {
			return deskkit.Refused("refused: " + p.target + " exists but is not a registered worktree of this " +
				"repo — refusing to clobber a stray directory")
		}
		if ierr := assertSameRepo(rt, repo); ierr != nil {
			return ierr
		}
		if lErr := ensureLock(dir, p.target, p.cfg); lErr != nil {
			return lErr
		}
		if serr := setCommitIdentity(p.target, botName, botEmail); serr != nil {
			return serr
		}
		ac.successResult = deskkit.ResultNoop
		ac.detail = "reused role worktree " + p.target + " (branch " + p.branch + ", identity " + botEmail + ")"
		fmt.Println(p.target)
		return nil
	} else if !os.IsNotExist(statErr) {
		return deskkit.Unverifiable("cannot stat target "+p.target, statErr)
	}

	// Fresh create. origin/main must resolve to exactly one commit (same gates as `add`).
	if aerr := checkBaseUnambiguous(dir, "origin/main"); aerr != nil {
		return aerr
	}
	if _, verr := runGit(dir, "rev-parse", "--verify", "--quiet", "origin/main^{commit}"); verr != nil {
		return deskkit.Unverifiable("refused: origin/main does not resolve to a commit", verr)
	}

	// If the branch already exists (its worktree was removed but the branch left behind),
	// attach the worktree to it; otherwise create a new branch tracking origin/main. Either
	// way the worktree ends up on <branch>, which tracks origin/main.
	if _, brErr := runGit(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+p.branch); brErr == nil {
		if _, aerr := runGit(dir, "worktree", "add", p.target, p.branch); aerr != nil {
			return deskkit.Unverifiable("git worktree add (existing branch "+p.branch+") failed", aerr)
		}
	} else if _, aerr := runGit(dir, "worktree", "add", "--track", "-b", p.branch, p.target, "origin/main"); aerr != nil {
		return deskkit.Unverifiable("git worktree add failed", aerr)
	}

	// Positively verify it is registered before locking/stamping it.
	set, lerr = guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}
	if !set[resolvePath(p.target)] {
		return deskkit.Unverifiable("git worktree add reported success but "+p.target+" is not in `git worktree list`", nil)
	}
	if lErr := ensureLock(dir, p.target, p.cfg); lErr != nil {
		return lErr
	}
	if serr := setCommitIdentity(p.target, botName, botEmail); serr != nil {
		return serr
	}

	ac.detail = "provisioned role worktree " + p.target + " (branch " + p.branch + " tracking origin/main, identity " + botEmail + ")"
	fmt.Println(p.target)
	return nil
}

// cmdRoleClean implements `deskwt role-clean --role <role> [--session <s>]`: unlock then
// remove the role's worktree, with the same tracked-clean / pushed guards as `remove`.
func cmdRoleClean(args []string) (err error) {
	ac := &auditCtx{verb: "role-clean"}
	defer func() { ac.finalize(err) }()

	p, perr := parseRoleParams("role-clean", args)
	if perr != nil {
		return perr
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	guard, gErr := newPathGuard(dir)
	if gErr != nil {
		return gErr
	}
	rt, cerr := guard.check(p.target)
	if cerr != nil {
		return cerr
	}
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}
	if !set[rt] {
		// Nothing registered here. If the directory is also absent, that is a clean NOOP
		// (idempotent teardown); a present-but-unregistered directory is refused, never
		// blindly deleted.
		if _, statErr := os.Lstat(p.target); os.IsNotExist(statErr) {
			ac.successResult = deskkit.ResultNoop
			ac.detail = "no role worktree at " + p.target + " to clean"
			fmt.Println("noop: nothing to clean at " + p.target)
			return nil
		}
		return deskkit.Refused("refused: " + p.target + " is not a registered worktree of this repo")
	}

	repo, rerr := currentRepo(dir)
	if rerr != nil {
		return rerr
	}
	ac.repo = repo
	if ierr := assertSameRepo(rt, repo); ierr != nil {
		return ierr
	}

	// Tracked-clean + fully-pushed guards, mirroring `remove` — a role worktree holds no local
	// work of its own (Evidence lands straight on main), so these pass in the normal case and
	// protect against tearing down a worktree that unexpectedly carries uncommitted/unpushed work.
	dirtyOut, derr := dirtyTracked(rt)
	if derr != nil {
		return derr
	}
	if dirtyOut != "" {
		return deskkit.Refused("refused: role worktree has uncommitted TRACKED changes — commit or discard them first:\n" + dirtyOut)
	}
	branch, berr := runGit(rt, "rev-parse", "--abbrev-ref", "HEAD")
	if berr != nil {
		return deskkit.Unverifiable("cannot resolve the worktree's branch", berr)
	}
	if branch == "HEAD" || branch == "" {
		return deskkit.Refused("refused: role worktree is in detached HEAD (no upstream to prove pushed) — refusing to remove")
	}
	if _, uerr := runGit(rt, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); uerr != nil {
		return deskkit.Refused("refused: branch " + branch + " has no upstream (cannot prove its commits are pushed) — refusing to remove")
	}
	ahead, aerr := runGit(rt, "rev-list", "--count", "@{u}..HEAD")
	if aerr != nil {
		return deskkit.Unverifiable("cannot count unpushed commits", aerr)
	}
	if ahead != "0" {
		return deskkit.Refused("refused: branch " + branch + " has " + ahead + " unpushed commit(s) ahead of its upstream — refusing to remove")
	}

	// UNLOCK before removing: removeWorktreeDir does os.RemoveAll + `git worktree prune`, and
	// prune SKIPS a locked admin entry, which would leave the worktree still registered after
	// the directory is gone. Tolerate "not locked" (idempotent).
	_, _ = runGit(dir, "worktree", "unlock", p.target)

	if rmErr := removeWorktreeDir(guard, dir, rt); rmErr != nil {
		return rmErr
	}
	ac.detail = "cleaned role worktree " + p.target
	fmt.Println("removed " + p.target)
	return nil
}

// assertSameRepo is the ORIGIN identity guard: the worktree at rt must belong to the same
// repo the tool booted from. A mismatch is REFUSED — never re-pointed or reset — because a
// path that resolves to a foreign repo (e.g. a sibling checkout that happens to occupy the
// session-scoped name) must not be locked, stamped, or torn down by this tool.
func assertSameRepo(rt, bootRepo string) error {
	wtRepo, err := currentRepo(rt)
	if err != nil {
		return deskkit.Unverifiable("cannot read the worktree's repo identity at "+rt, err)
	}
	if wtRepo != bootRepo {
		return deskkit.Refused("refused: " + rt + " belongs to a different repo (" + wtRepo +
			", booted from " + bootRepo + ") — never re-pointed or reset")
	}
	return nil
}

// ensureLock locks the worktree with a role-scoped reason. Locking an already-locked worktree
// is git-noisy but harmless; the "already locked" case is tolerated so the verb is idempotent.
func ensureLock(dir, target string, cfg roleWTConfig) error {
	reason := cfg.branchPrefix + " live session (deskwt role-init)"
	out, err := runGit(dir, "worktree", "lock", "--reason", reason, target)
	if err != nil && !strings.Contains(strings.ToLower(out+err.Error()), "already locked") {
		return deskkit.Unverifiable("git worktree lock failed for "+target, err)
	}
	return nil
}

// setCommitIdentity stamps the role's App identity onto the worktree, SCOPED to the worktree
// via extensions.worktreeConfig so it never bleeds into the primary checkout. The email
// carries the bot USER id, not the App id (#638) — the whole point of the component.
func setCommitIdentity(target, botName, botEmail string) error {
	if _, err := runGit(target, "config", "extensions.worktreeConfig", "true"); err != nil {
		return deskkit.Unverifiable("cannot enable worktree-scoped config at "+target, err)
	}
	if _, err := runGit(target, "config", "--worktree", "user.name", botName); err != nil {
		return deskkit.Unverifiable("cannot set worktree user.name at "+target, err)
	}
	if _, err := runGit(target, "config", "--worktree", "user.email", botEmail); err != nil {
		return deskkit.Unverifiable("cannot set worktree user.email at "+target, err)
	}
	return nil
}
