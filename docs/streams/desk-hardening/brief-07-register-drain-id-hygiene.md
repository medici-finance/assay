---
brief: desk-hardening/07
title: Register-drain loop + ID hygiene (findings drain, resolved-vocabulary, sequential-ID collisions)
why: >
  Every register with a dedicated draining window stays current (intake→intake-desk, PRs→
  pr-review-desk, Awaiting→verify-desk). The findings register has distributed filers and
  mechanical consumers but NO standing drainer, so 7 genuinely-stale findings sat resolved:false
  for up to two weeks — suppressing board rows the whole time — and a resolved:yes vs resolved:true
  frontmatter variance made register-wide tallies wrong. Separately, every allocate-next-sequential-
  ID convention breaks under parallel authorship (F-NN, I-NN, and now brief-NN have all collided),
  because merged-main is the wrong read surface. This gives the findings register an owning loop,
  pins one boolean vocabulary, and kills the sequential-ID collision class.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [117, 56, 118]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#117 (findings register has filers and consumers but no draining loop)"
  - "assay-toolkit#56 (same-wave dependencies break strict wave-layering — 02b depends on 02a, both wave 1)"
  - "assay-toolkit#118 (sequential register IDs collide across parallel work — F-NN, I-NN, now brief-NN)"
exec-tier: strong
exec-tier-why: "touches statusgen lint/critical-path logic (same-wave deps, ID-collision, resolved-vocabulary tallies) — a cross-artifact change where a wrong rule mis-schedules the whole board."
consumers:
  - "[toolkit] statusgen (parse/registers/idvalidate/tiering): fixed-here (resolved-vocabulary pin, same-wave-dep policy, ID-collision lint)"
  - "[oit] verify-desk SKILL.md: follow-up (candidate owner of the findings drain loop)"
  - "[toolkit/oit] a CI check needing gh at lint time: follow-up (brief-NN duplicate-across-open-PRs — likely CI-side, not statusgen core)"
---

# Brief 07 — Register-drain loop + ID hygiene

## Context
files:
- `[toolkit]` `statusgen/` (registers.go / parse.go / idvalidate.go / tiering.go — the resolved-vocabulary, same-wave, and ID-collision logic)
- `[toolkit]` docs/streams/FINDINGS.md header (pin the `resolved:` vocabulary), `docs/brief-rules.md` (same-wave-dep policy, next-free-ID guidance)
- `[oit]` `../oit/.claude/skills/verify-desk/SKILL.md` (candidate owner of the drain loop)
- `[toolkit/oit]` a CI check (`.github/workflows/`) for the brief-NN-across-open-PRs case (needs gh)
out-of-repo files: none
facts:
- #117: intake→intake-desk (3 untriaged of 88), PRs→pr-review-desk, Awaiting→verify-desk all stay current *because each has a standing drainer*; findings has filers (workers, verify-desk, intake exit) + consumers (statusgen ⚠-flags, batch-fanout exclusions) but no loop; the drain has only ever happened as an ad-hoc coordinator fan-out
- the tally bug: `resolved: yes` vs `resolved: true` variance (24 files) made register-wide counts wrong — the coordinator reported "45 unresolved" when the true open set was 21; the schema must pin ONE boolean vocabulary
- #56: 02b declared wave:1 while depending on 02a (also wave:1) — wave numbers no longer strictly layer; if the critical-path computation assumes deps point only to earlier waves, a same-wave dep miscomputes ordering
- #118: a brief was numbered 14 from next-free on merged main while open PR #880 already claimed 14; F-NN/I-NN collided 3x in one session; the per-entry-file (slug identity) migration fixed findings/intake — brief numbers are the remaining sequential holdout; the free set must be computed across merged state PLUS open PRs, or drop sequence entirely (slug/date identity)

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Findings-register drain loop (#117).** Give the findings register an owning loop with a
   cadence. **Candidate approaches:** (a) fold into **verify-desk** (it already generates findings
   and runs evidence discipline) — recommended by the finding; (b) a standing findings-desk
   window; (c) a scheduled statusgen job that flags stale `resolved:false` findings whose
   invalidating condition is fixed. Record the choice; wire it so a resolved finding stops
   suppressing its board row promptly.
   **Decision:** findings drain folded into verify-desk (oit repository SKILL.md (verify-desk, follow-up)
   follow-up). The statusgen `standingAlarmNotices` (alarms.go) already handles mechanical
   surfacing of stale unresolved findings; the verify-desk loop resolves them on its cadence.

2. **Pin the `resolved:` vocabulary (implemented).** Chose canonical `Resolved: yes`/`Resolved: no`
   for the rendered view. Both `yes`/`no` and `true`/`false` are accepted for backward
   compatibility. Documented in docs/streams/FINDINGS.md header. Both `yes`/`no` and
   `true`/`false` are normalized in `load.go` (`firstWord == "yes" || firstWord == "true"`)
   so tallies stay mechanical.

3. **Same-wave-dependency policy (#56) (implemented).** Decided: forbid same-wave deps.
   Recorded in `docs/brief-rules.md` rule 4. `statusgen --lint` (`checkBriefFiles` in
   `brieffile.go`) flags any dep whose wave >= the brief's own wave.

4. **Sequential-ID collision (#118) (implemented).** Chose (a) authoring guidance +
   (b) CI check now, with (c) slug-identified briefs noted as the durable fix.
   Guidance in `docs/brief-rules.md` rules 18-19: check open PRs when assigning a brief
   number. CI check in .github/workflows/statusgen.yml uses `gh pr list` to detect
   brief-NN collisions across open PRs.
## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'resolved: yes\|resolved: no\|one.*vocabulary' docs/streams/FINDINGS.md` | exit 0; ≥ 1 (vocabulary pinned) |
| 2 | craft two findings, one `resolved: yes` one `resolved: true`; run the tally/lint | the variance is flagged or normalized (positive control — the tally no longer silently miscounts) |
| 3 | `grep -ci 'same-wave\|strictly-earlier\|forbid.*same wave' docs/brief-rules.md` | exit 0; ≥ 1 (policy recorded) |
| 4 | add a brief file whose NN duplicates one on an open PR fixture; run the collision check | it FAILS naming the duplicate (positive control) |
| 5 | `cd status* && go run . --root .. --lint; echo $?` | exit 0 |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm Verify rows 2 and 4 actually fire on
the crafted collisions — a lint that passes the very variance/collision it exists to catch is the
dominant defect class again.
