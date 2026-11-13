---
brief: assay:assay:tool-refactoring:10
title: "Migrate Evidence publication to bounded deltas and typed outcome admission"
why: "Whole-file Evidence updates and separate outcome validation can disagree with current destination content. A typed delta and outcome path preserves concurrent edits and records exactly what the admitted publication changed."
wave: 2
depends: ["tool-refactoring/01", "tool-refactoring/03", "tool-refactoring/05", "tool-refactoring/08"]
unblocks: ["tool-refactoring/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: a2f7066f-c68c-4d4b-89ab-509f5364c386
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#3371 — related class evidence; resolution not asserted"
  - "medici-finance/assay#3195 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5753 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6272 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6270 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5543 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5023 — related class evidence; resolution not asserted"
  - "desk-tools/30 — retained owning work; see existing-work.md"
  - "desk-tools/46 — retained owning work; see existing-work.md"
  - "desk-tools/47 — retained owning work; see existing-work.md"
  - "desk-tools/50 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/cmd/deskevidence/deskevidence.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/writeflow.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcome.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcomejoin.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/publicationplan_test.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/evidencelanding.go: follow-up tool-refactoring/10 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 10 — Migrate Evidence publication to bounded deltas and typed outcome admission

## Context

files: tools/desk/cmd/deskevidence/deskevidence.go, tools/desk/cmd/deskevidence/writeflow.go, tools/desk/cmd/deskevidence/outcome.go, tools/desk/cmd/deskevidence/outcomejoin.go, tools/desk/cmd/deskevidence/publicationplan_test.go, tools/desk/internal/deskkit/evidencelanding.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-10.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: migrate evidence publication to bounded deltas and typed outcome admission; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: deskevidence adapter over publication and canonical outcome/definition owners.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-tools/30, desk-tools/46, desk-tools/47, desk-tools/50, statusgen/37. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: deskevidence adapter over publication and canonical outcome/definition owners
  contract: S-evidence-coverage; S-effect-recovery; publication plan from brief 08
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

1. Prepare a bounded Evidence delta or immutable outcome record against brief 01 tracking destination, using brief 03 definitions and brief 05 reduction. Append to freshly observed destination bytes; never overwrite non-Evidence edits with a stale whole brief.
2. Use brief 08 ordering, prepare outside the lock only with complete input binding, then independently re-admit verifier identity, source, protected criteria, destination and budget. Return the actual committed identity/receipt. Concurrent equivalent appends are idempotent; conflicting relevant edits hold.
3. Use the existing provider capability and raw-byte adapters. Incorporate PR 6284 and PR 5547 rather than reimplement them. Unknown comparison or blob completeness is not an empty success. Distinguish a diagnostic-only record from a verified transition using brief 05, without weakening closure guards.
4. Use fresh landing identities when an earlier Evidence PR is terminal. Preserve shared caps without treating cumulative logs as single mutable blobs. desk-tools/47 retains resumable landing orchestration, /30 verdict-transcription cutover, /46 accepted-PASS reuse and /50 environment evidence. This refactor supplies their existing publication seam, not another workflow.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Exercise direct Evidence-file, row and outcome-record publication forms. A second writer that copies a stale whole brief or credits a diagnostic-only record as a lifecycle transition must fail; destination CAS alone is not the delta-preservation oracle. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

deskevidence adapter over publication and canonical outcome/definition owners owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskevidence -run '^TestEvidencePublicationPlanMatrix$' -count=1` | exit 0; output contains "--- PASS: TestEvidencePublicationPlanMatrix"; delta, outcome and source-profile matrix | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskevidence -run '^TestEvidencePublicationPlanClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestEvidencePublicationPlanClassGuard"; stale whole-file and second write-path plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskevidence -run '^TestEvidencePublicationPlanFlow$' -count=1` | exit 0; output contains "--- PASS: TestEvidencePublicationPlanFlow"; concurrent landing to queue-consumption flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/10 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
