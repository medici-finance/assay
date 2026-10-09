# Managed Go caches

Enable managed compiler and module caches through the existing `cell.env`:

```dotenv
CELL_GO_CACHE=on
CELL_GO_CACHE_BYTES=8589934592
CELL_GO_CACHE_MIN_FREE=10737418240
```

The shipped default is **off**. Enabling selects an 8 GiB logical-byte budget
and 10 GiB minimum available-space floor. These are configurable starting values,
not a filesystem quota or a promise about peak build size. Budget bytes must be
positive; a floor of `0` disables only the free-space floor. Invalid values refuse
launch. No host-wide scheduled job is installed.

To enable it for every cell on a machine, put the same lines in the machine-wide
defaults file, `$CELLS_ROOT/defaults.env`, instead of in each `cell.env`, and opt
one cell back out in its own file:

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
the defaults file, while `CELL_GO_CACHE_BYTES` and `CELL_GO_CACHE_MIN_FREE` may be
shared there. `cellctl check <cell>` prints the effective state and the layer that
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
lock. Under byte-budget or free-space pressure, only inactive compiler/module
roots are removed. Active consumers protect both caches, including module readers.
Go's read-only module directories are made writable through verified open
directory descriptors during inactive cleanup. No `go clean -cache` runs.

The same lock serializes cleanup and consumer enrollment. Active records survive
supervisor crashes and uncertain child-tree cleanup; PID age never proves
inactivity. Long-running roles can hold caches beyond the byte budget. Existing
tasks are never killed to meet a floor.

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
removes data only under pressure; it does not force a warm purge. Recovery only
clears crash custody after explicit operator confirmation. It neither kills tasks
nor removes cache data. Never confirm while a consumer is live.

## Measurements and platforms

The root's `report.json` and `previous.json` retain the latest two live reports
for next-cleanup comparisons. Each complete JSON record includes policy,
filesystem identity, UTC before/after timestamps, logical usage, available bytes,
skipped active records, reclaimed logical bytes and cleanup failures. Retention
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
