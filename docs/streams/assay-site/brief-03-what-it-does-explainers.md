---
brief: assay-site/03
title: What-it-does explainers — briefs, registers, lifecycle, statusgen
why: >-
  After the honest hero and the three-tier lede, a visitor who wants to know what the thing
  actually IS needs four short, concrete explainers — briefs, registers, lifecycle, statusgen
  — each answering "what is this and what does it buy me" in a couple of sentences a
  non-engineer can follow. Roadmap 2.1/D5 names exactly these four as the page's what-it-does
  explainers; this brief writes them once so the build renders decided copy.
wave: 1
depends: ["assay-site/01"]
unblocks: ["assay-site/04"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["docs/site-messaging.md (assay-site/01 — IA slot + voice guardrails)", "docs/product-roadmap.md 2.1/D5 (the four explainers named)", "docs/product-brief.md (The product surface — the three things that ship)", "docs/market-analysis.md §0 (briefs/registers/lifecycle/statusgen definitions)", "docs/brief-template.md, docs/lifecycle.md, docs/registers.md (the source definitions)"]
---

# Brief 03 — What-it-does explainers

## Context
files: docs/site-explainers.md (NEW)
facts:
- CONTENT SPEC (not HTML): four short explainer blocks, rendered by brief 04, slotting into the
  IA position brief 01 assigned (after the three-tier section).
- THE FOUR EXPLAINERS (one concrete paragraph each, plain-language):
  1. **Briefs** — self-contained scope-and-DoD units with typed dependencies, derived risk
     gates, and an executable Verify block. Buys: any one agent can execute a piece of work
     without the whole plan in context, and "done" has a machine-checkable definition.
  2. **Registers** — append-only FINDINGS / INTAKE / RETRO logs with tombstone-not-delete and
     sequence-contiguity enforcement. Buys: new knowledge that invalidates in-flight work is
     tracked and can't be silently removed (the day-one falsification incident is why).
  3. **Lifecycle** — todo → in-progress → implemented → verified → done, where **verified is a
     distinct step run by a non-implementer** and done additionally requires a recorded review.
     Buys: agents don't grade their own homework.
  4. **statusgen** — a Go tool that *generates* the STATUS board + a cross-stream Next-up
     queue and lints the whole set in CI as the board's single writer. Buys: status is a build
     artifact, so a stale or fabricated board is a lint failure, not a matter of diligence.
- VOICE (F-08, from brief 01 guardrails): the honest frame is binding here too — statusgen
  makes drift/tampering *machine-visible*; it does not make agents trustworthy. No banned
  framings ("tamper-evident", "measured ground truth", etc.).
- Keep each explainer non-jargon and short (this is a landing page, not the docs). Link each to
  its canonical doc (brief-template.md / registers.md / lifecycle.md / statusgen/README.md) for
  the reader who wants depth.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write docs/site-explainers.md: the four explainer blocks (briefs, registers, lifecycle,
   statusgen), one concrete plain-language paragraph each, each with a "what it buys you" line
   and a link to its canonical doc.
2. Keep the honest frame (machine-visible, not trustworthy); no banned framings.
3. Add the page to the README docs/ index.

## Verify (executable — no prose-only DoD items)
Prose deliverable: PRESENCE gate (the four explainers + their tokens exist); prose quality is
the human review gate's call.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/site-explainers.md; echo $?` | `0` |
| 2 | `grep -Eic -e "brief" -e "register" -e "lifecycle" -e "statusgen" docs/site-explainers.md` | ≥ `4` (all four explainers present) |
| 3 | `grep -ic "verified" docs/site-explainers.md` | ≥ `1` (the non-implementer verify step is called out) |
| 4 | `grep -Eic -e "machine-visible" -e "single writer" docs/site-explainers.md` | ≥ `1` (honest statusgen framing present) |
| 5 | `grep -q "site-explainers.md" README.md; echo $?` | `0` (linked in the docs/ index) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns claim-quality; the grep rows only prove the elements exist.
