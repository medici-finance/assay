package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// getwd is the seam for the tool's working directory (the worktree it runs in).
// Production uses os.Getwd; tests point it at a scratch worktree fixture without
// os.Chdir (which would race parallel processes).
var getwd = os.Getwd

// tmpBaseDir is the parent directory of the `/private/tmp/tracker-*` sanctioned prefix.
// It is a package var ONLY so white-box tests can point it at a temp
// dir; production keeps the compiled-in `/private/tmp`. It is deliberately NOT wired to
// any env var or flag — an operator-relocatable prefix would defeat the point of a
// fixed allowlist.
var tmpBaseDir = "/private/tmp"

var (
	// nameRe bounds the worktree <name>: a single path segment, no slashes, no leading
	// dash (arg-injection) or dot. The worktree dir is `tracker-<name>`.
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// branchRe / refRe bound the --branch and --base VALUES so a constructed git argv can
	// never smuggle an option (no leading dash) — "constructed argv only".
	branchRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	refRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
)

// auditCtx accumulates the fields for the ONE audit line every invocation emits.
// finalize is deferred so exactly one line is written no matter which branch
// returns. deskwt has no PR / head, so those stay null in the schema.
type auditCtx struct {
	verb          string
	repo          string
	detail        string
	successResult string // ResultOK unless a noop set it to ResultNoop
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "deskwt",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	if err == nil {
		result := a.successResult
		if result == "" {
			result = deskkit.ResultOK
		}
		a.log(result, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		result = deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// pathGuard holds the RESOLVED (EvalSymlinks) sanctioned locations for one invocation,
// computed once from the shared git-common-dir.
type pathGuard struct {
	sharedCheckout string // resolved main-checkout root (git-common-dir's parent) — identity refusal
	commonDir      string // resolved git-common-dir itself — where the per-worktree admin dirs live
	tmpDir         string // resolved tmpBaseDir (parent of the tracker-* prefix)
	worktreesDir   string // resolved <repo-root>/.claude/worktrees
}

// newPathGuard resolves the sanctioned prefixes from the shared git-common-dir. Being
// outside a git repo (git-common-dir unresolvable) is unverifiable (exit 6), never a
// silent "assume anywhere is fine".
func newPathGuard(dir string) (*pathGuard, error) {
	commonDir, err := runGit(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || commonDir == "" {
		return nil, deskkit.Unverifiable("cannot resolve the shared git-common-dir (are we in a git repo?)", err)
	}
	root := filepath.Dir(commonDir)
	return &pathGuard{
		sharedCheckout: resolvePath(root),
		commonDir:      resolvePath(commonDir),
		tmpDir:         resolvePath(tmpBaseDir),
		worktreesDir:   resolvePath(filepath.Join(root, ".claude", "worktrees")),
	}, nil
}

// check enforces the path allowlist on a candidate path: it refuses the shared checkout BY IDENTITY,
// then requires the RESOLVED path to sit under a sanctioned prefix. It returns the
// resolved path for the caller to use as the git working dir.
func (g *pathGuard) check(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", deskkit.Unverifiable("cannot make path absolute: "+path, err)
	}
	rt := resolvePath(abs)
	if rt == g.sharedCheckout {
		return "", deskkit.Refused("refused: " + path + " is the shared checkout (" + g.sharedCheckout +
			") — refused by identity, never removed")
	}
	if !g.allowed(rt) {
		return "", deskkit.Refused("refused: " + path + " resolves to " + rt +
			", outside the sanctioned worktree prefixes (/private/tmp/tracker-* or <repo-root>/.claude/worktrees/)")
	}
	return rt, nil
}

// allowed reports whether a RESOLVED path is a sanctioned worktree location: a direct
// `tracker-*` child of the resolved tmp dir, or strictly under the resolved worktrees dir.
func (g *pathGuard) allowed(rt string) bool {
	if filepath.Dir(rt) == g.tmpDir && strings.HasPrefix(filepath.Base(rt), "tracker-") {
		return true
	}
	if strings.HasPrefix(rt, g.worktreesDir+string(filepath.Separator)) {
		return true
	}
	return false
}

// worktreePaths returns the set of registered worktree roots (resolved) for the repo
// rooted at dir. A git error is propagated so callers fail CLOSED rather than
// treat an unreadable list as "not registered" / "already gone".
func (g *pathGuard) worktreePaths(dir string) (map[string]bool, error) {
	roots, err := g.worktreeRoots(dir)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(roots))
	for _, rt := range roots {
		set[rt] = true
	}
	return set, nil
}

// worktreeRoots returns the ORDERED list of registered worktree roots (resolved) for the
// repo rooted at dir, preserving `git worktree list` order (main checkout first). A git
// error is propagated so callers fail CLOSED rather than treat an unreadable list
// as "no worktrees".
func (g *pathGuard) worktreeRoots(dir string) ([]string, error) {
	out, err := runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read `git worktree list`", err)
	}
	var roots []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			wt := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			roots = append(roots, resolvePath(wt))
		}
	}
	return roots, nil
}

// lockedWorktrees returns resolved-path → lock-reason for every LOCKED registered worktree
// under the repo rooted at dir (empty reason = locked with no `--reason`). A locked worktree
// is one git will REFUSE to remove or prune: `git worktree list --porcelain` emits a `locked`
// line — bare, or `locked <reason>` — inside that worktree's block. prune MUST consult this
// BEFORE the destructive step, otherwise it deletes the working directory and then cannot
// deregister the now-dangling admin entry (git honours the lock), leaving the worktree
// half-destroyed — directory gone, registration stranded (issue #264). Parsing is line-based
// over the porcelain block stream: a `worktree ` line opens a block, a `locked` line inside it
// marks the current path locked. Path resolution mirrors worktreeRoots (resolvePath) so keys
// compare equal to the roots the caller iterates. A git error propagates so callers fail
// CLOSED (treat unknown lock state as unsafe), never as "nothing is locked".
func (g *pathGuard) lockedWorktrees(dir string) (map[string]string, error) {
	out, err := runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read `git worktree list` for lock state", err)
	}
	locked := make(map[string]string)
	var cur string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = resolvePath(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
		case line == "locked" || strings.HasPrefix(line, "locked "):
			if cur != "" {
				locked[cur] = strings.TrimSpace(strings.TrimPrefix(line, "locked"))
			}
		}
	}
	return locked, nil
}

// dirtyTracked returns the porcelain listing of a worktree's uncommitted TRACKED changes
// (empty string = clean). Untracked build artifacts (node_modules, build/, dist/) are
// deliberately excluded (--untracked-files=no) — a check that fired on every real worktree
// would kill the verb. This is the SINGLE implementation of the tracked-clean gate, shared
// by `remove` and `prune` so neither can drift from the other.
func dirtyTracked(rt string) (string, error) {
	out, err := runGit(rt, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", deskkit.Unverifiable("cannot check the worktree's tracked status", err)
	}
	return out, nil
}

// removeWorktreeDir deletes a proven-safe worktree directory and drops its now-dangling
// admin entry, then positively verifies it is gone. The delete is a plain recursive
// remove — there is NO --force and no code path that overrides a failed guard. Callers MUST
// have already passed the path/identity + tracked-clean + pushed/merged gates; this helper
// performs the destructive step only, and is the SINGLE implementation shared by `remove`
// and `prune`.
func removeWorktreeDir(guard *pathGuard, dir, rt string) error {
	if rerr := os.RemoveAll(rt); rerr != nil {
		return deskkit.Unverifiable("failed to remove the worktree directory "+rt, rerr)
	}
	if _, prErr := runGit(dir, "worktree", "prune"); prErr != nil {
		return deskkit.Unverifiable("git worktree prune failed after removing "+rt, prErr)
	}
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}
	if set[rt] {
		return deskkit.Unverifiable("worktree removed from disk but "+rt+" is still registered after prune", nil)
	}
	return nil
}

// cmdAdd implements `deskwt add <name> [--branch B] [--base origin/main]`: create a
// worktree at `<tmpBaseDir>/tracker-<name>` on a NEW branch that TRACKS the base (so a
// fresh worktree is immediately clean & up-to-date, satisfying remove's upstream check).
func cmdAdd(args []string) (err error) {
	ac := &auditCtx{verb: "add"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	branch := fs.String("branch", "", "new branch name for the worktree (default: the worktree name)")
	base := fs.String("base", "origin/main", "start-point ref for the new worktree")
	// Usage puts <name> first, but Go's flag stops at the first positional;
	// parse in a loop so flags may appear before OR after the name (a stray/unknown flag
	// still errors → refused).
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if len(positionals) != 1 {
		return deskkit.Refused("refused: add takes exactly one <name> argument")
	}
	name := positionals[0]
	if !nameRe.MatchString(name) || strings.Contains(name, "..") {
		return deskkit.Refused("refused: <name> must be a single safe segment (no slashes, no leading dash/dot, no '..')")
	}
	br := *branch
	if br == "" {
		br = name
	}
	if !branchRe.MatchString(br) || strings.Contains(br, "..") {
		return deskkit.Refused("refused: --branch must be a plain branch name (no leading dash, no '..')")
	}
	if !refRe.MatchString(*base) || strings.Contains(*base, "..") {
		return deskkit.Refused("refused: --base must be a plain ref (no leading dash, no '..')")
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

	// --base must resolve to EXACTLY ONE ref. An ambiguous short name is could-not-check,
	// not a coin flip: git resolves it at exit 0 with only a stderr warning, so this must
	// be checked BEFORE rev-parse, whose --quiet swallows that warning entirely
	// (see ambiguousbase.go; docs/three-state-instrument-rule.md sub-rule 1).
	if aerr := checkBaseUnambiguous(dir, *base); aerr != nil {
		return aerr
	}
	// --base must resolve to a commit, else exit 6.
	if _, verr := runGit(dir, "rev-parse", "--verify", "--quiet", *base+"^{commit}"); verr != nil {
		return deskkit.Unverifiable("refused: --base "+*base+" does not resolve to a commit", verr)
	}

	guard, perr := newPathGuard(dir)
	if perr != nil {
		return perr
	}
	target := filepath.Join(tmpBaseDir, "tracker-"+name)
	if _, cerr := guard.check(target); cerr != nil {
		return cerr
	}
	// Never reuse or clobber an existing target.
	if _, serr := os.Lstat(target); serr == nil {
		return deskkit.Refused("refused: target already exists (never clobbered): " + target)
	} else if !os.IsNotExist(serr) {
		return deskkit.Unverifiable("cannot stat target "+target, serr)
	}

	// Local-only verb: NO AllowWrite (deskkit/ratelimit.go "Verb classes"). Constructed
	// argv only; no caller flag reaches git, and no --force exists.
	if _, aerr := runGit(dir, "worktree", "add", "--track", "-b", br, target, *base); aerr != nil {
		return deskkit.Unverifiable("git worktree add failed", aerr)
	}
	// Positively verify the worktree now exists (prove success, don't assume it).
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}
	if !set[resolvePath(target)] {
		return deskkit.Unverifiable("git worktree add reported success but "+target+" is not in `git worktree list`", nil)
	}
	ac.detail = "added " + target + " (branch " + br + " tracking " + *base + ")"
	fmt.Println(target)
	return nil
}

// cmdRemove implements `deskwt remove <path>`: refuse unless the RESOLVED path is a
// sanctioned, registered worktree that is clean (tracked), fully pushed, and NOT the
// shared checkout; then `git worktree remove` + prune. There is no --force flag.
func cmdRemove(args []string) (err error) {
	ac := &auditCtx{verb: "remove"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	// No flags are defined: a stray flag (e.g. --force) makes Parse error → refused.
	// There is NO force / override flag anywhere in this tool.
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("refused: remove takes no flags (there is no --force): " + perr.Error())
	}
	if len(positionals) != 1 {
		return deskkit.Refused("refused: remove takes exactly one <path> argument")
	}
	path := positionals[0]

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}

	guard, perr := newPathGuard(dir)
	if perr != nil {
		return perr
	}
	// Path safety FIRST — identity (shared checkout) then the RESOLVED-prefix match —
	// BEFORE registration/dirty/upstream, so a symlink resolving outside the prefix is
	// caught by the resolved-path logic itself, not incidentally by "not registered".
	rt, cerr := guard.check(path)
	if cerr != nil {
		return cerr
	}
	// Must be a registered worktree of this repo.
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return lerr
	}
	if !set[rt] {
		return deskkit.Refused("refused: " + path + " is not a registered worktree (`git worktree list`)")
	}
	// Dirty TRACKED tree blocks removal (the uncommitted-work guard).
	// Untracked build artifacts (node_modules, build/, dist/) do NOT block — a
	// refusal that fired on every real worktree would kill the verb.
	dirtyOut, derr := dirtyTracked(rt)
	if derr != nil {
		return derr
	}
	if dirtyOut != "" {
		return deskkit.Refused("refused: worktree has uncommitted TRACKED changes — commit or discard them first:\n" + dirtyOut)
	}
	// Upstream + unpushed-commits guard: a detached HEAD or a branch with NO upstream is
	// refused (its commits cannot be proven pushed); non-empty @{u}..HEAD is refused.
	branch, berr := runGit(rt, "rev-parse", "--abbrev-ref", "HEAD")
	if berr != nil {
		return deskkit.Unverifiable("cannot resolve the worktree's branch", berr)
	}
	if branch == "HEAD" || branch == "" {
		return deskkit.Refused("refused: worktree is in detached HEAD (no upstream to prove pushed) — refusing to remove")
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

	// Local-only verb: NO AllowWrite and NO --force anywhere. git's own `worktree remove` refuses whenever untracked files are
	// present — and EVERY real worktree carries build artifacts (node_modules, build/,
	// dist/) — so relying on it would force a --force, which the weakest-verb rule forbids.
	// Instead the tool's OWN precise guards above are the safety gate: the path is proven
	// to be a sanctioned, registered worktree that is tracked-clean and fully pushed, so
	// the only thing lost is untracked artifacts. removeWorktreeDir does the plain recursive
	// remove + prune + positive-gone verification — the SAME primitive `prune` uses;
	// there is no code path that overrides a failed guard.
	if rmErr := removeWorktreeDir(guard, dir, rt); rmErr != nil {
		return rmErr
	}
	ac.repo = repoOrEmpty(dir)
	ac.detail = "removed " + rt
	fmt.Println("removed " + path)
	return nil
}

// parseInterspersed parses fs allowing flags to appear before OR after positional
// arguments (Go's flag package otherwise stops at the first positional). It returns the
// collected positionals; an unknown/malformed flag surfaces as the Parse error.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	return positionals, nil
}

// resolvePath returns p with symlinks resolved on its longest EXISTING ancestor and the
// non-existent tail preserved and cleaned. It is the load-bearing path primitive: the
// prefix/identity checks compare RESOLVED paths, so a symlink pointing outside a
// sanctioned prefix cannot masquerade as inside it. On a path whose ancestors are all
// unresolvable it returns the cleaned input (still subject to the prefix/identity check,
// which refuses it).
func resolvePath(p string) string {
	p = filepath.Clean(p)
	cur := p
	var tail []string
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			full := resolved
			for i := len(tail) - 1; i >= 0; i-- {
				full = filepath.Join(full, tail[i])
			}
			return filepath.Clean(full)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}

// currentRepo verifies dir is inside a git worktree and returns its origin repo in
// owner/name form. Any state it cannot positively verify is unverifiable (exit 6).
func currentRepo(dir string) (string, error) {
	if out, err := runGit(dir, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
		return "", deskkit.Unverifiable("not inside a git worktree", err)
	}
	originURL, oerr := runGit(dir, "config", "--get", "remote.origin.url")
	if oerr != nil {
		return "", deskkit.Unverifiable("cannot read remote.origin.url", oerr)
	}
	repo, rerr := parseRepo(originURL)
	if rerr != nil {
		return "", deskkit.Unverifiable("cannot parse origin repo from "+originURL, rerr)
	}
	return repo, nil
}

// repoOrEmpty is a best-effort repo lookup for the audit line on the remove path (where
// repo is informational, not a gate); a lookup miss yields "" rather than a failure.
func repoOrEmpty(dir string) string {
	repo, err := currentRepo(dir)
	if err != nil {
		return ""
	}
	return repo
}

// parseRepo extracts owner/name from an https, ssh, or scp-style git remote URL.
func parseRepo(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	u = strings.TrimSuffix(u, ".git")
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return normRepoPath(rest[slash+1:])
		}
		return "", fmt.Errorf("no path in url %q", raw)
	}
	if at := strings.Index(u, "@"); at >= 0 && strings.Contains(u, ":") {
		// scp-like: [user@]host:owner/repo
		colon := strings.Index(u, ":")
		return normRepoPath(u[colon+1:])
	}
	return normRepoPath(u)
}

func normRepoPath(p string) (string, error) {
	p = strings.Trim(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("cannot parse owner/repo from %q", p)
	}
	owner, repo := parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", fmt.Errorf("empty owner/repo in %q", p)
	}
	return owner + "/" + repo, nil
}
