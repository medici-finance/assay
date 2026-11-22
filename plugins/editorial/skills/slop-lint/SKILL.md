---
name: slop-lint
description: Review slide copy, narration and outward-facing prose for empty phrasing, repetitive structure and claims that outrun the evidence.
---

# Slop lint

## Pass 1 — lexicon (grep-able; fix or justify each hit)

Literature-sourced (Kobak 2025, Liang 2024, Neri 2024 excess-frequency studies — via
llmstrip), merged with the house's own banned list:

- **High severity (rewrite on sight):** delve, tapestry, testament, realm ("In the realm
  of"), landscape (metaphorical), leverage/leveraging (→ use/using), showcase/showcasing,
  underscore(s) (verb), meticulous(ly), pivotal, "moreover", "furthermore" as paragraph
  glue.
- **Medium (usually cut or replace):** notably, streamline(d), robust (non-engineering
  sense), seamless(ly), holistic, comprehensive (as self-praise), "a wide range of",
  "plays a crucial/vital role", "it's important to note", "in today's fast-paced world".
- **Phrase constructions (slopbuster tiers):** copula avoidance — "serves as", "stands
  as", "boasts" where "is/has" belongs; negative parallelism — "not just X, but Y";
  forced rule-of-three lists; sycophantic openers ("Great question…"); title-case
  listicle headers; adjectives replacing facts.

Mask code blocks and quoted material before grepping — quotes keep their author's words.

## Pass 2 — the four metrics, as manual heuristics

(From humanizer-skill's transparent composite; apply by eye, no tooling required.)

1. **Burstiness:** sentence lengths should vary. A paragraph of same-length sentences
   reads machine-made — mix a two-beat verdict into the long ones.
2. **Lexical diversity:** the same abstract noun three times in a paragraph means the
   paragraph has one idea wearing three hats — cut two.
3. **Trigram repetition:** repeated 3-word scaffolds ("this means that", "in order to")
   across paragraphs.
4. **Slop density:** pass-1 hits per 100 words. More than ~2/100 in a final draft means
   the draft needs a rewrite pass, not spot fixes.

## Pass 3 — structural slop (editorial judgment; a regex cannot catch these)

Moral-of-the-story closers on every section; template symmetry (every paragraph the same
shape); invented specifics (a number or example that traces to nothing — the house rule
is every number traceable); hedge-stacking ("could potentially perhaps").

## Check claims before handoff

Tie each number to its source, interval and denominator. Distinguish measured outcomes from targets. Replace superlatives with the comparison the reader can actually check. Keep quoted language exact and preserve uncertainty through edits. Read the revised slide or script aloud and return a short diff with reasons for material changes.
