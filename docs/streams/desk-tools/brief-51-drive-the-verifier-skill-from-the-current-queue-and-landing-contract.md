---
brief: assay:assay:desk-tools:51
title: "Drive the verifier skill from the current queue and landing contract"
why: "The skill still asks agents to assemble their own answer from scattered mechanics. Once the tool contract exists, a short lane-to-action table should direct each run through the implemented path."
wave: 4
depends: ["desk-tools/45", "desk-tools/46", "desk-tools/47", "desk-tools/48", "desk-tools/49", "desk-tools/50", "desk-tools/52"]
unblocks: ["graph-execution/27"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "Changes verification scheduling/landing behavior relied on by adopters; human review must confirm the preserved provenance and authority boundaries."
decision-trigger: start
issues: []
schema: brief-v2
authored: "2026-10-17 maintainer-directed verify-flow refresh"
sources:
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - "docs/streams/desk-tools/README.md"
  - "freshness-checked 2026-10-17 @ 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae"
exec-tier: strong
exec-tier-why: "Cross-component queue, input provenance and effect boundaries must agree; a subtle error can suppress needed verification or advance the wrong state."
domain: complicated
outcome: none
version: 4
id: af2553b5-b208-4d3d-aab1-5d8c518b2add
consumers:
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up desk-tools/51 (register the skill-body cross-module read; existing CI already runs on every push and pull request)"
  - "tools/desk/cmd/verifyloop/flow_51_test.go: follow-up desk-tools/51 (planned test contract)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/cmd/deskdispatch/references/verifier-prompt.md: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/repair.go: follow-up desk-tools/51 (owned Evidence-PR inventory and repair discovery)"
  - "tools/desk/cmd/verifyloop/main.go: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/README.md: follow-up desk-tools/51 (this implementation brief)"
  - "changelog/verifier-lane-actions.md: follow-up desk-tools/51 (this implementation brief)"
---

# Brief 51 — Drive the verifier skill from the current queue and landing contract

## Context

files: tools/desk/cmd/verifyloop/repair.go, plugins/assay/skills/verify-desk/SKILL.md, tools/desk/cmd/deskdispatch/references/verifier-prompt.md, tools/desk/cmd/verifyloop/main.go, tools/desk/README.md, tools/desk/internal/deskkit/citrigger_test.go, changelog/verifier-lane-actions.md, tools/desk/cmd/verifyloop/flow_51_test.go
facts:
- The current skill already documents one-file Evidence landing, a PR-required lane and the separately gated verdict-transcription future. Preserve those permissions.
- The plan already has explicit wait and Evidence-only states. No unconditional rule that all human/risk items are model-flippable is valid.
- The existing desk-tools/30 migration remains separate. Publication of this skill changes neither activation state nor repository write scopes.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "One table derived from implemented actions and permissions, never a second dispatch policy or activation instruction."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: none — blocks until answered.

Hold the change.

## Candidate and priority amendment — 2026-10-23

Make the verifier prose recipe consume the same plan as the discrete client, including the
full configured root set, explicit current policy/time/capacity and the separately owned
Evidence-PR repair population. Use one multi-root plan per pass so shared blockers/receipts
reuse the existing pass caches; changing a local slot does not itself force another fleet scan.
Own the missing repair discovery in verifyloop/main.go and verifyloop/repair.go: inventory
owned Evidence PRs across the admitted repositories, read current head/review/check and
mergeability facts through existing typed forge adapters, and route correction work through
the existing sanctioned worker path. Reconcile accepted-run continuation with /47 without
treating every authored PR as accepted Evidence. Keep incomplete/stale inventory unknown;
no exact GH saving is claimed until this reader’s coverage and request counts are measured.
Preserve final pending-hold head checks and all landing gates. Compare the prose recipe and current CLI in the
already-declared flow_51_test surface; graph-execution/27 subsequently proves its discrete binding and document qualified pins before changing live recipes.

## Task

The repair-reader extension above is part of this brief’s implementation Task and its
flow_51 acceptance, not an implicit obligation of /45 or /47. The added repair.go consumer
raises this amended delivery contract from seven to eight corroborated sites.

1. Replace contradictory mechanics in the landing/autonomy sections with one lane-to-owner/action table bound to the implemented plan fields. Show dispatch, Evidence-only, accepted continuation, human gate, online, repair, wait and unknown states.
2. Name the actual existing flip command and profile-specific sanctioned Evidence path. Respect model-only flip exclusions, human-only acceptance and CI-owned model completion. A denied channel stops; it is not a request for another identity or a wider carve-out.
3. Wire the verifier prompt to the run-bound prerequisite record and canonical case identity. Tell the caller to attach to the returned existing obligation and to use the plan’s next act rather than reconstruct a gate from narrative.
4. Test rendered actions against the same fixture plans and CLI help, including stale source, unreadable roots and a human/risk item. Preserve generated guardrail blocks and run the existing skill parity/lint procedure. Register any cross-module or skill-body fixture reads and their CI triggering paths in the existing cross-module registry.
5. Document the minimum released paired tool versions and rollout/reversal requirements. Source merge alone must not instruct a consumer of an older pin to invoke a missing mode.

6. At implementation delivery, convert all 8 consumer entries that name this brief from `follow-up desk-tools/51` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 8 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

One table derived from implemented actions and permissions, never a second dispatch policy or activation instruction.

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
| 1 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowSkill$" -count=1 -v > "${TMPDIR:-/tmp}/flow-51-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowSkill " "${TMPDIR:-/tmp}/flow-51-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: each fixture plan resolves to an existing supported action; human/Evidence-only cases never suggest a model flip; stale prerequisite and missing release capability yield a hold | check:ci +mutation +flow +dereference |
| 2 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-51-drive-the-verifier-skill-from-the-current-queue-and-landing-contract.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/51 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 8 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 8 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 3 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-51-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-51-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestVerifyDriverPlan$' ./... > "$o" && grep -F -- '--- PASS: TestVerifyDriverPlan ' "$o")` | exit 0; named PASS. prose recipe and current verifyloop CLI consume one equal plan, including repair work and second-root candidates. graph-execution/27 subsequently accepts the discrete binding; this test does not depend on that later driver. Owned Evidence PR inventory, changed head, review/check/mergeability and correction routing execute through the production repair reader with fake transport; missing inventory stays unknown and bypassing repair discovery fails. Four roots sharing blocker keys read each distinct key once per pass; moved held-head and partial coverage still block. A repeated single-root sweep or skipped final check fails. | check:ci +flow +mutation |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
