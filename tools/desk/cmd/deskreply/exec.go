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
// Production binds it to exec.Command; tests wrap it to RECORD every argv so the
// "the only mutating gh call ever made is `pr comment`" and "no git push on any path"
// assertions run against the real constructed argv. Nothing else in this package
// constructs commands, so there is exactly one place argv is built.
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
// empty, gh calls use the ambient gh identity. deskreply ALWAYS mints the worker token
// before the gh pr comment so every reply is posted as the worker App.
var ghToken string

// runCmd executes name+args in dir and returns trimmed stdout. Commands are ALWAYS
// built from an explicit argv slice — never a shell string and never a caller-supplied
// git/gh flag. deskreply forwards NO caller flag into git or gh: the only external
// values that reach an argv are the repo, PR number, and body-file path, each placed in
// a fixed argv position, so no external input can inject an option.
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

// gh runs a gh subcommand in dir. deskreply ALWAYS authenticates as the worker App
// (the worker App) via the token set by mintWorkerToken; there is no ambient
// example-org fallback for the comment post. It refuses outright (rather than silently
// falling through to whatever gh identity happens to be ambient/active in this shell —
// e.g. a stale or invalid gh-CLI keyring account) if ghToken is unset, so a gh call can
// never run un-authenticated-as-worker even if a future code path forgets to mint first
// (#562: a worker-token keyring mismatch was raised as a candidate root cause; tracing
// every gh() call site shows they all run strictly after mintWorkerToken succeeds, so this
// guard is currently unreachable in practice — but it makes that invariant load-bearing
// instead of implicit).
func gh(dir string, args ...string) (string, error) {
	if ghToken == "" {
		return "", fmt.Errorf("refusing to run gh without a minted worker token — deskreply never falls back to the ambient gh identity/keyring")
	}
	return runCmd(dir, "gh", args...)
}

// mintWorkerToken calls desktoken worker --repo <repo> to get or reuse an installation
// token for the worker GitHub App (the worker App) SCOPED TO repo's owner, then sets it
// as the ghToken so every subsequent gh invocation authenticates as the worker App.
//
// --repo is not optional here (#562): the worker App is installed on more than one
// account (e.g. example-org AND medici-finance), and desktoken defaults the owner to
// "example-org" whenever --repo is absent. Without this flag, deskreply running against a
// medici-finance/* repo would silently mint a token for the WRONG installation — one with
// no access to the target repo — and the subsequent `gh pr view`/`gh pr comment` would
// fail with a GitHub API "Could not resolve to a Repository" error, surfacing as deskreply's
// exit 6. Passing the caller's own already-verified repo (deskreply only ever calls this
// with the repo it just confirmed via preflight() matches this worktree's origin) makes
// resolution deterministic regardless of which worktree/remote deskreply happens to run in.
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
