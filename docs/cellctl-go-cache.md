# Managed Go caches

Enable managed compiler and module caches through the existing `cell.env`:

```dotenv
CELL_GO_CACHE=on
CELL_GO_CACHE_BYTES=8589934592
CELL_GO_CACHE_MIN_FREE=10737418240
CELL_GO_CACHE_BUILD_MAX_AGE=24h
CELL_GO_CACHE_LOW_WATER_PERCENT=60
CELL_GO_CACHE_TRIM_MIN_AGE=2h
CELL_GO_TRIMPATH=on
```

The shipped default is **off**. Enabling selects an 8 GiB logical-byte budget,
a 10 GiB minimum available-space floor, a 24-hour compiler-cache trim age, a
60% low-water mark and a 2-hour trim safety floor. These are configurable starting values,
not a filesystem quota or a promise about peak build size. Budget bytes must be
positive; a floor of `0` disables only the free-space floor. The trim age is a
Go duration in whole seconds of at least `2h` (see "Trimming the compiler cache
under live consumers"). The low-water mark is a whole percentage of the budget
above 0 and below 75 (the high-water mark); the safety floor is a whole-second
duration of at least `2h`. Invalid values refuse launch. No host-wide scheduled job is installed; the `deskwt prune` sweep
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
the defaults file, while `CELL_GO_CACHE_BYTES`, `CELL_GO_CACHE_MIN_FREE`,
`CELL_GO_CACHE_BUILD_MAX_AGE`, `CELL_GO_CACHE_LOW_WATER_PERCENT` and
`CELL_GO_CACHE_TRIM_MIN_AGE` may be shared there. `cellctl check <cell>` prints the effective state and the layer that
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
`GOMODCACHE=<root>/mod`, `GOPATH=<root>/gopath`, `GOFLAGS` (see "Builds share
the cache across worktrees") and non-secret serialized `ASSAY_GO_CACHE_POLICY`. Codex receives these in its command environment;
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

## Builds share the cache across worktrees

Every consumer of a managed cache builds with `-trimpath`: the launcher exports
`GOFLAGS=-trimpath` beside `GOCACHE`, and records it in the policy. Without it,
`cmd/go` keys each package outside GOROOT on its absolute directory, so the same
code built in a new dispatch worktree — a fresh path per dispatch, review round
or verify run — is compiled again and stored under a new key, and the cache
grows by one full compile set per worktree however little the code changed.
With it, a second worktree at the same commit reuses the first one's entries.

An ambient `GOFLAGS` in the launching shell is extended, never replaced: the
launcher appends `-trimpath` unless it is already present. Opt one cell out with
`CELL_GO_TRIMPATH=off` in its `cell.env`, or every cell in `defaults.env`; the
launcher then exports no `GOFLAGS` and an ambient value passes through untouched.
Two effects of `-trimpath` to know before keeping it on: source paths in panics,
stack traces and `runtime.Caller` are module-relative rather than absolute, so
code that locates files beside its own source must not rely on them; and the flag
is part of every cache key, so builds with and without it do not share entries.
Consumers launched before the change carry no `GOFLAGS` in their recorded policy
and keep custody unchanged; relaunch them to share entries.

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
compiler cache is trimmed instead, as the next section describes.

### Trimming the compiler cache under live consumers

A record held by one long-running desk would otherwise stop the cache shrinking
at all, so the budget would only delay the next storage hold. Under the same
per-root lock, every live (non-dry-run) check trims the compiler cache when it
finds byte-budget or free-space pressure with consumer records held, or when
the cache's logical size is above the **high-water mark** (75% of the budget)
whether or not records are held. 75% is the margin the lock-free admission
pre-check (`Admit`) requires, so a cache trimmed below it lets the next launch
admit without waiting on the lock.

A trim lists every selected entry with its modification time and removes them
**oldest first** until the cache is at or under the **low-water mark**
(`CELL_GO_CACHE_LOW_WATER_PERCENT`, default 60% of the budget), so the next
few builds do not immediately trigger another trim. Past the low-water mark it
keeps going only through entries older than `CELL_GO_CACHE_BUILD_MAX_AGE`, so
everything the age trim always removed still goes. It never removes an entry
younger than the **safety floor** (`CELL_GO_CACHE_TRIM_MIN_AGE`, default `2h`,
never below `2h`), however far over the low-water mark the cache is: reaching
that floor first ends the trim with a reported shortfall. The floor is checked
again against each entry's modification time at removal, so an entry a build
looked up after the listing is kept. It selects the same names the Go toolchain's own trim selects: entries
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
  hour old. The safety floor is refused below `2h`, so it always leaves that
  refresh at least an hour of margin; the trim age, refused below
  `2h`, is never below it in practice.
- An entry missing at lookup is a cache miss: `go` rebuilds it, and never
  produces a wrong build from it. Two narrow windows remain. The first is a
  lookup that lands between the trim's age check and its removal. The second is
  a build step that opens an entry's path more than the safety floor minus an
  hour after looking it up. Either can fail that one build step, which is then
  retried. The Go toolchain's own trim accepts the same windows, with a 5-day
  margin where this one uses the configured floor.
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
nothing is trimmed, and the failure names the failing check. Such an entry
keeps admission `could-not-check` until it is removed. This fails safe: the
trim never runs on a cache it could not measure. Inside the trim, a directory
entry is checked in full before any of it is removed, and one bad member refuses
the whole directory. An entry that turns bad between measurement and the trim
is skipped, never followed, and reported, and the trim continues with the next
entry; the pass still ends `could-not-check`. Files are unlinked through the verified root, and directories are removed
by the same verified-descriptor path as inactive cleanup. If the trim clears pressure, admission passes with consumers
still running. If it does not, the result is `storage-deferred` as before.
Below the high-water mark and without pressure nothing is trimmed, so warm
caches stay warm. Above it, with consumers live, every pass trims down to the
safety floor where it must to reach the low-water mark, so the narrow windows
above apply on routine passes, not only under pressure. When what a trim can
never take (the module cache) alone holds the cache above the low-water mark,
emptying `build/` could not reach it: the trim then takes only entries past the
trim age and reports the shortfall as untrimmable, rather than evicting every
entry past the floor on each pass. Pressure with no consumer records still removes the compiler
and module roots whole, as before. An explicit `cellctl cache <cell> wipe` that
finds live records runs the same trim with a target of zero, so it removes every
entry older than the safety floor, and still reports `storage-deferred`,
because it could not discard the rest. A dry run
(`status`, `deskwt prune --dry-run`) trims nothing.

A policy recorded before these settings existed has no trim age, low-water mark
or safety floor; it trims at the compiled defaults. A setting left at its
default is not serialized, so a default policy's form is unchanged and
consumers launched by an older `cellctl` keep their custody.

Each report that trimmed carries a `low_water_trim` object: the trigger
(`pressure`, `high-water` or `wipe`), the target and floor it used, whether the
low-water mark was reached and, when the floor stopped the trim first, the
`shortfall_bytes` still above the target with a reason naming the floor.

The `deskwt prune` sweep runs the same pressure check on a cadence: each pass
discovers the marked roots under the cells root (a custom `CELL_GO_CACHE_ROOT`
outside it is out of the sweep's sight; `cellctl cache <cell> clean` remains
its instrument) and enforces each root's last recorded policy. It removes
whole roots only under pressure with no live consumer record, trims the
compiler cache as described above otherwise, and prints a count line only
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
removes data only under pressure or above the high-water mark; it does not
force a warm purge. With consumers live, that removal is the compiler-cache
trim only. Recovery only
clears crash custody after explicit operator confirmation. It neither kills tasks
nor removes cache data. Never confirm while a consumer is live.

## Measurements and platforms

The root's `report.json` and `previous.json` retain the latest two live reports
for next-cleanup comparisons. Each complete JSON record includes policy,
filesystem identity, UTC before/after timestamps, logical usage, available bytes,
skipped active records, reclaimed logical bytes, the number and summed logical
bytes of compiler-cache entries the trim removed (`trimmed_build_entries`,
`trimmed_build_bytes`), the trim's `low_water_trim` outcome and cleanup failures. The trimmed counts are what the
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
and proxy access are disabled.

### Mutation gate

Reproducible mutants live in `tools/desk/internal/cellcache/mutations.json`, and
`python3 internal/cellcache/mutate.py` (run from `tools/desk`, about ten minutes;
`--shard i/n` runs a slice) plants each one and requires a behaviour test
(`--- FAIL:`) to catch it. It plants them in a temporary copy of the module, so
the checkout is never written and a killed run leaves it clean. Every mutant ends
in one of three states: `CAUGHT`, `MISSED`, or `COULD-NOT-MUTATE` (the mutant
does not compile, its `file` is absolute, escapes the module or is not a file
there, its `old` text is empty, equals `new` or no longer matches, or the suite
timed out; `CELLCACHE_MUTATE_SUITE_TIMEOUT` sets the per-suite limit in seconds,
default 300). The gate exits 1 on any `MISSED` or `COULD-NOT-MUTATE`. The spec's
`control` mutation must be caught or the harness is reported broken (exit 2) with
no per-mutant verdict. SIGTERM and SIGINT stop the running suite and remove the
copy. The map uses its own runner rather than `muhar` because of one case:
`muhar`'s per-mutation run scores a mutant whose build fails as caught (its
schemata mode compiles mutants first and sets non-compiling ones aside, but
those still rebuild per mutation and count as caught), which is tracked in #2639.
Counting a build failure as a catch would hide exactly the stale mutations this
gate must surface.

CI runs the gate through the staged workflow
`ci/staged-workflows/cellcache-mutate.yml`; until a maintainer promotes it to
`.github/workflows/` the gate runs only when someone runs it. The
`TestMutateGate*` tests run on every PR and prove each state and guard above
against a throwaway module; each fails when the guard it covers is removed.

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
