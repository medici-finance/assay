---
brief: assay:assay:tool-refactoring:02
title: "Make dispatch planning and execution consume one validated operation subject"
why: "Dry-run can validate a different dispatch from the live path, and prompts can lose the caller task. One validated plan makes preview, claim, gate and emitted assignment agree without treating a preview as authority."
wave: 1
depends: ["tool-refactoring/01"]
unblocks: ["tool-refactoring/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 368a25b3-dd84-4ae2-bb8b-3011086699b6
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6173 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6279 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5441 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5683 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5525 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5999 — related class evidence; resolution not asserted"
  - "desktools-v2/06 — retained owning work; see existing-work.md"
  - "agent-execution/01 — retained owning work; see existing-work.md"
  - "agent-execution/03 — retained owning work; see existing-work.md"
  - "statusgen/21 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/cmd/deskdispatch/dispatch.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskdispatch/deliverable.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskdispatch/prompt.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskdispatch/packet_verifier.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskdispatch/operationplan.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskdispatch/operationplan_test.go: follow-up tool-refactoring/02 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 02 — Make dispatch planning and execution consume one validated operation subject

## Context

files: tools/desk/cmd/deskdispatch/dispatch.go, tools/desk/cmd/deskdispatch/deliverable.go, tools/desk/cmd/deskdispatch/prompt.go, tools/desk/cmd/deskdispatch/packet_verifier.go, tools/desk/cmd/deskdispatch/operationplan.go, tools/desk/cmd/deskdispatch/operationplan_test.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-02.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: make dispatch planning and execution consume one validated operation subject; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: deskdispatch application orchestration over existing domain, claim and admission owners.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desktools-v2/06, agent-execution/01, agent-execution/03, statusgen/21. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: deskdispatch application orchestration over existing domain, claim and admission owners
  contract: S-context-handoff; S-verification-domain
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

Use the existing workspace request/record and allocator port for any managed attempt or scratch reference. Legacy dispatch remains explicitly legacy until its allocator consumer is qualified. Do not implement another checkout allocator, lease store or inferred managed-path convention in this brief.

1. Construct one validated dispatch plan before claims/worktrees. Resolve the brief, gate, tracking decision, execution/delivery root, target PR, model-profile requirements, caller task bytes and per-attempt scratch/output destinations once from the admitted subject.
2. Have dry-run render that same plan, reporting any checks that require fresh execution-time observations as unchecked. It may neither claim nor file. Execution consumes the plan and independently rechecks current gate, role authority, claim and source before effects; dry-run success is not a reusable grant.
3. Use the existing qualified profile and prompt/packet owners. Carry caller task text intact and separate child scratch paths. Keep repository-specific credentials at their existing effect boundaries; no ambient fallback or new model selection policy.
4. Inject process lookup and external observations in fixtures so installed deskdecision or a developer PATH cannot alter a test. Remove duplicated gate/repository derivation in the migrated paths. Reuse landed fixes and the in-flight narrow repairs listed in existing-work.md.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate worker, reviewer, verifier and resume dispatch plans. Plant a second kit that omits brief/gate resolution or drops task text; the same plan/consumer contract must reject it, including dry-run and live planning. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

deskdispatch application orchestration over existing domain, claim and admission owners owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskdispatch -run '^TestDispatchOperationPlan$' -count=1` | exit 0; output contains "--- PASS: TestDispatchOperationPlan"; all kits and dry/live planning parity | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskdispatch -run '^TestDispatchOperationClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestDispatchOperationClassGuard"; second-site bypass plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskdispatch -run '^TestDispatchOperationFlow$' -count=1` | exit 0; output contains "--- PASS: TestDispatchOperationFlow"; cross-repository decision, claim and prompt flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/02 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
