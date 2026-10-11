# Cell maintenance

Status: implementation contract. Intent: operator request, 2026-10-11.
Source baseline: assay `18d752a4c`; current cache, cadence, comms and cockpit owners.

The existing recovery path requires manually finding orphan consumers, recovering
several checkpoints, inducing cache pressure, and provisioning comms credentials.
These commands replace that sequence with explicit operations:

| Command | Result |
| --- | --- |
| `cellctl cache CELL wipe` | Remove this cell's disposable build and module caches regardless of pressure. Keep policy, ownership, installed GOPATH binaries and other cell data. Active consumers still refuse deletion. |
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
unrelated cells are excluded. Partial failures report the failing stage and leave
the cell stopped; failed admission never licenses deletion outside planned roots.

Unix cache/reset support initially covers macOS and Linux. Unsupported platforms
refuse before mutation. Existing owner, no-link and single-filesystem cache checks
remain in force. The cache lock wait must accommodate full measurements and cleanup
transactions, rather than timing out before one normal scan finishes.

Comms init defaults to the configured Codex fast-tier model/effort and a native
contained reader shipped in cellctl. It uses a private reader config with a link to the existing Codex auth.json login, no copied
credential values or floating npm package. The reader accepts only default/read-only
ACP mode, creates no execution environments or dynamic tools, disables connectors,
plugins, hooks, shell, browser and child-agent features, and rejects callbacks.
An explicitly supplied external ACP reader requires command, model, version pin and
`--reader-contained` attestation;
the existing containment attestation and custody checks still apply.

Acceptance: wipe below budget actually empties disposable caches; a live consumer
prevents wipe; reset kills a detached fixture child but leaves another cell alive;
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
