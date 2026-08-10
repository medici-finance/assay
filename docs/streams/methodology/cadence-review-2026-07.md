# Cadence review — 2026-07-16

**Deliverable**: methodology/38 (cadence-compression research). Re-clocks every weekly/monthly
loop against measured velocity and recommends per-loop cadence changes. Enacts nothing — the
recommended-cadence table is a set of candidates routed through the desk/retro; the one-change
budget applies to ENACTMENT, not to recommendation.

**Data cut**: fresh at implementation time (`go run ./tools/statusgen --dora`, `--trend`,
per-day git/gh splits, 2026-07-17 @ 52cf9d52), superseding the authoring cut (2026-07-12 @
d0222490).

---

## Fact-check

### The claim under test

human:<name> (2026-07-12): "one of the key insights i'm looking at is all the 'weekly' jobs that have
been historically been done can get their cadence reviewed to a daily cycle due to the output
of work going on, and our velocity speeding up 2-3x (you should fact check this number)."

### Multiplier per metric

#### Commits-to-main/day (the cleanest velocity series)

| Window | Days | Commits | Avg/day |
|--------|------|---------|---------|
| Full 28-day DORA window | 28 | 2189 | 78.18 |
| Last 7 full days (07-10…07-16) | 7 | 859 | 122.71 |
| Prior 21 days (06-19…07-09) | 21 | 1330 | 63.33 |
| **Week-over-prior ratio** | | | **1.94x** |
| Pre-July-05 baseline (authoring prior-3-week) | ~21 | ~908 | ~43.2 |
| **Current vs stable pre-surge baseline** | | | **~2.84x** |
| Peak single day (07-16) | 1 | 202 | |
| **Peak vs authoring baseline** | | | **4.68x** |

At authoring the ratio was 2.6x (114/day vs 43.4/day). The ratio compressed to 1.94x not
because velocity slowed but because the baseline lifted — the high-velocity days are now
the baseline. Against the stable pre-July-05 baseline (~43/day), current velocity is 2.84x.

#### Merged PRs/day (deploy frequency)

| Window | PRs | Avg/day |
|--------|-----|---------|
| Full 28-day DORA window | 363 | 12.96 |
| Post-loop era (07-09…07-17) | 344 | 38.22 |
| Pre-loop era (before 07-09) | ~19 over ~15 days | ~1.27 |
| **Post-loop vs full-window ratio** | | **2.95x** |
| Post-loop vs pre-loop | | **30.1x** (PR-loop confound) |

The PR-loop migration confound still applies (the PR-review loop became the default path
2026-07-09; pre-loop merged PRs were infrequent because most work was direct-commit). Post-loop
velocity has *stabilized* at ~38 PRs/day — it is not a migration spike that subsided, it is
the sustained new rate. The brief's authoring noted 55 and 75 PRs on the first two loop days;
the last 7 days average 39/day, which is a lower but sustainable number.

#### Lead time (implemented to done)

| Metric | Authoring cut (07-12) | Current cut (07-16) |
|--------|----------------------|---------------------|
| implemented->done median | 21.4h | 2.6d (62.4h) |
| PR open->merge median | 1.0h (R-01: 0.7h) | 1.2h |
| Briefs reaching done in window | 35 (2026-07-09 read) | 80 (28-day DORA) |
| Done throughput per week (--trend) | 39 (period 07-06) | 67 (period 07-13) |

Lead time *increased* from 21.4h to 62.4h while done-throughput nearly doubled (39 to 67 per
period). This is a distribution shift: more briefs are reaching done, but the *median* brief
spends longer in the verification queue. The tail is longer — verification capacity is the
bottleneck (the "Awaiting verification" queue dropped from 66 to 35 period-over-period, but
the median lead time doesn't yet show it — the --trend 07-06 period had 39 done vs 66
awaiting; 07-13 has 67 done vs 35 awaiting). A calendar week now spans ~2.7 median brief
lifecycles (168h / 62.4h), down from ~8 at R-01. The *throughput* is higher; the *per-brief
latency* is higher. Both are true.

#### Change failure rate

| Metric | Authoring (07-12) | Current (07-16) |
|--------|-------------------|-----------------|
| Change failure (bug-issue slice) | 49% | 43% |
| New bug issues | — | 157 |
| Merged PRs | — | 363 |

The rate is improving (49% -> 43%) but remains elevated. This is a partial signal (bug-issue
slice only; verify-desk VERIFY:FAIL records are not yet automated). The 43% is diagnostic,
not a scoreboard.

### Verdict

**human:<name>'s 2-3x claim: RIGHT-to-understated.** No measured metric is below ~1.9x on
the moving-28-day baseline and ~2.8x against the stable pre-surge baseline. The PR-loop
confound is real but receding — post-loop velocity stabilized at 38 PRs/day, 3x the full-window
average. The lead-time increase (21.4h -> 62.4h) is the one metric moving against the
velocity story, and it is the verification bottleneck — a known, instrumented constraint, not
a velocity reversal. Done-throughput is up (67/week vs 39/week), even though per-brief latency
is higher.

---

## Inventory — cadenced jobs enumerated at implementation time

Re-enumeration via `grep -rniE 'weekly|daily|monthly|cron|schedule:' docs/streams
.github/workflows` + `grep -rni 'kind: CronJob' k8s/` at 52cf9d52. Diff against the brief's
seed table — new or changed since authoring noted.

| # | Loop / job | Current clock | Home (typed ID) | State 2026-07-16 | Change since seed |
|---|------------|---------------|-----------------|-------------------|-------------------|
| 1 | Retro R-NN | weekly (first-run rule: "weekly initially") | RETRO.md, methodology/08 | live; R-01 ran 2026-07-10; R-02 candidate inputs banked; rule-change-rate tracked | same |
| 2 | DORA system read | at-retro (= weekly; consumed as a retro input block) | RETRO.md conventions, methodology/18 | live; computed on-demand via `--dora` | same |
| 3 | Bottleneck report | daily (spec in mm/18 brief) | methodology-metrics/18 | todo | same |
| 4 | Artifact harvest | daily 06:00 UTC (GitHub Actions cron) | methodology-metrics/22, `../oit/.github/workflows/daily-harvest.yml` | **live** (was "brief merged, todo" at authoring) | **promoted**: workflow deployed, running, producing `docs/reports/daily/<YYYY-MM-DD>/` |
| 5 | Bugs GC (closed-issue file prune) | daily 06:00 UTC (companion job in daily-harvest.yml on ubuntu-latest) | `../oit/.github/workflows/daily-harvest.yml` `bugs-gc` job | live | **new** — companion job in the same workflow; light pruning, AI-free |
| 6 | Roadmap deck (daily grid, pages 1..N) | daily (spec) | methodology-metrics/23 + /24 | todo | same |
| 7 | WBR deck | weekly Mon 06:00 UTC (spec) | methodology-metrics/25 | todo | same |
| 8 | Exec summary | monthly, 1st 06:00 UTC (spec) | methodology-metrics/25 | todo | same |
| 9 | Standing code-review sweep | daily (proposed; no durable owner) | INTAKE [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md) | intake, scoped | same |
| 10 | Incremental review sweeps | ad-hoc (watermark register, proposed) | INTAKE [I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) | intake, new | same |
| 11 | Status regen (STATUS.md) | on-push-to-main (CI trigger, not scheduled) | `../oit/.github/workflows/status-regen.yml` | live | **new** — counted because it is cadenced at push frequency (~78 commits/day effectively) |
| 12 | Fleet health check | daily k8s CronJob | `../oit/k8s/dev/app/fleet-health-check-cronjob.yaml` | live | same |
| 13 | Log triage | daily k8s CronJob | `../oit/k8s/dev/app/log-triage-cronjob.yaml` | live | same |
| 14 | Smoke test | daily k8s CronJob | `k8s/{dev,prod}/app/smoke-test-cronjob.yaml` | live | same |
| 15 | Image refresh automation | periodic k8s CronJob | `k8s/{dev,prod}/app/image-automation.yaml` | live | same |

Cluster dailies (items 12-15) are enumerated for completeness; no-cadence-change recommended
(k8s CronJobs at daily are already at the right clock — the loop they feed is infrastructure
health, where <1h data-half-life and automated remediation make hourly a better candidate,
but you don't re-wire CronJobs for methodology reasons).

### Diff summary vs seed table

Two additions since authoring: (5) bugs-gc companion job (already daily, no change needed) and
(11) status regen (push-driven, no schedule to change). One promotion: (4) artifact harvest
went from "brief merged, todo" to live with a running GitHub Actions workflow — the seed
table's "todo" was stale. The harvest being live is itself an early win for cadence compression:
a daily artifact generation loop was specified, built, and deployed within the research window.

---

## Per-loop cadence table

Decision rule (applied per loop, not a blanket): cadence = f(decision latency it feeds,
data half-life, human attention cost). The generation/ratification split is the standard move
for human-attended loops: analysis/artifacts compress to daily (generation is ~free with agent
throughput), the human decision stays on the human's clock.

| # | Loop | Current clock | Recommended cadence | Rule inputs | Evidence |
|---|------|---------------|---------------------|-------------|----------|
| 1 | Retro R-NN | weekly | **Daily generated inputs + weekly human decision** — NOT daily meetings. Data generates daily (DORA snapshot, gate-yield log, staleness sweep, bug-count delta); the human retro session stays weekly, with the one-change budget governing each session. The retro's clock does NOT compress; its INPUTS do. | Decision latency: human attends weekly — no change. Data half-life: retro inputs (STATUS delta, gate yield, FINDINGS age, bug count) all change at daily velocity (~122 commits, ~39 PRs, multiple findings/bugs per day). A retro reading a weekly snapshot is acting on 5-day-stale data by Thursday. Generation: the DORA and staleness inputs are already computable on-demand; running them daily and banking them in `docs/retro-inputs/` costs nothing. | R-01 demonstrated that 2 days of lived data produced more than the estimate assumed (30 PRs, 2 incident-grade findings, 3 flow bugs). A week now spans ~850 commits and ~270 PRs. Daily input generation means the human walks into the retro with current data, not a week-old snapshot. The one-change-per-retro budget is a process-safety mechanism, not a data-freshness constraint — daily inputs make the budget decision better-informed, they don't change the budget. **Reconciles the tension**: analysis cadence compresses; the change-budget clock does not. Rule-change-rate was already made a tracked retro input in R-01; daily input generation feeds that rate daily instead of weekly. |
| 2 | DORA system read | at-retro (= weekly) | **Daily, machine-logged, consumed at retro** — the retro no longer runs DORA as a one-off. A daily `statusgen --dora --json >> docs/retro-inputs/dora-history.jsonl` (or `--dora --series` already implemented in mm/16) plus a retro-time summary. The full five-metric system read stays a retro input block; the daily recording means the retro gets a *trend*, not a point. | Decision latency: the retro's consumption clock doesn't change. Data half-life: DORA metrics move daily (commits, PRs, bugs, lead-time). A weekly DORA read is a single point from a daily-moving series. Generation: mm/16 (`--dora --series`) already computes this; wiring it as a daily harvest step is a one-line script change. | The 28-day DORA window at authoring reported 21.4h lead time; the current cut reports 62.4h. The metric changed 3x in 4 days — a weekly sample would have seen only the 62.4h number and missed the acceleration entirely. Daily recording captures the trajectory; the retro reads the series. |
| 3 | Bottleneck report | daily (spec) | **Daily** — already specified at daily, keep it. The constraint moves at daily velocity (dispatch -> verification in 2 days, R-01 observed). A slower clock would miss shifts. | Decision latency: feeds the desk's same-day dispatch decision. Data half-life: the bottleneck shifts intra-week (dispatch 07-08, verification 07-10 — 2 days). Human attention: desk reads it, not human:<name>; model-generated, desk-consumed. | The pipeline constraint moved twice in 3 days (R-01). A weekly bottleneck report would have reported "dispatch is the bottleneck" on Friday when verification had been the bottleneck since Tuesday. Daily is the minimum viable clock. |
| 4 | Artifact harvest | daily 06:00 UTC | **Daily** — already live at daily. No change. The harvest is the reference implementation of generation/ratification split: AI-free collector commits artifacts; desks consume them on their own clocks. | Already at the right cadence. | Live since 2026-07-16; `docs/reports/daily/` population is the proof. |
| 5 | Bugs GC | daily 06:00 UTC | **Daily** — already daily. No change. | Lightweight; runs in seconds; no human attention. | |
| 6 | Roadmap deck | daily (spec) | **Daily** — already specified at daily. Generation is free (deterministic `statusgen --roadmap` emitting the grid + per-stream panels). | Already at daily spec. | |
| 7 | WBR deck | weekly Mon 06:00 UTC | **WBR stays weekly** — do NOT compress to daily human review. The audience clock is calendar-set (human:<name> reads it on his rhythm; the deck is a synthesis artifact, not a real-time dashboard). Generate the deck daily (same pipeline as harvest/roadmap — free), but the human review remains weekly. **Optional add: a daily mini-trend sidebar** — one paragraph of "since last WBR: N done, M PRs merged, K new bugs, top bottleneck" auto-generated into `docs/reports/daily/` and cited in the weekly deck. This is the `--trend` output, already live. | Decision latency: human:<name> reads the WBR on his clock — maintain it. Data half-life: weekly is fine for a synthesis read; the daily mini-trend covers the freshness gap. Human attention: the WBR is a human-read artifact; generating it daily and sending it to human:<name> daily would fatigue him. Generation: the deck's data inputs are cheap (statusgen outputs); generating the full deck daily as a scheduled artifact is harmless — it sits in `docs/reports/daily/` and the WBR pulls the week's worth into a synthesis. | The `--trend` output already produces a weekly cadence read (period 07-06 vs 07-13: 39 -> 67 done, 66 -> 35 awaiting). A daily mini-trend is the same query with a daily bucket — one additional `--trend --daily` flag. The WBR deck (mm/25) already specifies a Monday-morning generation schedule; adding a daily mini-trend to the harvest pipeline is additive, not a replacement. |
| 8 | Exec summary | monthly, 1st 06:00 UTC | **Monthly exec summary unchanged** — but confirm rather than assume. The audience clock is calendar-set (external readers, monthly rhythm). Data half-life is slow for exec-level signals (month-over-month trends, not intra-week velocity). The generation pipeline that feeds it runs daily anyway (same harvest/trend data). **Verdict: monthly confirmed — no compression.** | Decision latency: external audience, monthly rhythm — no change. Data half-life: slow for exec signals (trends, not intra-week noise). Human attention: monthly is the contract with the audience; compressing it frustrates them. | Exec summaries aggregate; the daily noise is explicitly what they filter out. The harvest + trend machinery already captures the raw data daily; the monthly summary reads the month's series, not a point. |
| 9 | Standing code-review sweep | daily (proposed; no durable owner) | **Daily, watermark-diff, tiered models** (the I-20 x I-36 composition). I-36 carries the cost model: review only the delta since the last sweep's watermark, with cheap models on the wide pass and strong models on escalation. The cost model makes daily affordable (~85-90% cheaper than the 2026-07-09 16-Opus full sweep). **Recommendation: scope and own this.** The I-20 daily cadence demonstrated value (day-1 genius-sweep produced actionable needs-fixing / need-patch / red-team artifacts). I-36's incremental approach makes it affordable. The open question from I-20 (who owns the run?) is still open — either a standing desk window or a CI-scheduled job. Leans toward a CI-scheduled job (deterministic residue computation + `gh pr diff` data collection) with a desk consuming the output — same generation/ratification split as harvest. | Decision latency: the output feeds FINDINGS and the desk's dispatch decision; a daily signal has 1-day latency vs a weekly sweep's 7-day latency. Data half-life: at 122 commits/day, a weekly sweep reviews ~850 commits at once — too large to be thorough. Daily sweeps review ~122 commits each, a tractable batch. Human attention: the sweep is model-executed and desk-consumed; human:<name> doesn't read raw sweep output. | The 2026-07-09 genius-sweep found actionable defects that per-PR review missed (drift across merges, cross-cutting issues). The I-36 cost model: a daily watermark-diff sweep on ~122 new commits uses 2 cheap-model passes + selective escalation, vs a weekly sweep on 850 commits that would need a large-model pass to be thorough. Daily is cheaper AND more thorough — counterintuitive but true because batch size drives model cost. |
| 10 | Incremental review sweeps | ad-hoc (watermark register) | **Ad-hoc but armed with a daily trigger** — the watermark register is committed; a CI job checks "last watermark <= 24h ago" and runs if not. The sweep itself is the I-20 + I-36 composition (item 9). | Decision latency: same as item 9. Data half-life: same. Human attention: same. | This is the mechanism for item 9, not a separate loop. The watermark register is the durable home for the sweep cadence. |
| 11 | Status regen | on-push-to-main (CI) | **Push-driven** — already the right clock. Push frequency IS the right cadence for a derived artifact. No schedule change. | Push-triggered, not scheduled. | STATUS.md is a single-writer artifact; its clock is the push clock, which IS the velocity clock. |
| 12-15 | Cluster dailies | daily k8s CronJobs | **Daily** — already daily. No change. These are infrastructure health loops; daily is the right clock for fleet-health/log-triage/smoke-test/image-refresh. | Already at daily. | Enumerated for completeness; methodology cadence review doesn't drive infra CronJob changes. |

### Summary of recommendations

| Loop | Action |
|------|--------|
| Retro inputs (1) | Compress to daily generation; human session stays weekly |
| DORA read (2) | Daily machine-logged; consumed as a trend at retro |
| Bottleneck report (3) | Daily (keep) |
| Artifact harvest (4) | Daily (already live) |
| Bugs GC (5) | Daily (keep) |
| Roadmap deck (6) | Daily generation (keep spec) |
| WBR deck (7) | Weekly human review (keep); add daily mini-trend sidebar |
| Exec summary (8) | Monthly (keep; confirmed the brief's hypothesis) |
| Standing review sweep (9) | Daily watermark-diff, tiered models (scope and own) |
| Incremental sweeps (10) | Daily trigger via watermark register |
| Status regen (11) | Push-driven (keep) |
| Cluster dailies (12-15) | Daily (keep) |

**Four candidates from the brief, answered with evidence:**

1. **Retro -> daily inputs + weekly human decision** — RECOMMEND. The retro's clock does not compress; its inputs do. The one-change-per-retro budget is a process-safety mechanism, not a data-freshness constraint. Daily input generation makes the budget decision better-informed.

2. **Review sweeps -> daily watermark diffs (I-20 x I-36)** — RECOMMEND. The I-36 cost model makes daily affordable (~85-90% cheaper than full sweeps). Per-PR review at merge covers the per-change surface; the daily sweep covers composition (defects from PR interactions). Daily is cheaper AND more thorough because batch size drives model cost.

3. **WBR weekly deck stays; daily mini-trend sidebar** — RECOMMEND (keep weekly, add daily mini-trend). The WBR is a synthesis artifact for human reading; the audience clock is human:<name>'s. The daily mini-trend (`--trend --daily`) covers the freshness gap. The full deck is already specified to generate on a Monday schedule; the mini-trend is additive.

4. **Monthly exec summary unchanged** — CONFIRMED. The audience clock is external and monthly; the data half-life for exec-level signals is slow; compressing would frustrate the audience. The brief's hypothesis that this one stays monthly is correct.

---

## Amendments

Each accepted recommendation mapped to its owning artifact by typed ID. Rejected candidates get
one line of why. **These amendments are enumerated but NOT enacted in this PR** — enactment is
routed through the desk/retro under the one-change budget.

### Accepted (candidate amendments, per owning artifact)

| Artifact | What changes | Who ratifies |
|----------|-------------|-------------|
| **RETRO.md conventions (methodology/08)** | Add a "daily retro-input bank": a directory `docs/retro-inputs/` populated by a daily CI job (or the harvest workflow) with `statusgen --dora --json`, `--trend --daily`, gate-yield log, bug-count delta, staleness sweep. The retro checklist gains a line: "Read today's retro-input bank" instead of "Run DORA now." The one-change-per-retro budget and rule-change-rate tracking are unchanged. | human:<name> (retro participant; the retro's clock is his) |
| **methodology-metrics/18 (bottleneck report)** | No change to the brief — daily spec stands. If the brief's Verify block proves the daily path works, mark it done; if not, the amendment is the implementation, not the spec. | Desk (the report feeds the desk's dispatch decision) |
| **methodology-metrics/22 (daily harvest)** | Already live — no amendment needed. The harvest workflow is the reference implementation of the generation/ratification split. The brief's Verify block should confirm the daily cadence is producing artifacts and mark it done. | Desk (verify-gate) |
| **methodology-metrics/25 (WBR deck + exec summary)** | Add a daily mini-trend sidebar to the spec: `statusgen --trend --daily` output included in the harvest and referenced by the weekly WBR deck. The WBR weekly generation schedule and the monthly exec summary schedule are unchanged. The mini-trend is a small, additive feature, not a rescope of the brief. | human:<name> (the WBR is read on his clock; the mini-trend is machine-generated) |
| **INTAKE I-20 (daily code-review cadence)** | Disposition: **accepted** — scope and own. The I-20 proposal demonstrated value; the I-36 cost model makes it affordable. Create a methodology brief to own the implementation: a CI-scheduled job (or a standing desk window) that computes the watermark diff, runs the review sweep on it, and emits findings to the FINDINGS register. | human:<name> (scope decision: brief or desk-window) |
| **INTAKE I-36 (incremental review sweeps)** | Disposition: **accepted** as the cost-model companion to I-20. The I-20 owning brief bakes in the watermark + diff-scope + tiered-model design from I-36. The I-36 disposition moves to `scoped` (resolved into the owning brief). | human:<name> (same scope decision as I-20) |

### Rejected

| Candidate | Why rejected |
|-----------|-------------|
| Retro -> daily human sessions | The retro's clock is human:<name>'s attention. Compressing the human session to daily would burn his time for marginal gain. The generation/ratification split gives him daily data at a weekly meeting — the right trade. |
| WBR -> daily human review | Same: human:<name>'s reading clock, not the machine's generation clock. The daily mini-trend sidebar covers the freshness gap without fatigue. |
| Exec summary -> weekly | The audience clock is external and monthly. A weekly exec summary to an external audience reads as noise. The data half-life for exec signals is slow enough that monthly is right. |
| Blanket "make everything daily" | Explicitly rejected — each loop has its own decision-rule inputs. The cluster dailies are already daily; the exec summary should stay monthly; the retro human session should stay weekly. A blanket rule would get half the decisions wrong. |

---

## Tension resolution: retro-budget vs compressed cadence

R-01 decided rule-change RATE should trend DOWN toward the one-change-per-retro steady state.
Compressing the retro CADENCE (more retro sessions) would multiply the change budget. The
recommendation resolves this:

- **Retro inputs compress to daily** — the retro session gets fresher data.
- **The retro SESSION stays weekly** — the change-budget clock does not accelerate.
- **The one-change-per-retro budget is unchanged** — the budget is a process-safety mechanism,
  not a data-freshness constraint.
- **Daily input generation makes the budget decision BETTER-INFORMED, not larger** — a
  well-informed single change beats a poorly-informed single change.

The rule-change-rate tracking (R-01's final decision) is itself a retro input that benefits
from daily generation: instead of counting at retro time, the count is available daily.

---

## Goodhart guardrail

Cadence and velocity numbers are diagnostic, never targets. A cadence chosen to flatter a
metric is a retro finding. The recommendations above compress cadence because the data
half-life demands it (a weekly sample misses ~850 commits and ~270 PRs), not to make a
number look better. If daily DORA recording leads to daily DORA-watching and metric-gaming,
that is a retro finding — kill the daily recording and return to retro-time DORA.

---

*Generated 2026-07-17 by brief methodology/38 at commit 52cf9d52. Fact-check numbers
regenerate from `go run ./tools/statusgen --dora`, `--trend`, `git log --format="%ad" --date=short`, and `gh pr list --state merged --json mergedAt`. Recommendations are
candidates only — enactment is gated by the desk/retro under the one-change budget.*
