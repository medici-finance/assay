# Library-first component contracts

Status: implementation direction, 2026-10-25. The driver's ruling on #2395, recorded in
[`DR-forge-library`](streams/decisions/DR-forge-library.md), supersedes the 2026-10-08
offline-and-frozen-only ruling (`DR-forge-neutral-36`, kept as history): one shared forge
library for reads and writes, built by desktools-v2. `deskread` and the write verbs become thin
command-line wrappers around it, and statusgen links it for its forge calls instead of running a
desk-tools command. The library owns the credential: one client per role and repository, the
token in a private field no caller can read, never an ambient fallback. gh/glab launches are held
to at most 7 across desk-tools and statusgen by a down-only ratchet, and git uses go-git, with a
git launch only where go-git cannot do the operation. The briefs are desktools-v2/75 (library and
wrappers), /76 (go-git and the git-launch ratchet) and /77 (statusgen off the `deskread`
bridge), each under its own human gate. The first slice, the `forgeread` module, is described
under "The first shared read" below; the other rows describe upcoming work, not a claim that
those SDKs or extractions exist.

Shared behavior has one canonical implementation exposed through versioned libraries or
SDKs. Consumers link it directly where compatible with their authority and deployment.
CLIs are adapters, not mandatory cross-module integration boundaries. A process boundary
remains where it provides independent authority, isolation or deployment separation; its
client SDK carries typed requests, never the server's credentials. Imports alone are not
runtime containment. Taxonomy membership describes components and grants no authority.

## Ownership and allowed access

| Meaning | Canonical implementation / planned interface | Consumers and restrictions |
|---|---|---|
| Inter-cell message contract (cellmsg-v1) | `comms/contract` (module `github.com/medici-finance/assay/comms`, comms-core/01): `Envelope`, `ParseEnvelope`, `ParseEnvelopeWith`, `Vocabulary`, `Message`, `Address`, `Receipt`, `ErrorCode`, `Mint`, `Verify`, `VerifyEnvelope` | `tools/desk/internal/comms` (the Assay policy adapter: verb and class vocabulary, compiled lane ACL, `Admit`), then `commsgw`, `commsloop`, `deskcomms`, `commsqueue`. Follow-ups comms-core/02 (server) and /03 (client SDK) consume it. Structure only: no role names, no ACL, no credential, no network, no process launch. |
| Local performance observations | `perfrec` owns the recorder, typed versioned codecs, context validation, selectors and reducers. desktools-v2/67 adds the bounded v4 reader; /68 owns subsequent selected-profile cutover. | Desk wrappers and statusgen link the same module. No network, subprocess, credential, upload or authority moves into the library. v1–v3 remain historical contracts; no parallel recorder or authenticated statusgen adapter. |
| Forge facts and completeness | Existing forge adapters; narrow `forgeread` API, forge-neutral/36; typed positive issue-state adapter in Go API 1.1 | `deskread`, issueboard and statusgen's offline/frozen reader adapter. No generic query, raw token getter or write API on the consumer interface. Statusgen's online reads stay on `deskread` until desktools-v2/77 moves them onto the shared forge library. |
| Forge operations (reads and writes) and role credential | Planned shared forge library (`DR-forge-library`), desktools-v2/75: proposed root module `forgeclient/`, composing `forgeread`'s adapters and desktools-v2/58's credential provider | `deskread` and every desk write verb as thin wrappers; statusgen through its forge adapter layer only (desktools-v2/77). Enumerated typed operations only; one client per role and repository; token in a private field; no ambient fallback; no raw URL, request or query. `statusgen/evaluation` links none of it. |
| Local write-isolation policy | Proposed `deskcore/writepolicy`, [write-policy contract](write-policy.md), desktools-v2/66 | `writeguard` is a thin hook adapter; managed adapters supply validated allocation observations. Core has no effects or authority; independent guards keep their owners. |
| Work-reference interpretation | `statusgen/workref` (`Parse`, `Resolve`, copied supplied registry), milestone-support/01 | Both actual QID and trailer adapters call the package in-process with explicit, distinct legacy profiles. Typed brief adapter remains opt-in; no operational prefixed writes, capability advertisement, credential or authority moves. |
| Eligibility and evidence coverage | `statusgen/evaluation` (canonical owner since statusgen/15): `EvaluateEligibility`, `EvaluateCoverage` | statusgen commands, workflow bindings, projection producers. Explicit observations in; deterministic verdicts out. No admission, identity, capacity or claim authority. |
| Stream-view wire schema | Existing `statusgen/streamview` | Producers and authorized readers pin the contract; no policy or serving behavior in this package. |
| Workflow progression | `workflow` store/controller, graph-execution/19 and /21 | Calls evaluation and loop-admin APIs; process exit is not work acceptance. |
| Process supervision | `loopadmin`, graph-execution/26 | Standing desks and workflow stages; no dependency on workflow store, no effect credentials. |
| Operator lifecycle | cellctl's operator libraries and typed service clients | Operator provisioning stays outside working sessions; library reuse does not merge authorities. |

The existing semantic-owner index in `docs/contracts.md` remains the ownership register.
Implementations update its rows and existing architecture checks; this table is a migration
map, not a competing register. Module placement is not a new semantic owner.

## The message contract: `comms`

`comms/` is its own Go module (`github.com/medici-finance/assay/comms`, stdlib only, no `go.sum`),
the second extracted module after `forgeread`. Its one package, `comms/contract`, is the public
cellmsg-v1 contract; `tools/desk/internal/comms` is the Assay policy adapter over it.

| Layer | Owns | Does not own |
|---|---|---|
| `comms/contract` | The envelope shape and size caps, the parse-or-refuse order (`ParseEnvelope`; `ParseEnvelopeWith` takes a host `Vocabulary`), typed `Message`, `Address`, `Receipt`, `ErrorCode` and the legacy wire mapping, and the envelope identity primitives (`Mint`, `Verify`, `VerifyIdentityOnly`, `VerifyEnvelope`, `Ed25519Signer`, `Ed25519TrustStore`, `MemReplayGuard`). | Any verb, class or role name, the lane ACL, any signing key or key path, and any network, process or credential code. |
| `tools/desk/internal/comms` | The Assay verb and class vocabulary, the compiled lane ACL and its source `laneacl.yaml`, human-gate (reserved verb) refusal, and `Admit` / `ACL.AllowEnvelope`, which apply the ACL to a parsed message. Thin aliases keep the existing consumers compiling. | The envelope structure, caps and assertion mechanism: those are the contract's, and there is one implementation of each. |

Acceptance is not widened: `ParseEnvelope` here is `contract.ParseEnvelopeWith` with the Assay
vocabulary, and the legacy fixtures in `comms/contract/testdata/legacy` are checked through both
entrypoints. A structurally valid message the ACL forbids is accepted by the contract and
refused by `Admit`; `TestPolicyAdapterRefusesForbiddenMessage` holds that seam.

The contract declares two capabilities it does **not** implement and that cellmsg-v1 does not
guarantee: a per-delivery token (`delivery-token/v1`) and independent per-client identity
(`client-identity/v1`). The legacy trust store holds one public key per cell, and a role name does
not establish per-client identity. `contract.Capabilities()` lists them and `contract.Implemented`
returns false for both until a later versioned transport ships them.

Controls: `TestContractDependencyBoundary` walks the whole module closure and fails on
`tools/desk`, `deskkit`, forge, credential, `net`, `net/http` and `os/exec` packages and on any
third-party module, with planted-import negative controls. The module is wired into tools/desk as
a versioned consumer through `replace ../../comms`; the requirement is a placeholder
pseudo-version no proxy serves, so the local replace is not release proof: the same open gap as
`forgeread`, and the two modules are to be released at real versions together. The Dockerfiles
copy `comms/` beside `forgeread/`. CI path filters for `comms/**` are workflow-file changes and
belong to a maintainer with the workflows scope.

## Authority and compatibility

- Offline/frozen inputs require no credentials and remain the default. Missing, partial,
  stale and unsupported inputs retain distinct unavailable/held outcomes.
- Caller-facing SDK packages must not transitively import custody, credential minting,
  role executors or write adapters. Authenticated read adapters are effectful packages,
  composed only where the existing identity policy permits that process to hold access.
  *(Amended 2026-10-25 by `DR-forge-library`.)* The one exception is the shared forge library
  itself (desktools-v2/75): it composes the role credential provider and the desk write
  operations and holds the minted token in its client's private field. It is an effectful
  package, not a caller-facing SDK in this bullet's sense, and only its named consumers link
  it: the desk verbs, and statusgen's forge adapter layer (desktools-v2/77). Every other
  caller-facing package keeps this rule unchanged: the `forgeread` root package and
  `statusgen/evaluation` stay free of custody, credential minting, role executors and write
  adapters (desktools-v2/75 row 8, desktools-v2/77 rows 4 and 7).
- *(Superseded 2026-10-25 by `DR-forge-library`.)* Until then this bullet kept every online
  statusgen forge read on the `deskread` process adapter and kept credentials out of statusgen.
  Now statusgen links the shared forge library for its forge calls, and a credential exists
  inside the statusgen process only in the library client's private field, for one role and one
  repository, with no ambient fallback. Only statusgen's forge adapter layer constructs a client;
  `statusgen/evaluation` and the offline/frozen paths stay credential-free. `deskkit` stays
  internal and statusgen's module manifest still names nothing under `tools/desk`, which is why
  the library and the credential provider live outside that tree. desktools-v2/77 retires the
  bridge under its own human gate. No new daemon, token service or credential profile is
  implied.
- The CI workflow-token profile stays owned by forge-neutral/34. Extracting its constructor
  later must preserve opt-in, repository/host binding, read-kind limits and refusals. The SDK
  does not grant a new CI identity policy or widen token scopes.
- Versioned JSON remains supported for shell/non-Go consumers. The Go API and wire contract
  have distinct compatibility obligations. Preserve command flags, exit codes and envelopes.
- Same input means the same snapshot, evaluator version and policy digest. Library reuse
  alone does not guarantee identical answers from different observations or dependency pins.
- Pin module versions, keep release/build resolution working with `GOWORK=off`, and exercise
  an external consumer from a local module proxy with no workspace or relative replace.
  This is packaging qualification, not permission to cut a release or contact providers.

## The first shared read: `forgeread`

`forgeread/` is its own Go module (`github.com/medici-finance/assay/forgeread`, stdlib only, no
`go.work` at the repository root) holding the first shared fact read, the open issues of an
explicit repository set.

| Piece | What it is | Who links it |
|---|---|---|
| Root package `forgeread` | The `Reader` interface (`OpenIssues(ctx, repos)`), the `Issue` / `Result` / `Unavailable` types, the three-state envelope parser (`ParseIssuesEnvelope`, fail-closed on an unknown schema or kind), `Reconcile` (every requested repository appears exactly once, as data or as unavailable) and the offline and frozen-result readers. It imports no subpackage, no `net/http` and no `os/exec`; its whole dependency closure is checked. | statusgen and desk-tools. Offline and frozen inputs only. |
| `forgeread/adapters` and `adapters/github`, `adapters/gitlab` | The provider-specific pagination and mapping for open issues, written against an `Access` interface that serves one page of an already-admitted repository. They hold no credential, resolve no host and read no environment. | desk-tools only (`deskkit` composes them over its own authenticated transport). A statusgen import of anything below the module root is a counted failure. |
| `forgeread/adapters/github` positive issue states (Go API 1.1) | A narrow `IssueStateAccess` port for positive state/title observations of explicitly numbered issues. The query document is fixed, at most twenty aliases are active, and missing/null/changed identities, unknown states or any GraphQL error refuse the result. No raw query or credential input is added. | issueboard's original wrapped desk reader. Explicit CI read-only and outbound wrappers retain the capability; unknown wrappers and other providers keep serial reads. Statusgen's authenticated boundary does not move. |

The positive-state addition changes neither the root offline/frozen dependency
closure nor `EnvelopeSchema`. Listing absence still cannot retire a placeholder;
every proposed retirement needs the positive state observation. The existing
projection, trust, comment/history and label completeness controls stay with their
owners. Batching reduces HTTP sends for this observation only; GraphQL points
remain unknown, and no production saving follows from a local fixture alone.

How the two paths meet: `deskread issues` and the in-process SDK run the same adapters over the
same observations, so a repository is "known empty", "unavailable", "truncated" (the provider
kept paginating past the walk ceiling; no partial list is returned) or "unsupported schema"
(a refusal, not an answer) identically on both. Credential resolution, host and repository
validation and the CI-token read-only profile stay in `deskkit`; the composition it hands the
adapter refuses any repository other than the one its credential was admitted for, before a
request is built. statusgen's online reads still run the `deskread` verb until desktools-v2/77,
when that composition moves into the shared forge library (desktools-v2/75) and statusgen links
it; its default reader is the SDK's offline reader either way.

Two versions, separately. The Go API version is `forgeread.APIVersion` (semantic versioning of
the exported Go surface; a module release is tagged `forgeread/vX.Y.Z`). The JSON envelope
version is `forgeread.EnvelopeSchema` and moves only by a deliberate, announced change: an older
consumer refuses a newer schema rather than best-effort reading it. An additive wire field (the
optional `class` on an unavailable entry) does not move the schema number.

Controls: `tools/desk/scripts/forge-ban.sh` counts `forgeread/` as its own tree and fails on any
site; the ambient-token-read rule walks it with no permit; the path filters of the
`forge-surface-control` and statusgen workflows are to list `forgeread/**` (workflow files are
promoted by a maintainer; the prepared files are in `ci/staged-workflows/`).

## Routing and sequence

| Work | Owner | Dependency / retirement |
|---|---|---|
| First shared fact-read slice and SDK | forge-neutral/36 (new) | Existing OpenIssues implementations; no dependency on /18, /34 or /35. Preserves their identity work. |
| Remaining statusgen read migrations | forge-neutral/18 | Existing /08, /33 and /34; no dependency on /36. Moves each read onto statusgen's injected reader seam, served by `deskread` today; if /36 has landed, uses its OpenIssues types; preserve /35's separately reviewed ruling-control moves. desktools-v2/77 later swaps the seam's transport. |
| Ruling and sign-off reads | forge-neutral/35 | /18 and its existing controls; reads go through the reader seam (deskread, then the library under desktools-v2/77), no credential of statusgen's own. |
| Shared forge library and thin verb wrappers | desktools-v2/75 | desktools-v2/58's credential provider; carries the five custody records unchanged; owns the combined gh/glab ratchet. |
| go-git and the git-launch ratchet | desktools-v2/76 | Builds on `gitcore`; statusgen's git launches move to go-git with parity goldens. |
| Retire the deskread bridge for statusgen | desktools-v2/77 | /75, /58, /08, forge-neutral/18 and /35; env-token reads, API host literals and the `origin`-based forge classifier in statusgen retire, and the statusgen forge-ban total becomes a failing gate. |
| Scan and CI path coverage of the extracted module | forge-neutral/36 | Lands with the extraction: forge-CLI scan, ambient-token rule and workflow path filter cover `forgeread/`. |
| Further read kinds in the SDK | desktools-v2/75 | Authenticated operations land in the shared forge library, not in the `forgeread` root package. |
| Canonical evaluation extraction | statusgen/15 | Offline extraction from existing code; no dependency on fact-reader migration or workflow store. |
| Durable bindings | graph-execution/19 | Consumes statusgen/15; no second extraction or copied evaluator. |
| Hold no-forge-CLI / no-ambient behavior | desktools-v2/08 | After /18 AND /35; permit the `forgeread` root package and the shared forge library's caller package, never a `forgeread` adapter directly or a desk-tools package; keep the manifest check that statusgen names nothing under `tools/desk`. |
| CLI parsing | desktools-v2/16, /18 and remaining CLI briefs | Continue independently; no change to semantic or authority contracts. |

Wave 0: review this contract and the named consumer amendments; the driver's ruling on the
forge direction is recorded (#2395, `DR-forge-library`, superseding `DR-forge-neutral-36`). Wave 1: /36, under its own
human gate, and statusgen/15 independently. Wave 2: their named consumers, subject to existing dependencies; the forge
read chain (/18, /35, desktools-v2/08) keeps its own sequence and does not wait on /36.
The shared-library chain runs desktools-v2/58 → /75 → /77, with /77 also after /18, /35 and desktools-v2/08 (so /08's class a gate lands before /77 widens it);
/76 is independent of it.
Retirement follows consumer qualification, never merely library availability. Existing
implemented briefs keep their evidence and scope; changes land as follow-ups.

The first blockers are concrete: statusgen's implementation is `package main`, its evaluator
loads files, and the forge implementation is behind `tools/desk/internal`. Extract those
specific seams; `deskkit` stays internal, and nothing waits for a cell taxonomy registry.

## The evaluation library (statusgen/15)

`statusgen/evaluation` is the canonical owner of the eligibility and evidence-coverage
meanings (semantic-index rows `S-eligibility` and `S-evidence-coverage` in
`docs/contracts.md`). It is part of the statusgen module's public API surface; the module is
`github.com/medici-finance/assay/statusgen`.

**API.** `EvaluateEligibility(EligibilityInput)` takes the streams and their briefs, an optional
graph-repos registry and a sibling-checkout lookup, and returns one verdict per brief: eligible,
eligible-with-notice or held, each hold carrying its edge, its state (unsatisfied or
could-not-check) and the reason. `EvaluateCoverage(CoverageInput, Repo)` takes each brief's
Verify rows and recorded witnesses, optional pattern bindings and dependency manifests, and a
read-only `Repo` port for history questions, and returns one verdict per claim (pass, fail,
missing, wrong-revision, error or could-not-check) plus whether the brief is released. The
types are plain immutable values. The library never reads the environment or the clock,
opens a file or network connection, starts a process or holds a credential; its import graph
is held to the standard library's text, collection and hashing packages by a test that also
walks transitive imports. A nil `Repo` means no repository, and every question that needs
one answers cannot-tell rather than passing.

**What a verdict does not grant.** Eligibility grants no admission, identity, capacity or
claim authority; coverage publishes nothing and flips no status. Both only report. Missing,
wrong-revision, stale and unresolved evidence hold, and each of those states stays distinct
from a known no-binding or a known pass, so a consumer can tell "nothing owed" from "could not
tell".

**Migration.** statusgen's own commands are adapters: they observe the tree (streams, the
registry, sibling checkouts, evidence rows, a git-backed `Repo` with replacement objects
disabled), map the observations onto the library's inputs and render the library's verdicts.
The package-main evaluators were removed in the same change, so there is one implementation;
`--eligibility`, `--coverage`, Next-up and the drive frontier read it, and command flags, exit
codes and JSON envelopes are unchanged. graph-execution/19 consumes this package rather than
extracting a second evaluator.

**Provenance.** `evaluation.CurrentIdentity()` reports the evaluator version
(`statusgen-evaluation/2`) and a policy digest over the rule constants the verdicts depend on. Generation 2
adds explicit original acceptance-definition revisions and held provenance observations for protected
Verify tables; witness execution revisions and content applicability retain their existing meaning. The same
input yields the same verdicts under the same identity, and a change to a policy constant changes the digest, and a change to rule
behavior bumps the version, so a recorded verdict can be tied to what produced it.

**Pin and release.** Consumers pin the statusgen module at a version and resolve it with
`GOWORK=off`. The module manifest names nothing under `tools/desk`; `deskkit` stays internal.
The manifest carries one relative `replace`, for the in-repository sibling module `forgeread`
(the fact reader); a consumer ignores a dependency's `replace`, so an external pin must also
resolve `forgeread` at the version the manifest requires, which is currently a placeholder
pseudo-version that no published proxy serves. Until `forgeread` is released at a real version
and the manifest requires that version, an external pin of the statusgen module fails at module
resolution; this is an open gap, and the test below does not close it. A contract test
qualifies the packaging: a throwaway module outside this repository builds
and exercises both entry points against a local file module proxy that serves both modules, with
the workspace off, no replace in the consumer and every network proxy pointed at a dead address,
and a second consumer that expects an API the module does not offer fails to build.
This is packaging qualification only; it cuts no release and contacts no provider.

## Write-policy extraction — 18 October 2026

[The write-policy contract](write-policy.md) adds a narrowly scoped package to the
existing `deskcore` module. It does not export `deskkit` or move role/forge authority.
The command hook remains a supported process integration; compatible Go consumers
link the evaluator directly. Canonical decisions, observed filesystem context and
actual sandbox enforcement have different owners. desktools-v2/66 delivers the
extraction and consumer census.

Target-consumer clarification, 19 October 2026: graph-execution/22 directly consumes
/66 for the local harness boundary and independent sandbox proof. desktools-v2/31's
parser port is withdrawn (medici-finance/assay#3409 merged, ruled at
medici-finance/assay#3420), so /66 has no parser-port edge: writeguard keeps its current
parser, routed exempt, and /66 owns the hook adapter. The guard controls are not retired.

## Shared work-reference core — milestone-support/01

`statusgen/workref` is a stdlib-only package inside the existing statusgen module.
It parses/resolves explicit declaring context, graph-root aliases and supplied stable
identity bindings. Logical identity/key, input spelling, exact version selector and
legacy compatibility fullId remain separate. A resolution establishes neither record
existence nor revision availability; source observations and exact revision qualification
belong to host loaders. Missing requested revisions must remain unknown there.

`contexthandoff.ResolveQualifiedID` and desk's `SplitBriefTrailer` and
`SplitCrossRepoBriefTrailer` now call the core using separate preserved legacy
profiles. Existing strict QID and permissive trailer reduction contracts do not converge
by broadening either API. The explicit `deskkit.ResolveWorkTrailer` adapter accepts typed
brief references and retains revision selectors; existing operational writers do not call
it yet. Human/risk gates, subject checks, online forge reads and credential custody retain
their existing owners. See [the API contract](../statusgen/workref/README.md).

The desk manifest uses the repository's placeholder require/relative replace pattern for
statusgen. Both real consumers are exercised with `GOWORK=off`; an external fixture runs
the public API via a local file proxy with the actual statusgen manifest and no consumer
replace. That fixture qualifies this pure API closure, not a publication of statusgen or
its placeholder sibling requirements. The production workref closure is stdlib only;
existing effectful desk dependencies are not exported by this package. No module, helper
binary, daemon or subprocess integration seam was added.

Consumer routing: the two QID/trailer adapters and domain's isolated mutation snapshot
are fixed here. `statusgen/briefv2.go`/new authoring, record loaders, capability negotiation,
claims, ruling/domain adaptation and history/export remain the respective milestone
follow-ups /02, /03, /04, /08, /09 and /12. No operational new grammar is advertised here.
