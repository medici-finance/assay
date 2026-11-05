---
brief: assay:assay:desk-containers:33
title: Automatic desk instances and desired population through the shared runtime
why: Operators need multiple independently configured workers and reviewers without duplicate work, deliberately multiplied role targets or up restarting desks intentionally left paused.
wave: 9
depends: ["graph-execution/27", "desk-containers/15"]
unblocks: ["desk-containers/19", "desk-containers/20", "desk-containers/22", "desk-containers/28"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: Changes desired-population, instance identity and legacy-to-managed ownership interpretation; the human confirms that paused work, role pacing targets and independent safety limits survive migration before enrollment.
decision-trigger: spec
issues: []
schema: brief-v2
version: 3
authored: 2026-10-25 by Assay architecture authoring session
sources:
  - docs/proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md
  - "freshness-checked 2026-10-25 @ 57196d5da (fetched public main; launcher, cadence, width and existing claim/admission seams inspected)"
  - "2026-11-04 routing: instance stop and drain deltas moved here from completed desk-containers/16 and /18; parallel compatibility brief desk-containers/31 (shared cellctl files, no ordering edge) noted under Ground rules"
exec-tier: strong
exec-tier-why: Desired-state convergence, concurrent name allocation and cross-module instance ownership require design judgment.
domain: complicated
consumers:
  - "cellconfig/: follow-up desk-containers/33 (desired-population contract and resolution)"
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/33 (configuration, automatic names, up convergence and cell-op extension)"
  - "tools/desk/internal/cellop/: follow-up desk-containers/33 (required instance contract negotiation)"
  - "loopadmin/ standing clients and instance ownership: out-of-scope (consumed from graph-execution/26–27, not reimplemented)"
  - "tools/desk/internal/deskkit/cellstopflag.go: follow-up desk-containers/33 (instance-targeted stop mapping beside completed desk-containers/16 stops)"
  - "tools/desk/cmd/cellctl/drain.go: follow-up desk-containers/33 (instance selectors, exact-instance drain and admission barrier on completed desk-containers/18)"
  - "tools/desk/cmd/cellctl/ rolling replacement: follow-up desk-containers/19"
  - "tools/desk/cmd/cellctl/ compatibility Kubernetes routing: out-of-scope (desk-containers/31; stays singleton per role here)"
  - "managed Kubernetes renderer/operator: follow-up desk-containers/28"
  - "served schema/public client: follow-up cell-api/01"
---

# Brief 33 — Automatic desk instances and desired population

## Context

files: `cellconfig/` (desired-population schema/parser/resolution and fixtures),
`tools/desk/cmd/cellctl/{cell.go,cobra.go,desk.go,up.go,up_converge.go,launch.go}`,
`tools/desk/cmd/cellctl/instances.go` (planned), `tools/desk/cmd/cellctl/instances_test.go` (planned),
`tools/desk/cmd/cellctl/drain.go`, `tools/desk/internal/deskkit/cellstopflag.go`,
`tools/desk/internal/cellop/`, `spec/cell-op-v1.md`, `schemas/cell-op-v1.json`,
`docs/cellctl.md`, `docs/cellctl-desk-instances.md` (planned),
`docs/proposals/non-ci-architecture/desk-instance-words.txt` (versioned naming input),
`tools/cellctl/testdata/`, `tools/desk/cmd/cellctl/testdata/`,
`.github/workflows/` (actual cross-module trigger owners),
`changelog/desk-instances.md` (planned).

facts: Public main 57196d5da keys worktrees and cadence leases by role. Completed /15
and /17 provide the machine seam and singleton up behavior, and completed /16 and /18
the cell/role stops and drain/pause/resume; their historical Evidence is consumed, not
rewritten, and their instance deltas are Task 6–7 here. /27 supplies managed standing clients with shared R2/R3
ownership/admission. This brief adapts configuration and operator selection; it creates
no second queue, lease authority, process supervisor, credential broker or database.

single-point-of-failure: population/config resolution could select a wrong instance;
independent supervisor instance/generation/readiness checks and role executor authority
remain behind it. Verify I3 bypasses the configuration caller.

layering: cellconfig owns pure versioned config/name/selector rules; cellctl is an adapter
onto the existing standing-client and cell-op contracts. Runtime bindings live under
R2/R3. Reuse public APIs across modules, never tools/desk/internal from another module.

design-fit:
  owner: cell operation/configuration, consuming existing loopadmin coordination
  contract: none — extend existing cellconfig and cell-op contracts through required capability negotiation
  retires: [role-only managed instance selection, one-window-as-population assumption]
  weight: verbs +0 required, flags instance selector as needed, refusal predicates collision/exhaustion/ambiguity/unsupported required capability; exact delta reviewed at pickup
  why-add: One config/selector owner removes singleton coupling; a separate launcher or population controller would fork ownership.

## Read first

- [Desk-instance amendment](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md).
- [Standing clients](../graph-execution/brief-27-standing-desk-loop-admin.md).
- [Machine seam](brief-15-cell-op-machine-seam.md) and [existing up](brief-17-up-converges-the-cell.md).
- [Library boundaries](../../library-first.md) and [JSON extension contract](../../proposals/non-ci-architecture/29-extensible-json-contracts.md).

## Human decision

At pickup, present the exact desired-population encoding and legacy migration plan.
Demonstrate that paused desks and busy predecessors survive migration, that independent
refill converges toward the approximate role target, and that hard safety limits remain
intact when multiple workers and reviewers are introduced. No live desk is enrolled by this brief.

The recorded stop-control ruling on #4401 predates the instance-targeted
scope that Task 6 adds. Preserve that human gate: pickup must confirm the ruling covers the
instance mapping or obtain an explicit scoped ruling. Stronger-stop precedence and existing
safety controls remain unchanged.

Options:
1. **Accept the demonstrated scoped migration** — adopt the versioned population representation after its independent checks; runtime activation remains separate.
2. **Hold for correction** — preserve the supported singleton configuration until the missing compatibility or ownership proof is supplied.

Default if no answer: none — blocks until answered

## Ground rules

Documentation authoring is not implementation, deployment or activation. Implementation
uses an isolated branch and draft PR; no live cluster, model/provider call or credential
provisioning. Independent verification records Evidence. Refresh current claims and affected
active PRs before changing the selected contracts; keep historical Evidence and human gates.

desk-containers/31 (trusted compatibility Kubernetes desks) edits the same cellctl command
files in parallel. Neither depends on the other: /31 runs one StatefulSet per role and this
brief's legacy ROLES mapping keeps one implicit entry per role, so either landing order
preserves its behaviour. Whichever lands second merges main and keeps the other's routing
and goldens; a compatibility-profile entry stays singleton here and multiplicity for it refuses.

## Task

1. Publish the exact versioned desired-population encoding in operator-owned cell.env:
   stable desired-entry key, role, qualified profile references, desired running/paused/stopped
   state and optional narrower instance refill target. Parse without shell evaluation. Legacy
   ROLES maps to one implicit entry per role; reject ambiguous mixed input and duplicate
   singleton roles. Worker/review alone support multiplicity initially.
2. Allocate names automatically from the versioned 200-word input, role/harness/provider
   and cell; validate dictionary uniqueness/length. Persist entry-to-instance/name binding
   through the existing coordination owner with atomic collision checks. Retry collisions,
   hold on exhausted namespace, preserve bindings across up/restart and reserve names for
   paused/draining/unreconciled members. New replacement instances get new identities.
   Never derive authority from a descriptive name. Validate platform locator lengths and
   retain full names in metadata when a deterministic shorter locator is required.
3. Adapt desk/up/selectors and worktree/cadence references to existing instance-aware /27
   APIs. up reads desired population, starts missing running entries, preserves paused and
   stopped entries plus persistent stops and draining predecessors, and never implicitly
   creates a replacement or changes capacity. Exact-instance operations leave peers alone;
   role/cell operations expand explicitly and short selectors refuse ambiguity.
4. Extend cell-op results/plans using its existing required-capability/version rules,
   carrying instance ID/generation, desired revision, profile and replacement relationship
   alongside retained role identity. A reader unable to represent required instance state
   refuses rather than flattening to one role. Do not reopen completed /15 or /17 Evidence.
   Document the stable consumer contract for /18–24, /28 and cell-api/01.
5. Test actual cellconfig → cellctl → standing-client/supervisor flow with fixture
   harnesses and disposable coordination. Keep legacy singleton/goldens compatible where
   behavior is unchanged; record deliberate new-output/oracle differences and widen CI
   paths for every module/fixture the consumer checks read. Update public config/CLI docs
   and changelog. Do not add another implementation of role capacity or item claims.
6. Instance stops (follow-on from completed desk-containers/16): preserve existing
   machine/cell/role STOP meanings and add instance-targeted stop mapping through the common
   lifecycle owner. An instance pause holds only that instance's new assignments; role/cell
   stops hold every matching instance. Resume removes only the stop it owns, and up never
   removes a pause to satisfy population. Desired paused and runtime persistent stops are both
   respected.
7. Instance drain (follow-on from completed desk-containers/18): drain/pause/resume accept the
   instance selectors and results from Tasks 3–4 and keep per-instance lifecycle/handoff
   records. Role/cell selectors expand explicitly; an instance stop verifies exact
   instance/generation/process or Pod UID and never closes a peer. Expose a durable
   stop-new-admission barrier separate from waiting for drain, so /19 can overlap successor
   startup. Active children keep their item claims and contribute observed work to
   approximate pacing; unknown effects stay held independently. Preserve every existing
   plan/TTY and foreign-stop restriction from /18.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or required-case skips fail independent verification.

| # | Class | Command | Expect |
|---|---|---|---|
| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C cellconfig test -count=1 -v ./... -run '^TestDeskPopulation$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskPopulation ' "$o")` | Named PASS; legacy singleton, two mixed-profile workers/three reviewers and paused entry parse correctly; mixed ambiguous input, duplicate singleton and unsupported required version refuse. Mutation: collapse by role fails. |
| I2 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskNames$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskNames ' "$o")` | Named PASS; 200 distinct 3–4 letter words, concurrent collision retry, exhaustion hold, stable binding on repeated up, short-selector ambiguity and long Kubernetes locator tested. Mutation: overwrite collision or regenerate on every up fails. |
| I3 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskPopulationUp$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskPopulationUp ' "$o")` | Actual config/client/supervisor fixture preserves paused/draining peers, reuses instance IDs and rejects stale generation below caller; independent role pacing includes both repositories, tolerates transient over/under and corrects after refresh. Mutation: clear pause, key lease by role or use only each instance's own work as the role estimate fails. |
| I4 | check +flow +neighbour | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./internal/cellop/... -run '^TestDeskInstanceReaders$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceReaders ' "$o")` | Named PASS; real result/plan plus adjacent reader retains instance/generation/desired revision, stale plans refuse and incapable reader cannot flatten required population; singleton output remains supported. |
| I5 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskInstanceStops$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceStops ' "$o")` | Named PASS (zero matching tests is not a pass); pause one of two workers and the peer keeps working; a role stop then holds both; resume cannot lift a foreign stop and repeated up preserves the paused member. Mutation: use only the role stop path for an instance; the test fails. |
| I6 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDrainDeskInstance$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDrainDeskInstance ' "$o")` | Named PASS (zero matching tests is not a pass); an old worker stops taking new dispatch but finishes its held task while an unpaused peer continues; identity mismatch and drain timeout keep the targeted owner and report hold. Mutation: close role-wide surfaces or report an unknown stop as drained; the test fails. |
| I7 | check +dereference | `statusgen --root . --lint` | exit 0; independent verifier additionally resolves named implementation consumers against the landed delivery and its actual first parent; unchecked required integration remains held. |
| I8 | check:ci +dereference | `statusgen --consumers --brief desk-containers/33 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Evidence

<!-- Independent verifier records exact implementation SHA, commands, named results,
mutation outcomes, consumer coverage and environment. No implementation evidence yet. -->

## Review

Human gate. Confirm scoped legacy migration, the stop-ruling applicability for Task 6, independent instance ownership/capacity checks,
reader compatibility and effective paused-peer preservation. Passing fixtures do not enroll
live desks, approve a profile or authorize deployment.
