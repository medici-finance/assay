---
brief: assay-launch/06
title: assay-site repo — migrate the built site + harden (mirror site-repo)
why: >-
  The Assay website already exists — a parallel session built the landing + 4 explainer pages in
  assay-toolkit/site (commit 5c4eb49, assay-product/05). human:<name> wants it served from a standalone
  assay-site repo like site-repo, published automatically by the daily jobs. This migrates that
  built site into the new repo and hardens it to site-repo's invariants (self-hosted fonts, leak
  guard, no CDN) — reusing the work rather than rebuilding, and fixing the two things that block a
  safe public launch: the Google-Fonts CDN dependency and the SaaS/Enterprise copy.
wave: 0
depends: ["assay-product/05"]
unblocks: ["assay-launch/01", "assay-launch/03", "assay-launch/05", "assay-launch/07"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s assay-site direction)
sources: ["human:<name> 2026-07-17: 'we need an assay-site repo, similar to site-repo' + 'leave the SaaS/Enterprise versions out of the website for now'", "assay-toolkit commit 5c4eb49 (the built site being migrated: index.html landing + briefs/lifecycle/registers/statusgen explainers + assay.css; assay-product/05, implemented)", "../site-repo (the target invariants: standalone repo, self-hosted woff2 fonts — SAME 3 families the built site CDN-loads: EB Garamond/Inter/JetBrains Mono, no CDN/JS, Cloudflare Pages byte-for-byte, predeploy-check.sh leak guard, AAA Flowers S.A. footer)", "audit 2026-07-17: built site loads fonts.googleapis.com/gstatic.com (CDN — must self-host) and carries 2 SaaS/Enterprise references (must strip)", "finding findings/2026-07-17-assay-site-standalone-repo", "freshness-checked 2026-07-17 @ cc7fd623: no ../assay-site repo exists yet"]
gate-why: >-
  irreversible + customer-facing: this stands up the public site's repo from the migrated build.
  Creating the medici-finance/assay-site GitHub repo is human:<name>'s act (new medici-finance repos must
  grant `human:<name>` admin at creation; the org classifier blocks the PUT unprompted), as is connecting
  it to Cloudflare Pages and pointing assay.guide DNS. The implementer migrates + hardens the site
  locally and STOPS at a local preview; human:<name> creates the repo, wires Pages, does the first deploy,
  and confirms the SaaS/Enterprise strip + the legal-entity/mark posture (AAA Flowers S.A.,
  operating as Medici; Assay mark asserted ahead of the deferred formal TM clearance).
exec-tier: strong
exec-tier-why: (b) migrating + hardening to site-repo's self-contained/leak-guard invariants (self-hosting the CDN fonts, stripping SaaS copy) is the foundation every downstream page trusts.
---

# Brief 06 — assay-site repo: migrate the built site + harden

**Migration, not a rebuild.** The site already exists in `../assay-toolkit/site/` (assay-toolkit
`5c4eb49`, assay-product/05 = implemented): `index.html` (landing "Work held true"), 4 explainers
(`briefs`/`lifecycle`/`registers`/`statusgen`), `assay.css`. This brief MOVES it to the standalone
`../assay-site` repo and hardens it to site-repo's invariants. assay-product/05 stays `implemented`
(the build is its output; this relocates + hardens it).

**CROSS-REPO (new repo):** source `../assay-toolkit/site/` → new `../assay-site` repo. GitHub repo
creation + Cloudflare Pages + DNS are human:<name>'s (gate-why). This repo (oit): stream-row
only.

## Context
files: ../assay-site/ (NEW standalone repo, populated from ../assay-toolkit/site/); add
`_headers`, `assets/fonts/*.woff2`, `predeploy-check.sh` (planned), `NOTES.md`; mirror `../site-repo/` shape
facts:
- **Migrate the 5 pages + `assay.css`** from `../assay-toolkit/site/` into `../assay-site/` (git
  init the new repo; keep the page structure). The 4 explainers cover 4 mechanisms
  (briefs/lifecycle/registers/statusgen) — assay-launch/01 adds the overview + the two missing
  ones (desk-roles, tiering) and deepens these.
- **Harden #1 — self-host fonts (blocking).** The built site loads EB Garamond / Inter / JetBrains
  Mono from `fonts.googleapis.com`/`gstatic.com` (a CDN — violates site-repo's no-CDN, offline,
  privacy invariants). Copy site-repo's self-hosted variable `woff2` of the SAME three families
  into `assets/fonts/`, rewrite `assay.css` `@font-face`/link to use them, remove the CDN `<link>`.
- **Harden #2 — strip SaaS/Enterprise (human:<name>, 2026-07-17).** The built site has 2 SaaS/Enterprise
  references; remove them — the public site fronts the FREE/open-core toolkit only.
- **Harden #3 — site-repo scaffolding.** Add `_headers` (Cloudflare Pages font cache + security
  headers), `predeploy-check.sh` (planned) (copy + adapt site-repo's comment leak-guard: denylist person
  names, `stream/NN` IDs, `-TBD`, dates, private-repo refs, internal `.md`/`§`), and a repo-internal
  `NOTES.md` kept OUT of the deployed set. Confirm no `<script>` (already JS-free) and no remaining
  external asset URLs except the GitHub link.
- **Legal-entity footer** "AAA Flowers S.A., operating as Medici" (as site-repo) on the landing.
  Brand: WHITE background, blue accent.

## Ground rules
- NEVER git push to a public remote / create a GitHub repo / connect hosting / change DNS. Build
  the local `../assay-site` repo (`git init`, commit locally), previewable, and report
  BLOCKED-ON-IAN for repo creation + Cloudflare Pages + DNS.
- Stop at `implemented` — you do not set verified/done. Do not make anything public.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `git init ../assay-site`; migrate the 5 pages + `assay.css` from `../assay-toolkit/site/`.
2. Self-host the three font families (copy site-repo's woff2), rewrite the CSS, drop the CDN link.
3. Strip the 2 SaaS/Enterprise references.
4. Add `_headers`, `predeploy-check.sh` (adapted), `README.md` (one-command preview + hosting
   trade-off table for human:<name>), `NOTES.md` (out of the deployed set).
5. Assemble the BLOCKED-ON-IAN checklist: create `medici-finance/assay-site` (grant `human:<name>` admin
   at creation), connect Cloudflare Pages, point `assay.guide` DNS. Update the stream-README row.

## Verify (executable — presence + safety gates; design quality owned by review + human:<name>)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-site/index.html && test -f ../assay-site/_headers && test -f ../assay-site/predeploy-check.sh` | exit 0 (migrated + scaffolded) |
| 2 | `ls ../assay-site/*.html \| wc -l` | ≥5 (landing + 4 migrated explainers) |
| 3 | `grep -rhoiE "https?://[^\"' )]+" ../assay-site/*.html ../assay-site/*.css \| grep -viE -e "assay.guide" -e "github.com" -e w3.org \| wc -l` | 0 (no CDN — Google Fonts self-hosted) |
| 4 | `ls ../assay-site/assets/fonts/*.woff2 \| wc -l` | ≥1 (self-hosted fonts) |
| 5 | `grep -riE -e "desk console" -e "\benterprise\b" -e "\bsaas\b" -e "pricing" ../assay-site/*.html \| wc -l` | 0 (SaaS/Enterprise stripped — human:<name> 2026-07-17) |
| 6 | `cd ../assay-site && ./predeploy-check.sh; echo $?` (planned) | 0 (leak guard passes) |
| 7 | `grep -riE "<script" ../assay-site/*.html \| wc -l` | 0 (no JavaScript) |
| 8 | `test -d ../assay-site/.git` | exit 0 (local repo initialized; GitHub creation is human:<name>'s) |
| 9 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verifier run — VERIFY: FAIL (stays `implemented`) — glm-5.2-verifier, 2026-07-23

Cross-repo verify: fresh clone of `medici-finance/assay-site` at HEAD `3f23668` ("Merge pull request #5 … methodology-graphics") to `/private/tmp/verify-al06-site` (ground truth — the local sibling checkout is stale per the SSH-insteadOf hazard); rows 1–8 run against that clone, row 9 in this worktree (oit `c938f4ad`). Shared checkout not touched.

| # | Command | Exit | Key output | Result |
|---|---|---|---|---|
| 1 | `test -f index.html && _headers && predeploy-check.sh` (repo root) | 1 | files are in `public/`, not root — **PASS at `public/`** (all three present) | FAIL (literal) / PASS (`public/`) |
| 2 | `ls *.html \| wc -l` (repo root) | — | 0 at root; **6 in `public/`** (index + briefs/lifecycle/methodology/registers/statusgen) | FAIL (literal) / PASS (`public/`, 6≥5) |
| 3 | external-URL grep (`public/*.html` + `site.css`, minus assay.guide/github/w3) | 0 | 0 external URLs — no CDN | PASS |
| 4 | `ls public/assets/fonts/*.woff2 \| wc -l` | 0 | 3 (eb-garamond, inter, jetbrains-mono latin) | PASS |
| 5 | SaaS/enterprise/pricing/desk-console grep | 0 | 0 matches | PASS |
| 6 | `../assay-site/predeploy-check.sh` | 1 | **2 leaks**: `site.css` comment refs an internal brand-guide doc (§3,§6); `methodology.html` unresolved `TODO(human:<name>)` whitepaper placeholder | **FAIL** |
| 7 | `<script` grep in `public/*.html` | — | **12** (inline pre-paint init + `assets/theme.js` per page) | **FAIL** |
| 8 | `test -d .git` | 0 | repo initialized | PASS |
| 9 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (313 advisory NOTICEs, none blocking, none for this brief) | PASS |

**VERIFY: FAIL — stays `implemented`.** Two safety gates red at HEAD `3f23668`: row 6 (the leak guard catches a CSS-comment internal-doc ref + an unresolved TODO placeholder) and row 7 (the theme-toggle PR added inline JS + `assets/theme.js` on all 6 pages, departing from the no-JS site-repo invariant). **Both regressions were introduced by later PRs #4 (methodology-graphics) and #5 (theme-toggle), NOT the original brief-06 migration** — the core migration is sound (rows 3,4,5,8,9 PASS; rows 1,2 literal-path mismatch is benign — the site now serves from `public/` per `wrangler.jsonc`).

Filed as **medici-finance/assay-site#6** — strip the two leak-guard hits (remove the internal-doc CSS comment; resolve/remove the TODO) and decide the no-JS invariant (restore it, or amend brief-06 to permit the theme-toggle JS as a recorded exception). `gate: human` + `irreversible: yes` + `customer: yes` → Evidence recorded, status stays `implemented`; the flip is human:<name>'s once the invariants are restored or formally amended.

## Review
Gate: **human** (irreversible + customer-facing). Reviewer checks the migration is faithful, fonts
are self-hosted, SaaS copy is gone, the leak guard passes; the `done` Reviewed cell is `human:ian`.
