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

// roleWorktreeConfig maps every desk-role TOKEN — the App-role key the roster binds and
// desktoken mints under (its fixed role set: desk / worker / reviewer / verifier /
// issue-loop / intake-loop) — to the worktree it provisions. The KEY is the token role
// (RoleBotIdentity / RoleBotCommitIdentity are keyed on it, so `role-init verifier`
// resolves the verifier App's identity), and branchPrefix is the LOOP name deskboot boots
// (the-desk / worker-desk / pr-review-desk / verify-desk / intake-desk), which names the
// branch (<branchPrefix>/<session>) and the worktree leaf dir. Every loop deskboot can
// boot is represented, so `deskboot`'s "isolate first with role-init" step works for all
// of them, not only the verifier (#677); the mapping mirrors deskkit.LoopTokenRoles and the
// TestRoleWorktreeConfigMatchesLoopTokenRoles parity test guards it against drift.
//
// `intake-loop` is the one token role with no deskboot loop of its own (the intake window
// boots as intake-desk under the issue-loop App); it is still a role desktoken mints and the
// roster binds, so it provisions under its own prefix rather than being refused.
//
// The map is keyed on TOKEN roles only. A caller may name the role EITHER way — the token
// role or the loop name deskboot boots (`worker-desk`, `pr-review-desk`, a retired spelling
// the kill switch still honours) — and resolveRoleKey folds the loop spelling onto its token
// role through deskkit's own loop roster, so the two vocabularies cannot be spelled apart
// here. That fold is what makes deskboot's "isolate first" remediation runnable verbatim:
// deskboot knows its LOOP name, and a refusal that names a command the tool then refuses is
// a boot with no clean path out of the shared checkout.
var roleWorktreeConfig = map[string]roleWTConfig{
	"desk": {
		branchPrefix: "the-desk",
	},
	"worker": {
		branchPrefix: "worker-desk",
	},
	"reviewer": {
		branchPrefix: "pr-review-desk",
	},
	"verifier": {
		branchPrefix: "verify-desk",
	},
	"issue-loop": {
		branchPrefix: "intake-desk",
	},
	"intake-loop": {
		branchPrefix: "intake-loop",
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

// resolveRoleKey folds a caller's role spelling onto the roleWorktreeConfig key: a token
// role is returned as-is; a loop name (canonical or retired, per deskkit's kill-switch
// roster) resolves to the App role that loop acts under. ok=false is a refusal at the call
// site — never a default role, because provisioning under the wrong App identity is the
// failure this verb exists to prevent.
func resolveRoleKey(raw string) (key string, ok bool) {
	r := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := roleWorktreeConfig[r]; ok {
		return r, true
	}
	canonical, known := deskkit.CanonicalLoopName(r)
	if !known {
		return "", false
	}
	tok, bound := deskkit.TokenRoleForLoop(canonical)
	if !bound {
		return "", false
	}
	if _, ok := roleWorktreeConfig[tok]; !ok {
		return "", false
	}
	return tok, true
}

// roleSpellings lists every accepted role spelling — the token roles, then the loop names
// that fold onto them — for the refusal message, so an operator who typed the loop name
// deskboot printed can see it is accepted and the refusal is about something else.
func roleSpellings() string {
	loops := deskkit.LoopsWithTokenRole()
	return roleWorktreeRoles() + " (or a loop name: " + strings.Join(loops, ", ") + ")"
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
	role     string
	cfg      roleWTConfig
	session  string
	branch   string
	leaf     string // tracker-<branchPrefix>-<session> — the worktree's leaf dir name
	target   string // filled by the caller once the pathGuard is built: guard.worktreeTarget(leaf)
	repoRoot string // --repo-root: the checkout to provision from; "" = the working directory
	noFetch  bool   // --no-fetch: start from the local origin/main as-is (offline / fixture use)
}

// parseRoleParams validates the role (positional or --role), --session, --repo-root and
// --no-fetch, and derives the path + branch. It is the single place the naming convention
// lives.
//
// The role is accepted as ONE positional (`role-init worker-desk`) or as `--role`, and in
// either vocabulary — token role or loop name (resolveRoleKey). The positional form is what
// lets a launcher write `cd "$(deskwt role-init <role> --repo-root <path>)"` and what deskboot
// prints in its isolate-first remediation.
func parseRoleParams(verb string, args []string) (roleInitParams, error) {
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	role := fs.String("role", "", "desk role to provision a worktree for (token role or loop name; may also be given positionally)")
	session := fs.String("session", "", "session id (default $DESK_SESSION, then $CLAUDE_SESSION_ID)")
	repoRoot := fs.String("repo-root", "", "checkout of the repo to provision from (default: the working directory)")
	noFetch := fs.Bool("no-fetch", false, "start from the local origin/main as-is instead of fetching it fresh first")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return roleInitParams{}, deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	rawRole := strings.TrimSpace(*role)
	switch len(positionals) {
	case 0:
	case 1:
		// Both spellings may be given (a launcher passing the positional over a wrapper that
		// adds --role); they must resolve to the SAME role, in whichever vocabulary each uses.
		if rawRole != "" {
			a, aok := resolveRoleKey(rawRole)
			b, bok := resolveRoleKey(positionals[0])
			if aok && bok && a != b {
				return roleInitParams{}, deskkit.Refused("refused: role given twice and differently (positional " +
					positionals[0] + " → " + b + ", --role " + rawRole + " → " + a + ") — name it once")
			}
		}
		rawRole = positionals[0]
	default:
		return roleInitParams{}, deskkit.Refused("refused: " + verb + " takes at most one positional (the role); " +
			"got: " + strings.Join(positionals, " "))
	}
	if rawRole == "" {
		return roleInitParams{}, deskkit.Refused("refused: " + verb + " needs a role (positional or --role): one of " + roleSpellings())
	}
	key, ok := resolveRoleKey(rawRole)
	if !ok {
		return roleInitParams{}, deskkit.Refused("refused: role " + rawRole + " is not a desk role; must be one of: " + roleSpellings())
	}
	cfg := roleWorktreeConfig[key]
	root := strings.TrimSpace(*repoRoot)
	if root != "" {
		if strings.HasPrefix(root, "-") {
			return roleInitParams{}, deskkit.Refused("refused: --repo-root value looks like a flag: " + root)
		}
		abs, aerr := filepath.Abs(root)
		if aerr != nil {
			return roleInitParams{}, deskkit.Refused("refused: --repo-root " + root + " cannot be made absolute: " + aerr.Error())
		}
		root = abs
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
	// The target PATH is not built here: it depends on the sanctioned prefix chosen for the
	// host OS (guard.worktreeTarget), and the pathGuard is not available until the caller has
	// resolved the working directory. parseRoleParams owns only the naming convention — the
	// leaf dir name — and the caller fills p.target from the guard.
	leaf := "tracker-" + cfg.branchPrefix + "-" + sess
	return roleInitParams{role: key, cfg: cfg, session: sess, branch: branch, leaf: leaf, repoRoot: root, noFetch: *noFetch}, nil
}

// roleRepoDir resolves the checkout a role verb operates against: --repo-root when given
// (an explicit value is authoritative — no silent fall-back to the cwd), else the working
// directory. Either way the result must be inside a git worktree; anything else is
// unverifiable (exit 6), never "assume the cwd".
func roleRepoDir(p roleInitParams) (string, error) {
	if p.repoRoot != "" {
		if out, err := runGit(p.repoRoot, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
			return "", deskkit.Unverifiable("--repo-root "+p.repoRoot+" is not inside a git worktree", err)
		}
		return p.repoRoot, nil
	}
	dir, gerr := getwd()
	if gerr != nil {
		return "", deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	return dir, nil
}

// cmdRoleInit implements `deskwt role-init <role> [--repo-root <checkout>] [--session <s>] [--no-fetch]`
// (`--role <role>` is the same role spelled as a flag).
func cmdRoleInit(args []string) (err error) {
	ac := &auditCtx{verb: "role-init"}
	defer func() { ac.finalize(err) }()

	p, perr := parseRoleParams("role-init", args)
	if perr != nil {
		return perr
	}

	// The commit identity is derived from the roster entry's FORGE (the forge-qualified-identity
	// brief), never a fixed shape, and never the GitHub noreply shape for a GitLab account
	// (#677 — a GitHub-shaped email on a GitLab commit lands it under no GitLab identity).
	var botName, botEmail string
	if ident, bound := deskkit.EffectiveConfig().RoleBotIdentity(p.role); bound && ident.Forge == deskkit.ForgeGitLab {
		// GitLab: the service-account commit email embeds a group id and per-account suffix the
		// roster does not carry, so it is not CONSTRUCTIBLE. The established mechanism (#643) is
		// the two-identity model — the worktree commits under the trusted session / implementer
		// address the deployment lists in ASSAY_GITLAB_SESSION_EMAILS (the same allowlist the
		// commit-identity preflight accepts; a deployment committing AS the service account lists
		// that account's noreply address there). Read it from the trusted roster; refuse loudly
		// rather than fall back to the GitHub shape when none is configured.
		name, email, ok := deskkit.RoleGitLabCommitIdentity(p.role)
		if !ok {
			return deskkit.Refused("refused: role " + p.role + " is a GitLab identity (" + ident.Slug + "); its " +
				"service-account commit email (service_account_group_<group-id>_<suffix>@noreply.<host>) embeds a " +
				"group id and per-account suffix the roster does not carry, so it cannot be constructed — and it " +
				"must NOT fall back to the GitHub noreply shape. Configure the trusted GitLab session / implementer " +
				"commit address in " + deskkit.EnvGitLabSessionEmails + " (the two-identity mechanism; to commit AS " +
				"the service account, list its provisioned noreply address there), in " + deskkit.ConfigHomePath())
		}
		botName, botEmail = name, email
	} else {
		// GitHub: the App commit identity comes from the roster, not a source literal — the bot
		// USER id is deployment-specific. Refuse loudly rather than stamp an empty/unlinked identity.
		name, email, ok := deskkit.RoleBotCommitIdentity(p.role)
		if !ok {
			return deskkit.Refused("refused: role " + p.role + " has no bot commit identity in the roster — " +
				"pin it with a " + deskkit.EnvTrustedBotSlugs + " entry " + p.role +
				"=<app-slug>:<bot-user-id> (the bot USER id, from `gh api /users/<app-slug>[bot]`) in " +
				deskkit.ConfigHomePath())
		}
		botName, botEmail = name, email
	}

	dir, derr := roleRepoDir(p)
	if derr != nil {
		return derr
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
	// Build the target under the OS-portable sanctioned prefix now the guard is available.
	// It is ABSOLUTE by construction (both sanctioned prefixes are), and it is the ONE line
	// this verb prints on stdout on success — the machine-readable contract a launcher
	// relies on: `cd "$(deskwt role-init <role> --repo-root <path>)"`.
	p.target = guard.worktreeTarget(p.leaf)
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
		if lErr := ensureLock(dir, p.target, p.session, p.cfg); lErr != nil {
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

	// Fresh create. The base is a FRESH origin/main: a role worktree cut from a stale
	// remote-tracking ref starts the session behind main and every first write needs a
	// merge-main before it can land. `git fetch origin main` updates refs/remotes/origin/main
	// (the remote's configured refspec covers it) and touches nothing else in the checkout —
	// no index, no config, no working tree. A failed fetch is could-not-check (exit 6): the
	// verb does not know whether origin/main is current, so it does not pretend it is.
	// --no-fetch is the explicit opt-out for an offline checkout or a fixture whose origin is
	// not reachable; it is never the default.
	if !p.noFetch {
		if _, ferr := runGit(dir, "fetch", "--no-tags", "origin", "main"); ferr != nil {
			return deskkit.Unverifiable("cannot fetch origin/main for "+dir+" — a role worktree starts from a "+
				"FRESH origin/main; fix the fetch, or pass --no-fetch to cut it from the local origin/main as-is", ferr)
		}
	}
	// origin/main must resolve to exactly one commit (same gates as `add`).
	if aerr := checkBaseUnambiguous(dir, "origin/main"); aerr != nil {
		return aerr
	}
	if _, verr := runGit(dir, "rev-parse", "--verify", "--quiet", "origin/main^{commit}"); verr != nil {
		return deskkit.Unverifiable("refused: origin/main does not resolve to a commit", verr)
	}

	// Ensure the sanctioned parent prefix exists (the `.claude/worktrees/` prefix may not,
	// and `git worktree add` creates only the leaf, not missing parents). Touches only the
	// sanctioned parent, never the leaf checked never-to-clobber above.
	if merr := os.MkdirAll(filepath.Dir(p.target), 0o755); merr != nil {
		return deskkit.Unverifiable("cannot create the sanctioned worktree parent dir "+filepath.Dir(p.target), merr)
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
	if lErr := ensureLock(dir, p.target, p.session, p.cfg); lErr != nil {
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

	dir, derr := roleRepoDir(p)
	if derr != nil {
		return derr
	}
	guard, gErr := newPathGuard(dir)
	if gErr != nil {
		return gErr
	}
	// Build the target under the OS-portable sanctioned prefix now the guard is available
	// (must match the prefix role-init created it under).
	p.target = guard.worktreeTarget(p.leaf)
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
//
// The reason carries a `session=<id>` STAMP, and that stamp is load-bearing rather than
// decorative: it is the only thing that lets a later sweep attribute the lock to a session
// and so ever RETIRE it. Without it a lock outlives its session with no way to tell a live
// session's lock from a dead one's, which is precisely how the locked population grows
// without bound (see lockreclaim.go, and `deskwt prune --reclaim-stale-locks`). The session
// id is already validated as a single safe segment, so the stamp is one whitespace-free token.
func ensureLock(dir, target, session string, cfg roleWTConfig) error {
	reason := cfg.branchPrefix + " live session (deskwt role-init session=" + session + ")"
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
