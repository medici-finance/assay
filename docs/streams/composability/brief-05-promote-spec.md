---
brief: assay:assay:composability:05
title: Promote the draft to spec/component-v1.md + adopter doc delta
why: >-
  A model that lives only in a stream folder is a plan; one that lives in spec/ next to
  brief-v1, registers-v1 and lifecycle-v1 is a commitment third parties can implement against
  and adopters can read the removal path from. Once the four implementation briefs have landed,
  the draft's MUSTs describe the reference implementation, and that is the moment to version it.
wave: 3
depends: ["composability/01", "composability/02", "composability/03", "composability/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md — the draft-0 this brief promotes; its §12 divergence list is rewritten against what 00–04 shipped"
  - "spec/README.md — the versioning and change policy (draft phase; reference-implementation line; known divergences section) the promoted document must follow"
  - "docs/adopting-assay.md — the runbook that gains the component view and the removal path"
  - "composability/01, composability/02, composability/03, composability/04 — the shipped behaviour the spec describes"
version: 1
id: 8434eb43-806c-4a54-b5a5-6c3c2e88c0bf
---

# Brief 05 — Promote the draft to `spec/component-v1.md` (planned) + adopter doc delta

## Context

files:
- **create** `spec/component-v1.md` (planned) — from `component-model.md`, restructured to the shape of
  the three existing spec documents: version line, status line, "Describes reference
  implementation" line, scope, terminology, normative sections, conformance, known
  divergences.
- **edit** `spec/README.md` — add the fourth row to the document table; bump the
  specification-as-a-whole version per its policy.
- **edit** `docs/adopting-assay.md` — §2 Component inventory becomes a pointer to
  `components/KEYS.md` (planned) and the manifests; §3 gains "Removing a component" (the `deskdisable`
  path, the ledger, what stays for a human); the install scenario steps cite the record.
- **edit** `docs/streams/composability/component-model.md` — replace the body with a
  two-line pointer to `spec/component-v1.md` (planned) (the stream keeps the provenance; the spec keeps
  the text).
- **edit** `docs/distribution.md` — link the record and the reconcile engine.

facts:
- **Every MUST in the promoted document must be true of the reference implementation or listed
  under known divergences** — that is `spec/README.md`'s rule, and it is what separates a spec
  from a plan. The implementer re-derives §12 from the merged tree, not from the draft.
- **The provenance stays.** The paper citation and the mapping table (draft §1) move into the
  spec's scope section; the stream README keeps the narrative.
- **Prose deliverable — presence gates only.** Quality is the review gate's.
- **The stream closes with this brief**; no follow-up brief is opened here for the paper's
  open problems (structural typing, sandboxing) — they are listed as non-goals in the spec.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`.
- If a MUST in the draft is not true of the merged tree and cannot be listed as a divergence
  honestly (because it never will be), soften it to SHOULD in the promoted text and say so in
  the PR body; raise `NEEDS_CONTEXT` if the softening changes what 00–04 committed to.

## Task

1. Restructure the draft into `spec/component-v1.md` (planned) with the standard header lines and the
   existing documents' section order.
2. Re-derive the known-divergences section from the merged tree after 01–04.
3. Update `spec/README.md`'s table and version.
4. Write the adopter-doc delta: component view, removal path, record in the scenarios.
5. Reduce the stream-local draft to a pointer.

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f spec/component-v1.md && grep -c '^\*\*Version:\*\*\|^\*\*Status:\*\*\|^\*\*Describes reference implementation:\*\*' spec/component-v1.md` | exit 0; 3 |
| 2 | `grep -n 'component-v1.md' spec/README.md` | one table row |
| 3 | `grep -n -i 'Removing a component' docs/adopting-assay.md` | ≥ 1 |
| 4 | `grep -n 'components/KEYS.md' docs/adopting-assay.md` | ≥ 1 |
| 5 | `grep -c 'MUST' spec/component-v1.md` | ≥ 20 (the normative content survived the move) |
| 6 | `wc -l docs/streams/composability/component-model.md` | ≤ 15 (reduced to a pointer) |
| 7 | `grep -n 'Known divergences' spec/component-v1.md` | one heading |
| 8 | `statusgen --lint` (link check covers the new cross-references) | exit 0 |
| 9 | neighbour: `grep -n 'brief-v1.md\|registers-v1.md\|lifecycle-v1.md' spec/README.md \| wc -l` | ≥ 3 (existing rows intact) |

## Evidence

Pending — the implementer records its run here on reaching `implemented`; an independent
runner records a second run on merged main before `verified`.

## Review

gate: model — pending.
