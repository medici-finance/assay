---
brief: assay-product/06
title: Pitch deck — Assay presentation in ../decks/assay/pitch
wave: 1
depends: ["assay-product/01", "assay-product/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["assay-product/01 + /02 (content sources)", "../decks/README.md + tools/render-whitepaper-pdf.cjs (the pipeline)", "../decks/whitepapers/status-is-a-build-artifact.pdf + whitepapers/prevention-and-reconciliation.pdf (existing Assay-adjacent decks — reuse, don't duplicate)", "memory: buidl-btc-pitch-review-verdict (the traced-claims lesson: every number in a deck traces to a source)", "freshness-checked 2026-07-10 @ 78200803"]
---

# Brief 06 — Pitch deck: Assay presentation

**CROSS-REPO:** the deck lands in `../decks/assay/pitch/` (decks NEVER live in this repo).
This repo: stream-row update only.

## Context
files: ../decks/assay/pitch/ (new dir — deck source + rendered PDF per the decks pipeline)
facts:
- Audience/register: the same dual-use shape as other medici decks — present-to-partner
  and leave-behind; ~12–18 slides.
- Skeleton (adapt to 01/02, don't invent): the failure modes (agents grade their own
  homework) → what Assay is (briefs / registers / lifecycle / board) → how it's different
  (the §11 unfound-combination claim, honestly framed per [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)) → proof from our own
  operation (ONLY computed numbers with a traceable source — the deck-review lesson from
  the BUIDL/BTC deck applies: every figure traces or it's cut; [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) forbids the 30:1 class)
  → market (from 02) → roadmap (from 03 if landed, else 01's open decisions) → the family
  line (Plumb, identity held true — Assay, work held true) → ask (defined by human:<name>; leave a
  marked TODO slide if undefined at implementation time).
- Existing deck material: ../decks/whitepapers/status-is-a-build-artifact.pdf and
  ../decks/whitepapers/prevention-and-reconciliation.pdf carry article decks — lift visual language and
  any still-true diagrams; do not re-explain what a slide can link to.
- Brand: decks follow the whitepaper-program pipeline and docs/brand-guide.md; WHITE
  document style.
- Presenting/submitting the deck anywhere is human:<name>'s act — the brief produces the artifact.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (decks commits local; push is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build `../decks/assay/pitch/` per the decks repo's existing structure (match a recent deck
   dir's layout); render the PDF via the pipeline.
2. Every quantitative claim carries an inline source note (footer or presenter notes);
   claims without a source get cut or marked TODO-IAN.
3. Update the stream-README row.

## Verify (executable — presence gates; deck quality owned by review + human:<name>)
| # | Command | Expect |
|---|---------|--------|
| 1 | `ls ../decks/assay/pitch/ \| wc -l` | ≥2 (source + rendered artifact) |
| 2 | `ls ../decks/assay/pitch/*.pdf \| wc -l` | ≥1 (rendered) |
| 3 | `test -d ../decks/assay/pitch/ && ! grep -rqiE "30:1" ../decks/assay/pitch/ --include="*.md" --include="*.html"` | exit 0 (F-12: the forbidden `30:1` figure appears in no deck source). Guarded by `test -d ../decks/assay/pitch/ &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 4 | `grep -rci "held true" ../decks/assay/pitch/ --include="*.md" --include="*.html"` | ≥1 (family framing present) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

**Non-implementer verifier run — VERIFY: PASS** · 2026-07-20 · `glm-5.2-verifier` · merged main `73d01752`

**decks repo**: `../decks` · **SHA verified at**: `51bb00ccf9dc56879401b399c85f51da432a64e1` (`origin/main`; the brief records no specific decks SHA). Verified in an isolated clone `/private/tmp/verify-ap06-decks`; the shared decks checkout was **not** mutated.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `ls …/assay/pitch/ \| wc -l` | 0 | `3` (deck.md, deck.pdf, index.html — ≥2 required) |
| 2 | `ls …/assay/pitch/*.pdf \| wc -l` | 0 | `1` (deck.pdf, 852961 bytes; `file` → "PDF document, version 1.4, 8 pages") |
| 3 | `grep -rciE "30:1" …/assay/pitch/ --include="*.md" --include="*.html"` | 1 (grep no-match, expected) | `deck.md:0` + `index.html:0` → 0 (F-12 30:1-claim ban respected) |
| 4 | `grep -rci "held true" …/assay/pitch/ --include="*.md" --include="*.html"` | 0 | `deck.md:1` + `index.html:1` → 2 (≥1); match: "\| **Brand promise** \| Identity, held true \| Work, held true \|" |
| 5 | `statusgen --lint` (this repo, via built `/tmp/sxlint` binary — `--lint` is read-only per `main.go:319`) | 0 | NOTICEs only (pre-existing debt/alarms unrelated to assay-product/06); no errors, no lint-blocking findings |

Per the brief's own scope note ("Verify: executable — presence gates; deck quality owned by review + human:<name>"), the Verify table is presence-only. Review-section items (traced-claims audit per the BUIDL/BTC lesson, white-background brand conformance, slide count, TODO-slide handling) remain outstanding for the review gate. `gate: model`, `risk: {all no}` → risk-clear.
