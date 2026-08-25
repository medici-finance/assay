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
//     The scan-carrier lane is dispatched ONCE PER PASS, not once per item. The scan is
//     whole-scope — one run derives the delta for every issue in the scan scope — so a per-item
//     dispatch would run it N times against one branch and one PR, and each collision would surface
//     as a dispatch error and then as a false "front door leaked" line. Every mechanical item a
//     pass admits is folded into one scan dispatch, claimed under the SCAN's key (what must not
//     happen twice at once is a whole-scope scan against one target), while the ledger keeps one
//     exit per INBOUND item.
//
//   - Land — exactly ONE tracked exit per INBOUND item (placeholder / bug / finding /
//     needs-decision / rejected-watching). Zero exits and two exits are both refusals: an inbound
//     item that left no exit is the front door leaking, and that is the one property the desk
//     exists to hold. A batched scan dispatch lands one exit for each item it covered.
//
//   - OnIdle — the next monitor poll. Stop flags (DISABLED > STOP > STOP.<loop>) are the only exit,
//     honoured through deskkit.Guard on every cycle boundary, not just at boot.
//
// # The write destination is ONE repo, and it is known before anything is classified
//
// The scan target is the repo the placeholder delta is committed to and the scan PR is opened
// against — not the inbound item's repo. An issue on a repo that is in the intake READ scope but
// outside the write boundary is ordinary work: its placeholder lands in the scan target under a
// repo-stemmed name. Conflating the two made the write boundary look like a property of the inbound
// repo, so such an item refused inside the lane, counted as a dispatch error and flagged as a leak
// — on every pass, forever, for a condition retrying can never change.
//
// Which is the general rule this consumer holds: EVERY reason a mechanical lane must not run is
// decided at CLASSIFICATION time. A lane the classifier never selects costs nothing; a lane that
// refuses costs an error, a false leak flag, and a repeat next pass.
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
