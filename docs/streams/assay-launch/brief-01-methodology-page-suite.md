---
brief: assay-launch/01
title: Methodology page suite — add the overview + missing mechanism pages, deepen the migrated ones
why: >-
  The migrated site (assay-launch/06) has 4 explainer pages — briefs, lifecycle, registers,
  statusgen — but they are landing-depth one-pagers, there is no high-level overview that ties
  them together, and two mechanisms (the desk roles and model-tiering) have no page at all. human:<name>
  asked for a suite that starts high-level then goes deep on each mechanism; this closes that gap
  so "publish our methodology" teaches it, not just names it.
wave: 1
depends: ["assay-launch/06", "methodology/09", "methodology/10", "methodology/11"]
unblocks: ["assay-launch/02", "assay-launch/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s publish-the-methodology direction)
sources: ["human:<name> 2026-07-17: 'a suite of pages around our methodology, starting with a high level view, and then pages going into more detail on each one'", "assay-launch/06 (migrates the existing 4 explainers + landing into ../assay-site — this brief builds ON them)", "assay-toolkit 5c4eb49 (the 4 existing explainers: briefs/lifecycle/registers/statusgen — the starting point, landing-depth)", "methodology/09,10,11 (the three drafted articles in docs/articles/ — narrative source for the depth)", "docs/streams/methodology/README.md §11 (the mechanisms) + the desk skills (desk-roles page source)", "F-08/F-12 honest-framing constraint (binding on all copy)", "docs/brand-guide.md (WHITE background, blue accent)", "freshness-checked 2026-07-17 @ cc7fd623: 4 explainers exist (5c4eb49); no overview / desk-roles / tiering pages"]
exec-tier: strong
exec-tier-why: (a) the overview information architecture + deepening copy under the honest-framing constraint are not fully pre-specified by the facts.
---

# Brief 01 — Methodology page suite (overview + missing pages + deepen)

**Builds on the migrated site.** assay-launch/06 migrates the 4 existing explainers
(`briefs`/`lifecycle`/`registers`/`statusgen`) + the landing into `../assay-site`. This brief adds
what's missing and adds depth — it does not rebuild what exists.

**CROSS-REPO:** pages land in the standalone `../assay-site` repo (migrated + scaffolded by
`assay-launch/06`). This repo: stream-row update only.

## Context
files: ../assay-site/ (new pages: `methodology.html` overview, `desk-roles.html`, `tiering.html`;
edits to the 4 migrated explainers); ../assay-toolkit/README.md + the desk skills (sources)
facts:
- **What exists (migrated by 06):** 4 explainer pages — briefs, lifecycle, registers, statusgen —
  at landing depth, plus the landing (assay-product/05). **What's missing (this brief):**
  (1) a high-level **overview** page ("how the methodology works" in one screen — Briefs → Board →
  Gates loop, the register triad, the lifecycle); (2) a **desk-roles** page (author / batch-fanout
  / pr-review / verify); (3) a **model-tiering-as-policy** page. Target: 6 detail pages total
  (4 existing + 2 new) under one overview.
- **Deepen the 4 existing** where they are landing-thin: each detail page should carry what it is,
  why it exists, how it works, and a concrete example drawn from the adoption docs / the three
  drafted articles — not just a paragraph.
- **Source, do not invent.** Narrative draws from the three articles (`docs/articles/
  status-build-artifact.md`, `prevention-and-reconciliation.md`, `specs-that-converge.md`) and the
  toolkit adoption docs. Any claim not traceable to `assay-product/01` or the toolkit README is cut.
- **SaaS / Enterprise OUT (human:<name>, 2026-07-17):** free / open-core toolkit methodology only — no Desk
  Console, no SaaS/Enterprise tier, no pricing copy.
- **Honest-framing binding (F-08/F-12):** the "derived from agent-authored artifacts with
  consistency linting, not measured ground truth" framing appears on the overview; no ground-truth
  claims; no unsourced ratio/leverage numbers.
- **Match 06's conventions:** use the migrated `assay.css` + self-hosted fonts, static zero-build,
  no CDN/JS; must render fully offline. The three articles' PUBLICATION stays behind
  methodology/09–11's gate — build "further reading" slots, wire only when human:<name> publishes.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-site commits local; publish is human:<name>'s, via assay-launch/05).
- Stop at `implemented` — you do not set verified/done. Do NOT make any repo public, host, or
  change DNS. Prepare to local-preview and STOP.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the overview page (`../assay-site/methodology.html`) — the one-screen "how it works" with
   the honest-framing paragraph; link it from the landing.
2. Add the two missing mechanism pages: `desk-roles.html`, `tiering.html`.
3. Deepen the 4 migrated explainers (briefs/lifecycle/registers/statusgen) to full detail per the
   facts (what/why/how/example).
4. Cross-link: overview → each detail page and back; landing → overview. Add the "further reading"
   article slots (unwired until human:<name> publishes).
5. Update the stream-README row; report the local-preview command in Evidence.

## Verify (executable — presence gates; design quality owned by review + human:<name>)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-site/methodology.html && test -f ../assay-site/desk-roles.html && test -f ../assay-site/tiering.html` | exit 0 (overview + 2 new pages) |
| 2 | `ls ../assay-site/*.html \| wc -l` | ≥8 (landing + overview + 6 detail pages) |
| 3 | `grep -riE -e "agent-authored" ../assay-site/methodology.html \| wc -l` | ≥1 (honest framing on the overview) |
| 4 | `grep -riE -e "is ground truth" -e "measured ground truth[^.]" -e "30:1" ../assay-site/*.html \| wc -l` | 0 (F-08/F-12 violations absent) |
| 5 | `grep -riE -e "desk console" -e "\benterprise\b" -e "\bsaas\b" -e "pricing" ../assay-site/*.html \| wc -l` | 0 (SaaS/Enterprise absent — human:<name> 2026-07-17) |
| 6 | `for p in briefs lifecycle registers statusgen desk tiering; do ls ../assay-site/ \| grep -qi "$p" && echo ok; done \| wc -l` | 6 (all six mechanism pages present) |
| 7 | `python3 -m http.server -d ../assay-site 8099 & sleep 1; curl -sf localhost:8099/methodology.html >/dev/null; echo $?; kill %1` | 0 (renders offline) |
| 8 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Copy is reviewed against
the F-08/F-12 honest-framing constraint and traceability to assay-product/01.
