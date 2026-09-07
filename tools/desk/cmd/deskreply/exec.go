package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every child process this tool starts flows.
// Production binds it to exec.Command; tests wrap it to RECORD every argv so the "no git
// push on any path" assertion runs against the real constructed argv. Nothing else in this
// package constructs commands, so there is exactly one place argv is built.
//
// SINCE THE FORGE MIGRATION the only binaries that reach it are `git` (read-only worktree
// facts, plus the one worktree-scoped config write recordWorkpadID makes) and `desktoken`
// (the identity layer). Every forge read and write goes through the resolved deskkit.Forge,
// so there is no forge CLI to launch and no ambient CLI identity to fall back to.
var execCommand = exec.Command

// publicRepoGateFn is the seam for deskkit.PublicRepoGate — tests set it to a no-op
// stub so they don't need a real GitHub API connection. Production uses the real gate.
var publicRepoGateFn = deskkit.PublicRepoGate

// productionGateFn records what publicRepoGateFn was bound to at init, before any test
// could replace it. TestGateSeamIsRealInProduction compares it against
// deskkit.PublicRepoGate, so a seam that was never wired to the real gate — or was
// quietly rebound — fails loudly instead of leaving every gate test vacuous.
var productionGateFn = publicRepoGateFn

// ghToken holds the worker App installation token value set by mintWorkerToken. deskreply
// ALWAYS mints it before any forge call, so every read and every write is performed as the
// worker App.
//
// WHY AN EMPTY VALUE IS A HARD REFUSAL AND NEVER A FALLBACK. deskreply writes a comment
// under a role identity. With no minted token the write would land as whatever account the
// ambient credential happens to hold — in practice the operator's own login — and reads
// afterwards, in the timeline and to everyone after, as a human's words. That is exactly the
// ambient-identity lane the custody rules retire, and unlike a failed read it cannot be
// taken back once written. The refusal is enforced in TWO independent places: this package
// refuses to hand the resolver an empty token (forgeCustody, below), and both Forge backends
// refuse to build a client without one at the transport floor.
var ghToken string

// forgeAPIBase is a TEST-ONLY override of the API base the resolved GitHub backend is
// pointed at. It is EMPTY in production, meaning "the backend's own default" — so this tool
// binds no forge host literal of its own, and there is deliberately no flag or environment
// variable that sets it.
var forgeAPIBase string

// init installs deskreply's already-minted worker token as the GitHub custody step
// deskkit.ForgeFor calls.
//
// WHY A HOOK RATHER THAN ForgeFor's DEFAULT PATH. deskreply mints with an explicit
// `--repo` (#562: the worker App is installed on more than one account, and an omitted
// --repo mints for the wrong installation — a token with no access to the target repo).
// That mint has already happened by the time any forge call is made, so letting the resolver
// mint again would both duplicate the work and reintroduce the question of which
// installation it resolved. Handing it the token this tool already holds keeps ONE mint,
// with the repo scoping the #562 fix put there.
//
// The base URL is read HERE, at call time, so a per-test override still reaches the Forge
// this produces.
func init() {
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
		if ghToken == "" {
			return "", "", errors.New(
				"refusing to reach the forge with no minted worker token — deskreply never falls back to " +
					"an ambient forge identity/keyring")
		}
		return ghToken, forgeAPIBase, nil
	})
}

// forgeFor resolves the forge that serves repo, under the worker App's custody. Which forge
// it is comes from the resolver (the roster's binding, else an unambiguous origin host, else
// a refusal naming the configuration that would answer) — never from a flag, an environment
// variable, or a default.
func forgeFor(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name := splitOwnerRepo(repo)
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	if owner == "" || name == "" {
		return nil, fr, deskkit.Unverifiable("cannot parse owner/name from "+repo, nil)
	}
	fg, err := deskkit.ForgeFor(fr, "worker")
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}

// runCmd executes name+args in dir and returns trimmed stdout. Commands are ALWAYS
// built from an explicit argv slice — never a shell string and never a caller-supplied
// flag. deskreply forwards NO caller flag into a child process: the only external values
// that reach an argv are the repo and the worktree path, each in a fixed argv position, so
// no external input can inject an option.
func runCmd(dir, name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
	cmd.Dir = dir
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

// git runs a git subcommand in dir. deskreply never pushes, commits, or touches a tracked
// file — there is no code path that builds any of those verbs, so none can reach git
// through this seam. The one config-WRITE call site (recordWorkpadID, workpad.go) sets a
// single worktree-scoped key (`git config --worktree assay.workpad <id>`, gated behind
// enabling extensions.worktreeConfig first) — repository METADATA local to this checkout,
// never a ref, an object, a tracked file, or the remote. Every other call in this package
// is a read (rev-parse / config --get).
func git(dir string, args ...string) (string, error) {
	return runCmd(dir, "git", args...)
}

// mintWorkerToken calls desktoken worker --repo <repo> to get or reuse an installation
// token for the worker GitHub App SCOPED TO repo's owner, then sets it as ghToken so every
// subsequent forge call authenticates as the worker App.
//
// --repo is not optional here (#562): the worker App is installed on more than one
// account, and desktoken defaults the owner whenever --repo is absent. Without this flag,
// deskreply running against another org's repo would silently mint a token for the WRONG
// installation — one with no access to the target repo — and the subsequent reads would fail
// with a "Could not resolve to a Repository" error, surfacing as deskreply's exit 6. Passing
// the caller's own already-verified repo (deskreply only ever calls this with the repo it
// just confirmed via preflight() matches this worktree's origin) makes resolution
// deterministic regardless of which worktree/remote deskreply happens to run in.
func mintWorkerToken(repo string) error {
	out, err := runCmd("", "desktoken", "worker", "--repo", repo)
	if err != nil {
		return fmt.Errorf("desktoken worker --repo %s: %w", repo, err)
	}
	tokenPath := strings.TrimSpace(out)
	b, rerr := os.ReadFile(tokenPath)
	if rerr != nil {
		return fmt.Errorf("read worker token from %s: %w", tokenPath, rerr)
	}
	ghToken = strings.TrimSpace(string(b))
	if ghToken == "" {
		return fmt.Errorf("worker token at %s is empty", tokenPath)
	}
	return nil
}
