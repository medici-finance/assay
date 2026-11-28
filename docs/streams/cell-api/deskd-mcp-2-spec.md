# deskd `mcp/2` release specification

**Status:** draft

November 28, 2026 · Proposed owner: R7 (`cell-api`)

## Outcome and milestone

An operator can pin a qualified deskd image by digest and connect an MCP
`2026-07-28` client to read cell status and the escalation inbox. The workflow
runs on a disposable instance using the existing deskd configuration and identity model.

| Milestone | Prerequisite milestones | Contributions |
| --- | --- | --- |
| `assay:assay:cell-api:m-01@1` — deskd `mcp/2` | None. Use the released deskd image and published MCP specification as baselines. | Open. The acceptance criteria below are the handoff for implementation and release work. |

The milestone is recorded in authored scope while the [milestone contract](../../../spec/milestone-v1.md)
and [reader rollout](../milestone-support/README.md#outcome-and-criterion-coverage) are
being qualified. It adds no start or acceptance edge to an existing brief. The Cell API
migration, legacy consumer retirement, cell deployment and consumer re-pins follow their
own approval paths.

## Baseline and scope

At Assay revision `022a8c8840d22cb195d21502c94077ba4b89829e`, authenticated
`POST /mcp` advertises `2024-11-05` and handles `initialize`, `tools/list` and
`tools/call`. [Tool dispatch](../../../deskd/internal/mcp/mcp.go) and
[per-tool authorization](../../../deskd/internal/mcp/authz.go) are in the standalone
deskd Go module. The [release contract](../../release-images.md) includes deskd in
the umbrella image set. [Assay v1.0.95](https://github.com/medici-finance/assay/releases/tag/v1.0.95)
records a published Linux amd64 deskd image suitable as a compatibility baseline.

- Add a default-off `/mcp/2` endpoint in the existing deskd process. Keep the legacy
  `/mcp` request and result contract available. Explicit endpoint selection lets operators
  choose the protocol without relying on client fallback.
- Implement the [MCP `2026-07-28` Streamable HTTP contract](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)
  for the advertised capabilities. Serve `server/discover`, `tools/list` and `tools/call`.
  Validate request metadata, version and required HTTP headers. Return the specified
  protocol errors, completion and tool-error results, and deterministic lists with
  cache metadata. Keep caller-scoped metadata private and recheck authority on each call.
- Expose the closed read-only set `desk__status`, `desk__inbox_list` and `desk__item_get`
  through list and dispatch. Reuse their handlers, result meanings and unknown or stale
  states. Mount `/mcp/2` under the existing agent route policy, origin checks and caller
  authentication. Enforce cell and repository scope and refuse unlisted tools before
  dispatch. The authenticated caller and server policy establish authority independently
  of protocol headers and client metadata.
- Qualify a loopback instance with the real deskd image and an independent MCP SDK
  client. Record the client version. Hosted ingress, OAuth enrollment, writes, human
  actions, MCP Apps, Tasks, Events, remote skills and multi-round-trip interactions
  are outside this release scope.

## Independent release and rollback

The component release contains one deskd image from the existing Dockerfile and module.
Use an immutable digest and a deskd-scoped release identifier. The publication receipt
names the source commit, platform, image reference and digest, qualification evidence
and previous supported digest. Consumers opt in by selecting that digest and enabling
`/mcp/2` on the chosen instance.

Reuse the release producer's deskd build, provenance, image checks and protected approval.
The component path may build and publish only deskd. Preserve the umbrella producer's
artifact set, tags, release record and validation contract. The component path requires
no umbrella version advance or re-pin of statusgen, desk-tools, the plugin, runner images
or any cell. Keep any necessary Go dependency inside the deskd module and preserve the
deskd data format.

An operator can disable `/mcp/2` or restore the previous digest. A failed or partial
publication has no pinnable release receipt. A retry cannot move a published identifier
to different bytes.

## Acceptance and handoff

An independent verifier records the exact source, image digest, client version and fixture
for each applicable check. Use a disposable local instance with synthetic data and
identity fixtures. Publication evidence binds the same verified image to the existing
protected release approval. The R7 maintainer accepts the outcome through the existing
decision process.

| ID | Evidence required |
| --- | --- |
| MCP2-01 | A real MCP client discovers the released revision and calls all three tools. Compare the results against the same fixture through legacy MCP. Cover unavailable sources, an unsupported version and mismatched HTTP headers. |
| MCP2-02 | Verify mounted route and origin checks. Anonymous, out-of-scope and cross-repository requests reveal no protected results. A direct call to an unlisted tool stops before dispatch. Verify private cache scope for caller-scoped metadata and refusal after authority is revoked. |
| MCP2-03 | A legacy client completes the existing handshake and selected read and write regression cases with synthetic providers. That workflow also works with the new route disabled and after a rollback to the prior digest. The new path requires no state migration. |
| MCP2-04 | A component dry run checks only deskd and publishes nothing. A failed run leaves no release receipt and a retry refuses a conflicting digest. An authorized publish produces one deskd receipt. The umbrella release suite passes and the release introduces no change to existing consumer pins or brief dependencies. |

The endpoint, read allowlist and component-only release identifier are proposed design
choices. The maintainer approves the scoped publication entry point and identifier
before any workflow change. Publication and deployment remain separate operator acts.

At pickup, re-read the [R7 admission record](README.md#authority-and-admission),
[cell operator contract](../../proposals/non-ci-architecture/21-cell-operator-surface.md)
and [Go engineering guidelines](../../go-engineering.md). Recheck the mounted deskd route,
release producer and legacy client inventory at current revisions. Continue if the existing
authority and release boundaries support the design. Route any gap to its current owner
before adopting a new dependency or publication mechanism.
