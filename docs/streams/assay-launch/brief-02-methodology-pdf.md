---
brief: assay-launch/02
title: Downloadable methodology PDF — render + wire to a site download link
why: >-
  human:<name> asked for a PDF people can download that explains the methodology. A downloadable
  artifact is how a methodology travels beyond a browser tab — it gets forwarded, read
  offline, attached to a pitch. Without it the "publish our methodology" effort has no
  take-away, only a website someone has to be online to read.
wave: 2
depends: ["assay-launch/01"]
unblocks: ["assay-launch/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s publish-the-methodology direction)
sources: ["human:<name> 2026-07-17: 'a PDF that people can download that explains it'", "assay-launch/01 (the page-suite content is the PDF's source — one source of truth, two renders)", "../decks/tools/render-whitepaper-pdf.cjs (existing whitepaper→PDF renderer; freshness-checked 2026-07-17 — present)", "whitepaper-program (3 whitepapers, sources docs/articles/; PDFs are WHITE background w/ black logo banner)", "F-08/F-12 honest-framing (binding on PDF copy)", "docs/brand-guide.md"]
exec-tier: any
exec-tier-why: assembles existing page-suite content through an existing renderer; no design decisions left open once 01 lands.
---

# Brief 02 — Downloadable methodology PDF

**CROSS-REPO:** the PDF source + render step live in `../decks/` (the whitepaper pipeline);
the download link lands in `../assay-site/` (the site). This repo: stream-row only.

## Context
files: ../decks/ (whitepaper source dir + `tools/render-whitepaper-pdf.cjs`); the generated
PDF's home under `../assay-site/assets/` (e.g. `assets/assay-methodology.pdf`);
../assay-site/ landing + overview (the download link)
facts:
- **One source of truth.** The PDF is a render of `assay-launch/01`'s page-suite content
  (overview + the mechanism detail pages), NOT a separately-written document — so the two
  never drift. Assemble the markdown/HTML the renderer consumes from 01's pages.
- **Use the existing renderer.** `../decks/tools/render-whitepaper-pdf.cjs` is the whitepaper
  pipeline (per the whitepaper-program: PDFs are WHITE background with the black-logo banner).
  Do NOT introduce a new PDF toolchain — match how the existing whitepapers render.
- **Honest-framing binding (F-08/F-12):** the framing paragraph appears in the PDF; no ground-
  truth claims, no unsourced ratios. Same review-cut as the pages.
- **Wire the download.** A "Download the methodology (PDF)" link on the landing page and the
  overview page, pointing at the checked-in PDF asset. The link is present but the site stays
  unpublished until assay-launch/05.
- Brand: WHITE background, blue accent, black-logo banner (whitepaper convention +
  `../oit/docs/brand-guide.md`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (decks + assay-toolkit commits local; publish is human:<name>'s, via /05).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Assemble the PDF source from assay-launch/01's page-suite content (single source of truth).
2. Render it via `../decks/tools/render-whitepaper-pdf.cjs` into a checked-in asset under
   `../assay-site/assets/` (match the whitepaper render conventions: WHITE bg, banner).
3. Add a "Download the methodology (PDF)" link on the landing + overview pages.
4. Update the stream-README row; record the render command + output path in Evidence.

## Verify (executable — presence gates; design quality owned by review + human:<name>)
| # | Command | Expect |
|---|---------|--------|
| 1 | `ls ../assay-site/assets/*.pdf \| wc -l` | ≥1 (PDF rendered + checked in) |
| 2 | `file ../assay-site/assets/assay-methodology.pdf` | output contains "PDF document" |
| 3 | `grep -riE "download.*pdf" ../assay-site/index.html ../assay-site/methodology.html \| wc -l` | ≥1 (download link wired) |
| 4 | `grep -c "render-whitepaper-pdf" ../decks/tools/render-whitepaper-pdf.cjs` | ≥1 (renderer used, not replaced — no new toolchain file added) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Confirm the PDF
content matches 01's pages (no drift) and honest-framing holds.
