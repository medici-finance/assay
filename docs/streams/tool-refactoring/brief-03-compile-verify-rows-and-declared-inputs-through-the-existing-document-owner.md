---
brief: assay:assay:tool-refactoring:03
title: "Compile Verify rows and declared inputs through the existing document owner"
why: "A Verify table can be accepted by one reader and rejected or misexecuted by another. One document compiler makes row identity and declared input syntax agree before an expensive verification run."
wave: 0
depends: []
unblocks: ["tool-refactoring/04", "tool-refactoring/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 18d445a4-baf8-4f70-8eca-7ad0092dffa9
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#5575 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5097 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5013 — related class evidence; resolution not asserted"
  - "medici-finance/assay#4199 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5807 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5434 — related class evidence; resolution not asserted"
  - "statusgen/29 — retained owning work; see existing-work.md"
  - "statusgen/30 — retained owning work; see existing-work.md"
  - "statusgen/34 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
  - "statusgen/40 — retained owning work; see existing-work.md"
gate-why: "Declared inputs and executable row syntax feed verification admission. Human acceptance confirms unsupported or ambiguous declarations cannot expand allowed source or turn prose into an executed/credited check."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/briefdoc/verification.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/briefdoc/verification_test.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/verifyrun.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/evaluation/declared_api.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/declaredscope.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verificationdefinition_test.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
---

# Brief 03 — Compile Verify rows and declared inputs through the existing document owner

## Context

files: statusgen/briefdoc/verification.go, statusgen/briefdoc/verification_test.go, statusgen/verifyrun.go, statusgen/evaluation/declared_api.go, tools/desk/internal/deskkit/declaredscope.go, tools/desk/internal/deskkit/verificationdefinition_test.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-03.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: compile verify rows and declared inputs through the existing document owner; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/briefdoc for syntax; statusgen/evaluation for normalized evaluation inputs.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: statusgen/29, statusgen/30, statusgen/34, statusgen/37, statusgen/40. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/briefdoc for syntax; statusgen/evaluation for normalized evaluation inputs
  contract: S-context-handoff; S-evidence-coverage
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

1. Extend briefdoc with one pure compilation of Verify row identity, class, shell, explicit command, Expect and Context files/inputs declarations. Reuse existing section/fence grammar and evaluation declaration APIs; keep source enumeration and path confinement in adapters.
2. Characterize current accepted forms before migrating: inline/list/backticked declarations, NEW annotations, prose tails, escaped pipes, numeric and lettered row IDs, gate:model prose and explicit cmd markers. Preserve intentional legacy profiles. Do not choose the union of permissive parsers or execute a prose code span.
3. Migrate verifyrun and the deskkit declared-scope projection to the same compiled record. Lint/newbrief keep their existing structural and strength rules, consuming shared syntax where applicable. Unsupported syntax holds at authoring/preflight with the same typed cause, before row execution.
4. Remove local lexical copies and retain actual-caller tests with a planted divergent parser. This owns syntax consolidation only: statusgen/29–31 retain authoring/debt repair, /34 scratch, /37 witness-v2 writes and /40 approved acceptance transitions.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate Verify-row and Context-declaration parsers/projections across statusgen and deskkit. A planted local parser that accepts an ambiguous declaration or prose code span must fail the census/behavior control; legacy differences need a named profile, not a second grammar. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

statusgen/briefdoc for syntax; statusgen/evaluation for normalized evaluation inputs owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionMatrix$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionMatrix"; document and row grammar matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionClassGuard"; no prose execution and second-parser plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestVerificationDefinitionFlow$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionFlow"; runner to landing scope flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/03 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
