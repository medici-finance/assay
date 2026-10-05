# Managed task scratch

`cellctl` owns disposable task scratch under the cell's private `run/scratch`
directory. House-cell cadence passes and interactive launches enroll automatically,
set `TMPDIR`/`TMP`/`TEMP`, and propagate that owner to Codex command environments.
Each invocation records its session, role/task, immutable source revision, creation
and finish times, lifecycle, exit status, child process group, and evidence status.
Git role/worker worktrees remain with `deskwt`.

## Evidence handoff comes before disposal

Use the inherited temporary directory for unique body files and disposable output.
Keep source changes in the assigned worktree. At the end of a pass/session, after
all workers using this scratch have finished, persist required outcomes and evidence
to the existing canonical destination (PR workpad/review, verification receipt, or
durable artifact). Read that destination back, then acknowledge it:

```sh
cellctl scratch "$ASSAY_SCRATCH_CELL" ack \
  --id "$ASSAY_SCRATCH_ID" --receipt '<canonical destination and evidence reference>'
```

This is an explicit attestation by the caller, not an automatic forge upload or a
claim that a zero process exit proves handoff. Do not acknowledge while evidence,
resumption state, a running delegated task, or work not yet committed/pushed still
depends on scratch. Do not generate further required output after acknowledgment.
A missing receipt preserves the entire tree, including after successful exit.
An acknowledgment alone never overrides a live execution lease or surviving child.

Successful acknowledged tasks are reclaimed after child-tree cleanup. Failed or
interrupted acknowledged tasks keep their result record, receipt and the last
64 KiB of combined stdout/stderr; their disposable workspace is reclaimed. Capture
any other required diagnostic file at its canonical destination *before* acknowledgment.
Failed tasks without a receipt keep their workspace until an operator/desk completes
that handoff. Mark an inactive run resumable with `ack --resumable`; sweeps always
preserve it. Finish resumption in a new managed run; an old resumable directory is
intentionally not automatically released.

## Bounded one-off tasks

For a task outside a launched house desk, use the same lifecycle owner:

```sh
cellctl scratch <cell> run --session <session> --task <task> \
  --source <owned-worktree> --revision HEAD --snapshot \
  --snapshot-bytes 268435456 -- <executable> <args>
```

The child runs in the disposable workspace; `ASSAY_SOURCE_ROOT` and
`ASSAY_SOURCE_REVISION` identify its source. Without `--snapshot`, the workspace
starts empty, suitable for bounded checks using explicit source references.
The child must perform the evidence handoff above, or the desk can acknowledge
after it exits. Its nonzero exit status survives cleanup; cleanup failure following
an otherwise successful command produces a nonzero exit.

A snapshot streams *all tracked blobs* from the resolved Git revision, including
files marked `export-ignore`; it is not a recursive directory copy. Git metadata,
credentials in Git configuration, untracked files, and generated working-tree output
are not implicitly imported. When a test/review requires extra generated data, name
each regular source-relative file with repeatable `--input <path>`. The combined
snapshot/input byte limit is enforced; exceeding it fails rather than silently
dropping data. Increase the explicit limit or use source references when appropriate.
Submodules and escaping repository symlinks require an explicit separate checkout
or input plan; snapshots refuse them rather than presenting incomplete source.
Never point an input at managed output. Do not replace this with recursive copies
of the source working directory.

## Cleanup and recovery

```sh
cellctl scratch <cell> sweep                         # dry-run
cellctl scratch <cell> sweep --apply
cellctl scratch <cell> sweep --apply --max-age 168h --max-bytes 268435456
cellctl scratch <cell> inventory --path <legacy-temp-root>
```

Startup and completion run the same sweep; existing desk cadence supplies scheduling.
The default retention policy is seven days and 256 MiB for retained diagnostics,
oldest first. Set `CELL_SCRATCH_MAX_AGE` (a Go duration, such as `168h`) and
`CELL_SCRATCH_MAX_BYTES` (integer bytes) in the existing `cell.env` to persist
that policy across launches. One-off run and sweep flags override those values
for that invocation.
These are retention budgets, not a disk quota for active/pending work: protected data
can exceed them. Reports explicitly set `over_budget` instead of deleting protected
data; the sweep command returns 2 for that condition. Normal cleanup returns 0,
and partial/deletion failures return 1 with per-path reasons, logical file bytes,
and actual reclaimed bytes. Dry-run and apply use the same decision function.
Logical bytes are not allocated blocks or filesystem free-space measurements.
Directory entries and filesystem metadata are not counted.

Every sweep takes an OS advisory root lock and each run's execution lock. Live,
resumable, unacknowledged, unverified, and Git-containing trees stay intact, with a
reason in the report. The root and destructive operations use Go's confined
`os.Root`; symlinks cannot redirect deletion outside it. A failed inspection is
reported as partial, not as a clean observation.

Catchable termination uses the existing process-tree supervisor, then releases
ownership. After SIGKILL/restart, a free lease *plus* an enrolled process group
proved empty can recover an acknowledged run. PID existence and age alone never
authorize recovery. Missing enrollment, inaccessible liveness, surviving children,
and non-Unix abandoned-process recovery stay could-not-check/kept. Normal
completion works with the cross-platform process supervisor. The process group/job
contract cannot prove inactivity for a child that escapes containment or delegates
to an unrelated service: do not acknowledge those runs until their separate
ownership has been resolved.

## Existing scratch and rollout

Unknown legacy paths are inventory-only, even if old or named like a managed run.
There is no adopt-by-name or adopt-by-age deletion flag. Establish ownership,
activity, resumption and canonical evidence manually before moving any required
data into a newly enrolled run; leave the legacy directory untouched for its owner.
Never glob-delete a harness temporary root. This implementation does not inspect or
delete existing user scratch during installation.

Automatic enrollment covers the house launcher process paths, both interactive
and cadence. Scrubbed tmux sessions, container/cluster runtimes, external IDE
sessions, and harness-private roots that ignore inherited temporary-directory
settings require explicit `scratch run` or remain report-only. A nested managed
task keeps its own lease and receipt; the parent must wait for it before handing
off its own scratch. All Git checkouts (including clean clones) are conservatively
kept: use metadata-free snapshots for disposable source copies, and `deskwt` for
worktrees. This is not a second Git worktree pruner.

## Quality-history investigation

At the inspected source revision, `docs/quality/diffs.jsonl` is not tracked.
`qualgen/store.go` defines it as an append-only table; `qualgen/mine.go` mines
history and stores per-file line content. A working-directory copy therefore
imports generated historical data that a commit snapshot would not contain.
Tracked `docs/quality/metrics.jsonl` remains in snapshots: silently excluding it
would change the repository under test. Mining also records changes to tracked
quality artifacts; this lifecycle change does not redefine the quality model or
delete its historical evidence. Use bounded task-specific inputs or immutable
references when the complete history is unnecessary.

The snapshot manifest comes solely from Git objects and cannot traverse its output;
declared input imports reject managed output. Diagnostic capture replaces a bounded
tail instead of self-appending. These close the scratch-copy/output amplification
paths without truncating required source or canonical quality evidence.

## Verification

From `tools/desk`:

```sh
go test ./internal/cellscratch ./cmd/cellctl ./internal/cellprocess -run 'TestScratch|TestObserved|TestRun|TestCadenceExecutes|TestInteractiveOwner' -count=1 -timeout 45s
go run ./cmd/muhar -spec internal/cellscratch/mutations.json
```

Fixtures own all trees and subprocesses. They cover handoff, failure, SIGKILL,
surviving children, pending/resumable/foreign/Git paths, path escapes, competing
sweeps, partial deletion and retry, snapshot completeness and bounds, and repeated
worker/reviewer/verifier passes across the supported host harnesses.
