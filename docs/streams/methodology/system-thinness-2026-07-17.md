# Where the system is thin today — an evidence read of the loop model vs reality

**Date:** 2026-07-17 · **Origin:** human:<name>, 2026-07-17 — while updating the loops deck, "look at the actual
system evidence and see how it maps." The deck stays about the *flow*; this is the separate honest
read of *where the flow is thin today*, written up as a spec to be worked on. **Decision owner:** human:<name>.

## Method

Mapped each element of the loop model (two front doors → worker → review → merge → verify → done, with
the feedback loop) against real system evidence — `statusgen --dora`, the STATUS.md board, the
INTAKE / FINDINGS / RETRO registers, GitHub PR/issue/review activity, and the daily good/bad/ugly
harvest — over the 28-day window ending 2026-07-17. Every element is instantiated (nothing on the
diagram is aspirational-only). The point below is not "it's broken" — it's *where the loop is
lopsided*, with the number that says so.

## The evidence (28-day window, 2026-06-19 → 2026-07-17)

- **Forward pipeline — fat and fast.** 2281 commits / 281 merges; **392 PRs merged (14/day)**; commit→merge
  median **1.3h**. Every sampled merged PR carries a real `reviewer-app[bot]` verdict. The last
  20 merges were **all by `human:<name>`** (100% human merge). Build → review → merge is robust and as-drawn.
- **Verify tail — thin.** DORA reports **change-failure-recovery, rework, and part of change-failure all
  as "needs verify-desk"**; the board shows **13 verified awaiting review** (51 total: 38 at implemented,
  13 verified) against many `implemented`; the known
  verification-debt line (51 awaiting vs 169 done) is the standing constraint. The verify station is
  drawn as an equal, but it is the under-served tail.
- **Intake front door — drained, needs to stay drained.** **~19 of 80 intake entries are `disposition: new` (~24% un-triaged) —**
  the triage drain landed (eacedee, 2026-07-16); what remains needs to flow through the standing triage-verbs loop.
  The spec → author-brief conversion is keeping pace with what comes in.
- **Feedback loop — senses strongly, closes weakly.** **29 of 41 findings are about the *process itself*
  (b)**, not the product — so "the system files issues about itself" is strongly true. But **RETRO has
  2 entries** and the good/bad/ugly harvest has ~2 days of reports: the loop *observes* far more than it
  *converts to recorded process change*.
- **Change failure ≈ 41%** (partial: 162 bug-issues ÷ 392 merged PRs) — high, and flagged incomplete
  precisely because verify-desk isn't feeding the true rate.

## The thin areas (ranked) — each a candidate work item

1. **Verify-desk is the constraint — instrument it and drain it.** *Symptom:* change-failure-recovery,
   rework, and the true change-failure rate all read "unknown / needs verify-desk"; 13 verified awaiting review on the
   board. *Why it matters:* three of the five DORA signals are blind, and merged-but-unverified work
   accumulates as invisible risk. *Work:* wire verify-desk's VERIFY:FAIL / recovery / rework records
   into statusgen so change-failure, recovery-time, and rework compute for real; then drain the awaiting
   queue (a verification-throughput push, or more verify capacity).

2. **Keep the intake triage-verbs loop running — the drain landed, keep it drained.**
   *Symptom:* ~19/80 intake entries un-triaged (~24%) — the triage drain landed (eacedee, 2026-07-16).
   The runtime is correctly wired: `../oit/.claude/skills/intake-desk/SKILL.md` (delivered under the old issue-loop skill
   name by brief-11/PR #576, 2026-07-16; renamed to intake-desk by brief-13) already
   owns BOTH lanes — its boot runs the intake-debt alarm and its "intake lane" section makes draining
   the front door the desk's standing job. The only thing that was stale is brief-09's *Verify target*
   (it grepped the deleted `~/.claude/skills/the-desk/SKILL.md`), pure doc hygiene now landed via PR #701
   (F-41 resolved). The lane is a standing flow now — not a clog but a drip. *Work:* keep the triage-verbs loop as a
   **non-skippable standing step of each issue-loop run** (or a driven cadence) so the intake lane stays
   drained and can't silently fall behind the issue lane again. No wiring change needed — this is a keep-the-cadence
   fix, not a plumbing fix.

3. **Feedback loop closes weakly — findings pile up, RETROs don't.** *Symptom:* 29 process-findings vs 2
   RETRO entries. *Why it matters:* the self-improvement loop's *value* is the recorded process change
   (RETRO), not the observation; a register of 29 process-findings with 2 retros is a loop that senses
   and forgets. *Work:* a RETRO cadence that actually converts process-findings into recorded changes
   (the RETRO bootstrap methodology/08 exists — the cadence needs to run and cite findings), and a
   metric of *findings-resolved-via-RETRO* so the close-rate is visible.

4. **Change-failure 41% is high and half-blind.** *Symptom:* 162 bug-issues / 392 PRs, partial signal.
   *Why it matters:* either the true rate is materially different (verify-desk data would tell), or 41%
   is real and the review gate is passing too much. Depends on #1 to resolve. *Work:* once verify-desk
   feeds the rate, decide whether the number is a measurement gap or a quality signal, and route
   accordingly.

## Make this a standing analysis job (human:<name>, 2026-07-17) — item 5, and it fixes item 3

This evidence read — mapping each loop station to its own signal and flagging where the flow is thin —
must **not stay a one-off**. It belongs in the **daily analysis job** (the good/bad/ugly harvest — the
feedback loop's ANALYSIS station), run **daily as a snapshot and weekly as a trend**. This is itself the
fix for item 3: the loop's weakness is that it *senses* (findings) more than it *converts* — a standing
loop-health read that auto-files what it finds is the conversion.

Concretely, the harvest gains a **loop-health / "where is it thin" section** that computes, per station,
the same signals used here:

| Station | Signal it computes | "Thin" flag |
|---|---|---|
| worker-desk | throughput — PRs/day, commits/day | falls off trend |
| pr-review-desk | reviewer-App-verdict coverage — share of merges with an App review | < ~100% |
| merge gate | human-merge share | < 100% |
| **verify-desk** | `verified` ÷ `implemented`; and whether change-failure / recovery / rework are instrumented | ratio low, or signals read "unknown" |
| intake-desk | un-triaged count + oldest age | past the 3-day alarm / rising |
| feedback loop | process-vs-product findings split; **RETRO close-rate** (findings resolved via RETRO) | close-rate low |

It runs off data the system already emits (`statusgen --dora`, the register counts, `gh` PR/review
activity — most of it deterministic, no AI, the METRICS half; the "so-what" narration is the ANALYSIS
half). Its output is a first-class part of the good/bad/ugly report **and — closing the feedback loop —
its flagged thin-areas are filed back as intake requests or issues** (per the feedback-loop model), so
"where is the system thin" becomes the standing self-diagnostic the ANALYSIS station is meant to be,
not a manual read someone has to remember to run.

## The dashboard — the metrics with trendlines (item 6, human:<name> 2026-07-17)

Item 5's daily/weekly analysis must land in a **dashboard, not only a prose report**: the per-station
loop-health signals + the DORA system rendered as current values **and trendlines**. We already have the
data for trends — no new collection needed:
- **git history** over the window (28 days today) — throughput, lead time, merge/PR series;
- **`docs/streams/.history.jsonl`** — the single-writer lifecycle-transition log (todo→…→done over time);
- **`statusgen --dora --series` / `--trend --daily|--weekly`** — already buckets the metrics by day/week;
- the **daily good/bad/ugly harvest** series in `docs/reports/daily/`.

Shape of the dashboard (the visual surface of the ANALYSIS station):
- **one card per loop station** (intake-desk · worker-desk · pr-review-desk · merge · verify-desk) plus
  the **feedback loop**, each with its current signal and a **sparkline / trendline**;
- the **DORA system** (deployment frequency, lead time, change-failure, recovery, rework) with trends;
- the **thin flags** (verify-desk instrumentation gaps, intake untriaged age, RETRO close-rate) as
  at-a-glance state — *good / warning / thin*;
- **generated by the daily job**, single-writer like STATUS.md (never hand-edited), in the brand-dark
  ops style; charts follow the dataviz rules (validated palette, one axis, trendline with emphasized
  endpoint).

This is the concrete form of the "loop-monitoring dashboard / WIP website over the standing loops"
already in INTAKE (2026-07-10) — now fed by the item-5 metrics, so it stops being a mockup and becomes
the live self-diagnostic.

## Disposition

A spec for human:<name> to route into work. **Six** candidate briefs now:
1. **verify-desk instrumentation + drain** (load-bearing — unblocks three DORA signals, the real constraint);
2. keep the standing triage-verbs loop running (drain landed — eacedee, 2026-07-16);
3. RETRO close-rate — convert process-findings into recorded changes;
4. change-failure interpretation (behind #1);
5. **the loop-health / thinness read becomes a standing part of the daily & weekly analysis job**, auto-filing thin-areas back to the front doors;
6. **the metrics land in a generated dashboard with trendlines** (per-station cards + DORA + thin flags), the live form of the loop-monitoring dashboard.
Registered via `../oit/docs/streams/intake/2026-07-17-where-the-system-is-thin.md`.
