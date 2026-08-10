---
brief: methodology-metrics/05
title: FINDINGS alarm KPIs — rate, standing-alarm age, flood detection (ISA-18.2)
wave: 1
depends: ["methodology-metrics/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA — alarm KPIs per ISA-18.2)", "docs/streams/methodology/scada-ooda-lineage.md", "methodology-metrics/01 (historian)", "docs/streams/FINDINGS.md"]
---

# Brief 05 — FINDINGS alarm KPIs

## Context
files: `../assay-toolkit/statusgen/` (new `--alarms` view or fold into `--trend`), reads docs/streams/FINDINGS.md
+ `methodology-metrics/01`'s history (for age/rate over time).
facts:
- FINDINGS entries are the methodology's **alarms** ([I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md)): each flags briefs ⚠ stale until resolved.
  Process-control alarm management (ISA-18.2) defines the KPIs that keep an alarm system healthy:
  **alarm rate** (new/period), **standing-alarm age** (how long an unresolved finding has stood), and
  **flood detection** (too many active alarms at once = operator overload / the system is drowning).
- Today FINDINGS has no KPIs — a finding can stand for weeks unnoticed (RETRO's "FINDINGS age > 1 retro"
  check is manual). These KPIs make the alarm state measurable.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Compute from FINDINGS + the 01 history: **alarm rate** (findings opened per week), **standing-alarm
   age** (per unresolved finding: now − opened; flag any over a threshold, default 1 retro-cycle),
   **flood** (active-unresolved count over a threshold, default configurable).
2. Emit as a `statusgen --alarms` view and a `--lint` NOTICE for standing alarms past the age threshold
   (so the desk/retro sees them without manual scanning). Feed the numbers into `--trend` (03) if landed.
3. Thresholds are configurable + documented (no magic numbers buried in code).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Alarm` | exit 0 (rate/age/flood tests over testdata FINDINGS + history) |
| 2 | testdata: a finding opened 30 days ago, unresolved → `statusgen --alarms` | lists it as a standing alarm past the age threshold |
| 3 | testdata: N active findings over the flood threshold → `--alarms` | reports a flood condition |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` (real tree) | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run Alarm` | 0 | 5 Alarm tests PASS (`TestAlarmStandingAgePastThreshold`, `TestAlarmFloodCondition`) | 2026-07-10 | opus-verifier |
| 2 | testdata 30-day unresolved → `--alarms` | 0 | covered by TestAlarmStandingAgePastThreshold; real-tree `--alarms` renders rate/flood/standing-age KPIs (ISA-18.2 thresholds) | 2026-07-10 | opus-verifier |
| 3 | testdata N over flood → `--alarms` | 0 | covered by TestAlarmFloodCondition | 2026-07-10 | opus-verifier |
| 4 | `go vet && --lint` | 0 | clean | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — FINDINGS alarm KPIs (rate/flood/standing-age) render with ISA-18.2 thresholds.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
