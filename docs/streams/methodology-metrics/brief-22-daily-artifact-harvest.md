---
brief: methodology-metrics/22
title: 'Daily artifact harvest — a scheduled, AI-free collector commits the day''s metrics/board artifacts for later analysis'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-12: 'we have some daily jobs that need to be run… at a minimum we should be creating a pod or something to generate the artifacts (all the stuff we can do without a AI) so that they can be analyzed later'", "methodology-metrics/02 (--dora, verified — first harvest input)", "methodology-metrics/03 (--trend, verified — second harvest input)", "methodology-metrics/18 (daily bottleneck report, todo — its output joins the harvest when --bottleneck lands)", "methodology-metrics/16 (--dora --series, todo — same)", "docs/reports/factory-floor/2026-07-10.md (the one manual instance this mechanizes)", "freshness-checked 2026-07-12 @ post-#364 main: .github/workflows/ has zero schedule: triggers — every 'daily' job today is human-remembered"]
why: >-
  Every process-side "daily" job (bottleneck report, DORA tables, board snapshots, review-state
  counts) currently runs only when human:<name> or a desk remembers to ask — the generation is fully
  deterministic, no AI required, yet it's gated on a human opening a window. Splitting
  generation from analysis means the artifacts exist EVERY day (including days no session
  opens), day-over-day comparisons stop depending on who asked what when, and AI sessions
  become pure consumers of a consistent, dated record instead of re-deriving it live each time.
---

# Brief 22 — Daily artifact harvest (AI-free collector)

## Context

files: .github/workflows/daily-harvest.yml (future path) (new), `../assay-toolkit/statusgen/` (only if a `--json`
output mode is needed; no metric logic changes), docs/reports/daily/ (future path) (new artifact home).

facts:
- Cluster-side dailies are already solved (k8s CronJobs `fleet-health-check`/`image-refresh`/
  `log-triage`, Flux-managed). This brief is the REPO/process-side collector only.
- `.github/workflows/` has no `schedule:` trigger today; main-writes by CI use the
  `[skip-status-regen]` commit-marker pattern (see `status-regen.yml`) — the harvester must
  use the same marker so it never recurses the regen loop.
- Scheduled workflows run on GitHub's clock (UTC). Lightweight checks stay `ubuntu-latest`
  per repo rule; the harvester is lightweight (go run + gh reads) → `ubuntu-latest`.
- Working flags today: `--dora`, `--trend`, `--lint` (NOTICE stream), plain regen (Next-up,
  Awaiting queue). `--bottleneck` (mm/18), `--series` (mm/16), `--code` (mm/19) are todo —
  the harvester grows a line each when they land (those briefs gain a one-line "wire into
  daily-harvest" DoD item via their own flow; NOT this brief's scope).
- consumers: the-desk boot sequence (reads latest harvest instead of regenerating live),
  mm/18's day-over-day shift computation (needs yesterday's file — this brief is what
  guarantees yesterday exists), the retro loop (R-NN evidence), the exec/article metrics
  (methodology/34). Enumerated per author-brief rule 6; none change behavior until they
  opt in — the harvest is a new read-only surface.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. (The workflow's OWN scheduled runs are the exception by design —
  it commits its artifacts; that is the deliverable.)
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Collector script/cmd (Go-preferred per repo rule)**: gather, into
   `docs/reports/daily/<YYYY-MM-DD>/`:
   - `dora.txt` — `statusgen --dora` output; `trend.txt` — `statusgen --trend`;
     `notices.txt` — the NOTICE stream from `--lint` (exit code recorded, never gating);
     `board.md` — the regenerated Next-up + Awaiting sections (scratch regen, NOT committed
     to STATUS.md).
   - `prs.json` / `issues.json` — `gh` snapshots: open PRs (number, title, draft, review
     decision, head, age), open issues (number, title, labels, age). Machine-readable JSON —
     the "analyzed later by AI" contract.
   - `git.txt` — commits-to-main count + author split for the day (deterministic ledger read).
2. **Scheduled workflow** .github/workflows/daily-harvest.yml (future path): `schedule:` daily (pick an
   off-peak UTC hour) + `workflow_dispatch` for the human-triggered first run. Commits the
   dated directory to main with message `chore(harvest): <date> [skip-status-regen]`, with a
   retry-on-race push loop (same shape as status-regen's).
3. **Retention/idempotency**: re-running a day overwrites that day's directory (idempotent);
   no pruning in v1 (small text artifacts; revisit at retro if the dir gets heavy).
4. **Consumption note**: one line in the-desk skill's boot sequence is NOT edited here
   (out-of-repo, #221) — instead add a pointer line to `docs/streams/methodology-metrics/README.md`
   conventions: "daily artifacts land in docs/reports/daily/; desks read, never regenerate".

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `yq eval '.' .github/workflows/daily-harvest.yml > /dev/null` | exit 0 |
| 2 | `yq eval '.on.schedule[0].cron' .github/workflows/daily-harvest.yml` | a valid 5-field cron line (daily) |
| 3 | `grep -c "skip-status-regen" .github/workflows/daily-harvest.yml` | ≥ 1 |
| 4 | human (or verifier) triggers one run via `workflow_dispatch`; then `command ls docs/reports/daily/ \| tail -1` | a `YYYY-MM-DD` directory exists on main |
| 5 | `jq -e 'type=="array"' docs/reports/daily/<that-date>/prs.json` | exit 0 |
| 6 | `statusgen --root . --lint` | exit 0 (harvest artifacts trip no register/lint rule) |

## Evidence
<!-- appended at implementation time -->

### Non-implementer verifier run — VERIFY: FAIL (glm-5.2-verifier, merged main `3d3708ad`, 2026-07-16)

Rows 1–3, 6 PASS (YAML parses, cron `0 6 * * *`, `skip-status-regen` present, statusgen lint clean).
**Rows 4–5 FAIL**: the deliverable — a dated `docs/reports/daily/<YYYY-MM-DD>/` dir on main — has
**never** been produced (`docs/reports/daily/` does not exist on main). The scheduled run
2026-07-16 08:19:58Z (run `29483086565`) FAILED at the `harvest` job (`capturePRs failed`,
`captureIssues failed`, exit 1). The collector is locally correct — a local run with a valid
`gh auth token` produced all 7 artifacts + valid `prs.json`/`issues.json` arrays — so the failure is
CI-side (suspected `GITHUB_TOKEN` scoping in `daily-harvest.yml`). Brief stays `implemented`.
**Change Failure — bug: #575.**

### Non-implementer re-verify — VERIFY: PASS (glm-5.2-verifier, in-repo main `2f1b9ff5`, 2026-07-18)

The 2026-07-16 FAIL (#575) is **resolved**. `../oit/.github/workflows/daily-harvest.yml` now carries an `Ensure gh CLI
is available` step (pinned `gh` binary on self-hosted runners — the prior failure cause, since medici-builder
runners don't ship `gh`), `runs-on: medici-builder`, and a race-safe retry push loop mirroring `status-regen.yml`.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `yq eval '.' .github/workflows/daily-harvest.yml > /dev/null` | 0 | YAML parses |
| 2 | `yq eval '.on.schedule[0].cron' …` | 0 | `0 6 * * *` (daily 06:00 UTC) |
| 3 | `grep -c "skip-status-regen" …` | 0 | 3 (≥1) |
| 4 | `ls docs/reports/daily/ \| tail -1` | 0 | `2026-07-17`; 3 dated dirs (07-15/16/17), each with all 7 artifacts (board.md, dora.txt, git.txt, issues.json, notices.txt, prs.json, trend.txt) |
| 5 | `jq -e 'type=="array"' docs/reports/daily/<date>/prs.json` | 0 | each `prs.json`/`issues.json` is a JSON array of objects |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

All 6 rows PASS. `gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
