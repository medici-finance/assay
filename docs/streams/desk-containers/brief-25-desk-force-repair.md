---
brief: assay:assay:desk-containers:25
title: "`cellctl desk --force` — diagnose a failed desk boot, repair what it can, ask before each destructive step, `-y` answers yes"
why: >-
  A `cellctl desk` boot fails for a dozen different reasons — a stale role lease, a role worktree
  that will not merge main, a fetch that failed, storage admission. Each failure prints its own
  fix, and the operator has to read it, run it, and retry, one failure at a time. There is no
  single "get this desk running again" command. `--force` runs the boot, and on each known
  failure applies the matching repair: safe repairs run at once, destructive ones are named and
  confirmed first, and `-y` confirms them all. Failures another verb already owns (the reconcile
  preflight, `cellctl reset`, `deskwt`) stay with that verb.
wave: 2
depends: ["desk-containers/10"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-13 by the-desk (assay:author-brief, opus-5.5; requested by the driver); revised 2026-10-14 for review rounds 1 and 2
sources:
  - "Driver request 2026-10-13: a `force` option on desk start that attempts to fix whatever stopped the boot; it may ask before anything destructive; `-y` assumes yes and asks nothing"
  - "tools/desk/cmd/cellctl/desk.go cmdDesk — the cockpit refusal, the reconcile preflight call, the house-only role lease (cellcadence.AcquireRole, --end-stale-lock-holder)"
  - "tools/desk/cmd/cellctl/reconcile.go (#2880) — the reconcile preflight: its `[y/N]` prompt (`readYes`), its pid-and-start-time proof, its cadence clearing, and its recorded 'deliberately no --yes' decision. This brief leaves all of it unchanged."
  - "tools/desk/internal/deskkit/lockholder.go — AcquireHeld / Diagnose / recoverStale: the proven-stale holder path this brief reuses, never re-implements"
  - "tools/desk/cmd/cellctl/git.go fetchMainUnderLock / mergeRoleWorktree / worktreeViaDeskwt — the fetch, merge and worktree failure sites; the `cellctl-fetch.lock` handling the boot already owns; mergeRoleWorktree aborts a conflicting merge before it refuses"
  - "tools/desk/cmd/cellctl/launch.go deskLaunch — storage admission (exit 6), the deskwt-created role worktree symlink, the `live session` worktree lock every boot takes, and scrubbedDeskLaunch's session lock and `has-session` check, which run after the worktree step"
  - "tools/desk/cmd/cellctl/lock.go takeSessionLock (exit 4), main.go die (exit 3) / exitWith"
  - "tools/desk/cmd/cellctl/cell.go loadCell and reset.go resetCell — the `.reset-in-progress` refusal, and why re-running reset is not a repair this brief may take"
  - "tools/desk/cmd/cellctl/reset.go resetDirectoryInside — the containment helper; #2851 — its open symlink defect"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md — Git worktree pruning remains with `deskwt`"
  - "docs/streams/desk-containers/brief-21-doctor.md (`cellctl doctor`, apply interactive-only) and brief-24-gc.md (`cellctl gc`, keeps dirty and unmerged worktrees) — the neighbouring verbs this brief draws its boundary against"
  - freshness-checked 2026-10-14 @ e4df633dc (origin/main) — no `--force`, `-y` or `--yes` on `cellctl desk`; the reconcile preflight is the only `[y/N]` prompt in cellctl; nothing in cellctl exits 5; cell kinds are k8s, house, container and scrubbed
domain: complicated
consumers:
  - "docs/cellctl.md (the `desk` verb gains `--force` and `-y/--yes`): follow-up desk-containers/25 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 25 — `cellctl desk --force`: repair a failed desk boot

## Context
files:
- `tools/desk/cmd/cellctl/cobra.go` — declare `--force` and `-y`/`--yes` on `desk` only (beside `--end-stale-lock-holder`), never on `up`
- `tools/desk/cmd/cellctl/desk.go` — route each known refusal through the repair table below when `--force` is set
- `tools/desk/cmd/cellctl/git.go` — under `--force` only, `mergeRoleWorktree` hands its refusal back to the repair table instead of dying; without `--force` its message and exit code are unchanged
- `tools/desk/cmd/cellctl/repair.go` (planned) (new — the repair table, the per-row prompt, the repair log)
- `tools/desk/cmd/cellctl/repair_test.go` (planned) (new)
- `docs/cellctl.md` — the `desk` verb's `--force` / `-y` section
- `changelog/desk-containers-25-desk-force.md` (planned)

facts:
- **How a boot fails today.** Most refusals reachable from `cellctl desk` go through `die()`, which prints `cellctl: <msg>` and exits 3 (`main.go`). Two do not: the storage-admission refusal in `deskLaunch` exits 6 (`launch.go`, `exitWith(6)`), and the session-lock refusal exits 4 (`lock.go`, `exitWith(4)`). `--force` turns these refusals into a table of `{detect, classify, repair}` rows. A failure with no row stays exactly as it is today: same message, same exit code. Nothing in cellctl exits 5 today; this brief gives 5 one meaning (below).
- **What `--force` does not own.** Three repairs already have an owner, and `--force` neither duplicates nor overrides them:
  - **The reconcile preflight** (`reconcile.go`, #2880) runs before the lease on every `cellctl desk` in a house-kind cell. It proves an unfinished cadence checkpoint or orphan dead by pid and start time, and clears it on one `[y/N]` confirmation. It records a decision: "There is deliberately no --yes flag: clearing stays a human act." `--force` leaves the preflight exactly as it is: it runs first, unchanged, and `-y` never answers its question. Off a TTY it refuses as it does today, so `--force -y` off a TTY does not clear cadence state. That stays a human act.
  - **`cellctl reset`** owns an interrupted reset. Its executor stops every process in the cell scope, runs `worktree remove --force --force` and `RemoveAll` on the cell worktree directory, and in hard mode runs `clean -ffdx` and `branch -D`. Its marker records no mode. A re-run therefore cannot be made preserving from here, so the `.reset-in-progress` refusal is a `never` row.
  - **`deskwt`** owns git worktree pruning (common clauses: "Git worktree pruning remains with `deskwt`"). A registration whose directory is gone is a `never` row that names `deskwt prune`.
- **Three classes, decided per row in code, never by the operator:**
  - **safe** — runs without asking, on every cell kind:
    - re-fetch after a transient fetch failure, with bounded retries;
    - `cellctl cache <cell> clean` when storage admission refused (the exit-6 site). This cleans only the cell's own managed cache.
  - **destructive** — named and confirmed first, and only on a house-kind cell (see Liveness):
    - ending a proven-stale role-lease holder;
    - rescuing a role worktree that will not merge main, and cutting a fresh one if it still conflicts (one row, one prompt; below).
  - **never** — reported with the manual fix, no repair offered:
    - config and usage errors, model-policy refusals, comms config, shim generation, harness-specific failures;
    - a role lease or session lock held by a LIVE session (the exit-4 site, and a scrubbed cell's `has-session` refusal);
    - every destructive row on a k8s, scrubbed or container cell;
    - an interrupted reset (`.reset-in-progress`): names `cellctl reset <cell>`;
    - a dangling worktree registration: names `deskwt prune`;
    - an unremovable `FETCH_HEAD`;
    - an unavailable explicit cockpit (`desk.go` refuses rather than replaces it, by recorded design): names `--cockpit tmux`;
    - anything the reconcile preflight refused;
    - a role worktree that is, or resolves through, a symlink (see Containment);
    - anything the table does not name.
- **Liveness comes before any destructive row.** A destructive row may run only once this invocation itself holds the proof that no live session owns the role. Today only a house-kind cell has that proof before the worktree step: `cmdDesk` takes the role lease (`cellcadence.AcquireRole`) before `deskLaunch`. On a scrubbed cell, the session lock (`takeSessionLock`, exit 4) and the `tmux has-session` check run in `scrubbedDeskLaunch`, after `deskLaunch` has already merged main into the role worktree. On a k8s cell, nothing establishes liveness before that merge. A repair hooked to the merge failure could therefore commit or move a running desk's tree before the live-session refusal ever runs. So on any kind other than house, every destructive row is `never`: the merge refusal stays exactly as today, and only the safe rows run. The `live session` worktree lock that every boot takes records no pid, so this brief never reads it as a liveness signal in either direction. The lease is the proof.
- **Destructive here means "changes state the operator may want", never "loses it".** Each destructive repair preserves what it displaces, and the prompt names where it goes.
  - **The non-merging role worktree** is one row with one prompt. The prompt names the rescue ref, and names the aside path that will be used if the merge still conflicts. When `mergeRoleWorktree` refuses, it has already aborted the merge, so the tree is back at its pre-merge HEAD. The row then:
    1. If the tree is dirty, commits everything, untracked files included (`add -A`), as one WIP commit with the cell's commit identity.
    2. Creates `rescue/<role>-<UTC stamp>` at that HEAD.
    3. Retries the merge once. If it now merges, the row is done.
    4. If it still conflicts (the tree is clean at the rescue commit after the abort), runs `git worktree unlock <wt>`, then `git worktree move <wt> <wt>.aside-<UTC stamp>`, then `git worktree lock --reason "cellctl <cell>/<role> rescue aside"` on the moved tree. It then cuts a fresh worktree at `<wt>` from the fetched main on a new `<cell>-cell/<role>-<stamp>` branch, with a plain `worktree add` and no `-f`. `move` keeps the aside tree a valid, registered, locked worktree, so nothing needs pruning and no registration dangles. A plain rename would leave a missing-but-locked registration that only `add -f -f`, `prune` or `remove` could clear.
  - **A lease holder** gets SIGTERM only, and only when `deskkit.Diagnose` proves it stale: it holds the lock but its recorded role child has exited, and it is the same cell, the same role, a `cellctl` command, and not the current pid. This is exactly the `--end-stale-lock-holder` path from `tools/desk/internal/deskkit/lockholder.go`; `--force` calls it and never re-implements it. Never SIGKILL.
- **Displaced state is kept until a human removes it.** `--force` never deletes an aside tree or a `rescue/*` ref, and no verb does it automatically. Brief 24's `cellctl gc` keeps any worktree with uncommitted changes or an unmerged, unpushed branch, so it leaves both alone. `docs/cellctl.md` names the manual step: review, then remove the aside tree through `deskwt` and delete the ref by hand. Adopting them into `gc` would be a follow-up to brief 24, not part of this brief.
- **A live session is out of scope.** A role lease or session lock held by a live, current holder means a desk is already running. `--force` does not end it, even with `-y`. It prints the holder and `cellctl down <cell>` as the manual step. Ending a running desk kills its in-flight agents and loses their unsaved work, and that loss cannot be undone. Whether `--force` should ever do that is a separate decision for the driver, not this brief.
- **Prompts.** One prompt per destructive repair, naming the failure, the action, and where the displaced state goes, e.g. `role lease held by pid 4121 (stale: cellctl desk, its role child 4130 has exited). End it with SIGTERM? [y/N]`. Default is No. Anything but `y`/`yes` declines, the repair is skipped, and the boot stops with today's message and exit code. Read the answer with the existing `readYes` and gate it on the existing fail-closed `isTTY`; do not add a second prompt reader or TTY check.
  - `-y` / `--yes` answers yes to every prompt in THIS table and prints each action it took. It never changes a `never` row, and it never answers the reconcile preflight.
  - **No TTY and no `-y`:** refuse before any destructive step, with exit 5 and the fixed line `cellctl: desk --force: a destructive repair needs a TTY or -y`. The fixed line is part of the contract. `cellctl desk` passes the harness's own exit status through (`runForeground`), so a harness that exits 5 cannot be told apart from this refusal by the code alone. `docs/cellctl.md` says so and tells scripts to match the line.
  - `-y` without `--force` is a usage error (exit 1).
- **Loop bound.** After a repair the boot step that failed is retried. Each row may fire at most once per invocation, and the whole run makes at most 5 repairs. A failure that repeats after its repair stops the boot, with the repair log printed. `--force` must never spin.
- **Repair log.** Every action, safe or destructive, gets one line on stderr and is appended to `<cell>/run/repair.log`. A line holds exactly these fields: UTC stamp, role, row id, action, displaced-state location. It never captures git or credential-helper output, because fetch-failure output can carry a token. `DRY_RUN=1` prints the plan, prompts nothing, and changes nothing.
- **Containment.** Every path a repair moves, renames or removes must resolve inside the cell's own worktree root. Check this with `resetDirectoryInside` (`reset.go`), the helper reset uses; this brief adds no second, weaker check. An error from the helper, or a path outside the root, is a `never` row: containment fails closed. The helper's symlink handling has an open defect (#2851). Until #2851 is closed, a role worktree path that is a symlink or resolves through one is `never`. Note that a deskwt-created role worktree is linked into the cell as a symlink (`launch.go`), so this case is common.
  - **Writes to the shared git dir.** The repository's common git dir is shared, as on every boot. A repair writes to it only additively:
    - the WIP commit's objects;
    - the new `rescue/*` ref, which is create-only (`update-ref <ref> <new> ""`, or `branch` without `-f`): an existing ref is never overwritten, and a name collision is a `never` row;
    - the fresh role branch, which is new by name;
    - the registration update that `git worktree move/lock/unlock/add` makes for this cell's own role worktree.

    Nothing else is written there, and nothing in it is deleted or pruned.
- **No name clash.** `cellctl reset [--hard]` stays the cell-wide verb, and `--force` never calls reset's executor. `--force` exists today only on `cellctl set`, where it means something else; it is added to `desk` only.
- **Boundary with `cellctl doctor`** (brief 21, on main). doctor diagnoses every stuck state. Its fixes apply only through `doctor --apply <plan-id>`, on a TTY, with a confirm code. `--force` is the boot-path subset: it covers only the rows above, and only for this role. Two faults overlap, a proven-stale lease holder and storage over budget. For both, doctor names the same underlying action this brief uses: the existing stale-holder end and `cache clean`. Where both verbs cover a fault, the owner verb's action does the work and neither re-implements it. `--force` (with or without `-y`) never calls `doctor --plan` or `doctor --apply`, because doctor's apply path is interactive-only by design.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared cluster or shared checkout. Tests use temp dirs and throwaway git repos only.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-cell`, `example-org/example-repo`); no real organisation, host or person names.
- If a refusal site in the tree has no clear repair, or contradicts these facts, leave it `never` and record it in the PR body. Do not invent a repair.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Inventory every refusal reachable from `cellctl desk`: every `die()` and every `exitWith(n)` in cmdDesk → reconcilePreflight → deskLaunch → scrubbedDeskLaunch / runInteractiveHarness, plus loadCell. Map each one to a row id and a class, by cell kind. The inventory goes in `repair.go` as a comment table and in the PR body, with each site's exit code.
2. Implement the repair rows, the per-row prompt (reusing `readYes` and `isTTY`), the house-only gate on destructive rows, the `-y` handling, the no-TTY refusal and its fixed line, the loop bound, and the repair log. Leave `reconcile.go` and `reset.go` unchanged. In `git.go`, change only `mergeRoleWorktree`'s refusal hand-back under `--force`.
3. Declare `--force` and `-y/--yes` on `desk` in `cobra.go`. Update the CLI docs-parity tests.
4. Write `docs/cellctl.md`'s section and the changelog fragment. The section covers:
   - the three classes and the row table, per cell kind;
   - what `-y` does and does not do (it does not answer the reconcile preflight);
   - the live-session rule;
   - where displaced state goes and the manual step that removes it;
   - the exit-5 fixed line.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go vet ./cmd/cellctl/` | exit 0 |
| 2 | check | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairRows$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairRows' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairRows`. The suite covers each safe and destructive row on a house-kind cell. Each row's case induces its failure in a temp cell and runs `desk --force -y` with a stub harness, then asserts that the repair ran and the boot reached the harness. The stale-holder case uses a holder whose recorded role child has exited, the state `Diagnose` reads as stale. |
| 3 | check +neighbour +dereference | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairPreservesState$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairPreservesState' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairPreservesState`. A role worktree that is dirty (a tracked edit plus an untracked file) and conflicts with main ends with a `rescue/` ref whose commit holds both files. `git worktree list` shows the aside tree at `<wt>.aside-<stamp>`, still registered and locked with the rescue reason. The fresh worktree at `<wt>` was added with no `prune`, no `remove` and no `-f`. Nothing is deleted. A pre-existing `rescue/` ref of the same name is left untouched and the row is refused. A dirty tree that merges cleanly after its WIP commit ends with the rescue ref and no aside tree. |
| 4 | check +flow | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairLiveHolderRefused$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairLiveHolderRefused' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairLiveHolderRefused`. A live, current lease holder with `--force -y` exits non-zero. The holder process is still running afterwards, and the output names `cellctl down`. |
| 4b | check +flow | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairNonHouseNoDestructive$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairNonHouseNoDestructive' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairNonHouseNoDestructive`. A scrubbed cell with a live session (session-lock pid alive) and a k8s cell, each with a dirty role worktree that conflicts with main, are run with `--force -y` and no TTY. Each exits with today's merge-refusal code and message. Each leaves the worktree's files, its lock, `git for-each-ref` and `git worktree list` unchanged, and creates no `rescue/` ref. |
| 5 | check +flow | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairPrompt$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairPrompt' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairPrompt`. Answering `n` or empty skips the repair and keeps today's exit code. No TTY and no `-y` exits 5 and prints the fixed line before any state changes, checked by snapshotting the cell dir, the repository's refs (`git for-each-ref`) and its worktree list. `-y` without `--force` exits 1. |
| 6 | check | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairBounded$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairBounded' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairBounded`. A failure that recurs after its repair stops after one attempt, and no run makes more than 5 repairs. |
| 7 | check | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestRepairLeavesOwnedRefusals$' -count=1 -v > "${TMPDIR:-/tmp}/r25.out" 2>&1 && grep -F -e '--- PASS: TestRepairLeavesOwnedRefusals' "${TMPDIR:-/tmp}/r25.out"` | exit 0; output holds `--- PASS: TestRepairLeavesOwnedRefusals`. With `--force -y` and no TTY, each case is refused as it is today and changes nothing: an unfinished cadence checkpoint (`checkpoint.json` still present; the reconcile preflight's own refusal is printed), a `.reset-in-progress` marker (names `cellctl reset`), a dangling worktree registration (names `deskwt prune`; `git worktree list` unchanged), and a symlinked role worktree (the link and its target are unchanged). |
| 8 | check | `cd tools/desk && go test ./cmd/cellctl/ -count=1` | exit 0. The whole package passes, including the CLI docs-parity and shape tests and the existing reconcile and reset tests. |
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run 'Lock' -count=1` | exit 0. The stale-holder path the repair calls is unchanged. |
| 10 | check | `statusgen --root . --lint > "${TMPDIR:-/tmp}/r25-lint.out" 2>&1 && grep -q '^LINT: PASS' "${TMPDIR:-/tmp}/r25-lint.out"` | exit 0; the lint output ends `LINT: PASS` |
| 10a | check | `statusgen --root . --consumers --brief desk-containers/25` | exit 0 — the `consumers:` routing holds against the branch diff |
| 11 | gate:model +flow | On a scratch **house-kind** cell whose role worktree is a real directory under the cell (no `deskwt` on PATH), with a prior `cellctl desk` stopped by `kill -STOP` and a dirty role worktree that conflicts with main. (a) Run `cellctl desk example-cell worker-desk --force`. (b) `kill -CONT` the prior desk, run `cellctl down example-cell`, then re-run the same command, answering `y`. | (a) The stopped desk's role child is still alive, so `Diagnose` reads the holder as current: the run refuses, names `cellctl down`, sends no signal, and the stopped process still exists. (b) One prompt appears, naming the rescue ref and the aside path. The desk launches. `run/repair.log` holds that line. The ref exists, and `git worktree list` shows the aside tree registered and locked. Record the transcript. No scratch cell available → could-not-check. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no. Every destructive repair preserves what it displaces and runs only behind a held role lease. A live session is never ended, and the stale-holder signal is the existing reviewed path. The repairs that delete (`cellctl reset`) or clear without a `-y` (the reconcile preflight) stay with their owner verbs, as `never` rows.

The reviewer checks rows 3, 4, 4b, 5 and 7. They also confirm that no repair row:
- deletes a directory;
- deletes, force-moves or overwrites a branch ref;
- prunes or removes a worktree, or forces a worktree add or move;
- sends SIGKILL.

They also confirm that `repair.go` reaches none of reset's executor, the cadence-clearing path, or doctor's plan or apply path. Instrument: `grep -n -E 'RemoveAll|"-D"|"-f"|"--force"|"prune"|"remove"|SIGKILL|resetCell|resetGit|clearCadence|reconcileApply|doctorApply|update-ref' tools/desk/cmd/cellctl/repair.go`. Any hit is a finding, with two exceptions: a create-only `update-ref <ref> <new> ""` call, and the `--force` flag's own name inside a message. Also confirm that the diff leaves `reconcile.go` and `reset.go` unchanged and touches `git.go` only in `mergeRoleWorktree`.
