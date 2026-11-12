---
brief: assay:assay:tool-refactoring:09
title: "Migrate PR create update and edit to the shared publication plan"
why: "PR create, update and edit derive roots, remote tips and failure recovery separately. A shared publication plan removes wrong-repository and partial-success behavior without relaxing scan or identity checks."
wave: 2
depends: ["tool-refactoring/01", "tool-refactoring/08"]
unblocks: ["tool-refactoring/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 87975545-1fd0-4239-a480-c1cf6d4d5e1d
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6281 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5914 — related class evidence; resolution not asserted"
  - "medici-finance/assay#2408 — related class evidence; resolution not asserted"
  - "medici-finance/assay#2356 — related class evidence; resolution not asserted"
  - "medici-finance/assay#1967 — related class evidence; resolution not asserted"
  - "medici-finance/assay#4838 — related class evidence; resolution not asserted"
  - "desk-tools/61 — retained owning work; see existing-work.md"
  - "desk-tools/62 — retained owning work; see existing-work.md"
  - "desk-tools/63 — retained owning work; see existing-work.md"
  - "desk-tools/64 — retained owning work; see existing-work.md"
  - "milestone-support/08 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/cmd/deskpr/deskpr.go: follow-up tool-refactoring/09 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskpr/createpublish.go: follow-up tool-refactoring/09 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskpr/edit.go: follow-up tool-refactoring/09 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskpr/operationplan_test.go: follow-up tool-refactoring/09 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/publishidentity.go: follow-up tool-refactoring/09 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 09 — Migrate PR create update and edit to the shared publication plan

## Context

files: tools/desk/cmd/deskpr/deskpr.go, tools/desk/cmd/deskpr/createpublish.go, tools/desk/cmd/deskpr/edit.go, tools/desk/cmd/deskpr/operationplan_test.go, tools/desk/internal/deskkit/publishidentity.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-09.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: migrate pr create update and edit to the shared publication plan; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: deskpr command adapter over publication, work-selection and forge owners.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-tools/61, desk-tools/62, desk-tools/63, desk-tools/64, milestone-support/08. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: deskpr command adapter over publication, work-selection and forge owners
  contract: S-publication-scan; S-context-handoff; publication plan from brief 08
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

1. Have create/update/edit use the same explicit root, subject, destination and publication profile. Select incoming-commit identity range from a proven remote tip, while scan scope remains governed by desk-tools/64. Keep draft-only create, existing-PR update and immutable link-trailer edit rules.
2. Expose the actual scope of offline preflight, remote preflight and live publication. Unknown remote facts stay unchecked; do not claim that offline --check has observed the live tip. Execution independently revalidates source/tip and all current guards.
3. Handle push-success/PR-open-failure, delayed branch visibility, remote-head mismatch and response loss through bounded authoritative observation. Reconcile before retry and hold unknown effects. An existing matching PR is an identified no-op; a merged/closed branch is never reused.
4. Remove duplicated preparation and coordinate derivation. Keep credential custody, shared scanner and outbound override ownership unchanged; desk-tools/61–64 implement scanner policy, and milestone-support/08 owns reference grammar. Include the existing false-positive corpus as preservation cases, not new exemptions.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Exercise create, update and edit through actual adapters. A second path that substitutes cwd for explicit root, expands identity range incorrectly, or retries an unknown PR-open effect must fail; scanner scope is not changed by this contract. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

deskpr command adapter over publication, work-selection and forge owners owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskpr -run '^TestPRPublicationPlanMatrix$' -count=1` | exit 0; output contains "--- PASS: TestPRPublicationPlanMatrix"; three operations and preflight profiles | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskpr -run '^TestPRPublicationPlanClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestPRPublicationPlanClassGuard"; cwd/tip confusion and second-site plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskpr -run '^TestPRPublicationPlanFlow$' -count=1` | exit 0; output contains "--- PASS: TestPRPublicationPlanFlow"; push visibility and response-loss flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/09 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
