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
- **Amended 2026-09-23 (correction found at review; the bullet above and the `why:` are left
  as ratified).** `actions: write` grants `workflow_dispatch` but NOT `pending_deployments`
  approval. Approving a pending deployment needs `Deployments: write`, and GitHub lets only
  the environment's required reviewers approve — required reviewers are users or teams, never
  an App. Under the `release-runner` App credential the GitHub approve path therefore cannot
  succeed: the forge reports `current_user_can_approve: false` and `ApproveGate` refuses as
  could-not-check before any write. Whether that path ships as a documented could-not-check or
  is withdrawn is for the human who ratified this brief to decide on its decision issue.
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskrun/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRunWorkflowAmbiguousCorrelationRefuses -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestApproveGateUnmatchedEnvironmentRefuses -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunDispatchesThroughBackend -count=1 -v && go test ./cmd/deskrun/... -run TestDeskrunApprovesThroughBackend -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesOnHumanBoundCredential -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | pass exit=0 | sha256:25ab0e8c73c4 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c 'deskrun' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 12 | `release-runner` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --consumers --brief forge-neutral/14` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 14 | `GitLab dry-run row` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:da0ee8884549 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |

**Verifier notes — 2026-09-28, assay-verifier-app[bot] (model claude-opus-5-5) (on-behalf-of human:ian), merged main c50a38fc12518a4eec4db37e8dd847d49e79149a; implemented by #1563 (squash e6b8dee7f85e3bf1bdf3035b0f20a6a411698d3a).**
Gate is human (`sensitive-data: yes`): Evidence only, status stays `implemented`; a model does not sign this off.

Witness above: pinned statusgen v1.0.27 (darwin-arm64 binary, sha256 matching the release pin), run directly (no wrapper; the roster config was read for the Runner cell, no forge credential was in the environment), under macOS `sandbox-exec` with the network denied except loopback, `GOFLAGS=-count=1`, a fresh GOCACHE and `GOPROXY=off`. Every check:ci row (1-9, 12, 13) recorded could-not-run because this darwin host has no network-off sandbox (`unshare --net`) — tracked by #1800; they clear on a Linux runner. Row 10 is a genuine pass. Row 11 is a genuine pass (output hash decodes to "1", so the count is at least 1).

Hand-run targeted package tests (same sandbox and env; loopback-only profile; no cached replay), recorded here as Evidence the witness could not give on this host:

- Row 1: `go build ./...` exit 0. The full `go test ./...` suite was not run by this verifier (scope limited to targeted package tests); the witness's could-not-run (#1800) stands for that half.
- Row 2: `go test ./cmd/deskrun/... -count=1` exit 0 (ok, not cached).
- Row 3: both runs exit 0; `TestForgeNoPassthrough` subtests include "method set equals the committed inventory".
- Row 4: all three exit 0. GitHub goldens `run_workflow`, `run_workflow_refuses_*` (2), `approve_gate`, `approve_gate_credential_…`, `approve_gate_refuses_…` (manual-job shape), `run_status`, `run_status_waiting` pass; GitLab goldens `run_workflow`, `approve_gate_manual_job`, `approve_gate_environment`, `approve_gate_refuses_…` (undeclared shape), `approve_gate_manual_job_…` (unmatched), `run_status`, `run_status_failed` pass. Coverage: "committed inventory … reconciles: 54 operations, all covered".
- Row 5: exit 0 — "ambiguous run correlation … 2 runs (ids 601, 600 …) match the correlation key … Refusing to guess which one this dispatch created".
- Row 6: exit 0 — subtests github and gitlab both pass.
- Row 7: both exit 0 — "dispatched through the recording fake: … run 4242 (inputs: dry_run,version)"; approve passes for the gitlab declared-shape and github cases.
- Row 8: exit 0 — dispatch, approve and status each "refused (exit 5) with zero forge calls: … bound to human:ada … Nothing was minted and no ambient credential was read".
- Row 9: exit 0 — subtests github mint-fails, github empty-token, gitlab no-custody-file pass.
- Row 10: exit 0 (also the witness pass).
- Row 12 (mutation, run by hand on a copy of the module outside the checkout): the resolver's human check changed to `if false && cred.Human != "" {` in the run-credential resolver → `go test ./cmd/deskrun/... -count=1` exit 1, the human-bound refusal test FAILs; source restored → exit 0. The mutation is live.
- Row 13: pinned `statusgen --root . --consumers --brief forge-neutral/14` on merged main → exit 2, COULD-NOT-CHECK "not in the diff against c50a38fc…". Re-run at the implementing commit with `--base` its parent → exit 0 but "0 corroborated, 0 disproved, 6 unchecked": the routing claims were written by the brief-authoring commits, not by #1563's diff, so that gate carries no evidence either way. Dereferenced by hand instead: #1563's diff touches every "follow-up forge-neutral/14" path (forge.go, cmd/deskrun, rosterconfig.go, the forge-gitlab inventory, the tools/desk README), and the forgeban allowlist is untouched (its "out-of-scope" claim holds).
- Row 14: could-not-check by design, as the brief states (no CI-reachable GitLab fixture in this repo). The witness exit 127 is the prose row.
- No test wrote artifacts into the source tree (`git status` clean after every run).

Observations:
- Row 12 check definition: the witness takes the command cell's FIRST code span, which in row 12 is `release-runner`. So on a Linux runner it would run `release-runner` (exit 127), not the mutation. The row needs a runnable command (for example, a mutation harness over the committed mutations file) before any witness can prove it.
- Row 12 mechanics: under the mutant the verb STILL refused with exit 5 and zero forge calls. The verb's own fail-closed check ("binding is not the release-runner role") caught it, and the test reddened on its message assertion ("human action" not named). So a disabled resolver check does not in practice make a human-bound repo dispatch: there is a third layer beyond the two the SPOF note names.
- Row 4's Expect and the consumers claim say inventory rows 38-40. The ops landed at rows 49-51 because rows 38-48 were taken first; the inventory records this. The brief text is stale, not the code.
- The GitHub approve arm is a documented could-not-check under an App credential (`current_user_can_approve` false → refused before any write). The human's ruling on #1556 (2026-09-23) chose to ship it that way.
- Task 2 said the GitLab release-runner credential is "custody-rotated the way desktoken gitlab rotates a PAT". The implementation instead REFUSES self-rotation for that role (a pipeline trigger token has no self-rotate endpoint; rotation is an operator act, and `--no-rotate` verifies custody). This departs from the brief's wording; the human should confirm it.
- No POST to the repository-dispatch endpoint exists; the only `dispatches` POST is the workflow-dispatch path. GitLab starts pipelines through the trigger endpoint with no access-token header.

Risk-bearing values — enumeration over #1563's non-test diff (the deskkit forge seam and both backends, the run-credential resolver, rosterconfig in both readers, cmd/deskrun, desktoken):
`ReleaseRunnerRole = "release-runner"` (runcredential.go:46); `humanRunBindingPrefix = "human:"` (runcredential.go:50); `runBindingNameRe` (runcredential.go:67); `EnvRunCredentials = "ASSAY_RUN_CREDENTIALS"` (tools/desk/internal/deskkit/rosterconfig.go:397, mirrored in statusgen/rosterconfig.go:381); `GateShapeEnvironment = "environment"`, `GateShapeManualJob = "manual-job"` (forge.go:1213, 1216); `ghRunCorrelationAttempts = 5`, `ghRunCorrelationWait = 3 * time.Second` (forge_github.go:2529-2530); `ghWorkflowFileRe` (forge_github.go:2538); `floor := g.clock().UTC().Truncate(time.Second)` (forge_github.go:2609); `per_page = "100"` (forge_github.go:2623); `"state": "approved"` (forge_github.go:2741); `gitlabPipelineDefinition = ".gitlab-ci.yml"` (forge_gitlab.go:3998); the trigger path `/projects/%s/trigger/pipeline` (forge_gitlab.go:4056); `PerPage: 100` (forge_gitlab.go:4109, 4139); `Status: "blocked"` (forge_gitlab.go:4141). Ranked by irreversibility (the act is starting a release run or clearing a deployment gate as the wrong actor): human prefix, release-runner role, correlation floor, approve state, GitLab trigger path, roster key, GitHub page size, GitLab page size. Last and reversible (no derivation needed): the correlation attempts/wait, gate-shape strings, the workflow-name regex, the pipeline-definition name, "blocked".

- RISK-VALUE: DERIVED — humanRunBindingPrefix = "human:" @ tools/desk/internal/deskkit/runcredential.go:50 — the same `human:<slug>` token class verifyrun emits (statusgen/verifyrun.go:986) and identity.md defines, as the brief requires. It is matched before any mint (row 8: zero forge calls), and the mutation proves it is live (row 12).
- RISK-VALUE: DERIVED — ReleaseRunnerRole = "release-runner" @ tools/desk/internal/deskkit/runcredential.go:46 — the role Task 2 names. Any binding value other than `human:<name>` or this role invalidates the whole key. A release-runner App slug shared with another desk role is refused, so the release credential is never a worker, reviewer or verifier App repurposed.
- RISK-VALUE: NAMED, NOT DERIVED — floor := g.clock().UTC().Truncate(time.Second) @ tools/desk/internal/deskkit/forge_github.go:2609 — the floor is taken before the dispatch and rounded down, so it can only widen the window against the LOCAL clock. Two things are not derived. First, the forge list lag and host-to-forge clock skew: a host clock ahead of the forge excludes the new run, which fails closed as could-not-check after 5 reads. Second, the single-hit acceptance: if two same-actor dispatches of one workflow and ref race, and only the OTHER run is listed at the first read, that run is returned as this dispatch's run. The ambiguity refusal fires only when both runs are visible in one read. Deriving this needs forge-side ordering and consistency guarantees that cannot be established offline. Open question for the human: is same-actor concurrent dispatch acceptable as-is, or does it need serialising (for example, one in-flight deskrun dispatch per repo and workflow)?
- RISK-VALUE: DERIVED — "state": "approved" @ tools/desk/internal/deskkit/forge_github.go:2741 — sent only for the single environment id whose name exactly matches the requested gate, and only when the forge reports `current_user_can_approve`. Otherwise it is a could-not-check refusal before any write (row 6, and the credential-cannot-approve golden). Shipping this arm as a documented could-not-check is the human's 2026-09-23 ruling on #1556.
- RISK-VALUE: NAMED, NOT DERIVED — GitLab trigger path "/projects/%s/trigger/pipeline" @ tools/desk/internal/deskkit/forge_gitlab.go:4056 — the code does take the trigger endpoint, with a client that carries no access-token header. The brief's premise is that a pipeline trigger token can start pipelines and nothing else, which makes it narrower than a project or user token. That premise is a GitLab product fact that cannot be checked offline, and the brief's gate-why reserves this judgment to the human (review question 3).
- RISK-VALUE: DERIVED — EnvRunCredentials = "ASSAY_RUN_CREDENTIALS" @ tools/desk/internal/deskkit/rosterconfig.go:397 — the same literal is registered in the statusgen reader (statusgen/rosterconfig.go:381). Both readers share one roster, and an unknown ASSAY_ key refuses the whole configuration (row 10).
- RISK-VALUE: DERIVED — per_page = "100" @ tools/desk/internal/deskkit/forge_github.go:2623 — the list is newest-first. If the forge reports more runs than the page holds and any hit exists, the result is refused as ambiguous, never picked. A truncated page with no hit exhausts to could-not-check. Neither outcome returns a wrong run.
- RISK-VALUE: DERIVED — PerPage: 100 @ tools/desk/internal/deskkit/forge_gitlab.go:4109 and 4139 — no pagination, but a gate that is not on the first page yields zero matches, which is a refusal. Manual-job names are unique within one pipeline, and the deployment read is narrowed to this pipeline id, so page truncation can only fail closed.

VERIFY: BLOCKED — on this darwin host the witness could prove only rows 10-11 (2/14). Rows 1-9, 12 and 13 are check:ci could-not-run (#1800). Row 12's command cell also cannot be witnessed as written (its first code span is `release-runner`). Row 14 is could-not-check by design. Every hand-run targeted package test and the hand mutation (row 12) behaved as the Verify table expects. No row FAILED. Human gate: two NAMED, NOT DERIVED values (the correlation floor and single-hit acceptance; the GitLab trigger-token default) and the GitLab release-runner rotation departure are open questions for the human.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskrun/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRunWorkflowAmbiguousCorrelationRefuses -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestApproveGateUnmatchedEnvironmentRefuses -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunDispatchesThroughBackend -count=1 -v && go test ./cmd/deskrun/... -run TestDeskrunApprovesThroughBackend -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesOnHumanBoundCredential -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskrun/... -run TestDeskrunRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | pass exit=0 | sha256:48627b580af8 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c 'deskrun' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 12 | `release-runner` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --consumers --brief forge-neutral/14` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 14 | `GitLab dry-run row` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:8f49b6f98509 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-09-28 at batch tree e1afb99a after main changed `tools/desk/internal/deskkit/forgeresolve.go` and `tools/desk/README.md` — non-implementer verifier (verify-desk, model claude-opus-5-5).** The witness table directly above ran at e1afb99aca990bbdb14ef44eb65fceb0610b18e8, which is merged main 02a2f75fb532af2bf7bc6284e3f5f4c6e9337f48 plus this batch's Evidence commit (main re-read with the verifier App token: still 02a2f75fb). Envelope: the pinned statusgen v1.0.27 binary run directly (sha256 matches the pin), `env -i` with the real HOME for the Runner cell, PATH reduced to the system dirs plus a scratch dir holding only `go` and `statusgen`, KUBECONFIG=/dev/null, GOFLAGS=-count=1, a fresh empty GOCACHE, GOPROXY=off, GOTOOLCHAIN=local, under macOS `sandbox-exec` with the network denied except loopback. No forge credential was in the environment. gate: human: Evidence only, status stays `implemented`.

Drift assessment: both drifted files changed in exactly one main commit since the previous run (c50a38fc), 31d2ad536 (#1650, issue #1631). In forgeresolve.go it ADDS one function, `GitHubTokenIdentityForRepo` (an identity probe for an inherited token); its only caller is `cmd/deskdispatch`. No existing function in the file changed, and deskrun, the run-credential resolver and the run/gate ops do not call it. In the README it rewrites one paragraph of the deskdispatch section (verified-export precedence for an inherited GH_TOKEN); the `deskrun` line is untouched (row 11 still counts 1). The change does not touch this brief's behaviour.

Per row, vs the previous run (c50a38fc):
- Rows 1-9, 12, 13: witness could-not-run again, check:ci on a darwin host (#1800). Targeted package tests, run directly in the same envelope (the whole module `go test ./...` was NOT run):
  - Row 1: `go build ./...` exit 0.
  - Row 2: `go test ./cmd/deskrun/...` exit 0, all seven deskrun tests PASS.
  - Row 3: both `-run` names exit 0.
  - Row 4: all three exit 0; coverage "committed inventory … reconciles: 54 operations, all covered" (unchanged).
  - Row 5: exit 0, the ambiguous-correlation refusal names 2 runs (ids 601, 600).
  - Row 6: exit 0, github and gitlab subtests PASS.
  - Row 7: both exit 0 (dispatch run 4242 and approve through the recording fake).
  - Row 8: exit 0, dispatch/approve/status each refused exit 5 with zero forge calls.
  - Row 9: exit 0, subtests github mint-fails, github empty-token and gitlab no-custody-file PASS.
  - Row 12: the witness executes the first code span `release-runner`, not the mutation (#1805). The mutation was run directly in this run's own worktree: the resolver's human check at runcredential.go:155 changed to `if false && cred.Human != "" {`, then `go test ./cmd/deskrun/... -count=1` exit 1 (the human-bound refusal test FAILs because the refusal no longer names the human; the verb still refuses exit 5 with zero forge calls, via its own "not the release-runner role" check, as the previous run observed); the file restored by path-specific checkout, re-run exit 0. Same as before.
  - Row 13: pinned `statusgen --root . --consumers --brief forge-neutral/14` now exits 0 (previously exit 2) because the batch tree sits one commit ahead of main, so the base resolves to 02a2f75fb; the result is still vacuous, "0 corroborated, 0 disproved, 6 unchecked" (#1281). The previous run's hand dereference against #1563 still stands; nothing in the drift touches those paths except the README paragraph above.
- Row 10: witness pass exit=0 again and genuine (a real test run under the envelope, no cached replay). The output hash moved (25ab0e8c73c4 to 48627b580af8) because `-v` prints timings; the direct run shows `--- PASS` for the known-key-set test.
- Row 11: witness pass exit=0 again, hash 4355a46b19d3 unchanged, which decodes to `1`, so the count is at least 1. Genuine.
- Row 14: witness could-not-run exit=127 again, prose row (#1805), could-not-check by design as the brief states. The output hash moved (da0ee8884549 to 8f49b6f98509) most likely because the not-found message names the shell, and this run's reduced PATH no longer carries the package-manager bin dir the previous run had.
- No run left artifacts in the tree: `git status` showed only this brief modified.

Unchanged from the previous run: the RISK-VALUE lines and open questions above (correlation floor and single-hit acceptance; the GitLab trigger path; the release-runner rotation departure). The drift adds no literal in this brief's diff scope.

VERIFY: BLOCKED — at e1afb99a the witness genuinely proves rows 10-11 (2/14). Rows 1-9, 12 and 13 are check:ci could-not-run on darwin (#1800), row 12's cell also cannot be witnessed as written (#1805), row 13 is vacuous on merged main (#1281), and row 14 is could-not-check by design. Every direct targeted test and the direct mutation behaved as the Verify table expects; no row FAILED.

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
