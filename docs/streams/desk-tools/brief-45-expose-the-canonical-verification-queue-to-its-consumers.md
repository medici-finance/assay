---
brief: assay:assay:desk-tools:45
title: "Expose the canonical verification queue to its consumers"
why: "The board computes owned verification queues, but the JSON seam drops that classification and throughput counts all implemented briefs. One shared contract must distinguish actual model work, continuation, human action and unreadable state."
wave: 1
depends: []
unblocks: ["desk-tools/46", "desk-tools/48", "desk-tools/51"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "Changes verification scheduling/landing behavior relied on by adopters; human review must confirm the preserved provenance and authority boundaries."
decision-trigger: start
decision-issue: 3382
ruling: "https://github.com/medici-finance/assay/issues/3382#issuecomment-6258267531"
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
id: b40ce6af-2881-4f1a-874e-59182406b326
consumers:
  - "tools/desk/cmd/verifyloop/flow_45_test.go: fixed-here (planned test contract)"
  - "tools/desk/cmd/deskboard/flow_45_test.go: fixed-here (planned test contract)"
  - "statusgen/flow_45_test.go: fixed-here (planned test contract)"
  - "statusgen/main.go: fixed-here (this implementation brief)"
  - "statusgen/awaiting_bucket.go: fixed-here (this implementation brief)"
  - "statusgen/gatescores_e2e_test.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/deskboard/roots.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/deskboard/throughput.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/main.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/queueclass.go: fixed-here (this implementation brief)"
  - "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (this implementation brief)"
  - "changelog/verify-queue-contract.md: fixed-here (this implementation brief)"
---

# Brief 45 — Expose the canonical verification queue to its consumers

## Context

files: statusgen/main.go, statusgen/awaiting_bucket.go, statusgen/gatescores_e2e_test.go, tools/desk/cmd/deskboard/roots.go, tools/desk/cmd/deskboard/throughput.go, tools/desk/cmd/verifyloop/main.go, tools/desk/cmd/verifyloop/queueclass.go, tools/desk/internal/deskkit/citrigger_test.go, changelog/verify-queue-contract.md, statusgen/flow_45_test.go, tools/desk/cmd/deskboard/flow_45_test.go, tools/desk/cmd/verifyloop/flow_45_test.go
facts:
- bucketAwaiting and placeAwaiting already classify human gate, rework, environment-blocked, runner-pending, desk-actionable and unknown states; paused/parked are separate placements.
- runGateScores currently emits brief, score, blockedCount, stream, status and repo only. deskboard throughput counts status==implemented.
- verifyloop already has wait, stuck-flip, online, repair and Evidence-only dispositions. These are operational overlays, not a reason to create a competing base taxonomy.
- The existing supported cross-module seam is the statusgen command output. Do not promise importing its package main; reuse an importable canonical API only after proving that the selected caller/module can actually use it.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "One canonical queue meaning, additive JSON fields and explicit unknown coverage; no new independent classifier or subprocess per row."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Extend the existing JSON rows additively with a versioned queue classification, owner, next action, gate/risk facts and explicit classification validity. Preserve old fields and ordering contracts. Export the actual current bucket owner, not the retired blockerSegment/classifyAwaiting design.
2. Have deskboard awaiting and throughput consume the same result. Distinguish judgment dispatch, runnable offline rows, continuation and human action; exclude non-actionable rows from advice to widen model verification. Report unreadable roots and unsupported/missing contract fields as coverage gaps rather than zero.
3. Make verifyloop render text and an opt-in JSON plan from the same plan value. Keep diagnostics off JSON stdout and include explicit healthy/skipped/unreadable root coverage. Compose existing receipt/blocked/online/risk overlays with the canonical base classification and document precedence. Keep Evidence-only human work separate from authority to flip.
4. Use shared fixtures for board/JSON/planner agreement across every class and mixed roots. A test reading another module must join the existing cross-module registry and its triggering paths; do not silently make the cross-component contract advisory.

5. At implementation delivery, convert all 12 consumer entries that name this brief from `follow-up desk-tools/45` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 12 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

One canonical queue meaning, additive JSON fields and explicit unknown coverage; no new independent classifier or subprocess per row.

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
| 1 | `cd statusgen && go test ./... -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log"` | exit 0; output contains the named PASS line. Cases: all base buckets, paused/parked and unknown; existing JSON fields remain compatible; corrupt Evidence does not become an empty queue | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/deskboard -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log"` | exit 0; output contains the named PASS line. Cases: ./cmd/deskboard ./cmd/verifyloop: one fixture is consistently classified by both consumers; missing fields/roots suppress widening and preserve partial coverage | check:ci +mutation +flow +dereference |
| 3 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: ./cmd/deskboard ./cmd/verifyloop: one fixture is consistently classified by both consumers; missing fields/roots suppress widening and preserve partial coverage | check:ci +mutation +flow +dereference |
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/45 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 12 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 12 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 5 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-45-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
