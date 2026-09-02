// Command clusterguard is an EXEC-BOUNDARY shim for cluster CLIs.
//
// THE GAP IT CLOSES. Permission rules that match on command TEXT cannot see a
// cluster call made from inside a committed script, and a written policy only
// covers the paths somebody thought to write a line about. The premise is not
// that sessions ignore prose — an explicit refusal is respected — it is that
// uncovered surface has nothing behind it and, worse, leaves no trace: the only
// way anyone learns a probe happened is if the caller says so.
//
// A shim at the exec boundary survives both problems. The cluster CLIs on a
// session's PATH resolve to this one binary, which refuses by default, records
// the attempt, and passes through only in a shell that deliberately exported
// the operator opt-in. "Sessions are offline" stops being an instruction and
// becomes a mechanical property with a refusal log behind it.
//
// HOW IT IS WIRED. A shim DIRECTORY holds one symlink per shimmed CLI
// (kubectl, flux, helm, talosctl, k9s), each pointing at this binary; sessions
// prepend that directory to PATH. The guard reads which CLI it is from argv[0]
// and, on pass-through, resolves the real binary by scanning PATH for the first
// executable of that name that is not ITSELF (os.SameFile against the running
// executable — the self-resolution loop is the classic shim bug, and a
// deployment may have the shim directory on PATH more than once).
//
// THE POLICY.
//
//	ASSAY_ALLOW_CLUSTER unset      every shimmed CLI is refused (exit 5)
//	ASSAY_ALLOW_CLUSTER=1          read-only verbs pass; mutating verbs refused (exit 5)
//	ASSAY_ALLOW_CLUSTER=mutate     every verb passes
//	ASSAY_ALLOW_CLUSTER=<other>    refused (exit 5) — a typo in a safety opt-in is never a guess
//
// The read-only classification is an ALLOWLIST per CLI, so an unclassified verb
// — including a subcommand added upstream tomorrow — is treated as mutating.
// k9s has no read-only lane at all: it is an interactive TUI whose mutating
// operations are reachable from inside the session, so nothing in its argv can
// establish that the call is read-only.
//
// The opt-in is a per-SHELL export an operator makes deliberately, the same
// shape as the writeguard shared-checkout token. A desk window, a dispatched
// worker, or a script one of them runs must never export it. It is declared in
// deskkit only so that recording it in the shared roster.env cannot collapse
// the roster (see deskkit.EnvAllowCluster); recording it there does not grant
// it.
//
// STOP FLAGS ONLY TIGHTEN. deskkit.Guard() is called before any verdict, but an
// armed flag is itself a refusal (exit 3). This guard deliberately does NOT
// participate in the kill switch the way an acting tool does: a refusal-guard
// that stopped intercepting when disabled would fail OPEN, handing every
// stopped session the cluster CLIs back — the exact inversion of what arming a
// kill switch is for. The uninstall path is removing the shim directory from
// PATH.
//
// STATED LIMITS — this is one layer, and a narrow one.
//
//   - A call made by ABSOLUTE PATH never consults PATH and is never
//     intercepted. That is by design, not an oversight, and there is a test
//     asserting the bypass exists so the limit cannot quietly stop being true.
//   - A session that constructs its own environment without the shim directory
//     evades this layer entirely. PATH resolution is the single point of
//     failure, which is why a credential quarantine belongs BEHIND it: a probe
//     that evades the shim should still find nothing to authenticate with.
//   - The guard keys on five CLI NAMES. HTTP or SDK access, a credentialed tool
//     integration, and ssh/docker hops are outside it. It narrows one exec
//     path; it is not a network boundary.
//
// Every test in this package runs against FIXTURE binaries written into a temp
// directory. Nothing here contacts a live CLI, a live cluster, or the network.
//
// Exit codes (deskkit contract): 0 passed through · 3 stop flag armed ·
// 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import "os"

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) }
