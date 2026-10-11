# Library-first component contracts

Status: implementation direction, 2026-10-08, ruled for offline and frozen inputs only. The
driver ruled on #2395 for option 1, recorded in
[`DR-forge-neutral-36`](streams/decisions/DR-forge-neutral-36.md): statusgen may link a narrow
module's offline and frozen packages; every online forge read stays on the read verb, `deskkit`
stays internal, and no credential moves into a process that has none today. The forge-seam
direction of 2026-09-14 (statusgen reaches the seam by running the read verb, never by linking
a package; see the desktools-v2 spec header) is amended in that part only. forge-neutral/36
becomes eligible for pick-up and keeps its own human gate; the ruling is not its sign-off. The
first slice, the `forgeread` module, is described under "The first shared read" below; the
other rows describe upcoming work, not a claim that those SDKs or extractions exist.

Shared behavior has one canonical implementation exposed through versioned libraries or
SDKs. Consumers link it directly where compatible with their authority and deployment.
CLIs are adapters, not mandatory cross-module integration boundaries. A process boundary
remains where it provides independent authority, isolation or deployment separation; its
client SDK carries typed requests, never the server's credentials. Imports alone are not
runtime containment. Taxonomy membership describes components and grants no authority.

## Ownership and allowed access

| Meaning | Canonical implementation / planned interface | Consumers and restrictions |
|---|---|---|
| Forge facts and completeness | Existing forge adapters; narrow `forgeread` API, forge-neutral/36 | `deskread`, statusgen's reader adapter. No generic query, raw token getter or write API on the consumer interface. |
| Eligibility and evidence coverage | `statusgen/evaluation` (canonical owner since statusgen/15): `EvaluateEligibility`, `EvaluateCoverage` | statusgen commands, workflow bindings, projection producers. Explicit observations in; deterministic verdicts out. No admission, identity, capacity or claim authority. |
| Stream-view wire schema | Existing `statusgen/streamview` | Producers and authorized readers pin the contract; no policy or serving behavior in this package. |
| Workflow progression | `workflow` store/controller, graph-execution/19 and /21 | Calls evaluation and loop-admin APIs; process exit is not work acceptance. |
| Process supervision | `loopadmin`, graph-execution/26 | Standing desks and workflow stages; no dependency on workflow store, no effect credentials. |
| Operator lifecycle | cellctl's operator libraries and typed service clients | Operator provisioning stays outside working sessions; library reuse does not merge authorities. |

The existing semantic-owner index in `docs/contracts.md` remains the ownership register.
Implementations update its rows and existing architecture checks; this table is a migration
map, not a competing register. Module placement is not a new semantic owner.

## Authority and compatibility

- Offline/frozen inputs require no credentials and remain the default. Missing, partial,
  stale and unsupported inputs retain distinct unavailable/held outcomes.
- Caller-facing SDK packages must not transitively import custody, credential minting,
  role executors or write adapters. Authenticated read adapters are effectful packages,
  composed only where the existing identity policy permits that process to hold access.
- A role-owned credential is not passed into statusgen to avoid a subprocess. **Every online
  forge read statusgen makes stays on the deskread read-verb adapter**, a process boundary,
  under the identity the verb resolves. In-process SDK use inside statusgen is limited to
  offline and frozen inputs, which need no credential. `deskkit` stays internal: statusgen's
  module manifest names nothing under `tools/desk`. No brief in this plan retires the
  deskread bridge for any online read. Retiring it would need its own human-gated brief and a
  driver ruling, and none is authored. No new daemon, token service or credential profile is
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

How the two paths meet: `deskread issues` and the in-process SDK run the same adapters over the
same observations, so a repository is "known empty", "unavailable", "truncated" (the provider
kept paginating past the walk ceiling; no partial list is returned) or "unsupported schema"
(a refusal, not an answer) identically on both. Credential resolution, host and repository
validation and the CI-token read-only profile stay in `deskkit`; the composition it hands the
adapter refuses any repository other than the one its credential was admitted for, before a
request is built. statusgen's online reads still run the `deskread` verb; its default reader is
the SDK's offline reader.

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
| Remaining statusgen read migrations | forge-neutral/18 | Existing /08, /33 and /34; no dependency on /36. Online reads stay on deskread; if /36 has landed, uses its OpenIssues types, and moves no adapter into the SDK; preserve /35's separately reviewed ruling-control moves. |
| Ruling and sign-off reads | forge-neutral/35 | /18 and its existing controls; reads go through deskread, no independent credentials. |
| Scan and CI path coverage of the extracted module | forge-neutral/36 | Lands with the extraction: forge-CLI scan, ambient-token rule and workflow path filter cover `forgeread/`. |
| Further read kinds in the SDK | not yet briefed | Each authenticated adapter moved into the SDK is a /36-class extraction under a human gate. |
| Canonical evaluation extraction | statusgen/15 | Offline extraction from existing code; no dependency on fact-reader migration or workflow store. |
| Durable bindings | graph-execution/19 | Consumes statusgen/15; no second extraction or copied evaluator. |
| Hold no-forge-CLI / no-ambient behavior | desktools-v2/08 | After /18 AND /35; permit the narrow SDK's offline and frozen packages only; keep the manifest check that statusgen names nothing under `tools/desk`. |
| CLI parsing | desktools-v2/16, /18 and remaining CLI briefs | Continue independently; no change to semantic or authority contracts. |

Wave 0: review this contract and the named consumer amendments; the driver's ruling on the
forge-read direction is recorded (#2395, `DR-forge-neutral-36`). Wave 1: /36, under its own
human gate, and statusgen/15 independently. Wave 2: their named consumers, subject to existing dependencies; the forge
read chain (/18, /35, desktools-v2/08) keeps its own sequence and does not wait on /36.
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
(`statusgen-evaluation/1`) and a policy digest over the rule constants the verdicts depend on. The same
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
