// Command scanloop is the intake-desk REFERENCE CONSUMER of the deterministic drain engine
// (tools/desk/internal/loopengine) — the FOURTH desk drain, after the verify, worker and review
// consumers. It mechanizes the intake loop that is otherwise carried as ~100 lines of skill prose:
// arm the inbound monitor, read the inbound surface, apply the trust gate, coalesce, regenerate the
// scan PR's body, record the exit. Prose that two sessions can read two ways is the repeatability
// hole this binary closes; the CLI is the contract, not the Go API.
//
// # Engine mapping
//
//   - SelectQueue — inbound items across the configured intake SCAN scope (deskkit.ScanRepos, the
//     same roster key the scanner itself reads, so coverage cannot drift between them). The events
//     come from the durable inbound monitor script, which is WRAPPED and never re-implemented: its
//     three anti-blindness properties (explicit identity, per-repo retained state, burst cap) are
//     not worth losing to a second hand-rolled poller.
//
//   - TierPolicy — the MECHANICAL half only. A worker-legible item (a new issue needing a
//     placeholder, a retire, a close-on-fix) is TierLocal and the scan-carrier lane executes it
//     deterministically. Everything whose disposition is a JUDGMENT — which of the five tracked
//     exits an item takes, and the ownership/origin routing test — is TierSession: it is EMITTED
//     for a model tier to route and is NEVER computed here. A loop that guessed those would be
//     wrong in the direction that compounds through every worker the placeholder spawns.
//
//   - Dispatch — one of two lanes, both behind the single seam in lane.go: the scan-carrier-PR lane
//     (isolated worktree, the scan write, a derived title/body, a draft PR through the sanctioned
//     PR verb) or the issue-filing lane (the sanctioned filing verb).
//
//   - Land — exactly ONE tracked exit per item (placeholder / bug / finding / needs-decision /
//     rejected-watching). Zero exits and two exits are both refusals: an inbound item that left no
//     exit is the front door leaking, and that is the one property the desk exists to hold.
//
//   - OnIdle — the next monitor poll. Stop flags (DISABLED > STOP > STOP.<loop>) are the only exit,
//     honoured through deskkit.Guard on every cycle boundary, not just at boot.
//
// # The two configuration rules that are code here rather than prose
//
//  1. COALESCE IS BOUNDED. A scan PR younger than the coalesce window (20 minutes by default)
//     absorbs the next inbound batch; at or past the window a FRESH branch and PR are cut. The
//     unbounded rule this replaces let every new inbound append to the same PR forever, so the PR
//     never reached a stable head: it could be approved, green and mergeable and still be a growing
//     draft hours later, because every coalesce moved the head and re-opened the review cycle. The
//     window seals each scan PR at a stable head. A PR whose age cannot be established does NOT
//     coalesce — could-not-check takes the bounded direction.
//
//  2. THE BODY IS REGENERATED ON EVERY PUSH, not just the first. The title and body state counts
//     describing a diff that grows with every coalesced commit, so a body written once is wrong by
//     the second push. Regeneration is structural here: the push step and the regenerate step are
//     one function that cannot perform half of itself.
//
// # Subcommands
//
//	scanloop plan --root <repo> [--inbound <file|->] ...
//	    READ-ONLY. Prints the inbound queue — surface, item, lane, age, claim state — plus the
//	    monitor's arming state and this consumer's own unread surfaces. It does not run the monitor
//	    (a monitor run ADVANCES per-repo baselines and would consume the very events it reports),
//	    spawns nothing, and writes nothing.
//
//	scanloop run --root <repo> ...
//	    The drain. Arms the monitor if it is not armed, applies the trust gate BEFORE queueing,
//	    executes the dispatch lanes and records one tracked exit per item.
//
//	scanloop --version
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
package main
