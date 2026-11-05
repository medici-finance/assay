# deskd relocation candidate

This standalone module moves the existing service and its full executable,
test and embedded-web closure into Assay for
[#3669](https://github.com/medici-finance/assay/issues/3669).
The Medici-owned source is released under [Apache-2.0](LICENSE) with
[attribution](NOTICE); dependency licenses remain their respective upstream
licenses. The original source checkout is unchanged.

From this directory, `GOWORK=off go build ./cmd/deskd` builds the service without
a workspace or a reference to the former checkout. `cmd/openapigen` retains the
wire-schema generator, and `cmd/cellsvalidate` retains the offline schema validator
used by the mutation harnesses. No image, release, installation or runtime is activated
by this source placement.

From the repository root, `make deskd-build` runs that same build with `-trimpath`
into `deskd/dist/deskd` (ignored; nothing is written outside `deskd/dist/`), and
`make deskd-image` builds a local container image from `images/deskd/Dockerfile`
with this directory as the only build context. The image is tagged
`assay-deskd:<source SHA>` (with a `-dirty` suffix for uncommitted changes under
`deskd/` or `images/deskd/`), runs as a numeric non-root user, carries no shell,
and must pass the repository's layer-secret scan. Without `docker` or a reachable
daemon the target exits non-zero with a named could-not-check reason. The image is
local-only: the target never pushes, logs in or exports to a registry, and its tag
is a build name, not an artifact pin. It stays local until a separate release act
publishes it. On Windows, `scripts/build-windows.ps1 deskd-build` builds
`deskd\dist\deskd.exe`, and `deskd-image` refuses as unsupported.

## Architecture pickup

Disposition: **adapt for a bounded compatibility relocation**.
The Assay pickup uses main revision `116a2f1`, the current implementation-pickup,
Go engineering, library-first and CLI contracts, and the approved R7 ownership
record in `docs/streams/cell-api/README.md`. The exact source revision, file
hashes and private consumer crosswalk remain in the private handoff receipt.

The existing command, domain logic, adapters, index, authentication, custody,
mounted routes, terminal controls, MCP surface and embedded web view move as one
closure. The executable links container and local terminal drivers; it does
not link the Kubernetes pod driver or Kubernetes SDK. The existing assertion
of that exclusion remains. The comment adapter and purity checker are included
for the existing negative/positive import-closure tests, without linking the
comment path into the service. The generated TypeScript declarations remain as
an input to the existing route contract checks, not a migrated desktop client.

This placement retains legacy SQLite projections, polling and session behavior
as compatibility evidence. It does not adopt them as the R2/R6 target, complete
cell-api/01–07, or lift their dependency, acceptance, custody or activation gates.
The R7 refactor remains their work. The existing `flag` entrypoint is preserved;
this relocation makes no new parser/library choice. The standalone module is a
reversible packaging choice declared through the normal draft-PR decision
mechanism.

## Explicit human principal

Start with `--human-login YOUR_GITHUB_LOGIN` to enable human operations. The
flag carries no token: write credentials still accompany individual requests,
and the service never stores them. There is no environment, HTTP or config-file
setter for this process policy. A missing or empty flag disables human actions;
an invalid login fails startup. The policy freezes once at command startup.

This is an intentional safe-default compatibility change: deployments that
previously relied on a compiled human principal must explicitly select their
existing principal. Selecting that principal preserves the existing custody,
per-cell `answer_as`, repository, role, origin and capability checks. Custody
keeps its case-insensitive, whitespace-trimmed login comparison and returns the
canonical configured identity; the writer verifies that canonical identity.
An `answer_as` entry alone cannot enable an unconfigured process. The current proving-cell allowlist is exactly `assay` and `house`; it is
code-owned, not configurable. Namespace restrictions are unchanged. Consumer pins,
runbooks and runtime cutover require separate acceptance; this PR performs none.

## Loopback sign-in for a private proving cell

The current policy permits per-user sign-in for private cells named exactly
`assay` or `house`. Their configuration must explicitly set
`per_user_identity: enabled` and declare a development environment. Merely
having a development environment or OAuth variables never enables sign-in.
All other private-cell opt-ins, including case or suffix near misses, are
refused. The operator's deployed configuration stays outside this repository.

Keep the default loopback bind, `127.0.0.1:8787`. Configure the existing per-owner
read credential and per-user OAuth App through the operator environment:

```sh
export DESKD_GITHUB_TOKEN_EXAMPLE_ORG=...  # read-scoped; substitute the repo owner
export DESKD_OAUTH_CLIENT_ID=...
export DESKD_OAUTH_SECRET=...
export DESKD_OAUTH_REDIRECT_URL=http://127.0.0.1:8787/auth/callback
GOWORK=off go run ./cmd/deskd --config /path/to/cells-house.yaml \
  --index deskd-index.db --human-login YOUR_GITHUB_LOGIN
```

The callback must match the operator-created OAuth App registration. The default
bind already derives the callback above. Private pages reject anonymous access;
signing in supplies a viewer identity whose repository capability is derived
server-side. OAuth App creation, secrets and the deployed opt-in remain operator
acts. Session state is in memory; restart ends sessions. The machine boundary
and per-user authorization both apply. A non-loopback deployment requires the
separate bearer-authentication boundary and its own qualification.

## Consumer and retirement map

| Consumer | Relocation disposition | Retirement / acceptance owner |
|---|---|---|
| Service entrypoint and its imports | Included with the complete executable closure | R7; qualify a replacement before retiring the legacy service |
| Embedded HTML/JS/CSS and served MCP | Included; existing mounted authorization, SSE, ETag and uncertainty checks retained | R7 compatibility and authorization qualification |
| Go wire client and schema generator | Included with existing contract tests | R7 public contract/client work; external importability remains separate |
| Generated TypeScript declarations | Included as existing test input; desktop host and build remain at their current home | R7 client migration and last-consumer inventory |
| Legacy CLI, desktop client and browser integrations | Retained at their current home; no cutover or deletion | Source consumer owner and cell-api/04, then /07 |
| Image, installer, operator integration and pins | Retained at their current home | cell-api/05–07; no new image or runtime authority here |
| Adopter configuration, identities and trust enrollment | Excluded; an empty trust-anchor example enrolls nobody | Existing configuration/credential owner |
| Adopter-specific config tests | Three real-config guards remain unchanged at the source; excluded from this reusable module | Source owner retains enforcement; migrate their schema consumer before source retirement |
| Active source tasks, claims, open changes and verification obligations | Not closed or assumed empty; a current source snapshot is checked, while full active-obligation inventory remains a retirement gate | Source owner receipt before any retirement |

## Validation and limits

The validation receipt records bounded per-package runs with `GOWORK=off`,
`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` and `KUBECONFIG=/dev/null`.
The existing dependency versions are assessed on
[#3669](https://github.com/medici-finance/assay/issues/3669#issuecomment-6324063778).
No downloads or tool installation are part of that validation.

The initial schema run retained two tests of the source owner's real
`config/cells.yaml` and failed because that private configuration is excluded.
The same source schema suite passes in its original checkout. Those two
adopter-only guards remain unchanged at that source home, pinned by function
hashes in the private handoff, and are excluded from this reusable closure.
The validator command also retains its three generic tests; its separate real-configuration
CLI guard remains unchanged at the source owner. Every generic validation/identity test remains; the final reusable schema suite
passes. No synthetic replacement or new skip masks the initial two failures.
The source owner must migrate those guards onto the qualified public schema
before retiring their current consumer. Existing Docker integration skips and
the absent legacy client-host check remain could-not-check. Linux
cross-compilation is not Linux execution or Kubernetes qualification.

The executed mutation receipt covers the new 21-case policy harness. Retained
legacy mutation scripts are separate checks; their presence does not imply
qualification of every historical mutation. In particular, the attach harness
contains a pod-import mutation whose source driver is intentionally excluded.

Assay's current CI discovers new tracked modules for build and vet, but executes
full tests only for named modules. The private handoff includes the exact
additive workflow hunk needed to test this module. It has not been applied,
published or triggered. Full automatic test coverage is therefore pending.

No current consumer is declared retired and no source repository is modified.
Publication checks cover the complete proposed tracked tree and outward text;
private provenance and the source consumer inventory remain outside this tree.
