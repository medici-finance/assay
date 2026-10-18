---
brief: assay:assay:desk-tools:49
title: "Deduplicate verifier filings by their canonical work identity"
why: "Title similarity already blocks some duplicate issues, but different wording can file the same verification failure again. Exact existing work identity should lead the verifier to its owned obligation without losing distinct findings."
wave: 1
depends: []
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
version: 1
id: 2aad844d-9d46-42bb-8597-6a27eba517ad
consumers:
  - "tools/desk/cmd/verifyloop/flow_49_test.go: follow-up desk-tools/49 (planned test contract)"
  - "tools/desk/cmd/deskfile/flow_49_test.go: follow-up desk-tools/49 (planned test contract)"
  - "tools/desk/cmd/deskfile/deskfile.go: follow-up desk-tools/49 (this implementation brief)"
  - "tools/desk/cmd/deskfile/matcher.go: follow-up desk-tools/49 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/repair.go: follow-up desk-tools/49 (this implementation brief)"
  - "tools/desk/internal/deskkit/repairobligation.go: follow-up desk-tools/49 (this implementation brief)"
  - "changelog/verifier-filing-key.md: follow-up desk-tools/49 (this implementation brief)"
---

# Brief 49 — Deduplicate verifier filings by their canonical work identity

## Context

files: tools/desk/cmd/deskfile/deskfile.go, tools/desk/cmd/deskfile/matcher.go, tools/desk/cmd/verifyloop/repair.go, tools/desk/internal/deskkit/repairobligation.go, changelog/verifier-filing-key.md, tools/desk/cmd/deskfile/flow_49_test.go, tools/desk/cmd/verifyloop/flow_49_test.go
facts:
- deskfile already performs title-based dedupe and fails closed on a failed search; keep that legacy layer.
- verifyloop already derives RepairObligation identity from repo, brief, source receipt and failing rows. Its real durable sink remains separately activation-gated.
- Role provenance is not a human-approval stamp. Canonical brief IDs must resolve through the existing registry rather than a hardcoded alias map.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "One exact identity before existing fuzzy matching; no new global suppress-all-on-brief rule or automatic issue closure."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Before title matching, reuse the existing repair/case identity where present and add an exact canonical brief plus verifier-role lookup for repeated observations of the same case. Reconcile to the existing issue/obligation and print its number.
2. Scope the search to the actual repository and role/case. Same numbers in another root, unrelated defect classes on one brief and a different source receipt must not be suppressed as if already addressed; attach additional evidence only when the existing obligation covers it.
3. Keep legacy title dedupe for filings without a structured identity and retain the current audited force-new reason path. A failed or truncated exact read is unknown and files nothing; an ambiguous response is reconciled before another create.
4. Extend the existing typed forge seam and caller; do not add a private issue cache or activate the dry-run repair sink. Test varied titles for one case, a second independent case and cross-root collisions.

## Interface contract

One exact identity before existing fuzzy matching; no new global suppress-all-on-brief rule or automatic issue closure.

## Ground rules

- Isolated branch and draft PR; no merge, main push, workflow activation or live infrastructure query in this brief.
- Existing narrow landing permissions apply at runtime; a refusal stops the operation and never licenses another identity or channel.
- Do not rewrite old Evidence, invent a witness, close real gate cards or fabricate a human stamp.
- Stop at implementation; independent verification owns acceptance Evidence.

## Verify

New test names below are planned contracts. The named PASS assertion must fail if a selector runs no test. Run commands from the repository root.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskfile -run "^TestFlowFileKey$" -count=1 -v > "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-deskfile.log" 2>&1 && grep -F -e "--- PASS: TestFlowFileKey " "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-deskfile.log"` | exit 0; output contains the named PASS line. Cases: same case under unrelated titles is deduped; distinct cases and cross-root same numbers remain separate; unreadable/partial search creates nothing; audited override still records its reason | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowFileKey$" -count=1 -v > "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowFileKey " "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: same case under unrelated titles is deduped; distinct cases and cross-root same numbers remain separate; unreadable/partial search creates nothing; audited override still records its reason | check:ci +mutation +flow +dereference |
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
