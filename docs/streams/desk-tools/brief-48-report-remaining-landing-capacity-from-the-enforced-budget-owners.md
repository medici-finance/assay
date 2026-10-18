---
brief: assay:assay:desk-tools:48
title: "Report remaining landing capacity from the enforced budget owners"
why: "Pool width is already bounded, but raw queue depth and old fixed writes-per-flip assumptions still mislead widening advice. Operators need a dated capacity reading derived from the actual enforced meters."
wave: 2
depends: ["desk-tools/45"]
unblocks: ["desk-tools/51"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "Changes verification scheduling/landing behavior relied on by adopters; human review must confirm the preserved provenance and authority boundaries."
decision-trigger: start
issues: []
schema: brief-v2
authored: "2026-10-17 maintainer-directed verify-flow refresh"
sources:
  - "docs/streams/desk-tools/README.md"
  - "freshness-checked 2026-10-17 @ 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae"
exec-tier: strong
exec-tier-why: "Cross-component queue, input provenance and effect boundaries must agree; a subtle error can suppress needed verification or advance the wrong state."
domain: complicated
outcome: none
version: 2
id: 31b86553-356d-406c-9fb2-2b4513fd5a49
consumers:
  - "tools/desk/cmd/deskroster/flow_48_test.go: follow-up desk-tools/48 (planned test contract)"
  - "tools/desk/cmd/deskboard/flow_48_test.go: follow-up desk-tools/48 (planned test contract)"
  - "tools/desk/internal/deskkit/ratelimit.go: follow-up desk-tools/48 (this implementation brief)"
  - "tools/desk/internal/deskkit/ratelimitread.go: follow-up desk-tools/48 (this implementation brief)"
  - "tools/desk/internal/deskkit/width.go: follow-up desk-tools/48 (this implementation brief)"
  - "tools/desk/cmd/deskroster/main.go: follow-up desk-tools/48 (this implementation brief)"
  - "tools/desk/cmd/deskboard/throughput.go: follow-up desk-tools/48 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/main.go: follow-up desk-tools/48 (this implementation brief)"
  - "changelog/verify-landing-capacity.md: follow-up desk-tools/48 (this implementation brief)"
---

# Brief 48 — Report remaining landing capacity from the enforced budget owners

## Context

files: tools/desk/internal/deskkit/ratelimit.go, tools/desk/internal/deskkit/ratelimitread.go, tools/desk/internal/deskkit/width.go, tools/desk/cmd/deskroster/main.go, tools/desk/cmd/deskboard/throughput.go, tools/desk/cmd/verifyloop/main.go, changelog/verify-landing-capacity.md, tools/desk/cmd/deskboard/flow_48_test.go, tools/desk/cmd/deskroster/flow_48_test.go
facts:
- Rate policies and bounded ledger reads already exist in deskkit; deskroster width already enforces budget and shared-token limits.
- The September 10-writes/hour, five-flips/hour and three-issues/day estimates are not current universal limits. Operation cost differs by target and landing profile.
- A local ledger does not prove account-wide provider capacity or other-host usage. Those observations have explicit scope and timestamps.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "A dated read model over current meters, never a new limiter, cap increase, reservation authority or universal GraphQL budget."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Add a read-only budgets/capacity view to the existing roster/report owner. Read enforced policies, charged events, breakers, in-flight reservations and provider observations from their owners; do not hand-copy caps into a new configuration table.
2. Expose remaining known writes and the cost of the proposed remaining effects. Label estimate scope, observation time and unknown provider/cross-host capacity. An exhausted or unreadable meter must not render zero work or positive spare capacity.
3. Bound advice by current actionable lanes, landing capacity and the existing roster maximum. Distinguish model dispatch from continuation work; a blocked write budget is never a reason to widen. Leave current authorization and limits unchanged.
4. Use clock/ledger fixtures for reset windows, heterogeneous effect cost, concurrent reservations, missing provider reads and partial roots. Compare report values with the actual enforcement meter on the same fixture, including a planted second exhausted target.

5. At implementation delivery, convert all 9 consumer entries that name this brief from `follow-up desk-tools/48` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 9 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

A dated read model over current meters, never a new limiter, cap increase, reservation authority or universal GraphQL budget.

## Ground rules

- Isolated branch and draft PR; no merge, main push, workflow activation or live infrastructure query in this brief.
- Existing narrow landing permissions apply at runtime; a refusal stops the operation and never licenses another identity or channel.
- Do not rewrite old Evidence, invent a witness, close real gate cards or fabricate a human stamp.
- Stop at implementation; independent verification owns acceptance Evidence.

## Verify

New test names below are planned contracts. The named PASS assertion must fail if a selector runs no test. Run commands from the repository root.

Consumer-diff provenance: obtain the full immutable `DELIVERY_HEAD` and `DELIVERY_BASE` from this brief's actual delivering PR; the base is its reviewed merge-base, not whichever main happens to be current. Record both SHAs and the command output in Evidence. After merge, use the delivery merge's second parent as the head and the merge-base of its two parents as the base, as described in `docs/brief-rules.md`. Run the consumer row in a separate checkout at that exact head, with those variables set; never move an already admitted verifier home to satisfy it. An absent delivery binding or empty/unrelated diff is could-not-check, never acceptance. The literal coverage summary accounts for every entry; no unchecked residue is accepted by this authored contract.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskroster -run "^TestFlowCapacity$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskroster.log" 2>&1 && grep -F -e "--- PASS: TestFlowCapacity " "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskroster.log"` | exit 0; output contains the named PASS line. Cases: report and enforced meter agree; breaker/reset/race boundaries; partial provider evidence remains unknown; held/human-only depth cannot produce widen advice | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/deskboard -run "^TestFlowCapacity$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskboard.log" 2>&1 && grep -F -e "--- PASS: TestFlowCapacity " "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskboard.log"` | exit 0; output contains the named PASS line. Cases: report and enforced meter agree; breaker/reset/race boundaries; partial provider evidence remains unknown; held/human-only depth cannot produce widen advice | check:ci +mutation +flow +dereference |
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up desk-tools/48' docs/streams/desk-tools/brief-48-report-remaining-landing-capacity-from-the-enforced-budget-owners.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/48 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 9 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no self-deferred route, and all 9 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-48-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
