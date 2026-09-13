### Added
- `commsloop` now routes **every** accepted inbound message through a contained prose
  router (`decide.go`) — no deterministic routing table, no fast path. A retired mechanical
  shortcut used to land report-shaped verbs (`status`/`metrics`/`help-offered`) done with no
  consult at all; every accepted message that clears the routing-boundary ACL re-check now
  reaches the SAME consult. The router picks one action from a closed set
  (`route-work-dispatch`/`route-review`/`route-verify`/`land-report`/`file-question-issue`/
  `escalate-human-issue`/`quarantine`, default `quarantine`) and never names a runner
  selection; `assign.go`'s compiled `(action, class, risk) -> Tier` table does that lookup
  deterministically. The router's reader runs under the same refuse-everything containment
  profile as the outbound prose gate: an empty filesystem root, and fs/terminal/tool
  callbacks refused and filed as containment anomalies.

### Changed
- `assign.go`'s `KnownActions` is now bound to the router's own declared action set by a
  diff test (`TestRouterActionsMatchAssignKnownActions`), replacing the earlier mirrored
  copy. The brief's literal spelling of the dispatch action (`route-work-ready`) collided
  with `deskkit.NewQuestion`'s construction-time reserved-verb deny-list (the `ready` token
  is reserved for the PR-ready-flip guard) — a real, boot-time refusal, not a hypothetical
  one — so it is spelled `route-work-dispatch` instead; same semantics, no collision,
  recorded at its definition.
- An invalid/timed-out/budget-exhausted/valve-disabled consult resolves to the default
  action (`quarantine`), so `commsloop` is fail-closed whether or not a decider is even
  configured — mirroring the outbound gate's own posture.
