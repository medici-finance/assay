# Local forge caller authority — Task1 design proposal

Status: draft proposal under forge-neutral/57. This document selects no operational
issuer, credential, endpoint, image, lifetime, policy value or deployment. Runtime
pickup waits for review of this concrete mechanism proposal and its actual fixture
prerequisites. A proposal or signature roundtrip grants no caller/custody acceptance.

## Outcome and retained boundaries

An explicitly configured local launcher supplies a read-only caller capability, and
an independently current operator authority checks it again before protected rows
are released. The query role cannot sign a stronger grant, change current authority,
or obtain authority merely by presenting a scope string or sharing a Unix UID.

The controlling architecture is [proposal22](../docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
with [proposal21](../docs/proposals/non-ci-architecture/21-cell-operator-surface.md)'s
external-machine mTLS/light-client signed-token class retained separately. This
proposal is only the LOCAL launcher class. It adds no external listener, remote
OIDC/workload credential, executor, outbox, execution claim or provider credential.
The later read-only handler in forge-neutral/56 still checks application policy,
source admission and the canonical mirror/store independently.

[Operator incarnation v1](operator-incarnation-v1.md) remains authoritative:
operator signing custody is outside PostgreSQL and the role's readable control
mount; restored records are provenance, not current authority; reconstruction and
ownership transfer rotate both incarnation and key reference. A read grant does
not relax any write/executor or reconstruction requirement.

## Architecture pickup

Inspected actual main source `c2a42d69402f96ec7a1812f4996197452e57344c`, the current
57 contract, and the actually published [wire source](https://github.com/medici-finance/assay/pull/5425)
at `b188d292986900ac5d4b0788bf2dadc17e4b7226`. These sources are genuinely merged
into the owned implementation history; their lifecycle/human acceptance is separate.
The [Go engineering policy](../docs/go-engineering.md) governs reuse.

Observed existing components:

- `comms/contract/identity.go` supplies an Ed25519 signer and verification
  primitives. Its existing Assertion signs cell, role, message, nonce and a window;
  it does not contain forge identity/epochs or a current-state producer. Its
  default TTL/skew and optional single-use replay port are not read-grant policy.
- `celllaunch` binds an independently permitted launch specification and observed
  session/process/container identity. Its records and Permit are explicitly not
  live caller authority. The existing observation adapters must establish actual
  ownership; a request or saved SessionRecord cannot stand in for them.
- `cellprocess` supervises an owned process tree and explicitly is not a sandbox
  against a hostile harness. Same-UID host processes are not a qualifying profile.
- `cellcontainer` already describes an explicit local engine, immutable image,
  separate role volumes, read-only root filesystem, dropped capabilities,
  no-new-privileges and protected mount checks. It is a containment candidate.
  Existing role App-key mounts do not supply or qualify this authority's key.
- Current incarnation/control predicates and newly inspected cellctl role/stop
  observations supply no live forge producer. Implementing that producer is 57's
  work, rather than a dependency on the downstream write executor.

Both comms and forgeread are existing direct desk-module requirements with local
module replacements. Their public packages are importable here; their internal
packages are not. No new dependency version or generic JWT/auth framework is
proposed. Disposition: continue bounded design; hold runtime mechanism selection
until the profile/current-source and fixture issues below are resolved.

## Proposed mechanism for review

The preferred assessment candidate puts the operator-owned controller in a dedicated
trusted producer container, alongside separate query and sibling containers on the
SAME explicitly selected Linux engine. The trusted launcher/modular Cell application
supervises this component outside every query/untrusted role container. It owns
live current state and the signing handle. It has no default listener or startup
provisioning. Embedding must explicitly select and supervise it; this library is
not a new standalone daemon or general credential broker.

The controller is a NEW concrete R3 implementation under `forgeservice/auth`.
It is not an injected predicate whose caller can echo a grant. It owns a serialized
control writer, private signing handle, current grant registry, revocations and
current generation/epoch/incarnation state. Trusted launcher composition is its
only writer. Query composition receives a verifier and a read-only current-source
client, without a writer or signer handle. No authority is read from the mirror,
cache, restored PostgreSQL or a Git revision.

Reuse the existing Ed25519 signing primitive rather than the existing comms
Assertion as a forge credential. The proposal needs one closed, domain-specific
grant payload covering all fields below. A fixed typed payload uses the existing
JSON library and exact deterministic bytes under a distinct domain prefix; no
agent-selected algorithm/key, generic claims map or second serializer is added.
The verifier obtains the current public key from the protected current-source
binding, never from the presented grant. Exact encoding/signature rules must be
pinned before code; cryptographic primitive reuse is not issuer/custody acceptance.

The operator-only control writer registers a launch only after independent Permit
and live resource ownership checks. It derives stable caller/represented actor,
role and cell from trusted launcher policy, binds the exact observed launch and
source profile, and issues only the predetermined mirror-query grant. Neither
request headers/environment nor a caller-supplied repository/admission selects it.
A failed or uncertain ownership observation registers nothing.

### Exact claim and current-state inventory

A grant binds issuer, audience equal to one cell, stable caller, represented actor,
role, cell, exact provider/host/repository, source ID, composition ID, admission ID,
operation equal to mirror-query, incarnation ID, authority/launch generation,
policy epoch, configuration epoch, access epoch, grant ID, key reference, issued-at
and finite expiry. No write logical identity, effect digest or arbitrary operation
is accepted in this read-only contract.

The live registry independently stores each permitted tuple and its revocation
state. Verification compares every field with that current tuple and actual
trusted consumer policy. A valid signature over another tuple remains refused.
Read grants may be reused only while the exact tuple remains current; they are not
comms message nonces or single-logical-write authorizations. Missing/unknown grant,
revocation, clock uncertainty, malformed claims and unavailable current state hold.

Every duration and bound is an explicit validated operator input: maximum grant
lease, permitted clock skew, current-read validity/deadline and revocation bound,
plus finite registry/channel/count/byte/time budgets. No numeric default or
primitive default is imported. The implementation must reject missing inputs
before any listener, process or source contact. Capacity refusal cannot evict live
revocation/current-authority state and later accept a revoked grant.

### Concrete channel and containment candidate

The local-container candidate separates two narrow per-launch channels:

1. A grant-delivery endpoint supplies only that launch's already authorized grant.
   It accepts no requested role, key, repository, source, algorithm or arbitrary
   payload to sign. This is launcher capability delivery, not generic minting.
2. A query-service current-state endpoint permits bounded fresh reads of current
   verification state only. There is no control mutation, key export, issuance or
   operator action in its protocol. Verification binds its response to the exact
   request, live producer instance and deadline; no cached read becomes authority.

Their Unix endpoints live in separate per-channel named volumes on that Linux engine,
not in a presumed macOS-host socket bind. The producer creates the instance's socket
resources in those volumes. The producer's key and writable control resources have
an additional producer-only volume, with no query/sibling mount.
The proposed server bootstrap pins that instance's public verification key and
key reference in its explicit read-only launch profile; a grant cannot replace
that trust input. A current-read request carries a fresh cryptographic challenge.
The producer signs a separate domain-specific response binding that challenge,
the exact presented-grant digest, its live instance, current tuple/revocation
state and response time/deadline. The verifier checks the pinned key, challenge,
tuple and explicit current-read lease after I/O. Replaying an earlier response,
replacing a socket peer or returning a new public key cannot establish authority.
Rotation makes an old bootstrap hold until explicit trusted re-bootstrap.
Only the explicitly corresponding container receives the required endpoint mount.
Neither query nor sibling containers mount the signing location, writer channel,
operator home, engine socket or arbitrary host directories. The engine administrator, trusted producer and launcher
are outside the untrusted-role threat boundary. Socket modes are defense in depth;
actual namespace/mount isolation and signed/current tuple checks establish the
boundary. A same-UID host harness is explicitly unqualified.

Existing `cellcontainer` mount/runtime validation must be reused for the entire
profile. The same-engine volume view is proposed as follows:

| Resource | Producer | Query verifier | Its intended caller | Sibling |
|---|---|---|---|---|
| Private key and control writer volume | Read/write | Absent | Absent | Absent |
| Current-read channel volume | Read/write | Read-only mount | Absent | Absent |
| Verification bootstrap volume | Read/write | Read-only mount | Absent | Absent |
| One launch's grant channel volume | Read/write | Absent | Read-only mount | Absent |

A read-only socket-directory mount does not make RPC operations read-only. The
current channel implements only its closed read/challenge operation; the grant
channel only returns the prebound launch grant. Neither accepts a control write
or arbitrary signing input. Actual fixtures must prove socket connectivity,
filesystem write denial, mount exclusion and wrong-channel refusal separately.
No container receives the engine socket, host PID namespace or a privileged/
capability-bearing runtime profile. A channel volume is never the key volume.

[OrbStack's architecture](https://docs.orbstack.dev/architecture) places its engine
in the Linux VM; [its volume documentation](https://docs.orbstack.dev/docker/file-sharing)
says named-volume data is Linux-side. [Docker's volume contract](https://docs.docker.com/engine/storage/volumes/)
supports different read-only/read-write views. These facts motivate the candidate;
they do not prove AF_UNIX socket or role-isolation behavior. Arbitrary host-socket
forwarding across macOS/Linux is an unqualified portability assumption, not a
proven unsupported feature. The real fixture must test the selected engine/profile.
 Its current general role plan also mounts a role App key and has no
read-only authority-channel profile; it cannot silently be reused as this profile.
A narrowly closed profile/composition extension is therefore required, with source
scope and actual consumers amended before implementation. It must omit provider
credentials and admit only the two specific channel resources. No arbitrary
mount/transport builder is proposed.

The launcher integration is NEW composition over existing celllaunch Permit and
actual ownership observations. It keeps capabilities out of persistent LaunchSpec
Args/Env, ordinary observations, snapshots, logs and performance records. There is
no operational launch during implementation qualification.

## Revocation, rotation, restart and restore

The controller serializes updates and current reads. It reuses deskkit's existing
nonblocking exclusive file-lock primitive over a bound protected control-root
file; lock-holder records are diagnostic only and automatic stale-holder signaling
is disabled. The descriptor is not inherited by role processes. Before minting or
answering a current challenge, it must still own the gate and the configured
root/path must name the same opened filesystem identity. Replacing/restoring the
lock file therefore makes the old producer hold, rather than allowing two live
controllers over different inodes. Uncertain identity/ownership refuses.
Revocation or an epoch change
invalidates the old tuple; no refresh/verification operation extends its lease.
The server performs current validation after protected data I/O and at the final
release boundary, with cancellation/deadline checks after current-source I/O.
The chosen linearization point and maximum revocation/entitlement lease must be
explicitly qualified; already returned bytes cannot be recalled.

A live-instance anchor consists of the independently supervised producer container
and process identities, exclusive ownership of its control resource, a fresh instance identity
and current signing/incarnation state. Starting from disk records alone cannot
construct current authority. On producer restart the proposed conservative mode
invalidates all old grants, obtains a fresh incarnation and signing key, creates
new instance channels, and requires explicit trusted re-registration. It does not
automatically replay a saved grant registry. Ordinary failover without rotation is
outside this first profile until intact currentness is independently proved.

Rotation stops old issuance, invalidates old tuples/channels and rotates both
identity and key reference. An old verifier unable to establish the new binding
holds until trusted re-bootstrap; it cannot learn a replacement trust root from a
grant. Actual cleanup/fencing and old-process uncertainty must be qualified, not
inferred from an empty lease file or a PID number.

Restored filesystem records are ignored as current authority and cannot revive a
live instance. The fixture must restore matching old control/SQL provenance and
prove a new producer still refuses it. Whole-host or VM memory snapshot rollback
can restore a process, key and kernel lock together; a local lock alone cannot
qualify that case. It is UNSUPPORTED by this first candidate unless an independently
non-restored operator anchor and real rotation/fencing procedure are qualified.
No lock detects arbitrary rollback. This boundary does not weaken custody1A:
restored operator state still must rotate identity AND key before any authority
is reopened, and this profile never automatically resumes restored authority.
No universal restore protection or available external anchor is claimed.

## Supported alternatives and hard prerequisites

The existing restricted local-container profile is the preferred candidate because
its actual mount/process boundary can separate same-host roles without changing
R7 or requiring an operational write executor. Its prerequisites are an explicit
owned local engine, an admitted immutable fixture image/platform, exact channel
mount/profile support and actual independent-process tests. None has been probed
or selected by this document.

A distinct-OS-principal host profile is a bounded alternative only if the existing
launcher supplies a qualified principal transition, protected current-source
channel and independent key/control denial tests. Current cellprocess supervision
and same-UID file modes do not supply it. Adding such a platform mechanism would
need its own concrete owner/source assessment; it is not a silent fallback.

External mTLS/OIDC/workload authentication is another caller class, retained with
its existing owners. It is not a substitute for local role isolation. A passive
file/current-source port, a signature roundtrip, socket ownership alone or a fake
authority is not an eligible alternative.

Hard prerequisites before runtime selection: review the proposed live gate,
pinned bootstrap and challenge-response current-source design; settle the
non-restored anchor/snapshot scope, the closed existing-container profile
extension and supported fixture platform. The gate, channel response and launch
composition are new R3 code; deskkit file locking, existing signing, container
runtime validation and celllaunch observations are reused unchanged where their
actual APIs fit. An API/scope mismatch is amended at its existing owner before
code; neither an arbitrary callback nor a copied implementation is a substitute.
The regular Go builder explicitly has no Docker daemon. There is also an actual
Ubuntu/Docker PostgreSQL qualification pattern in assay-workflow.yml, inspected at
source `1ddfb16e68ebb7be54d4dc5a606a07e74e2a7259`; it uses a pinned checkout,
explicit Go artifact/version/checksum and disposable resources. Propose a separate
pinned containment job using that existing pattern, not a Docker grant to the
regular builder. Workflow publication remains a separate human act.

The proposed fixture image is a source-bound minimal image containing only the
compiled production package plus its explicit test helper, built from a declared
recipe/toolchain and addressed by its resulting immutable image identity. This is
a proposal to avoid a guessed runtime/base image; exact recipe, identity and
platform are still reviewed inputs before execution. Fixture names/volumes and
all engine commands must remain confined to their exact owned resource set; no
ambient engine discovery, broad listing/prune or operational material is allowed.

Normal unit-only CI cannot certify containment. The separate job must opt into an
explicit disposable fixture profile and assert each required named PASS, refusing
missing profile, skipped custody, setup failure or an unavailable engine. Any
ordinary run without the profile remains explicitly unqualified; its green unit
results do not discharge this brief's actual containment Verify rows. No engine
socket/privilege, fixture image, threshold or workflow grant is inferred.

## Independent qualification and handoff

Real ephemeral producer and consumer/sibling processes must demonstrate ordinary
in-scope verification and failure to read signing material, mutate current state,
select another role/source, mint a stronger grant or revive an old producer.
Use the actual selected namespace/mount/channel mechanism and existing launcher
composition, with ephemeral fixture keys/resources and no operational material.

Exercise every one-field claim mismatch; reusable valid reads; revocation and
policy/config/access/incarnation changes during I/O; cancellation and expiry at
release; controller failure/restart; old channel and restored-state replay; and
uncertain process/resource ownership. Plant compiling faults in each guard and
retain baseline, failure, exact restoration and immutable source receipts. An
unavailable containment environment fails qualification rather than passing a
mode/shape check.

57 owns the concrete producer/profile source and its tests. 56 consumes only its
qualified actual public API/profile and keeps request parsing, admitted Store/
refresh composition and final row-release checks. 55 owns operator source/budget
profiles; full40 retains executor/write custody. No acceptance of those parents,
production activation, or complete tool cutover follows from this design.
