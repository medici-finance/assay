---
brief: assay:assay:desk-tools:45
title: "Expose the canonical verification queue to its consumers"
why: "The board computes owned verification queues, but the JSON seam drops that classification and throughput counts all implemented briefs. One shared contract must distinguish actual model work, continuation, human action and unreadable state."
wave: 1
depends: []
unblocks: ["desk-tools/46", "desk-tools/48", "desk-tools/51", "graph-execution/27"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "Changes verification scheduling/landing behavior relied on by adopters; human review must confirm the preserved provenance and authority boundaries."
decision-trigger: start
decision-issue: 3382
design: DR-desk-tools-45
ruling: "https://github.com/medici-finance/assay/issues/3382#issuecomment-6258267531"
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

Default if no answer: none — blocks until answered.

Hold the change.

## Candidate and priority amendment — 2026-10-23

Expose the current verification base classification as the same pure candidate-plus-policy
input consumed by the prose skill and graph-execution/27's discrete role binding. Keep this
brief's existing queue taxonomy, overlay precedence and approved gate/ruling; no new classifier.
Use the complete multi-root coverage result and explicit policy/time/capacity; a lone --root
plan cannot prove all roots idle. graph-execution/27 is an additional consumer of the existing
planned API, not a reason to import statusgen package main. /47 retains accepted-run landing continuation; /51 owns Evidence-PR repair
discovery and actual skill integration; the candidate observation must show that owed work
rather than infer a drained desk from an empty Awaiting subset. Reuse existing flow_45_test
surfaces and register any new cross-module read. No active attempt is rebound by this document.

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

Consumer-diff provenance (command amendment for #4905): row 4 pins the landed implementation merge from PR #4900, `27bbd0f0430ab7b234efb2bbda7ff18ed1d105f2`, and its exact first parent, `6393b774630c612714639eb1e369388547f8168b`. The native `--delivery` mode reads the immutable delivered tree and checks all twelve declared routes against that bounded delivery diff, including inherited claims. A later amendment, routing-only change, empty diff or wider history range cannot substitute for this implementation. Run the row from the admitted execution checkout that contains the delivery; do not move an admitted verifier home to a feature-branch head.

The consumer diff's first-parent base is distinct from the acceptance table's divergence-base coordinate in `Verification-Table:`. Derive that protected record and the separately landed version-4 amendment approval under `docs/protected-paths.md`; neither this source edit nor its merge grants acceptance. Record the immutable coordinates and actual output in fresh independent Evidence only after approval. The literal twelve-site expectation and row 5's negative control are unchanged; unchecked residue is never a pass.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log"` | exit 0; output contains the named PASS line. Cases: all base buckets, paused/parked and unknown; existing JSON fields remain compatible; corrupt Evidence does not become an empty queue | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/deskboard -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log"` | exit 0; output contains the named PASS line. Cases: ./cmd/deskboard ./cmd/verifyloop: one fixture is consistently classified by both consumers; missing fields/roots suppress widening and preserve partial coverage | check:ci +mutation +flow +dereference |
| 3 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: ./cmd/deskboard ./cmd/verifyloop: one fixture is consistently classified by both consumers; missing fields/roots suppress widening and preserve partial coverage | check:ci +mutation +flow +dereference |
| 4 | `git merge-base --is-ancestor 27bbd0f0430ab7b234efb2bbda7ff18ed1d105f2 HEAD && test "$(git rev-parse '27bbd0f0430ab7b234efb2bbda7ff18ed1d105f2^1')" = "6393b774630c612714639eb1e369388547f8168b" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/45 --delivery 27bbd0f0430ab7b234efb2bbda7ff18ed1d105f2 --base 6393b774630c612714639eb1e369388547f8168b > "$o" 2>&1 && grep -F -e 'summary: 12 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 12 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 5 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-45-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |
| 6 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestVerifyPlanParity$' ./... > "$o" && grep -F -- '--- PASS: TestVerifyPlanParity ' "$o")` | exit 0; named PASS. canonical observation and plan values match an independent base-queue oracle under explicit policy, with second-root work visible. Missing repair-source coverage is unknown and cannot establish whole-desk idle. Unsupported base contracts and copied classifiers fail. Actual repair/skill integration belongs to dependent /51 and discrete integration to graph-execution/27, not this upstream test. Existing positive consumer-count proof remains unchanged. | check:ci +flow +mutation |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->
### 2026-10-30 — independent post-merge verification

Non-implementer verifier run under admission `run=6d588edf42fc` on source `5fdae0d07b58` (merged main containing the PR #4900 delivery `27bbd0f0430a` and the PR #4932 row 4 amendment `c0327f233dd1`). `deskdispatch --check-verifier` exited 0 before any row ran and again after this Evidence edit. `deskpathguard rederive` re-derived all six rows from base, with zero author-added rows.

Container lane, rows 1, 2, 3, 5 and 6: the darwin desk host has no `unshare` network sandbox, so these `check:ci` rows ran inside one admission bracket in the pinned verify-runner image v1.0.67 (image id below) with `--network none`, the source and its git directory mounted read-only, `GOPROXY=off`, `GOFLAGS=-mod=readonly`, `KUBECONFIG=/dev/null` and no forge token in the environment. Every row exited 0 under `sandbox=container-netns`, and `--bracket-digest` reported `verdict=match`. The primary logs show the named PASS lines with every listed subtest passing: statusgen TestFlowQueue (6 subtests), deskboard TestFlowQueue (7), verifyloop TestFlowQueue (7, including `landing_guard_holds_with_planner_bypassed`), the consumer negative control (DISPROVED then CORROBORATED), and TestVerifyPlanParity.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test ./... -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-statusgen-allallall.log"` | pass exit=0 sandbox=container-netns | sha256:830ed34557cd | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskboard -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-deskboard.log"` | pass exit=0 sandbox=container-netns | sha256:d505e861a6a1 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowQueue$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowQueue " "${TMPDIR:-/tmp}/flow-45-tools-desk-cmd-verifyloop.log"` | pass exit=0 sandbox=container-netns | sha256:203780a1b274 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-45-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-45-consumer-negative.log"` | pass exit=0 sandbox=container-netns | sha256:1004d9ad5c89 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestVerifyPlanParity$' ./... > "$o" && grep -F -- '--- PASS: TestVerifyPlanParity ' "$o")` | pass exit=0 sandbox=container-netns | sha256:a8493cc13e57 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da
Container-Image: sha256:b4c22f9a7914874a0002b2205532cfaa1eee499fd11a65ceaabee20e86dc5e4f
Verification-Attestation: medici-finance/assay#5002 run=6d588edf42fc64c832b1cdfce9feb5c25987aaf2006f44af4fe4eee04eba840a source=5fdae0d07b58aba9042b112258727ac33862bd93 brief=docs/streams/desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=6d588edf42fc64c832b1cdfce9feb5c25987aaf2006f44af4fe4eee04eba840a source=5fdae0d07b58aba9042b112258727ac33862bd93 tree=sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da witness=sha256:018d12e3a8ee39543cd2d7dedd9b79997b8f9e5dbad0c735b551ea61fc90b16a opened=2026-10-30T17:42:00Z closed=2026-10-30T17:42:51Z before=receipt after=check-verifier

- **Row 4** (2026-10-30): `statusgen verifyrun --root . --brief docs/streams/desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md --acceptance-record <descriptor> --acceptance-forge --rows 4` runs on the host as the single protected transition invocation, with no forge token variable set. Expected: the original-plus-approved obligation plan runs row 4 and prints its witness. Observed: exit 2 before any row ran, with `protected table: acceptance transition: human approval could not be corroborated: fetching comment 6512561669 on medici-finance/assay: HTTP transport error: could-not-check: no forge token variable is set and the stored forge login is not used inside an admitted verifyrun`. The descriptor names delivery `27bbd0f0430ab7b234efb2bbda7ff18ed1d105f2`, base `1304d40f5c32cae5136e87f6b1f0cbe5a008c72a`, original `1304d40f5c32cae5136e87f6b1f0cbe5a008c72a` and amendment `c0327f233dd19132a59e20ab815ba059578db4ae`. It cites the approval https://github.com/medici-finance/assay/issues/4905#issuecomment-6512561669 (rows=4). That comment was posted under the driver's own login and has not been edited. The verifier was dispatched without a forge token, so row 4 has no witness and earns no credit (tracked by #4905).
- Supplemental, no credit, discharges no row (2026-10-30): the row 4 command ran by hand on the host from the same source, outside verifyrun. It exited 0 and printed `summary: 12 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. This is a reading aid only. Row 4 still needs the protected transition run.

RISK-VALUE: DERIVED — `Version = 1` @ statusgen/verifyqueue/queue.go:6, with `Supported` refusing any other version and any unknown class. Task 1 introduces a versioned additive contract, and this is its first version. Consumers pin it (deskboard roots.go:134, awaiting_bucket.go:800) and report a mismatch as a coverage gap (deskboard and verifyloop TestFlowQueue `old_statusgen_is_a_gap_not_zero` / `an_incomplete_canonical_read_exits_6`). Reversible by edit and redeploy.
RISK-VALUE: DERIVED — `AuthorityFlip = "flip"`, `AuthorityEvidence = "evidence-only"`, `AuthorityNone = "none"` @ tools/desk/internal/verifyflow/plan.go:31-33. `Compose` only ever demotes: only `judgment-dispatch` keeps dispatch, and `human-action` keeps Evidence-only authority (plan.go:36-57). This follows Task 3, "Keep Evidence-only human work separate from authority to flip". The lower layer is exercised with the planner bypassed (verifyloop `landing_guard_holds_with_planner_bypassed`). Reversible.
RISK-VALUE: DERIVED — `status == "implemented" && class == Judgment` @ statusgen/verifyqueue/queue.go:50-51 (`JudgmentWork`, the only depth deskboard counts as widenable, throughput.go:436). Task 2 says to exclude non-actionable rows from advice to widen model verification. Exercised by deskboard `depth_counts_only_judgment`. Reversible.

**VERIFY: BLOCKED** — rows 1, 2, 3, 5 and 6 pass in the admitted container bracket. Row 4, the protected acceptance-transition row, could not run because the approval read requires a forge token in the verifyrun environment and this verifier was dispatched without one. The brief is `gate: human` with `risk: customer: yes`, so it routes to the driver regardless.

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
