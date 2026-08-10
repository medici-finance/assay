---
brief: methodology/18
title: DORA metrics — instrument outcomes (lead time + stability), not just merge throughput
wave: 1
depends: ["methodology/08", "methodology-metrics/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (velocity read exposed we measure output not outcomes; human:<name> → adopt DORA)
sources: ["https://dora.dev/ (DORA Core, four keys + rework)", "methodology/brief-08 (RETRO)", "verify-desk skill (Change-Failure sensor)", "[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) SCADA/OODA (statusgen --trend historian)"]
---

# Brief 18 — DORA metrics in the retro

## Context
files: `../oit/docs/streams/RETRO.md`, `docs/streams/methodology/brief-08-retro-bootstrap.md` (owns the retro
checklist), `../assay-toolkit/statusgen/` (computes what it can), the `verify-desk` skill (feeds the instability half).
facts:
- The 2026-07-09 velocity read exposed the gap: we measure **output** (35 PRs merged / 69 commits in a day
  = elite *deployment frequency*) while the **outcome** — lead time to `done` — and ALL stability signals
  are invisible (35 merged, 5 `done`). DORA's central finding: elite teams win on throughput AND stability
  *simultaneously*; optimizing merges alone is half-blind.
- DORA Core is **five metrics** in two families:
  - *Throughput*: **Deployment Frequency**, **Change Lead Time**, **Failed-Deployment Recovery Time**.
  - *Instability*: **Change Failure Rate**, **Deployment Rework Rate**.
- DORA philosophy (load-bearing for adoption): measure **outcomes not output**; use them **as a system**
  (never cherry-pick one); **per application context**, NOT cross-team comparison or individual scorecards;
  beware **Goodhart's Law** — a metric made a target gets gamed. These guardrails are part of the deliverable,
  not decoration.

## Mapping (DORA → this methodology)
| DORA metric | Our definition (measurable) | Source |
|---|---|---|
| Deployment Frequency | brief-PRs merged / commits to main per period | `gh pr list --state merged` / `git log` |
| Change **Lead Time** | age from brief `implemented` (merge) → `done` (verified+reviewed); and commit→merge | brief status timestamps + git |
| Change Failure Rate | (VERIFY: FAIL on merged main + new `bug` issues) ÷ merged briefs | verify-desk + `gh issue list --label bug` |
| Failed-Deploy Recovery Time | time from a broken `main` / bad Flux deploy to green | git + Flux events |
| Deployment Rework Rate | follow-up briefs/bugs spawned by a post-merge defect (e.g. #106 rescue, C2-HTTP, #118) ÷ merged | FINDINGS/INTAKE + issues |

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`. STATUS.md single-writer (no branch commits).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Add the five DORA metrics to the RETRO checklist** (in `RETRO.md`'s format + brief-08's owner text):
   each retro reports all five, with the definitions above, from generated/logged data only (no prose
   estimates). Present them as a *system* (throughput + instability together), never one in isolation.
2. **Instrument what statusgen/gh can compute now** and mark the rest as verify-desk/manual inputs:
   deployment frequency + lead-time (commit→merge) are computable from git/gh today; lead-time
   (implemented→done) needs brief status timestamps (add a dated status-transition log if not present);
   change-failure + rework come from the verify-desk pass + `bug` issues. Wire a first cut (a
   `statusgen --dora` or a retro-input script) that emits the computable ones; the SCADA `--trend`
   historian ([I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md)) is the durable home.
3. **Bake in the anti-gaming guardrails** as explicit retro rules: metrics are per-project context,
   for continuous improvement not targets, never individual/cross-team scorecards; a metric that starts
   driving behavior perversely (e.g. gaming lead time by rubber-stamping verify) is itself a retro finding.
4. **Seed R-01** (the first real retro) with the five metrics computed from this week's actual data as the
   worked example.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | the metric-emitter (`statusgen --dora` or the retro-input script) on real data | prints all 5 metrics (or names the ones needing manual/verify-desk input) with numbers, exit 0 |
| 2 | `grep -ci "deployment frequency\|change lead time\|change failure\|recovery time\|rework rate" docs/streams/RETRO.md` | ≥5 (all five present in the retro checklist) |
| 3 | `grep -ci "goodhart\|not a target\|per-project\|continuous improvement" docs/streams/RETRO.md docs/streams/methodology/brief-08-retro-bootstrap.md` | ≥1 (anti-gaming guardrail recorded) |
| 4 | R-01 entry in RETRO.md | contains the five metrics computed from real data, presented as a system |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go run ./tools/statusgen --dora --root .` | 0 | prints all 5 DORA metrics with numbers — deploy freq 58.82 commits/day (173 PRs), lead time 21.4h median implemented→done, recovery-time + rework `unknown [needs: verify-desk\|manual]`, change-failure 54% partial; anti-gaming note in header | 2026-07-10 | opus-verifier |
| 2 | `grep -ci "deployment frequency\|change lead time\|change failure\|recovery time\|rework rate" docs/streams/RETRO.md` | 0 | 13 (≥5) | 2026-07-10 | opus-verifier |
| 3 | `grep -ci "goodhart\|not a target\|per-project\|continuous improvement" RETRO.md brief-08` | 0 | RETRO.md:3, brief-08:1 (≥1) | 2026-07-10 | opus-verifier |
| 4 | R-01 entry contains 5 metrics from real data, presented as a system | — | R-01 block (RETRO.md:215-241) carries all five DORA metrics from real `--since 2026-07-08` data, read "as a system", `needs:` inputs honestly marked | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — all four rows pass; DORA metrics instrumented in RETRO with anti-gaming guardrails and R-01 worked example.

## Review
Gate: model (methodology docs, all risk no). Reviewer records verdict + date in the stream README.
Note the framing risk explicitly in review: DORA metrics are diagnostic, not a scoreboard — a review that
reads them as targets has missed the point.
