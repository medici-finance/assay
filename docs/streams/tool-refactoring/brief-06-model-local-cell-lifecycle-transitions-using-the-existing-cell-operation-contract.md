---
brief: assay:assay:tool-refactoring:06
title: "Model local cell lifecycle transitions using the existing cell-operation contract"
why: "The reported holder-loss bug already has a source repair. Up, reconcile and down still interpret ownership observations in CLI-local paths; consolidating decision-only logic behind the existing cadence owner prevents a sibling path from reintroducing the class."
wave: 0
depends: []
unblocks: ["tool-refactoring/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 9c11fcfe-e4f4-42e7-9695-0b33e634328a
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6251 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6170 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6217 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6148 — related class evidence; resolution not asserted"
  - "desk-containers/15 — retained owning work; see existing-work.md"
  - "desk-containers/17 — retained owning work; see existing-work.md"
  - "desk-containers/18 — retained owning work; see existing-work.md"
  - "graph-execution/20 — retained owning work; see existing-work.md"
  - "graph-execution/26 — retained owning work; see existing-work.md"
gate-why: "Changes how a role process is identified as stopped or stale. Human acceptance confirms that pid reuse, owner generation and independent lease/process checks still prevent signaling or settling another owner."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/internal/cellcadence/lifecycle.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/internal/cellcadence/lifecycle_test.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/down_settle.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/internal/cellcadence/interactive.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/lifecycleplan_test.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
---

# Brief 06 — Model local cell lifecycle transitions using the existing cell-operation contract

## Context

files: tools/desk/internal/cellcadence/lifecycle.go, tools/desk/internal/cellcadence/lifecycle_test.go, tools/desk/cmd/cellctl/down_settle.go, tools/desk/internal/cellcadence/interactive.go, tools/desk/cmd/cellctl/lifecycleplan_test.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-06.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: model local cell lifecycle transitions using the existing cell-operation contract; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Source repair already present: `down_settle.go` captures Sequence/LastStart and `recordIsOwners` matches the checkpoint after holder removal for issue 6251; preserve it, do not implement it again.
- Existing owner: tools/desk/internal/cellcadence; loopadmin remains process-supervision owner.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-containers/15, desk-containers/17, desk-containers/18, graph-execution/20, graph-execution/26. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: tools/desk/internal/cellcadence; loopadmin remains process-supervision owner
  contract: existing cell-op-plan/result and loop-admin-runner-v1 contracts
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

## Human decision

Role lifecycle decisions determine which process a cell may settle or restart.
The refactor must preserve the existing owner-generation and lease boundaries.

Options:
1. Approve the decision-only consolidation after the independent owner and process controls pass.
2. Keep the current paths and request a narrower change.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Cell-role/process transitions are distinct from workspace allocating/ready/active/retained/retiring/retired states already owned by `workspace/contract.go`. Consume its allocation generation and lifecycle observations; do not create another workspace state machine, allocator, lease or collector. This brief models only current role up/down/settle behavior.

1. Add a pure local lifecycle transition model to the existing cellcadence owner over explicit holder identity, pid/start identity, checkpoint generation, pause/drain state and observed process result. Reuse existing plan/result schemas and runner identity; do not create a second runtime store or lease manager.
2. Express up/reconcile/down/settle and plan-stale decisions through that model. Preserve the already implemented signalled-owner/checkpoint capture repair as a characterization case; do not infer a different owner from a reused pid. An absent or unreadable observation remains could-not-check, and repeated down/up must not invent a completed transition.
3. Keep checkpoint persistence, process signalling and locks at existing adapters. Preserve compare-and-apply identity and one-shot plan confirmation. Remove only duplicated transition judgment after real caller migration; retain independent process-identity/fencing checks.
4. Bind to the supported current local adapter and existing runner protocol. graph-execution/26–27 retain durable launch supervision and standing-client migration; desk-containers/19–24 retain new operator commands. A local refactor neither implements nor activates those future capabilities.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate up, reconcile, down and settle transitions and checkpoint writers. Plant a second path that uses a reused pid or changes checkpoint state without the recorded owner generation; reject it independently of its formatted message. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

tools/desk/internal/cellcadence; loopadmin remains process-supervision owner owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleMatrix$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleMatrix"; transition and ownership matrix | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleClassGuard"; reused pid and stale-generation plants | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellLifecyclePlanFlow$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecyclePlanFlow"; checkpoint/holder adapter flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/06 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
