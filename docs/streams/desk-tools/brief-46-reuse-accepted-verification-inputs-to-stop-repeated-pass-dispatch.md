---
brief: assay:assay:desk-tools:46
title: "Reuse accepted verification inputs to stop repeated PASS dispatch"
why: "Failed and blocked receipts already avoid needless reruns. Successful work still needs a trustworthy input identity so an unchanged PASS can be continued instead of re-verified, while a changed deliverable is checked again."
wave: 2
depends: ["desk-tools/45"]
unblocks: ["desk-tools/47", "desk-tools/51"]
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
version: 1
id: a178d182-a26e-4f77-b1eb-bb405edc5f6d
consumers:
  - "tools/desk/cmd/verifyloop/flow_46_test.go: follow-up desk-tools/46 (planned test contract)"
  - "tools/desk/cmd/deskevidence/flow_46_test.go: follow-up desk-tools/46 (planned test contract)"
  - "tools/desk/cmd/verifyloop/briefscan.go: follow-up desk-tools/46 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/queueclass.go: follow-up desk-tools/46 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/blockerread.go: follow-up desk-tools/46 (this implementation brief)"
  - "tools/desk/internal/deskkit/verifywake.go: follow-up desk-tools/46 (this implementation brief)"
  - "tools/desk/internal/deskkit/verifyoutcomes.go: follow-up desk-tools/46 (this implementation brief)"
  - "tools/desk/cmd/deskevidence/outcomerecord.go: follow-up desk-tools/46 (this implementation brief)"
  - "changelog/verify-pass-inputs.md: follow-up desk-tools/46 (this implementation brief)"
---

# Brief 46 — Reuse accepted verification inputs to stop repeated PASS dispatch

## Context

files: tools/desk/cmd/verifyloop/briefscan.go, tools/desk/cmd/verifyloop/queueclass.go, tools/desk/cmd/verifyloop/blockerread.go, tools/desk/internal/deskkit/verifywake.go, tools/desk/internal/deskkit/verifyoutcomes.go, tools/desk/cmd/deskevidence/outcomerecord.go, changelog/verify-pass-inputs.md, tools/desk/cmd/deskevidence/flow_46_test.go, tools/desk/cmd/verifyloop/flow_46_test.go
facts:
- scanAwaitingIn already evaluates complete failed/blocked WakeReceipt records and carries hold/fire/unknown into queue classification.
- A verified sidecar plus nonempty Evidence currently produces stuck-flip without comparing the successful run with current declared inputs.
- The approved input identity is the executable Verify definition plus immutable tree identities for declared files. Existing receipt/witness provenance and independent-runner checks remain authoritative.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "Reuse is scheduling evidence bound to immutable inputs and an accepted independent run; it grants no verified/done transition by itself."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Extend the existing accepted outcome/receipt representation for successful runs; do not add a competing verified-at log or scheduling receipt. Record the exact Verify definition, declared input revisions, applicable tool version, source run/witness, runner and source revision.
2. A current independently accepted PASS with matching inputs becomes continuation work, not fresh model dispatch. Changed table, declared file tree, relevant sibling input or applicable tool must invalidate reuse. Unknown/unreadable inputs or untrusted/ambiguous provenance cannot prove unchangedness.
3. Preserve the existing failed/blocked wake behavior, partial-row holds, explicit recheck, future-clock guard and legacy receipt handling. Legacy records may be reclassified; never invent missing provenance or hash old Evidence as if it were a new execution.
4. Honor the approved table-only fallback for a brief with no files declaration, marking the incomplete scope in the plan. Do not call that fallback equivalent to full deliverable coverage. Cleanup/backfill belongs to the existing notice-debt operation.
5. Test a second independently planted changed input and a forged same-runner PASS. A copied success marker or a changed source with a stable display SHA must not suppress verification.

## Interface contract

Reuse is scheduling evidence bound to immutable inputs and an accepted independent run; it grants no verified/done transition by itself.

## Ground rules

- Isolated branch and draft PR; no merge, main push, workflow activation or live infrastructure query in this brief.
- Existing narrow landing permissions apply at runtime; a refusal stops the operation and never licenses another identity or channel.
- Do not rewrite old Evidence, invent a witness, close real gate cards or fabricate a human stamp.
- Stop at implementation; independent verification owns acceptance Evidence.

## Verify

New test names below are planned contracts. The named PASS assertion must fail if a selector runs no test. Run commands from the repository root.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowPassInputs$" -count=1 -v > "${TMPDIR:-/tmp}/flow-46-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowPassInputs " "${TMPDIR:-/tmp}/flow-46-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: matching accepted PASS yields continuation; table/file/tool changes dispatch again; unreadable input is unknown; forged or legacy PASS cannot claim acceptance; failed/partial wake behavior is preserved | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/deskevidence -run "^TestFlowPassInputs$" -count=1 -v > "${TMPDIR:-/tmp}/flow-46-tools-desk-cmd-deskevidence.log" 2>&1 && grep -F -e "--- PASS: TestFlowPassInputs " "${TMPDIR:-/tmp}/flow-46-tools-desk-cmd-deskevidence.log"` | exit 0; output contains the named PASS line. Cases: matching accepted PASS yields continuation; table/file/tool changes dispatch again; unreadable input is unknown; forged or legacy PASS cannot claim acceptance; failed/partial wake behavior is preserved | check:ci +mutation +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; the implementation diff corroborates declared routing | check +flow |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
