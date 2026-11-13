---
brief: assay:assay:tool-refactoring:08
title: "Separate immutable publication preparation from fresh effect admission"
why: "Expensive Evidence preparation holds a global audit lock and blocks unrelated writers. An immutable preparation seam allows safe work outside the lock while retaining fresh authority, budget and conditional-write checks inside it."
wave: 1
depends: ["tool-refactoring/01"]
unblocks: ["tool-refactoring/09", "tool-refactoring/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 64d20596-9999-4477-96f2-d3bfb9db8ef3
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#5023 — related class evidence; resolution not asserted"
  - "medici-finance/assay#2356 — related class evidence; resolution not asserted"
  - "medici-finance/assay#2408 — related class evidence; resolution not asserted"
  - "desktools-v2/75 — retained owning work; see existing-work.md"
  - "graph-execution/04 — retained owning work; see existing-work.md"
gate-why: "The refactor changes ordering around the global audit lock and conditional write. Human acceptance confirms fresh authority and budget checks remain serialized and changed material inputs cannot reuse an old preparation."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/publicationplan.go: follow-up tool-refactoring/08 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/publicationplan_test.go: follow-up tool-refactoring/08 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/publishidentity.go: follow-up tool-refactoring/08 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/evidencelanding.go: follow-up tool-refactoring/08 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/writeflow.go: follow-up tool-refactoring/08 (this brief; source implementation is still owed)"
---

# Brief 08 — Separate immutable publication preparation from fresh effect admission

## Context

files: tools/desk/internal/deskkit/publicationplan.go, tools/desk/internal/deskkit/publicationplan_test.go, tools/desk/internal/deskkit/publishidentity.go, tools/desk/internal/deskkit/evidencelanding.go, tools/desk/cmd/deskevidence/writeflow.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-08.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: separate immutable publication preparation from fresh effect admission; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: deskkit publication application seam; existing guard, budget and forge owners retain authority.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desktools-v2/75, graph-execution/04. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: deskkit publication application seam; existing guard, budget and forge owners retain authority
  contract: S-effect-recovery; S-publication-scan; existing conditional publication contract
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

1. Create typed immutable preparation values for PR publication and Evidence append using brief 01 subjects. Bind source/destination observations, exact candidate, material local input digests and the existing scanner/policy revision; the value carries no token or reusable permission.
2. Specify and implement the common prepare/revalidate/admit/apply/reconcile ordering through existing seams. Hoist only immutable expensive work. Authority, local input, destination tip, trust/decision and budget checks remain fresh under the existing serialized write window before the conditional effect and audit append.
3. Keep distinct PR and Evidence profiles: identity delta, full publication-scan scope and append transformation have different meanings and must not become one permissive generic policy. Unknown provider outcomes retain stable operation identity and observation-based reconciliation; no blind replay or new effect journal.
4. Prove changed-material-input refusal and unrelated-change compatibility. Preserve lock order, cap and deadline rather than widening them for throughput. Reuse existing drainloop effect-recovery rules and PR 5035; desktools-v2/75 retains shared forge library extraction.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate PR/Evidence publication entrypoints and their AllowWrite-to-effect-to-audit paths. A second caller outside the shared admission ordering or a deliberate early lock release must fail under two concurrent writers; comparing helper output alone is insufficient. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

deskkit publication application seam; existing guard, budget and forge owners retain authority owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestPublicationPlanMatrix$' -count=1` | exit 0; output contains "--- PASS: TestPublicationPlanMatrix"; preparation and revalidation matrix | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestPublicationPlanClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestPublicationPlanClassGuard"; stale input and unlocked-write plants | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestPublicationPlanConcurrency$' -count=1` | exit 0; output contains "--- PASS: TestPublicationPlanConcurrency"; two concurrent callers and budget flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/08 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
