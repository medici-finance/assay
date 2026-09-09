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
// encoding in an annotated tag object, the same GitHub-stamped `tagger.date` for a single
// skew-free clock, and the same deskkit exit codes 0/5/6) so a Go dispatcher and any
// remaining bash dispatcher derive the SAME claim key and ref and never double-dispatch
// during the transition. It shells `gh api` for every forge call, exactly as the script
// does, so it inherits the same ambient identity and the byte-identical REST surface.
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
