# Managed Go caches

Enable managed compiler and module caches through the existing `cell.env`:

```dotenv
CELL_GO_CACHE=on
CELL_GO_CACHE_BYTES=8589934592
CELL_GO_CACHE_MIN_FREE=10737418240
CELL_GO_CACHE_BUILD_MAX_AGE=24h
```

The shipped default is **off**. Enabling selects an 8 GiB logical-byte budget,
a 10 GiB minimum available-space floor and a 24-hour compiler-cache trim age. These are configurable starting values,
not a filesystem quota or a promise about peak build size. Budget bytes must be
positive; a floor of `0` disables only the free-space floor. The trim age is a
Go duration in whole seconds of at least `2h` (see "Trimming the compiler cache
under live consumers"). Invalid values refuse launch. No host-wide scheduled job is installed; the `deskwt prune` sweep
pressure-checks every marked cache root under the cells root on each pass as
the cadence backstop (see "Launch, cleanup and recovery").

To enable it for every cell on a machine, put the same lines in the machine-wide
defaults file, `$CELLS_ROOT/defaults.env`, instead of in each `cell.env`, and opt
one cell back out in its own file. `cellctl new` writes that file with these keys
already in it, commented out at their compiled defaults (`cellctl defaults init`
writes it for a cells root that has none), so enabling the cache is uncommenting
one line and changing `off` to `on`:

```dotenv
# $CELLS_ROOT/defaults.env — every cell under this cells root
CELL_GO_CACHE=on
```

```dotenv
# <cell-dir>/cell.env — this cell only
CELL_GO_CACHE=off
```

A cell's `cell.env` overrides the defaults file key by key, and the defaults file
overrides the shell `cellctl` was started from. Each cell still gets its own root
and its own budget: `CELL_GO_CACHE_ROOT` names one cell's root and is refused in
the defaults file, while `CELL_GO_CACHE_BYTES`, `CELL_GO_CACHE_MIN_FREE` and
`CELL_GO_CACHE_BUILD_MAX_AGE` may be shared there. `cellctl check <cell>` prints the effective state and the layer that
supplied it, for example `managed Go cache: on (CELL_GO_CACHE=on from defaults
file)`. Container cells ignore the setting, and on a platform the cache does not
support a machine-wide `on` refuses every launch just as a per-cell `on` does. See
[Machine-wide defaults](cellctl.md#machine-wide-defaults--defaultsenv).

## Scope and trust

The default root is `<canonical-cell-directory>/go-cache`. Optional
`CELL_GO_CACHE_ROOT` must be a clean absolute path with an existing parent. The
manager creates the final directory with mode 0700 and an ownership marker
bound to the canonical cell directory. Existing unmarked roots, another cell's
marker, symlink components, foreign ownership and unsafe write permissions are
refused. Configure a new empty path to move storage; never adopt a user cache.
Ambient `GOCACHE`, `GOMODCACHE` and `GOPATH` are not overrides for this feature.

The cell is the trust boundary: its tasks share writable cache data, while
distinct cells do not. This is cooperation within one OS account and cell, not
a sandbox against a malicious process with that account's privileges. Only
managed consumers may use these paths. Do not export them into unrelated shells
or delegate cache users to services that outlive process supervision.

Before HOME changes, the launcher composes `GOCACHE=<root>/build`,
`GOMODCACHE=<root>/mod`, `GOPATH=<root>/gopath`, and non-secret serialized
`ASSAY_GO_CACHE_POLICY`. Codex receives these in its command environment;
scrubbed cells add only these validated values to their composed environment.
Changing task HOME does not create another compiler cache. Launch plans print
the effective root, trust-domain digest and policy without initializing a cache.

The byte budget covers `build` and `mod` together on one filesystem. Traversed
entries must have the root's device identity. Symlinks, hardlinks, foreign owners
and filesystem crossings refuse measurement/cleanup, preventing alias counting
or traversal into another storage domain. Each cell has its own byte budget;
cells on the same filesystem sample the same available-space pool. There is no
aggregate cross-cell quota or reservation for future build writes. `gopath` is
stable but never reclaimed: installed executables are not disposable compiler
or module cache entries. Scratch, worktrees and session evidence are excluded.

## Launch, cleanup and recovery

Managed harness launches acquire durable cache-consumer custody before starting
a child. Host interactive and cadence paths use the existing process supervisor.
Scrubbed tmux panes retain a `cellctl cache-run` supervisor inside the pane, so
detaching the caller does not end custody. Interactive streams remain terminal
descriptors; cache management adds no output pipes or log tee. Container
launchers must configure caches inside their own runtime: no host root is mounted
automatically.

Ordinary completion keeps warm caches. Every managed launch, cadence pass,
`deskdispatch`, and explicit `clean` measures usage under a per-root interprocess
lock. Under byte-budget or free-space pressure with no active consumer record,
the compiler and module roots are removed whole. Go's read-only module
directories are made writable through verified open directory descriptors during
that inactive cleanup. No `go clean -cache` runs.

Under pressure **with** active consumer records, the module cache and `gopath`
are untouched: active records protect them, including module readers. The
compiler cache is age-trimmed instead, as the next section describes.

### Trimming the compiler cache under live consumers

A record held by one long-running desk would otherwise stop the cache shrinking
at all, so the budget would only delay the next storage hold. Under the same
per-root lock, and only under byte-budget or free-space pressure, every live
(non-dry-run) check removes compiler-cache entries whose modification time is
older than `CELL_GO_CACHE_BUILD_MAX_AGE`, whether or not consumer records are
held. It selects the same names the Go toolchain's own trim selects: entries
named `*-a` or `*-d` inside the 256 two-hex-digit subdirectories of `build/`.
A `*-d` entry is usually a file, but the executables that `go run` and `go tool`
cache are `*-d` **directories**; an aged one is removed whole, as the toolchain
removes it. Other files under `build/`, the whole of `mod/` and `gopath/` are
never selected. Where it differs from the toolchain is custody: the toolchain
removes whatever it selects, while this trim first applies the checks below and
skips anything that fails them.

This is safe with consumers live because of how the Go toolchain uses its cache:

- `go` refreshes an entry's modification time when it looks up one that is more
  than an hour old, so an entry a running build has looked up is at most about an
  hour old. The trim age is refused below `2h`, which leaves that refresh an
  hour of margin.
- An entry missing at lookup is a cache miss: `go` rebuilds it, and never
  produces a wrong build from it. Two narrow windows remain. The first is a
  lookup that lands between the trim's age check and its removal. The second is
  a build step that opens an entry's path more than the trim age minus an hour
  after looking it up. Either can fail that one build step, which is then
  retried. The Go toolchain's own trim accepts the same windows, with a 5-day
  margin where this one uses the configured age.
- Removing a file another process has open does not disturb that process on
  macOS or Linux.

Module directories have neither property: they carry no use stamp, and a module
removed under a running build cannot be refetched mid-build. That is why `mod/`
stays behind the consumer records.

The trim uses the same checks as measurement. An entry, or any member of a
directory entry, that is linked, foreign, on another device or writable by
others is refused, and nothing is followed through it. Measurement runs first
and applies those checks to all of `build/` and `mod/`. A bad entry anywhere
there therefore makes the pass `could-not-check` **before the trim runs**:
nothing is trimmed, and the failure names the first bad member. Such an entry
keeps admission `could-not-check` until it is removed. This fails safe: the
trim never runs on a cache it could not measure. Inside the trim, a directory
entry is checked in full before any of it is removed, and one bad member refuses
the whole directory. An entry that turns bad between measurement and the trim
is skipped, never followed, and reported, and the trim continues with the next
entry; the pass still ends `could-not-check`. Files are unlinked through the verified root, and directories are removed
by the same verified-descriptor path as inactive cleanup. If the trim clears pressure, admission passes with consumers
still running. If it does not, the result is `storage-deferred` as before.
Without pressure nothing is trimmed, so warm caches stay warm; an explicit
`cellctl cache <cell> wipe` that finds live records runs the same trim and still
reports `storage-deferred`, because it could not discard the rest. A dry run
(`status`, `deskwt prune --dry-run`) trims nothing.

A policy recorded before this setting existed has no trim age; it trims at the
24-hour default. Its serialized form is unchanged, so consumers launched by an
older `cellctl` keep their custody.

The `deskwt prune` sweep runs the same pressure check on a cadence: each pass
discovers the marked roots under the cells root (a custom `CELL_GO_CACHE_ROOT`
outside it is out of the sweep's sight; `cellctl cache <cell> clean` remains
its instrument) and enforces each root's last recorded policy. It reclaims
only under pressure, removes whole roots only with no live consumer record
(otherwise only the compiler-cache age trim runs), and prints a count line only
when it acts. A root whose owning directory the sweep itself removed
is reaped as an orphan, gated on its lease set re-read under its own lock.

The same lock serializes cleanup and consumer enrollment. Active records survive
supervisor crashes and uncertain child-tree cleanup; PID age never proves
inactivity. Long-running roles can hold the module cache, and compiler-cache
entries younger than the trim age, beyond the byte budget. Existing tasks are
never killed to meet a floor.

If safe cleanup cannot satisfy policy, `deskdispatch` reports typed
`storage-deferred`, exit 6 and JSON measurements **before** credentials, claims,
worktrees or prompts. All execution kits are conservatively treated as potentially
storage-heavy, including review and verification. The item remains queued.
Missing measurements are `could-not-check`, also exit 6. Retry the same item after
space returns: the gate measures again and normal claim deduplication still
applies. A held harness launch starts no child; cadence records a failed pass
and can retry at its next interval.

```sh
cellctl cache example status
cellctl cache example clean
# After externally checking ALL consumers and surviving children have stopped:
cellctl cache example recover --confirm-stopped
```

`status` is dry-run: no initialization, removal or report rewrite. A missing root
is could-not-check, never invented as zero usage. `clean` writes a report and
removes data only under pressure; it does not force a warm purge. With consumers
live, that removal is the compiler-cache age trim only. Recovery only
clears crash custody after explicit operator confirmation. It neither kills tasks
nor removes cache data. Never confirm while a consumer is live.

## Measurements and platforms

The root's `report.json` and `previous.json` retain the latest two live reports
for next-cleanup comparisons. Each complete JSON record includes policy,
filesystem identity, UTC before/after timestamps, logical usage, available bytes,
skipped active records, reclaimed logical bytes, the number and summed logical
bytes of compiler-cache entries the age trim removed (`trimmed_build_entries`,
`trimmed_build_bytes`) and cleanup failures. The trimmed counts are what the
trim removed, barring a concurrent removal or a partially failed directory
removal; reclaimed bytes are a before/after difference that concurrent
builds can blur. Retention
is bounded; export reports if longer history is needed. Status and dispatch also
emit current JSON. Failure to persist a live report refuses admission.

Usage and reclaimed bytes are **logical**, not allocated blocks. Sparse files,
compression and copy-on-write mean logical reclamation need not equal space
gained; free space is measured again after cleanup. Unavailable measurements are
JSON `null` with a partial/could-not-check outcome, never zero or success. Active
builds can change files during the walk; a disappearance conservatively defers
admission for retry. This is a sampled budget, not a hard write quota.

macOS and Linux use native device/owner metadata, available blocks and advisory
locking. Windows and other platforms explicitly refuse opt-in; cache-off launch
behavior is unchanged. Cross-compilation is not native Windows runtime proof.

Focused fixtures use small synthetic files, race consumers and cleanup including
path replacement, check uncertain custody and typed dispatch recovery, and build
an import-free Go package twice with different HOME values. Toolchain downloads
and proxy access are disabled. Reproducible mutants live in
`tools/desk/internal/cellcache/mutations.json`.

## Dispatch destination volumes

On macOS and Linux, `deskdispatch` also checks the item checkout (`--root`) and
its process temporary directory before any forge read, claim, or worktree creation.
These destinations may be on different filesystems from the managed cache. They
inherit the managed cache minimum-free-space floor. Set `DESK_MIN_FREE_BYTES` in
the dispatch process environment to select a separate nonnegative byte floor,
including for dispatches without managed Go caches. With neither setting present,
existing admission behavior is unchanged; `0` disables these destination checks.
The cache's own budget and floor remain governed by its existing policy.

Each checked destination emits a `storage-volume` JSON report with the configured
floor, available bytes, path, dry-run state, and admission outcome. Insufficient
space or an unreadable measurement returns exit 6 (`storage-deferred`) before
claiming work. Free space and retry the same item. Dry-run performs the same
read-only measurements and reports the hold. This is a point-in-time admission
check, not a storage reservation or active-task quota; other writers can consume
space after admission. It does not measure arbitrary custom output destinations.
An explicit nonzero floor on an unsupported platform fails closed.
