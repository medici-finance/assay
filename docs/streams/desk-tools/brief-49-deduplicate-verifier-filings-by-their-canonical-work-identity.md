---
brief: assay:assay:desk-tools:49
title: "Deduplicate verifier filings by their canonical work identity"
why: "Title similarity already blocks some duplicate issues, but different wording can file the same verification failure again. Exact existing work identity should lead the verifier to its owned obligation without losing distinct findings."
wave: 1
depends: []
unblocks: ["desk-tools/51", "verification-handoff/02"]
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
id: 2aad844d-9d46-42bb-8597-6a27eba517ad
consumers:
  - "tools/desk/cmd/verifyloop/flow_49_test.go: fixed-here (planned test contract)"
  - "tools/desk/cmd/deskfile/flow_49_test.go: fixed-here (planned test contract)"
  - "tools/desk/cmd/deskfile/deskfile.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/deskfile/matcher.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/repair.go: fixed-here (this implementation brief)"
  - "tools/desk/internal/deskkit/repairobligation.go: fixed-here (this implementation brief)"
  - "changelog/verifier-filing-key.md: fixed-here (this implementation brief)"
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

Default if no answer: none — blocks until answered.

Hold the change.

## Task

1. Before title matching, reuse the existing repair/case identity where present and add an exact canonical brief plus verifier-role lookup for repeated observations of the same case. Reconcile to the existing issue/obligation and print its number.
2. Scope the search to the actual repository and role/case. Same numbers in another root, unrelated defect classes on one brief and a different source receipt must not be suppressed as if already addressed; attach additional evidence only when the existing obligation covers it.
3. Keep legacy title dedupe for filings without a structured identity and retain the current audited force-new reason path. A failed or truncated exact read is unknown and files nothing; an ambiguous response is reconciled before another create.
4. Extend the existing typed forge seam and caller; do not add a private issue cache or activate the dry-run repair sink. Test varied titles for one case, a second independent case and cross-root collisions.

5. At implementation delivery, convert all 7 consumer entries that name this brief from `follow-up desk-tools/49` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 7 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

One exact identity before existing fuzzy matching; no new global suppress-all-on-brief rule or automatic issue closure.

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
| 1 | `cd tools/desk && go test ./cmd/deskfile -run "^TestFlowFileKey$" -count=1 -v > "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-deskfile.log" 2>&1 && grep -F -e "--- PASS: TestFlowFileKey " "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-deskfile.log"` | exit 0; output contains the named PASS line. Cases: same case under unrelated titles is deduped; distinct cases and cross-root same numbers remain separate; unreadable/partial search creates nothing; audited override still records its reason | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowFileKey$" -count=1 -v > "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowFileKey " "${TMPDIR:-/tmp}/flow-49-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: same case under unrelated titles is deduped; distinct cases and cross-root same numbers remain separate; unreadable/partial search creates nothing; audited override still records its reason | check:ci +mutation +flow +dereference |
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-49-deduplicate-verifier-filings-by-their-canonical-work-identity.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/49 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 7 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 7 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-49-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-49-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
