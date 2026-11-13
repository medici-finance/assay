---
brief: assay:assay:tool-refactoring:01
title: "Bind operation subjects to explicit tracking, delivery and evidence repositories"
why: "Tracking decisions, delivery candidates and Evidence destinations are different repositories in cross-repository work. Re-deriving those coordinates per command repeatedly sends reads, prompts or writes to the wrong home."
wave: 0
depends: []
unblocks: ["tool-refactoring/02", "tool-refactoring/08", "tool-refactoring/09", "tool-refactoring/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 7a0c0968-a001-4aab-acea-681909b3034f
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6274 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5525 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5500 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5999 — related class evidence; resolution not asserted"
  - "milestone-support/01 — retained owning work; see existing-work.md"
  - "milestone-support/08 — retained owning work; see existing-work.md"
  - "graph-execution/28 — retained owning work; see existing-work.md"
gate-why: "A wrong tracking, delivery or evidence coordinate can direct a credentialed adapter at another repository. Human acceptance confirms exact subject binding and independent effect authorization survive the consolidation."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/domain/operation.go: follow-up tool-refactoring/01 (this brief; source implementation is still owed)"
  - "statusgen/domain/operation_test.go: follow-up tool-refactoring/01 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/workselection.go: follow-up tool-refactoring/01 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/operationcontext.go: follow-up tool-refactoring/01 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/operationcontext_test.go: follow-up tool-refactoring/01 (this brief; source implementation is still owed)"
---

# Brief 01 — Bind operation subjects to explicit tracking, delivery and evidence repositories

## Context

files: statusgen/domain/operation.go, statusgen/domain/operation_test.go, tools/desk/internal/deskkit/workselection.go, tools/desk/internal/deskkit/operationcontext.go, tools/desk/internal/deskkit/operationcontext_test.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-01.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: bind operation subjects to explicit tracking, delivery and evidence repositories; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/domain composed with statusgen/workref; deskkit remains the observation adapter.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: milestone-support/01, milestone-support/08, graph-execution/28. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/domain composed with statusgen/workref; deskkit remains the observation adapter
  contract: S-verification-domain; S-context-handoff
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

## Human decision

This migration changes a control-bearing shared meaning or publication boundary.
The implementation must preserve exact subject binding, current permission and
source checks, and honest unavailable outcomes at independent effect boundaries.

Options:
1. Approve the bounded refactor after its independent class and caller controls pass.
2. Keep the existing behavior and request a narrower migration.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Workspace identity, request idempotency, allocation generations and scratch references already belong to `workspace/contract.go` and `spec/cell-workspaces-v1.md`. An operation subject only references those supplied observations; it must not redefine allocation identity, state or path ownership. The current source exposes validation/ports, not proof that every allocator effect is shipped.

1. Extend the existing qualified work/candidate records with an immutable, supplied-observation operation subject. Identify the declaring brief revision, tracking source, delivery candidate, review target, decision target and evidence destination separately. Reuse WorkSelection, BriefSubject and workref parsing; do not mint another work or claim identity.
2. Define named dispatch, review, PR-publication and evidence-publication projections. Missing coordinates stay unknown; one repository cannot silently substitute for another. A source path is a locator, never authorization. Preserve legacy identifiers and exact-version selection through existing profiles.
3. Build the same-repository and two-repository fixture matrix, including review of a delivery PR, a tracking decision and outcome publication. Keep credentials, alias/file/forge reads and custody outside the pure owner. Return structured problems; no new public wire schema is required.
4. Publish the consumer census for briefs 02, 09 and 10; update the existing owner rows rather than add a second registry. Preserve milestone-support/08 subject/claim work and its capability negotiation.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate all operation-subject constructors and every dispatch/review/PR/Evidence consumer in the declared files. A second consumer with a wrong tracking/effect coordinate must be rejected; a declared locator or alias cannot override the exact observed subject. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

statusgen/domain composed with statusgen/workref; deskkit remains the observation adapter owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectMatrix$' -count=1` | exit 0; output contains "--- PASS: TestOperationSubjectMatrix"; subject matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestOperationSubjectClassGuard"; class guard and planted wrong-repository caller | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestOperationContextAdapterFlow$' -count=1` | exit 0; output contains "--- PASS: TestOperationContextAdapterFlow"; real observation adapter flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/01 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
