# Cell maintenance

Status: implementation contract. Intent: operator request, 2026-10-11.
Source baseline: assay `18d752a4c`; current cache, cadence, comms and cockpit owners.

The existing recovery path requires manually finding orphan consumers, recovering
several checkpoints, inducing cache pressure, and provisioning comms credentials.
These commands replace that sequence with explicit operations:

| Command | Result |
| --- | --- |
| `cellctl cache CELL wipe` | Remove this cell's disposable build and module caches regardless of pressure. Keep policy, ownership, installed GOPATH binaries and other cell data. Active consumer records protect `mod/`, `gopath/` and whole-root removal; with records held, wipe removes only compiler-cache (`build/`) entries older than the trim age (`CELL_GO_CACHE_BUILD_MAX_AGE`), with or without pressure, and reports `storage-deferred`. |
| `cellctl reset CELL` | Stop this cell's owned processes and cockpit surfaces, clear stopped role/comms/cache custody, wipe caches, and remove all cell-owned worktrees. Preserve repository branches, comms messages, configuration, keys and login stores. |
| `cellctl reset CELL --hard` | Also discard comms messages and refresh every repository in `CELL_ROOTS`: fetch origin/main, remove all linked worktrees, reset the primary checkout to main, remove other local branches and untracked/ignored files. Preserve remote branches and operator configuration outside repositories. |
| `cellctl comms CELL init` | Generate the private signing seed, matching trust store, role bindings, reader and manifest, then select the manifest. Never print or commit credentials. Repeated init validates and reuses the existing setup. |

Soft reset preserves messages. Hard reset discards them. Both retain cell.env,
provider configuration and signing keys. If CELL_ROOTS is empty, hard reset uses
CELL_REPO; configured roots otherwise define its full repository scope.

Reset is a destructive operator command, not automatic housekeeping. `DRY_RUN=1`
prints the exact scope without killing processes, fetching, clearing messages,
removing worktrees or changing repositories. Development tests use local disposable
repositories and compiled process fixtures, never an operator's checkout.

Before deleting data, reset establishes maintenance exclusion, preflights paths
and Git registrations, stops attributable consumers (including detached children),
and verifies the owned leases are free. A live lock is never "fixed" by unlinking
its inode. Process identity and cell attribution are checked before signalling;
unrelated cells are excluded. Failures report the stage and whether process
quiescence was established. A failed preflight or prefetch can leave existing
consumers running. Once installed, the maintenance marker blocks new launches until
reset completes successfully. Failed
admission never licenses deletion outside planned roots. Shared repository fetch
locks are never reclaimed by reset: an existing lock refuses preflight, and hard
prefetch owns a newly created lock only for its fetch transaction.

Preflight inventories the selected signing keys, trust store, manifests, provider
configuration and login stores, including the actual source of the native reader's
auth link. It rejects overlap with every cleanup root or file: disposable cache
directories, removed worktrees, hard-reset repositories, discarded queue storage
and service-state files. It also rejects credentials at reset's own marker/lock
paths, before creating any maintenance controls. Credentials in those locations
must be moved before reset.
Equivalent filesystem paths cannot bypass retention checks: existing entries and
ancestors are compared by filesystem identity. Uncertain identity lookups refuse
reset; distinct entries on a case-sensitive filesystem remain distinct.

A reset plan must compare filesystem locations consistently when deciding whether
cell configuration is inside a repository or a worktree belongs to the cell.
Spelling a location through a symbolic link must not change that decision.
Paths that cannot be resolved safely must produce an explicit refusal rather
than an assumption that the location is outside the protected tree.

Planning resolves both sides of directory comparisons, including aliases of
cell and worktree paths. Hard reset refuses nested cell configuration, repository
roots, and comms storage. Soft reset selects only the cell's worktrees, and
execution uses that same selection. Missing optional storage is allowed; broken
links, unreadable paths, and non-directory paths are errors. The worktree
directory itself must still be a real directory before cleanup.

`DRY_RUN=1` prints the plan without executing cleanup. The planning, preflight and
execution regressions use explicit symbolic links, temporary repositories and a
symlinked `TMPDIR`, so Linux runs exercise the path spelling macOS produces by default.

Unix cache/reset support initially covers macOS and Linux. Unsupported platforms
refuse before mutation. Existing owner, no-link and single-filesystem cache checks
remain in force. The cache lock wait must accommodate full measurements and cleanup
transactions, rather than timing out before one normal scan finishes. The selected
default is 60 seconds; the alternative was the previous 5-second wait, which refused
normal full-cache scans. This is a reversible constant with no migration or stored
state: restoring 5 seconds shortens contention waits and restores those scan refusals.

Comms init defaults to the configured Codex fast-tier model/effort and a native
contained reader shipped in cellctl. It uses a private reader config with a link to the existing Codex auth.json login, no copied
credential values or floating npm package. The reader accepts only default/read-only
ACP mode, creates no execution environments or dynamic tools, disables connectors,
plugins, hooks, shell, browser and child-agent features, and rejects callbacks.
An explicitly supplied external ACP reader requires command, model, version pin and
`--reader-contained` attestation;
the existing containment attestation and custody checks still apply.

Acceptance: wipe below budget actually empties disposable caches; a live consumer
prevents a whole-cache wipe (wipe then trims only aged compiler-cache entries); reset kills a detached fixture child but leaves another cell alive;
soft reset preserves mailbox contents and local branches; hard reset affects all
configured fixture repositories and no outside repository; dry-run performs no
mutation; failed fetch precedes Git destruction; init creates matching Ed25519
material with owner-only custody and is idempotent; the native reader emits no
filesystem/execution tools and passes inert classification and refusal fixtures.

Herdr is scoped by cell. `up` creates one workspace labelled with the cell name;
role tabs and the comms tab use that workspace explicitly. Repeated starts reuse
its private receipt. Workspace metadata carries a random ownership token, so a
recycled server ID cannot authorize closing another cell's workspace. `down` and
`reset` close the recorded cell workspace. Existing unrecorded tabs are never
adopted automatically; migrate them by closing them and starting the cell again.
This requires Herdr workspace metadata support.

The two-session integration test `TestTwoSessionsCommsHelloRoundTrip` builds the
shipped client, gateway, drain and contained-reader binaries. Distinct worker and
reviewer session contexts exchange hello payloads in both directions through the
private socket and actual queue/mailbox paths. It verifies sender attribution,
repeatable polling, durable acknowledgments and empty mailboxes afterward. Only
the model's classification responses use a deterministic offline fixture.
