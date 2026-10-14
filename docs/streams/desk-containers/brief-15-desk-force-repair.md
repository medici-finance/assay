---
brief: assay:assay:desk-containers:15
title: "`cellctl desk --force` — diagnose a failed desk boot, repair what it can, ask before each destructive step, `-y` answers yes"
why: >-
  A `cellctl desk` boot fails for a dozen different reasons — a stale role lease, an unfinished
  cadence checkpoint, a reset marker, a worktree that will not merge main, a fetch that failed.
  Each failure prints its own fix, and the operator has to read it, run it, and retry, one
  failure at a time. There is no single "get this desk running again" command. `--force` runs
  the boot, and on each known failure applies the matching repair: safe repairs run at once,
  destructive ones are named and confirmed first, and `-y` confirms them all.
wave: 2
depends: ["desk-containers/10"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-13: a `force` option on desk start that attempts to fix whatever stopped the boot; it may ask before anything destructive; `-y` assumes yes and asks nothing"
  - "tools/desk/cmd/cellctl/desk.go cmdDesk — the role lease (cellcadence.AcquireRole, --end-stale-lock-holder) and the unfinished-cadence-pass refusal"
  - "tools/desk/internal/deskkit/lockholder.go — AcquireHeld / Diagnose / recoverStale: the proven-stale holder path this brief reuses, never re-implements"
  - "tools/desk/cmd/cellctl/git.go fetchMainUnderLock / mergeRoleWorktree / worktreeViaDeskwt — the fetch, merge and worktree failure sites"
  - "tools/desk/cmd/cellctl/cell.go loadCell — the `.reset-in-progress` marker refusal"
  - "tools/desk/cmd/cellctl/cache.go / cadence.go — the existing `cache <cell> clean` and `cadence <cell> recover <role> --confirm-stopped` repair verbs"
  - freshness-checked 2026-10-13 @ bcc02252e (origin/main) — no `--force`, `-y` or `--yes` on `cellctl desk`; no y/N prompt helper anywhere in cellctl
domain: complicated
consumers:
  - "docs/cellctl.md (the `desk` verb gains `--force` and `-y/--yes`): follow-up desk-containers/15 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 15 — `cellctl desk --force`: repair a failed desk boot

## Context
files:
- `tools/desk/cmd/cellctl/cobra.go` — declare `--force` and `-y`/`--yes` on `desk` only (beside `--end-stale-lock-holder`), never on `up`
- `tools/desk/cmd/cellctl/desk.go` — route each known refusal through the repair table below when `--force` is set
- `tools/desk/cmd/cellctl/repair.go` (planned) (new — the repair table, the prompt helper, the repair log)
- `tools/desk/cmd/cellctl/repair_test.go` (planned) (new)
- `docs/cellctl.md` — the `desk` verb's `--force` / `-y` section
- `changelog/desk-containers-15-desk-force.md` (planned)

facts:
- Today every boot failure ends in `die()` (exit 3) with a message that names the manual fix. `--force` turns that list of messages into a table of `{detect, classify, repair}` rows. A failure with no row stays exactly as it is today: same message, same exit code.
- **Three classes, decided per row in code, never by the operator:**
  - **safe** — runs without asking: re-fetch after a transient fetch failure (bounded retries); `git worktree prune` of a registration whose directory is provably gone; removing a stale `FETCH_HEAD` lock the fetch left behind; `cellctl cache <cell> clean` when storage admission refused.
  - **destructive** — named and confirmed first: ending a proven-stale role-lease holder; `cadence recover` of an unfinished checkpoint; re-running an interrupted `cellctl reset` for this cell; moving a role worktree that will not merge main aside and cutting a fresh one; falling back from an unavailable cockpit to tmux.
  - **never** — reported with the manual fix, no repair offered: config and usage errors, model-policy refusals, comms config, shim generation, harness-specific failures, a role lease held by a LIVE session, and anything the table does not name.
- **Destructive here means "changes state the operator may want", never "loses it".** Every destructive repair preserves what it displaces, and the prompt names where it went:
  - uncommitted work in a role worktree → committed to `rescue/<role>-<UTC stamp>` with the cell's commit identity, then the merge retried;
  - a worktree whose merge conflicts → the branch ref is kept as `rescue/<role>-<UTC stamp>`, the directory is moved aside (renamed), never deleted, and a fresh worktree is cut from `origin/main`;
  - a lease holder → SIGTERM only, only when `deskkit.Diagnose` proves it stale (same cell, same role, a `cellctl` command, its pid not the current session). This is exactly the `--end-stale-lock-holder` path from `tools/desk/internal/deskkit/lockholder.go`; `--force` calls it and never re-implements it. Never SIGKILL.
  - `cadence recover` runs only after the repair proves the checkpoint's recorded child pid and its process group are gone. If either is alive, the row is `never`: the message names the pid, and the boot stops.
- **A live session is out of scope.** A role lease held by a live, current holder means a desk is already running. `--force` does not end it, even with `-y`; it prints the holder and `cellctl down <cell>` as the manual step. Ending a running desk kills its in-flight agents and loses their unsaved work, and that loss cannot be undone. Whether `--force` should ever do that is a separate decision for the driver, not this brief.
- **Prompts.** One prompt per destructive repair, naming the failure, the action, and where the displaced state goes, e.g. `role lease held by pid 4121 (stale: cellctl desk, started 2026-10-12T22:04Z). End it with SIGTERM? [y/N]`. Default is No. Anything but `y`/`yes` declines, the repair is skipped, and the boot stops with today's message.
  - `-y` / `--yes` answers yes to every destructive prompt and prints each action it took. It never changes a `never` row.
  - **No TTY and no `-y`:** refuse before any destructive step, exit 5, naming `-y`. Use the existing fail-closed `isTTY`; do not add a second TTY check.
  - `-y` without `--force` is a usage error (exit 1).
- **Loop bound.** After a repair the boot step that failed is retried. Each row may fire at most once per invocation, and the whole run makes at most 5 repairs. A failure that repeats after its repair stops the boot, with the repair log printed. `--force` must never spin.
- **Repair log.** Every action, safe or destructive, gets one line on stderr and is appended to `<cell>/run/repair.log` (UTC stamp, role, row id, action, displaced-state location). `DRY_RUN=1` prints the plan, prompts nothing, and changes nothing.
- **Containment.** Every path the repair moves or prunes must resolve inside the cell's own worktree root. Check this with the same containment helper the rest of cellctl uses. An open defect in that helper's symlink handling is being fixed separately; this brief adds no second, weaker check. A path that fails containment is a `never` row.
- **No name clash.** `cellctl reset [--hard]` stays the cell-wide verb, and `--force` never calls `reset --hard`. `--force` exists today only on `cellctl set`, where it means something else; it is added to `desk` only.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared cluster or shared checkout. Tests use temp dirs and throwaway git repos only.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-cell`, `example-org/example-repo`); no real organisation, host or person names.
- If a refusal site in the tree has no clear repair, or contradicts these facts, leave it `never` and record it in the PR body. Do not invent a repair.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Inventory every `die()` reachable from `cellctl desk` (cmdDesk → deskLaunch → runInteractiveHarness, plus loadCell). Map each one to a row id and a class. The inventory goes in `repair.go` as a comment table and in the PR body.
2. Implement the prompt helper (TTY, `-y`, default No, no-TTY refusal), the repair rows, the loop bound, and the repair log.
3. Declare `--force` and `-y/--yes` on `desk` in `cobra.go`. Update the CLI docs-parity tests.
4. Write `docs/cellctl.md`'s section: the three classes, the row table, what `-y` does and does not do, the live-session rule, and where displaced state goes. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go vet ./cmd/cellctl/` | exit 0 |
| 2 | check | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepair' -count=1` | exit 0. The suite covers each safe and destructive row. Each row's case induces its failure in a temp cell and runs `desk --force -y` with a stub harness, then asserts that the repair ran and the boot reached the harness. |
| 3 | check +neighbour | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepairPreservesState' -count=1` | exit 0. A dirty worktree and a conflicting worktree each end with a `rescue/` ref holding the prior HEAD (plus a WIP commit holding the dirty files), and the moved-aside directory still exists. Nothing is deleted. |
| 4 | check +flow | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepairLiveHolderRefused' -count=1` | exit 0. A live, current lease holder with `--force -y` exits non-zero. The holder process is still running afterwards, and the output names `cellctl down`. |
| 5 | check +flow | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepairPrompt' -count=1` | exit 0. Answering `n` or empty skips the repair and keeps today's exit code. No TTY and no `-y` exits 5 before any state changes, checked by snapshotting the cell dir. `-y` without `--force` exits 1. |
| 6 | check | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepairBounded' -count=1` | exit 0. A failure that recurs after its repair stops after one attempt, and no run makes more than 5 repairs. |
| 7 | check | `cd tools/desk && go test ./cmd/cellctl/ -run 'TestRepairCadenceRecoverNeedsDeadChild' -count=1` | exit 0. A checkpoint whose child pid is alive is not recovered, even with `-y`. |
| 8 | check | `cd tools/desk && go test ./cmd/cellctl/ -count=1` | exit 0. The whole package passes, including the CLI docs-parity and shape tests. |
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run 'Lock' -count=1` | exit 0. The stale-holder path the repair calls is unchanged. |
| 10 | check | `statusgen --root . --lint` | exit 0 |
| 10a | check | `statusgen --root . --consumers --brief desk-containers/15` | exit 0 — the `consumers:` routing holds against the branch diff |
| 11 | gate:model +flow | On a scratch cell: induce a stale lease (kill -STOP a prior `cellctl desk`, then let it be found stale), plus a dirty role worktree behind main. Run `cellctl desk example-cell worker-desk --force`, answering each prompt `y`. | Two prompts appear, each naming its action and the rescue location. The desk launches. `run/repair.log` holds both lines. Record the transcript. No scratch cell available → could-not-check. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: every destructive repair preserves what it displaces, a live session is never ended, and the stale-holder signal is the existing reviewed path. The reviewer checks rows 3, 4, 5 and 7. They also confirm that no repair row deletes a directory, deletes a branch ref, or sends SIGKILL (grep `repair.go` for `RemoveAll`, `branch -D`, `SIGKILL`). A hit is a finding.
