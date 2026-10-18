---
brief: assay:assay:desk-tools:47
title: "Resume interrupted Evidence landing through the existing sanctioned paths"
why: "A successful run can land Evidence and still leave its lifecycle transition unfinished. Repeating verification does not complete that write; an idempotent continuation must resume the remaining authorized effect."
wave: 3
depends: ["desk-tools/46"]
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
id: c2ff642d-db3c-47d3-a49d-03b05fbb0dde
consumers:
  - "tools/desk/cmd/verifyloop/flow_47_test.go: follow-up desk-tools/47 (planned test contract)"
  - "tools/desk/cmd/deskevidence/flow_47_test.go: follow-up desk-tools/47 (planned test contract)"
  - "statusgen/flow_47_test.go: follow-up desk-tools/47 (planned test contract)"
  - "tools/desk/cmd/deskevidence/deskevidence.go: follow-up desk-tools/47 (this implementation brief)"
  - "tools/desk/cmd/deskevidence/flip.go: follow-up desk-tools/47 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/land.go: follow-up desk-tools/47 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/durable.go: follow-up desk-tools/47 (this implementation brief)"
  - "statusgen/verifyflip.go: follow-up desk-tools/47 (this implementation brief)"
  - "changelog/verify-continuation.md: follow-up desk-tools/47 (this implementation brief)"
---

# Brief 47 — Resume interrupted Evidence landing through the existing sanctioned paths

## Context

files: tools/desk/cmd/deskevidence/deskevidence.go, tools/desk/cmd/deskevidence/flip.go, tools/desk/cmd/verifyloop/land.go, tools/desk/cmd/verifyloop/durable.go, statusgen/verifyflip.go, changelog/verify-continuation.md, statusgen/flow_47_test.go, tools/desk/cmd/deskevidence/flow_47_test.go, tools/desk/cmd/verifyloop/flow_47_test.go
facts:
- deskevidence flip and statusgen verifyflip already derive a model-only branch commit from strict accepted PASS provenance and reject human/risk-bearing transitions.
- The ordinary Evidence writer accepts one file per invocation. verifyloop defaults to a dry-run sink; the real main-writing sink remains activation-gated.
- desk-tools/30 already owns the separate verdict-transcription migration. This brief neither activates that lane nor retires the existing carve-out.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "One resumable operation in the existing Evidence owner, with explicit receipts per effect and no additional authority."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Extend the existing Evidence tool/landing adapter with a persisted continuation keyed to the accepted run and its input identity. Re-read which effects actually landed before resuming; do not trust a local success printout or redo completed writes.
2. For PR-only repositories, use the existing derived model-only flip and draft-PR path. A main landing is available only under that repository’s existing explicit narrow authorization. Unknown profile or any refusal is a hold, never permission to switch identity, endpoint or write channel.
3. Keep gate:human, any risk yes, irreversible and human-stamp exclusions of verifyflip intact. Evidence-only work may land under its existing permission; it cannot turn into a model-authored Reviewed stamp or a human gate closure.
4. Exercise interruption before Evidence, after accepted Evidence, after local flip and after uncertain remote response. Resume must reconcile exact content/commit receipts, preserve accepted runner attribution, add no PROBLEM, and never enable gitDurable by default.
5. Record actual effect cost for capacity planning, with distinct completed/remaining/unknown effects. Preserve the existing publication, admission, append-only and before/after lint checks.

6. At implementation delivery, convert all 9 consumer entries that name this brief from `follow-up desk-tools/47` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 9 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

One resumable operation in the existing Evidence owner, with explicit receipts per effect and no additional authority.

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
| 1 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowResume$" -count=1 -v > "${TMPDIR:-/tmp}/flow-47-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowResume " "${TMPDIR:-/tmp}/flow-47-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: crash at each boundary, same-run retry is idempotent, uncertain write is reconciled, denied channel stops, branch profile does not touch main, risk/human cases remain Evidence-only | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/deskevidence -run "^TestFlowResume$" -count=1 -v > "${TMPDIR:-/tmp}/flow-47-tools-desk-cmd-deskevidence.log" 2>&1 && grep -F -e "--- PASS: TestFlowResume " "${TMPDIR:-/tmp}/flow-47-tools-desk-cmd-deskevidence.log"` | exit 0; output contains the named PASS line. Cases: crash at each boundary, same-run retry is idempotent, uncertain write is reconciled, denied channel stops, branch profile does not touch main, risk/human cases remain Evidence-only | check:ci +mutation +flow +dereference |
| 3 | `cd statusgen && go test ./... -run "^TestFlowFlipBoundary$" -count=1 -v > "${TMPDIR:-/tmp}/flow-47-statusgen-allallall.log" 2>&1 && grep -F -e "--- PASS: TestFlowFlipBoundary " "${TMPDIR:-/tmp}/flow-47-statusgen-allallall.log"` | exit 0; output contains the named PASS line. Cases: strict PASS/provenance/sha/runner validation and all human/risk exclusions survive the continuation path | check:ci +mutation +flow +dereference |
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up desk-tools/47' docs/streams/desk-tools/brief-47-resume-interrupted-evidence-landing-through-the-existing-sanctioned-paths.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/47 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 9 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no self-deferred route, and all 9 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 5 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-47-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-47-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
