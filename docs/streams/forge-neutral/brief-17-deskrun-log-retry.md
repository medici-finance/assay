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
