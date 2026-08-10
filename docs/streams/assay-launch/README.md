---
stream: assay-launch
serves: assay
status: active
priority: P1
track: ecosystem
issues: []
---

# Assay-Launch Stream — publish the Assay methodology at assay.guide

**human:<name>'s direction (2026-07-17):** *"we need to start publishing our methodology. (1) a
website with our CTAs and 'this is how wonderful we are' stuff, linking to a suite of
methodology pages — high-level view first, then a page per mechanism in more detail;
(2) a downloadable PDF that explains it; (3) the metrics we are collecting today showing
how it runs. Fill the gaps and give me a document that shows all the moving parts across
streams — ideally automated by statusgen as a cross-cutting stream."*

**Refined (human:<name>, 2026-07-17, follow-up):** *"leave the SaaS/Enterprise versions out of the
website for now. We need an `assay-site` repo, similar to `site-repo`. The daily jobs need
to be able to publish into it (without manual intervention)."*

This is that document. It is a **cross-cutting coordination stream**: it does not re-create the
articles or the metrics — those are owned elsewhere and referenced via typed cross-stream
`depends:`. It **owns** the site itself (a new standalone `assay-site` repo), the four gap
deliverables (deep page suite, PDF, public metrics page, `statusgen --launch`), the automated
publish pipeline, and the single human go-live gate. Because every upstream dependency is a
typed `depends:` ID, `statusgen` keeps this board honest automatically — Next-up will not
surface the launch gate as eligible until the real work across all streams is `done`.

**Front door (human:<name>, 2026-07-17):** the site is **Assay's own** (`assay.guide`) — the methodology
*is* the product. It lives in a **standalone `assay-site` repo** mirroring `site-repo` (see
Shared conventions), NOT inside `assay-toolkit`.

## What already exists (reference it) vs. what this stream builds

| Your ask | Owned by | State today |
|----------|----------|-------------|
| Methodology narrative content | [`methodology/09`](../methodology/brief-09-article-status-build-artifact.md), [`/10`](../methodology/brief-10-article-convergence-thesis.md), [`/11`](../methodology/brief-11-article-convergable-specs.md) | 3 articles drafted (`docs/articles/`); publication gated on lived data + human:<name>'s go |
| Metrics "how it runs" | [`methodology-metrics/02`](../methodology-metrics/brief-02-dora-emitter.md) (`--dora`), [`/03`](../methodology-metrics/brief-03-statusgen-trend.md) (`--trend`), [`/22`](../methodology-metrics/brief-22-daily-artifact-harvest.md) (daily harvest) | emitters **done**; harvest **implemented**; no public rendering, no auto-publish |
| PDF renderer | `../decks/tools/render-whitepaper-pdf.cjs` | exists (whitepaper pipeline); no methodology PDF wired |
| Site pattern | `../site-repo` (`medici-finance/site-repo`) | working template: standalone, static, Cloudflare Pages, `predeploy-check.sh` leak guard |
| Landing + 4 explainer pages | [`assay-product/05`](../assay-product/brief-05-website.md) | **implemented** — a parallel session built the "Work held true" landing + briefs/lifecycle/registers/statusgen explainers + `assay.css` in `assay-toolkit/site/` (assay-toolkit `5c4eb49`). `assay-launch/06` **migrates** it into `../assay-site` + hardens it. See finding [`F-assay-site-repo`](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-17-assay-site-standalone-repo.md) |

## The gaps this stream owns

- **06** — **migrate** the already-built site (`assay-toolkit/site/`, `5c4eb49`) into the standalone
  **`assay-site` repo** + harden to site-repo invariants: self-host the Google-Fonts CDN it
  currently loads, strip the 2 SaaS/Enterprise references, add `_headers` + the `predeploy-check.sh`
  leak guard. The site's foundation.
- **01** — add the high-level **overview** + the two **missing** mechanism pages (desk-roles,
  model-tiering) and **deepen** the 4 migrated explainers to full detail.
- **02** — a single **downloadable methodology PDF**, wired to a site download link.
- **03** — a public **"how it runs" live-metrics page** rendering the DORA/trend snapshot.
- **04** — `statusgen --launch`: a readiness rollup so one command shows every moving part and
  what is still blocking go-live.
- **07** — the **automated publish pipeline**: the daily jobs regenerate + push into `assay-site`
  with no manual step (Cloudflare Pages auto-deploys), gated on the leak guard.
- **05** — the **launch readiness + go-live gate** (`gate: human`, irreversible): the single node
  that is green only when every part is. human:<name> performs the first launch.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 06 | [assay-site repo — migrate the built site + harden (mirror site-repo)](./brief-06-assay-site-repo.md) | 0 | M | implemented | — | — |
| 01 | [Methodology page suite — add overview + missing pages, deepen the migrated ones](./brief-01-methodology-page-suite.md) | 1 | M | todo | — | — |
| 02 | [Downloadable methodology PDF — render + wire to a site download link](./brief-02-methodology-pdf.md) | 2 | M | todo | — | — |
| 03 | [Public "how it runs" live-metrics page — DORA/trend snapshot](./brief-03-live-metrics-page.md) | 1 | M | todo | — | — |
| 04 | [`statusgen --launch` — launch-readiness rollup view](./brief-04-statusgen-launch-view.md) | 0 | M | done | 2026-07-20 opus-verifier | 2026-07-19 assay-reviewer-app[bot] |
| 07 | [Automated publish pipeline — daily jobs push into assay-site, no manual step](./brief-07-auto-publish-pipeline.md) | 2 | M | todo | — | — |
| 05 | [Launch readiness + go-live gate (human, irreversible)](./brief-05-launch-gate.md) | 3 | S | todo | — | — |

## Critical path

```
06 (migrate built site + harden, gate:human)  ← THE REAL HEAD
        │
        ├──► 01 (overview + missing pages + deepen) ──► 02 (PDF) ───┐
        ├──► 03 (metrics page) ──► 07 (auto-publish) ──────────────► 05 (go-live gate, human)
        └──► (04 --launch view runs in parallel, no deps) ─────────┘
                                          ▲
                    methodology/09,10,11 (articles) ────────────────┘
```

**The REAL head of the critical path is `assay-launch/06`.** The site content already exists
(built in `assay-toolkit/site/`, `5c4eb49`, `assay-product/05` = implemented), but it is not yet in
a standalone repo, still CDN-loads fonts, and carries SaaS copy — so `06` migrates + hardens it, and
**creating the `medici-finance/assay-site` repo is human:<name>'s act** (new `medici-finance` repos must
grant `human:<name>` admin at creation; the org classifier blocks the PUT unprompted), as is the
Cloudflare Pages + `assay.guide` DNS wiring. Everything else builds on that repo, so **the smallest
unblocking move is migrating the site into `assay-site` + a local preview.** Verified, not assumed:
no `assay-site` repo exists under `../` as of 2026-07-17 (the `assay-site-stream` worktree is a
branch of `assay-toolkit`, not a separate repo), and repo creation is human-gated.

Page/PDF/metrics **content** (01/02/03) build on the migrated repo. `statusgen --launch` (04) has no
dependency and can start now.

## Dependency waves

```
Wave 0: [06, 04]                    (06: migrate + harden, human:<name> creates the repo; 04: no deps)
Wave 1: [01, 03] ← 06
Wave 2: [02 ← 01,  07 ← 06,03]
Wave 3: [05]  ← 01, 02, 03, 04, 06, 07 (+ external methodology/09,10,11)
```

Critical path (this stream's own chain): **06 → 01 → 02 → 05**. The retargeted landing
(`assay-product/05`) also depends on 06 and feeds the go-live gate, in parallel with 01.

## Shared conventions (inherited by every brief)

- **Site home = a standalone `assay-site` repo** (`medici-finance/assay-site`), mirroring
  `site-repo`: static, **zero build step**, fully self-contained (self-hosted fonts, no CDN, no
  JS), served **byte-for-byte by Cloudflare Pages** — a push IS the deploy. NOT inside
  `assay-toolkit`. `NOTES.md`/`README.md` stay out of the deployed asset set.
- **SaaS / Enterprise is OUT of scope for the public site (human:<name>, 2026-07-17).** Public copy covers
  the **free / open-core toolkit** only — no Desk Console, no SaaS/Enterprise tier, no pricing.
  The premium story is deferred; this launch is the methodology + toolkit.
- **Auto-publish safety — the leak guard is mandatory.** Every push to `assay-site` (manual or
  from the daily job) MUST pass a `predeploy-check.sh` mirrored from site-repo: it fails if any
  internal detail (person names, brief/stream IDs, dates, `-TBD` markers, private-repo refs,
  internal `.md`/`§` references) reaches a shipped artifact via comments. An *automated* publisher
  makes this guard load-bearing, not optional.
- **Honest-framing is binding (F-08 / F-12).** All public copy uses the "derived from
  agent-authored artifacts with consistency linting, not measured ground truth" framing; no
  ground-truth claims, no unsourced leverage/ratio numbers (e.g. no "30:1"). Hard review-cut on
  every page and the PDF; DORA is "diagnostic, not a target."
- **Brand:** WHITE background, blue accent (`../oit/docs/brand-guide.md` + the
  white-backgrounds-for-documents rule) — the site is a document surface, not the app UI.
- **Nothing publishes before the gate.** Briefs 01–04 stop at `implemented` (locally previewable /
  rendered artifact only); 06 stands the repo up to a first *preview*, not a public launch; 07
  builds the pipeline but it only goes live after 05. Making a repo public, hosting, DNS, and the
  first publish are `assay-launch/05` / human:<name>'s acts. No brief here git-pushes to a public site,
  triggers a public deploy, or runs mutating kubectl on its own authority.
- **Cross-repo:** the site + pipeline land in `../assay-site` (new repo); the PDF renderer lives in
  `../decks/`; `statusgen` code + these stream rows land in this repo. Note cross-repo SHAs where a
  brief pairs a change across a repo boundary; the daily-harvest workflow (in this repo) pushing to
  `assay-site` is such a pair.
