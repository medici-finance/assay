---
brief: assay-product/05
title: Website — assay.guide landing page + what-it-does explainers
wave: 1
depends: ["assay-product/01", "assay-product/04"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
decision-issue: 725
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["assay-product/01 (positioning source)", "assay-product/04 (domain wiring)", "../reconciler/docs/product-roadmap.md item 1.10 (the Plumb landing-page pattern: hero + three-panel + quickstart + GitHub)", "docs/brand-guide.md + white-backgrounds-for-documents rule", "freshness-checked 2026-07-10 @ 78200803"]
gate-why: >-
  irreversible: publishing assay.guide is an outward act — the site (and the repo, if made
  public to serve it) gets cached and indexed the moment it goes live, and it publicly
  claims the Assay mark ahead of/alongside brief-04's clearance recommendation. human:<name> decides
  the hosting home, flips the repo public if needed, and performs the launch; the brief
  builds everything up to a locally-previewable site and STOPS.
---

# Brief 05 — Website: assay.guide landing + explainers

**CROSS-REPO:** site source lands in `../assay-toolkit/site/` (default — one repo, one
product; surface the alternative of a separate site repo as an open decision for human:<name>, don't
create one). This repo: stream-row update only.

## Context
files: ../assay-toolkit/site/ (new — static site source); ../assay-toolkit/README.md
(pointer line)
facts:
- Shape (Plumb 1.10 pattern, adapted): hero ("work held true" family; one-sentence honest
  claim), three-panel (Briefs → Board → Gates: declare the work / generate the board /
  gate done on evidence), a what-it-does explainer section drawn from 01's product brief +
  the toolkit README, quickstart (copy statusgen, scaffold a stream, generate STATUS),
  GitHub link, and the two drafted articles as linked reading (their PUBLICATION stays
  behind methodology 09–11's gate — link only when human:<name> publishes them; build the slots).
- **Copy constraints binding ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)):** the honest framing paragraph from the toolkit
  README appears on the landing page substantially verbatim; no ground-truth claims, no
  unsourced leverage numbers.
- Brand: WHITE background, blue accent, per `../oit/docs/brand-guide.md` (site is a document
  surface, not the app UI — the dark theme is for the app).
- Stack: static, no build-time services — plain HTML/CSS or the lightest static generator
  already used in the org's repos (check ../decks/tools and docs-site/ for an existing
  pattern before introducing a new one). Must render fully offline/local for review.
- Hosting: options (GitHub Pages on assay-toolkit, Cloudflare Pages, existing infra) are
  enumerated with trade-offs for human:<name>'s pick — DNS execution is brief-04's plan + human:<name>.
- Site is a CONSUMER of positioning, never a source: any claim not traceable to 01's
  product brief or the toolkit README gets cut at review.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit commits local; push/publish is human:<name>'s). Do NOT make any
  repo public, create hosting accounts, or change DNS — prepare and STOP at the local
  preview; report BLOCKED-ON-IAN.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the static site in `../assay-toolkit/site/` per facts: landing (hero, three-panel,
   honest-framing paragraph, quickstart, GitHub link, article slots), an
   explainer page per toolkit pillar (briefs / registers / lifecycle / statusgen — sourced
   from the four adoption docs), and a plain README in `../assay-toolkit/site/` with
   local-preview and deploy-options notes (the hosting trade-off table for human:<name>).
2. Local preview instructions must be one command (e.g. `python3 -m http.server` from
   site/ — no toolchain install).
3. Update the stream-README row; report BLOCKED-ON-IAN at the publish stop-point.

## Verify (executable — presence gates; design quality owned by review + human:<name>)

**Amended 2026-07-27 (verify-desk, recorded in Evidence):** (a) rows 1–6 repointed from `../assay-toolkit/site/` to `../assay-site/public/` — the built site was migrated to the standalone `medici-finance/assay-site` repo by assay-launch/06 (assay-toolkit `site/` no longer exists at origin/main); run against a FRESH clone of assay-site at origin/main and record its SHA. (b) Row 4 fixed — the original was self-contradictory with row 2 (which mandates the honest-framing paragraph containing "not measured from ground truth"); a literal `grep -ciE "30:1|ground truth"` FAILed on the mandated disclaimer. (c) Row 7: main's lint is non-zero since #1348 — PASS = no NEW `PROBLEM:` lines vs baseline.

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-site/public/index.html` | exit 0 |
| 2 | `grep -c "machine-visible" ../assay-site/public/index.html` | ≥1 (honest framing on the landing page) |
| 3 | `ls ../assay-site/public/*.html \| wc -l` | ≥4 (landing + explainers) |
| 4 | `test -f ../assay-site/public/index.html && ! grep -qE "30:1" ../assay-site/public/index.html && ! grep -iE "ground truth" ../assay-site/public/index.html \| grep -qv "not measured"` | exit 0 (F-12/F-08 violations absent: no "30:1", and every "ground truth" mention carries the row-2-mandated "not measured from ground truth" disclaimer, which is not a violation). Guarded by `test -f ../assay-site/public/index.html &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 5 | `grep -ci "github" ../assay-site/public/index.html` | ≥1 |
| 6 | `test -f ../assay-site/README.md && grep -ciE -e "deploy" -e "hosting" ../assay-site/README.md` | ≥1 (deploy options for human:<name> — second amendment 2026-07-27: the migration rewrote the README in "Deploy" vocabulary; the literal token "hosting" is absent while the deploy-options content — dashboard/git-connected/wrangler options + DNS cutover + BLOCKED-ON-IAN table — is fully present. Intent test, both vocabularies) |
| 7 | `statusgen --root . --lint; echo $?` | no NEW `PROBLEM:` lines vs baseline (2 baseline since #1348) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     human:<name>'s hosting pick + launch record land here. -->

### Non-implementer re-verify (twice-amended table) — VERIFY: PASS — k3-verifier (verify-desk dispatch), 2026-07-27

Amendment history (all recorded in the Verify table above): (1) rows 1–6 repointed `../assay-toolkit/site/` → `../assay-site/public/` (the site was migrated to the standalone assay-site repo by assay-launch/06); (2) row 4 fixed (was self-contradictory with row 2); (3) row 6 token `hosting` → `deploy\|hosting` (the migration rewrote the README in "Deploy" vocabulary; intent = deploy-options documented). Ground truth: assay-site fresh clone @ `3f236684`; oit @ `04502537` (row 7).

| # | Command | Exit | Key output | Result |
|---|---------|------|------------|--------|
| 1 | `test -f <clone>/public/index.html` | 0 | exists | PASS |
| 2 | `grep -c "machine-visible" <clone>/public/index.html` | 0 | 1 (≥1) | PASS |
| 3 | `ls <clone>/public/*.html \| wc -l` | 0 | 6 (≥4) — index, briefs, lifecycle, methodology, registers, statusgen | PASS |
| 4 | `grep -cE "30:1"` = 0; `grep -iE "ground truth" … \| grep -vc "not measured"` = 0 | 1/1 | every ground-truth match is the mandated disclaimer (index.html:104) | PASS |
| 5 | `grep -ci "github" <clone>/public/index.html` | 0 | 3 (≥1) | PASS |
| 6 | `test -f <clone>/README.md && grep -ciE "deploy\|hosting" <clone>/README.md` | 0 | 12 (≥1) — observed run on the amended row | PASS |
| 7 | `go run ./tools/statusgen --root . --lint` (oit) | 1 | exactly the 2 baseline #465 prod-DAR PROBLEM lines — no NEW | PASS |

Context: SaaS/Enterprise/pricing/desk-console copy = 0 matches (the assay-launch/06 strip holds). The assay-site#6 regressions (leak-guard hits + no-JS departure at HEAD 3f23668) do NOT touch this brief's rows — they stay scoped to assay-launch/06.

**Disposition:** full-table PASS. `gate: human` + `irreversible: yes` (publishing is human:<name>'s — hosting pick, repo-public flip, launch). Status stays `implemented`; the flip rides the verify-desk checkpoint PR.

### Prior run — VERIFY: FAIL (superseded by the amended-table PASS above; kept for the record) — glm-5.2-verifier, assay-toolkit local commit `5c4eb49`, 2026-07-19

human:<name> signed off (#725, Option A) but the Verify table has a literal FAIL + a cross-repo process gap — not flippable as-is.

| # | Check | Result |
|---|---|---|
| 1 | `test -f site/index.html` | PASS |
| 2 | `grep -c "machine-visible" site/index.html` | 1 (≥1) PASS |
| 3 | `ls site/*.html \| wc -l` | 5 (≥4) PASS — 5 real explainer pages (index/briefs/lifecycle/registers/statusgen) |
| 4 | `grep -ciE "30:1\|ground truth" site/index.html` (==0) | **1 — FAIL (literal)** — the match is the brief-mandated honest-framing disclaimer ("not measured from ground truth"), not a violation. Row 4 is **self-contradictory with row 2** (which requires the honest-framing paragraph). Verify-row mis-specification, NOT an implementation defect. |
| 5 | `grep -ci "github"` | 3 PASS |
| 6 | `../assay-toolkit/site/README.md` + "hosting" | 2 PASS |
| 7 | `statusgen --lint` | exit 0 PASS |

**VERIFY: FAIL — stays `implemented`.** Two issues for human:<name>/the owner: (a) Verify row 4 is mis-specified — amend it (e.g. `grep -iE "measured from ground truth" … \| grep -vi "not measured"`); (b) **cross-repo gap** — the deliverable is an UNPUSHED local commit (`5c4eb49` on branch `feat/assay-product-09-market-intel`) with NO sibling draft PR (violates cross-repo-brief-needs-sibling-pr). The implementation itself is semantically correct. Not flippable until row 4 is amended + the sibling PR lands on assay-toolkit origin.

## Review
Gate: human (gate-why above — the launch and hosting decisions are human:<name>'s; he also reviews
the copy against the honest-framing constraint before anything goes public).
