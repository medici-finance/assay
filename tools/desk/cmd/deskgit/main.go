// Command deskgit is the git-workflow desk tool (issue #1555).
// It gives the desk loops the ONE git verb they legitimately need unprompted — refresh
// the remote-tracking refs from origin — through a narrow binary whose own argv parser
// refuses everything else, instead of the allowlist rule `Bash(git fetch *)`.
//
// Why a binary and not a glob (issue #1555): `Bash(git fetch *)` is an unanchored
// wildcard granting git fetch's whole flag surface, including the option that names a
// program to run on the "remote" end — which for a local-path remote is THIS machine — so
// the allow rule is unprompted arbitrary code execution. A glob has no end anchor. This
// tool's main() does: `deskgit fetch` takes a fixed set of modes, each building a FIXED
// refspec from a validated value — nothing appendable can change what runs.
//
// `deskgit fetch` now runs IN-PROCESS (internal/gitcore, go-git): it starts no child process
// for any origin shape — a local-path or file:// origin included, which gitcore serves from the
// local repository's storage rather than through go-git's stock local transport (that one
// starts a git helper program with this process's environment; see gitcore/localtransport.go). So
// there is no program name to pin, no child environment to scrub, no config key that can name
// a program to execute, no credential helper and no askpass file. The environment still
// parameterises the connection itself (the Go HTTP client's proxy and trust-store variables;
// the ssh client's agent socket and known_hosts) — a route or a trust root, never a program.
// What is left, and what this tool still enforces:
//   - the refspec is a Go value built from a validated mode (never a caller flag), and the
//     explicit tracking refspec confines a bare fetch to refs/remotes/origin/*, so a
//     malicious remote.origin.fetch cannot redirect writes to local branches. No tag is
//     followed, in any mode: the refspec is the whole write set, so no refs/tags/* ref is
//     created or replaced (`git fetch` auto-followed tags but never replaced an existing one;
//     go-git's default mode would replace one, so the fetch follows none);
//   - a --branch/--pr fetch never writes a branch some worktree is using — its HEAD branch,
//     a branch it is rebasing or bisecting, or one an in-progress `rebase --update-refs`
//     will rewrite: git's own refusal set;
//   - it gates on the origin URL — the repository's own configured remote.origin.url, exactly
//     one value — and connects to THAT SAME STRING, so the decision and the connection cannot
//     diverge; it rejects remote-helper (`<helper>::…`) transport forms, and requires an
//     exact owner/repo path for any HOST-BEARING URL, so a padded URL cannot present an
//     allowed slug in its trailing components. Two routing bypasses of that rule — a
//     `scheme://` URL whose PATH contains '@', and a scp-like URL with no `user@` — are
//     closed; and a BARE LOCAL PATH is gated too — its identity is a match against the
//     configured local-roots allowlist, not its last two path components (#215). See
//     parseRepo, which documents the one residual that remains (host is NOT bound to
//     github.com);
//   - the `--as` form binds a credential to that one gated URL, in memory, over https to
//     github.com only (roleTokenForRepo refuses any other host before a token is read).
//
// Global- and worktree-scope git config (url.<base>.insteadOf, pushurl) no longer play any
// part in where a FETCH goes: only the repository's own config is read, and it is read
// through the same lens for the gate and for the connection. `deskgit push` still runs the
// git binary (a later brief migrates it) and keeps its own destination gate.
//
// It is NOT a sandbox against a caller who picks the repository: cmdFetch binds to
// os.Getwd() and does not check the worktree against deskkit's known roots, so the CALLER
// chooses the repo and therefore the config that governs the fetch.
//
// fetch is a local-read verb: it reaches the network read-only, makes no outward WRITE
// (no GitHub mutation, no shared-state change), so like deskwt it takes the audit line and
// the kill switch but NOT the outward-write rate limit. It is allowlisted ONLY at its
// root-owned installed path (`/opt/desk-tools/bin/deskgit`); the `go run` form is excluded
// because it would run agent-writable source — the statusgen source exemption covers
// writes/creds, not local code execution.
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled, 5 refused,
// 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskgit — the desk's narrow git verb: refresh refs from origin, and push the current branch.

USAGE:
  deskgit fetch [--prune]         # refs/remotes/origin/* only (--prune drops stale ones; no tags fetched)
  deskgit fetch --pr <N>          # pull/<N>/head -> local branch pr<N> (N digits only)
  deskgit fetch --branch <B>      # origin's <B> -> local branch <B> (not main/master in any case)
  deskgit fetch --as <role>       # any fetch mode above, authenticated from <role>'s token file
  deskgit push --as <role>        # push the CURRENT branch to origin, authenticated (not main/master)
  deskgit --version

deskgit is safe by construction: each mode builds a FIXED refspec from a validated value —
no caller flag and no arbitrary refspec reach the transport, and fetch writes only that
refspec's refs (no tag is followed or replaced; a branch any worktree is using — checked out,
being rebased or bisected — is never written). fetch runs in-process (no git
child, so no program to name, no child environment, no credential helper); push runs git with a
fixed argv that pins --receive-pack=git-receive-pack and refuses --force/--delete/--no-verify
by name. Both gate on the origin URL. --as reads the role's 0600 token file and sends the token
over https to github.com only — fetch hands it to the in-process transport in memory, push to
its one git child through an ephemeral helper that clears every ambient one; the token never
reaches argv, a URL, stdout, or the audit line. push also pins --no-recurse-submodules.
It is not a sandbox against a fully attacker-controlled .git/config. On any state it cannot
positively verify it refuses.

Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskgit sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "fetch":
		err = cmdFetch(rest)
	case "push":
		err = cmdPush(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskgit: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
