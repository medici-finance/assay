# Local read-only caller authority

The canonical owner is `tools/desk/internal/forgeservice/auth`. It implements the
bounded offline mechanism approved in
[the human design ruling](https://github.com/medici-finance/assay/issues/5428#issuecomment-6631318014).
This supplies local mirror-query authentication. Remote machine mTLS, lighter
external signed-token ingress and write/executor custody retain their existing
owners and acceptance requirements.

`NewLauncher` takes trusted operator policy and sealed `cellcontainer` runtime
handles. Fresh engine observations bind the exact owned producer and caller
containers. The fixed helper starts a fresh empty controller; it does not restore
authority from saved records. Explicit private Initialize and Register operations
create the signing identity and finite caller grants. Revocation, epoch changes,
rotation and stop remain serialized private operations. Unknown control replies
hold the writer until exact version/operation read-back; no blind retry occurs.

The controller reuses `comms/contract.Ed25519Signer` and the existing exclusive
`deskkit.HeldLock`. Its key stays in the producer's memory. There is no key import,
export, generic signing endpoint, SQL authority store or handler issuer. Public
bootstrap and verifier records are policy/provenance, never current authority.

`Verifier.Authorize` binds issuer/audience, caller/actor/role/cell, canonical
repository and issue/change kind, source/composition/admission, mirror-query only,
incarnation/generation/key, access/config/policy epochs, limits and expiry. It
requires a fresh challenge-bound signed current reply from the live producer.
`CheckRelease` is the final single-use check after encoding and other callbacks.
`BindRefresh` supplies the existing `refresh.CallerAuthority` contract and obtains
fresh authority for each check; it grants no execution ownership or source access.

Grant and request leases retain actual process-local monotonic deadlines. Wall
expiry is independently checked, including after current-state I/O and at release.
Proof arrival cannot restart a lease. Cancellation cannot be revived with a new
context. Every lease/skew/budget and issuer/audience is an explicit trusted input;
the fixture values are not operational defaults. Canonical `refresh.Limits.Validate`
checks the existing policy before resource/key/listener contact.

The closed `cellcontainer` extension preserves all legacy interactive/console
validation. Its producer mounts private/current/bootstrap and selected grant
volumes RW. Query gets only current/bootstrap RO; each caller gets its own grant
RO; sibling gets none. Fixed UID501 image directories initialize owned named
volumes by actual copy-up without a root init helper. Source-bound image metadata,
current full container identity, ownership, mounts and isolation are rechecked
before use. Operator-supplied binary/engine/image paths are trusted composition
inputs; their spelling alone is not authentication or qualification.

The original exclusive lock refuses active or uncertain owners. After actual old
producer removal, a new owner may reclaim only an exact fixed-channel socket with
current lock/root/inode/type checks. Non-sockets, symlinks, replaced roots/locks
and duplicate channel starts refuse. No stale holder is signalled or recovered.
Restart begins not-ready, generates a new identity/key on explicit initialization,
and requires fresh caller registration; old grants remain held.

The source-bound `testdata/qualify_fixture.py` entrypoint requires explicit owned
source/work paths, Go/Python/Docker binaries, caches, Unix engine and platform.
It builds FROM scratch without pull/network, executes real independently isolated
producer/query/two-caller/sibling flow and compiling production faults, checks
exact restoration and independently checks every resource's cleanup. Missing
profile/Docker input is UNQUALIFIED, never a named custody PASS. The ordinary Go
floor checks compatibility and CI wiring; the separate hosted Docker job must
actually pass before its required aggregate succeeds. Prospective workflow text
or a local platform result does not qualify an unrun hosted platform.

Local Linux/arm64 qualification covers the actual selected source-bound profile,
current protocol, revocation, lease, restart and custody fault witnesses. Formal
review, independent verification and actual hosted qualification remain separate.
The network-none/credential-free verifier fixture does not qualify the later
service's SQL/provider connectivity, private listener or admitted native read
credential custody. That consumer must compose and qualify those actual sources
without the R3 signer/control key; no new source proxy or write prerequisite is
introduced.

Production restore and whole-VM memory rollback remain UNQUALIFIED and HELD until
the controlling independent non-restored anchor and actual identity/key rotation
are demonstrated. No operational issuer, credential, entitlement, lease value,
deployment or activation is assigned by this implementation.
