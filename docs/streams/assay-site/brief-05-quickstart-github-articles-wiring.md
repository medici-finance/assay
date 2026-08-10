---
brief: assay-site/05
title: Quickstart / GitHub / articles wiring — outbound CTAs, nav, footer, links resolve
why: >-
  A landing page that describes the product but gives a visitor nowhere to go is a dead end.
  Roadmap 2.1/D5 names the required outbound destinations — quickstart, GitHub, article links.
  This brief wires the CTA band and footer so every destination the page promises actually
  resolves, and so the three-tier section's per-tier links point somewhere real (repo/quickstart
  now; setup pages when they ship — never a dead link).
wave: 3
depends: ["assay-site/04"]
unblocks: ["assay-site/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["web/site/index.html (assay-site/04 — the built page + its CTA band placeholder)", "docs/product-roadmap.md 2.1/D5 (quickstart link, GitHub link, article links)", "README.md § Quickstart (the adopter path this links to)", "docs/articles/ (the article destinations)", "docs/site-three-tier.md (per-tier link targets — repo/quickstart now, setup pages when they ship)"]
---

# Brief 05 — Quickstart / GitHub / articles wiring

## Context
files: web/site/index.html (EDIT — the CTA band + footer/nav)
facts:
- SCOPE: wire the outbound destinations into the page built by brief 04. This is link + CTA
  work, not new sections. Fill the CTA band brief 04 left marked.
- REQUIRED DESTINATIONS (roadmap 2.1/D5): (a) **Quickstart** → the repo README § Quickstart
  (the adopter path); (b) **GitHub** → the repo (`medici-finance/assay-toolkit`); (c)
  **Articles** → the docs/articles/ pieces (today: docs/articles/assay-and-jira.md; link
  whatever exists — do not invent titles).
- PER-TIER LINKS (from site-three-tier.md): FREE → repo Quickstart (+ the Desk Solo setup page
  WHEN desk-solo/03 ships; until then the repo/design, never a dead link); PREMIUM / ON-PREM →
  a commercial-contact CTA (no fabricated signup flow).
- HONEST LINK RULE: every href must resolve to something real. No placeholder `href="#"`
  leading nowhere for a primary CTA, no invented URLs, no linking un-shipped setup pages as if
  live. A "coming soon" CTA is allowed if it reads as such.
- Keep the page self-contained (no new external resource loads — anchors to github.com etc. are
  fine); keep the brand system and theme-awareness intact.
- This brief EDITS an existing HTML file (web/site/index.html). It changes the page's copy
  surface, so brief 06 (honest-claims review) runs AFTER it — do not add banned framings.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Wire the CTA band + footer/nav in web/site/index.html: a Quickstart link (repo README §
   Quickstart), a GitHub link (medici-finance/assay-toolkit), and article link(s) to the
   docs/articles/ pieces that exist.
2. Point the three-tier section's per-tier links at real targets (repo/quickstart now;
   commercial-contact CTA for the paid tiers; no dead links, no fabricated URLs).
3. Keep it self-contained, brand-consistent, and free of banned overclaim framings.

## Verify (executable — no prose-only DoD items)
HTML deliverable: PRESENCE + link-integrity gates; visual/copy quality is the human review
gate's call.
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "assay-toolkit" web/site/index.html` | ≥ `1` (GitHub link to the repo present) |
| 2 | `grep -Eic -e "quickstart" -e "get started" web/site/index.html` | ≥ `1` (quickstart CTA present) |
| 3 | `grep -Eic -e "article" -e "assay-and-jira" web/site/index.html` | ≥ `1` (article link present) |
| 4 | `test -f web/site/index.html && ! grep -Eiq -e "<script[^>]*src=" -e "<link[^>]*stylesheet" -e "fonts.googleapis" web/site/index.html` | exit 0 (still self-contained after wiring). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 5 | `test -f web/site/index.html && ! grep -q 'href="#"' web/site/index.html` | exit 0 (no CTA points at a dead `#` anchor). Guarded by `test -f web/site/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns whether the CTAs read well; the grep rows only prove the links exist
and resolve to real targets.
