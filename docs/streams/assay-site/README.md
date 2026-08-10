---
stream: assay-site
status: active
priority: P2
track: product
tiering: implement=cheap verify=strong
---

# Assay Site Stream

Ships the **assay.guide landing page** — Assay's first publicly-visible surface (roadmap
2.1 / D5). Landing-page scope, **not** a full docs-site build:
one hand-authored, self-contained, brand-compliant page that sells the product and points at
the repo, the quickstart, and the articles. The headline it carries is the **open-core
three-tier story** — FREE (the toolkit + Desk Solo) / PREMIUM (the hosted Desk Console) /
ON-PREM (the Console licensed into the customer's cluster) — which is newer than the
roadmap's website line (F-02-console-scope; tiers ruled by human:<name> 2026-07-15/16) and is now
the lede, ahead of the "what-it-does" explainers.

The copy inherits the product's honest positioning verbatim: the board is **derived from
agent-authored artifacts with consistency linting, not measured from ground truth**
(product-brief.md *Honest limits*; market-analysis.md
§0; the F-08 no-overclaim rule). No "tamper-evident", "cannot lie", or "measured" framing —
brief 06 gates that mechanically.

Deliverables land in **this repo** (`assay-toolkit`, Apache-2.0). Content specs live under
`docs/`; the page build lives under `web/site/` alongside the existing marketing assets
(web/teaser/ — the homepage-hero source, and the brand reference
this stream reuses). **Publication is not in scope**: serving the page at assay.guide is
brief 07's human:<name>-gated act.

## Supersedes

This stream **supersedes the legacy oit `assay-product/05` row** ("Website — assay.guide
landing page", `todo`, in `../oit`). The site work moves here, next to the
tier design docs it must sell. **Recommendation: tombstone the oit `assay-product/05` row**
(pointer here) — but that edit is the oit desk's call and is **not** made from this stream
(no cross-repo edit here; `depends:` stays in-repo).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Messaging spine + information architecture — hero honest claim, section map, voice guardrails](./brief-01-messaging-spine-ia.md) | 0 | M | implemented | — | — |
| 02 | [Three-tier open-core section — free / premium / on-prem, each pitch + who-it's-for + link](./brief-02-three-tier-section.md) | 1 | S | todo | — | — |
| 03 | [What-it-does explainers — briefs, registers, lifecycle, statusgen](./brief-03-what-it-does-explainers.md) | 1 | S | todo | — | — |
| 04 | [Landing page build — static self-contained brand-compliant HTML](./brief-04-landing-page-build.md) | 2 | M | todo | — | — |
| 05 | [Quickstart / GitHub / articles wiring — outbound CTAs, nav, footer, links resolve](./brief-05-quickstart-github-articles-wiring.md) | 3 | S | todo | — | — |
| 06 | [Honest-claims review — F-08 no-overclaim gate over the rendered copy](./brief-06-honest-claims-review.md) | 4 | S | todo | — | — |
| 07 | [Domain wiring — assay.guide host choice + records (execution is human:<name>'s)](./brief-07-domain-wiring.md) | 5 | S | todo | — | — |

## Critical path

```
01 messaging spine + IA          02 three-tier section ─┐
(REAL HEAD — the tier story       (headline; ← 01)      │
 is the newest/least-settled  ───►                      ├─► 04 page build ─► 05 wiring ─► 06 claims review ─► 07 domain wiring
 input; settle it FIRST)          03 what-it-does ──────┘        (← 01,02,03)   (← 04)        (← 04,05)          (PUBLISH GATE
                                   explainers (← 01)                                                              — gate:human)
```

**The real head is 01 — the messaging spine — and specifically the open-core three-tier
messaging inside it.** The tier story (free → premium-SaaS → hosted-on-prem) is the *newest*
input in the whole product (F-02, and the on-prem posture ruled in only 2026-07-16), so it is
the least-settled thing the page depends on. Nail it in prose (01 → 02) before any HTML exists.

- **Tempting-but-wrong first step**: opening `web/site/index.html` and building the page (04)
  first, because the teaser (`web/teaser/index.html`) is already a hero and "it's just a
  layout." Building before the three-tier spine is settled bakes the least-settled copy into
  markup and forces a re-cut of the whole page when the tier framing moves. The page is the
  *rendering* of 01+02+03; it is not the head.
- **Roadmap hard-dependency, verified**: roadmap 2.1/D5 says the site hard-depends on (a)
  **naming clearance D4 — DONE** (docs/naming-clearance.md; the domain is registered, the
  mark is `clear-with-constraints` pending human:<name>/counsel), and (b) **domain wiring** —
  `assay.guide` is **registered but NOT wired** (Cloudflare, clean slate, no A/CNAME;
  naming-clearance §9.1). Naming clearance is therefore **not** a content blocker; **domain
  wiring (07) is the publish gate, not a content gate** — the whole content pipeline (01–06)
  runs without it, and only the final publish waits on human:<name>.

## Dependency waves

```
Wave 0: [01]
Wave 1: [02 ← 01] · [03 ← 01]
Wave 2: [04 ← 01,02,03]
Wave 3: [05 ← 04]
Wave 4: [06 ← 04,05]
Wave 5: [07 ← 06]        (gate:human — the only human gate in the stream)
```

Longest chain: **01 → 02 → 04 → 05 → 06 → 07**. It is a content pipeline, so the graph is
mostly linear; the one parallel pair is **02 (tier section)** and **03 (what-it-does
explainers)**, both authorable at once once 01 fixes the IA.

## Gates

Every content brief (01–06) is **`gate: model`** — presence gates over prose/HTML plus the
standing human review gate for quality (the honesty rule, `brief-rules.md` §8). Only **07
(domain wiring) is `gate: human`**: wiring a host to `assay.guide` is the first public
exposure of the product and the mark (customer-facing; also gated on human:<name>'s mark go/no-go per
`naming-clearance.md` §8.1). The model authors the host choice + exact records + any in-repo
artifacts (a `CNAME` file, `_redirects`) and **stops**; creating DNS and serving the page are
human:<name>'s acts (naming-clearance §9.7 BLOCKED-ON-IAN).

## Definition of Done (stream conventions)

- **Presence, not quality.** For every prose/HTML brief, the Verify table gates that required
  elements *exist* (a file, a section, a token, a resolving link). Quality — does the copy
  read well, is the claim honest — is the human review gate's call, never a grep's.
- **Honest-claim bar is binding on all copy** (F-08): no "tamper-evident" / "cannot lie" /
  "measured ground truth" / "makes agents trustworthy" framing; the hero states the derived-
  not-measured claim. Brief 06 is the mechanical backstop; every author is bound by it too.
- **Discoverability = the README docs/ index + the web asset README.** A content doc that
  isn't in README.md § docs/ index, or a page whose directory has no
  `README.md`, is not "shipped" (matches the desk-solo convention). **llms.txt/docs-site
  regen does NOT apply here** — `assay-toolkit` has no `docs-site/` or `llms.txt` (checked
  2026-07-16; the empty `docs-site/` is a placeholder). That regen rule is the parent oit
  repo's; this repo's discoverability surface is the README index and the `web/*/README.md`
  files.
- **Brand reference**: no docs/brand-guide.md exists in this repo; the authoritative brand
  reference for this stream is **web/teaser/README.md** +
  the CSS system in web/teaser/index.html (Medici
  branded-house, Electric Blue `#3366FF` lead, Gold `#D4A843` the one Assay differentiator,
  theme-aware). The page reuses that system rather than inventing one.

## Feathering — external context this stream leans on

Not re-authored here; `depends:` stays in-repo. Statuses freshness-checked on this repo's
`origin/main` (2026-07-16) and cited from the design docs.

| External / cross-repo item | Status | Role here |
|---|---|---|
| oit `assay-product/05` (website row) | todo (oit main) | **Superseded by this stream** — recommend tombstone; edit is the oit desk's, not made here |
| oit F-08 (honest-claim rule) | standing | The no-overclaim bar all copy inherits (via `product-brief.md`, `market-analysis.md` §0) |
| docs/naming-clearance.md §9 | prepared | Domain state + the exact Cloudflare wiring plan brief 07 executes-on-paper |
| desk-solo / desk-console streams | active | The products the page sells; the page links their public setup pages when they exist, never claims un-built features |
