---
brief: assay-product/03
title: Product repo hygiene — product roadmap + README doc index in assay-toolkit (hand-authored STATUS.md dropped per [F-33](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-assay-product-03-unmerged-pr4-and-status-md-verify-contradiction.md))
wave: 1
depends: ["assay-product/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["../reconciler/STATUS.md + docs/product-roadmap.md (the D4/D6/D7 shape to mirror)", "assay-product/01 (positioning it phases)", "[I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) / PR #206 (self-containment — the roadmap's machinery track)", "[I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md) (adoption track)", "[I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md) (marketplace track)", "freshness-checked 2026-07-10 @ 78200803"]
---

# Brief 03 — Product repo hygiene: roadmap + README doc index

> **Amended 2026-07-12 ([F-33](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-assay-product-03-unmerged-pr4-and-status-md-verify-contradiction.md) resolution step 1):** the hand-authored STATUS.md deliverable and
> its Verify row are REMOVED — assay-toolkit inherits the single-writer rule (STATUS.md is
> generated on its main by CI), so a brief asking a worker to hand-write it was asking for a
> rule violation. The README instead carries a single-writer note (landed on assay-toolkit PR #4).

**CROSS-REPO:** deliverables land in `../assay-toolkit` (docs/product-roadmap.md,
README edits — commit there, SHA in Evidence). This repo: stream README gains the
`external: ../assay-toolkit/STATUS.md` frontmatter pointer (mirroring reconciler-spinout's)
in the SAME change, so the board tracks the product repo from then on.

## Context
files: ../assay-toolkit/docs/product-roadmap.md (new);
../assay-toolkit/README.md (doc-index section); docs/streams/assay-product/README.md
(external: pointer)
facts:
- Mirror Plumb's STATUS.md shape exactly (roll-up table of phases with done/total, then
  per-phase item tables with priority/status) — it is the proven "one-file snapshot" that
  external: pointers consume.
- Roadmap phases to draft (adjust against 01's positioning): Phase 0 = product docs +
  clearance (this stream); Phase 1 = self-containment + versioned releases ([I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) / PR #206
  — reference, don't duplicate); Phase 2 = website + launch (briefs 04/05/06);
  Phase 3 = second consumer (`../reconciler` adopting the toolkit was [I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)'s plan) +
  dogfood marketplace ([I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md)); Phase 4 = multi-person/SME adoption ([I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md) → assay-adoption
  stream). Each phase: goal line + item table + gaps.
- README gains a docs index table (D6 pattern) linking product-brief, market-analysis,
  roadmap, and the four existing adoption docs.
- statusgen `external:` behavior: the pointer renders in STATUS.md's roll-up Notes — the
  path must exist when lint runs on a checkout that has the sibling repo; CI runners do NOT
  have it. Check how reconciler-spinout's pointer behaves in CI lint before wiring
  (`git log` its introduction) — if CI tolerates a missing sibling path, wire it; if not,
  report NEEDS_CONTEXT with the evidence rather than breaking the PR gate.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit commits local; push is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `../assay-toolkit/STATUS.md` (Plumb shape) with the phase roll-up seeded from the
   roadmap below; mark this stream's briefs as the Phase 0 items with live statuses.
2. Write `../assay-toolkit/docs/product-roadmap.md` per facts (phases, items, gaps —
   referencing [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)/[I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md)/[I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md) by ID, never duplicating their scoped work).
3. Add the README doc-index table.
4. Wire `external: ../assay-toolkit/STATUS.md` into this stream's README frontmatter,
   subject to the CI-tolerance check in facts.
5. Update the stream-README row.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci "STATUS" ../assay-toolkit/README.md` | ≥1 (README carries the STATUS.md-is-generated single-writer note; hand-authored file dropped per F-33) |
| 2 | `test -f ../assay-toolkit/docs/product-roadmap.md && grep -cE "Phase [0-4]" ../assay-toolkit/docs/product-roadmap.md` | ≥4 |
| 3 | `grep -cE "I-19|I-24|I-27" ../assay-toolkit/docs/product-roadmap.md` | ≥3 (sibling scopes referenced, not absorbed) |
| 4 | `grep -c "docs/" ../assay-toolkit/README.md` | ≥4 (doc index present) |
| 5 | `statusgen --root . --lint; echo $?` | 0 (with or without the external: pointer per the CI-tolerance check) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verified 2026-07-13 by `opus-verifier` (non-implementer, verify desk) against merged main `a7f48277`
and sibling `medici-finance/assay-toolkit` @ `faf363b` (deliverables landed via assay-toolkit PR #4,
merged 2026-07-12, merge commit `cf0dbd6`).

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -ci "STATUS" ../assay-toolkit/README.md` | 0 | `16` (≥1) — README states the single-writer rule at README:56-57 and :79-82, not an incidental keyword hit |
| 2 | `test -f ../assay-toolkit/docs/product-roadmap.md && grep -cE "Phase [0-4]" ../assay-toolkit/docs/product-roadmap.md` | 0 | `19` (≥4) |
| 3 | `grep -cE "I-19\|I-24\|I-27" ../assay-toolkit/docs/product-roadmap.md` | 0 | `5` (≥3) — [I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md)/[I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)/[I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md) cited as `**Scope reference:**`, not duplicated |
| 4 | `grep -c "docs/" ../assay-toolkit/README.md` | 0 | `23` (≥4) — 9-row doc-index table present |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | `0` — NOTICE-only, no lint errors |

Substance: no hand-authored `STATUS.md` exists in assay-toolkit (`git ls-files STATUS.md` → 0) —
[F-33](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-assay-product-03-unmerged-pr4-and-status-md-verify-contradiction.md)'s second blocker is cleared. The only `STATUS.md` in that repo is
`../assay-toolkit/examples/adopter-scaffold/STATUS.md`, a generated-header fixture from the initial extraction commit.

Follow-ups (not verify blockers, filed for the desk): [F-33](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-assay-product-03-unmerged-pr4-and-status-md-verify-contradiction.md) is materially satisfied and ready to close;
the brief's Task 1 and facts line still say "write ../assay-toolkit/STATUS.md", contradicting the
2026-07-12 amendment note above them; the stream README `external:` pointer at
`../assay-toolkit/STATUS.md` is dangling until that repo's own CI generates one (statusgen tolerates
it — Verify row 5 explicitly permits it).

## Review
Gate: model. Reviewer checks the roadmap references sibling streams by ID instead of
duplicating them, and the external:-pointer CI question was answered with evidence.
