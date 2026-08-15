// Command deskgit is the git-workflow desk tool (issue #1555).
// It gives the desk loops the ONE git verb they legitimately need unprompted — refresh
// the remote-tracking refs from origin — through a narrow binary whose own argv parser
// refuses everything else, instead of the allowlist rule `Bash(git fetch *)`.
//
// Why a binary and not a glob (issue #1555): `Bash(git fetch *)` is an unanchored
// wildcard granting git fetch's whole flag surface. `git fetch --upload-pack="sh -c …"
// <local-path>` runs that program on the "remote" end — which for a local-path remote is
// THIS machine — so the allow rule is unprompted arbitrary code execution. A glob has no
// end anchor. This tool's main() does: `deskgit fetch` takes a fixed set of modes, each
// building a FIXED git argv from literal verbs plus a refspec derived from a validated
// value — nothing appendable after the verb can change what runs.
//
// A fixed argv closes FLAGS but not config or environment; the security review (#1555)
// proved git honours the upload-pack program from `.git/config` and from injected env
// config too. deskgit closes those proven vectors:
//   - it pins `--upload-pack=git-upload-pack` on the CLI, which overrides
//     remote.origin.uploadpack from config OR env — the code-execution vector;
//   - it pins `--refmap=` + an explicit `+refs/heads/*:refs/remotes/origin/*`, so a
//     malicious remote.origin.fetch cannot redirect writes to local branches;
//   - it scrubs the child environment to an allowlist (see exec.go), dropping every
//     GIT_* var (GIT_SSH_COMMAND, GIT_CONFIG_*, GIT_ASKPASS, …);
//   - it gates on the EFFECTIVE origin URL (`git ls-remote --get-url`, which expands
//     insteadOf), rejects remote-helper (`<helper>::…`) transport forms, and requires an
//     exact owner/repo path for any HOST-BEARING URL, so a padded URL cannot present an
//     allowed slug in its trailing components. Two routing bypasses of that rule — a
//     `scheme://` URL whose PATH contains '@', and a scp-like URL with no `user@` — are
//     closed as of the security review; see parseRepo, which also documents the two
//     residuals that remain (host is NOT bound to github.com, and bare local paths — which
//     an insteadOf rewrite can reach — are not subject to the exact-path rule at all).
//
// It is NOT a sandbox, and the boundary is wider than "a compromised repo" (security
// review). cmdFetch binds to os.Getwd() and does not check the worktree against
// deskkit's known roots, so the CALLER chooses the repo and therefore the `.git/config`
// that governs the fetch. In the #1555 threat model the caller IS the adversary, so an
// attacker-controlled `.git/config` is the ordinary reachable state, not an edge case.
// Under it, ANY config key that names a program is an execution route — the class, not
// just the examples: `core.sshCommand`, `core.gitProxy` (whose env twin GIT_PROXY_COMMAND
// *is* scrubbed, making it easy to misread as closed), `core.fsmonitor`,
// `remote.<n>.vcs` (which makes git run `git-remote-<name>` while `ls-remote --get-url`
// still reports an innocent URL, so the gate is structurally blind to it), and others.
//
// deskgit also TRUSTS `PATH` (security review): `PATH` is on the env allowlist and
// runGit invokes `git` by bare name, so the binary that runs is whatever PATH resolves —
// and pinning `--upload-pack=git-upload-pack` pins that program's NAME, not its path.
// The "a glob has no end anchor, but a main() does" argument therefore holds only while
// PATH is trusted. Resolving git to an absolute path and passing a fixed minimal PATH is
// a follow-up, not done here because a minimal PATH risks breaking ssh/credential helpers
// on the desk machine.
//
// deskgit closes the proven upload-pack, env, fetch-refspec and submodule vectors, and the
// insteadOf IDENTITY-SUBSTITUTION vector FOR HOST-BEARING URLs — but NOT for local paths,
// which residual 2 above leaves open. It claims no more. (The unqualified "and insteadOf"
// this list used to carry contradicted that residual four lines up; security review.)
//
// fetch is a local-read verb: it reaches the network read-only, makes no outward WRITE
// (no GitHub mutation, no shared-state change) and holds no credentials, so like deskwt
// it takes the audit line and the kill switch but NOT the outward-write rate
// limit. It is allowlisted ONLY at its root-owned installed path (`/opt/desk-tools/bin/
// deskgit`); the `go run` form is excluded because it would run agent-writable source —
// the statusgen source exemption covers writes/creds, not local code execution.
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled, 5 refused,
// 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskgit — the desk's narrow git verb: refresh refs from origin.

USAGE:
  deskgit fetch [--prune]        # refs/remotes/origin/* (--prune drops stale ones)
  deskgit fetch --pr <N>         # pull/<N>/head -> local branch pr<N> (N digits only)
  deskgit fetch --branch <B>     # origin's <B> -> local branch <B> (not main/master in any case)
  deskgit --version

deskgit is safe by construction: each mode builds a FIXED git argv from literal verbs
plus a refspec derived from a validated value — no caller flag, no arbitrary refspec, no
--upload-pack/--exec reaches git. It pins --upload-pack=git-upload-pack (overriding the
config/env upload-pack code-execution vector), pins --refmap= + an explicit refspec,
scrubs the child env to an allowlist, and gates on the effective origin URL (expands
insteadOf). It is not a sandbox against a fully attacker-controlled .git/config. On any
state it cannot positively verify it refuses.

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
	default:
		fmt.Fprintf(os.Stderr, "deskgit: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
