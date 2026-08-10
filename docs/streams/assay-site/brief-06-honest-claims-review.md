---
brief: assay-site/06
title: Honest-claims review — F-08 no-overclaim gate over the rendered copy
why: >-
  Assay's whole moat is that it refuses to overclaim — "derived, not measured", "machine-visible,
  not trustworthy", no leverage number it can't recompute (product-brief Honest limits; F-08).
  A landing page that quietly breaks that bar would contradict the product it sells. This brief
  is the mechanical + human backstop: it greps the rendered page for banned framings and records
  a claim-by-claim honesty pass before publication is ever considered.
wave: 4
depends: ["assay-site/04", "assay-site/05"]
unblocks: ["assay-site/07"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["web/site/index.html (assay-site/04 + 05 — the final rendered copy)", "docs/product-brief.md (Honest limits — the binding claims)", "docs/market-analysis.md §0 (F-08 honest framing, binding on all product copy)", "docs/site-messaging.md §Voice guardrails (assay-site/01 — the banned/required list)"]
---

# Brief 06 — Honest-claims review

## Context
files: web/site/index.html (READ), docs/streams/assay-site/honest-claims-review.md (NEW — the review record)
facts:
- THIS IS A REVIEW BRIEF, not a copy edit. It (a) runs a mechanical no-overclaim grep over the
  final page, (b) records a claim-by-claim honesty pass. If it finds a banned framing, it files
  the fix as a mid-flight tweak against the owning brief (04/05) — it does not silently rewrite.
- BANNED FRAMINGS (F-08 — must be ABSENT from web/site/index.html as actual claims):
  "tamper-evident" / "unforgeable", "cannot lie" / "can't lie", "measured ground truth", "makes
  agents trustworthy", and any published productivity-multiplier / leverage number (e.g.
  "10x", "Nx faster", an engineer-equivalent figure presented as fact). The teaser's separate
  metrics caveats (web/teaser/README.md) are the precedent: a leverage number is publishable
  only when recomputed from a ledger artifact — the landing page publishes none.
- REQUIRED FRAMING (must be PRESENT): the derived-not-measured claim ("derived") and the
  machine-visible-not-trustworthy framing ("machine-visible").
- gate = model (per the stream's derive-honestly rule: content briefs are model-gated with the
  standing human review gate owning quality; PUBLICATION is separately human-gated at brief 07).
  This brief does NOT publish anything — it gates the copy that a human will later publish.
- HONEST NOTE: the grep proves banned TOKENS are absent; it cannot prove a sentence is honest.
  The review record's claim-by-claim pass is the substance; the human review gate signs it off.
  State this in the review record — it is exactly the presence-vs-quality distinction the
  methodology exists to keep visible.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Run the no-overclaim grep over web/site/index.html (Verify row 1). If any banned framing is
   present, file the fix against brief 04 or 05 (mid-flight tweak, same commit if it changes
   their Verify) and re-run — do not ship this brief with a banned framing on the page.
2. Write docs/streams/assay-site/honest-claims-review.md: a claim-by-claim pass over the page's
   substantive claims (each: claim quoted, verdict honest/overclaim, note), plus the explicit
   presence-vs-quality caveat and a pointer that publication is brief 07's human gate.
3. Add the review record to the README docs/ index (or the stream's own listing).

## Verify (executable — no prose-only DoD items)
Review deliverable: PRESENCE + mechanical no-overclaim gates; the honesty JUDGEMENT is the
human review gate's call — this table proves the banned tokens are absent and the record exists.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f web/site/index.html && ! grep -Eiq -e "un-?forg" -e "cannot lie" -e "can't lie" -e "measured ground truth" -e "makes agents trustworthy" web/site/index.html` | exit 0 (no banned overclaim framings on the page). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 2 | `test -f web/site/index.html && ! grep -Eiq -e "[0-9]+x faster" -e "[0-9]+x with" web/site/index.html` | exit 0 (no published leverage/multiplier number). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 3 | `grep -ic "derived" web/site/index.html` | ≥ `1` (required honest claim present) |
| 4 | `grep -ic "machine-visible" web/site/index.html` | ≥ `1` (required honest framing present) |
| 5 | `test -f docs/streams/assay-site/honest-claims-review.md; echo $?` | `0` (the review record exists) |
| 6 | `grep -Eic -e "presence" -e "quality" docs/streams/assay-site/honest-claims-review.md` | ≥ `1` (the presence-vs-quality caveat is recorded) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns the honesty judgement over each claim; rows 1–2 only prove the
banned framings are mechanically absent.
