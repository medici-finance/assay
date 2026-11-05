# Source-bound local authority fixture

This fixture executes the actual controller, Ed25519 signer, live registry,
protected launcher, AF_UNIX current protocol and verifier. Its helper is built
from this immutable repository revision with the explicitly pinned local Go
toolchain, `CGO_ENABLED=0`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOWORK=off`,
`GOOS=linux`, the explicitly selected engine architecture, `-trimpath` and
`-buildvcs=false`. The build receipt records the source revision, toolchain and
binary hashes, recipe hash and resulting content-addressed image ID. No helper
or signing implementation is downloaded.

The declared build uses the already existing selected local daemon's default
BuildKit docker driver, an explicit validated local Unix endpoint and a dedicated
empty Docker CLI configuration. It uses `--pull=false --network=none`, no new
builder container and no registry push. The context contains only this frozen
Dockerfile, the compiled helper and the declared directory seed files. The image
is FROM scratch, UID501:501, fixed `/authority-helper`, with no recipe ENV, CMD or
VOLUME. The observed engine-injected standard PATH is pinned by the closed image
validator; any other environment fails qualification.

Every named volume starts with the image-owned directory copy-up from
`COPY --chown=501:501 seeds/ /`. There is no root initialization container,
privileged helper, host bind or chmod workaround. Actual UID write and read-only
refusal tests must pass on the exact image and engine. A preexisting volume is
usable only after its exact fixture ownership label matches.

Five separate containers use the actual closed authority profile: producer,
query verifier, caller A, caller B and sibling. The producer alone receives the
private control volume, current/bootstrap write views and both grant views.
The query gets current/bootstrap read-only; each caller gets only its own grant
read-only; the sibling gets no authority volume. All use read-only root, all
capabilities dropped, no-new-privileges, network none, private process namespace,
32 PIDs, 64MiB memory, 0.5 CPU and an 8MiB tmpfs. These are disposable fixture
inputs, not operational policy defaults.

The fixture packet supplies only explicit trusted engine/tool/image/resource
bindings. No ambient context, registry credential or production key participates.
Fresh `AuthorityRuntime.Observe` and sealed handles qualify actual container
identity and mounts before protected launcher construction and every helper call.
The protected launcher supplies fresh policy through its fixed producer exec,
then separately initializes and registers independently observed callers. Saved
bootstrap and verifier files alone cannot restore current authority. The query
helper receives no controller, private key, engine socket or control volume.

`TestForgeLocalAuthorityCustody` requires `AUTH57_CUSTODY_INPUT` naming this owned
packet. Missing input is explicitly UNQUALIFIED, never a passing custody result.
The fixture checks actual positive authentication and independent role exclusions,
wrong launch/signature/scope, final lease expiry, private revocation and rotation.
Cleanup rechecks exact ownership before removing every known container and volume.
The build owner separately checks and removes its exact owned image.

This network-none verifier fixture does not qualify the later authenticated
service's combined SQL/native connectivity, listener or provider credential
custody. That consumer must independently compose already-admitted read-only
source access without exposing issuer/operator keys, and qualify its combined
isolation. No remote source proxy or write/execution dependency is introduced.
Whole-VM memory rollback and production snapshot restoration remain unsupported
without the controlling independent non-restored anchor and actual rotation.
Local fixture success cannot accept a production issuer, credential, policy TTL,
restore profile or deployment.
