# Local forge caller authority — offline implementation contract

Status: the concrete mechanism is approved for bounded OFFLINE implementation and
fixture qualification under forge-neutral/57. Operational issuer, credential,
endpoint, image, lifetime, policy values and deployment remain unselected. Actual
producer, caller/custody and containment qualification remains owed.

Design ruling: https://github.com/medici-finance/assay/issues/5428#issuecomment-6631318014

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

### Offline implementation pickup

The original owner continues Tasks2-7 after the exact own-human option1 ruling.
Current owned source `98f956f9ff35a378c6c3b2eab0832e519d15b0e9` contains the
qualified published contract successor `d0224e4645b96df61bd84c10e588a6dd05fe89c3`
and corrected wire source `45095639f7ecf97e91b3a92b5d89597047e0ecf5` through
two-parent merges. The wire source has merged; acceptance and independent
verification are separate. The original Task/Verify/Evidence and the reviewed
HumanDecision bytes are preserved.

Disposition: **continue** the approved offline controller, live registry, closed
AF_UNIX protocols, verifier and launcher/profile composition. **Adapt** only the
existing refresh owner with additive `Limits.Validate() error`, delegating its
existing private policy predicate and ErrPolicy. Auth constructors call that
owner before private-key/listener I/O; no predicate, quota or default is copied.
The source and consumer edge are declared before the additive API implementation.

The actual existing signing primitive is `comms/contract.Ed25519Signer.Sign`;
the forge-specific payload does not reuse Assertion or its default TTL/skew.
Existing `deskkit.AcquireHeld` supplies the named nonblocking gate with stale
recovery disabled. Opened root/file identities are independently checked before
issuance/current reads. `encoding/json` owns encoding; one fixed typed canonical
round-trip decoder rejects unknown, duplicate, missing or noncanonical fields.
No general parser, signer callback or current-authority callback is introduced.

The protected process/channel boundary is retained because query code must not
receive the signer or operator writer. The new closed cellcontainer profile
reuses the existing runtime/mount validation owner and preserves legacy Plan
refusals. The actual primitive receipt qualifies only image/platform/volume/socket
mechanics. Actual producer/signature/current-state/custody tests remain required.
Whole-VM rollback and production restoration remain held for the required
independent anchor and real identity/key rotation. No operational input or numeric
policy default is selected by this continuation.

The actual legacy LaunchSpec is a console/harness contract and a ready
SessionRecord requires a console handle. The noninteractive authority components
therefore use additive `celllaunch.AuthorityLaunch` and `AuthorityPermit` over the
existing immutable ContainerIdentity/ContainerObservation. Closed component kinds
are producer, query, caller and sibling; logical caller roles remain existing
launcher policy. The adapter preserves every legacy predicate and grants no
caller authority by itself. The actual cellcontainer profile independently
observes owned/running resources, then the private operator writer binds that
observation to independently permitted intent. No synthetic console, saved-record
fallback or caller-supplied Allowed flag is used. This is a faithful integration
adaptation within the selected mechanism, checkpointed before adapter code.

### Initial Task1 source assessment (history)

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
proposed. The Task1 assessment held runtime mechanism selection until its design
and fixture prerequisites were concrete. The subsequent human ruling authorizes
bounded offline implementation; operational inputs and restore acceptance remain
held under the unchanged controlling requirements.

Design ruling: https://github.com/medici-finance/assay/issues/5428#issuecomment-6631318014

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

The proposed operator writer is a closed management operation executed only through
the trusted launcher's exact producer-container handle. Its in-container helper
connects to a separate operator-control socket in the producer-only private volume;
no consumer/sibling mounts it and none receives engine access. The operation takes
bounded typed control data, not shell text or a role-selected endpoint. The helper
calls the actual production controller, rather than being a fake fixture authority.
Its embedding/entrypoint is declared with launcher composition; this adds no public
operator verb or remotely reachable management listener.

The operator-only control writer registers a launch only after independent Permit
and live resource ownership checks. It derives stable caller/represented actor,
role and cell from trusted launcher policy, binds the exact observed launch and
source profile, and issues only the predetermined mirror-query grant. Neither
request headers/environment nor a caller-supplied repository/admission selects it.
A failed or uncertain ownership observation registers nothing.

### Actual56 native-source composition remains independent

The credential-free query-verifier role below is the 57 qualification fixture and
R3-key isolation surface. It is NOT a claim that the actual56 service may hold no
provider read credential. An authenticated read-through service can directly
compose already-admitted native access, including its separately qualified real
read token, while never receiving the 57 issuer/operator signing key or writer.
There is no new remote source proxy or full40 executor dependency.

Actual source inspection distinguishes these owners. At the published mapper/wire
pickup, deskkit.NewMirrorReadSource(Forge, ForgeResolution) retains the already
admitted backend (including its explicit read-only fence), and resolves no
credential. ResolveForge/custody belongs to the existing trusted credential owner;
a request cannot supply that role, endpoint or installation. In the later native/
SQL composition inspected at qualified LOCAL source
`a39325183bea2fbedb92d18700cc539343c155ef`, NewMirrorIngestSource additionally
binds admission ID, source ID, trusted clock and the canonical AttemptAuthority.
DB.Refresh takes MirrorStore, RefreshConfig and CallerAuthority and supplies the
shared gate/attempt authority; it initializes nothing and chooses no credential.
55's ProvisionRefresh/role SQL prepare known operator DB profiles only. There is
NO service Factory or Admission issuer in that source. This inspection is not a
claim that the local55 delivery is published or that custody is accepted.

The later56 owner must supply actual trusted native/custody composition using
those existing constructors, derive exact source/admission from server-side
profile selection and retain independent current admission at each source read.
The joined native/SQL test currently uses synthetic tokens and TLS fixtures; it
does not prove real installation selection or caller authentication. Native
credential selection/refresh and production acceptance remain their existing
owners. 57 must not silently mount those credentials into its closed fixture or
claim their absence forces a new broker;56 must not mount the R3 private key into
its data-plane service. Provider access stays inside the shared cache service;
clients keep zero source/credential fallback.

The same distinction applies to network none: it is the pure57 qualification
fixture setting, not a restriction that makes actual56 native/SQL composition
impossible. The served56 fixture/profile must explicitly combine the actual
credential/admission and SQL/native-source connectivity it needs with the R3 key/
control-volume exclusions. It requires a separately supported bounded test/service
egress profile and real combined-isolation qualification; no network setting,
listener or production target is selected here. Do not relax the closed57 fixture
silently or force a new source proxy to satisfy its fixture-only restrictions.

Actual56 served-flow qualification must place client, service verifier and current
channel in a proven same-engine/container arrangement, plus owned SQL/provider
fixture connectivity. Neither the primitive result nor macOS Go tests prove that
host-process AF_UNIX access to Linux named volumes works. That unqualified bridge
cannot become a hidden handler prerequisite or a mocked authentication pass.

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
The launcher permits one canonical producer container identity for a cell on the
selected engine. It reuses actual Engine inspection/validation and full immutable
container identity, rather than trusting a name/label or a file lock in a cloned
volume. A second volume or differently named producer cannot establish another
current authority for that cell. Existing/unknown producer state holds bootstrap;
replacement requires independently confirmed fencing of the exact old producer.
Changing engine or multi-engine failover is outside this first profile unless
old authority is independently invalidated and the new binding is qualified.

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
A current read linearizes when the producer holds that serialized gate and copies
the exact registry tuple and revocation state for the fresh challenge. A control
update linearizes under the same gate; reads after it must see the new state.

For each final authorization, the verifier privately captures stdlib time.Now()
before generating/sending the challenge and retains that value's monotonic
component. It measures elapsed time with time.Since(start), not with arbitrary
refresh.Clock.Now values, serialized timestamps or a UTC-normalized start.
refresh.Clock supplies trusted WALL expiry/provenance checks only; its actual API
is just Now() time.Time and does not promise a monotonic value. No clock callback,
clock policy default or second global clock owner is introduced.

The producer similarly retains a process-local monotonic grant deadline at fresh
registration, so a backward wall-clock jump cannot renew an expired grant for a
new current read. The verifier converts the conservative remaining grant window
at initial validation into a private local monotonic deadline as an additional
cap; it still rejects newly observed wall-clock expiry/uncertainty. Neither local
monotonic value is signed, marshaled, normalized or restored. Restart discards it
and invalidates the registry, rather than reconstructing a lease from disk.
The active-host/clock behavior is part of the actual supported profile; no bound
is claimed across the explicitly unsupported host/VM rollback case.

Tests must independently step a wall-only clock backwards while the local lease
expires, exercise forward expiry and a delayed final release, and prove that no
serialization or response-arrival path restarts the lease. Pure timing tests may
use the supported standard testing facilities; real channel/process qualification
remains separate and cannot be replaced by a fake authority.

Release must occur before the earliest
of request-start plus the explicitly selected current-read lease, the conservatively
verified grant expiry and the caller deadline. The finite current-read lease may
not exceed the selected maximum revocation bound. It is never renewed at response
arrival or by an observer/cache operation. Thus a read linearized just before
revocation can release only within that declared bound; instantaneous distributed
revocation is not claimed. Clock-skew/expiry policy remains explicit and tested,
not copied from comms defaults.

The server obtains fresh current validation after protected data I/O and checks
cancellation/deadline after current-source I/O. After all other blocking operations,
encoding and callbacks, its last local release check enforces that same receipt's
remaining monotonic lease, grant expiry and request deadline before the first
protected byte is handed to transport. A new query or cache hit cannot reuse an
earlier request's receipt. Already returned bytes cannot be recalled.

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
Production restoration remains HELD until a trustworthy anchor outside the
restored volume and actual identity/key rotation are qualified under the existing
incarnation/custody ruling. Neither offline implementation nor this primitive
proof waives that requirement. No snapshot/backup restore becomes current.
No universal restore protection or available external anchor is claimed.

## Supported alternatives and hard prerequisites

The existing restricted local-container profile is the preferred candidate because
its actual mount/process boundary can separate same-host roles without changing
R7 or requiring an operational write executor. Its prerequisites are an explicit
owned local engine, an admitted immutable fixture image/platform, exact channel
mount/profile support and actual independent-process tests. The source-bound
primitive probe described below now qualifies limited local image/platform, volume
views and Unix-channel mechanics. It does not qualify the closed R3 profile,
producer/current writer, signing, caller authentication or custody. The design
mechanism remains unselected pending the spec-stage ruling.

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

## Proposed typed API and protocol inventory

These are concrete proposed responsibilities/types, not delivered APIs. Freeze
exact signatures/encoding after this Task1 assessment, before caller-producer code.
All constructors are typed explicit composition; no arbitrary callback, default
transport, ambient credential, engine discovery or provider operation is accepted.

| Type or entrypoint | Contract |
|---|---|
| `ProducerConfig` | Issuer/cell, fixed primitive/profile version, explicit leases/skew/revocation and frame/registry/I/O bounds. Pure validation precedes key, endpoint or engine contact. |
| `ProducerBinding` | Exact engine identity, producer immutable container identity, live instance and disjoint private/channel/bootstrap volume identities established by trusted launcher observations. No request-decoded binding. |
| `LaunchBinding` | Existing independently permitted LaunchSpec and actual observed SessionRecord identity, stable caller/actor/role and the exact operator-approved grant scope. A passive record alone cannot construct it. |
| `GrantScope` | Canonical mirror.Scope (repository, issue/change kind and admission) plus source/composition, mirror-query only, caller/actor/role/cell and policy/config/access references. Repository/scope validation reuses actual mirror.Scope.Validate; no alias normalization. |
| `GrantClaims` | GrantScope plus issuer/audience, grant ID, incarnation/key reference/generation, issued-at and expiry. No caller-selected algorithm, key, quota values or write operation. |
| `Controller` | Concrete serialized live registry, signer and gate owner in the producer. Construct uninitialized; fresh initialization and every mutation require its private operator path. No query-side constructor can discover an operational key. |
| `ControlCommand` | Closed union: fresh initialize, register exact launch, revoke exact grant, advance current epochs, rotate/fence, or stop. Expected current version binds each mutation; conflicts/unknown outcomes hold, without blindly retrying or adding an R2 claim. |
| `ControlReceipt` | Bounded applied/current version and tuple digest for the private operator. No signing material or token. Read-back resolves uncertain control results before authority resumes. |
| `SignedGrant` | Opaque exact typed payload bytes and signature, delivered only by the prebound launch channel. Reusable for reads only while current and within the original lease. No arbitrary mint payload. |
| `CurrentProbe` | Fresh verifier-generated challenge, exact grant digest and expected producer/channel instance. The query cannot choose a role, source or current-state record. |
| `CurrentProof` | The same challenge/digest/instance, current tuple/revocation, current policy snapshot and bounded capture/deadline data, signed by the pinned current key. It cannot replace the verifier's trust bootstrap. |
| `VerifierConfig` | Explicit read-only bootstrap key/key reference, producer/channel binding and the trusted caller/source policy selected by composition. Unknown/unreadable binding refuses. |
| `CurrentClient` | Concrete bounded AF_UNIX client to that protected current channel; no generic authority callback or local-current fallback. Cancellation and post-I/O deadline/lease checks are mandatory. |
| `VerifiedRead` | Detached trusted identity/policy plus opaque request-bound release lease. No signer/writer handle; a zero/fabricated value refuses. It is not reusable proof for another request. |
| `Verifier.Authorize` | Verify the exact signed grant, request scope and fresh current proof; return VerifiedRead or a sanitized held result. It cannot infer authentication from body Scope. |
| `Verifier.CheckRelease` | Final local monotonic-lease/expiry/deadline check after encoding, callbacks and other I/O. It cannot renew a receipt or fetch a source. |

Proposed signatures (not shipped source) make the constructor/control/current
boundary explicit. `ProducerResources` contains only already-bound producer
private/channel roots and gate resources; `CurrentClient` is concrete, not an
arbitrary caller-authority implementation. The existing refresh.Clock is trusted wall-clock
composition input only, never decoded from a request. Process-local lease elapsed
time uses the separate actual stdlib monotonic mechanism described above.

```go
func CheckConfig(ProducerConfig) error
func NewController(ProducerConfig, ProducerResources, refresh.Clock) (*Controller, error)
func (c *Controller) ApplyControl(context.Context, ControlCommand) (ControlReceipt, error)
func (c *Controller) ServeGrant(context.Context, GrantChannel) error
func (c *Controller) ServeCurrent(context.Context, CurrentChannel) error
func NewCurrentClient(CurrentConfig, refresh.Clock) (*CurrentClient, error)
func NewVerifier(VerifierConfig, *CurrentClient, refresh.Clock) (*Verifier, error)
func (v *Verifier) Authorize(context.Context, SignedGrant, mirror.Scope) (VerifiedRead, error)
func (v *Verifier) CheckRelease(context.Context, VerifiedRead) error
func (v *Verifier) BindRefresh(SignedGrant) (refresh.CallerAuthority, error)
```

No exported constructor returns a private key or signer. Fresh initialization
uses the existing concrete signing primitive inside the producer; it cannot load
an operational key chosen by a query or replace it with a fake-signing callback.
Grant/current channels are established from the closed launcher profile, not
arbitrary per-request listener/path/role arguments.

The eventual refresh adapter implements the ACTUAL existing
`refresh.CallerAuthority.Authorize(context.Context, refresh.Caller, mirror.Scope)`.
It binds a request's signed grant and concrete verifier, compares caller ID,
admission, policy class/epoch, config/access epochs and the exact trusted policy
snapshot (including refresh.Limits), and obtains current authority on each call.
Limits retain the existing R2/refresh owner and are never taken from request text
or interpreted as permission to contact a provider. This uses the existing51 API;
it creates no compile dependency on unimplemented55 or a write executor.

The two data-plane protocols have exactly grant-read or current-challenge messages.
The private operator protocol alone accepts the closed ControlCommand union.
Version, operation and all required fields are exact; unknown/duplicate/noncanonical
or oversized material refuses before allocation/contact. Typed JSON/canonical byte
checks use the selected standard library and one domain-specific codec, not a
parallel generic parser/JWT framework. The entire canonical payload is signed
under distinct grant/current-response domains; fields cannot ride unsigned.
Authentication bytes and raw protocol errors never enter service observations,
cache keys, ordinary diagnostics or performance exports.

### Closed cellcontainer source delta

Add one typed authority-profile builder for producer, query verifier and sibling/
caller fixture roles. It owns exact images/platform, fixed entrypoint/role argv,
UID, noninteractive settings, finite runtime limits, no published network ports
and the explicit volume-view matrix. No caller supplies arbitrary mounts, shell
text, entrypoint, environment or engine endpoint through a query. Reuse common
runtime/mount/ownership validation inside the existing package; preserve the
legacy Plan/Config API and EVERY existing refusal/assertion unchanged.

Actual current Plan.Validate requires its original image entrypoint, interactive
TTY/stdin, role App-key/config/work mounts and exact mount count. Those are real
API mismatches for this credential-free component; do not add an unchecked boolean
or relax legacy validation. The new closed profile must independently check every
common isolation property and its exact own entrypoint/argv/mount set. The bounded
operator-control execution path targets only the frozen producer identity and
fixed helper, with typed size-capped input; it is not arbitrary Docker exec.

## Required explicit operator inputs

| Input | Source and refusal rule |
|---|---|
| Local engine endpoint and platform | Explicit trusted launcher configuration; no ambient endpoint/context discovery. Only exact owned fixture resources may be contacted during qualification. |
| Immutable producer/query/sibling images | Declared recipe, source/toolchain and resulting digest. Fixture selection grants no operational image approval. No floating pull or image default. |
| Issuer and one-cell audience | Operator-owned identity/profile, never learned from a grant. Missing or inconsistent identity refuses. |
| Stable caller, represented actor and role | Protected launcher mapping plus actual independently observed launch/container identity; not request/header/environment claims. |
| Repository, source/composition/admission binding | Exact trusted profile identities supplied to R3; native admission/permissions and later R2 source/budget composition remain independent ceilings. |
| Incarnation, key reference, generation and all epochs | Live operator writer state; no restored SQL/config record may supply currentness. Restart requires fresh trusted initialization and re-registration. |
| Private and per-channel volume identities | Exact closed profile, disjoint resources and expected RO/RW views. No key/control writer or engine socket in any consumer/sibling mount. |
| Grant lease, clock skew, current-read lease and maximum revocation bound | Explicit finite validated values, with current-read lease no greater than the revocation bound and no post-arrival renewal. No comms default or invented production number. |
| Registry, frame/count/byte and I/O limits | Explicit finite bounds. Exhaustion holds without forgetting a live revocation or weakening another caller's grant. |
| Restore and restart procedure | Fresh registration/identity/key after restart or restore; no automatic replay. Whole-host/VM memory rollback is unsupported until an independent anchor and real rotation are proved. |

The engine is a launch/qualification dependency, not a per-query authorization
probe. Request-time currentness uses the actual isolated producer/channel; it
never runs Docker, gh, git or a source fallback. Engine/process observation errors
during launch hold registration and do not turn a saved identity into authority.

## Observed primitive qualification and limits

A source-bound FROMscratch helper at source
`82e67d42798fdc92dcee456e275c254020aecbe8` was built with the actual existing
Go1.27.2 darwin/arm64 toolchain, targeting CGO-disabled linux/arm64. Binary digest:
`sha256:3eaea922669988061fb2c699b68e670566bc09fe1bf923d0522f992bf5efd5a7`.
The exact recipe and helper are under
`tools/desk/internal/forgeservice/auth/testdata/profile-probe/`.

The actual local daemon's explicitly selected BuildKit docker driver built image
`sha256:5c46e042f47cc0edb68224905e981b8cd3af7ac6153469c8c4ccb258764eb106`.
It had linux/arm64, UID/GID501:501, the fixed helper entrypoint and no VOLUME.
The Dockerfile declared no ENV. The engine inserted exactly its standard PATH
baseline; the primitive proof pinned that one observed value and rejected any
other environment entry. This is not a change to legacy validation or an accepted
operational authority environment policy.

Actual separate producer/client/sibling containers proved image-owned directory
copy-up to fresh named volumes at UID501, producer marker write/read and AF_UNIX
listen, client private-marker exclusion plus EROFS on a RO channel-directory write
and a successful socket roundtrip, and sibling exclusion from both private and
channel resources. Runtime inspections checked read-only rootfs, drop-ALL,
no-new-privileges, network none,32pids,64MiB and0.5CPU, no host binds/engine socket.
All three containers, two named volumes and the image were ownership-checked and
removed. Evidence receipt digest:
`sha256:f9b8492e5c5779ed4e084052ee9eb3a42f615d8a853d621f6474614edf325b90`.

Two classic-builder timeout attempts and one strict image-ENV assertion failure
remain retained as unqualified; the successful observation does not erase them.
No signing, grant, live current writer, actual caller authority, production profile,
provider credential, Ubuntu/amd64 or CI acceptance was tested by this helper. The
PG image was not relabeled as an auth image: its actual metadata showed a PG
entrypoint/default data volume and no UID501 channel proof. Actual production
producer/profile and independently enforced CI qualification are still 57 work.

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

## Protocol allocation ceilings

The closed implementation refuses configured registry capacities above1024 and
concurrent channel capacities above64 before resource contact. These are absolute
allocation ceilings, not selected operator quotas or defaults. Actual budgets,
leases, skew and profiles remain required explicit inputs. Frame size is bounded
by the existing celllaunch.MaxRecordBytes ceiling and the explicitly lower
configured frame limit. Registry exhaustion refuses without dropping revocations.

MaxGrants is a lifetime registration budget for the current producer generation:
expired and revoked entries still consume capacity. Only explicit Rotate clears
that history while changing instance and signing key. ApplyControl returns
ErrCapacity on exhaustion, not ErrConfig; it never evicts an entry. Before any
new registration for a launch, an existing non-revoked entry requires explicit
Revoke, even after wall or elapsed expiry. This conservative rule prevents a
clock rollback or ambiguous expiry from implicitly replacing current state.
ErrRegistrationNeedsRevoke names this refusal; ErrConflict is reserved for an
ExpectedVersion mismatch. Both refusals leave version and entries unchanged.
The protected control wire deliberately returns only its existing uniform held
response; these distinctions are for the trusted in-process operator API.

The current-proof signature is qualified independently from the grant signature:
a valid but revoked grant plus a fresh, otherwise matching current reply signed
under another key remains held. Its production control disables only proof
signature verification, leaving grant verification intact.

The first S-forge-local-authority marker registers Verifier.Authorize as the
canonical consumer evaluator. This is a first-owner design-fit registration for
the previously absent producer boundary; no existing ceiling or owner is widened.

## Closed fixture profile implementation

The additive cellcontainer profile has four fixed component kinds and a fixed
helper entrypoint. It shares the legacy isolation/mount evaluator, preserving
every legacy value and refusal. The authority profile requires explicit lower
resource limits, fixed UID501, no TTY/stdin, read-only rootfs, drop-ALL,
no-new-privileges, private process namespaces, network none and no host binds or
engine socket. Image metadata must match the source-bound helper with no VOLUME
or CMD and the exactly observed engine PATH baseline.

A profile manifest may name all nonsecret channel volumes. Names are metadata,
not mount authority: the kind determines an unconditional closed mount matrix.
Producer has private/current/bootstrap and selected grant slots RW; query has
only current/bootstrap RO; a caller has only its predetermined grant volume RO;
sibling has none. Knowing another volume name grants no access. Eight fixed
image-owned grant-slot directories provide qualified UID ownership through named
volume copy-up without root initialization or broad chmod. The operator explicitly
selects the actual subset; there is no channel-count or resource-budget default.

The fixed PID1 standby entrypoint holds no authority. The protected launcher
observes the actual container ID before explicitly starting the production
controller via the fixed helper with bounded typed stdin. No saved configuration
or registry automatically starts a producer. Private control, grant read, current
challenge, query verification and isolation probes have separate closed helper
operations. No arbitrary command/path, role-selected engine or external listener
is added. The actual57 query/caller fixture remains network none; a later56
SQL/native served-flow composition needs its own combined connectivity/isolation
qualification and private listener under its existing owner.

`NewLauncher` is the concrete operator assembly over sealed AuthorityRuntime /
AuthorityHandle inputs. It validates all operator policy before engine/key/listener
contact, derives launch identities from those handles and fresh real observations,
and only then forms ProducerStart. It accepts no caller-provided Allowed flag or
saved observed record. Explicit Run starts the controller; Initialize/Register/
Revoke/Advance/Rotate/Stop use the private control channel. Failed control replies
leave the writer held until read-back confirms the exact version and operation
digest; it never blindly retries. The single private control connection is reserved
separately from the bounded shared data-channel capacity so a slow read cannot
consume the operator writer's slot.

After a verified old producer has stopped, a new explicitly launched controller
must acquire the original exclusive private HeldLock before reclaiming a retained
fixed-channel socket. The protected roots and exact socket inode/type must still
match. It never removes a non-socket, symlink, unrelated path or anything while
the lock/root identity is uncertain. Per-controller channel claims reject duplicate
or concurrent startup before touching an existing listener. No old process is
signalled, no stale-holder recovery is enabled, and no saved bootstrap/registry
becomes current. Re-established channels start with no ready authority; explicit
fresh initialization, rotation and caller registration remain required.

The fixed protected-control mount is `/authority-control`. This public component
name describes its purpose; `Private`/`PrivateVolume` remain semantic API fields
for the producer-only resource. Source-bound image seeds, producer/control paths,
profile mount checks and cleanup use this exact namespace together. Fixture volume
names use the corresponding control suffix. This is an actual namespace change,
not an alternate representation of another identifier; every isolation and leaked
control-volume refusal remains required and must be requalified on the new image.

## Existing Docker adapter and forge exec registration

The existing `cellcontainer.Docker(host)` helper resolves a literal Docker name
through PATH, carries HOME/PATH/TERM, uses a background timeout with combined
output, and exposes interactive foreground execution. It does not supply this
profile's explicit absolute operator executable, empty CLI configuration, original
caller context, finite typed stdin or bounded sanitized stdout. Reusing that helper
as-is would change the approved custody/I/O boundary. The closed
`AuthorityRuntime.command` adapter remains the sole authority-profile exec site;
its inspection and isolation/mount evaluation reuse the existing cellcontainer
owner. This correction adds no constructor or execution path.

The exact existing site is declared in canonical forgeban `UnresolvedArgv`. This
is a could-not-check register, not `AllowedInvocations`: an operator-selected
absolute executable cannot be certified as Docker or forge-free by the static
scanner. The register grants no forge permit, and every unregistered unresolved
site, resolved forge invocation and stale register row retains its existing
refusal. The permit inventory, ceiling and scanner assertions remain unchanged.

## Canonical local endpoint and CI workspace pickup

Raw host-local endpoints belong to `internal/commstransport`. Authority serving
reuses its existing `Listen`: absolute path, real owner-only parent owned by the
current user, and0600 socket creation. The existing authority root/inode and
original exclusive-gate claim remains outside that primitive; the returned Unix
listener has automatic unlink disabled and exact owner-held cleanup remains at
the caller. Source-bound image seed directories become0700 for this existing
transport boundary, with UID501 copy-up; no runtime chmod or privileged initializer
is introduced. Legacy comms listener/dial/stale-recovery predicates remain intact.

The existing `Dial` has no context parameter. Add only
`DialUnixContext(context.Context, string, time.Duration) (net.Conn, error)` at the
canonical transport owner: nonnil uncancelled context and positive finite timeout
are required before filesystem contact; existing absolute-path, owner-only parent,
socket-type and current-user ownership checks precede the actual original-context
Unix dial. Windows explicitly refuses this Unix-only primitive, without changing
its named-pipe API or providing a fallback. Current grant/probe/control clients
reuse this primitive; existing bounded framing, challenge, cancellation/deadline
and final-release rules are unchanged. The endpoint-site reader recognizes the
actual inline `net.Dialer.DialContext` Unix call and permits only its checked
canonical owner site, with planted outside-site and missing-owner controls.

The required hosted qualification job needs a clean linked checkout of its exact
source for compiling production fault controls. It is a disposable GitHub runner
job outside a cell, not an interactive agent workspace or an allocation authority.
The existing workspace caller census routes only `.github/workflows/ci.yml`,
symbol `-`, effect `recipe`, as temporary compatibility under WS-10; the route
retires when that step supplies the required CI fixture checkout adapter, or the
recipe is removed. Existing census assertions still reject every unrouted caller
and stale route. No Git/workspace effect, allocator capability or cleanup right
is added by the census entry.


## Retained-channel liveness qualification

A socket inode may be reused after unlink and successful rebinding. Numeric inode
inequality is not a restart-generation witness. The positive retained-channel
fixture proves an actual bounded canonical current request/reply exchange; the
fresh uninitialized controller must return typed held with no grant/proof/receipt
within the original absolute deadline. A connection alone is insufficient, and
individual attempts cannot renew that deadline. Production fixed-channel,
exclusive-lock/root/inode checks and all negative ownership/path assertions stay
unchanged. This is a correction to the positive test witness, not authority from
socket presence and not a wider timeout or stale-owner recovery mechanism.
