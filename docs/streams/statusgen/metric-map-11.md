# statusgen/11 — metric map: DevLake | ours | dropped

> **Charter (Ian, 2026-08-12): hybrid, NOT deletion.** Apache DevLake serves the
> commodity engineering metrics any GitHub org gets off the shelf; the metrics only
> this methodology can express stay ours, first-class; a residue that is neither
> commodity-servable nor methodology-bearing is dropped. This page is the one-page
> mapping the operator reviews before the code change — every metric statusgen's
> (now-split) commodity files and the consolidated metrics-harvest emit today gets
> exactly **one** home, and no metric is mapped twice.

Companion deliverable: the DevLake deployment spec + runbook that stands up the
`DevLake` column is staged (not applied) at
[`./devlake/`](./devlake/README.md) — see [Deployment](#deployment-staged-not-applied) below.

## How to read the three homes

| Home | Meaning |
|---|---|
| **DevLake** | A commodity metric Apache DevLake computes from ingested GitHub events (commits, PRs, PR reviews, issues, deployments). Served by DevLake after the staged deployment is applied platform-side; the statusgen standalone surface for it is removed. |
| **ours** | A methodology metric derived from this project's brief / Evidence / historian semantics (statuses like `implemented`/`verified`, the `‡` did-not-run marker, gate states). DevLake has **no model** of these, so they are **retained** in statusgen, first-class. |
| **dropped** | Neither commodity-servable by DevLake nor a methodology metric — a residue retired with its split-out file, or a re-derivable value that was never worth storing. Nothing in the retained methodology set is dropped (that would be a NEEDS_CONTEXT stop, not a judgment call — Ground rules). |

## The map

Grounded in the shipped code as of this branch: the removed commodity files
`dora.go` (standalone `--dora`), `codeefficiency.go` (`--code`), `trend.go`
(`--trend`); the consolidated reducer `tools/metrics-harvest`; and the retained
methodology surfaces `bottleneck.go`, `methmetrics.go`, `unrun.go`, `briefflow.go`,
`awaitage.go`, `roadmapdora.go`.

| # | Metric | Emitted by (today) | Home | Reason |
|---|--------|--------------------|------|--------|
| 1 | Change lead time (`change_lead_time`) | `dora.go` `--dora` / `--dora-timing` | **DevLake** | DORA core; PR create→merge (→deploy) over ingested GitHub events. |
| 2 | Deployment frequency (`deployment_frequency`) | `dora.go` `--dora` | **DevLake** | DORA core; deploy/merge cadence from GitHub events. |
| 3 | Change failure rate (`change_failure_rate`) | `dora.go` `--dora` | **DevLake** | DORA core; failed-change share, off-the-shelf. |
| 4 | Failed-deploy recovery / time-to-restore (`failed_deploy_recovery_time`) | `dora.go` `--dora` / `--dora-timing` | **DevLake** | DORA core (MTTR); incident→restore from GitHub events. |
| 5 | Rework rate (`rework_rate`) | `dora.go` `--dora` (#766) | **DevLake** | Derivable from PR reviews DevLake ingests (CHANGES_REQUESTED + re-review cycles per merged PR). |
| 6 | PR throughput / commodity velocity | `trend.go` `--trend` throughput signal | **DevLake** | PR/commit throughput over GitHub events is exactly DevLake's off-the-shelf velocity. |
| 7 | SLOC delta/day (added / removed / net) | `codeefficiency.go` `--code` | **DevLake** | Commit additions/deletions per day; native to DevLake's commit domain. |
| 8 | Defect density (bug issues ÷ KSLOC) | `codeefficiency.go` `--code` | **DevLake** | Bug-typed issue count ÷ code volume; DevLake quality/DORA. |
| 9 | Review depth (review comments ÷ merged PR) | `codeefficiency.go` `--code` | **DevLake** | PR review/comment counts per merged PR; native to DevLake's PR-review domain. |
| 10 | Open PRs by state (`prsOpenByState`: draft/ready) | `tools/metrics-harvest` | **DevLake** | Open-PR-by-state is a point-in-time GitHub-native count DevLake tracks. |
| 11 | Open issues by label (`issuesOpenByLabel`) | `tools/metrics-harvest` | **DevLake** | Open-issue-by-label counts; native to DevLake's issue domain. |
| 12 | Open issues unlabeled (`issuesOpenUnlabeled`) | `tools/metrics-harvest` | **DevLake** | Same GitHub-native issue domain as row 11. |
| 13 | Open PRs by review decision (`prsOpenByReviewDecision`) | `tools/metrics-harvest` (planned; emitted `null` today) | **DevLake** | Review-decision is a GitHub-native PR field DevLake ingests; never built out here. |
| 14 | Factory-floor bottleneck: per-stage WIP × dwell, constraint, shift, action | `bottleneck.go` `--bottleneck` | **ours** | WIP × median-dwell over the historian's lifecycle stages — a methodology view of brief flow DevLake has no model of. |
| 15 | Awaiting-verification backlog / verification-debt curve | `methmetrics.go` `--verif-backlog` | **ours** | Standing count of briefs at `implemented`/`verified` (merged-not-done) over time — this methodology's verification-gate states; the retained half of the former trend.go rollup (see the trend.go trap). |
| 16 | `‡`-derived run-coverage counts (the unrun set) | `unrun.go` | **ours** | Coverage derived from verifiers' explicit "did not run" (`‡`) markers in Evidence — a brief/Evidence semantic DevLake cannot see. |
| 17 | Issue-loop net-delta / per-stream net-flow + stall flag | `briefflow.go` `--net-flow` | **ours** | Historian arrivals − completions per stream, plus a live stall flag — computed from brief transitions, not GitHub events. |
| 18 | Gate-queue age (awaiting-queue gate scores) | `awaitage.go` / `--gate-scores` | **ours** | Age of briefs sitting at a methodology gate (awaiting verification/review) — a gate-state metric with no DevLake analogue. |
| 19 | Grouped-DORA core (`computeDoraGrouped`) — the roadmap DORA tiles | `roadmapdora.go` (retained), consumed by `roadmap.go` / `roadmap_streampage.go` | **ours** | Retained as the in-Go source the internal roadmap pages render; per the settled direction (Ian, 2026-08-18) DevLake **feeds INTO** these pages, not the reverse — so the tile source stays ours. |
| 20 | Status-funnel view (briefs-per-status over time) | `trend.go` `--trend` funnel signal | **dropped** | A methodology-state funnel DevLake cannot reproduce, retired with `trend.go`; the one funnel signal worth keeping (the verification-debt curve) is retained as row 15. |
| 21 | Churn ratio (lines re-touched within N days ÷ lines added) | `codeefficiency.go` `--code` | **dropped** | No clean DevLake model and not a methodology metric; retired with `codeefficiency.go`. |
| 22 | Change spread (median files per change) | `codeefficiency.go` `--code` | **dropped** | No stock DevLake metric; not methodology-bearing; retired with `codeefficiency.go`. |
| 23 | Commits-to-main nightly snapshot | `tools/metrics-harvest` (deliberately out of scope) | **dropped** | Re-derivable from the API at any later date — snapshotting it into a nightly commit is storage, not measurement (aggregate.go). |
| 24 | Merged-PR throughput / lead-time nightly snapshot | `tools/metrics-harvest` (deliberately out of scope) | **dropped** | Re-derivable queryable data; not snapshotted by the harvest. The commodity metric itself is served live by DevLake (rows 1, 6). |

**Count check (Verify row 1):** 24 metrics classified — 13 `DevLake`, 6 `ours`,
5 `dropped` — well over the ≥ 10 bar, every commodity-file and harvest metric mapped, none twice.

## The retained set is fixed (no metric silently dropped)

The **ours** rows (14–19) are the operator-fixed retained set. Moving any of them to
`dropped` is a NEEDS_CONTEXT stop, never a judgment call (brief Ground rules). Two
guards this map records:

- **The `trend.go` trap.** `trend.go` was on the OUT list but carried the
  awaiting-verification backlog curve — a *retained* methodology metric. It was
  rehomed into `methmetrics.go` (`--verif-backlog`, row 15) **before** the file was
  deleted, so a straight file deletion could not silently drop it (the anti-hybrid
  failure). Its commodity siblings — the status funnel (row 20) and commodity
  throughput (row 6) — went to `dropped`/`DevLake` respectively.
- **The grouped-DORA core stays ours.** `dora.go`'s standalone commodity `--dora`
  surface is removed, but `computeDoraGrouped` + its `DoraGroup`/`DoraGroupedReport`
  types were rehomed to `roadmapdora.go` (row 19), because the internal roadmap
  pages consume it and DevLake feeds INTO those pages (settled direction). Removing
  the standalone CLI is a surface removal, not a metric drop.

## Deployment (staged, not applied)

The `DevLake` column above is served by a self-hosted Apache DevLake (Apache-2.0)
targeting the platform k8s cluster. That deployment is **staged, not deployed** by
this brief: the helm values / manifests, the GitHub connection + scope config, and
the human/platform runbook live at [`./devlake/`](./devlake/README.md). Standing it
up — GitHub App/PAT provisioning, `helm`/`kubectl apply`, first ingest — is a
human/platform act (a cross-repo follow-up), not run here. Until then the `DevLake`
rows describe the intended home, not a running system.
