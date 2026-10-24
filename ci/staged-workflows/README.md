# Staged workflows — pending human promotion

CI workflow files under `.github/workflows/` are added and changed by a **maintainer**, not
by automation: no bot or App in this project holds workflow-push permission, and GitHub
hard-rejects any App push that creates or updates a `.github/workflows/*` file. That is a
deliberate boundary — an identity that can rewrite CI is a supply-chain surface — so a
prepared workflow lands here first, in the ordinary pull-request flow, and a maintainer
**promotes** it into place.

## Promoting a staged workflow

```
git mv ci/staged-workflows/<name>.yml .github/workflows/<name>.yml
git commit -m "ci: activate <name> workflow"
git push
```

The push that lands the file under `.github/workflows/` must come from a maintainer's
credential (or a narrowly-scoped promote credential), which is what makes activation a
human's act. Until a file here is promoted it runs nowhere; the staged copy is the
reviewable artifact, not a run.

## Contents

- `verify-gate-close.yml` — live workflow twin with the board-writer
  Environment restriction, byte-identical to its live counterpart, enforced by
  `declared-changes.json`.

- `assay-qualgen.yml` — live workflow twin with the board-writer Environment
  restriction, plus the classified board push (#2910): the QUALITY.md commit step
  sources `statusgen/pushcred.sh`, preflights the push credential (a dry-run push)
  before the commit and before each retry, and retries only a non-fast-forward
  rejection; an expired, revoked, under-scoped or otherwise refused push fails once
  with its own line and never reads as "main moved". **Pending promotion**
  (`declared-changes.json` mode `pending`); its push step is tested by
  `statusgen/pushcred_test.go`.

- `evidence-automerge-tests.yml` — offline refusal-classifier, step-shell and poll suites,
  including the errexit status-capture class guard across staged, activation and live
  workflow copies. The live `ci.yml` build-test job already runs all three suites
  through `tools/desk/internal/regression/evidenceautomerge_test.go` on every PR
  and push. This optional standalone workflow is **staged, not live**; promotion
  would add a separate check for changes under those workflow directories or
  `tools/evidence-automerge/`.

- `evidence-automerge.yml` — the Evidence-PR auto-merge lane. Unlike the other files here
  this one is **already live**: it was promoted, and the copy in this directory is kept
  as the reviewable edit surface, because no App may push a `.github/workflows/*`
  file. The board-writer Environment/poll migration is included in the live
  file. This copy includes protected completion for already-clean Evidence PRs
  and is identical to the live workflow; see `tools/evidence-automerge/README.md`.
  Every change to that lane
  therefore lands here first, and a maintainer re-promotes it by copying the file over the
  live one (`cp ci/staged-workflows/evidence-automerge.yml .github/workflows/evidence-automerge.yml`)
  in a separate maintainer-credentialled commit. Until that copy lands, the merged change
  is inert: the running workflow is still the old file.
- `truth-suite.yml` — the standing truth suite (`docs/test-policy.md` § "Standing truth
  suite"): the test corpus plus the release mutation gate, on a nightly schedule and on manual
  dispatch, reporting three-state. **Already live** — promoted to
  `.github/workflows/truth-suite.yml` on 2026-09-10; a change to it is authored here first
  and re-promoted by a maintainer commit. **Pending promotion (#3292):** the staged twin adds
  `internal/cellop/mutations.json` (the `cellctl` plan/apply guards) to the `mutation-gate`
  matrix, which no workflow runs today; it was otherwise byte-identical to the live file when
  staged, and `truth-suite.yml.pending` classifies the difference. Because a `pending` twin is
  no longer compared with live by the drift guard, a change made to the live file on main
  after staging would be silently reverted by a bare copy. So promote it in two steps, in a
  maintainer-credentialled commit: first review the full `diff -u` against live and confirm it
  shows only the cellop matrix line and its comment block (anything else is a live change
  made since; re-base the twin on the live file, three-way, before copying), then copy it
  over the live one:
  ```
  diff -u .github/workflows/truth-suite.yml ci/staged-workflows/truth-suite.yml
  cp ci/staged-workflows/truth-suite.yml .github/workflows/truth-suite.yml
  git commit -m "ci: promote truth-suite.yml"
  git push
  ```
  Until then the cellop sweep runs only by hand
  (`cd tools/desk && go run ./cmd/muhar -j 0 -spec internal/cellop/mutations.json`); its
  anchors are still dry-run on every PR by the muhar stale-anchor guard.
- `assay-statusgen.yml` — a STAGED TWIN of the live `.github/workflows/assay-statusgen.yml`
  carrying the decision-gate hold's network layer (`spec/lifecycle-v1.md` §4.5), alongside
  the classified board push (#2910) described below, which `declared-changes.json` classes
  `pending`: the
  corroborate job gains `issues: read` and a `statusgen --decision-gate --pr <N>` step after
  the corroborate step. The step has no shell path pre-filter: the binary decides from the
  PR's own file listing whether the board (`docs/streams/` or `docs/archive/`) was touched, and
  it exits 0 with a notice while the base-built statusgen predates `--decision-gate`. **Pending promotion**:
  re-base on the live file (three-way) and copy it over the live one in a
  maintainer-credentialled commit. Promoting it does NOT make the check required; that is a
  separate branch-protection act. Until promotion, the hold's offline layer still runs in
  `--lint`; the live corroborate step's `--corroborate --pr` runs the network lane only on PRs
  that touch a findings entry; and `deskevidence` / `--close-verify` run the network check
  themselves.
- `verify-gate-close.yml` — a STAGED TWIN of the live `.github/workflows/verify-gate-close.yml`
  carrying ONLY a decision-gate refusal branch in the "Advance ..." step. When
  `statusgen --close-verify` refuses a close with the literal "decision-gate hold", the branch
  relays the refusal, reopens the card and commits nothing. `TestDecisionGateCloseWorkflowKeyedPhrase`
  in `statusgen` pins the phrase on this staged copy, or on the live file once it is gone.
  **Pending promotion**, the same way. Until then `--close-verify` still refuses the close and
  the live workflow commits nothing, but the refusal reaches its generic first-attempt branch.
  That branch leaves the card closed and comments a reason that does not name the decision
  issue or the remedy.
- `loopadmin.yml` — the loopadmin module's test job (graph-execution/20): `gofmt`, `go vet`,
  `go test` and the mutation gate (`go run ./mutate`) from `loopadmin/`, on path triggers for the
  module, `spec/loop-admin-runner-v1.md` and `schemas/loop-admin-runner-v1.json`. **Staged, not
  live** — a maintainer promotes it to `.github/workflows/loopadmin.yml`. Until then `ci.yml`'s
  build-test job still builds and vets the module (it discovers every `go.mod`) but runs no tests.
- `workspace.yml` — the shared workspace contract module's job (`spec/cell-workspaces-v1.md`):
  `gofmt`, `go vet`, `go test` and the mutation gate (`muhar -spec mutations.json`, harness built
  from `tools/desk`) from `workspace/`, on path triggers for the module, its spec, its schema and
  `docs/workspace-callers.md`. **Staged, not live** — a maintainer promotes it to
  `.github/workflows/workspace.yml`. Until then `ci.yml` builds and vets the module, and the desk
  suite runs its tests on every change (`TestWorkspaceModuleSuite` in
  `tools/desk/internal/regression`); only the mutation gate runs nowhere.
- `cellcache-mutate.yml` — the cell Go-cache mutation gate (#3148). Runs
  `python3 internal/cellcache/mutate.py --shard i/3` from `tools/desk` on path triggers for the
  cache, process and launcher packages, and fails on any `MISSED` mutant and on any
  `COULD-NOT-MUTATE` one (a mutant that does not compile, a `file` that escapes the module, a
  stale anchor, a suite timeout). No existing job runs it: the suite takes about ten minutes, far
  past what `ci.yml`'s `build-test` job should absorb per PR. That job instead runs the
  `TestMutateGate*` tests, which prove each state the gate reports against a throwaway module.
  **Staged, not live** - a maintainer promotes it (`git mv ci/staged-workflows/cellcache-mutate.yml
  .github/workflows/cellcache-mutate.yml`); until then the gate is run by hand, as
  `docs/cellctl-go-cache.md` ("Mutation gate") says.
- `forge-surface-control.yml` — twin of the live forge-surface job, which carries `forgeread/**` in
  both path filters (`pull_request` and `push`) and a step running the shared fact reader's
  controls (the module's tests, the ambient-token rule over `forgeread/`, the lower-layer scope
  check, statusgen's import-path guard), and notes that `forge-ban.sh` exits non-zero on a
  `forgeread` site. **Already live** — a maintainer promoted it, and the manifest declares it
  `identical`. The same maintainer commit applied the forgeread part of the `assay-statusgen.yml`
  staged change to the live file: the `forgeread/**` path filters, and the base-tree archive that
  carries the sibling module, without which the corroboration guard cannot build a base that
  already has `forgeread/`. The rest of that staged change (the hourly `reconcile` job and the
  board-writer Environment) was promoted with it, as described below.
- `assay-statusgen.yml` — perfrec part (#3101): the staged copy also carries the `perfrec/**`
  path filters (`pull_request` and `push`) and a base-tree archive that carries the sibling
  `perfrec/` module beside `statusgen/`, for the same reason as the forgeread part above:
  statusgen's `go.mod` replaces `perfrec` with `../perfrec`, so without it the corroboration
  guard cannot build a base that already has the module. **This part is promoted** into the
  live `.github/workflows/assay-statusgen.yml` in the same pull request that adds `perfrec/`
  (#3113); the rest of the staged copy (#2910) stays pending.
- `assay-statusgen.yml` — full-history regen part (#3364): the staged copy's `regen` job checks
  out with `fetch-depth: 0`, as the `lint` job already does. The regen runs the same history-reading
  checks as `--lint` (the consumed-changelog-fragment exemption, the Evidence staleness read), and
  on a shallow clone those are could-not-check by design, so a release that consumes a fragment a
  brief names turns the default-branch regen red while the PR lint passes. **Live**: the same
  one-line `fetch-depth: 0` landed on its own in the live `regen` job's checkout `with:` (#3385),
  and this copy's line is now byte-identical to it; the rest of the staged copy (#2910) stays
  pending. `TestBoardRegenFullHistory` in `statusgen` pins the line on the live file, this copy
  and the `tools/ci-load/activation/` copy.
- `gremlins-daily.yml` — the daily Gremlins mutation sweep (#2647) over the desk tools'
  muhar-spec packages (`tools/gremlins/run.sh`). **Discovery, never a gate**: surviving
  mutants go to the job summary and the uploaded JSON artifact as leads, and survivors in
  code no muhar spec covers are listed as candidate guards with file:line. The job fails
  only when the sweep itself broke (install or pin check, a Gremlins failure, a package red
  in the mutant copy layout, or no evidence at all). Daily cron at 03:43 UTC plus
  `workflow_dispatch`, `contents: read`, one concurrency group, an estimated 60 to 80 runner-minutes a
  day on the 2-CPU public runner. See `tools/gremlins/README.md`. **Staged, not live** —
  a maintainer promotes it:
  ```
  git mv ci/staged-workflows/gremlins-daily.yml .github/workflows/gremlins-daily.yml
  git commit -m "ci: activate gremlins-daily.yml"
  git push
  ```
- `winparity.yml` — the Windows-build ↔ Makefile target-parity gate (#665). Runs
  `cd tools/winparity && go run . --root ../..` on the self-hosted `medici-builder-public` runner
  (hand-installed Go, no `make`), asserting that `scripts/build-windows.ps1`'s declared target set
  equals the root `Makefile`'s `.PHONY` set — so a target added on one side and not the other
  reddens rather than shipping a Windows build that fell behind the Unix target set. The Windows
  script runs the same `tools/winparity` guard as a preflight; this leg is the Linux-CI half, so a
  Makefile-only edit (the change that never runs the Windows script) is still caught. Promote it to
  `.github/workflows/winparity.yml` to activate.
- `windows-ci-leg.yml` — **retired** (#4106), with the live `.github/workflows/windows-ci-leg.yml`,
  `celllaunch-windows.yml`, `git-maintenance-windows.yml` and
  `scripts/windows-bootstrap-hashcheck-smoke.ps1`. Every CI job that needed a native Windows runner
  is gone, and so are the other jobs that lived in that file. Nothing in CI now executes on native
  Windows: not the desk suite, the `statusgen --lint`/`--version` smoke, the Windows PowerShell 5.1
  parse check, or the bootstrap's runtime sha256 refusal of a tampered download. What remains
  runs on Linux: the Windows release assets are still cross-compiled and checksummed in
  `release.yml`, `ci.yml`'s `desk-platform-compile` job vets the desk tools and compiles their
  test binaries for `windows/amd64` without running them, `winparity.yml` (above) still guards the Windows build's target set, and
  `TestPS1EncodingIs51Safe` in `tools/desk/internal/deskkit` (run by `ci.yml`'s build-test job)
  still fails any `*.ps1` carrying a byte above 0x7F without a UTF-8 BOM. That byte scan is not a
  parser: any other 5.1-only parse error is no longer caught by CI.

- `release-on-merge.yml` — the "release by merge" merge-detector (iso-9001/07). **New,
  pending promotion.** On `push: branches: [main]` it detects that the pushed commit is the
  merge of a release PR (title `release: vX.Y.Z` + a `RELEASE: vX.Y.Z` body line, merged with
  `merge_commit_sha` == the pushed commit), mints a release-cutter App token, and creates
  `refs/tags/assay/vX.Y.Z` then `refs/tags/vX.Y.Z` at the merge commit via the git-data API. The
  plain `vX.Y.Z` tag — created by an App installation token, a distinct identity — triggers
  `release.yml`'s `push: tags: ['v*']` build; a GITHUB_TOKEN-created tag would NOT (GitHub's
  recursion guard). This job's own GITHUB_TOKEN stays read-only. **Prerequisite (repo-admin
  act):** a release-cutter GitHub App (contents:write; no workflows/actions/administration
  write) with its key wired as the Actions secrets `RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY`
  (a human may instead point these at the existing board-writer App's `BOARD_APP_*`). Promote it:
  ```
  git mv ci/staged-workflows/release-on-merge.yml .github/workflows/release-on-merge.yml
  git commit -m "ci: activate release-on-merge.yml"
  git push
  ```
- `release.yml` — an identical twin of the live `.github/workflows/release.yml`.
  One approved `release` job owns image pushes, tag creation and GitHub release
  publication on the dedicated Docker-capable release runner. The former
  `docker-publish.yml` workflow and its pending companion are retired. Changes
  to the twin and live workflow must land together through the maintainer lane.
  The separate `release-on-merge.yml` above remains inactive.
- `assay-statusgen.yml` — the live board workflow plus the hourly `reconcile` job
  (derived-board/04), landed as generated output per the ruling on assay#2688
  (desk-tools/36): a `schedule:` trigger guarded to this repository (a fork's schedule
  never runs it), a read-only token (`contents`, `pull-requests`, `issues`: read) for
  the PR-witness reads, and any generated stream-README change landed DIRECTLY on the
  default branch — one commit, fast-forward only, no pull request. The job runs in two
  steps. The compute step runs `statusgen` holding only the read token and emits the
  commit it made, parented on the main the tick fetched. The publish step runs no
  `statusgen`; it pushes that commit to the default branch, and a main that moved
  between the fetch and the push is a rejected push and a failed tick — the next
  hourly tick recomputes. The board-writer App
  token is minted only when there is a commit, narrowed to `contents` write, and
  only the publish step's environment holds it. The job uses
  the corroborate job's isolation: a job-local, checksum-verified Go toolchain and
  caches, `GOENV=off` and `GOWORK=off`,
  so no `go.work` left above `RUNNER_TEMP` chooses the source it builds, and it runs
  no `gh` at all — no pull request is opened, read or edited through it. It also keeps no
  state an earlier job on the runner could leave: no checkout into the workspace, but a
  fresh clone of the default branch under `RUNNER_TEMP`, a git configuration of its own
  with hooks off (`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_NOSYSTEM`, a discovery ceiling at
  `RUNNER_TEMP`), a job-local `GH_CONFIG_DIR`, a push to the forge URL by name, and a
  credential helper bound to that host. Its
  clone, compute and publish `run:` texts are tested end to end by
  `statusgen/reconcilejob_test.go`, which extracts them from the staged YAML, runs them
  in a rig that plants hooks, a push address and URL rewrites, and checks that a list of
  edits to the job each turns a named test red, and the workflow-level wiring the job
  depends on (the hourly cron, the scheduled runs' concurrency group, the lint step's test
  selection and its `paths:` entries) is pinned the same way, and the board workflow's
  PR lint job runs those tests. Two outcomes to expect: a tick that loses a race with a
  concurrent push to main fails and the next tick recomputes on the new main; and a
  tick on which nothing changed state pushes nothing at all. The same file holds every job that mints the App token to that
  isolation, except `regen` and `model-autoflip`, which are live today without it and
  are listed as exempt (the list may only shrink). The job pushes the default branch as its single-writer landing — the STATUS.md-regen pattern — and scheduled runs sit in their own concurrency group so they cannot displace a pending push regen. **Already live** — a maintainer
  promoted it, and the manifest declares it `identical`: the job tests read the staged
  copy, so the parity check is what keeps an edit made straight to the live file from
  leaving them green. The conversion to this shape is serialized in
  `docs/runsheets/reconcile-lane-hunk.diff` (desk-tools/36, assay#2688) with its
  receipts on that thread; until the driver applies it, the staged and live job still
  opens the `board/reconcile` draft PR under the standing rules. Once applied, the
  board-writer App needs `Contents: write` for the push and its `Pull requests: write`
  grant is no longer used (the conversion removes the PR step); keep that grant until
  the apply lands, because the live job's token mint still requests it.
  **Pending promotion (#2910):** the staged copy adds the classified board push to
  the `regen` and `model-autoflip` push steps (credential preflight before the
  commit and before every retry; only a non-fast-forward rejection retries or says
  main moved; every credential class fails once with its own line; git output
  masked), and a lint-job step running `statusgen/pushcred_test.go`, which executes
  those steps against a stub forge. The manifest declares it `pending` until a
  maintainer promotes it.

## Declared changes and drift guard

`declared-changes.json` inventories every staged YAML file. `identical` means no
pending change: the file must equal its live counterpart byte for byte. `pending`
classifies an intentional proposal documented above; its promotion requires a
separate review of the full `diff -u` against live. Pending proposals are excluded
from the parity check, rather than certified as drift-free or activated here.
The manifest records the sibling scope decisions, without pinning unrelated bases.

`go test ./internal/deskkit -run '^TestStagedDrift'` from `tools/desk` checks every
staged YAML has a classification and every identical twin equals live. New staged
files require explicit classification: either a manifest entry or a nonempty
`<filename>.pending` companion stating the proposal and pending promotion, which
lets parallel proposals avoid editing one shared inventory. A manifest entry takes
precedence over a companion, so a parity twin cannot be exempted by adding one. A
companion whose YAML file is gone, a manifest entry with no staged file, an empty
pending reason and an unknown mode each fail the guard.
The existing unconditional `ci.yml`
build-test job runs this guard on every PR and main push.

## Release image ownership and one approval

The live `release.yml` and its identical staged twin build binary artifacts
first, then use one protected `release` job to build/check/push all nine images
and publish the immutable GitHub release. The job holds `contents: write` and
`packages: write` behind one `release` Environment approval. It serializes on
the normalized umbrella version before registry absence checks; image and
reference validation finish before the tag or GitHub release is written.

The old reusable `docker-publish.yml` is retired, so it cannot create a second
approval or an independent image-publication route. Consumers never dispatch an
image build. `release-on-merge.yml` remains pending and is not activated here.
An already-published immutable release cannot acquire metadata later; use a
complete later release and never move or reuse a published version.

When changing this twin, update `.github/workflows/release.yml` in the same
reviewed maintainer commit. The drift guard requires them to remain identical.
