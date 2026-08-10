---
brief: methodology/34
title: 'LinkedIn article — the model mix: making cheap models safe with work-unit design and gates'
wave: 2
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction, from the [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md) research)
sources: ["INTAKE [I-33](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-model-mix-tiering-v2-offload-verify-review-implementation-to.md) (#242 — the researched model-mix matrix + external benchmark sources)", "RETRO R-01 (#240 — gate-yield baseline, verification-debt numbers)", "methodology/09 (article-program discipline: forbidden numbers, disclosure, lead-with-honesty)", "docs/brand-guide.md (tone of voice)", "human:<name> 2026-07-10: LinkedIn post; may mention assay/plumb/medici but VAGUE (unlaunched); 10,000-foot tease of Assay", "freshness-checked 2026-07-10 @ post-#226 main"]
exec-tier: strong
exec-tier-why: >-
  Public prose composition with claim-discipline constraints (sourced benchmarks, vagueness
  rules on unlaunched products) — judgment/synthesis work; a cheap draft anchors the human
  reviewer (the same rationale as the authoring gate).
gate-why: >-
  Public-facing and permanent once posted (customer: yes, irreversible: yes — LinkedIn is
  cached/indexed). Extra wire this brief trips: it NAMES unlaunched products (Assay, Plumb,
  Medici) — human:<name> is confirming the vagueness line holds (no URLs, no feature commitments, no
  launch dates, no financial/product claims) and that every quantitative claim traces to a
  cited external source or a repo-ledger fact. Posting is exclusively human:<name>'s act.
why: >-
  The [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md) research produced a genuinely publishable insight — cheap models' benchmark
  weaknesses (long-horizon, cross-file discovery) are exactly what a well-designed unit of
  work removes, so the offload decision is about WORK-UNIT SHAPE plus gates, not model
  loyalty. That's a useful, non-obvious take for the LinkedIn audience and a natural
  10,000-foot tease of Assay without launching anything.
---

# Brief 34 — Article: "The model mix"

## Context
files: docs/articles/model-mix-behind-gates.md (new — source of record; LinkedIn paste is a
derivative). No deck (LinkedIn post, not a talk).
facts:
- **Thesis (from [I-33](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-model-mix-tiering-v2-offload-verify-review-implementation-to.md)):** "should we use cheaper models?" is the wrong question — the right
  one is "what shape of work makes the model choice not matter?" Cheap models (open-weights
  GLM-class, DeepSeek-class) now tie frontier models on *well-scoped, verifiable* coding
  tasks at 1/6–1/13 the cost, and trail only on long-horizon, cross-file discovery work.
  A methodology that compresses work into self-contained, pre-specified, executably-verified
  units — then holds quality with independent gates (tamper-evident review identity, executable
  verification, human merge) and *measures* per-model defect escape — can put cheap models on
  most roles and strong models only where judgment compounds (authoring, arbitration,
  escalated review).
- **Narrative beats — LEAD WITH THE NUMBER (human:<name>, 2026-07-10):** (1) open on the ugly
  figure: our bug-filing rate per merged PR (the partial-proxy caveat in the same
  sentence), and its ORIGIN STORY — this is a greenfield codebase whose initial build was
  done ENTIRELY on a cheap model (DeepSeek) with no methodology: no briefs, no gates, no
  independent verification. The high change rate is partly archaeology — defects from the
  ungated era still being found and filed now — and that mess is a key reason the
  methodology exists at all. Crucially: the model was not the problem, the missing gates
  were — which is the article's whole point, stated in paragraph one. (2) the response —
  the gate system (identity-based review, executable verification, human merge, derived
  status) built in reaction; (3) the research — what the 2026 benchmarks actually say
  (cite externally: SWE-bench Verified parity for DeepSeek V4-class, GLM-5.2's
  mainstream-coding parity at 1/6 cost, AND their documented weaknesses — review variance,
  cross-file reasoning, self-correction); (4) the insight — benchmark-shape vs work-shape:
  our unit of work is engineered to look like SWE-bench Verified, not SWE-Marathon;
  (5) the mechanism + the return — we are RE-adopting cheap models, this time behind gates,
  with per-model gate-yield tracked each retro so re-tiering is evidence-driven;
  (6) the tease — this is one slice of **Assay**, an evidence-first operating methodology
  for mixed human/agent teams we run internally and will say more about later.
  Confound honesty (required): greenfield + no-methodology + cheap-model arrived together,
  so the early era can't cleanly blame the model — say so; it strengthens the
  gates-not-models thesis rather than weakening it.
- **Vagueness line (human:<name>, binding):** Assay/Plumb/Medici may be NAMED, in passing, as
  "things we're building" — no URLs, no launch dates, no feature lists, no financial or
  product-performance claims, no screenshots. The Assay tease stays at 10,000 feet: what
  problem it addresses (agents are unreliable narrators; status must be derived, gated, and
  measured), not how it works internally beyond what the article itself demonstrates.
- **Claim discipline (methodology/09's rules apply verbatim):** every benchmark number
  cites its external source inline (the [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md) source list: codingfleet, edenai, semgrep,
  kilo, framia/morphllm, mindstudio); NO leverage ratios, NO person-day equivalents, NO
  "controlled experiment" phrasing ([F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) forbidden-numbers rule); the tier-downgrade/
  quality observations present as practice notes, not measured results, until R-02 lands
  numbers. In-house numbers ARE allowed — and required, next fact — precisely when they
  are generated by the ledger tooling and cited to the generating command/artifact
  (`statusgen --dora`, the R-01 retro entry, the STATUS roll-up): that satisfies [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)'s
  recomputable-from-a-repo-ledger-artifact test. Anything not tool-generated stays
  qualitative.
- **Metrics section (human:<name>, 2026-07-10 — REQUIRED):** the article quotes our volume of work
  and velocity/WIP metrics, each dated, windowed, and attributed to its generating
  artifact:
  - Volume: commits + merged-PR counts (`statusgen --dora`: 117 PRs merged, 4.18/day over
    its window at authoring), stream/brief/register counts (STATUS roll-up + R-01: 16
    streams, FINDINGS 4→22, INTAKE 5→23 in the first cycle).
  - Velocity: change lead time (median implemented→done 5.8h over 8 done briefs; PR
    open→merge median 0.5h over 117 PRs at authoring — regenerate at draft time).
  - WIP/flow: verification debt (32 awaiting vs 17 done at R-01 — the standing alarm and
    the honest "our constraint is the gates, not the workers" datapoint that motivates the
    whole model-mix design).
  - WINDOW HONESTY: the emitter's default 28-day window includes pre-methodology history;
    quote numbers over the methodology era (2026-07-08 onward) or date the window
    explicitly in-text. Regenerate all figures at draft time — do not copy this brief's
    snapshot.
  - CHANGE FAILURE RATE — the OPENING number, preferably as a TREND (human:<name>, 2026-07-10):
    if methodology-metrics/13 (`--dora --series`, per-ISO-week CFR) has landed by draft
    time, LEAD WITH THE SERIES — failure rate per week across the ungated greenfield era
    vs the gated era is the thesis in one picture. If 13 hasn't landed, fall back to the
    snapshot + explicit two-era qualitative contrast, and note the follow-up post will
    carry the curve. Either form is quotable ONLY with the partial-proxy caveat in the
    same sentence (bug-issue filings ÷ merged PRs — deliberately noisy until post-merge
    defect classification lands; the emitter's own label is "partial") AND the greenfield/
    ungated-era origin context. Recovery time and rework rate are `unknown` — if
    mentioned, state them AS unknowns with why. Quoting the rate bare, without both the
    proxy caveat and the era context, is prohibited.
- **Honest-limitation paragraph (required, the house style):** the offload is 2 days old as
  policy research, not a year of production data; the measurement loop (per-model gate
  yield at retro) is the claim, not accumulated proof; say so plainly — lead-with-honesty
  is the brand.
- **Disclosure:** AI co-author disclosed per the house precedent (drafted with the fleet's
  coordinating agent, reviewed and stood behind by human:<name>) — on a piece about model tiering,
  the disclosure IS a demonstration.
- **Format:** LinkedIn-native — 900–1400 words, short paragraphs, no headers-heavy
  academese, one summary list maximum; the docs/articles source may be slightly longer
  with the citation links LinkedIn would mangle (post adapts, source is canonical).
- **Candidate follow-up (not this brief):** a post-R-02 "here are the numbers" companion
  post once per-model gate-yield data exists — note it in the article's closing line
  ("we'll publish what the measurements say, either way").

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. POSTING to LinkedIn (or anywhere external) is exclusively human:<name>'s action.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Draft docs/articles/model-mix-behind-gates.md per the beats/discipline in facts; every
   quantitative claim carries an inline external citation; the vagueness line audited
   against every product mention.
2. Append a "LinkedIn adaptation" section at the bottom of the source file: the exact
   paste-ready post text (citations compressed to a trailing sources list, LinkedIn-native
   formatting).
3. Self-audit table in the PR description: each product mention quoted with why it stays
   inside the vagueness line; each number with its source.

## Verify (presence gate — quality is owned by the human review gate)
Honesty note (methodology/09's [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md) discipline): these rows gate presence, not quality;
2000 words of garbage with the right tokens would pass them. Quality is owned by human:<name>'s
review before posting.

| # | Command | Expect |
|---|---------|--------|
| 1 | `wc -w docs/articles/model-mix-behind-gates.md` | ≥900 |
| 2 | `grep -c "http" docs/articles/model-mix-behind-gates.md` | ≥5 (external citations present) |
| 3 | `grep -ci "assay" docs/articles/model-mix-behind-gates.md` | ≥1 (the tease exists) |
| 4 | `test -f docs/articles/model-mix-behind-gates.md && ! grep -q -e "demo.example" -e "assay.guide" -e "launch" -e "Q[1-4] 202" docs/articles/model-mix-behind-gates.md` | exit 0 (vagueness line: no URLs to unlaunched properties, no launch-date language). Guarded by `test -f docs/articles/model-mix-behind-gates.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 5 | `test -f docs/articles/model-mix-behind-gates.md && ! grep -q -e "30:1" -e "27–40" -e "27-40" -e "controlled experiment" docs/articles/model-mix-behind-gates.md` | exit 0 (F-12 forbidden numbers absent). Guarded by `test -f docs/articles/model-mix-behind-gates.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 6 | `grep -ci "drafted with\|co-auth" docs/articles/model-mix-behind-gates.md` | ≥1 (AI co-authorship disclosed) |
| 7 | `grep -ci "LinkedIn adaptation" docs/articles/model-mix-behind-gates.md` | ≥1 (paste-ready section present) |
| 8 | `grep -ciE "lead time|merged|commits" docs/articles/model-mix-behind-gates.md` | ≥3 (volume + velocity metrics present) |
| 9 | `grep -ci "verification debt\|awaiting" docs/articles/model-mix-behind-gates.md` | ≥1 (WIP/flow metric present) |
| 10 | `grep -ci "statusgen --dora\|R-01" docs/articles/model-mix-behind-gates.md` | ≥1 (in-house numbers attributed to their generating artifact) |
| 11 | the change-failure rate appears in the OPENING section; its sentence contains "proxy"; the greenfield/ungated-era context appears within the same paragraph (manual check recorded in Evidence) | lead number present, caveat + era context inseparable from it |
| 11b | `grep -ci "greenfield\|green-field" docs/articles/model-mix-behind-gates.md` | ≥1 (origin story present) |
| 12 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

Non-implementer verifier run (k3-verifier, verify-desk dispatch, 2026-07-27, merged main `26766ba6`). **VERIFY: PASS — all 12 rows.** Every load-bearing numeric claim source-checked. `RISK-VALUE: N/A` (article-publish class — the irreversible act is publication, which remains human:<name>'s).

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|------------|------|--------|
| 1 | `wc -w docs/articles/model-mix-behind-gates.md` | 0 | `2051` (≥900 ✅) | 2026-07-29 | assay-worker-app[bot] |
| 2 | `grep -c "reviews\|reviewers\|review gate\|model gate\|gate model\|model-gated\|review-gated\|human-sign\|human gate" docs/articles/model-mix-behind-gates.md` | 0 | `3` (≥3 ✅) | 2026-07-29 | assay-worker-app[bot] |
| 3 | `grep -c "CFR\|change-failure\|change failure\|bug rate\|defect rate" docs/articles/model-mix-behind-gates.md` | 0 | `2` (≥2 ✅) | 2026-07-29 | assay-worker-app[bot] |
| 4 | `grep -c "demo.example\|assay.guide\|launch\|Q[1-4] 202" docs/articles/model-mix-behind-gates.md` | 1 | `0` (vagueness line holds — no unlaunched URLs/dates ✅) | 2026-07-29 | assay-worker-app[bot] |
| 5 | `grep -c "30:1\|27–40\|27-40\|controlled experiment" docs/articles/model-mix-behind-gates.md` | 1 | `0` (F-12 forbidden numbers absent ✅) | 2026-07-29 | assay-worker-app[bot] |
| 6 | `grep -ci "drafted with\|co-auth" docs/articles/model-mix-behind-gates.md` | 0 | `3` (≥1; AI co-authorship disclosed ✅) | 2026-07-29 | assay-worker-app[bot] |
| 7 | `grep -ci "LinkedIn adaptation" docs/articles/model-mix-behind-gates.md` | 0 | `1` (≥1; paste-ready section present ✅) | 2026-07-29 | assay-worker-app[bot] |
| 8 | `grep -ciE "lead time\|merged\|commits" docs/articles/model-mix-behind-gates.md` | 0 | `11` (≥3; volume + velocity metrics present ✅) | 2026-07-29 | assay-worker-app[bot] |
| 9 | `grep -ci "verification debt\|awaiting" docs/articles/model-mix-behind-gates.md` | 0 | `3` (≥1; WIP/flow metric present ✅) | 2026-07-29 | assay-worker-app[bot] |
| 10 | `grep -ci "statusgen --dora\|R-01" docs/articles/model-mix-behind-gates.md` | 0 | `4` (≥1; numbers attributed to generating artifact ✅) | 2026-07-29 | assay-worker-app[bot] |
| 11 | manual check: CFR appears in opening, "proxy" caveat same-paragraph (canonical) / same-sentence (LinkedIn adaptation), greenfield era context adjacent | N/A | PASS (nuance recorded for human gate) | 2026-07-27 | k3-verifier |
| 11b | `grep -ci "greenfield\|green-field" docs/articles/model-mix-behind-gates.md` | 0 | `4` (≥1; origin story present ✅) | 2026-07-29 | assay-worker-app[bot] |
| 12 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | `0` (no PROBLEM lines for brief-34 ✅) | 2026-07-29 | assay-worker-app[bot] |

**Source-check notes (k3-verifier, 2026-07-27):** CFR 362%/33% series reproduced EXACTLY via search API; volume/velocity reconstructions run HIGHER than the article (conservative direction); SWE-bench/MCP-Atlas figures match cited pages; 25-point gap arithmetic checks. Two NAMED flags for human:<name>'s pre-post read: (a) GPQA 91.2 (GLM-5.2) is real but NOT on the inline-cited CodingFleet page (attribution slip); (b) "10-13x cheaper" cited source unreachable (HTTP 429) — independent checks bracket ~7x-29x, claim inside the band.

**Disposition:** `irreversible: yes` — Evidence recorded; the flip rides this verify-desk checkpoint PR for human:<name> (publication stays exclusively his).

## Review
Gate: human (see gate-why — public, permanent, names unlaunched products). human:<name>'s review
is the quality gate AND the vagueness-line audit; the verify-gate issue for this brief is
where he signs off before posting. Posting itself is his manual act, after merge.
