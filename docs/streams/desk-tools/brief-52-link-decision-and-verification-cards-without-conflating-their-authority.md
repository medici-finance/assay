---
brief: assay:assay:desk-tools:52
title: "Link decision and verification cards without conflating their authority"
why: "A design decision and final verification sign-off can concern one brief but authorize different acts. Typed identity and purpose links must make the next act clear without turning an earlier ruling into acceptance."
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
version: 2
id: bc0e7a98-87bb-4a7f-9081-a31ac9a34ab4
consumers:
  - "statusgen/flow_52_test.go: follow-up desk-tools/52 (planned test contract)"
  - "statusgen/decisionissues.go: follow-up desk-tools/52 (this implementation brief)"
  - "statusgen/verifyissues.go: follow-up desk-tools/52 (this implementation brief)"
  - "statusgen/briefv2.go: follow-up desk-tools/52 (this implementation brief)"
  - "statusgen/decisiongatehold.go: follow-up desk-tools/52 (this implementation brief)"
  - "changelog/verification-card-links.md: follow-up desk-tools/52 (this implementation brief)"
---

# Brief 52 — Link decision and verification cards without conflating their authority

## Context

files: statusgen/decisionissues.go, statusgen/verifyissues.go, statusgen/briefv2.go, statusgen/decisiongatehold.go, changelog/verification-card-links.md, statusgen/flow_52_test.go
facts:
- The current consumer closer already accepts legacy and canonical v2 verify-gate markers. A parser rewrite from the September plan is not owed.
- Decision filing and verify-gate cards currently have different marker/purpose owners. An approved design is not proof that the later Verify table passed.
- The human-only close path and its exact source/run provenance remain the only authority for a human acceptance stamp.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "Backward-compatible card links with explicit purpose; no migration of human authority or replay of historical closes."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: hold.

## Task

1. Add canonical brief identity, gate purpose and relevant source/verification revision links to the existing emitter contract. Keep old markers readable; make purpose explicit to consumers and the desk.
2. Link existing decision and verification records for one brief. Preserve distinct records when they authorize distinct acts; do not blindly copy a verify marker onto a design decision or treat a closed earlier decision as final sign-off.
3. If a combined card is supported, require explicit same human-authorized act, matching brief/revision and an independently satisfied verification prerequisite. Unknown, foreign alias, mixed revisions or forged bot close cannot advance the lifecycle.
4. Cover legacy IDs, v2 IDs, optional letter suffixes, cross-repo aliases, purpose mismatch, old closed decisions and malformed/malicious marker text with shared conformance fixtures. Do not reopen or close real historical issues from this implementation.

5. At implementation delivery, convert all 6 consumer entries that name this brief from `follow-up desk-tools/52` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 6 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

Backward-compatible card links with explicit purpose; no migration of human authority or replay of historical closes.

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
| 1 | `cd statusgen && go test ./... -run "^TestFlowCard$" -count=1 -v > "${TMPDIR:-/tmp}/flow-52-statusgen-allallall.log" 2>&1 && grep -F -e "--- PASS: TestFlowCard " "${TMPDIR:-/tmp}/flow-52-statusgen-allallall.log"` | exit 0; output contains the named PASS line. Cases: legacy/v2 linkage round-trips; decision-only close cannot advance verification; mismatched revision, foreign repository and bot-origin close fail; an authorized exact sign-off remains usable | check:ci +mutation +flow +dereference |
| 2 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-52-link-decision-and-verification-cards-without-conflating-their-authority.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/52 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 6 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 6 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 3 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-52-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-52-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
