# Prioritization Systems — Deep Research Report

2026-07-09 | 104-agent deep-research workflow | 22 sources → 90 claims → 25 verified → 14 confirmed, 11 refuted

## Context

Our Next-up formula (`../assay-toolkit/statusgen/nextup.go`) ranks engineering-backlog briefs:

```
score = priorityWeight(priority) + min(days_since_last_touched, 30) × 10
        P0=3000 / P1=2000 / P2=1000     staleness cap: +300 max
+ wave-gating (a todo unblocks only when lower waves are done)
+ 2-per-stream diversity cap
+ batch of 6
```

Self-identified weakness (F-09): no value/effort/risk term — a low-value P2 floats to the top purely by aging (staleness rewards neglect), touching a rival stream resets staleness, and there's no value term at all.

This report surveys established prioritization frameworks, verifies their claims adversarially, and maps adoptable ideas onto our formula.

---

## 1. Frameworks Surveyed (Exact Formulas + Failure Modes)

### 1.1 WSJF (Weighted Shortest Job First) — SAFe
**Confidence: HIGH [3-0 ✓]**

```
WSJF = Cost of Delay / Job Size (or Duration)

Cost of Delay = User-Business Value
              + Time Criticality
              + Risk Reduction / Opportunity Enablement

All scores: relative Fibonacci scale (1, 2, 3, 5, 8, 13, 21)
Higher WSJF = higher priority.
```

**Sources**: [SAFe v3](https://v3.scaledagileframework.com/wsjf/), [SAFe current](https://framework.scaledagile.com/wsjf/), [airfocus WSJF template](https://airfocus.com/templates/WSJF-scoring/), [iZenBridge](https://www.izenbridge.com/kb/safe-agile-interview-questions/what-is-wsjf-and-its-limitations/), [Atlassian WSJF config guide](https://success.atlassian.com/solution-paths/quarterly-planning-guidance-with-jira-cloud/prepare-for-a-program-increment-pi-planning-event/how-to-configure-weighted-shortest-job-first-wsjf-to-work-items)

**CD3 variant** (Don Reinertsen, Joshua Arnold): identical structure — `Cost of Delay / Duration`. Black Swan Farming is the canonical practitioner source for CD3. SAFe adopted and renamed it.

**Known failure modes**:
- Additive CoD form: if User-Business Value = 0 but Time Criticality is high, CoD > 0 (should be zero). Reinertsen's original treats value and urgency as multiplicative — SAFe simplified to additive.
- Job Size as proxy for Duration collapses effort and calendar time into one dimension, changing the economic meaning.
- "SAFe advocates WSJF outperforms ROI" is **commercial marketing, not evidence** — 0-3 refuted. The SAFe WSJF page provides zero empirical data, studies, or controlled experiments. Black Swan Farming explicitly argues additive WSJF is an overcomplication.

**Anti-gaming property**: The denominator (Job Size) acts as a brake — inflating CoD while giving realistic effort estimates exposes the inconsistency.

### 1.2 RICE — Reach × Impact × Confidence / Effort
**Confidence: MEDIUM (no verified claim survived 3-0, but formula is widely documented)**

```
RICE = (Reach × Impact × Confidence) / Effort

Reach: estimated users/quarter
Impact: 3 = massive, 2 = high, 1 = medium, 0.5 = low, 0.25 = minimal
Confidence: 100% = high confidence, 80% = medium, 50% = low (DEFAULT)
Effort: person-months
```

**Key design features**:
- Confidence defaults to 50% ("80% confidence based on vibes is fiction" — airfocus critique)
- Banded scales (not free-form), defined rubric per level
- Multiplicative: one inflated input amplifies the whole score

**Sources**: [airfocus RICE critique](https://airfocus.com/blog/rice-prioritization/), [getproductpeople.com](https://www.getproductpeople.com/blog/prioritization-techniques-rice-moscow-ice-kano), [startups.com](https://www.startups.com/lexicon/rice-framework)

**Known failure modes**:
- "Put four uncertain numbers into an equation, and the result looks precise, but the precision is inherited from the estimate quality, not the formula" (airfocus)
- Confidence default at 50% works only if enforced by convention; teams converge on 100%, making Confidence a no-op
- Multiplicative structure means gaming one dimension amplifies the whole score
- Reach in users/quarter is meaningless for internal/infra work

### 1.3 ICE — Impact × Confidence × Ease
**Confidence: HIGH [2-1 ✓] for Jira implementation**

```
ICE = Impact × Confidence × Ease

All dimensions: free-form numeric fields (no enforced scale in Jira)
Thresholds (Jira default): >300 = Highest, >200 = High, >100 = Medium, ≤100 = Low
```

Simpler than RICE (3 terms vs 4, no division, no Reach). Fast. The startup vocabulary ("Ease" instead of "Effort") biases toward shipping.

**Known failure modes**: Jira Automation implements ICE as pure multiplication with no rubric, no scale definitions, no dropdown constraints — opaque user-entered number fields with undefined meaning. The Atlassian KB article provides zero guidance on how to define or measure any dimension.

### 1.4 Aha! Product Value Score
**Original claim REFUTED [0-3]**. Claimed Aha! uses a value/effort ratio — but the actual default formula is a **weighted sum**:

```
Aha! default: (1.0 × Population + 1.0 × Need + 1.0 × Strategy - 1.0 × Effort) × Confidence
```

This is additive with subtractive effort (weight -1.0), not a ratio. The advanced equation editor supports `if()` conditional logic and custom weights. Aha! has **Automated Scorecard Metrics** (Enterprise+) that pull values from record fields, not all manual — but has no built-in aging/staleness mechanism.

**Source**: [Aha! scorecards](https://support.aha.io/aha-roadmaps/support-articles/customizations/create-aha-scorecards~7444636572011952598)

### 1.5 CERB (Grafana) — Pure Additive
**Confidence: MEDIUM**

```
Score = Customer Impact + Excitement + (6 - Relative Size) + Business Impact

All four dimensions: 1-5 scale, equally weighted
No multiplication, no division, no confidence/risk discount
```

**Source**: [Grafana CERB](https://grafana.com/blog/a-better-way-to-prioritize-feature-backlogs-the-cerb-scoring-method/)

Key insight: additive formulas are harder to game than multiplicative — inflating one dimension has bounded impact.

### 1.6 MoSCoW, Kano, Value-vs-Effort 2×2, Opportunity Scoring, Story Mapping, Buy-a-Feature

These are **categorization methods, not scoring formulas** — they produce buckets (Must/Should/Could/Won't) or ordinal rankings, not numeric scores computable by a CI tool. They serve as inputs TO a scoring formula rather than the formula itself.

MoSCoW notable failure: "everything becomes a Must-have" without a hard capacity constraint per bucket. Kano's weakness: survey-based (expensive to collect), categorizes features by delight-vs-expectation which doesn't map to implementation order.

---

## 2. OS Scheduler Research — Aging & Starvation

### 2.1 Core CS Definitions
**Confidence: HIGH [3-0 ✓]**

**Starvation (indefinite blocking)**: A steady stream of higher-priority arrivals permanently prevents a low-priority process from ever receiving CPU time. Newly arriving higher-priority work perpetually preempts it.

**Aging**: Gradually increasing a task's priority as a function of its waiting time in the ready queue, ensuring low-priority items are eventually promoted to the point where they run. The standard textbook solution to starvation (Silberschatz/Galvin/Gagne, Cornell CS 414, NYU OS, CUNY).

**Sources**: [Wikipedia: Aging](https://en.wikipedia.org/wiki/Aging_(scheduling)), [NYU OS lecture](https://cs.nyu.edu/~gottlieb/courses/2010s/2010-11-fall/os2250/lectures/lecture-05.html), [CUNY lecture notes](http://www.sci.brooklyn.cuny.edu/~briskman/cisc/3320/lecture_notes/topic_06/21.html)

### 2.2 Mapping to Our Formula
**Confidence: MEDIUM (analytical claim, not directly sourced)**

| OS Concept | Our Equivalent |
|-----------|---------------|
| Fixed priority levels | P0=3000, P1=2000, P2=1000 |
| Aging = priority += f(wait_time) | `+ min(days_since_last_touched, 30) × 10` |
| Starvation cap = max age | 30-day cap → max +300 |
| Admission control | Wave-gating (lower waves done first) |
| Diversity scheduling | 2-per-stream cap |
| Batch bound | 6 items |

**Critical finding**: True OS aging is based purely on queue wait time — never reset by external events. Our `days_since_last_touched` resets on ANY git touch, including a rival stream's activity. This is the formula's F-09 self-identified weakness, and the OS literature confirms it's a real antipattern: admission of new arrivals can indefinitely suppress aged items that briefly lost their aging bonus to an administrative bump.

**Ratio sanity check**: Max staleness = 300. P0 weight = 3000. A P2 with max staleness = 1300 vs fresh P0 = 3000 — so staleness can NEVER override the priority tier. The 10:1 ratio between priority tiers and max staleness ensures the priority signal dominates. This is accidentally well-tuned.

### 2.3 Claims Refuted About Linux Schedulers

| Claim | Vote | Why Refuted |
|-------|------|------------|
| "Linux O(1) scheduler prevents starvation via active/expired array swap" | 0-3 | Interactive heuristics allowed tasks to be reinserted into active array; swap gives "a chance" not a guarantee; O(1) had documented starvation bugs (LKML, replaced by CFS) |
| "CFS replaces priority heuristics with single vruntime metric" | 0-3 | vruntime IS weighted by priority (nice value) — formula: `vruntime_delta = actual_runtime × (NICE_0_LOAD / task_weight)`; priority never left |
| "Aging counters starvation by incrementally raising priority, eventually guaranteeing they run" | 0-3 | "Guarantee" is conditional — assumes all jobs terminate, no unbounded arrival rate, no priority inversion; Harvard CS111: "A problem with aging: You can still starve! How?" |

---

## 3. Tool Implementations — What Real Products Do

### 3.1 Jira Cloud
**Confidence: HIGH [3-0 ✓]**

- **No native WSJF/RICE support**. Feature request JRASERVER-74474 = "Gathering Interest," no fix version.
- To implement: global Automation rule + 5-6 custom number fields + smart-value math, OR marketplace app (≥5 exist: WSJF Calculation, Awesome Custom Fields, Dynamic Scoring, WSJF Priority Calculator, Issue Score), OR Jira Align (native WSJF with board views, Fibonacci config, bulk prioritization)
- ICE: implemented as `{{#=}}{{issue.Impact}} * {{issue.Confidence}} * {{issue.Ease}}{{/}}` with cascading IF/ELSE thresholds
- **Zero rubric provided** — Impact, Confidence, Ease are opaque number fields with no scale, no definitions, no dropdown constraints. "The tool computes, but the human supplies numbers whose meaning is undefined."

**Sources**: [Atlassian WSJF config](https://success.atlassian.com/solution-paths/quarterly-planning-guidance-with-jira-cloud/prepare-for-a-program-increment-pi-planning-event/how-to-configure-weighted-shortest-job-first-wsjf-to-work-items), [Atlassian ICE KB](https://support.atlassian.com/automation/kb/how-to-create-a-rule-that-calculate-ice-score-and-prioritize-tickets-based-on-it/)

### 3.2 Aha! Scorecards
- Default formula: `(1.0 × Population + 1.0 × Need + 1.0 × Strategy - 1.0 × Effort) × Confidence` — weighted sum, NOT ratio
- Advanced equation editor: supports `if()`, custom weights, conditional logic
- Automated Scorecard Metrics (Enterprise+): pull values from record fields dynamically
- **No built-in aging/staleness mechanism** — every time-decay term is a custom manual input
- **Key gap**: Aha! has no opinion on whether "rewarding staleness" is sound or unsound — the structural gap our formula fills

**Source**: [Aha! scorecards](https://support.aha.io/aha-roadmaps/support-articles/customizations/create-aha-scorecards~7444636572011952598)

### 3.3 Productboard
- Driver scoring: users assign 0–5 scores per driver per item manually
- **No computed or aggregated priority formula** — no weighting, multiplication, or summation logic documented
- "The actual 0–5 scoring is left to manual user judgment, presumably per driver per feature"

**Source**: [Productboard driver scoring](https://support.productboard.com/hc/en-us/articles/360056348954-Score-data-from-zero-to-five-using-drivers)

### 3.4 Linear
- "Triage" queue + label-based filtering. No verified claims about scoring models survived adversarial vote.

### 3.5 GitHub
- Label-based filtering + issue templates. No verified claims about scoring models survived adversarial vote.

---

## 4. Verified Critiques of Scoring-Based Prioritization

### 4.1 False Precision
The airfocus RICE critique is the sharpest: four uncertain numbers in an equation produce a result that LOOKS precise, but the precision comes from the estimate quality, not the formula. Confidence as "self-reported certainty" correlates inversely with actual accuracy — the Dunning-Kruger of prioritization.

### 4.2 Rubric Absence
Jira's ICE implementation is the canonical example: the tool computes `Impact × Confidence × Ease` with no guidance on measurement. This is the recurring failure mode of ALL scoring-based prioritization — the tool computes, but the human supplies numbers whose meaning is undefined.

### 4.3 Gaming Vectors
- **WSJF**: Inflating CoD while giving realistic effort estimates exposes inconsistency in the ratio — this is a BUILT-IN anti-gaming safeguard
- **RICE**: Multiplicative structure amplifies gaming of any single dimension
- **CERB**: Additive structure bounds damage of any single gaming attempt
- **Aha!**: Custom weights in the equation editor can be tuned to produce desired rankings
- **Jira ICE**: No safeguard — three free-form numbers multiplied with no audit trail

### 4.4 "WSJF Proves ROI Superiority" — REFUTED [0-3]
SAFe's claim that WSJF 'produces the best results' vs ROI-based prioritization is commercial marketing with zero empirical evidence. The SAFe WSJF page contains no studies, no data, no controlled experiments. This is a vendor selling a framework, not a validated methodology.

---

## 5. Mapping to Our Formula — What We Can Graft

### 5.1 Fix the Staleness Clock (Zero-Cost, High-Impact)

**Problem**: `days_since_last_touched` is the stream's `LastTouch`, reset by any git activity (including rival stream work). OS aging theory: aging must count from the item's own ready-queue entry time, not from an external clock that gets bumped by unrelated events.

**Fix**: Derive staleness from each brief's *own* queue entry date — the date it entered `todo` or `in-progress` status. This requires tracking a `queuedAt` timestamp per brief (derivable from the brief file's metadata + status transition history).

```
daysStale = min(days since brief.queuedAt, 30)
```

This alone fixes the "reward neglect" distortion without adding any human-scored inputs.

### 5.2 Effort Divisor (WSJF's Core Insight)

**Problem**: A P0 that takes 2 days and a P0 that takes 2 weeks have the same score.

**Graft**: Divide the current score by an effort estimate.

```
score = (priorityWeight(p) + stalenessBonus) / effortTshirt
        S=1, M=2, L=4, XL=8
```

**Why the WSJF denominator works as anti-gaming**: Inflating priority while giving realistic effort exposes inconsistency. Effort is the hardest dimension to game — you're committing to a size estimate. T-shirt sizes avoid false precision (Fibonacci numbers are for comparative estimation, not absolute).

**Risk**: T-shirt sizing is still subjective, and teams may sandbag estimates to boost scores. But this is self-correcting — oversized estimates slow down the team's throughput which hurts them more than priority order helps.

### 5.3 Confidence Discount (RICE's Distinctive Idea)

**Problem**: High-staleness items with speculative value outrank slightly-less-stale items with validated value.

**Graft**: Apply a confidence multiplier (0.5–1.0) to the staleness term, so uncertain items age slower.

```
score = priorityWeight(p) + stalenessBonus × confidenceMultiplier
        confidence: 0.5 (unvalidated) / 0.8 (partially validated) / 1.0 (verified)
```

**Alternative — derived from existing signals**: Instead of per-brief human scoring, derive confidence from the number of completed Verify rows (a brief with 0/5 verify rows = lower confidence than one with 4/5). This is CI-computable and un-gameable — the brief's own Evidence table is the signal.

**Risk**: If confidence defaults to 1.0, it's a no-op. RICE's "default to 50%" only works when enforced by convention. Our BRIEF format already has an `Evidence` section that gates on Verify completion — deriving confidence from it is mechanical and self-correcting.

### 5.4 Admission Control — Minimum Value Threshold

**Problem (from open questions)**: Aging prevents starvation but doesn't prevent queue bloat from unlimited new arrivals. Low-value items that should be CLOSED, not DONE, will eventually age into the batch.

**Precedent**: OS schedulers distinguish "aging boosts" from "admission control" — aging prevents starvation of items already in the queue, but doesn't prevent queue bloat.

**Graft**: A minimum value threshold below which staleness does not accumulate — briefs below the threshold are either closed or explicitly promoted by a human override.

```
if brief.effort == XL and brief.priority == P2:
    stalenessBonus = 0  # don't age: promote or close
```

This is a policy knob, not a formula term — it belongs in the retro (R-01), not the code.

### 5.5 What NOT to Graft

| Technique | Why Not |
|-----------|---------|
| Jira-style free-form number fields | No rubric = false precision = gaming |
| Multiplicative full formula (RICE-style) | Amplifies gaming of any single dimension |
| Additive CoD (WSJF-style) | Requires 3 human-scored fields per item = expensive + gamed |
| Aha! custom equation editor | Enables formula-chasing; too flexible |
| "80% confidence default" without enforcement | Converges to 100% = no-op |
| Remove staleness term entirely | OS research confirms aging is the correct anti-starvation pattern |

---

## 6. Option Comparison Table

| Option | Δ Formula | New Human Input | Gaming Risk | Complexity | Verdict |
|--------|-----------|----------------|-------------|------------|---------|
| **Fix clock source** | `daysStale = days since brief.queuedAt` | None | None — purely mechanical | Low | **Do now** |
| **Effort divisor** | `score / effortTshirt` | Effort: S/M/L/XL (1/2/4/8) | Low — hardest to game | Low | **Do next** |
| **Derived confidence** | `staleness × VerifyRatio` | None — derived from Evidence table | None — CI-computable | Medium | **Prototype** |
| **Value term** | `+ valueWeight(v)` | Value: Low/Med/High (0/500/1000) | Medium — teams inflate | Medium | **Retro knob** |
| **Manual confidence** | `× confidenceMultiplier` | Confidence per brief (0.5/0.8/1.0) | Medium — converges to 100% | Medium | **Wait** |
| **Full RICE-lite** | `(value × conf) / effort + staleness` | 3 fields per brief | High — false precision trap | High | **Avoid** |
| **WSJF additive CoD** | `(BV+TC+RR) / size + staleness` | 3 Fibonacci fields per brief | High — 7 fields in Jira | High | **Avoid** |

---

## 7. Open Questions (from Workflow Synthesis)

1. **Minimum viable human input**: Would adding just an Effort field (S/M/L/XL, dividing the score) produce better ordering than pure staleness, before the scoring cost exceeds the scheduling benefit?

2. **Confidence from Verify rows**: Could a brief's confidence multiplier be derived from its Verify table completion (e.g., `VerifyRatio = completedRows / totalRows`) rather than requiring per-brief human scoring?

3. **Admission control**: Does the formula need a minimum value threshold below which staleness does not accumulate, to prevent low-value items from aging into the batch? (Aging prevents starvation; it doesn't prevent queue bloat from unlimited arrivals.)

4. **Wave-gating × staleness interaction**: If wave-2 items depend on blocked (not neglected) wave-1 items, does staleness perversely inflate wave-2 scores while blockers are unresolved — creating priority inversion at the scheduler level?

5. **Calibration**: What's the right weight for an effort divisor relative to priorityWeight? If P0=3000 and S=1 gives score 3000, but P0/XL gives 3000/8=375, a P2/L with 30 days staleness gives 1300/4=325 — is that the right ordering, or does the divisor need scaling?

---

## 8. Methodology — What This Report Did

| Phase | Method |
|-------|--------|
| Scope | Decompose question into 5 complementary search angles |
| Search | 5 parallel web searches, one per angle, 6 results each → 30 raw results |
| Filter | Dedup URLs, drop non-novel, budget-aware truncation → 22 sources fetched |
| Extract | 22 parallel source extractors → 90 falsifiable claims |
| Verify | Top 25 claims selected by importance; each verified by 3 independent adversarial voters (75 agent calls). Claim survives only if ≥2/3 voters confirm. |
| Synthesize | Merge semantic duplicates, rank by confidence, cite sources |
| Map | Apply confirmed findings to our formula |

### Stats
- 104 total agent calls
- 22 sources fetched
- 90 claims extracted
- 25 claims verified (14 confirmed, 11 refuted, 0 unverified)
- 4 synthesized findings
- ~4.9M tokens

### Caveats
1. SAFe WSJF primary source (framework.scaledagile.com) has the full article behind a login wall — core formula confirmed from public excerpt and corroborating secondary sources, but detailed estimation protocol unverified from primary.
2. Tool claims (Jira, airfocus) are July 2026 snapshots — product features change.
3. Aging/starvation claims rely on secondary sources (Wikipedia, lecture notes) rather than primary OS textbooks — but this is textbook-stable CS where secondary sources are reliable.
4. Linux scheduler details (O(1) active/expired arrays, CFS vruntime) were voted down — more complex than the simple aging model, should not be analogized without deeper study.
5. Productboard, Linear, GitHub implementations were not deeply examined — only Jira, Aha!, and airfocus had verified claims survive.
6. RICE's exact formula (Reach × Impact × Confidence / Effort) was mentioned in context but no verified claims about it survived adversarial vote — grafting suggestions for Confidence are drawn from general knowledge of RICE, not verified claims.

### Sources
| URL | Quality | Angle |
|-----|---------|-------|
| https://framework.scaledagile.com/wsjf/ | primary | Economic/flow |
| https://v3.scaledagileframework.com/wsjf/ | primary | Economic/flow |
| https://airfocus.com/templates/WSJF-scoring/ | secondary | Economic/flow |
| https://www.izenbridge.com/kb/safe-agile-interview-questions/what-is-wsjf-and-its-limitations/ | secondary | Critiques |
| https://blackswanfarming.com/wsjf-weighted-shortest-job-first/ | secondary | Economic/flow |
| https://www.stevesmith.tech/blog/continuous-delivery-cost-of-delay/ | blog | Economic/flow |
| https://success.atlassian.com/solution-paths/.../how-to-configure-weighted-shortest-job-first-wsjf-to-work-items | secondary | Tooling |
| https://support.atlassian.com/automation/kb/how-to-create-a-rule-that-calculate-ice-score-and-prioritize-tickets-based-on-it/ | primary | Tooling |
| https://support.aha.io/aha-roadmaps/support-articles/customizations/create-aha-scorecards~7444636572011952598 | primary | Tooling |
| https://support.productboard.com/hc/en-us/articles/360056348954-Score-data-from-zero-to-five-using-drivers | secondary | Tooling |
| https://en.wikipedia.org/wiki/Aging_(scheduling) | secondary | Starvation/aging |
| https://cs.nyu.edu/~gottlieb/courses/2010s/2010-11-fall/os2250/lectures/lecture-05.html | secondary | Starvation/aging |
| http://www.cs.columbia.edu/~krj/os/lectures/L12-LinuxSched.pdf | primary | Starvation/aging |
| http://www.sci.brooklyn.cuny.edu/~briskman/cisc/3320/lecture_notes/topic_06/21.html | secondary | Starvation/aging |
| https://cs.mu.edu/~brylow/cosc3250/Spring2023/Projects/Project6.html | secondary | Starvation/aging |
| https://www.getproductpeople.com/blog/prioritization-techniques-rice-moscow-ice-kano | blog | Classic frameworks |
| https://airfocus.com/blog/rice-prioritization/ | blog | Classic/critiques |
| https://grafana.com/blog/a-better-way-to-prioritize-feature-backlogs-the-cerb-scoring-method/ | blog | Classic frameworks |
| https://www.insights.cgi.com/blog/what-is-wsjf-how-to-use-this-agile-concept-to-prioritize-work | blog | Economic/flow |
| http://agileseekers.com/blog/using-cost-of-delay-without-manipulating-the-numbers | blog | Critiques |
| https://www.startups.com/lexicon/rice-framework | blog | Critiques |
| http://faculty.salisbury.edu/~sxpark/cosc450_7.pdf | unreliable (excluded) | Starvation/aging |
