package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every git and gh invocation flows.
// Production binds it to exec.Command; tests wrap it to RECORD every argv (so the
// "no --force in any git argv / --draft always present" assertions are checked on the
// real constructed argv) while still delegating to a real process. Nothing else in
// this package constructs commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// publicRepoGateFn is the seam for deskkit.PublicRepoGate — tests set it to a no-op
// stub so they don't need a real GitHub API connection. Production uses the real gate.
var publicRepoGateFn = deskkit.PublicRepoGate

// productionGateFn records what publicRepoGateFn was bound to at init, before any test
// could replace it. TestGateSeamIsRealInProduction compares it against
// deskkit.PublicRepoGate, so a seam that was never wired to the real gate — or was
// quietly rebound — fails loudly instead of leaving every gate test vacuous.
var productionGateFn = publicRepoGateFn

// ghToken holds the worker App installation token value set by mintWorkerToken. When
// empty, gh calls use the ambient gh identity (example-org fallback). When set, every gh
// invocation adds GH_TOKEN to the command environment so the call authenticates as
// the worker App.
var ghToken string

// requireWorkerAuth is set true by cmdCreate/cmdUpdate for the duration of a run whose
// --as-app flag is on (the default) — i.e. whenever the caller INTENDS gh to
// authenticate as the worker App. Unlike deskreply (#563), deskpr has a real,
// documented ambient-identity fallback (--as-app=false, "the example-org fallback"), so
// gh() cannot unconditionally refuse an unset ghToken the way deskreply's does — that
// would break --as-app=false outright. This flag scopes the fail-closed guard to the
// as-app path only: when true, gh() must never run without a minted token even if a
// future code change forgets to mint first; when false, an unset ghToken is the
// intended, ambient-identity behavior.
var requireWorkerAuth bool

// runCmd executes name+args in dir and returns trimmed stdout. Commands are ALWAYS
// built from an explicit argv slice — never a shell string and never a caller-supplied
// git flag. Callers pass literal verbs plus values derived from git
// state, so no external input can inject a git option.
func runCmd(dir, name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
	cmd.Dir = dir
	if name == "gh" && ghToken != "" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+ghToken)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout := strings.TrimSpace(out.String())
	if err != nil {
		return stdout, fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "),
			err, strings.TrimSpace(errb.String()))
	}
	return stdout, nil
}

// git runs a git subcommand in dir. The argv is fixed by the caller; deskpr never
// forwards a caller flag into it, so a `--force` can never reach git through this path.
func git(dir string, args ...string) (string, error) {
	return runCmd(dir, "git", args...)
}

// gh runs a gh subcommand in dir. On the --as-app path (requireWorkerAuth == true) deskpr
// ALWAYS authenticates as the worker App via the token set by mintWorkerToken; it refuses
// outright (rather than silently falling through to whatever gh identity happens to be
// ambient/active in this shell — e.g. a stale or invalid gh-CLI keyring account) if
// ghToken is unset, so a gh call can never run un-authenticated-as-worker even if a future
// code path forgets to mint first (mirrors #563's deskreply hardening for the identical
// #562 shape). On the --as-app=false path, an unset ghToken is the intended, documented
// ambient-identity fallback, so the guard does not apply there.
func gh(dir string, args ...string) (string, error) {
	if requireWorkerAuth && ghToken == "" {
		return "", fmt.Errorf("refusing to run gh without a minted worker token — deskpr's --as-app path never falls back to the ambient gh identity/keyring")
	}
	return runCmd(dir, "gh", args...)
}

// mintWorkerToken mints or reuses the installation token for the App role THIS session
// acts under and sets it as ghToken, so every subsequent gh invocation (list, create,
// view) authenticates as that App. It resolves the role from the session loop identity via
// deskkit.SessionTokenRole and calls `desktoken <role> --repo <repo>`; the WORKER App is
// the default, kept only when no loop is set or the loop carries no App role (see below).
//
// WHY THE ROLE IS RESOLVED, NOT HARDCODED (#396). deskpr create/update/edit all land a PR
// authored by the App whose token this mints. A verify-desk session
// (DESK_LOOP=verify-desk, which SessionTokenRole maps to the verifier role) opens Evidence
// PRs whose branch commits deskevidence already authored as the VERIFIER via the Contents
// API. Hardcoding the worker token here authored the PR under a DIFFERENT App than its own
// commits: on this repo's PR-required main that misattributes verification landings to the
// worker role and defeats any workflow keyed on "PR author is the verifier App". Resolving
// the role keeps PR author and commit author the same App. The name is kept for a
// body-only diff, but the function is no longer worker-only.
//
// WORKER IS THE DEFAULT, NEVER A GUESS. When no loop is set, or the loop carries no App
// role, SessionTokenRole returns an error and this falls back to the worker App — the
// historical behavior, and the right one for an unbadged dispatch — but SAYS SO on
// deskprStderr rather than adopting the default silently, so the fallback is auditable and
// not mistaken for a resolved role identity. It never guesses a NON-worker role: a
// non-worker role is only ever adopted from an explicit, recognised loop identity. (In
// practice deskpr's main() already refuses via RequireLoopIdentity before any subcommand
// runs when the loop is unset or unrecognised, so the fallback is a defensive floor.)
//
// --repo is not optional here (#565, mirroring #563/#562): each App is installed on more
// than one account (e.g. example-org AND medici-finance), and desktoken defaults the owner
// to "example-org" whenever --repo is absent. Without this flag, deskpr running against a
// medici-finance/* repo would silently mint a token for the WRONG installation — one with
// no access to the target repo — and the subsequent `gh pr create`/`gh pr view` would fail
// with a GitHub API "Could not resolve to a Repository" error, surfacing as deskpr's exit
// 6. Passing the caller's own already-verified repo (deskpr only ever calls this with the
// repo it just confirmed via preflight() matches this worktree's origin) makes resolution
// deterministic regardless of which worktree/remote deskpr happens to run in. Returns an
// error if desktoken is not available or the token file is unreadable.
func mintWorkerToken(repo string) error {
	// Resolve the App role this session acts under from its loop identity; fall back to
	// the worker App, and say so, when there is no App role to resolve (#396).
	role := "worker"
	if r, _, rerr := deskkit.SessionTokenRole("deskpr"); rerr == nil {
		role = r
	} else {
		fmt.Fprintf(deskprStderr, "deskpr: no App role resolved for this session — defaulting to the worker App token (%v)\n", rerr)
	}
	// Run desktoken <role> --repo <repo>; it prints the path to the cached token file.
	out, err := runCmd("", "desktoken", role, "--repo", repo)
	if err != nil {
		return fmt.Errorf("desktoken %s --repo %s: %w", role, repo, err)
	}
	tokenPath := strings.TrimSpace(out)
	b, rerr := os.ReadFile(tokenPath)
	if rerr != nil {
		return fmt.Errorf("read %s token from %s: %w", role, tokenPath, rerr)
	}
	ghToken = strings.TrimSpace(string(b))
	if ghToken == "" {
		return fmt.Errorf("%s token at %s is empty", role, tokenPath)
	}
	return nil
}
