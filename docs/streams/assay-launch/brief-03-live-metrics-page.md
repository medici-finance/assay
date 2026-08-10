---
brief: assay-launch/03
title: Public "how it runs" live-metrics page — DORA/trend snapshot
why: >-
  human:<name> asked to show "the metrics we are collecting today showing how it runs." We already
  compute DORA + trend + a daily harvest — but only for ourselves; nothing renders them for
  a visitor. A public metrics page turns "trust us, the methodology works" into "here is it
  running" — the single most credible thing on the site, and the honest-framing makes it
  defensible rather than a vanity dashboard.
wave: 1
depends: ["assay-launch/06", "methodology-metrics/02", "methodology-metrics/03", "methodology-metrics/22"]
unblocks: ["assay-launch/05", "assay-launch/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s publish-the-methodology direction)
sources: ["human:<name> 2026-07-17: 'the metrics we are collecting today showing how it runs'", "methodology-metrics/02 (statusgen --dora, done — 5 DORA metrics)", "methodology-metrics/03 (statusgen --trend, done — historian time series)", "methodology-metrics/22 (daily artifact harvest, implemented — the AI-free collector that will feed a live snapshot; assay-launch/07 wires it to auto-publish)", "assay-launch/06 (the assay-site repo this page lands in)", "F-08/F-12 honest-framing (binding — DORA is 'diagnostic, not a target'; numbers derive from agent-authored artifacts)", "docs/brand-guide.md", "freshness-checked 2026-07-17 @ cc7fd623: no public metrics page exists in ../assay-site/", "distinct from I-28 (loop-monitoring HMI, scoped-to desk-tools) — that is an INTERNAL, private operator screen over the loops; this is the PUBLIC point-in-time metrics page"]
exec-tier: strong
exec-tier-why: (b) correctness depends on cross-artifact reasoning — the page consumes statusgen's --dora/--trend output format and must not misrepresent it.
---

# Brief 03 — Public "how it runs" live-metrics page

**CROSS-REPO:** the page lands in `../assay-site/`; it consumes output from
`tools/statusgen` (this repo) and the daily-harvest artifacts (methodology-metrics/22). This
repo: stream-row only (no metric-logic changes).

## Context
files: ../assay-site/ (new metrics page, e.g. `metrics.html`); consumes
`go run ./tools/statusgen --dora --json` and `--trend` output + the daily-harvest artifact
home (`docs/reports/daily/` per methodology-metrics/22)
facts:
- **Consumer, not a changer.** This page RENDERS existing statusgen output; it must not add or
  alter any metric logic in `tools/statusgen`. If it needs a machine-readable feed, `--dora
  --json` already exists (methodology-metrics/02); prefer it over scraping text.
- **Snapshot, not a live server.** The site is static (no build-time services). Render a
  point-in-time snapshot at build/render time from the harvest artifacts (methodology-metrics/22
  produces them AI-free on a schedule). "Live" = refreshed by the harvest cadence, not a
  websocket. State the as-of date on the page.
- **Honest-framing binding (F-08/F-12), extra-load-bearing here:** DORA is *diagnostic, not a
  target* (methodology-metrics/02's own framing); the numbers derive from agent-authored
  artifacts with consistency linting, not measured ground truth. The page says so plainly. No
  vanity framing, no "30:1", no cherry-picked window.
- Brand: WHITE background, blue accent; if charts are used, brand colors per
  `../oit/docs/brand-guide.md`, not library defaults.
- **Not the same as I-28** (loop-monitoring HMI, `scoped-to: desk-tools`): that is an
  internal, private operator screen that must never expose transcript material. This page is
  a curated PUBLIC snapshot of aggregate DORA/trend numbers only.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. Do NOT change metric logic in tools/statusgen.
- Stop at `implemented` — you do not set verified/done. Do not publish; that is /05.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build `../assay-site/metrics.html` rendering a DORA snapshot (the 5 metrics) + a
   short trend view, from `statusgen --dora --json` / the daily-harvest artifacts.
2. Put the honest-framing + "diagnostic, not a target" + as-of-date statements on the page.
3. Link it from the landing + overview pages ("how it runs").
4. Document the one-command regen step (how a maintainer refreshes the snapshot from the
   harvest) in a comment or the site README.
5. Update the stream-README row.

## Verify (executable — presence + flow gates)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-site/metrics.html` | exit 0 |
| 2 | `grep -riE -e "diagnostic, not a target" -e "agent-authored" ../assay-site/metrics.html \| wc -l` | ≥1 (honest framing present) |
| 3 | `statusgen --root . --dora --json > /tmp/dora.json; jq -e '.' /tmp/dora.json >/dev/null; echo $?` | 0 (the feed the page consumes actually emits — flow row) |
| 4 | `grep -riE -e "as of" -e "as-of" -e "snapshot" ../assay-site/metrics.html \| wc -l` | ≥1 (as-of date stated) |
| 5 | `git -C . diff --name-only origin/main -- tools/statusgen \| wc -l` | 0 (no metric-logic changes in statusgen) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. -->

## Review
Gate: model. Reviewer confirms the page does not overclaim, states diagnostic framing + as-of
date, and reads statusgen output rather than duplicating metric logic.
