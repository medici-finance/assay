# Workspace caller census

Every production site in this repository that creates, removes, prunes, recursively copies or
recursively deletes a checkout, delegates that to `deskwt`, or emits a recipe telling an agent or
operator to do it — with its disposition under the cell workspace contract
([spec/cell-workspaces-v1.md](../spec/cell-workspaces-v1.md) §9), the migration step that owns
moving it, and the condition under which its row is deleted.

Source revision: `1948dc0873705930b202f0e747047df844010d1a`

The table is not a hand count. `TestWorkspaceCallerCensus` (`workspace/census_test.go`) scans the
real tree and fails on any discovered site this table does not route, and on any row whose site,
symbol and effect the scan no longer finds; it also dereferences the known `S-worktree` creation
duplicates of [docs/contracts.md](contracts.md) against source. Run it from the module:

```sh
cd workspace && GOWORK=off go test -count=1 -run '^TestWorkspaceCallerCensus$' ./...
```

A missing census or missing source tree is a skipped (could-not-check) run, never an empty pass.

## What the scan reads

- **Go** (production files; `_test.go` excluded): per function, or per package-level `const`/`var`,
  the string literals in source order and the calls. `"worktree"` followed by `add`, `remove` or
  `prune` is that effect; a call carrying `"clone"` with `"git"` (or through a git-named function)
  is `create`; a call carrying `"deskwt"` in a function naming a `deskwt` lifecycle verb is
  `delegate` (outside `deskwt` itself); `os.RemoveAll` in a function that creates, removes or prunes
  worktrees, or is named for one, is `rmtree`; a `filepath.WalkDir` that recreates directories and
  writes files is `copy`; a literal naming `git worktree add|remove|prune`, `git clone`, the
  Claude Code `EnterWorktree` tool or `codex … --worktree` is `prompt`. Comments never count.
- **Recipes** (`.md`, `.txt`, `.tmpl`, `.sh`, `.bash`, `.ps1`, `.py`, `.yml`, `.yaml`, `.mjs`): any
  line with one of those commands makes the whole file a `recipe` (symbol `-`).
- **Not read:** `.git`, `testdata/`, `tests/`, `node_modules/`, `vendor/`, `docs/`, `changelog/`,
  `spec/`, `schemas/`, `workspace/` (its tests plant callers on purpose), `CHANGELOG.md` and
  `*.test.sh`. Test fixtures build throwaway repositories; they are not cell callers.

## Dispositions

- **managed** — the site becomes a request to the workspace allocator for a workspace of the
  matching kind; its inline Git effect is deleted, and any surviving command is a facade.
- **snapshot** — the site needs immutable files of an exact commit, not a Git checkout; it reads a
  metadata-free snapshot from the existing scratch owner (`cellscratch`) instead.
- **temporary-compatibility** — the site keeps its current behaviour until its owner step lands,
  because it is a message, a shipped recipe or a non-cell runner step. Its retirement condition says
  what replaces it; a non-cell site states why it stays outside a cell. A site under `/tmp` is not
  non-cell for that reason alone.

## Owner steps

| Step | Scope |
|---|---|
| WS-02 | Independent repository stores |
| WS-03 | Transactional allocation and Git effects |
| WS-04 | Leases, retention and collection |
| WS-05 | Desk launch and cell maintenance |
| WS-06 | Worker, review and verification dispatch |
| WS-07 | Scan and reconcile |
| WS-08 | Detached comms, verification and merge-check worktrees |
| WS-09 | statusgen consumes the shared module |
| WS-10 | Harnesses and shipped recipes |
| WS-11 | Legacy inventory and adoption |
| WS-12 | Transfer and drain |
| WS-13 | Qualification |

## Census

| Site | Symbol | Effect | Disposition | Owner step | Retirement condition |
|---|---|---|---|---|---|
| `.cursor/references/claude-code.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.cursor/references/codex.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.cursor/references/cursor.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.cursor/skills/pr-shepherd/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.cursor/skills/verify-desk/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.cursor/skills/worker-desk/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | harnessgen regenerates this copy from its plugins/ source once that source is rerouted; never hand-edited |
| `.github/workflows/assay-statusgen.yml` | `-` | recipe | temporary-compatibility | WS-09 | a CI-runner clone under the runner's temp directory, outside any cell; stays non-cell compatibility, re-censused if the job is ever moved into a cell |
| `ci/staged-workflows/assay-statusgen.yml` | `-` | recipe | temporary-compatibility | WS-09 | a CI-runner clone under the runner's temp directory, outside any cell; stays non-cell compatibility, re-censused if the job is ever moved into a cell |
| `containers/README.md` | `-` | recipe | temporary-compatibility | WS-10 | the document describes the allocator path (or states why its command is a non-cell developer step) when shipped recipes are rerouted |
| `loopadmin/mutate/main.go` | `copyTree` | copy | snapshot | WS-08 | the copy is a cellscratch snapshot of the exact subject; no recursive copy of a live checkout |
| `plugins/assay/references/claude-code.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `plugins/assay/references/codex.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `plugins/assay/references/cursor.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `plugins/assay/skills/pr-shepherd/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `plugins/assay/skills/verify-desk/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `plugins/assay/skills/worker-desk/SKILL.md` | `-` | recipe | temporary-compatibility | WS-10 | the shipped recipe tells the agent to work in the allocator-returned workspace whenever managed mode is active, and keeps the raw command only for unmanaged adopters |
| `statusgen/failfirst.go` | `runFailFirstBase` | create | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/failfirst.go` | `runFailFirstBase` | prune | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/failfirst.go` | `runFailFirstBase` | remove | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/failfirst.go` | `runFailFirstBase` | rmtree | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/idvalidate.go` | `grandfatheredBaseFallbackNotices` | prompt | temporary-compatibility | WS-09 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `statusgen/lintaudit.go` | `productionWorktree` | create | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/lintaudit.go` | `productionWorktree` | remove | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/lintaudit.go` | `productionWorktree` | rmtree | managed | WS-09 | statusgen requests a check workspace for the exact commit from the workspace module and retires it through the allocator; no raw git worktree call left |
| `statusgen/main.go` | `run` | prompt | temporary-compatibility | WS-09 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `statusgen/README.md` | `-` | recipe | temporary-compatibility | WS-09 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `statusgen/registers.go` | `registerBaseFallbackNotices` | prompt | temporary-compatibility | WS-09 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `statusgen/scanguard.go` | `scanIsolationRefusal` | prompt | temporary-compatibility | WS-09 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/cellctl/check.go` | `cmdCheck` | prompt | temporary-compatibility | WS-05 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/cellctl/git.go` | `Cell.worktreeViaDeskwt` | delegate | managed | WS-05 | cellctl launch, reset and role setup request and retire desk workspaces through the allocator; no raw worktree effect or deskwt delegation left |
| `tools/desk/cmd/cellctl/help/concepts.txt` | `-` | recipe | temporary-compatibility | WS-05 | the help text describes allocator-owned desk homes |
| `tools/desk/cmd/cellctl/launch.go` | `Cell.deskLaunch` | create | managed | WS-05 | cellctl launch, reset and role setup request and retire desk workspaces through the allocator; no raw worktree effect or deskwt delegation left |
| `tools/desk/cmd/cellctl/reset.go` | `Cell.resetCell` | remove | managed | WS-05 | cellctl launch, reset and role setup request and retire desk workspaces through the allocator; no raw worktree effect or deskwt delegation left |
| `tools/desk/cmd/cellctl/reset.go` | `Cell.resetCell` | rmtree | managed | WS-05 | cellctl launch, reset and role setup request and retire desk workspaces through the allocator; no raw worktree effect or deskwt delegation left |
| `tools/desk/cmd/commsloop/dispatch_native.go` | `gitDetachedWorktreeComms` | create | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/commsloop/dispatch_native.go` | `gitDetachedWorktreeComms` | prompt | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/commsloop/dispatch_native.go` | `gitDetachedWorktreeComms` | remove | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/commsloop/dispatch_native.go` | `gitDetachedWorktreeComms` | rmtree | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskboot/boot.go` | `boot` | delegate | managed | WS-04 | boot collection runs the allocator's retention-aware collection, not a raw prune |
| `tools/desk/cmd/deskdispatch/prompt.go` | `assemblePrompt` | prompt | temporary-compatibility | WS-06 | the dispatched prompt hands the agent the allocator-returned cwd instead of an isolate-yourself recipe |
| `tools/desk/cmd/deskdispatch/references/common-clauses.md` | `-` | recipe | temporary-compatibility | WS-06 | the dispatched prompt hands the agent the allocator-returned cwd instead of an isolate-yourself recipe |
| `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md` | `-` | recipe | temporary-compatibility | WS-06 | the dispatched prompt hands the agent the allocator-returned cwd instead of an isolate-yourself recipe |
| `tools/desk/cmd/deskdispatch/reviewworktree.go` | `createDispatchWorktree` | delegate | managed | WS-06 | dispatch requests a worker/review/verify workspace for the exact subject and claim; no direct worktree or clone |
| `tools/desk/cmd/deskdispatch/workerkind.go` | `writeResumeIsolation` | prompt | temporary-compatibility | WS-06 | the dispatched prompt hands the agent the allocator-returned cwd instead of an isolate-yourself recipe |
| `tools/desk/cmd/deskmerge/currency.go` | `newWorktree` | create | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskmerge/currency.go` | `newWorktree` | rmtree | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskmerge/currency.go` | `worktree.remove` | prune | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskmerge/currency.go` | `worktree.remove` | remove | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskmerge/currency.go` | `worktree.remove` | rmtree | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/deskpr/deskpr.go` | `preflightOpts` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/deskpushguard/main.go` | `run` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/deskreconcile/reconcile.go` | `Run` | create | managed | WS-07 | scan and reconcile attempts run in managed workspaces; the inline worktree code is deleted |
| `tools/desk/cmd/deskreconcile/reconcile.go` | `Run` | remove | managed | WS-07 | scan and reconcile attempts run in managed workspaces; the inline worktree code is deleted |
| `tools/desk/cmd/deskwt/deskwt.go` | `cmdAdd` | create | managed | WS-03 | the Git effect moves into the allocator's effect adapter; deskwt keeps only a facade that calls it |
| `tools/desk/cmd/deskwt/deskwt.go` | `cmdAdd` | prompt | managed | WS-03 | the Git effect moves into the allocator's effect adapter; deskwt keeps only a facade that calls it |
| `tools/desk/cmd/deskwt/deskwt.go` | `removeWorktreeTree` | rmtree | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/deskwt.go` | `strandedAfterDeregister` | prompt | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/deskwt.go` | `strandedAfterDeregister` | prune | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/main.go` | `usage` | prompt | managed | WS-03 | the Git effect moves into the allocator's effect adapter; deskwt keeps only a facade that calls it |
| `tools/desk/cmd/deskwt/prune.go` | `bookkeepingPrune` | prune | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/prune.go` | `pruneSweep` | prompt | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/roleinit.go` | `cmdRoleInit` | create | managed | WS-03 | the Git effect moves into the allocator's effect adapter; deskwt keeps only a facade that calls it |
| `tools/desk/cmd/deskwt/roleinit.go` | `cmdRoleInit` | prompt | managed | WS-03 | the Git effect moves into the allocator's effect adapter; deskwt keeps only a facade that calls it |
| `tools/desk/cmd/deskwt/stalereg.go` | `clearStaleRegistrations` | prompt | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/deskwt/stalereg.go` | `clearStaleRegistrations` | prune | managed | WS-04 | removal and pruning run through the allocator's retention and retirement rules; deskwt keeps only a facade |
| `tools/desk/cmd/fanoutloop/dispatch.go` | `homeWorktreeClause` | prompt | temporary-compatibility | WS-06 | the dispatched prompt hands the agent the allocator-returned cwd instead of an isolate-yourself recipe |
| `tools/desk/cmd/muhar/harness.go` | `copyTree` | copy | snapshot | WS-08 | the copy is a cellscratch snapshot of the exact subject; no recursive copy of a live checkout |
| `tools/desk/cmd/muhar/README.md` | `-` | recipe | temporary-compatibility | WS-10 | the document describes the allocator path (or states why its command is a non-cell developer step) when shipped recipes are rerouted |
| `tools/desk/cmd/scanloop/lane.go` | `scanCarrierPRLane.carry` | create | managed | WS-07 | scan and reconcile attempts run in managed workspaces; the inline worktree code is deleted |
| `tools/desk/cmd/scanloop/lane.go` | `scanCarrierPRLane.carry` | remove | managed | WS-07 | scan and reconcile attempts run in managed workspaces; the inline worktree code is deleted |
| `tools/desk/cmd/verifyloop/dispatch_native.go` | `gitDetachedWorktree` | create | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/verifyloop/dispatch_native.go` | `gitDetachedWorktree` | prompt | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/verifyloop/dispatch_native.go` | `gitDetachedWorktree` | remove | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/verifyloop/dispatch_native.go` | `gitDetachedWorktree` | rmtree | managed | WS-08 | the detached check/verification worktree is a managed allocation retired by the allocator; the inline add/remove pair is deleted |
| `tools/desk/cmd/writeguard/callout.go` | `Config.checkCallout` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/writeguard/guard.go` | `Config.block` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/cmd/writeguard/guard.go` | `Config.sentinelBlock` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/internal/deskkit/preflight.go` | `checkSiblings` | prompt | temporary-compatibility | WS-10 | the message names the allocator verb (or the returned workspace) instead of a raw git recipe |
| `tools/desk/internal/deskkit/verifiercrossrepo.go` | `prepareVerifierCrossRepo` | create | managed | WS-06 | dispatch requests a worker/review/verify workspace for the exact subject and claim; no direct worktree or clone |
| `tools/desk/internal/deskkit/verifiercrossrepo.go` | `prepareVerifierCrossRepo` | rmtree | managed | WS-06 | dispatch requests a worker/review/verify workspace for the exact subject and claim; no direct worktree or clone |
| `tools/desk/README.md` | `-` | recipe | temporary-compatibility | WS-10 | the document describes the allocator path (or states why its command is a non-cell developer step) when shipped recipes are rerouted |
| `tools/gremlins/README.md` | `-` | recipe | temporary-compatibility | WS-10 | the document describes the allocator path (or states why its command is a non-cell developer step) when shipped recipes are rerouted |
| `tools/harnesslint/banned-tokens.md` | `-` | recipe | temporary-compatibility | WS-10 | the document describes the allocator path (or states why its command is a non-cell developer step) when shipped recipes are rerouted |
