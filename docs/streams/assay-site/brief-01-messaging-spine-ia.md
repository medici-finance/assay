---
brief: assay-site/01
title: Messaging spine + information architecture — hero honest claim, section map, voice guardrails
why: >-
  The landing page is a rendering of its messaging, and the newest, least-settled input in the
  whole product is the open-core three-tier story (free/premium/on-prem, F-02 + the on-prem
  posture ruled 2026-07-16). If the page is built before the spine is fixed in prose, the
  least-settled copy gets baked into markup and the whole page has to be re-cut when the
  framing moves. This brief settles the hero claim, the section order, and the voice
  guardrails once, so every downstream brief renders a decided message instead of inventing one.
wave: 0
depends: []
unblocks: ["assay-site/02", "assay-site/03", "assay-site/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["docs/product-roadmap.md 2.1/D5 (website line + what it renders)", "docs/product-brief.md (Executive summary + Honest limits — the derived-not-measured claim)", "docs/market-analysis.md §0 (tier scoping + honest framing, F-08)", "docs/streams/FINDINGS.md F-02 (open-core tier ruling)", "web/teaser/README.md (brand reference — no brand-guide.md exists in repo)", "docs/messaging-guide.md (2026-07-17 comprehension diagnosis — see F-04: it contradicts this brief's hero-leads-with-honest-claim directive; reconcile before authoring)"]
---

# Brief 01 — Messaging spine + information architecture

## Context
files: docs/site-messaging.md (NEW)
facts:
- ROLE: this is a CONTENT SPEC, not HTML. It decides (a) the hero, (b) the ordered section
  map, (c) the voice/claim guardrails. Briefs 02–04 render it; they do not re-decide it.
- HERO = the honest claim, verbatim in substance, above the fold — AMENDED per F-04
  (messaging-guide §1 B3/B9, §2): the hero OPENS with the scene/value (the who-checks
  problem and the computed board) and the honest claim — the board is *derived from
  agent-authored artifacts with consistency linting, not measured from ground truth*; the
  value is that drift, missing evidence, and register tampering are made **machine-visible,
  not that agents are made trustworthy** (product-brief Honest limits; market-analysis §0)
  — CLOSES the hero as its boundary line. The claim is still the moat and still page-one;
  it no longer *leads* before the reader has a mental model (honesty-before-comprehension
  reads as hedging; the same sentence after the model lands as rigor).
- SECTION ORDER (headline first): hero → the open-core THREE-TIER section (free / premium /
  on-prem — the lede, per human:<name>; detailed content is brief 02) → what-it-does explainers
  (briefs, registers, lifecycle, statusgen — brief 03) → quickstart/GitHub/articles CTAs
  (brief 05). Record this order as the IA; downstream briefs slot into it.
- THE THREE TIERS (name them so 02 has a fixed frame): FREE = the Apache-2.0 toolkit
  (statusgen + briefs/registers/lifecycle + loop skills) plus **Desk Solo** (the Supacode
  cockpit); PREMIUM = the hosted **Desk Console** (SaaS); ON-PREM = the Console licensed into
  the customer's own cluster. (desk-console-design §2.0; desk-console-saas §3.)
- VOICE GUARDRAILS (binding on all copy, F-08): BANNED framings — "tamper-evident", "cannot
  lie" / "can't lie", "measured ground truth", "makes agents trustworthy", any published
  productivity-multiplier / leverage number. REQUIRED framing — "derived", "machine-visible",
  honest limits stated not hidden. Brief 06 enforces this mechanically; state it here as the
  contract 02–05 write against.
- BRAND: no docs/brand-guide.md in repo; cite web/teaser/README.md + web/teaser/index.html as
  the brand system (Medici branded-house "Assay by Medici", Electric Blue #3366FF lead, Gold
  #D4A843 differentiator, theme-aware). This brief only NAMES the brand source; the build (04)
  applies it.
- DO NOT claim un-built features (Desk Console/Desk Solo are design-stage): the page describes
  each tier honestly and links its public setup page WHEN one exists, never before.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write docs/site-messaging.md: the messaging spine + IA for the assay.guide landing page.
2. Include, as explicit sections: **Hero** (the derived-not-measured claim as headline +
   sub-lede), **Section map** (the ordered IA above, one line each), **Three-tier frame**
   (the three tier names + one-sentence positioning each — the skeleton brief 02 fills), and
   **Voice guardrails** (the banned/required framings list, citing F-08).
3. Add the page to the README docs/ index (§ docs/ index).

## Verify (executable — no prose-only DoD items)
Prose deliverable: PRESENCE gate (required sections/tokens exist); prose quality is the human
review gate's call.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/site-messaging.md; echo $?` | `0` |
| 2 | `grep -ci "derived" docs/site-messaging.md` | ≥ `1` (the honest claim is present) |
| 3 | `grep -ic "machine-visible" docs/site-messaging.md` | ≥ `1` (the honest framing, not "trustworthy") |
| 4 | `grep -Eoi -e "free" -e "premium" -e "on-prem" docs/site-messaging.md \| wc -l \| tr -d ' '` | ≥ `3` (all three tiers named) |
| 5 | `grep -ic "guardrail" docs/site-messaging.md` | ≥ `1` (voice guardrails section present) |
| 6 | `grep -Eic -e "un-?forg" -e "measured ground truth" docs/site-messaging.md` | ≥ `1` (the guardrail section names the banned framings so 02–05 can avoid them; the no-overclaim ENFORCEMENT over rendered copy is brief 06) |
| 7 | `grep -q "site-messaging.md" README.md; echo $?` | `0` (linked in the docs/ index) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns claim-quality; the grep rows only prove the elements exist.
