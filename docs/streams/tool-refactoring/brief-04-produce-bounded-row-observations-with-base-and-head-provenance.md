---
brief: assay:assay:tool-refactoring:04
title: "Produce bounded row observations with base and head provenance"
why: "Final pipeline exits and base red/green labels omit the observations needed to judge a row. Retaining bounded stage and base/head evidence prevents an infrastructure error or a prose fragment from becoming a credited test result."
wave: 1
depends: ["tool-refactoring/03"]
unblocks: ["tool-refactoring/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: e59bd5f3-8f67-4176-85ca-4ae2fd0edafd
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#5899 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5826 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5575 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5544 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5565 — related class evidence; resolution not asserted"
  - "medici-finance/assay#1794 — related class evidence; resolution not asserted"
  - "statusgen/37 — retained owning work; see existing-work.md"
  - "desktools-v2/74 — retained owning work; see existing-work.md"
  - "desktools-v2/81 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/verifyrun.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/failfirst.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/verifyincontainer.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/containerbracket.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/rowobservation.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/rowobservation_test.go: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
  - "statusgen/rowobservation-mutations.json: follow-up tool-refactoring/04 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 04 — Produce bounded row observations with base and head provenance

## Context

files: statusgen/verifyrun.go, statusgen/failfirst.go, statusgen/verifyincontainer.go, statusgen/containerbracket.go, statusgen/rowobservation.go, statusgen/rowobservation_test.go, statusgen/rowobservation-mutations.json, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-04.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: produce bounded row observations with base and head provenance; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: verifyrun execution adapter; evaluation owns judgment and existing witness-v2 owns serialization.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: statusgen/37, desktools-v2/74, desktools-v2/81. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: verifyrun execution adapter; evaluation owns judgment and existing witness-v2 owns serialization
  contract: S-evidence-coverage; S-verification-domain
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

1. Run only the compiled executable rows from brief 03. Separate spawn/sandbox availability, process/pipeline observations and Expect assessment. Record relevant stage exits and bounded primary stdout/stderr references for base and head, not only the final filter exit or a green/red label.
2. Keep fail-first delivery history in the delivery repository while criteria remain bound to the tracking source. Integrate the existing cross-repository repairs rather than create an alternate base resolver. Preserve legitimate expected-nonzero commands; an upstream nonzero alone does not prove that the test never ran.
3. Feed observations into the existing witness-v2 writer contract in statusgen/37. Any additional field must use its extension/version policy and qualified readers, not a third witness format. Bound retained output and disclose truncation; output-derived acceptance requiring unavailable primary evidence stays held.
4. Preserve network-off container brackets, read-only source, independent admission/close and scrubbed credentials. A base infrastructure failure, no executable check or incomplete stage observation cannot be credited as a discriminating failure. Keep the hostile-child and malformed-output controls across host and container fixture adapters.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate host/container/base/head execution paths and result constructors. Plant a second execution path that reports only the final filter exit or treats a prose row as executable; incomplete observations must not acquire execution/fail-first credit. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

verifyrun execution adapter; evaluation owns judgment and existing witness-v2 owns serialization owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v . -run '^TestRowObservationMatrix$' -count=1` | exit 0; output contains "--- PASS: TestRowObservationMatrix"; base/head and pipeline observation matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v . -run '^TestRowObservationClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestRowObservationClassGuard"; spurious pass and credential inheritance plants | check:ci +mutation +neighbour |
| 3 | `cd statusgen && GOWORK=off go test -v . -run '^TestRowObservationFlow$' -count=1` | exit 0; output contains "--- PASS: TestRowObservationFlow"; tracking criteria to delivery history to bracket flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/04 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
