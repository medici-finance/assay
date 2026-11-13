---
brief: assay:assay:tool-refactoring:05
title: "Share outcome decoding and reduction between verification producers and readers"
why: "The outcome writer can accept values its queue readers reject, while row pass and verified acceptance mean different things. A shared reducer prevents contradictory or incomplete records from stranding work or skipping closure checks."
wave: 0
depends: []
unblocks: ["tool-refactoring/10", "tool-refactoring/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: cca3ff32-d298-45a7-9f43-d7cfc226253a
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6002 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5827 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5950 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6272 — related class evidence; resolution not asserted"
  - "medici-finance/assay#1830 — related class evidence; resolution not asserted"
  - "desk-tools/46 — retained owning work; see existing-work.md"
  - "desk-tools/47 — retained owning work; see existing-work.md"
  - "desk-tools/49 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
gate-why: "Outcome normalization controls whether verified-closure and held-row checks run. Human acceptance confirms a row result or unknown outcome cannot bypass those independent gates."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/evaluation/outcome.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "statusgen/evaluation/outcome_test.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "statusgen/awaiting_bucket.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verifyoutcomes.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verifywake.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcome.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcomerecord.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcomeprotocol_test.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
---

# Brief 05 — Share outcome decoding and reduction between verification producers and readers

## Context

files: statusgen/evaluation/outcome.go, statusgen/evaluation/outcome_test.go, statusgen/awaiting_bucket.go, tools/desk/internal/deskkit/verifyoutcomes.go, tools/desk/internal/deskkit/verifywake.go, tools/desk/cmd/deskevidence/outcome.go, tools/desk/cmd/deskevidence/outcomerecord.go, tools/desk/cmd/deskevidence/outcomeprotocol_test.go, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-05.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: share outcome decoding and reduction between verification producers and readers; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/evaluation outcome package with adapters in deskkit and statusgen.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-tools/46, desk-tools/47, desk-tools/49, statusgen/37. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/evaluation outcome package with adapters in deskkit and statusgen
  contract: S-evidence-coverage; S-verification-domain
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

1. Extract the existing record vocabulary, normalization and reduction into an importable pure package; retain legacy/per-file adapters and exact record naming compatibility. Row pass is not an overall verified outcome. Reject unknown values rather than let writers create records readers cannot interpret.
2. Use one typed reduction of row_results, held-row selection, run coverage and outcome. Distinguish a blocked row from a closure-only blocker; an empty held set cannot erase observed non-pass rows. Preserve exact row IDs and explicitly represent absent evidence as unavailable.
3. Keep append-only diagnostic/outcome observation separate from changing accepted lifecycle state. A later BLOCKED diagnostic on an already-verified delivery neither demotes prior acceptance nor authorizes a new verified flip. Apply current protected-closure rules at the actual state-changing boundary.
4. Migrate actual writer and queue readers, delete copied enum/reducer bodies, and qualify old/new record readers. Do not implement fixed-delivery policy, PASS reuse or wake deduplication here: those remain with desk-tools/46–51 and their precise decisions.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate every outcome writer and queue/closure reader, including legacy and per-file layouts. Plant a second reader/writer with a divergent enum or held-row reduction; the shared codec/reducer contract must detect it. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

statusgen/evaluation outcome package with adapters in deskkit and statusgen owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v ./evaluation -run '^TestOutcomeProtocolMatrix$' -count=1` | exit 0; output contains "--- PASS: TestOutcomeProtocolMatrix"; wire vocabulary and reducer matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v ./evaluation -run '^TestOutcomeProtocolClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestOutcomeProtocolClassGuard"; writer/reader drift and second enum plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskevidence -run '^TestOutcomeProtocolFlow$' -count=1` | exit 0; output contains "--- PASS: TestOutcomeProtocolFlow"; publication and queue reader flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/05 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
