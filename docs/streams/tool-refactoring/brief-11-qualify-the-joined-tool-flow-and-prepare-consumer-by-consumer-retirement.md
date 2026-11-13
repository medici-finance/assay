---
brief: assay:assay:tool-refactoring:11
title: "Qualify the joined tool flow and prepare consumer-by-consumer retirement"
why: "Individual packages can pass while the dispatch-to-Evidence path still fails or a consumer runs an incompatible binary. Joined adapter qualification and explicit retirement evidence make the refactor usable through the existing release owners."
wave: 3
depends: ["tool-refactoring/02", "tool-refactoring/04", "tool-refactoring/05", "tool-refactoring/07", "tool-refactoring/09", "tool-refactoring/10", "statusgen/37"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 45c935f4-4a2b-4707-ac90-906587b67c05
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6274 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6002 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6272 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5023 — related class evidence; resolution not asserted"
  - "desktools-v2/14 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
  - "graph-execution/25 — retained owning work; see existing-work.md"
  - "milestone-support/14 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/toolrefactor_flow_test.go: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
  - "statusgen/toolrefactor_flow_test.go: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
  - "docs/streams/tool-refactoring/qualification.md: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
  - "docs/streams/tool-refactoring/issue-routing.csv: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
  - "docs/library-first.md: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
  - "docs/contracts.md: follow-up tool-refactoring/11 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 11 — Qualify the joined tool flow and prepare consumer-by-consumer retirement

## Context

files: tools/desk/internal/deskkit/toolrefactor_flow_test.go, statusgen/toolrefactor_flow_test.go, docs/streams/tool-refactoring/qualification.md, docs/streams/tool-refactoring/issue-routing.csv, docs/library-first.md, docs/contracts.md, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-11.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: qualify the joined tool flow and prepare consumer-by-consumer retirement; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: existing qualification and release/pin owners; no new controller.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desktools-v2/14, statusgen/37, graph-execution/25, milestone-support/14. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: existing qualification and release/pin owners; no new controller
  contract: verification-domain-v1; witness-v2; existing publication and runner contracts
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

## Human decision

The proposed refactor changes a control-bearing path used by automated workers.
The implementation must preserve explicit identity, independent source and permission
checks, and truthful unavailable outcomes. Planning approval cannot activate it.

Options:
1. Approve this bounded migration after the reviewed implementation proves the old controls still refuse unsafe cases.
2. Keep the current path and request a narrower refactor.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Keep workspace allocator, launch/dispatch and retention migration with S-worktree and its existing callers. The joined test records whether it used a current legacy or qualified managed profile; a fake allocation cannot establish a supported managed rollout.

1. Run the real command/application adapters through dispatch to execution to publication to queue/acceptance projection, using isolated repositories and stateful provider fakes. The product under qualification is the actual adapters and shared rules; no demo-only coordinator or new runtime. Publish a real/substitute matrix and its evidence limits.
2. Execute fail-first controls for wrong tracking/delivery repository, forged/partial outcome, stage failure, stale relevant source, unrelated edit, disappearing process owner, concurrency and unknown provider result. Preserve existing protected/human acceptance and all scanner-positive refusals. Include a second planted caller per class.
3. Account for every routed issue: current implementation regression case, existing-brief obligation, new-class case, operational/adoption action or explicit unresolved candidate. An issue title match is not proof of root cause or resolution. Preserve source-pinned closed-fix controls; no bulk issue closure.
4. Build each affected module with GOWORK=off and qualify the minimum supported old/new record and binary combinations. Produce a manifest of required releases, source/pin and remaining consumer changes for existing release owners. Retire duplicate paths only after their last consumer is proved migrated. No tag, re-pin, activation or live infrastructure act is authorized by this qualification brief.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Exercise every migrated adapter through the joined flow and enumerate remaining legacy consumers. Plant one wrong-subject, unadmitted-write or divergent-result path outside the original incident site; component-green or a missing consumer cannot satisfy joined acceptance. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

existing qualification and release/pin owners; no new controller owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestToolRefactorJoinedFlow$' -count=1` | exit 0; output contains "--- PASS: TestToolRefactorJoinedFlow"; joined actual-adapter workflow | check:ci +flow |
| 2 | `cd statusgen && GOWORK=off go test -v . -run '^TestToolRefactorWireCompatibility$' -count=1` | exit 0; output contains "--- PASS: TestToolRefactorWireCompatibility"; cross-module witness/outcome compatibility | check:ci +flow +dereference |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestToolRefactorJoinedClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestToolRefactorJoinedClassGuard"; joined class plants and independent lower-boundary refusals | check:ci +mutation +neighbour |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/11 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
