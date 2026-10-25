---
brief: assay:assay:forge-neutral:17
title: deskrun log and retry — read-only run-log access broadly, retry roster-bound like dispatch
why: >-
  Brief 14 gives the fleet a `deskrun` verb for dispatching a workflow run and approving a
  pending-deployment gate, but a dispatched run is opaque without two more things: reading its
  log, and re-running it when it fails. Both need a forge permission, and the two permissions
  are NOT the same shape — `actions: read` (GitHub) / `read_api` (GitLab) can only read; retry
  needs `actions: write` / `api`, the same over-broad scope that made dispatch itself
  roster-bound in brief 14. Splitting `log` and `retry` into their own brief keeps that
  asymmetry visible instead of letting one over-broad grant quietly cover both.
wave: 2
depends: ["forge-neutral/01"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-neutral-17
decision-issue: 1557
ruling: "https://github.com/medici-finance/assay/issues/1557#issuecomment-6326324256"
issues: []
schema: brief-v2
authored: 2026-09-13 by forge-neutral authoring session
sources:
  - "#992 — the forge-neutral desk-verbs series tracking issue (briefs 14-18, RunWorkflow/ApproveGate through human-only surfaces); this is brief 4 of 5, deskrun log/retry"
  - "docs/streams/forge-neutral/brief-14-*.md (sibling, forge-neutral/14) — RunWorkflow/ApproveGate/RunStatus and the `deskrun` verb this brief extends. Not yet authored/landed at freshness-check time (2026-09-13 @ cc4f9e22): no `deskrun` command, no `RunWorkflow`/`ApproveGate`/`RunStatus` symbol exists in the tree. This brief specifies `log`/`retry` from #992's description of brief 14's planned shape rather than a landed file; per #992, each brief in the series is authored independently against the tracking issue, so this authoring proceeds on its own timeline"
  - "docs/streams/forge-neutral/README.md — the design principle quoted verbatim in Context, and the measured-matrix method this brief's facts follow"
  - "docs/streams/forge-neutral/identity.md — the forge-qualified roster grammar and forge-agreement refusal `retry` inherits unchanged"
  - "docs/streams/forge-gitlab/spec.md §6 — the frozen `Forge` interface and its freeze rule (an added op needs a consuming call site in the same change)"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op → tool → call-site table this brief appends to, after whichever rows brief 14 adds"
  - "tools/desk/internal/forgeban/allowlist.go:72 — allowedInvocationCeiling = 7 at authoring base cc4f9e22"
  - "freshness-checked 2026-09-13 @ cc4f9e22 (origin/main) — `grep -rn 'workflow\\|actions\\|pipeline' tools/desk/internal/forgeban/allowlist.go` returns no run/workflow-shaped permit row; `deskrun` is not a directory under tools/desk/cmd/"
exec-tier: strong
exec-tier-why: >-
  This brief decides which GitHub/GitLab scope is safe to grant BROADLY (worker and reviewer
  Apps both) versus which stays roster-bound to a dedicated credential — a subtle error here
  either under-grants (log reads stay blocked, so the motivating leak-sweep-shaped problem is
  not actually solved) or over-grants (retry's `actions: write` reaches a human-only App,
  reopening exactly the cancel/delete-logs/disable-workflow exposure brief 14 was written to
  avoid). Both mistakes pass a happy-path test that only dispatches and reads; only a
  negative-path refusal test on the human-bound roster case catches the second.
gate-why: >-
  `retry` inherits brief 14's identity rule: the desk never borrows an ambient human credential
  for a release-adjacent write, and the verb REFUSES (exit 5) when the roster resolves the
  retry-credential to a human rather than a dedicated App/token. This brief's human gate
  confirms two things: (1) that `log`'s `actions: read` / `read_api` grant is safe to widen to
  BOTH the worker and reviewer Apps — a scope-widening decision, even though read-only — and
  (2) that `retry` genuinely carries the same over-broad-scope shape as `RunWorkflow` and so
  earns the same roster-bound refusal rather than a lighter gate because it "only" retries.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/forge.go: follow-up forge-neutral/17-implementation (two new ops, `RunLog` and `RetryRun`, added to the frozen seam alongside brief 14's `RunWorkflow`/`ApproveGate`/`RunStatus`, each with a consuming call site in the same change — this brief is authoring-only, per Ground rules)"
  - "tools/desk/cmd/deskrun: follow-up forge-neutral/17-implementation (the `log`/`retry` subcommands on the verb brief 14 creates)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/17-implementation (two appended rows, numbered after whichever rows brief 14 lands first)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (this brief adds no new call site outside the enumerated ops, so no permit row is added or retired here)"
version: 1
id: f7a8021b-db1e-433d-9ef7-40afb1b6f0b5
---

# Brief 17 — deskrun log and retry: read-only run-log access broadly, retry roster-bound like dispatch

## Context

This is a **brief-authoring task, not an implementation task** (same posture as brief 14,
per #992) — the deliverable is this specification, with Verify rows a future implementer
runs. The design principle for the whole five-brief series, quoted verbatim from #992 because
every brief in the series inherits it rather than restating it:

> "every desk action a human does through the forge CLI today gets a desk verb with a
> forge-neutral mapping; the human-only ones stay human-only because SERVER-SIDE permissions
> make them so, not because the desk lacks a verb."

`log` and `retry` are the two remaining pieces of that principle for run/workflow visibility.
A human today reads a failed run's log and clicks retry through the forge's own UI or `gh run
view --log` / `gh run rerun`; nothing stops a desk App from doing the same, but nothing lets it
either — there is no `Forge` operation for either, and no `deskrun` subcommand exists yet
because brief 14, which creates the verb, has not landed.

files:
- `tools/desk/internal/deskkit/forge.go` (planned) — two new operations, `RunLog(repo, runID)`
  and `RetryRun(repo, runID)`, added beside brief 14's `RunWorkflow`/`ApproveGate`/`RunStatus`.
  Not touched by this authoring brief itself — this is the future implementer's file list.
- `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` (planned) — both backends'
  mappings.
- `tools/desk/cmd/deskrun/` (planned) — the `log` and `retry` subcommands on brief 14's verb.
- `docs/streams/forge-gitlab/inventory.md` (planned) — two appended rows.

single-point-of-failure: for `retry`, the single control is the SAME one brief 14 names for
`RunWorkflow` — which credential the roster resolves the retry identity to. Two independent
layers stand behind it, inherited unchanged from brief 14 and `forge-neutral/01`'s resolver:
the backends refuse an unminted token outright, and the roster-resolution step itself refuses
(exit 5) rather than falling through to an ambient human credential when the bound identity is
a human. For `log`, there is deliberately **no** analogous single point of failure to defend —
read access has no destructive side effect for a control to guard against, which is the whole
argument Task 1 makes for granting it broadly.

facts:
- **The scope split is the brief.** GitHub's `actions: read` permission can list workflow
  runs, read a run's status, and download/read its log — nothing else. GitHub's
  `actions: write` grants all of that PLUS cancelling any run, deleting run logs, re-running a
  job or workflow, and disabling a workflow repo-wide — brief 14's own why (#992) names this as
  the reason no desk App holds it broadly today. `log` needs only the first; `retry` needs the
  second, in full, because "retry" is exactly one of the write-shaped operations bundled into
  that scope.
- **Why `actions: read` is safe to grant broadly and `actions: write` is not — stated
  explicitly, not left implicit.** Read-only access to a run's log can be misused to leak the
  log's CONTENTS to whoever holds the credential — a confidentiality question, addressed by the
  ordinary access-control question of who the App/token belongs to, exactly like read access to
  any other repo content. It cannot be misused to cancel a run, delete a log, or disable a
  workflow, because the permission carries no write verb for any of those — there is no request
  shape `actions: read` authorizes that has a destructive side effect. `actions: write`'s
  authorization surface is a superset that includes retry PLUS three unrelated destructive
  operations no verb in this brief or brief 14 wants to grant. This is the same reasoning
  `repository_dispatch`-vs-workflow-dispatch gets in brief 14 for the write side; here it runs
  once for the read/write split instead.
- **The motivating case for `log` mattering, stated generically.** A gate whose verdict is a
  simple pass/fail commit status can carry a detail (which check failed, and why) that the
  status text has no room for; if the worker App that needs that detail has no `actions: read`
  equivalent, the detail has to live somewhere else the worker CAN reach — a side channel a
  human or a more-privileged reader populates by hand. That side channel is a workaround for
  exactly the credential gap this brief closes: once `log` is a desk verb any role can call
  directly, the detail can live in the run's own log again, and the side channel becomes
  optional rather than load-bearing. No private repo, channel, or issue is named here — the
  shape generalizes past this house's own instance of it.
- **Identity rule for `retry` — referenced, not restated.** Brief 14 specifies the roster-bound
  refusal for `RunWorkflow`: the desk resolves the run/approve credential from the roster, and
  REFUSES (exit 5) rather than silently falling back to an ambient human credential when the
  roster binds that role to a human. `retry` is the same shape of over-broad-scope write as
  `RunWorkflow` (both need `actions: write` / GitLab `api`), so it inherits that rule verbatim —
  this brief does not re-derive it, only asserts that `retry` is covered by it.
- **GitLab mapping.** A job's trace/log (`GET /projects/:id/jobs/:job_id/trace`) needs only
  `read_api` — the safe, read-only case, same reasoning as GitHub's `actions: read`. Retrying a
  job (`POST /projects/:id/jobs/:job_id/retry`) needs the broader `api` scope, which — like
  GitHub's `actions: write` — reaches far past retry (it is GitLab's one undifferentiated
  read/write API scope), so it gets the same roster-bound treatment as GitHub's `retry`.
- **The op-table numbering is deliberately left open.** `docs/streams/forge-gitlab/inventory.md`
  is at row 37 as of this brief's freshness check (`cc4f9e22`); brief 14 will append its own
  rows for `RunWorkflow`/`ApproveGate`/`RunStatus` first. This brief's `RunLog`/`RetryRun` rows
  are numbered relative to whatever brief 14 lands, not to a literal row number pinned here —
  pinning one now would either collide with brief 14's rows or leave a numbering gap depending
  on landing order, and the freeze rule cares that each op has ONE consuming call site in its
  own change, not that its row number was predicted correctly a brief in advance.
- **The interface stays frozen.** Both new ops are consumed by `deskrun log` / `deskrun retry`
  in the same change that adds them (spec §6); neither is a generic/passthrough method, and
  neither takes a caller-supplied endpoint.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. This is a brief-authoring task: no code in this brief's own diff touches
  `tools/desk/` or `.github/workflows/*`.
- Stop at `implemented` — you do not set verified/done. This applies to the FUTURE implementer
  of this brief's Task section, not to this authoring work, which stops once the brief file and
  the docs-index/README updates are in place.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not add a generic/passthrough method, a new `gh`/`glab` shell-out, or a speculative
  interface operation with no consuming call site. `.github/workflows/forge-surface-control.yml`
  must stay green unchanged.

## Task

This section specifies the work a future implementer of `forge-neutral/17` performs, once
brief 14 has landed `deskrun` and its `RunWorkflow`/`ApproveGate`/`RunStatus` ops.

1. **Add `RunLog(repo ForgeRepo, runID string) (io.Reader, error)` (or an equivalent
   streamed/bounded-read shape — the implementer picks the concrete Go signature, consistent
   with how `ReadFile` (op 22) already returns content) to `Forge`, both backends.**
   - GitHub: `GET /repos/{o}/{r}/actions/runs/{run_id}/logs` (a redirect to a zip archive of
     per-job logs) under `actions: read`. A run whose logs have expired or been deleted answers
     404, propagated as `IsForgeNotFound` rather than an empty success.
   - GitLab: `GET /projects/:id/jobs/:job_id/trace` under `read_api` — GitLab's trace is
     per-JOB, not per-pipeline, so a pipeline-level `RunLog` call resolves to its jobs first (an
     existing or new read, per the implementer's judgement) and reads each job's trace; a
     pipeline with more than one job is NOT silently reduced to the first job's trace.
   - Consumed by `deskrun log <run>` in the same change.
2. **Add `RetryRun(repo ForgeRepo, runID string) error` to `Forge`, both backends.**
   - GitHub: `POST /repos/{o}/{r}/actions/runs/{run_id}/rerun` (or `rerun-failed-jobs` — the
     implementer states which and why in the PR body; `rerun-failed-jobs` is the narrower,
     safer default and should be preferred absent a reason to retry the whole run) under
     `actions: write`.
   - GitLab: `POST /projects/:id/jobs/:job_id/retry` under `api`.
   - Consumed by `deskrun retry <run>` in the same change.
3. **`deskrun retry` resolves its credential through the SAME roster-bound identity rule brief
   14 specifies for `RunWorkflow`** (do not re-derive; cite brief 14's identity section). A
   roster entry that binds the retry role to a human REFUSES (exit 5) before any request is
   made; the refusal names the role and the human login it resolved to, so the failure reads as
   a policy statement rather than an opaque credential error.
4. **`deskrun log` carries no such refusal.** It resolves its credential the same way every
   other read-only desk verb does — the calling role's own minted token — and succeeds for
   BOTH the worker and reviewer roles against a fixture run, on both forges. There is no
   roster-bound identity gate on `log`; Task 1's per-note fact is the argument for why one is
   unnecessary, not merely omitted.
5. **Append two rows to `docs/streams/forge-gitlab/inventory.md`**, numbered after whichever
   rows brief 14 added, with each row's GitHub call, GitLab call, and scope named per Task 1/2
   above, and each row's `gitlab impl` column ticked only once a golden-pinned contract case
   exists for it (the existing convention every row in that table already follows).
6. **Docs-index row.** Add `deskrun log` and `deskrun retry` to whichever desk-verbs index
   document lists verb-to-scope mappings (the file brief 14 creates or extends for
   `RunWorkflow`/`ApproveGate`), so a reader auditing "what can this App do" finds both verbs
   there rather than only in this brief.

## Verify (executable — no prose-only DoD items)

These rows are the future implementer's deliverable Verify table, written now so this brief is
dispatchable without a second authoring pass. This brief follows the `Class`-column convention
brief 13 established (mistake-proofing/03): `+dereference` marks a row that resolves a claim
rather than counting its presence, `+flow` a row that exercises the cross-component path (a
write verb → `ForgeFor` → the backend) end to end. Test names are planned, created by the
implementer: `TestDeskrunLogSucceedsWorkerAndReviewer` (planned), `TestDeskrunRetryRefusesOnHumanRoster` (planned),
`TestDeskrunRetrySucceedsOnAppRoster` (planned), `TestForgeNoPassthroughAfterRunLogRetry` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunLogSucceedsWorkerAndReviewer -count=1 -v` | exit 0; the fixture run's log is read successfully under BOTH the worker-role and reviewer-role fixture credential, on both forges (GitHub fixture + GitLab fixture) — POSITIVE, proves the broad grant actually works end to end (verb → `ForgeFor` → backend) for both roles rather than only the one the implementer tested first |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetryRefusesOnHumanRoster -count=1 -v` | **negative path**: with the roster's retry-role entry bound to a human login (fixture), `deskrun retry` REFUSES (exit 5), performs zero forge requests (asserted via a recording fake), and the refusal message names the role and the resolved human login — both forges |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetrySucceedsOnAppRoster -count=1 -v` | with the roster's retry-role entry bound to a dedicated App/token (fixture), `deskrun retry` succeeds and issues exactly one retry request against the fixture run — both forges |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoPassthroughAfterRunLogRetry -count=1 -v` | exit 0 — the two new ops do not widen the seam into a generic/endpoint-taking method; `.github/workflows/forge-surface-control.yml`'s no-passthrough shape check stays green |
| 6 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1` | exit 0 — `run_log` / `retry_run` golden-pinned contract cases pin each backend's REAL wire shape (not merely that a case exists) for both new ops |
| 7 | check | `grep -cE -e 'RunLog' -e 'RetryRun' docs/streams/forge-gitlab/inventory.md` | prints `2` or more — both ops are recorded in the frozen op table with a ticked `gitlab impl` column |
| 8 | check | a docs-index row confirming both verbs are listed in the desk-verbs index (exact file TBD by whatever brief 14 creates) | `grep -cE -e 'deskrun log' -e 'deskrun retry' <that index file>` prints `2` or more |
| 9 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/17` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff, not merely counted as present |
| 10 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskrun/... -count=1` — the test command this mutation row is judged by. **Mutation demonstration for the retry refusal and the custody gate.** In `cmdRetry`, ignore the resolver's refusal so a human-bound repo proceeds (then, separately, make `githubCustodyMint` accept any role, and widen the `log` role check) — run the command above on each mutated tree; restore each file and re-run it | exit **0** on the unmutated tree; exit non-zero on every mutated tree (the committed `cmd/deskrun/mutations.json` entries, run with `muhar -spec`, all report CAUGHT) — exit **0** again after restoring. Proves the human-bound refusal and the closed credential set are live controls, not checks nothing exercises |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| `log` is granted only to the worker App, leaving the reviewer App with the same blind spot brief 14's motivating case describes | row 2's both-roles assertion |
| `retry`'s roster-bound refusal is implemented for `RunWorkflow` but forgotten for `RetryRun` (a second, separately-coded write path that shares the same over-broad scope) | row 3 |
| `retry`'s refusal test only checks the ERROR string, not that zero forge requests were made — a fixture that quietly issues the request anyway before returning an error would still pass | row 3's explicit "zero forge requests" assertion via a recording fake |
| The GitLab `retry` mapping is left ungated because `api` "sounds like" a normal scope rather than the same over-broad shape as GitHub's `actions: write` | fact section states the equivalence explicitly; row 3/4 run on both forges |
| `RunLog`/`RetryRun` grow into a generic "run a request against this run" method, reopening the passthrough the surface-control CI exists to forbid | row 5 |
| The inventory table gains prose describing the ops without a ticked golden-pinned case, so `gitlab impl` reads implemented when no test backs it | row 6 + row 7 (both required together, the `TestForgeGitlabCoverage` reconciliation pattern from brief 13) |
| GitHub's `rerun` vs `rerun-failed-jobs` choice is made silently, so a future reader cannot tell which one a `deskrun retry` call actually performs | Task 2 requires the choice stated in the PR body; review-gated, no Verify row substitutes for review judgement here |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Verifier evidence — 2026-10-21 (b9263b40b), non-implementer, evidence-only (gate: human, sensitive-data: yes) — superseded by the 2026-10-25 run below

Recorded verdict of this earlier run, in plain words: blocked. Row 1 was could-not-check in the network-off image, where five packages the delivery did not touch failed on the image's own git and platform. Restated after review: rows 2-7 and 10 met their Expect. Rows 8 and 9 passed only on substituted commands, which are authoring defects (#1915 covers row 9). Row 4 issues one GitLab retry per failed job, an interpretation for the human gate. This run carried no verifyrun execution witness. Status cells untouched.

Verification-Attestation: medici-finance/assay#3998 run=ee145a74db28e14dc859ae282204eca7d51e1f73afebf6a5ae9bcced499f1a2c source=b9263b40b67aeb976099e337af74484acfa20451 brief=docs/streams/forge-neutral/brief-17-deskrun-log-retry.md model=claude-opus-5-5 tier=strong

Verified source: merged main b9263b40b (delivering merge f4e4da990, PR #2382; no tools/desk change between that merge and the verified source). Every check:ci row ran in the assay-verify:go1.27-py image with networking off, as uid 65534, against a full-history scratch clone at the verified source (the home's gitdir is not visible inside the container); Go modules came from a read-only module cache with GOPROXY=off.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | cd tools/desk && go build ./... && go test ./... | exit 0 | could-not-check: rc=1 in the mandated image; 112 packages ok, 5 FAIL (deskmerge, desksourceguard, deskwt, gitmaintenance, regression), none touched by the delivery diff. Causes are the image: its git 2.47 lacks the native hook-list subcommand that the maintenance and temporary-ref tests require; the image's linux-arm64 platform is not one desk-tools publishes (desksourceguard default-platform test); the cell shim fixture suite fails under the image, which also ships gh. Every package this brief touches is ok in the same run (cmd/deskrun, cmd/desktoken, internal/deskkit, internal/forgeban). Supplemental, no acceptance credit: the five failed packages re-run on a host with git 2.56, offline module resolution, all ok, rc=0 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunLogSucceedsWorkerAndReviewer -count=1 -v | exit 0; log read under worker AND reviewer credential on both forges | rc=0; subtests github-worker, github-reviewer, gitlab-worker, gitlab-reviewer PASS; the test asserts each read uses the calling role's own token and never a release-runner token, strips terminal control bytes, and tail-truncates | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetryRefusesOnHumanRoster -count=1 -v | exit 5 refusal, zero forge requests, message names role and human login, both forges | rc=0; github and gitlab subtests PASS; asserted: exit 5, message names release-runner, the fixture human login and the repo, and zero recording-fake calls, zero resolver calls, zero mints, no output | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetrySucceedsOnAppRoster -count=1 -v | retry succeeds, exactly one retry request, both forges | rc=0; github and gitlab PASS. GitHub: exactly one POST to the rerun-failed-jobs endpoint. GitLab: one GET of the run's jobs, then one retry POST per FAILED job (2 of the fixture's 3 jobs), because GitLab has no run-wide retry for failed jobs; that is one retry request per failed job, and the reviewer should weigh it against the singular Expect. Dry-run issues no request | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoPassthroughAfterRunLogRetry -count=1 -v | exit 0, no passthrough widening | rc=0; the test PASS; the delivery diff leaves the forgeban package unchanged, and that package is ok in row 1's run | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 | exit 0, run_log / retry_run goldens pin the real wire shape | rc=0 on both. Verbose run: GitHub cases run_log, run_log_mixed_whole_and_steps, run_log_steps_only, run_log_empty_archive, run_log_expired, run_log_refuses_path_shaped_id, retry_run, retry_run_forbidden, retry_run_refuses_unminted_token; GitLab cases run_log, run_log_no_jobs, run_log_trace_not_found, retry_run, retry_run_partial, retry_run_no_failed_job, retry_run_refuses_unminted_token; all PASS. The GitHub retry_run golden pins a single POST to the run's rerun-failed-jobs path | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | grep -cE -e 'RunLog' -e 'RetryRun' docs/streams/forge-gitlab/inventory.md | prints 2 or more | printed 3, rc=0; op rows 62 (RunLog) and 63 (RetryRun) both read implemented | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | grep -cE -e 'deskrun log' -e 'deskrun retry' tools/desk/README.md | prints 2 or more | printed 2, rc=0; the index is the desk-verbs Tool reference that brief 14 created, and both verbs are listed | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | statusgen --root . --consumers --brief forge-neutral/17 --delivery f4e4da9904f8e9ee523707dafd4b9c5817f06f1e --base 1470343e86359001c42aee3944b70fb05a012151 | exit 0, every consumers claim corroborated against the delivering diff | rc=0; 3 corroborated (deskkit forge seam, cmd/deskrun, the gitlab inventory), 0 disproved, 1 unchecked: the forgeban allowlist entry is out-of-scope, which no diff can settle; checked by hand: the delivery diff leaves the forgeban package unchanged. On merged main the row's literal command (without the delivery pin) exits 0 but reports 0 corroborated and 4 unchecked, because merged main carries no branch diff. The delivery-pinned form is statusgen's own post-merge mode for exactly this case. statusgen was built from the verified source | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | cd tools/desk && go test ./cmd/deskrun/... -count=1 (unmutated, each of the three named mutations applied then restored) and muhar -spec cmd/deskrun/mutations.json | exit 0 unmutated; non-zero on every mutation; exit 0 restored; muhar all CAUGHT | unmutated rc=0. Retry ignores the resolver's refusal: rc=1 (retry human-roster refusal test FAIL, github and gitlab); restored rc=0. Custody mints any role: rc=1 (custody-mint-closed test FAIL); restored rc=0. Log role set widened: rc=1 (log-refuses-other-roles and reads-leave-write-gate tests FAIL); restored rc=0. muhar rc=0: baseline GREEN, positive control CAUGHT, 36 caught, 0 NOT CAUGHT, 0 could-not-mutate | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Risk-bearing values (sensitive-data: yes), at b9263b40b. ENUMERATE:

- V1 = logRoles map with worker: true and reviewer: true @ tools/desk/cmd/deskrun/verbs.go:293
- V2 = custody mint gate, role must be the release-runner role or in logRoles @ tools/desk/cmd/deskrun/forge.go:67
- V3 = ReleaseRunnerRole = "release-runner" @ tools/desk/internal/deskkit/runcredential.go:46
- V4 = GitHub retry endpoint /actions/runs/%d/rerun-failed-jobs @ tools/desk/internal/deskkit/forge_github.go:3479
- V5 = GitLab retry targets only jobs whose status is Failed, one retry POST per job @ tools/desk/internal/deskkit/forge_gitlab.go:4548 (filter) and :4558 (retry path)
- V6 = readVerbs map with status and log @ tools/desk/cmd/deskrun/verbs.go:476
- V7 = DeskrunReadTool = "deskrun-read" @ tools/desk/internal/deskkit/ratelimit.go:414
- V8 = RunLogPartCap 4 MiB, RunLogReadCap 64 MiB, RunLogMaxParts 200, RunLogRunCap 64 MiB, runLogArchiveCap 64 MiB @ tools/desk/internal/deskkit/forge.go:1578-1582
- V9 = gitlabPerPage 100, gitlabMaxCIPage 25 @ tools/desk/internal/deskkit/forge_gitlab.go:465 and :467

RANK (by how far a wrong value moves the sensitive surface): V1, V2 (who can read run-log contents), V3 (which identity writes), V4, V5 (how wide a write reaches), V6, V7 (which audit and breaker lane a read uses), V8, V9 (reversible size and paging bounds).

DERIVE:

- RISK-VALUE V1 = {worker, reviewer}: DR-forge-neutral-17's accepted ruling (medici-finance/assay#1557 comment 6326324256) grants the log read to the worker and reviewer roles only, as a closed two-role set. The value matches that set exactly, with no third role. Read scope cannot change or destroy a run. The log-widening mutant is CAUGHT (row 10).
- RISK-VALUE V2: the mint gate admits only V3 and V1. Its role is to keep custody from minting any other role's token. The custody mutant is CAUGHT (row 10), and row 2 asserts a log read never borrows the release-runner token.
- RISK-VALUE V3 = release-runner: the same ruling allows retry only through the roster-bound release-runner credential. The resolver refuses a human binding (exit 5) before any mint or request, and row 3 asserts zero calls. The refusal-ignored mutant is CAUGHT (row 10).
- RISK-VALUE V4 = rerun-failed-jobs: the brief prefers the narrower endpoint, which re-runs only failed jobs, over a full re-run. The golden pins one POST to it (row 6).
- RISK-VALUE V5 = failed jobs only: this is the GitLab equivalent of V4's narrowing. The partial and no-failed-job goldens pin it (row 6). Row 4's per-job request count follows from it.
- RISK-VALUE V6/V7: log is a read verb on its own deskrun-read audit bucket, separate from the write budget and breaker. The read-set-drop mutant is CAUGHT (muhar, row 10).
- RISK-VALUE V8/V9: named only. These are reversible operational bounds on log size and paging, ranked last, and neither one changes who can read or write.
### Verifier evidence — 2026-10-25 (61eb62a4a), non-implementer, evidence-only (gate: human, sensitive-data: yes)

**VERIFY: BLOCKED** — rows 2, 5, 6, 7 and 10 meet their Expect, witnessed. Rows 3, 8 and 9 cannot pass as written; these are authoring defects, not product failures (#4395 for rows 3 and 8, #1915 for row 9). Row 3's test passes, but its Expect states the verb's exit 5 as the command's exit code. Rows 8 and 9 pass only on substituted commands, which are recorded below as supplemental and take no acceptance credit. Row 4 is witnessed green on both forges, but on GitLab it issues one retry per failed job rather than the single retry request the Expect names; whether that meets the Expect is an interpretation the human gate decides, so row 4 takes no pass credit here. Row 1 is could-not-check in the container lane. The witness records fail exit=1, red at the base as well. A supplemental package breakdown in the same image shows 108 packages ok and 10 failing on image causes: git 2.39.5 (#4207), the 10-minute per-package timeout under amd64 emulation (#4323), and jq absent from the image. Every package the delivery touches is ok except internal/deskkit, and its failing tests are not in the delivery diff. Status cells untouched; the brief stays held for the human gate (decision #1557).

Verified source: merged main 61eb62a4a (delivering merge f4e4da990, PR #2382; its first parent 1470343e8 is the fail-first base). The deskrun code, the gitlab inventory, tools/desk/README.md and the brief are unchanged from the earlier 2026-10-21 run's source b9263b40b.

Execution witness. `statusgen verifyrun --fail-first --base 1470343e86359001c42aee3944b70fb05a012151` ran in the container admission bracket: `--sandbox=container-netns` inside `docker run --network none` at default seccomp, as a non-root user. The image is pinned by digest: amd64, go 1.27.2, git 2.39.5. statusgen v1.0.59 is the official release binary, and its sha256 matches the release checksums. The home is mounted read-only with its gitdir. Go modules come from a read-only module cache with GOPROXY=off. The host opened and closed the bracket, and `--bracket-digest` reports verdict=match. The witness bytes follow, unedited:

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 sandbox=container-netns | sha256:a8c6b0d02242 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 red rc=1 · head=61eb62a4a61b |
| 2 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunLogSucceedsWorkerAndReviewer -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:c385851fd1f7 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 green rc=0 · head=61eb62a4a61b |
| 3 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetryRefusesOnHumanRoster -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:be8ba191659e | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 red rc=0 · head=61eb62a4a61b |
| 4 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetrySucceedsOnAppRoster -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:a6bdf236de80 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 green rc=0 · head=61eb62a4a61b |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoPassthroughAfterRunLogRetry -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:0c671f5caf0d | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 green rc=0 · head=61eb62a4a61b |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1` | pass exit=0 sandbox=container-netns | sha256:a7850c110146 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 green rc=0 · head=61eb62a4a61b |
| 7 | `grep -cE -e 'RunLog' -e 'RetryRun' docs/streams/forge-gitlab/inventory.md` | pass exit=0 sandbox=container-netns | sha256:1121cfccd591 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 red rc=1 · head=61eb62a4a61b |
| 8 | `a docs-index row confirming both verbs are listed in the desk-verbs index (exact file TBD by whatever brief 14 creates)` | fail exit=2 sandbox=container-netns | sha256:ab3f0b3d9b3d | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 red rc=2 · head=61eb62a4a61b |
| 9 | `statusgen --root . --consumers --brief forge-neutral/17` | fail exit=2 sandbox=container-netns | sha256:39b35f13c2a8 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 red rc=2 · head=61eb62a4a61b |
| 10 | `cd tools/desk && go test ./cmd/deskrun/... -count=1` | pass exit=0 sandbox=container-netns | sha256:e3b5ec78cbe1 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) | base=1470343e8635 green rc=0 · head=61eb62a4a61b |
Tree-Digest: sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9
Container-Image: sha256:840101ee78c4f508da9d7e035a553fa0c1e5e4f73bf3dd4859091d55dd7687f4
Verification-Attestation: medici-finance/assay#4372 run=f4f51e1f415446316c83d96dd770324f304b2e256e130eb858d6647008a98729 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 brief=docs/streams/forge-neutral/brief-17-deskrun-log-retry.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=f4f51e1f415446316c83d96dd770324f304b2e256e130eb858d6647008a98729 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 tree=sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9 witness=sha256:59e832d29eb93122e9777e3a9e75a814e8fb453f3943e0a5054db86df1d9c42c opened=2026-10-25T13:23:39Z closed=2026-10-25T14:35:29Z before=receipt after=check-verifier

Row-by-row reading of the witness. The witness rows above carry the acceptance result. Anything marked supplemental here takes no acceptance credit.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | cd tools/desk && go build ./... && go test ./... | exit 0 | could-not-check. Witness fail exit=1 (sha256:a8c6b0d02242), red at the base too. Supplemental breakdown, same image and mounts, network off (log sha256:3a228cf97dd8): 108 packages ok, 10 FAIL. Causes, all in the image: (a) git 2.39.5 (#4207). Native hook list absent, which hits gitmaintenance, deskwt and deskmerge. Git 2.41 tracking-attribute inspection is needed by deskevidence. gitquiet sees a maintenance fork. (b) 10m per-package timeouts under amd64 emulation (#4323). Affected: deskwt, muhar, internal/deskkit, reviewtest. (c) jq absent from the image. deskkit TestReviewCallerFlow and reviewtest fail with "pr-monitor: jq not found". (d) cellctl TestBinaryLaunchBanner and deskdispatch TestResumeRealAllocator: not diagnosed further. None of the failing test files is in the delivery diff. cmd/deskrun, cmd/desktoken and internal/forgeban are ok. internal/deskkit's only failure is TestReviewCallerFlow (jq) plus a 10m timeout in TestReviewPlanNoFetch; its forge goldens and passthrough test pass in rows 5 and 6 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunLogSucceedsWorkerAndReviewer -count=1 -v | exit 0; log read under worker and reviewer credential on both forges | witness pass exit=0 (sha256:c385851fd1f7). Fail-first: non-discriminating, green at base 1470343e8, because at the base the test name matches nothing and go test exits 0. The witness output hash is the record of the four subtests | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetryRefusesOnHumanRoster -count=1 -v | exit 5 refusal, zero forge requests, both forges | cannot pass as written (#4395). The witness records fail exit=0 (sha256:be8ba191659e) because it reads the Expect's exit 5 as the go test exit code; the same reading is red at the base. Supplemental, same image, network off: the test PASSES; the github and gitlab subtests each log "refused (exit 5) with zero forge calls", and the message names release-runner, the fixture human login and the repo | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRetrySucceedsOnAppRoster -count=1 -v | retry succeeds, exactly one retry request, both forges | witness pass exit=0 (sha256:a6bdf236de80); green at the base (non-discriminating, as row 2). GitHub issues one POST to rerun-failed-jobs. GitLab issues one jobs GET, then one retry POST per failed job (2 of the fixture's 3 jobs), because GitLab has no run-wide retry for failed jobs. That differs from the singular Expect, and it is an interpretation for the human gate, not a pass of the Expect as worded | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoPassthroughAfterRunLogRetry -count=1 -v | exit 0, no passthrough widening | witness pass exit=0 (sha256:0c671f5caf0d); green at the base (non-discriminating). The delivery diff 1470343e8..f4e4da990 leaves tools/desk/internal/forgeban unchanged (0 diff lines) | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 | exit 0, run_log / retry_run goldens pin the wire shape | witness pass exit=0 (sha256:a7850c110146). Green at the base, because both golden suites already exist there; the run_log and retry_run cases themselves arrive with the delivery | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | grep -cE -e 'RunLog' -e 'RetryRun' docs/streams/forge-gitlab/inventory.md | prints 2 or more | witness pass exit=0 (sha256:1121cfccd591, the same output hash as the earlier run, which printed 3). Fail-first: red at the base, pass at head | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | a docs-index row confirming both verbs are listed in the desk-verbs index (exact file TBD by whatever brief 14 creates) | grep prints 2 or more | cannot pass as written (#4395). The Command cell is prose, so the witness records fail exit=2 (sha256:ab3f0b3d9b3d), red at the base too. Supplemental: grep -cE -e 'deskrun log' -e 'deskrun retry' tools/desk/README.md printed 2, rc=0 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | statusgen --root . --consumers --brief forge-neutral/17 | exit 0, every consumers claim corroborated | cannot pass as written (#1915). The witness records fail exit=2 (sha256:39b35f13c2a8): on merged main, statusgen v1.0.59 reports COULD-NOT-CHECK, because the brief is not in the diff against main and the delivery cannot be derived. Supplemental, with --delivery f4e4da9904f8e9ee523707dafd4b9c5817f06f1e --base 1470343e86359001c42aee3944b70fb05a012151: rc=0, 3 corroborated (deskkit forge seam, cmd/deskrun, gitlab inventory), 0 disproved, 1 unchecked. The unchecked claim is the forgeban allowlist entry, out-of-scope, which no diff can settle; the delivery diff leaves forgeban unchanged | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | cd tools/desk && go test ./cmd/deskrun/... -count=1, and muhar -spec cmd/deskrun/mutations.json | exit 0 unmutated; every mutation CAUGHT | witness pass exit=0 (sha256:e3b5ec78cbe1); green at the base (the package exists there). muhar is the official v1.0.59 release binary, checksum-matched. It ran in the same image with the network off, against a full git archive of 61eb62a4a: rc=0, baseline GREEN, positive control CAUGHT, 36 caught, 0 NOT CAUGHT, 0 could-not-mutate. All three brief-17 mutants were CAUGHT: retry ignores the resolver's refusal, the log role set is widened, custody mints any role. muhar log sha256:58fc900516bc | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Fail-first summary. Rows 1, 3, 7, 8 and 9 are red at the base. Row 7 is the only one that turns green at head. Rows 3, 8 and 9 stay red as authored, and row 1 stays red on the image causes above. Rows 2, 4, 5, 6 and 10 are green at the base: the -run selectors match nothing there and exit 0, or the package already existed. The witness records them as non-discriminating. Row 10's muhar mutants are the discriminating evidence for the risk-bearing controls.

Earlier-run follow-ups: #4010, which covered the go1.27-py image causes that the 2026-10-21 run hit, was closed as completed on 2026-10-23 when the container admission-bracket lane landed (#4158). This run uses that lane.

Risk-bearing values (sensitive-data: yes), at 61eb62a4a. ENUMERATE:

- V1 = logRoles map with worker: true and reviewer: true @ tools/desk/cmd/deskrun/verbs.go:293
- V2 = custody mint gate, role must be the release-runner role or in logRoles @ tools/desk/cmd/deskrun/forge.go:67
- V3 = ReleaseRunnerRole = "release-runner" @ tools/desk/internal/deskkit/runcredential.go:46
- V4 = GitHub retry endpoint /actions/runs/%d/rerun-failed-jobs @ tools/desk/internal/deskkit/forge_github.go:3484
- V5 = GitLab retry targets only jobs whose status is Failed, one retry POST per job @ tools/desk/internal/deskkit/forge_gitlab.go:4552 (filter) and :4562 (retry path)
- V6 = readVerbs map with status and log @ tools/desk/cmd/deskrun/verbs.go:476
- V7 = DeskrunReadTool = "deskrun-read" @ tools/desk/internal/deskkit/ratelimit.go:414
- V8 = RunLogPartCap 4 MiB, RunLogReadCap 64 MiB, RunLogMaxParts 200, RunLogRunCap 64 MiB, runLogArchiveCap 64 MiB @ tools/desk/internal/deskkit/forge.go:1589-1593
- V9 = gitlabPerPage 100, gitlabMaxCIPage 25 @ tools/desk/internal/deskkit/forge_gitlab.go:465 and :467

RANK, by how far a wrong value moves the sensitive surface:
1. V1, V2: who can read run-log contents.
2. V3: which identity writes.
3. V4, V5: how wide a write reaches.
4. V6, V7: which audit and breaker lane a read uses.
5. V8, V9: reversible size and paging bounds.

The values are unchanged from the earlier run. Only the line numbers in forge_github.go, forge_gitlab.go and deskkit forge.go moved, from unrelated edits above them.

DERIVE:

- RISK-VALUE V1 = {worker, reviewer}: the accepted ruling (medici-finance/assay#1557 comment 6326324256) grants the log read to the worker and reviewer roles only, as a closed two-role set. The value matches that set exactly. Read scope cannot change or destroy a run. The log-widening mutant is CAUGHT (row 10).
- RISK-VALUE V2: the mint gate admits only V3 and V1, so custody cannot mint any other role's token. The custody mutant is CAUGHT (row 10).
- RISK-VALUE V3 = release-runner: the same ruling allows retry only through the roster-bound release-runner credential. The resolver refuses a human binding with exit 5 before any mint or request (row 3 supplemental: zero forge calls). The refusal-ignored mutant is CAUGHT (row 10).
- RISK-VALUE V4 = rerun-failed-jobs: the brief prefers the narrower endpoint, which re-runs only failed jobs, over a full re-run. The golden pins one POST to it (row 6).
- RISK-VALUE V5 = failed jobs only: this is the GitLab equivalent of V4's narrowing. The partial and no-failed-job goldens pin it (row 6), and the "retries every job" mutant is CAUGHT (row 10). Row 4's per-job request count follows from this value.
- RISK-VALUE V6/V7: log is a read verb on its own deskrun-read audit bucket, separate from the write budget and breaker. The read-set-drop and bucket-fold mutants are CAUGHT (row 10).
- RISK-VALUE V8/V9: NAMED, NOT DERIVED. These are reversible operational bounds on log size and paging, ranked last. Neither one changes who can read or write.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between `deskrun retry` and a write performed as the wrong
   (human-borrowed) identity, and is that acceptable? (The roster-resolution refusal, inherited
   from brief 14's `RunWorkflow` rule; the backends' own unminted-token refusal is the second,
   independent layer behind it.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed?
   (Row 3: with the roster bound to a human, the verb refuses before any request — the backend
   never even sees a call to independently refuse, so the roster check is proven to be the one
   actually firing, not merely present alongside a second check that would have caught it
   anyway.)
3. Is granting `actions: read` / `read_api` broadly (worker AND reviewer) actually safe, or does
   it quietly widen who can see something sensitive in a run's log? (This is a confidentiality
   judgement about log CONTENTS, not an authorization-surface question — the reviewer confirms
   the fixture roles this brief tests are the right set to grant, not a superset chosen for
   convenience.)
