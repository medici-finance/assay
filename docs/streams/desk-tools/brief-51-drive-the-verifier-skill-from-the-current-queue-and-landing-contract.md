---
brief: assay:assay:desk-tools:51
title: "Drive the verifier skill from the current queue and landing contract"
why: "The skill still asks agents to assemble their own answer from scattered mechanics. Once the tool contract exists, a short lane-to-action table should direct each run through the implemented path."
wave: 4
depends: ["desk-tools/45", "desk-tools/46", "desk-tools/47", "desk-tools/48", "desk-tools/49", "desk-tools/50", "desk-tools/52"]
unblocks: []
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
id: af2553b5-b208-4d3d-aab1-5d8c518b2add
consumers:
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up desk-tools/51 (register the skill-body cross-module read; existing CI already runs on every push and pull request)"
  - "tools/desk/cmd/verifyloop/flow_51_test.go: follow-up desk-tools/51 (planned test contract)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/cmd/deskdispatch/references/verifier-prompt.md: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/cmd/verifyloop/main.go: follow-up desk-tools/51 (this implementation brief)"
  - "tools/desk/README.md: follow-up desk-tools/51 (this implementation brief)"
  - "changelog/verifier-lane-actions.md: follow-up desk-tools/51 (this implementation brief)"
---

# Brief 51 — Drive the verifier skill from the current queue and landing contract

## Context

files: plugins/assay/skills/verify-desk/SKILL.md, tools/desk/cmd/deskdispatch/references/verifier-prompt.md, tools/desk/cmd/verifyloop/main.go, tools/desk/README.md, tools/desk/internal/deskkit/citrigger_test.go, changelog/verifier-lane-actions.md, tools/desk/cmd/verifyloop/flow_51_test.go
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

Default if no answer: hold.

## Task

1. Replace contradictory mechanics in the landing/autonomy sections with one lane-to-owner/action table bound to the implemented plan fields. Show dispatch, Evidence-only, accepted continuation, human gate, online, repair, wait and unknown states.
2. Name the actual existing flip command and profile-specific sanctioned Evidence path. Respect model-only flip exclusions, human-only acceptance and CI-owned model completion. A denied channel stops; it is not a request for another identity or a wider carve-out.
3. Wire the verifier prompt to the run-bound prerequisite record and canonical case identity. Tell the caller to attach to the returned existing obligation and to use the plan’s next act rather than reconstruct a gate from narrative.
4. Test rendered actions against the same fixture plans and CLI help, including stale source, unreadable roots and a human/risk item. Preserve generated guardrail blocks and run the existing skill parity/lint procedure. Register any cross-module or skill-body fixture reads and their CI triggering paths in the existing cross-module registry.
5. Document the minimum released paired tool versions and rollout/reversal requirements. Source merge alone must not instruct a consumer of an older pin to invoke a missing mode.

## Interface contract

One table derived from implemented actions and permissions, never a second dispatch policy or activation instruction.

## Ground rules

- Isolated branch and draft PR; no merge, main push, workflow activation or live infrastructure query in this brief.
- Existing narrow landing permissions apply at runtime; a refusal stops the operation and never licenses another identity or channel.
- Do not rewrite old Evidence, invent a witness, close real gate cards or fabricate a human stamp.
- Stop at implementation; independent verification owns acceptance Evidence.

## Verify

New test names below are planned contracts. The named PASS assertion must fail if a selector runs no test. Run commands from the repository root.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowSkill$" -count=1 -v > "${TMPDIR:-/tmp}/flow-51-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowSkill " "${TMPDIR:-/tmp}/flow-51-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: each fixture plan resolves to an existing supported action; human/Evidence-only cases never suggest a model flip; stale prerequisite and missing release capability yield a hold | check:ci +mutation +flow +dereference |
| 2 | `statusgen --root . --consumers` | exit 0; the implementation diff corroborates declared routing | check +flow |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
