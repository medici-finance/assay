---
stream: assay-product
serves: assay
status: active
priority: P1
track: ecosystem
external: ../assay-toolkit/STATUS.md
issues: []
---

# Assay Product Stream — build Assay up as a product (the Plumb playbook)

human:<name>'s direction (2026-07-10): *"we need to build up the assay product. look at what we did
for reconciler, and we will be doing similar things (website, explanations of what it does,
presentations etc)."* The reconciler precedent is **Plumb** (`../reconciler`): a sibling
product repo whose **Docs & Strategy layer** (product brief, market analysis, architecture,
roadmap, naming, README index, STATUS.md — its D1–D7) was completed before code extraction,
plus decks/whitepapers via the `../decks` pipeline and a landing-page roadmap item
(Plumb 1.10). This stream mirrors that layer for **Assay** (`../assay-toolkit`, named
2026-07-09 per methodology/13, home domain **assay.guide**, Apache-2.0).

**What already exists (do not re-create):** the extracted toolkit + adoption docs
(methodology/07); the name decision (methodology/13 — formal trademark clearance explicitly
NOT done, brief 04 here); 2 of 3 articles drafted with decks (methodology/09–11 own them and
their publication gate — this stream consumes, never duplicates); the honest F-08 framing
("derived from agent-authored artifacts with consistency linting, not measured ground truth")
which is binding on ALL product copy.

**Boundaries (sibling work this stream must not absorb):**
- `assay-adoption` (future stream; scoping input I-19 / `assay-growth-2026-07-09.md`) —
  multi-person/SME adoption *engineering*.
- I-24 / PR #206 — machinery self-containment (assay-toolkit as versioned source of truth).
- `assay-dogfood` (PR #227, I-27) — dogfooding via the marketplace.
- methodology/09–11 — the three articles and their publication gate (R-01 + human:<name>'s go).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Product brief — what Assay is, for whom, honestly](./brief-01-product-brief.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 02 | [Market analysis — the AI-native PM / agent-orchestration landscape](./brief-02-market-analysis.md) | 0 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 03 | [Product repo hygiene — STATUS.md, roadmap, README doc index in assay-toolkit](./brief-03-repo-status-roadmap.md) | 1 | M | done | 2026-07-13 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 04 | [Naming clearance + domain wiring — domain wiring (Plumb+Assay); formal TM search deferred 2026-07-14](./brief-04-naming-clearance-domain.md) | 0 | S | implemented | — | — |
| 05 | [Website — assay.guide landing + what-it-does explainers](./brief-05-website.md) | 1 | M | done | 2026-07-27 k3-verifier | 2026-07-27 human:ian |
| 06 | [Pitch deck — Assay presentation in ../decks/assay/pitch](./brief-06-pitch-deck.md) | 1 | M | done | 2026-07-20 glm-5.2-verifier | 2026-07-20 human:ian |
| 07 | [Artifact-freshness cadence — version history on every artifact + deterministic staleness check](./brief-07-artifact-freshness-cadence.md) | 1 | M | done | 2026-07-16 glm-5.2-verifier | 2026-07-20 human:ian |
| 08 | [Periodic critical-thinking review — stale/accreted/would-not-build-today (anti-Rube-Goldberg)](./brief-08-critical-thinking-review.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 09 | [Market-intelligence skill — product-agnostic field scan as assay:market-intelligence + first run](./brief-09-market-intelligence-skill.md) | 2 | M | done | 2026-07-27 k3-verifier | 2026-07-25 assay-reviewer-app[bot] |

## Critical path

```
01 (product brief) ──► 05 (website) ──► public launch (human)
                  └──► 06 (deck)
02 (market) ──────────► 06
04 (naming/domain, human) ──► 05
```

**Smallest unblocking move: brief 01.** **Verified as the REAL head, not assumed:** every
input 01 needs exists on main today (I-02, I-19 + `assay-growth-2026-07-09.md`, the toolkit
README's positioning paragraphs, the red-team A5 scope-honesty finding, both drafted
articles) — nothing upstream blocks it. The tempting-but-wrong first step is the website:
built before the product brief it would restate the toolkit README instead of the
positioning, and it hard-depends on 04's domain/hosting decisions (human gate) anyway.

## Dependency waves

```
Wave 0: [01, 02, 04]
Wave 1: [03 ← 01]   [05 ← 01, 04]   [06 ← 01, 02]   [07]   [08]
Wave 2: [09 ← assay-dogfood/01 (cross-stream typed dep)]
```

Briefs 07–09 (assay-toolkit#12, "assay is evolving") are the **evolution/currency layer** —
recurring product-ops off the launch critical path: 07 keeps the artifacts current with
version history, 08 is the recurring anti-Rube-Goldberg review of the methodology surface,
09 is the reusable market-intelligence skill (Assay is its first product; Medici/Plumb/
Midnight are next). 07 and 08 have no in-repo blockers (their surfaces exist on main today);
09 waits only on the assay-dogfood/01 plugin scaffold it ships in.

## Shared conventions

- **Artifact freshness:** `go run ./tools/freshness` in assay-toolkit — deterministic check;
  regeneration is a scheduled analysis-desk pass, weekly until methodology/38 re-clocks it.
- **Cross-repo:** briefs 01–03 land files in `../assay-toolkit` (its own git repo — commit
  there, note the SHA in Evidence); brief 06 lands in `../decks` (NEVER decks in this repo);
  this repo carries only the stream docs. Manifest/k8s: none — nothing here deploys.
- **Honest framing is binding copy-review criterion:** no product artifact may claim
  measured ground truth, "agents can't lie", or unsourced leverage numbers (F-08/F-12) —
  the differentiator is drift/evidence/tampering made *machine-visible*.
- **Brand:** documents/site use WHITE backgrounds with the blue accent
  (`../oit/docs/brand-guide.md`; white-backgrounds-for-documents rule). Decks follow the
  whitepaper-program pipeline (`../decks/tools/render-whitepaper-pdf.cjs`).
- **Publication is human-gated everywhere:** making the site public, submitting/presenting
  the deck, and any trademark/domain spend are human:<name>'s acts; briefs prepare and STOP.
- **Critical review:** monthly strong-tier run (brief 08), next due 2026-08-17; clock owned
  by methodology/38 once landed. First run: `docs/streams/assay-product/critical-review-2026-07.md`.
  Outputs route through the findings/intake registers — the review never edits skills, statusgen,
  or CLAUDE.md directly.
