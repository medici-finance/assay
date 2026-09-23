---
brief: assay:assay:forge-neutral:14
title: Run and gate-approval verbs — RunWorkflow, ApproveGate and deskrun on the resolver
why: >-
  A release or a gated CI run is started today by a human's own ambient forge CLI —
  `gh workflow run` to dispatch a workflow, or approving a `pending_deployments` gate by
  hand in the Actions UI — because GitHub's `actions: write` permission grants dispatching
  a run and approving a deployment gate, but ALSO grants cancelling any run, deleting run
  logs, and disabling workflows repo-wide. No desk App is safely grantable that scope, so
  no desk verb exists for either operation and both fall to whoever's ambient credential is
  active. GitLab needs an entirely different pair of calls for the same two operations, so
  this is not a one-forge gap either. #949's own verify step needed exactly this: a
  release-workflow dry run (`gh workflow run release.yml -f version=<v> -f
  dry_run=true`) captured as evidence that the change worked — and the only way to run it
  was a human's own ambient `gh` session, because no forge-neutral, safely-scoped verb
  existed to dispatch it with.
wave: 2
depends: ["forge-neutral/01"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-neutral-14
decision-issue: 1556
exec-tier: strong
exec-tier-why: "this decides which credential is allowed to start a release or unblock a gated deployment, and a subtle error — a resolver that accepts a human-bound roster entry and mints an ambient token anyway, a RunRef correlation that silently picks the wrong run — survives every happy-path test and hands the wrong actor a release trigger (questions a and c)."
gate-why: >-
  This brief decides who may start a workflow run or clear a deployment gate, and it adds
  the one refusal that keeps a desk verb from ever borrowing a human's ambient credential
  for that purpose. The human confirms three things: that `repository_dispatch` is
  correctly left un-adopted as the default trigger (it only needs `contents: write`, a
  scope desk Apps already hold, which is exactly why it is the wider surface); that the
  GitLab default is the narrow, start-only pipeline trigger token rather than a
  project-level token that can also read/write; and that a roster entry naming
  `human:<name>` is a legitimate state `deskrun` refuses on, not a bug to route around.
issues: [949]
schema: brief-v2
authored: 2026-09-13 by forge-neutral authoring session
sources:
  - "#949 (merged) — its own \"How to verify\" step reads `gh workflow run release.yml -f version=<v> -f dry_run=true`: a release-workflow dry run captured as evidence, dispatched from a human's own ambient `gh` session for lack of any other credentialed path"
  - "docs/streams/apps-installer/brief-03-deskapps-install-prove.md:78 — an independent, pre-existing rule in this repo: \"never call `gh workflow run`\" for the desk-apps installer identity, for the identical reason this brief generalises"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver (`ForgeFor`), its refusal contract, and the per-forge custody binding this brief's identity rule extends"
  - "docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md — the precedent this brief follows: add the ops the seam lacks, with a consuming verb, in the same change; the freeze rule is amended, not bypassed"
  - "docs/streams/forge-neutral/identity.md — the forge-qualified roster grammar (`[role=]<forge>:<slug-or-login>[:<id>]`) and the established `human:<slug>` identity token (statusgen/verifyrun.go:654) this brief's refusal condition reads"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op → tool → call-site table; this brief appends rows 38–40"
  - "tools/desk/README.md — the Tool reference table (\"## Tool reference\"), this repo's desk-verbs index; this brief adds deskrun's row"
  - "freshness-checked 2026-09-13 @ cc4f9e22 (origin/main) — `grep -rn 'gh workflow run' tools/ docs/` finds the askassay probe's ban-test fixture and the apps-installer brief's own rule; no verb dispatches a workflow or approves a deployment gate anywhere in the tree; `tools/desk/internal/deskkit/forge.go`'s `Forge` interface carries 30 methods, none of them a run-dispatch or gate-approval op; `git ls-files | grep -c gitlab-ci` returns 0 — no `.gitlab-ci.yml` exists in this repo's own CI, so no CI-reachable GitLab fixture exists for a live dry-run row"
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/forge.go: follow-up forge-neutral/14 (three ops added to the frozen seam — RunWorkflow, ApproveGate, RunStatus — both backends, each consumed by deskrun in this same change)"
  - "tools/desk/cmd/deskrun: follow-up forge-neutral/14 (new verb: `deskrun <repo> <workflow> --ref <r> -f k=v` dispatches; `deskrun approve <run>` approves a gate; both refuse exit 5 on a human-bound run-credential)"
  - "tools/desk/internal/deskkit/rosterconfig.go: follow-up forge-neutral/14 (the new per-repo run-credential binding registers in the known-set; a `release-runner` role joins the role-bindings vocabulary)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (this brief retires no permit row — deskrun is a NEW verb with no prior `gh workflow run` call site to retire; it exists today only as a human-run command, never a tracked forgeban row)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/14 (rows 38-40)"
  - "tools/desk/README.md: follow-up forge-neutral/14 (deskrun's row in the Tool reference table)"
version: 1
id: a91ce800-824a-4f84-9604-094f5d3674fa
---

# Brief 14 — Run and gate-approval verbs

## Context

files:
- `tools/desk/internal/deskkit/forge.go` — the frozen seam; three ops added.
- `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` — the two backend
  implementations of the three new ops.
- `tools/desk/internal/deskkit/forge_github_golden_test.go`,
  `forge_gitlab_golden_test.go` — golden fixtures for the new ops, both backends.
- `tools/desk/internal/deskkit/forgeresolve.go` — `ForgeFor` and the custody binding;
  the run-credential resolution this brief adds reuses this, not a parallel path.
- `tools/desk/internal/deskkit/rosterconfig.go` — the new per-repo run-credential config
  key and the `release-runner` role register here.
- `tools/desk/cmd/deskrun/` (new) — the verb.
- `docs/streams/forge-gitlab/inventory.md` — the frozen op table, rows 38-40 appended.
- `tools/desk/README.md` — the Tool reference table, `deskrun`'s row.

single-point-of-failure: the roster's run-credential binding is the one control between
"a workflow starts, or a gate clears" and the wrong actor doing either. Two independent
layers stand behind it: the backends refuse an unminted token outright, the same as every
other `Forge` operation (`forge_github.go` `restClient()`, `forge_gitlab.go` `client()`),
so a resolver bug that hands `deskrun` a nil token still cannot reach the forge; and
`deskrun`'s own human-bound refusal trips on the ROSTER VALUE itself, before any mint is
attempted, which is a different signal in a different place from an unminted-token
failure. A binding that reads `human:<name>` and a custody path that fails to mint are two
distinct failure modes the design must not conflate: the first is "nobody automated this
on purpose," the second is "someone tried to and the credential was not there."

facts:
- GitHub's `actions: write` permission is the ONLY scope that grants both
  `workflow_dispatch` and `pending_deployments` approval, and it also grants
  `DELETE /repos/{o}/{r}/actions/runs/{run_id}`, `DELETE
  /repos/{o}/{r}/actions/runs/{run_id}/logs`, and disabling a workflow
  (`PUT .../actions/workflows/{id}/disable`) repo-wide. No fine-grained PAT or App
  permission separates "start/approve" from "cancel/delete/disable" — GitHub ships one
  scope for all of it. This is why no desk App holds it today and why the operation has
  fallen to a human's own `gh` session by default rather than by policy choice.
- `docs/streams/apps-installer/brief-03-deskapps-install-prove.md:78` independently states
  *"Never widen a grant, never edit an installation's permissions, never call `gh workflow
  run`"* for the desk-apps install-prove identity — the same conclusion this brief reaches
  for a different reason (there this is a permission-widening hazard for an installer
  identity; here it is that no App scope safely grants the operation at all). Neither rule
  supersedes the other; they agree.
- The GitHub Actions workflow-dispatch endpoint (`POST
  /repos/{o}/{r}/actions/workflows/{workflow_file}/dispatches`) returns **204 with no
  body** — it does not hand back the run it created. A caller that needs the run's own id
  (to poll it, or to approve a gate on it) must resolve it by a follow-up list read
  (`GET .../actions/workflows/{workflow_id}/runs`) filtered to runs created at or after the
  dispatch call, keyed by the actor and the workflow. More than one run matches when two
  dispatches race close together, which this brief treats the same way
  `OpenChangeForBranch` (brief 13) treats two open changes on one branch: an ambiguity is a
  could-not-check REFUSAL naming the ambiguity, never a silent newest-first guess — because
  a caller that resolved the WRONG run would then approve or read the state of a run it did
  not start.
- GitHub's deployment-gate approval (`POST
  /repos/{o}/{r}/actions/runs/{run_id}/pending_deployments`) takes an `environment_ids`
  array, not an environment NAME; the run's pending environments are read first
  (`GET .../actions/runs/{run_id}/pending_deployments`) and the name the caller supplied is
  resolved against that list. A name that matches none of the run's pending environments is
  a could-not-check REFUSAL naming the run and the requested gate — never an approval of
  whichever environment happened to be pending.
- GitLab exposes two ways to start a pipeline: `POST /projects/:id/pipeline` (needs a
  project token or a user token with at least Developer role — broad: the same credential
  can also read/write issues, merge requests, and repository content) and
  `POST /projects/:id/trigger/pipeline` authenticated by a **pipeline trigger token**
  (a credential that can start pipelines and nothing else — it carries no API scope beyond
  that one endpoint). The trigger token is the narrower credential for the one operation
  this brief needs, so it is the default; `POST /projects/:id/pipeline` is documented as
  the fallback for a deployment that genuinely needs a project-scoped identity (audit
  trail attribution to a named account, say), never adopted as the default for the same
  reason `repository_dispatch` is not adopted as GitHub's default below.
- GitLab has two gating shapes with no single unifying endpoint: a `when: manual` job
  (played via `POST /projects/:id/jobs/:job_id/play`, resolved from the pipeline's job
  list by name) on a protected branch, and a protected-environment multi-approval
  (`POST /projects/:id/deployments/:deployment_id/approval`, tier-gated — Premium+ per
  GitLab's own tiering, not measured live in this brief; see the open question below).
  Which shape a target project uses is a property of THAT project's CI configuration, not
  something `ApproveGate` can infer from the gate name alone — the caller (or the roster
  entry) states which shape applies.
- `github.com/repos/{o}/{r}/dispatches` (the `repository_dispatch` event) needs only
  `contents: write` to fire — a scope every desk App already holds for its own writes.
  That is precisely why it is NOT this brief's default trigger: any App holding
  `contents: write` — which is most of them — could fire a release-shaped event, which
  is a far wider "who can start a release" surface than the roster-bound, single-purpose
  credential this brief specifies. It is recorded here as the documented alternative for a
  future, more narrowly scoped use (a repository dispatch consumed by a workflow that
  itself re-checks the actor before doing anything release-shaped), not adopted now.
- The forge-qualified roster grammar (`identity.md`) already carries a `human:<slug>`
  identity token — `verifyrun`'s `executingRunner` emits it as the last fallback
  (`statusgen/verifyrun.go:654`) when no bot identity and no CI environment can be
  resolved. This brief's refusal condition reads the SAME token class from a per-repo
  run-credential binding, so a human-bound entry is recognisable by the identity layer
  this stream already established, not a new vocabulary invented here.
- `ForgeFor(repo ForgeRepo, role string) (Forge, error)` (brief 01,
  `tools/desk/internal/deskkit/forgeresolve.go:412`) is the one function that constructs a
  backend; `SetGitHubCustodyMinter` (`:282`) is the existing hook a verb registers its
  token-minting function through. `deskrun` reuses both — it does not open a second
  construction site or a parallel custody path.
- The `Forge` interface carries 30 methods today (`forge.go`, freshness-checked above);
  brief 13 was the most recent addition (rows 33-36 of
  `docs/streams/forge-gitlab/inventory.md`) and rows 37 (`RefExists`) is the current tail.
  This brief's three ops land at rows 38-40.
- `deskrun` has NO permit row in `tools/desk/internal/forgeban/allowlist.go` to retire: the
  operation it replaces (`gh workflow run`, a human clicking Approve in the Actions UI) has
  never been a desk-tool call site, tracked or otherwise — it is a human action today, not
  a banned one. This brief therefore does not move the ratchet; it closes a gap the ratchet
  was never counting.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. This applies doubly here: do not exercise `deskrun` against a real
  workflow or a real deployment gate while implementing or verifying this brief — every
  Verify row runs against a recording fake `Forge`, never a live GitHub Actions run or a
  live GitLab pipeline.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not add a generic/passthrough method, a new `gh`/`glab` shell-out, or a caller-supplied
  endpoint. `.github/workflows/forge-surface-control.yml` must stay green unchanged.
- Refusal, never fallback: a run-credential bound to `human:<name>`, an ambiguous run
  correlation, an unmatched gate name, or an unsupported GitLab approval shape are all
  could-not-check or refused — never a guess, never a silent GitHub-shaped default on a
  GitLab repo.
- `repository_dispatch` is documented, not implemented, in this brief. Do not wire it as a
  fallback trigger "for coverage" — that is precisely the widened surface the gate-why
  exists to keep out.

## Task
1. **Add three ops to `Forge`, both backends, each consumed by `deskrun` in this same
   change** (the brief-13 precedent: the freeze rule is amended, not bypassed).
   - `RunWorkflow(repo ForgeRepo, in RunWorkflowInput) (RunRef, error)` where
     `RunWorkflowInput` carries `Workflow` (the workflow file name), `Ref` (the branch/tag
     to run on), and `Inputs map[string]string`. `RunRef` is an opaque handle
     (`{ID, URL string}`) — a caller never constructs or parses one.
     - GitHub: `POST /repos/{o}/{r}/actions/workflows/{workflow}/dispatches`, then resolve
       the created run per the facts above (list + filter + ambiguity refusal). Record the
       correlation key used (actor + workflow + a timestamp floor taken BEFORE the dispatch
       call, never after) so a slow list read cannot miss the very run it just created.
     - GitLab: `POST /projects/:id/trigger/pipeline` with the trigger token, `ref`, and
       `variables[k]=v` per input, by default; document `POST /projects/:id/pipeline`
       (project/user token) as the alternative for a deployment that needs project-scoped
       attribution, per the facts above — not adopted as the default.
   - `ApproveGate(repo ForgeRepo, run RunRef, gate string) error`.
     - GitHub: read `GET .../actions/runs/{run_id}/pending_deployments`, resolve `gate`
       against the returned environment names, could-not-check REFUSE naming the run and
       the gate if none matches, then `POST .../actions/runs/{run_id}/pending_deployments`
       with that environment's id, `state: "approved"`.
     - GitLab: dispatch on the roster-declared gating shape for the target project — a
       `when: manual` job (resolve by name from the pipeline's job list, `POST
       /projects/:id/jobs/:job_id/play`) or a protected-environment approval (`POST
       /projects/:id/deployments/:deployment_id/approval`). A project whose declared shape
       does not match what the read finds is a could-not-check refusal, never a guess at
       the other shape.
   - `RunStatus(repo ForgeRepo, run RunRef) (*RunState, error)` — `RunState` carries
     `Status` (queued/in_progress/waiting/completed, forge-neutral vocabulary) and
     `Conclusion` (empty until concluded). GitHub: `GET .../actions/runs/{run_id}`.
     GitLab: `GET /projects/:id/pipelines/:pipeline_id`.
   Record all three in `docs/streams/forge-gitlab/inventory.md` (rows 38-40) with each
   row's GitLab mapping, and give each a both-backend golden contract case (the `/13`
   precedent: `TestForgeGithubGolden` / `TestForgeGitlabGolden` / `TestForgeGitlabCoverage`
   gain the new op tokens).
2. **The identity rule and the roster binding.** Add a per-repo run-credential binding
   (a new `ASSAY_*` key, registered in `rosterconfig.go`'s known-set per brief-01's
   pattern) whose value is either a `human:<name>` token (the identity.md vocabulary,
   already established by `verifyrun`) or the `release-runner` role — a new entry in the
   role-bindings vocabulary, bound the same way `worker`/`reviewer`/`verifier` are today
   (`role-bindings=…,release-runner=<slug-or-token-file>`). GitHub's `release-runner`
   binding is a dedicated, single-purpose App installation or fine-grained PAT an operator
   deliberately creates and roster-binds for this exact purpose — never the desk's existing
   worker/reviewer/verifier identity repurposed, because none of those is scoped for
   `actions: write`-adjacent operations and none should become so by accident. GitLab's
   `release-runner` binding is the pipeline trigger token, custody-rotated the way
   `desktoken gitlab` rotates a PAT today (`tools/desk/cmd/desktoken/gitlab.go:83,163`).
3. **`deskrun`, the verb.** `deskrun <owner/repo> <workflow> --ref <r> [-f k=v ...]`
   dispatches a run through `RunWorkflow` and prints the resolved `RunRef`; `deskrun
   approve <owner/repo> <run-id> --gate <name>` calls `ApproveGate`. Both resolve their
   forge via `ForgeFor` and their credential via the run-credential binding from task 2.
   **Before minting anything**, `deskrun` reads the resolved binding: if it is a
   `human:<name>` token, `deskrun` REFUSES (exit 5) naming the human and the repo, and
   prints that dispatching/approving this repo is a human action today — it does NOT
   attempt to read an ambient `gh`/`glab` credential, and it does not treat an unbound
   entry the same as a human-bound one (an unbound entry is a configuration refusal
   naming the missing key, exactly as brief-01's unconfigured-forge refusal does; a
   human-bound entry is a DELIBERATE state, not a gap). Only a `release-runner` binding
   proceeds to mint and call the backend.
4. **Wire `deskrun` into this repo's desk-verbs index.** Add its row to
   `tools/desk/README.md`'s Tool reference table (`## Tool reference`), naming its verbs,
   class (`outward write`), and write budget/breaker, in the same shape every other row in
   that table already carries.

## Verify (executable — no prose-only DoD items)

This brief follows brief 13's Class-column convention: `check:ci` runs in CI; `check` is a
local/grep-level assertion; `+dereference` resolves a claim rather than counting presence;
`+flow` exercises the cross-component path (verb → `ForgeFor` → backend) end to end;
`+mutation` is a mutation-demonstration row. Test names below are this brief's planned test
deliverables, created by the implementer:
`TestRunWorkflowAmbiguousCorrelationRefuses` (planned),
`TestApproveGateUnmatchedEnvironmentRefuses` (planned),
`TestDeskrunRefusesOnHumanBoundCredential` (planned),
`TestDeskrunRefusesWithoutMintedToken` (planned),
`TestDeskrunDispatchesThroughBackend` (planned),
`TestDeskrunApprovesThroughBackend` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskrun/... -count=1` | exit 0 — the new verb's own suite is green |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 — the seam grows three ops and stays closed |
| 4 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | exit 0 — `run_workflow` / `approve_gate` / `run_status` golden cases pin both backends' wire against RECORDED FIXTURES (no live API call in the test suite), and coverage reconciles the seam against inventory rows 38-40 |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestRunWorkflowAmbiguousCorrelationRefuses -count=1 -v` | **negative path**: two runs of the same workflow created within the correlation window → a could-not-check REFUSAL naming the ambiguity, never a newest-first guess |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestApproveGateUnmatchedEnvironmentRefuses -count=1 -v` | **negative path**: a gate name matching none of the run's pending environments → a could-not-check REFUSAL naming the run and the requested gate, never an approval of a different pending environment |
| 7 | check:ci +flow | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunDispatchesThroughBackend -count=1 -v && go test ./cmd/deskrun/... -run TestDeskrunApprovesThroughBackend -count=1 -v` | exit 0 — POSITIVE: with a `release-runner` binding and a minted token, `deskrun` dispatches/approves through the recording fake `Forge` (never a live call); the fake records exactly one `RunWorkflow`/`ApproveGate` invocation with the expected arguments |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesOnHumanBoundCredential -count=1 -v` | **negative path**: a `human:<name>` run-credential binding → `deskrun` REFUSES (exit 5) naming the human and the repo; the fake `Forge` records ZERO calls; no ambient `gh`/`glab` credential is read |
| 9 | check:ci | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesWithoutMintedToken -count=1 -v` | **negative path**: a `release-runner` binding whose custody token is absent → `deskrun` REFUSES, zero forge calls — the backends' existing unminted-token refusal, exercised through this verb |
| 10 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | exit 0 — the new run-credential config key and the `release-runner` role are registered in the roster's known-set (an unregistered key fails the fleet closed, per brief-01's task 4) |
| 11 | check | `grep -c 'deskrun' tools/desk/README.md` | ≥ 1 — `deskrun`'s row exists in this repo's desk-verbs index (`## Tool reference`), not merely described in this brief |
| 12 | check:ci +mutation | **Mutation demonstration for the human-bound refusal.** In the run-credential resolver, disable the human-token check (make it always fall through to the `release-runner` path) — then `cd tools/desk && go test ./cmd/deskrun/... -count=1`; restore the file and re-run | exit **1** on the mutant: `TestDeskrunRefusesOnHumanBoundCredential` reddens (a human-bound repo would now dispatch through whatever `release-runner` binding exists, or attempt an ambient mint) — exit **0** again after restoring. Proves the refusal is a live control, not a check nothing exercises |
| 13 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/14` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |
| 14 | check | GitLab dry-run row | **could-not-check-by-design**: this repo carries no `.gitlab-ci.yml` and no CI-reachable GitLab project (freshness fact above; `gitlab-ci-half.md` templates a pipeline for ADOPTERS, it is not a fixture this repo's own CI can run against) — there is no CI-reachable GitLab fixture to dry-run `RunWorkflow`/`ApproveGate` against from inside this repo's test suite. Rows 4-9 cover the GitLab mapping against recorded fixtures; a live dry run is deferred to whichever brief next extends the GitLab live pilot (`docs/streams/forge-gitlab/pilot-report.md`) to cover run/gate operations, and is named here rather than silently skipped |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| `deskrun` silently mints and reads an ambient `gh`/`glab` credential when the roster binding is a human | row 8 — zero forge calls, exit 5, naming the human |
| The human-bound refusal is implemented but nothing exercises it, so it silently rots | row 12, the mutation demonstration |
| `RunWorkflow`'s run-correlation picks the wrong run when two dispatches race, and a later `ApproveGate` approves the wrong one | row 5 |
| `ApproveGate` approves whichever environment happens to be pending rather than the one named | row 6 |
| `repository_dispatch` creeps in as a convenience fallback trigger, widening who can start a release | **no row** — enforced by the Ground rules and the Review gate reading the diff for it; a code search finding a `dispatches` POST to the *repository* dispatches endpoint (as opposed to the *workflow* dispatches endpoint) is a Review-time finding, not a Verify row, because there is no negative behavior to provoke against code that was never written |
| GitLab's broader `POST /projects/:id/pipeline` becomes the default instead of the narrower trigger token | review-only — rows 4/7 exercise whichever path the implementation took; the Review gate reads the diff against the stated default |
| The new roster key is unregistered, failing the fleet closed the moment anyone sets it | row 10 |
| `deskrun` never appears in this repo's own verb index, so an operator does not know it exists | row 11 |
| A live GitHub Actions run or GitLab pipeline is exercised during implementation or verification | **no row** — a Ground-rules violation, not a code defect; caught only by review of what commands were actually run |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date
in the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between "a workflow starts, or a gate clears" and the wrong
   actor doing either? (The roster's run-credential binding, read before any mint is
   attempted.) Is it acceptable alone? (No — the backends' unminted-token refusal is the
   second, independent layer: even a resolver bug that misreads the binding still cannot
   reach the forge without a real token.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed?
   (Row 9: with no minted token, the backend itself refuses regardless of what the
   human-bound check decided; row 12's mutation proves the human-bound check is live, not
   vacuous.)
3. Is `repository_dispatch` correctly left undocumented-as-default, and is the GitLab
   trigger token correctly the default over the broader project/user token? (Both are
   stated in `facts:` and Task 1; this is the security judgment the gate exists for — a
   model does not self-certify that a narrower credential was actually chosen over a
   broader one that would have made the happy-path tests pass identically.)
