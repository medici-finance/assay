# Forge service wire contract v1

`forge-service-v1` carries scoped requests and advisory results between a client and
an admitted service. The canonical types and codec are in
[`forgeread/service`](../forgeread/service/); the separately imported HTTP adapter
is in [`forgeread/client`](../forgeread/client/).
[`schemas/forge-service-v1.json`](../schemas/forge-service-v1.json) describes the
closed canonical JSON shape. It does not replace `ValidateRequest`,
`ValidateResponse(response, request)`, current authorization or domain evaluation.

This contract defines transport DTOs, including effect and receipt DTOs. Handler
availability, source access, credential custody and execution are supplied by their
existing owners. Implementing these types grants no activation or effect authority.

## Envelope and binding

A request has `schema`, a nonempty request `id`, `scope`, `operation`, and the
operation's selector. Repository-specific operations name `repository`. A response
echoes the schema, request ID, operation, repository and entire scope, then supplies
`metadata` and the operation's result. The decoder binds the response to the
original owned request; matching a request ID alone is insufficient.

Scope contains the explicit repository set, `admissionId`, `accessEpoch`, stable
`callerId`, and represented-actor text (empty when not represented). Repository identity is the typed
`Provider`/`Host`/`Path` triple used by the mirror. The set comparison neither
resolves aliases nor discovers repositories. Provider/path validation reuses
`mirror.RepositoryID.Validate`; case-preserving identities are carried unchanged, with
additional wire byte/control/host bounds. The service independently establishes
current caller authority, admission and epochs; caller-supplied scope is a binding
claim, not a grant. The ingestion identity does not establish caller visibility.

Requests select one operation payload. `review-observation` and
`issue-projection` instead use the complete admitted scope and an empty repository
identity, with no per-repository selector.

| Operation | Request selector | Result when supported and sufficient |
|---|---|---|
| `mirror-query` | `mirror`: record kind, `maxAgeMs`, optional scoped minimum | One complete `mirror` snapshot or the bounded legacy `issues` envelope |
| `file-at` | `source`: exact revision and relative path | Exact source revision/path and bounded content |
| `diff` | `source`: two exact revisions | Bounded diff content and typed entries |
| `merge-base` | `source`: two exact revisions | A confirmed base revision |
| `create-issue` | `effect`: canonical operation ID, title and body | Provider outcome and separate projection state |
| `create-change-comment` | `effect`: canonical operation ID, parent number and body | Provider outcome with the comment's own identity and parent |
| `lookup-operation` | `lookup`: canonical operation ID | Existing outcome or explicit unknown; no resubmission |
| `read-observation` | `minimum`: exact scoped observation token | Observation metadata, without an additional payload |
| `review-observation` | Scope-wide; no selector | Existing review observation transported without reclassification |
| `issue-projection` | Scope-wide; no selector | Existing resolved issue rows and quarantined identities |

Unknown operations and schema versions are refused. Revision-based requests use
explicit lowercase 40- or 64-hex object IDs. Relative paths cannot escape their
repository. These fields do not permit an arbitrary URL, clone, executable or SQL
operation. Capability absence remains unavailable.

## Result states and progress

`metadata.state` is one of `older`, `confirmed`, `newer`, `projection-pending`,
`unavailable` or `permission`. Completeness, freshness, provenance and observation
progress remain separate fields.

| State | Meaning |
|---|---|
| `older` | The supplied observation cannot meet the scoped minimum; no result payload; cached age/freshness is separately described |
| `confirmed` | The permitted observation satisfies the request; a supplied minimum is confirmed by its exact token |
| `newer` | The observation owner supplies a distinct scoped token relative to the request's minimum |
| `projection-pending` | Provider outcome and local projection availability have not become one complete current observation |
| `unavailable` | Explicit transport, rate-limit, truncation or unknown failure, without result data |
| `permission` | Current access is unavailable, without result data |

A token binds scope, repository, record kind and an opaque value. Equality is
supported; lexicographic token ordering, a local SQL sequence or provider-global
ordering is not. `older`/`newer` require the request's scoped minimum and a distinct
response token; runtime request-bound validation enforces those relationships.
Observation progress is never effect, claim or permission authority. Tokens must name a
canonical repository in the scope; scope-wide review/projection replies have no
selected-repository progress token.

The outer reason `permission` is allowed only with state `permission`; it cannot
be relabeled as an older observation while retaining provenance or progress.
Permission results omit bodies, counts, provenance, executor identity,
diagnostic links, observation tokens and all retry/cache advice. They cannot disclose
whether a private fill is running. Plain unavailable results likewise carry no
result data or source provenance, unknown freshness and `complete: null`. A currently
authorized original cached observation can instead qualify a mirror-query unavailable
reply as described below; it still supplies no result payload or complete answer.

`metadata.retryAt` is an explicit nullable timestamp. A non-null value is service-owned
retry advice for an unfulfilled read or receipt lookup; it never permits a create
effect, provider request, replay or client fallback. It copies the shared owner's
nonzero retry floor, rather than inventing a client timer. Null means no qualified
advice, not immediate eligibility. Every later service request rechecks current
admission and budgets. The SDK makes no automatic retry, sleeps or source calls.

`metadata.cached` is an explicit nullable `CachedObservation` with original `coverage`,
trusted `evaluatedAt` and nullable `ageMs`. It is limited to an authorized owned
snapshot for this exact mirror query in `older` or `unavailable` state. It contains
no rows, counts or authority and never appears on permission or fulfilled replies.
A synthesized hold is not an original cached observation. Cached permission-hold
coverage is refused even when the outer reply says unavailable; a state label must
not reveal previously held metadata. Current admission and
original snapshot qualification are the serving owner's responsibility.

Age is elapsed whole milliseconds since original `coverage.LastComplete` at the
trusted evaluation instant; it is null when that complete time is unknown or the
elapsed representation is unavailable. Negative or future timestamps are refused.
Evaluation must not advance any coverage time, overwrite original provenance or
make an incomplete population complete. When supplied, `metadata.source` retains
original source/version/revision and its observation time. Canonical mirror
validation owns coverage and freshness; transport does not implement another planner.

A qualified cached unavailable reply has `complete: false`, original fresh/stale
status and original provenance when known. Token-progress `older` still requires a
scoped minimum and a distinct response token. Its cached observation may independently
be fresh or stale by age; freshness never satisfies a newer minimum token. A qualified cached older reply may also carry `source: null` when original
provenance is unknown; it must not invent a source or observation time. Without
cache metadata, the original older stale profile and non-null provenance requirement
are retained. Fulfilled
confirmed/newer replies retain their complete result and carry no retry/cache hold
metadata. Unknown completeness is never converted to false or complete-empty merely
because a service or provider is down.

Effect outcomes are `succeeded`, `failed` or `unknown`; projection state is
`current`, `pending` or `unavailable`. A successful provider outcome can coexist
with a pending local projection. Canonical operation keys and payload digests are
supplied by the existing effect owner. The wire neither manufactures a new key
namespace nor authorizes replay. A typed payload conflict is `ErrConflict`, never
a successful receipt. Returned issue/change-comment identities retain the provider
ID and, for a comment, its distinct parent number.

## Presence, coverage and existing projections

The codec preserves `null`, explicit zero/false, empty strings and empty
collections where the producer's type distinguishes them. `BodyKnown=false`
keeps a body unavailable; `BodyKnown=true` with an empty body is known-empty.
Examples include an
unknown measurement count versus known zero, unknown completeness versus known
false, and unknown mirror collections versus observed-empty collections. File
content uses JSON base64: `""` is an observed empty file; `null` is not.

A successful mirror inventory has a non-null record collection; `[]` can be a
qualified complete-empty inventory. `InInventory=false` means qualified absence
from that inventory, not a fabricated closed state. Inventory completeness alone
does not qualify omitted bodies, review facts, checks, timestamps or consumer
classifications. A projection needing unrepresented facts remains unavailable.
Current freshness and source coverage are evaluated by the mirror/domain owner,
not inferred from a transport state label.

The `review` payload references
[`review-observation-v1.json`](../schemas/review-observation-v1.json) unchanged.
Identity, head/base, applicability, inputs, rows, ranks, lanes, holds and depths
are carried as producer values. Explicit zero/false/empty values survive. A
complete review or issue-projection result requires its supplied scalar pointers
and collections to be present; missing/null facts are not default conclusions.
`reviewflow.PlanObservation` remains responsible for recomputing domain conclusions
and validating consequential use. Transport code does not copy its classifier.

Existing unwrapped issues and review readers retain their own root schemas and
acceptance rules. A service envelope does not rename or deprecate those roots.
When an existing issues envelope is carried in a successful service response, the
bounded transport profile requires schema integer `1`, kind `issues`, one exact
repository, non-null issue population and explicit empty `partial`. The outer
service schema remains the string `forge-service-v1`.

## Encoding and bounds

Canonical objects require every exported non-`omitempty` member, including an
explicit `null` for nullable fields and an explicit zero/empty value where permitted.
Only producer-declared `omitempty` members, such as legacy issue fields, may be
omitted. `null` is refused for non-nullable scalars, value objects and timestamps.

The production codec refuses unknown member names, case aliases, duplicate members,
unknown schema/operation values, malformed/trailing JSON, excessive nesting and
oversized messages. Nested field spelling follows its producer: service fields
use their JSON tags, while imported mirror structs retain their exact exported
field names. Do not case-fold or silently discard a field.

The protocol ceilings are 1 MiB of encoded UTF-8 per message, 256 KiB for bounded
content, 256 collection entries, 4,096 bytes for fields using the general-text cap, and 32 levels of
JSON nesting. Named identifiers have their stricter limits in the codec. A mirror
request's declared age is 1–86,400,000 milliseconds. Trusted composition can impose
lower bounds and budgets; none of these ceilings grants retention, provider calls
or execution. JSON Schema character limits and annotations do not replace the
codec's encoded-byte limits, duplicate checks or request-bound semantic checks.

Changing a closed member set requires a deliberate schema/codec compatibility
change and the relevant producer fixtures. Neither a matching schema name nor
successful JSON parsing proves a result satisfies a current query.

## HTTP client and measurements

The client receives an explicit endpoint, finite timeout, exact expected scope and
an admitted `http.RoundTripper` from trusted composition. It owns its request and
scope data. It supplies no default transport, endpoint discovery, credential
lookup, provider fallback or retry loop. HTTP is admitted only for a literal
loopback IP; other endpoints require HTTPS. Userinfo, URL query/fragment and
ambiguous escaped endpoint paths are refused. One invocation calls the supplied
transport once, follows no redirect and closes a response body before returning.

Only HTTP 200 with the admitted JSON media type is decoded as a response. HTTP
401/403/404 map to permission, 429 to rate-limited, and 409 to conflict. Other
non-success, malformed, oversized or mismatched responses remain held. Cancellation
and the caller's earlier deadline still apply after the final I/O. These failure
paths do not create an empty or current result.

Client logical calls, client transport attempts and server handler executions are
separate measurements. The client's report describes its own invocation; the wire
`measurements` contains only the named server-handler boundary/unit. A nullable
count stays unknown, and known zero remains zero. Recipients must not copy a
server count into client attempts or sum process-local counts as provider work.

Diagnostic parent/link identifiers require explicit consent at both transport
ends. The client sends no automatic diagnostic headers. A permitted response can
only echo the admitted request link; a link is not a credential, role or raw
work/session locator. Permission/unavailable responses carry no diagnostic links.

Authenticated serving, trusted profile/accounting provisioning, retention purge,
provider qualification and tool cutover retain their independent implementation
and acceptance gates. A contract/codec test is not evidence that those services
are deployed or that missing producer facts have become complete.

## Read-through fixture scope

The client flow fixtures exercise cold server-side fill, shared failure cooldown,
original cached coverage/age and retry advice, then recovery without client source
or credential discovery. Those fixture HTTP handlers qualify the wire/client
boundary only. They are not the authenticated production ingress, persistent SQL
integration or an operator-selected policy profile; those remain with their own
owners and qualification. No deployed service or tool cutover follows from this
schema/codec amendment.
