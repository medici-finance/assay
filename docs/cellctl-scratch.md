# Managed task scratch

Command-line forms (flags, precedence, exit codes) are listed in [the `cellctl` command reference](cellctl-cli-reference.md); what changed from the earlier hand-written parser is in [the compatibility table](cellctl-cli-compat.md).

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
A missing receipt preserves the tree, including after successful exit. The one
exception is abandoned build output, described under "Periodic maintenance" below.
An acknowledgment alone never overrides a live execution lease or surviving child.

Successful acknowledged tasks become eligible for explicit maintenance after child-tree cleanup. Failed or
interrupted acknowledged tasks keep their result record, receipt and the last
64 KiB of captured output; their disposable workspace is reclaimed. Interactive
terminal descriptors pass through directly to preserve terminal behavior. The
diagnostic bundle records that terminal output could not be captured; redirected
streams and non-interactive launches retain their bounded output tail. Capture
any other required diagnostic file at its canonical destination *before* acknowledgment.
Failed tasks without a receipt keep their workspace until an operator/desk completes
that handoff, apart from abandoned build output. Mark an inactive run resumable with `ack --resumable`; sweeps always
preserve it. Finish resumption in a new managed run; an old resumable directory is
intentionally not automatically released.

## Bounded one-off tasks

For a task outside a launched house desk, use the same lifecycle owner:

```sh
cellctl scratch <cell> run --session <session> --task <task> \
  --source <owned-worktree> --revision HEAD --snapshot \
  --snapshot-bytes 268435456 -- <executable> <args>
```

`--source` must name the root of a non-bare Git working tree; linked worktrees
are supported. Admission and local Git reads ignore inherited Git routing settings.
Declared inputs must belong to that same working tree.

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
each regular source-relative file with repeatable `--input <path>`. Inputs require
literal directory components and a single filesystem link; source identities are
checked through opened directory/file handles. The combined
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

Startup and completion only register/finalize ownership. They never inventory historical
payloads or reclaim data. The running supervisor's periodic sweep (below) and the
explicit sweep above do the recursive accounting and reclamation; interrupted
completion conservatively leaves the task protected.
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

Maintenance serializes other sweeps with a maintenance lease. Recursive inventory runs outside the root admission lock; each eligible run's execution lease remains held through revalidation and deletion. Short root transactions cover enrollment and lifecycle metadata. Live,
resumable, unacknowledged, unverified, and Git-containing trees stay intact, with a
reason in the report. Abandoned build output is the only part of an unacknowledged
tree a sweep can remove. The root and destructive operations use Go's confined
`os.Root`, so a symlink cannot redirect deletion outside the scratch root. While
planning, the sweep pins the identity (device and inode) of every path it may
delete through, the task, its `work` and each planned entry, before measuring
them. Inside the root, every removal opens the target's parent one real directory
at a time from the root, checking on each open handle that it is not a link, kept
its identity once opened, and is the directory the plan pinned. The last name must
also still be the one pinned: a directory is opened and proved the same way,
emptied through that handle, then removed only while empty; anything else, a link
included, is unlinked itself, never followed. The pins are also checked under the
sweep lock before any record is written. A path component swapped for a link, or a
different directory or file renamed onto a planned name, after planning therefore
keeps the task or fails the removal (reported as partial) instead of redirecting it
into another task. Not covered: a file renamed onto the last name in the instant
between its identity check and the unlink, or an empty directory renamed there
before the final empty-only removal, is removed in its place (on systems without a
directory-only removal, any file renamed there then), and entries moved into a
planned directory are removed with it. A failed inspection is reported as partial,
not as a clean observation.

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
sessions, and harness owned roots that ignore inherited temporary-directory
settings require explicit `scratch run` or remain report-only. A nested managed
task keeps its own lease and receipt; the parent must wait for it before handing
off its own scratch. All Git repositories (including bare clones, detached metadata
directories, and clean worktrees) are conservatively kept. The filesystem classifier recognizes
Git markers and repository storage/reference layouts without running Git or
following metadata links; partial or damaged layouts can therefore remain protected.
An ordinary output file named `HEAD` alone is not a repository marker. Use
metadata-free snapshots for disposable source copies, and `deskwt` for worktrees. This is not a second Git worktree pruner.

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

Maintenance streams directory entries and retains at most 262,144 inventory entries per walk/list and 64 directory levels. Exceeding either limit reports partial accounting. When the scratch root's own listing is over the limit, the sweep reclaims nothing. When one entry's walk is over it, that entry alone is kept, with the reason `over the maintenance inventory limit` in its report row and its name in the sweep's error, and the rest of the plan still runs: fully measured tasks beside it are reclaimed by the usual rules. The kept entry's bytes are what the walk counted before it stopped, a lower bound, so the budget rules can only reclaim less, never more. Any other failed inspection still withholds reclamation from the whole sweep. A sweep with an over-limit entry is partial, so the periodic sweep records a failed attempt rather than a stamp. A wider legacy inventory returns an error rather than presenting a prefix as complete. These are reversible local safety defaults, not latency objectives.

For an inventory wider than the default, explicitly raise the maintenance-only budget, for example `ASSAY_MAINTENANCE_ENTRY_LIMIT=524288 cellctl cache house clean` (or the existing scratch sweep command). Values must be 1 through 1,048,576. Invalid values and exhausted budgets fail closed; there is no unlimited fallback. This setting does not change retained-data protections, per-role authority or launch pressure policy. Trees beyond the finite ceiling or depth limit remain incomplete and require owner-assisted layout correction.

The default entry budget covers the representative fixture with 150,000 compiler-cache entries and 75,000 module-cache entries. The override remains finite; it is a maintenance resource budget, not a launch-policy or retention override.

## Periodic maintenance

A desk launched by `cellctl` runs the scratch sweep itself while its supervisor is
alive, so a long-running cell does not depend on an operator remembering to sweep.
The first sweep runs ten minutes after launch and later ones follow
`CELL_SCRATCH_SWEEP_INTERVAL` in `cell.env` (a Go duration of at least `1m`;
default `6h`; `0` turns periodic maintenance off). The sweep never runs on the
launch or completion path. Sibling supervisors in one cell share the same
maintenance lease and a `maintenance.stamp` file in the scratch root, so while
sweeps finish cleanly the cell sweeps at most once per interval however many desks
are running. A sibling that finds the lease busy skips quietly. The stamp is
written only after a sweep that finishes without errors or partial accounting. A
sweep that fails or ends partial writes `maintenance.stamp.attempt` instead, with
its time and how many sweeps in a row have failed, and no supervisor on the cell
starts another sweep before the retry delay has passed: ten minutes after the
first failure, doubling with each further failure, never more than the interval.
A missing or damaged attempt record counts as no failure, so the cell is due again
as soon as its stamp is. A supervisor that did not just run a sweep checks again
after ten minutes (or the interval, if shorter); the one that ran the failing sweep
waits a whole interval before its own next check. The retry therefore runs at the
first sibling check after the delay, or a whole interval later on a cell with one
supervisor, and siblings coalesce onto one retry rather than each sweeping in
turn. The next finished sweep writes the stamp and removes the attempt record. A sweep stopped with its
supervisor records nothing, so the next supervisor's check finds it due. A
periodic sweep waits at most ten seconds for a sibling launch's scratch
transaction and writes no lock-wait line to the desk's terminal. Stopping the supervisor cancels a sweep in
progress: it stops before its next deletion and reports partial accounting.
Each sweep appends one JSON summary line to `run/scratch-maintenance.log` beside
the scratch root (rotated once past 1 MiB), and a cadenced supervisor also
prints it on stderr. A failed sweep is logged and never stops the desk.

The periodic sweep applies the same policy as `sweep --apply`. That policy has
one narrow rule for tasks whose session died without acknowledging them. Such a task loses
only its compiler and test-binary output, and only when all of these hold:

- it has no receipt and is not resumable;
- its execution lease is free, so its owner is gone;
- its child process group is recorded as reaped or proved empty;
- its last recorded activity is older than the retention max-age.

The reclaimable output is a top-level `work/go-build<digits>` entry (the Go
toolchain's work directory) or a top-level `work/<tool>-bin-<suffix>` entry with
both parts non-empty (the `os.MkdirTemp` shape for built test and tool binaries,
such as `cellctl-bin-123` or `deskpins-bin-4`), which any rebuild regenerates; a
name such as `go-builder`, `go-build-notes`, `cellctl-bin-` or `-bin-x` is ordinary
work and stays. Git data is judged per entry: each matching entry is walked in
full, and one holding Git data anywhere inside it stays, while build output beside
a Git worktree in the same task can go. An entry that cannot be walked fails the
sweep closed: the task is kept and the sweep reports partial. Each planned entry
is checked again for Git data right before removal, and any change keeps the whole
task. The age test
uses the same max-age as the rest of the policy, so `CELL_SCRATCH_MAX_AGE=0`
makes an unacknowledged task's build output reclaimable at the first sweep after
its lease is free. Symlinks are never followed:
a `work` that is not a real directory is kept and never planned, every directory
above a deletion target is checked again right before removal, and the removal
itself walks those directories as open handles held to the identities the plan
pinned (see above). A link planted before planning keeps the task; a link or a
renamed directory put in place after planning keeps the task if it lands before
the deletion check and fails the removal if it lands after. Neither can redirect a
sweep into another task's workspace.
The record, diagnostics, output tail and every other workspace entry stay, so
an operator can still complete or inspect the handoff. The report row's
action is `reclaim-build`, and a dry run lists the same rows.
