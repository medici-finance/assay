---
brief: desk-apps/05
title: worker App + deskpr/deskreply App-identity cutover
why: >-
  Today every worker action lands as `the-org` — one shared login — so author ≠ approver is
  honor-system and the audit trail can't tell workers apart. Cutting deskpr/deskreply over to
  worker-app[bot] makes GitHub's self-approval block enforce the separation for real, and per-worker
  attribution (later, multiple worker Apps) isolates blast radius.
wave: 2
depends: ["desk-apps/03", "desk-apps/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (worker-app authors PRs; author≠approver GitHub-enforced; per-worker attribution later)", "desk-tools/04 deskpr + desk-tools/07 deskreply (the worker-identity tools to cut over)", "desk-tools/07 deskreply (already posts as the ambient the-org worker — the App voice stays with deskpost)"]
---

# Brief 05 — worker App + deskpr/deskreply cutover

**BLOCKED-ON-HUMAN:** requires the worker App to exist (created by human:<name> via guide 02). Cross-stream:
modifies desk-tools tools (04 deskpr, 07 deskreply).

## Context
files: `../assay-toolkit/tools/desk/cmd/deskpr/` (desk-tools/04, modified); `../assay-toolkit/tools/desk/cmd/deskreply/` (desk-tools/07,
modified); uses `desktoken worker` (brief 03).
facts:
- **deskpr** (desk-tools/04): today pushes the worker's branch + opens/updates the draft PR as the
  ambient `gh` identity (the-org). Cutover option: push + PR as `assay-worker-app[bot]` via
  `desktoken worker`. The branch push (committed code) is the worker's authorship voice.
- **deskreply** (desk-tools/07): posts worker-identity PR replies. Its whole point (per
  desk-tools/07) is "never the App voice" — it posts as the-org today. Cutting it to worker-app[bot]
  is the natural home (it's a worker App, not the reviewer App). The reviewer-App/worker-App
  separation deskreply already enforces is preserved; only the actor upgrades from the-org →
  worker-app[bot].
- **author ≠ approver (the invariant):** worker-app[bot] authors PRs; reviewer-app[bot] reviews them.
  GitHub blocks worker-app from approving its own PR (distinct actors). Today this is honor-system
  (both could be the-org); after cutover it's enforced.
- deskreply's no-App-token-path test (desk-tools/07 Task 3) is updated: worker-app token path is now
  ALLOWED for deskreply (it's a worker tool) — the prohibition was specifically against the
  *reviewer* App voice.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. deskpr: add `--as-app` (default on once worker App exists) → push + PR as worker-app[bot] via
  `desktoken worker`; keep the-org fallback for the transition. deskreply: post as worker-app[bot].
2. Tests: a worker PR's commits/author = worker-app[bot]; reviewer-app can review it; worker-app
  cannot self-approve (GitHub enforces — assert the distinct-actor property holds); deskreply still
  never uses the reviewer-App token path (updated test).

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskpr/... ./tools/desk/cmd/deskreply/... -count=1` | exit 0 |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | **(live, blocked-on-human)** open a worker PR via deskpr; `gh pr view <n> --json author,commits` | author/commits = `assay-worker-app[bot]` |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer. -->

Verified 2026-07-30 by glm-5.2-verifier (non-implementer) against merged main `f4570798`. Landed in two merged PRs: #1204 (2026-07-24, introduced `--as-app` default `false` + deskreply `mintWorkerToken` + `TestNoAppTokenPath`) and #1278 (2026-07-27, flipped the default `false`→`true` + 4 default-pinning tests).

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskpr/... ./tools/desk/cmd/deskreply/... -count=1` | 0 | `ok deskpr 16.972s`, `ok deskreply 7.871s`; the 4 new default tests (`TestCreateDefaultMintsWorkerToken`, `TestUpdateDefaultMintsWorkerToken`, `TestCreateNoAsAppUsesAmbientIdentity`, `TestUpdateNoAsAppUsesAmbientIdentity`) + `TestNoAppTokenPath`/`TestReplyMintsWorkerToken` all PASS. |
| 2 | `go vet ./tools/desk/...` | 0 | clean (no output). |
| 3 | **(live, blocked-on-human)** open worker PR via deskpr; `gh pr view <n> --json author,commits` | UNRUN | non-implementer verifier opens no PRs / mints no creds. Read-only corroboration: `assay-worker-app` (ID `4331284`) installed on the-org + medici-finance (install `147393366`); 3 existing `assay-worker-app[bot]` PRs (#1272, #1273, #1309); `gh pr view 1273` → author `app/assay-worker-app` (`is_bot: true`). Caveat: "commits = assay-worker-app[bot]" holds at PR-author level only — deskpr does not rewrite git commit authorship. |
| 4 | statusgen `--lint` | 0 | pinned `statusgen v0.5.0` → `LINT: PASS` (0 ERROR/WARN). In-repo `go run ./tools/statusgen` exits 1 — the known frozen-copy #465 false prod-DAR PROBLEM, unrelated to this brief. |

RISK-VALUE (all DERIVED, no `NAMED, NOT DERIVED` flags):
- `asApp` default = `true` @ `tools/desk/cmd/deskpr/deskpr.go:114,219` — brief Task 1 "default on once worker App exists"; worker App provisioned 2026-07-18 → cutover trigger met. Reversible (`--as-app=false`).
- `assay-worker-app[bot]` / App ID `4331284` — role→App mapping spec-derived (desk-apps Provisioned Apps table + I-38 "one App per role"); ID confirmed live via `gh api /app/4331284`, resolved from env not hardcoded.
- `forbiddenImportSub={"deskpost"}` + `forbiddenLiteralSub={REVIEWER_APP_ID, REVIEWER_INSTALL_ID, REVIEWER_APP_PRIVATE_KEY}` @ `deskreply_test.go:510,513` — enumerates deskpost's import path + the three REVIEWER env vars; matches the reviewer-App voice separation the brief enforces.

## Review
Gate: model + `/security-review`. Reviewer confirms author≠approver is now GitHub-enforced (worker-app
can't self-approve) and the reviewer-App voice stays exclusive to deskpost.
