---
brief: assay-site/04
title: Landing page build — static self-contained brand-compliant HTML
why: >-
  With the messaging spine, the three-tier lede, and the four explainers all decided in prose,
  the page can be built as a single hand-authored, self-contained HTML file that renders them
  in the brand system. Static + self-contained is the honest match for the host options
  (Cloudflare Pages / GitHub Pages, naming-clearance §9.2) and mirrors the existing web/teaser
  asset, so there is no framework or build step to carry.
wave: 2
depends: ["assay-site/01", "assay-site/02", "assay-site/03"]
unblocks: ["assay-site/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["docs/site-messaging.md (assay-site/01 IA + hero)", "docs/site-three-tier.md (assay-site/02)", "docs/site-explainers.md (assay-site/03)", "web/teaser/index.html + web/teaser/README.md (the brand system + homepage-hero source; no docs/brand-guide.md exists)", "docs/naming-clearance.md §9.2 (static-host options — informs self-contained/no-build)"]
---

# Brief 04 — Landing page build

## Context
files: web/site/index.html (NEW), web/site/README.md (NEW)
facts:
- BUILD = render, not re-decide. The page renders the decided content: hero (site-messaging.md)
  → three-tier section (site-three-tier.md) → what-it-does explainers (site-explainers.md) →
  a placeholder CTA band that brief 05 wires. If any content doc is missing a decided element,
  report NEEDS_CONTEXT — do not invent copy.
- SELF-CONTAINED: one index.html, no external fonts/scripts/CDN, inline CSS (a static host
  serves it as-is). Mirror web/teaser/index.html's structure and constraints.
- BRAND (reuse, do not reinvent — no docs/brand-guide.md exists): lift the CSS system from
  web/teaser/index.html — the CSS variables, Electric Blue #3366FF lead, Gold #D4A843 as the
  single Assay differentiator, theme-aware (light + dark via the same data-theme /
  prefers-color-scheme pattern the teaser uses), "Assay by Medici" branded-house. The teaser is
  named the homepage-hero source in web/teaser/README.md — reuse its hero, extend downward.
- SECTION ORDER is brief 01's IA, exactly. The THREE-TIER section is the headline block
  (immediately after the hero), visually the most prominent of the body sections.
- HONEST COPY (F-08, binding): the rendered page must NOT contain "tamper-evident", "cannot
  lie"/"can't lie", "measured ground truth", "makes agents trustworthy", or any published
  leverage/multiplier number. Brief 06 gates this mechanically; author to pass it.
- web/site/README.md: describe the page (what it is, brand basis = teaser, honest-content
  note), same shape as web/teaser/README.md, so the asset is discoverable and its provenance
  clear. This is the web-asset discoverability surface (llms.txt/docs-site does not apply here).
- RESPONSIVE: max-width container, relative units, wide content scrolls in its own box — no
  horizontal body scroll (match the teaser).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build web/site/index.html: a single self-contained page rendering the hero + three-tier
   section + four explainers, in the teaser's brand system, theme-aware, responsive. Leave a
   clearly-marked CTA band for brief 05 to wire.
2. Write web/site/README.md (what it is, brand basis, honest-content note) — mirror
   web/teaser/README.md.
3. Add web/site/ to the README docs/ index (or the README's web-assets listing) so it is
   discoverable.

## Verify (executable — no prose-only DoD items)
Prose/HTML deliverable: PRESENCE + structural gates (elements exist, no external deps, banned
framings absent); visual quality is the human review gate's call.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f web/site/index.html && test -f web/site/README.md; echo $?` | `0` |
| 2 | `grep -Eic -e "desk solo" -e "desk console" -e "on-prem" web/site/index.html` | ≥ `3` (three-tier section rendered) |
| 3 | `grep -Eic -e "brief" -e "register" -e "lifecycle" -e "statusgen" web/site/index.html` | ≥ `4` (explainers rendered) |
| 4 | `grep -ic "derived" web/site/index.html` | ≥ `1` (the honest hero claim is on the page) |
| 5 | `test -f web/site/index.html && ! grep -Eiq -e "<script[^>]*src=" -e "<link[^>]*stylesheet" -e "fonts.googleapis" -e "@import" web/site/index.html` | exit 0 (self-contained — no external script/stylesheet/font loads; note: external ANCHOR links to GitHub/articles are fine and NOT matched here). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 6 | `test -f web/site/index.html && ! grep -Eiq -e "un-?forg" -e "cannot lie" -e "can't lie" -e "measured ground truth" -e "makes agents trustworthy" web/site/index.html` | exit 0 (no banned overclaim framings). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 7 | `grep -c "3366FF" web/site/index.html` | ≥ `1` (Electric Blue brand token present — brand system reused) |
| 8 | `grep -q "web/site" README.md; echo $?` | `0` (asset listed/linked in README) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns visual + copy quality; the grep rows only prove the elements exist
and the banned framings are absent.
