---
brief: metrics-harvest/01
title: Multi-domain harvester — per-domain repo config, cross-org gh/git snapshots, dated artifact tree
wave: 0
depends: []
unblocks: ["metrics-harvest/02"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-20 by Opus session (human:<name> direction via desk)
sources: ["docs/streams/INTAKE.md I-07 (human:<name> direction 2026-07-20)", "oit methodology-metrics/22 (tools/daily-harvest — the single-repo/single-org precedent this generalizes; its fail-loud shape + capture set + [skip-status-regen] marker are reused; its scheduled-run token gate is Change-Failure oit#575)", "docs/desk-console-design.md (Measures pane — the eventual consumer)", "freshness-checked 2026-07-20: all three domains' repos are PRIVATE across two orgs (medici-finance/reconciler + example-org/agent-runtime both PRIVATE); assay-toolkit tools/ already hosts a per-tool Go module (tools/freshness/); .github/workflows/statusgen.yml commits main with the [skip-status-regen] marker; assay-toolkit statusgen exposes --lint/--format only (no --dora/--trend), so this collector is gh/git-only)"]
---

# Brief 01 — Multi-domain harvester

## Context
files:
- `tools/metrics-harvest/` (new, standalone Go module — its own `go.mod`, mirroring
  `tools/freshness/`): `main.go`, `domains.yaml` (the per-domain repo config)
- .github/workflows/metrics-harvest.yml (new scheduled workflow)
- `reports/daily/` (new artifact tree home at repo root)

facts:
- **Config is data**: tools/metrics-harvest/domains.yaml declares `grouping -> [org/repo, ...]`
  for **four groupings** — the three products (`assay`, `plumb`, `lending`) plus `org`
  (org-wide, not a product), exactly as the stream README's grouping table lists them (human:<name>
  rulings 2026-07-20). Adding a repo edits this file, never the Go. Grouping specifics:
  `medici-decks` is in **lending** (so lending spans both orgs); `decks` + `platform-repo`
  are in **org**, ACTIVE; `proposals` ships as a single commented line ("deferred — quiet
  repo") and is NOT collected; the eight legacy `the-org` repos are excluded entirely.
- **Repo-agnostic capture set, per repo per domain** — via the `gh` API (`gh pr list` /
  `gh issue list` with `--repo <org>/<repo>`, and `git`-equivalent history reads):
  - `prs.json` — open PRs: `number,title,isDraft,reviewDecision,headRefName,createdAt`
    (+ derived age in days)
  - `issues.json` — open issues: `number,title,labels,createdAt` (+ derived age in days)
  - `git.txt` — commits-to-`main` count + author split for the day (deterministic ledger read)
  - `throughput.json` — DORA-style throughput **where derivable** from the above (e.g.
    merged-PR count for the day, PR lead time from `createdAt`→merge); emit only fields the
    gh/git data actually supports, never estimated.
- **Artifact tree**: `reports/daily/<YYYY-MM-DD>/<grouping>/<repo>/<artifact>` where
  `<grouping>` is one of `assay`/`plumb`/`lending`/`org`. Default date is
  YESTERDAY (previous complete day) — same rationale as the oit harvester (a 06:00Z run sees a
  6h-old "today"; the prior full day gives complete day-over-day data). Re-running a day
  overwrites it (idempotent); no pruning in v1.
- **Fail-LOUD**: a failed capture still writes its artifact (exit code + stderr on disk) and
  the process exits non-zero so a scheduled run goes red. Collect every artifact first, then
  exit non-zero if any hard capture failed (same shape as tools/daily-harvest/main.go).
- **Auth = per-product + all-covering GitHub Apps (the REAL HEAD — see stream README critical
  path; human:<name> ruling 2026-07-20)**: every product's repos are PRIVATE and span two orgs, so the
  workflow's default `GITHUB_TOKEN` cannot read them. Instead of one god-token, the collector
  uses a **least-privilege App fleet**, tokens injected as CI secrets:
  - `metrics-harvest-assay` — reads ONLY the `assay` repo set; used for the `assay` harvest.
  - `metrics-harvest-plumb` — reads ONLY the `plumb` repo set; used for the `plumb` harvest.
  - `metrics-harvest-lending` — reads ONLY the `lending` repo set (incl. `medici-decks`); used
    for the `lending` harvest.
  - `metrics-harvest-all` — reads ALL product repos + the `org`-wide repos; used for the `org`
    harvest here and by the aggregator (brief 02).
  Each App's minimal permission set is **`contents:read`, `issues:read`, `pull_requests:read`**
  (metadata:read implied). The collector selects the App token **per grouping** (a
  grouping→secret map in config/env), so a per-product run carries only its product's
  privilege. **App creation + installation is an human:<name> act, external to this brief** — build the
  collector to read the correct token per grouping from env / `gh` auth and fail LOUD when it
  lacks read for a repo; do NOT create, install, or commit any App/secret here.
- **Non-recursion**: scheduled commits use `chore(harvest): <date> [skip-status-regen]` so
  they never trip `statusgen.yml`'s regen loop; push with a retry-on-race loop (same shape as
  `status-regen`/`statusgen.yml`).
- consumers (shared values introduced here — the `domains.yaml` schema AND the
  `reports/daily/<date>/<domain>/<repo>/` tree layout): `metrics-harvest/02` (the aggregator
  reads this exact tree — fixed-here by defining it); `desk-console/05` (metrics CronJob — may
  schedule this collector; follow-up, cross-stream); `desk-console/12` Measures pane (renders
  the aggregator's output; follow-up, cross-stream). Any consumer that hard-codes the tree
  layout or the config schema is a stranded assumption — they are defined once, here.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions. (The workflow's OWN scheduled runs are the by-design exception — committing
  its artifacts IS the deliverable.)
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess. The
  groupings are RULED (human:<name> 2026-07-20) — use them as-is; do not re-litigate or add repos not
  in the README grouping table.

## Task
1. **tools/metrics-harvest/domains.yaml**: the four groupings (`assay`/`plumb`/`lending`/`org`)
   from the stream README's grouping table; `proposals` as a single commented "deferred —
   quiet repo" line.
2. **Collector (`tools/metrics-harvest/`, Go, standalone module)**: for each repo in each
   grouping in the config, write the capture set (`prs.json`, `issues.json`, `git.txt`,
   `throughput.json`) under `reports/daily/<date>/<grouping>/<repo>/`. AI-free, fail-loud,
   idempotent per day. `--date`, `--config`, and a grouping selector (e.g. `--grouping assay`
   for the per-product runs) flags; default date = yesterday. Selects the App token **per
   grouping** (product App for a product; `metrics-harvest-all` for `org`); a repo that returns
   403/404 for lack of read is a LOUD failure recorded on disk, not a silent skip.
3. **.github/workflows/metrics-harvest.yml**: `schedule:` daily at an off-peak UTC hour +
   `workflow_dispatch`; injects each App's token secret and runs the collector per grouping
   with the matching App; commits the dated tree to main with
   `chore(harvest): <date> [skip-status-regen]` via a retry-on-race push loop.
   `runs-on: ubuntu-latest` (lightweight).
4. **Discoverability**: link the tool + artifact tree from the README docs/ index; add a
   tools/metrics-harvest/README.md (what it captures, the config, the per-product/all-covering
   App auth model + minimal scopes, how to run locally).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/metrics-harvest && go build ./...` | exit 0 |
| 2 | `yq eval '.' tools/metrics-harvest/domains.yaml > /dev/null` | exit 0 (valid YAML) |
| 3 | `yq eval '.assay \| length, .plumb \| length, .lending \| length, .org \| length' tools/metrics-harvest/domains.yaml` | `4`, `3`, `4`, `2` (active repos per grouping — lending incl. `medici-decks`, `org` = `decks`+`platform-repo`; `proposals` is a comment, not an entry) |
| 4 | `yq eval '.' .github/workflows/metrics-harvest.yml > /dev/null && yq eval '.on.schedule[0].cron' .github/workflows/metrics-harvest.yml` | exit 0; a valid daily 5-field cron |
| 5 | `grep -c 'skip-status-regen' .github/workflows/metrics-harvest.yml` | ≥ 1 |
| 6 | local run against a multi-org `gh` token: `cd tools/metrics-harvest && go run . --date <a past date>` then `ls reports/daily/<that-date>/lending/oit/` | contains `prs.json issues.json git.txt throughput.json` |
| 7 | `jq -e 'type=="array"' reports/daily/<that-date>/lending/oit/prs.json` | exit 0 (valid JSON array — the "analyzed later" contract) |
| 8 | FLOW (App fleet reads its scope): local run of the `lending` grouping (which alone spans BOTH orgs — `example-org/*` + `medici-finance/medici-decks`); `find reports/daily/<that-date>/lending -name prs.json \| xargs -I{} jq -e 'type=="array"' {}` | exit 0 for the the-org AND the medici-finance repo — proves the product App reads its full cross-org set, the real head |
| 9 | `cd statusgen && go run . --root .. --lint` | exit 0 (harvest artifacts trip no register/lint rule) |

Row 8 is the flow-level check for the shared value (the cross-org token): a local pass
proves the collector; the SCHEDULED run's green is gated on the external secret and is the
verifier's/human:<name>'s step, recorded in Evidence when the token lands.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
