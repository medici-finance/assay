// Command deskclaim-ref is the pure-Go port of the consumer dispatch-claim script — the
// GitHub-durable, cross-machine dispatch claim (methodology/42). It is what deskdispatch
// invokes for its claim-acquire step when the repo carries no tools/dispatch-claim.sh, and
// it is the ONLY claim path that works on a native-Windows adopter: it is a plain executable
// resolved by CreateProcess through PATH, with no shebang and no `bash.exe` association to
// depend on.
//
// WHY A GO PORT AND NOT A SHELL DROP. tools/dispatch-claim.sh is a bash script with a
// `#!/usr/bin/env bash` shebang. Windows `CreateProcess` does not honour a shebang, so a
// `.sh` drop only runs where the operator already wired a bash association — exactly the
// "run Git Bash" paper-over issue 708 rules out. This binary carries the SAME wire protocol
// as the script (the `refs/dispatch/<id>` claim ref namespace on the forge, the same holder
// encoding in an annotated tag object carrying `dispatch-claim <id> owner=… state=… branch=…`,
// and the same deskkit exit codes 0/5/6) so a Go dispatcher and any remaining bash dispatcher
// derive the SAME claim key and ref and never double-dispatch during the transition.
//
// FORGE TRANSPORT. Every forge access is an IN-PROCESS git-smart-HTTP call over go-git
// (internal/gitcore) — no `gh`/`glab`/any CLI, no external `git` process, no credential helper.
// A claim is minted as an annotated tag targeting the empty blob and placed with an
// EXPLICIT-OLD (server-side compare-and-swap) receive-pack push: create is old=zero (the server
// rejects a create against an existing ref — that is the "already held" answer), and
// advance/steal is old=<the value last read> (the server rejects a stale old — closing the
// races the old `PATCH force=true` and DELETE-then-POST steal left open). Reads use a filtered
// upload-pack, so a bash/REST-minted tag (commit target) and a Go-minted tag (empty-blob target)
// both read the same way. The forge (github/gitlab) resolves token-free from ASSAY_REPO_FORGES
// or the origin host; the credential comes from --token-file or
// GH_TOKEN/GITHUB_TOKEN/GITLAB_TOKEN — the same ambient identity the script uses, so this
// changes the transport, not who the claim is placed as. The tagger date is client-stamped (a
// documented change from the server-stamped gh-CLI port): mutual exclusion rests on the
// server-side CAS, not the timestamp, which now drives only the TTL age display.
//
// VERBS (identical to the script):
//
//	deskclaim-ref acquire  <id> [--repo O/R] [--owner O] [--branch B]
//	deskclaim-ref progress <id> [--repo O/R] [--owner O] --branch B
//	deskclaim-ref release  <id> [--repo O/R]
//	deskclaim-ref steal    <id> --reason "<why>" [--repo O/R] [--owner O]
//	deskclaim-ref show     <id> [--repo O/R]
//	deskclaim-ref list          [--repo O/R]
//	deskclaim-ref --version
//
// <id> is the claim key, identical to the deskclaim key convention:
//
//	<repo>--<stream>--<NN>   briefs        e.g. at--methodology--42
//	<repo>--issue-<NN>       issue-shaped  e.g. at--issue-575
//
// The <repo> prefix is MANDATORY — two repos own a stream named the same, and an
// unqualified key cross-locks the wrong brief.
//
// Exit codes follow the deskkit contract (tools/desk/internal/deskkit/exitcodes.go):
//
//	0 ok/acquired · 5 refused (a live holder owns it — do NOT proceed) · 6 unverifiable
//
// 6 is the fail-closed code: a claim we could not read or write is NEVER "assume free".
//
// SCOPE NOTE — the `refs/dispatch/*` namespace this port speaks matches the bash script it
// ports (and the transition requirement that a Go and a bash dispatcher collide on the same
// ref). It is NOT the same namespace as deskkit.ClaimRefsPrefix (`refs/heads/dispatch/*`),
// which the fleet's Go claim READERS (desksupervise, fanoutloop, loopengine) list against.
// That divergence predates this port (deskdispatch already shells the `refs/dispatch/*`
// script today) and is flagged for a house ruling in issue 708's PR — this binary
// deliberately does not resolve it, because unilaterally moving the namespace here would
// stop a Go acquire colliding with a still-running bash acquire, the exact double-dispatch
// this claim exists to prevent.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskclaim-ref — the GitHub-durable, cross-machine dispatch claim (pure-Go port of
tools/dispatch-claim.sh; the only claim path that runs native on Windows).

USAGE:
  deskclaim-ref acquire  <id> [--repo O/R] [--owner O] [--branch B]
  deskclaim-ref progress <id> [--repo O/R] [--owner O] --branch B
  deskclaim-ref release  <id> [--repo O/R]
  deskclaim-ref steal    <id> --reason "<why>" [--repo O/R] [--owner O]
  deskclaim-ref show     <id> [--repo O/R]
  deskclaim-ref list          [--repo O/R]
  deskclaim-ref --version

<id> is the claim key: <repo>--<stream>--<NN> or <repo>--issue-<NN>. The <repo> prefix is
MANDATORY — a bare stream name cross-locks another repo's stream.

The forge credential is read from --token-file <0600 file> when given, else GH_TOKEN /
GITHUB_TOKEN / GITLAB_TOKEN. The forge (github/gitlab) resolves from ASSAY_REPO_FORGES or the
origin remote host; every forge call is an in-process git-smart-HTTP request (no gh/glab CLI).

acquire   take the claim if free; reclaim it if the holder is past its TTL; refuse (exit 5)
          if a live holder owns it. Never steals a live claim inline.
progress  advance an acquired claim to state=dispatched (TTL 20m -> 120m).
release   delete the claim ref (branch-as-claim takes over). A missing claim is a no-op.
steal     forcibly take the claim, recording --reason in the replacement — an auditable
          takeover, not a hand-delete.
show      print the claim's holder/state/branch/age, or FREE.
list      show every dispatch claim in the repo.

Exit codes (deskkit contract): 0 ok/acquired · 5 refused (a live holder owns it) ·
6 unverifiable (a claim we could not read or write — NEVER "assume free").`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(out, "deskclaim-ref sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(errOut, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}
	return dispatchVerb(args)
}
