---
brief: assay-product/08
title: 'Periodic critical-thinking review — what is stale, what only accreted, what we would not build today (anti-Rube-Goldberg), as a recurring strong-tier task'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable session (assay-toolkit#12 point 2)
sources: ["assay-toolkit#12 (human:<name>, verbatim): 'Run a critical thinking task to evaluate what is stale and no longer makes sense, keeping in mind stable systems have different needs than greenfield sites. Rube Goldberg machines are not what we want'", "docs/streams/methodology/red-team-2026-07-09.md (the precedent — one-off adversarial whole-surface critique; this brief makes that move recurring and aims it at accretion instead of claims)", "docs/streams/RETRO.md (R-NN weekly retro — the shallow/frequent sibling; one-process-change-per-retro budget)", "methodology/38 (cadence-compression research — authored on OPEN PR #389, unmerged at authoring; its per-loop decision rule owns this task's clock once landed)", "freshness-checked 2026-07-12 @ ab92e96e: no recurring simplification/accretion review exists — the red-team doc is a single dated artifact, RETRO.md's checklist reviews the week's events, and nothing on main asks 'would we build this today?'"]
why: >-
  The methodology grew mechanism-by-mechanism under greenfield conditions — each rule, check,
  and loop was added to stop a specific incident — and nothing ever asks whether the pile
  still makes sense as the system stabilizes. Stable systems need LESS process than the
  greenfield that accreted it, and unreviewed accretion is how a working system becomes a
  Rube Goldberg machine: every part defensible, the whole indefensible. A recurring
  zero-based read finds what to retire while retiring is still cheap.
---

# Brief 08 — Periodic critical-thinking review (anti-Rube-Goldberg)

## Context

files: `docs/streams/assay-product/critical-review-2026-07.md` (planned) (the first run's
output — dated; later runs add siblings); `docs/streams/findings/` (retire/simplify verdicts
that invalidate existing briefs become entries); `docs/streams/intake/` (simplification ideas
not yet brief-shaped); `docs/streams/assay-product/README.md` (row + convention line).

facts:
- **How this differs from the R-NN retro (both survive; neither absorbs the other):** the
  retro is weekly, incremental, consumes the week's events, and carries a one-change budget.
  THIS task is rarer and deeper: a strong-tier, zero-based read of the WHOLE methodology
  surface that ignores the week and asks what the standing system should be. Retro output is
  one process tweak; this task's output is a keep/simplify/retire verdict per mechanism.
- **The surface to read, enumerated (the run may extend, never shrink, this list):** the loop
  skills (the-desk, batch-fanout, pr-review-desk, verify-desk, author-brief — user-level and
  project layer); `../assay-toolkit/statusgen/` machinery (every flag, lint rule, NOTICE, register view);
  the brief-v1 conventions (frontmatter fields, gates, Evidence, class-sweep, freshness-check
  rules); the registers (findings/intake/retro shapes); CLAUDE.md's resident rules; the
  streams themselves (a stream whose remaining briefs no longer matter is accretion too).
- **The four questions, per mechanism:** (a) what problem did it solve when added — cite the
  incident/brief; (b) does that problem still exist at current scale and velocity; (c) would
  we build it TODAY, knowing what we know; (d) what would LESS look like — the
  stable-system form vs the greenfield form. Verdict: `keep` / `simplify` / `retire`, each
  with the evidence line. The Rube Goldberg test applies across mechanisms: any chain of
  three-plus mechanisms serving one outcome that a single simpler mechanism would serve gets
  named as a chain, not defended piecewise.
- **Output routing (findings register is the enforcement rail):** every `retire`/`simplify`
  verdict that invalidates an existing brief's assumptions → a findings entry
  (`docs/streams/findings/`, `affects:` the owning briefs); simplification work that is
  brief-shaped → authored per author-brief in the owning stream; not-yet-scoped ideas →
  intake entries. A run finding NOTHING to retire must argue why (a system this young and
  this accreted producing zero candidates is itself suspicious) — "all keep" without argument
  fails review.
- **Clock:** first run lands with this brief; thereafter monthly by default, re-clocked by
  methodology/38's per-loop decision rule when that research lands (its generation/
  ratification split applies here too: the READ can compress; ratifying retirements of
  human-attended rituals stays on human:<name>'s clock).
- **Recursion guard:** this task must not itself become accretion — each run's doc opens by
  reviewing the PREVIOUS run (did its retirements happen? did its keeps rot?), and the task
  stands for retirement by its own criteria like everything else.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- This run READS the whole surface but WRITES only its own doc + register entries — it never
  edits skills, statusgen, or CLAUDE.md directly; changes route through findings/briefs.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Execute the first review at strong tier: walk the enumerated surface, apply the four
   questions per mechanism, write `docs/streams/assay-product/critical-review-2026-07.md` (planned)
   with (a) a `## Verdicts` table — one row per mechanism: mechanism · built-for (typed
   ID/incident) · still-needed? · verdict · evidence; (b) a `## Chains` section naming any
   Rube Goldberg chains found (or arguing their absence); (c) a `## Previous run` section
   (first run: states there is none); (d) the routed outputs list.
2. File the findings/intake entries the verdicts require, per output routing above.
3. Record the recurrence: a convention line in this stream README ("critical review: monthly
   strong-tier run, next due <date>; clock owned by methodology/38 once landed") and the
   README row.

## Verify (executable — presence gates; verdict quality owned by the review gate)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/assay-product/critical-review-2026-07.md && wc -w < docs/streams/assay-product/critical-review-2026-07.md` | ≥ 2000 |
| 2 | `grep -cE -e '\*\*keep' -e '\*\*simplify' -e '\*\*retire' docs/streams/assay-product/critical-review-2026-07.md` | ≥ 15 (the enumerated surface is walked, not sampled) |
| 3 | `grep -ci "greenfield" docs/streams/assay-product/critical-review-2026-07.md` | ≥ 1 (stable-vs-greenfield lens applied) |
| 4 | `grep -cE -e '^## Chains' -e '^## Previous run' docs/streams/assay-product/critical-review-2026-07.md` | 2 |
| 5 | `grep -cE -e retire -e simplify docs/streams/assay-product/critical-review-2026-07.md && ls docs/streams/findings/ docs/streams/intake/ \| grep -c "2026-07"` | ≥ 1 routed entry, OR the doc's all-keep argument section exists (`grep -c "nothing to retire" …` ≥ 1) |
| 6 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `2f1b9ff5`, 2026-07-18)

Deliverable `docs/streams/assay-product/critical-review-2026-07.md` present (3973 words); all presence gates met.
The brief's Verify rows 2, 4, 5 carried **#509-class regex bugs** (`\|` inside `grep -E` ERE → literal pipe, not
alternation; row 2 also markdown-bold-intolerant) — the literal commands returned 0 but the **presence intent** is
unambiguously met (validated with corrected patterns). Rows 2/4/5 amended in this commit to the #509-compliant form.

| # | Command | Exit | Output |
|---|---------|------|--------|
| 1 | `test -f … && wc -w` | 0 | 3973 words (≥2000) |
| 2 | `grep -cE -e '\*\*keep' -e '\*\*simplify' -e '\*\*retire' …` (amended; was `\|`-ERE + bold-intolerant) | 0 | 26 (≥15) — 25 verdict cells + 1 body mention |
| 3 | `grep -ci "greenfield"` | 0 | 4 (≥1) |
| 4 | `grep -cE -e '^## Chains' -e '^## Previous run' …` (amended; was `\|`-ERE) | 0 | 2 |
| 5 | `grep -cE -e retire -e simplify …` (amended) && `ls findings/ intake/ \| grep -c 2026-07` | 0 | 19; 118 routed entries (≥1) |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

All presence gates satisfied; the 3 literal-command zeroes traced to brief-authoring regex bugs (#509), not a
deliverable shortfall. `gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.
Verify-table rows 2/4/5 amended in-commit (mid-flight-tweak rule) to clear the #509 NOTICES.

## Review
Gate: model — this run produces findings and proposals only; every retirement/simplification
it proposes carries its OWN gate when authored (changes to human-attended rituals, tamper-guard
logic, or anything risk-flagged stay human-gated there). Reviewer checks verdicts cite
evidence (not vibes), the chain analysis is real, and an all-keep outcome is argued rather
than asserted.
