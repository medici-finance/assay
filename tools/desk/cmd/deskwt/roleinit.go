package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
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
//   - the role's App CREDENTIAL HELPER as a per-worktree config too (#1309 item 7): the unscoped
//     helper chain is reset at worktree scope and one inline helper reading the role's 0600 token
//     file is added under the HOST-SCOPED key `credential.https://<origin-host>.helper` (#1374
//     security review — an unscoped helper hands the token to any host/protocol), so https
//     fetch/push to the origin authenticate as the role and never fall through to a sibling
//     role's leftover helper in shared config, while a foreign host or plaintext http gets
//     nothing — the token FILE chosen by the forge serving the repo (#1573: a GitLab repo reads
//     the role's provisioned PAT custody file and never the GitHub App minter) — followed by the
//     role's own PREFLIGHT, run against the provisioned worktree, so a red envelope is found here
//     and not at boot;
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
		// TIER TWO — see deskkit/helprequest.go.
		if deskkit.IsHelpRequest(perr) {
			return roleInitParams{}, deskkit.ErrHelpRequested
		}
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
		repo, err := gitcore.Open(p.repoRoot)
		if err != nil || !repo.InsideWorkTree() {
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

	// The commit identity is the ONE SHARED resolver every worktree-stamping tool uses
	// (deskkit.RoleWorktreeCommitIdentity): it derives the GitHub-vs-GitLab shape from the
	// role's forge-qualified roster entry — never a fixed shape, never the GitHub noreply
	// shape for a GitLab account (#677) — and returns a typed Refused (exit 5) naming the
	// roster key when the role has no derivable identity (#638). role-init, `deskwt add
	// --role` and deskdispatch's worktree-create all resolve through it, so one role can
	// never stamp three different identities.
	botName, botEmail, ierr := deskkit.RoleWorktreeCommitIdentity(p.role)
	if ierr != nil {
		return ierr
	}
	// credUser is the username the inline credential helper answers with: GitHub App
	// installation tokens authenticate as `x-access-token`; a GitLab PAT as `oauth2`. It is
	// read from the SAME roster entry the identity was, so the two cannot disagree.
	credUser := "x-access-token"
	if ident, bound := deskkit.EffectiveConfig().RoleBotIdentity(p.role); bound && ident.Forge == deskkit.ForgeGitLab {
		credUser = "oauth2"
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
		// Reuse re-wires the credential helper too: a reused worktree is exactly the one a
		// sibling role's stale helper has had time to pollute (#1309 item 7).
		if werr := wireRoleCredential(p.target, p.role, repo, credUser); werr != nil {
			return werr
		}
		ac.successResult = deskkit.ResultNoop
		ac.detail = "reused role worktree " + p.target + " (branch " + p.branch + ", identity " + botEmail + ")"
		fmt.Println(p.target)
		return roleInitPreflightRun(p, repo)
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
	roleInitRepo, rierr := gitcore.Open(dir)
	if rierr != nil {
		return deskkit.Unverifiable("refused: origin/main does not resolve to a commit", rierr)
	}
	if ok, verr := roleInitRepo.CommitVerifyQuiet("origin/main"); verr != nil || !ok {
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
	if brOK, brErr := roleInitRepo.CommitVerifyQuiet("refs/heads/" + p.branch); brErr == nil && brOK {
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
	if werr := wireRoleCredential(p.target, p.role, repo, credUser); werr != nil {
		return werr
	}

	ac.detail = "provisioned role worktree " + p.target + " (branch " + p.branch + " tracking origin/main, identity " + botEmail + ")"
	fmt.Println(p.target)
	return roleInitPreflightRun(p, repo)
}

// roleCredential is deskkit's forge-aware role-credential resolver (#1573): it resolves WHICH
// forge serves the repo before any token is touched, then reads that forge's custody — GitLab:
// the provisioned `gitlab-<role>.token` file, never minted or rotated, a missing/loose/empty file
// REFUSED (exit 5) naming it with no fall-through to the GitHub App minter; GitHub (or an
// unresolved forge, the historical default): the GitHub App token, minted or reused. role-init
// carries no forge branch of its own. It is a package var ONLY as a test seam: a fixture has no
// App credential to mint.
var roleCredential = deskkit.ResolveRoleCredential

// roleCredentialPath returns the PATH of the role's credential file for repo, as selected by the
// forge that serves it. originURL is the TARGET worktree's own origin remote — not the process's
// working directory — so a --repo-root provisioning resolves the repo it is actually wiring
// (ASSAY_REPO_FORGES first, then that remote's host). Only the PATH is returned; the token value
// is discarded here and never logged.
func roleCredentialPath(role, repo, originURL string) (string, error) {
	owner, name, _ := strings.Cut(repo, "/")
	cred, err := roleCredential(role, deskkit.ForgeRepo{Owner: owner, Name: name}, originURL)
	if err != nil {
		return "", err
	}
	return cred.Path, nil
}

// roleInitPreflight runs the role's envelope preflight and returns its one-line refusal, or nil
// when every check passes. Package var ONLY as a test seam; production is the real deskkit
// preflight — the same six checks the desk boot runs.
var roleInitPreflight = func(req deskkit.PreflightRequest) error { return req.Run().Err() }

// roleInitPreflightRun is the LAST step of role-init (#1309 item 7): having wired the identity
// and the credential helper it knows how to mint, the verb proves them by running the role's
// preflight against the provisioned worktree itself. Before this, a sibling-root role-init
// handed back a worktree whose first https fetch died with "could not read Username", and the
// desk found out at boot — after which that root's whole queue was invisible. The target path
// has already been printed (the worktree IS provisioned and idempotently reusable); a red
// preflight is exit 6, with the report on stderr, so a launcher stops rather than boots blind.
func roleInitPreflightRun(p roleInitParams, repo string) error {
	err := roleInitPreflight(deskkit.PreflightRequest{
		Role:    p.role,
		Root:    p.target,
		Repo:    repo,
		Landing: deskkit.Landing{Dir: p.target, Remote: "origin"},
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "deskwt: role-init preflight green for "+p.role+" at "+p.target)
	return nil
}

// wireRoleCredential writes the WORKTREE-SCOPED credential helper for the role's App token
// (#1309 item 7), so every https fetch/push this worktree makes to the ORIGIN's host
// authenticates as the role — never as whatever ambient keychain entry or sibling role's
// leftover helper the shared .git/config happens to carry.
//
// WHY THIS IS NOT ROUTED THROUGH `deskgit --as` (the sanctioned inline-helper replacement,
// README §"Authenticated transport"). `deskgit --as` authenticates ONE git child it spawns
// itself, with an ephemeral GIT_ASKPASS, and persists NOTHING in config. role-init's
// deliverable is the opposite: a provisioned worktree whose OWN later raw-`git` operations
// authenticate — starting with the write-transport probe in the preflight this verb runs next
// (deskkit.writeTransportProbe shells raw `git push --dry-run`), and every subsequent desk-role
// fetch/push from the worktree. Routing those through deskgit would mean rewriting the shared,
// forge-neutral preflight probe (and the GitLab custody arm) to call deskgit — out of this PR's
// scope. deskgit --as also refuses unless `--as <role>` equals the SESSION's own bound loop
// role, but role-init provisions ANY of the six roles (deskboot runs it per-role), so a session
// provisioning a role other than its own would be refused. Persisting a per-host helper is the
// only mechanism that satisfies both. Both constraints are structural, not effort.
//
// SECURITY — the key is SCOPED to the origin's https host (#1374 security review). The helper
// MUST NOT be installed under the unscoped `credential.helper` key: that key answers for EVERY
// host over EVERY protocol, and this helper ignores git's stdin request (it emits a fixed
// username/password), so an unscoped install hands the role's App token to any host — a foreign
// https host, or even plaintext http on the real host — that `git credential fill` is ever asked
// about. Instead the helper is added under `credential.https://<origin-host>.helper`, where the
// host is RESOLVED FROM THE ORIGIN remote (never a wildcard, never hardcoded — the tool is
// forge-neutral and serves GitLab too). git's per-URL matching then offers the token ONLY on
// https to that exact host; a foreign host or plaintext http matches no helper and gets nothing.
//
// Shape: the chain is still RESET at worktree scope (an empty unscoped `credential.helper`
// clears every helper accumulated from system/global/shared config — the shadowing that
// produced the "Invalid username or token" 401s — for ALL hosts), THEN the one inline helper is
// added under the host-scoped key. The inline helper reads the 0600 token file at auth time
// inside git's own shell; the token never appears in argv, in a URL, on stdout, or in the audit
// line — only its PATH does. Scoped via extensions.worktreeConfig (already on from
// setCommitIdentity), so the primary checkout's config is never mutated.
func wireRoleCredential(target, role, repo, username string) error {
	// The origin URL is read FIRST: it is the fallback input to the forge resolution that picks
	// which credential file to wire (#1573), and — below — the source of the scoping HOST
	// (effective URL, so an insteadOf rewrite is honoured, parsed with the scp-aware deskkit
	// parser rather than a hand-rolled split). A host we cannot resolve is could-not-check
	// (exit 6): we refuse to install a credential helper we cannot scope, rather than fall back
	// to the leaky unscoped key.
	originURL, oerr := runGit(target, "remote", "get-url", "origin")
	if oerr != nil {
		return deskkit.Unverifiable("cannot read the origin remote URL at "+target+" to scope the credential helper", oerr)
	}
	path, err := roleCredentialPath(role, repo, strings.TrimSpace(originURL))
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return deskkit.Unverifiable("the credential resolver returned no token path for the "+role+" role on "+repo, nil)
	}
	if strings.ContainsAny(path, "'\n") {
		return deskkit.Refused("refused: token path " + path + " cannot be quoted into a credential helper")
	}
	host, herr := deskkit.HostOfRemote(strings.TrimSpace(originURL))
	if herr != nil || strings.TrimSpace(host) == "" {
		// A hostless origin — a local filesystem path (a fixture, or a directory clone) — is
		// served by git's local transport, which consults NO credential helper, so there is no
		// https host to authenticate to and no token to wire. Skip (fail-safe): installing an
		// unscoped helper "just in case" is exactly the leak this fix removes, so the absence of a
		// host is a reason to wire NOTHING, never to fall back to the unscoped key. Still reset the
		// worktree chain so a stale sibling helper cannot answer from shared config.
		if _, rerr := runGit(target, "config", "--worktree", "--replace-all", "credential.helper", ""); rerr != nil {
			return deskkit.Unverifiable("cannot reset the worktree-scoped credential helper chain at "+target, rerr)
		}
		fmt.Fprintln(os.Stderr, "deskwt: origin at "+target+" has no https host (local transport) — no role credential helper wired")
		return nil
	}
	scopedKey := "credential.https://" + host + ".helper"
	helper := "!f(){ echo username=" + username + "; echo \"password=$(cat '" + path + "')\"; }; f"
	// Reset the whole (unscoped) chain first — this clears any stale sibling helper from shared
	// config for every host — then add the role helper ONLY under the host-scoped key.
	if _, err := runGit(target, "config", "--worktree", "--replace-all", "credential.helper", ""); err != nil {
		return deskkit.Unverifiable("cannot reset the worktree-scoped credential helper chain at "+target, err)
	}
	if _, err := runGit(target, "config", "--worktree", "--replace-all", scopedKey, helper); err != nil {
		return deskkit.Unverifiable("cannot set the host-scoped credential helper ("+scopedKey+") at "+target, err)
	}
	fmt.Fprintln(os.Stderr, "deskwt: host-scoped credential helper set for the "+role+" App (https://"+host+", token file "+path+")")
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
	roleRemoveRepo, rrerr := gitcore.Open(rt)
	if rrerr != nil {
		return deskkit.Unverifiable("cannot resolve the worktree's branch", rrerr)
	}
	branch, berr := roleRemoveRepo.AbbrevRefHEAD()
	if berr != nil {
		return deskkit.Unverifiable("cannot resolve the worktree's branch", berr)
	}
	if branch == "HEAD" || branch == "" {
		return deskkit.Refused("refused: role worktree is in detached HEAD (no upstream to prove pushed) — refusing to remove")
	}
	upstream, uerr := roleRemoveRepo.UpstreamRef()
	if uerr != nil {
		return deskkit.Refused("refused: branch " + branch + " has no upstream (cannot prove its commits are pushed) — refusing to remove")
	}
	ahead, aerr := roleRemoveRepo.AheadCount(upstream, "HEAD")
	if aerr != nil {
		return deskkit.Unverifiable("cannot count unpushed commits", aerr)
	}
	if ahead != 0 {
		return deskkit.Refused(fmt.Sprintf("refused: branch %s has %d unpushed commit(s) ahead of its upstream — refusing to remove", branch, ahead))
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

// clearCommitIdentity SHADOWS the shared checkout's user.name/user.email with an EMPTY
// worktree-scoped value (#1490), so a worktree created without a role can never inherit and
// silently commit under the shared checkout's identity — the misattribution this closes.
//
// The value must be SET to empty at worktree scope, not `--unset`: git config precedence
// resolves user.name from the highest scope that DEFINES it, so `--unset` at worktree scope
// falls straight through to the shared .git/config value (the very identity we are refusing
// to inherit). An empty worktree-scoped value is the highest scope AND defines the key, so it
// wins — and git rejects a commit with an empty author identity ("Author identity unknown"),
// which is the fail-closed outcome: a caller that meant to commit here must set an identity
// first (`deskwt add --role`, `role-init`, or a per-commit `git -c user.*`). Scoped via
// extensions.worktreeConfig so the shared checkout's config is never mutated.
func clearCommitIdentity(target string) error {
	if _, err := runGit(target, "config", "extensions.worktreeConfig", "true"); err != nil {
		return deskkit.Unverifiable("cannot enable worktree-scoped config at "+target, err)
	}
	if _, err := runGit(target, "config", "--worktree", "user.name", ""); err != nil {
		return deskkit.Unverifiable("cannot clear worktree user.name at "+target, err)
	}
	if _, err := runGit(target, "config", "--worktree", "user.email", ""); err != nil {
		return deskkit.Unverifiable("cannot clear worktree user.email at "+target, err)
	}
	return nil
}
