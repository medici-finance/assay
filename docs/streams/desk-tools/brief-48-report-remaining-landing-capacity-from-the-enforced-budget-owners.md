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
design: DR-desk-tools-48
decision-issue: 5837
ruling: "https://github.com/medici-finance/assay/issues/5837#issuecomment-6899956912"
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
  - "tools/desk/cmd/deskroster/flow_48_test.go: fixed-here (planned test contract)"
  - "tools/desk/cmd/deskboard/flow_48_test.go: fixed-here (planned test contract)"
  - "tools/desk/internal/deskkit/ratelimit.go: fixed-here (this implementation brief)"
  - "tools/desk/internal/deskkit/ratelimitread.go: fixed-here (this implementation brief)"
  - "tools/desk/internal/deskkit/width.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/deskroster/main.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/deskboard/throughput.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/main.go: fixed-here (this implementation brief)"
  - "changelog/verify-landing-capacity.md: fixed-here (this implementation brief)"
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

Default if no answer: none — blocks until answered.

Hold the change.

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
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-48-report-remaining-landing-capacity-from-the-enforced-budget-owners.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/48 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 9 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 9 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-48-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

### Post-merge verification, 2026-11-18 (non-implementer verifier)

- Verified source: merged main 443583e3e95695126299217e3229fc9a1883a1b9. Statusgen v1.0.87. Risk line `customer: yes`, so every row owes a fail-first witness.
- Delivery: PR #5842, merge a3c678dee039d0e0adff3ea87311562b6e02890d. DELIVERY_HEAD (second parent) is 5e0a1ea12729e12b0926deeaa5c56ff74f24738f. DELIVERY_BASE (merge-base of the parents) is 9e563e164e7a89c53d32e90e29a8ed91cff58d71. The Verify table is unchanged from the delivery: re-derivation found 4 rows, 0 author-added.
- Rows 1, 2 and 4 (check:ci) ran in one container-netns admission bracket, with `--fail-first --delivering-commit a3c678dee039` (base 5638ed549a0c, the merge's first parent) and the history pack imported read-only. Bracket-digest verdict=match. The closed witness bytes follow verbatim.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go test ./cmd/deskroster -run "^TestFlowCapacity$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskroster.log" 2>&1 && grep -F -e "--- PASS: TestFlowCapacity " "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskroster.log"` | pass exit=0 sandbox=container-netns | sha256:f1ba8a5976c0 | 2026-11-18 | assay-verifier-app[bot] @ 443583e3e956 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=5638ed549a0c red rc=1 · head=443583e3e956 · delivering=a3c678dee039 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000001.log |
| 2 | `cd tools/desk && go test ./cmd/deskboard -run "^TestFlowCapacity$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskboard.log" 2>&1 && grep -F -e "--- PASS: TestFlowCapacity " "${TMPDIR:-/tmp}/flow-48-tools-desk-cmd-deskboard.log"` | pass exit=0 sandbox=container-netns | sha256:e791ab1a63cc | 2026-11-18 | assay-verifier-app[bot] @ 443583e3e956 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=5638ed549a0c red rc=1 · head=443583e3e956 · delivering=a3c678dee039 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000002.log |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-48-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-48-consumer-negative.log"` | pass exit=0 sandbox=container-netns | sha256:99a686106f74 | 2026-11-18 | assay-verifier-app[bot] @ 443583e3e956 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=5638ed549a0c green rc=0 · head=443583e3e956 · delivering=a3c678dee039 output=sha256:99a686106f7428bce301267e363c901ec2da3ee99d995d3af6d9dd0d7a34b038 log=row-000003.log |
Tree-Digest: sha256:fe8e8e618266a6b13d0cec5919807607574915bbe6dd6e796b77328e06eed723
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6935 run=79a49d837d5e2bd0ebe63cfc7a2f6eaafbc0c8715481eaa53657febc44cc6a83 source=443583e3e95695126299217e3229fc9a1883a1b9 brief=docs/streams/desk-tools/brief-48-report-remaining-landing-capacity-from-the-enforced-budget-owners.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=79a49d837d5e2bd0ebe63cfc7a2f6eaafbc0c8715481eaa53657febc44cc6a83 source=443583e3e95695126299217e3229fc9a1883a1b9 tree=sha256:fe8e8e618266a6b13d0cec5919807607574915bbe6dd6e796b77328e06eed723 witness=sha256:07c9d6a6f4ec41fa1a2bbeba4450f48880494c0f61ea4890f34c7d7ead0cf02c opened=2026-11-18T16:36:23Z closed=2026-11-18T16:40:46Z before=receipt after=check-verifier

Verifier accounting, one line per original row. The supplemental observations below get no acceptance credit.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | row 1 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line; container-netns witness above. Fail-first: red at base 5638ed549a0c (rc=1; the test does not exist there). Discriminating. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | row 2 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line; container-netns witness above. Fail-first: red at base 5638ed549a0c (rc=1). Discriminating. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | row 3 as in the Verify table, with DELIVERY_HEAD=5e0a1ea12729 and DELIVERY_BASE=9e563e164e7a | exit 0 at the exact delivery head; summary 9 corroborated, 0 disproved, 0 unchecked | could-not-run. The row requires HEAD equal to DELIVERY_HEAD, and the brief forbids moving the admitted home, which sits at 443583e3e956. In a separate checkout at DELIVERY_HEAD, host verifyrun refused before any row: "pre-work verifier admission failed; no Verify rows executed ... missing original pre-work verifier record". This is a check-definition / lane gap, not a deliverable failure. No fail-first witness is possible. | 2026-11-18 | assay-verifier-app[bot] (observed the refusal) (on-behalf-of human:ian) |
| 4 | row 4 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line, but **non-discriminating**. The row is green at base 5638ed549a0c (rc=0; the base log holds "--- PASS: TestConsumersGateFiresOnFalseFixedHere"). The named test and the statusgen consumers code are byte-identical between base and the verified source, so the row passes with or without the delivery. Under risk `customer: yes` this is a row-strength FAIL (check-definition). | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3-supp-head | the row 3 command by hand, in a separate checkout at DELIVERY_HEAD (no acceptance credit) | diagnostic only | rc=0; "summary: 9 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" (all 9 consumer sites CORROBORATED) | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3-supp-base | the row 3 command by hand, in a separate checkout at DELIVERY_BASE (no acceptance credit) | diagnostic only | rc=1; the HEAD precondition fails, and the consumers gate reports COULD-NOT-CHECK (brief not in the diff) | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Risk-bearing values: the enumeration covered the delivering diff of PR #5842 (deskkit ratelimit, ratelimitread and width; the deskroster capacity view; deskboard throughput; verifyloop help). No cap was introduced or changed: RateLimitPerPRPerHour = 20, RateLimitPerRepoPerHour = 100 and BreakerTrip = 5 are untouched, and the capacity view reads them from the gate plan. All entries below are reversible, read-only reporting knobs.

- RISK-VALUE: DERIVED — GatePR = -1 (pr-review-desk / deskpost) @ tools/desk/internal/deskkit/width.go:239, mapped to t.PR, t.SharedOnly = 1, true @ tools/desk/internal/deskkit/width.go:560. Each review item writes on a different PR, so no single per-PR bucket can be named at loop level. Reading only the shared (repo, tool, breaker) meters is the strongest meter set that is valid for every item. The value is not a cap.
- RISK-VALUE: DERIVED — GatePR = 0 (verify-desk / deskevidence) @ tools/desk/internal/deskkit/width.go:253, with WritesPerItem = 2 @ tools/desk/internal/deskkit/width.go:255. Evidence and outcome landings write the unnumbered bucket, and each item costs a row landing plus an outcome record (the verifyloop help text says two writes per item). GatePR = 0 for the deskpr loops (width.go:221, 271, 285) matches GatePRCreate's unnumbered new-PR bucket.
- RISK-VALUE: DERIVED — snap.Remaining = 0 @ tools/desk/internal/deskkit/ratelimit.go:1221. If no budget meter was read, the plan reports zero remaining capacity, never spare capacity. That is the brief's Task 2 fail-closed requirement.
- RISK-VALUE: NAMED, NOT DERIVED — --in-flight default = -1 @ tools/desk/cmd/deskroster/capacity.go:35 (assume every slot of the resolved width is unlanded). Whether full-width reservation is the right conservative default rather than a ledger-derived count is a policy choice. No spec states it, so it is routed to the human gate review.

**VERIFY: FAIL** — row 4 is non-discriminating: green at base 5638ed549a0c and pass at head. Risk is `customer: yes`, so this is a row-strength check-definition FAIL. Rows 1 and 2 pass and are discriminating (red at base). Row 3 could-not-run: its separate-checkout lane has no admission. Total: 2 of 4 rows passed. Status not flipped; gate is human.

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
