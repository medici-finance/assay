# Attributed local performance — perf-v3 and extensible perf-v4

**Status:** draft — v3 contract requested 19 October 2026; extensible v4 successor requested 21 October 2026.

**Future writer target:** [perf-v4](#perf-v4-successor--requested-21-october-2026). Earlier sections preserve the v3 contract and source account; the dated successor defines the reader-first migration, not an assertion of deployment.
**Routes-to:** desktools-v2/63–65, amended desktools-v2/62, statusgen/26 and
performance-collection/01–09. Authoring does not activate collection or upload.

## Outcome and observed baseline

An operator must be able to answer which brief or issue a tool served, which desk and
assigned agent role performed it, which session/agent/invocation emitted it, and how many
forge requests, GraphQL points and milliseconds that invocation consumed. A session may
work on several subjects; unattributed/shared overhead remains visible.

Source inspection: main `2e02111ed`, 19 October 2026. PR #3431 at `2013b8dc6`
implements perf-v2 in `perfrec/{perfrec,schema,session}.go` and deskkit's `PerfRun`.
It measures one invocation boundary, own CPU and bounded counters/phases. It has no work,
session, agent or invocation identity, no request timing and no GraphQL point field.
Its `Session` is an in-memory timer, not an agent identity. `install` identifies the local
state installation. `statusgen` and boot/launch steps still emit perf-v1. `perfdata/`
is absent. These observations are source support, not release or deployment claims.

The current code explicitly forbids identifiers in old records, and only some request
seams are observed. These are compatibility and coverage constraints, not reasons to
pretend zero means complete. No new churn/complexity or deployed-overhead baseline was
collected for this authoring change. Extend the existing owners with preservation and
matched-overhead tests; no instrument rewrite or new profiler process is justified.

## One local instrument, selected disclosure

`perfrec` remains the single local per-invocation schema, measurement and file writer.
Introduce **perf-v3**; never append fields to perf-v1/perf-v2 or reinterpret their values.
Retain their fixtures, readers, opt-in and seven-UTC-day history. An existing
`ASSAY_TELEMETRY=1` with no new profile retains the existing producer schema/behavior.

New writing needs `ASSAY_TELEMETRY=1` and explicit `ASSAY_PERF_PROFILE=tool|workflow`
(or the equivalent parsed `--perf-profile`). `tool` selects perf-v3 measurements without
work/session/agent linkage; `workflow` additionally permits the bounded attribution below.
The environment switch is still required when a flag selects a profile. Inherited identity
alone never selects a profile. Empty/off profile suppresses v3 and falls back to the existing
producer behavior; invalid explicit profile/context is reported as invalid coverage without
copying the value or changing the underlying operation's result. A decoder rejects invalid
stored records. No profile enables network transmission or grants operational authority.

Workflow records are **identifiable local operational diagnostics**, not anonymous telemetry:
typed qualified work references are stored locally so an operator can identify the brief/issue.
Session and agent IDs are random, not user names, machine names, PIDs or transcript paths.
Use existing restrictive file permissions and retention. Raw argv, paths, titles, bodies,
queries, variables, URLs, tokens, response bodies and free-form errors remain excluded.
Errors/logs/help must not echo attribution values; documented synthetic examples are allowed.

The future `perfdata` importer applies its existing explicit workflow profile and source/epoch
alias policy before putting these local references into its spool, preview or upload. Raw
work references and native harness IDs never pass that export boundary. Its event identity,
spool, source epochs and delivery policy remain its own contract; do not add a second spool,
alias mapper or sender to perfrec. Importing local v3 is not consent to send it.

## Attribution contract and lifecycle

Only the workflow profile carries the linked context below; a fresh random `invocationId`
is also present in the tool profile as an unlinked record identity. Freeze exact JSON names, bounds and examples
in /63 before integrating producers. All optional fields carry explicit missing/invalid/
unsupported coverage; never synthesize identity by searching Git, branch names or argv.

| Field | Meaning and owner |
|---|---|
| `sessionId` | Random logical harness-session ID assigned at session launch. Preserve only on an explicit resume of that session; a fresh session gets a fresh ID. A tool process inherits its emitting agent's session. |
| `agentId` | Random ID for the logical agent, assigned by its launcher. Stable across explicit resume; each newly spawned/replacement subagent gets a fresh ID. Independent of role, work item and session timestamp. |
| `parentAgentId` | Immediate spawning agent when known; absent for a root or externally launched agent. Never substitute the standing desk ID for the child ID. |
| `invocationId` | Fresh random ID generated by the tool for this invocation, or by the server for a handler invocation with a distinct observation kind, including early terminal/refused paths where observable. Never inherited/reused by a child. Scope uniqueness with the existing installation/source identity; no timestamp-only IDs. |
| `parentInvocationId` | Immediate spawning invocation, supplied by that caller when known. Missing across an uninstrumented launcher stays missing; do not guess from timing. |
| `deskRole` | Validated originating desk role, resolved from the existing `DESK_LOOP` roster. It can remain worker-desk while the assigned agent performs review. |
| `role` | Actual assigned execution function: `coordinate`, `implement`, `review`, `verify`, `triage`, `investigate`, `other`, or explicit unavailable. The launcher supplies it; it is not inferred from deskRole or command name. |
| `work` | Bounded typed references with one optional primary subject and related references. Brief: repository scope plus canonical qualified brief ID and stable brief UUID when available. Issue: forge-instance scope plus repository identity and issue number. Support a brief and its issue together; distinguish identically numbered issues in different repositories/instances. Local typed locators are allowed only here. |

Work changes **per launch/invocation**. A persistent agent keeps its own ID when switching
subjects; the next invocation receives the new work context and no stale previous subject.
Batch/sweep/session overhead may carry no primary subject or an explicit shared set; never
charge an entire session to its last brief or split time evenly without a declared method.
Resume/retry does not reuse invocation IDs; attempts may link to existing operational attempt
references through the collection adapter, without making perf records authoritative work state.

## Environment, CLI and shared library seam

The launcher/desk sets attribution defaults and passes through only an operator-selected
telemetry switch/profile; it never creates opt-in or defaults to workflow capture. Tools accept
explicit attribution through their real parser.
Use one credential-free typed resolver in perfrec, reusable from separate Go modules;
deskkit stays internal. The shared CLI adapter binds values; it does not own their meaning.

| Parsed flag | Explicit environment source |
|---|---|
| `--perf-profile` | `ASSAY_PERF_PROFILE` |
| `--perf-session-id` | `ASSAY_PERF_SESSION_ID` |
| `--perf-agent-id` | `ASSAY_PERF_AGENT_ID` |
| `--perf-parent-agent-id` | `ASSAY_PERF_PARENT_AGENT_ID` |
| `--perf-parent-invocation-id` | `ASSAY_PERF_PARENT_INVOCATION_ID` |
| `--perf-loop-role` | existing `DESK_LOOP` (attribution only; a flag never changes authorization role) |
| `--perf-role` | `ASSAY_PERF_ROLE` |
| repeatable `--perf-work` | `ASSAY_PERF_WORK`, one bounded JSON array of the same typed references |

Freeze the work flag's bounded JSON-object encoding and positive/negative examples in /63;
never parse arbitrary free-form descriptions. Flag > named env > unavailable; a present empty
value clears the inherited field. A repeated work flag supplies the complete invocation list,
replacing the env list, rather than appending a stale session subject. No config-file search,
automatic environment capture or new credential binding. Attribution is descriptive and
untrusted; it never changes custody, claims, dispatch admission, budgets or work completion.

Before spawning, an instrumented caller exports **its own** invocation ID as the child's
`ASSAY_PERF_PARENT_INVOCATION_ID`. Ordinary tool children inherit the agent/session/work
context; a newly launched agent gets fresh agent/session IDs, the spawning agent as parent,
its assigned role, and its selected work. Native harness IDs, when available, stay in a
private launcher mapping (in memory or private launcher state with 0700 directories/0600 files, retained
no more than seven UTC days). Never commit, log or export the mapping. If resume mapping is
lost/expired, mint fresh IDs with an explicit continuity gap; lack of a native ID does not
prevent generating opaque IDs.
Do not inherit a parent's invocation ID as the child's ID. Freeze/finalize context once per
invocation to prevent concurrent tasks overwriting process-global work attribution.

Existing tools acquire this seam through /64's shared adapters and explicit maintained-entrypoint
inventory, with standalone adoption through statusgen/26. New tools, migrated tools and future
runtime/runner adapters must adopt it or explicitly report an uninstrumented boundary. Keep
help/version offline and parser/output/exit compatibility. Invalid attribution disables only
the affected optional measurements/context; it cannot turn a successful operation into failure.

## Forge observations: requests, points and time

Both forges may share aggregates. Retain a bounded protocol/resource class so REST requests,
GraphQL points and independent quota pools are never added as interchangeable units.

- Observe each actual attempt at the existing transport leaf, including retry, failed send,
  cancellation and 304. A cache answer or local refusal before transport makes zero attempts.
  Record attempt count, outcome/status class and monotonic request duration; state the boundary
  as dispatch through response-body EOF/close or terminal error. Include body handling in that
  interval without reading it twice, buffering it, altering close/cancellation or blocking on
  telemetry. Incomplete bodies/timers remain incomplete coverage. Report request count, summed
  request work and mergeable duration distributions; concurrent durations overlap wall time.
- Record actual per-operation GraphQL points only when the backend's existing response path
  observes an attributable cost. Preserve `measured`, `estimated`, `unknown`, `unsupported`
  and failed-observation states with measured sample counts and unknown sample counts. Estimates
  carry method/version and never merge into measured totals. A missing cost is not zero.
- Shared rate-limit remaining/used deltas can include other clients and are not actual request
  cost. Do not infer points by subtracting those snapshots. Preserve resource, unit and reset/
  window provenance where available; never sum gauges as per-request charges.
- No separate monitoring request, dry-run cost query, expanded business selection or new
  credentials. A bounded provider-supported cost metadata field may join the existing
  GraphQL operation when v3 is selected; qualify unchanged business results/error semantics
  and disclose its overhead. Never add a second request or retry solely to obtain cost.
  Without a qualified metadata path, cost remains unknown. Decode bounded typed cost metadata at the owning provider
  adapter; perfrec never receives or parses request/response bodies.
- Separate provider work from authentication, local service calls and subprocess attempts with
  bounded classes. Token-mint HTTP is observable HTTP work, not a GraphQL charge. No monetary
  forge billing estimate is promised by these fields.

Provider implementation anchors, checked against official documentation on 19 October 2026:
[GitHub query cost](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api#returning-the-point-value-of-a-query)
can be returned by `rateLimit { cost }` in the same operation. A returned
[GitLab query complexity](https://docs.gitlab.com/api/graphql/getting_started/)
score is a complexity unit, not GitHub quota points. Preserve these distinct units even
when both forges share one report. Missing/error/mutation paths that do not return such
metadata remain explicit unknown; implementation must qualify the supported backend versions.

Each instrumented process records **its own** requests and CPU. The parent records its own
subprocess starts/wait intervals and causal links; it does not recursively scrape children
or copy descendant request totals into its own request counter. Uninstrumented children are
coverage gaps. Reports deduplicate by source/invocation identity, preserve installation/epoch
boundaries, and may aggregate distinct observed own-request counts/points. They never add parent
inclusive wall, child wall, phases and request durations into elapsed time, or treat request
duration sum as an additive component of process CPU. Report terminal-only/killed-process gaps.

## Query/cache service and passthrough boundaries

Measure the planned forge query/cache/mirror service **as well as** direct and passthrough
GitHub/GitLab work. The service does not have to exist before local v3 ships. Source check on
19 October 2026 at `2e02111ed`: forge-neutral/37–45 are todo delivery contracts; the planned
`forgeread/{mirror,service,client}`, `forgeservice`, `forgeexecutor` and `forgeserve` packages
are absent. Existing direct forge adapters/read-cost meters are reuse input, not proof of a
running cache/query layer. [The service design](proposals/non-ci-architecture/22-forge-service-and-source-queries.md)
and its admitted transport/store/custody boundaries remain the owners of that implementation.

Freeze a bounded observation-kind/boundary vocabulary in /63 so three views remain distinct:

| Boundary | Required observations | Accounting meaning |
|---|---|---|
| Caller → query service | Fixed typed operation, logical calls and transport attempts/retries, caller-observed duration, result/coverage and timeout/cancellation. | Client experience; separate from server handler time and provider attempts. Service calls need not be HTTP. |
| Query service handler / mirror / source-query cache | Handler duration, directly observed queue/wait/compute phases, query/result counts where authorized, response source (`hit`, `miss`, `stale`, `bypass`, `unavailable`), and coalescing leader/waiter outcome. | Server work, including locally served answers that contact no forge. Stale/partial/denied answers retain correctness/coverage. |
| Service/executor/refresh worker → forge, or a direct client → forge | Actual provider attempts, outcomes, retry/304 timing, bytes when measured and attributable point/complexity cost in its own unit. Bounded origin: `direct`, `service-passthrough`, `refresh`, `poll`, `repair`, `unknown`. | Only this boundary incurs observed upstream provider work; authentication/local-service traffic stays separately classified. |

Use existing request identity and optional opaque diagnostic request/span and parent links
when available to connect the views; do not derive links from coincident timestamps or expose
raw URLs, SQL/GraphQL text, variables, cache keys or capability material. These links are
workflow-profile correlation, never an authorization claim. A caller-selected work/role label
cannot override authenticated server identity or access policy, and a shared/coalesced answer
cannot reveal other callers or cross-scope cache activity.

A selected performance profile alone never authorizes sending context to a service. Only
an explicitly admitted diagnostic propagation option in the service's existing typed protocol
and configured transport may carry bounded opaque links, with workflow consent at both ends.
No automatic new headers, endpoint discovery, new metric request or raw local work/session/
agent locator forwarding. The ordinary authorized business request retains its own scope and
identity. Without admitted propagation, both ends can record local measurements but linkage
is unavailable; no correlation is fabricated. Aliasing before perfdata spool/export remains
required. This is optional protocol context, not a new telemetry uploader or authority lane.

Long-lived servers record per-handler operations using the same perfrec measurement API
and explicit `caller` boundary, with an observation kind separating handler/request work from
process/invocation summaries. Context is immutable per request, not process-global. Request
latency and provider attempts are attributable; whole-process CPU sampled across concurrent
handlers is not exclusive per-request CPU. Keep it in a separate process summary unless a
qualified request-scoped CPU source exists; missing attribution is unavailable, not duplicated.

A cache-hit request can have **measured zero** upstream attempts only when that handler's
upstream seams were fully observed. An uninstrumented server is unknown even if the caller
sent one successful service request. A paginated passthrough records each actual page and
retry. Background polling/repair and a shared refresh own their upstream work independently:
record one shared refresh identity and bounded causal links when consent permits, never one
copy of its points per waiting client. Leave cost shared/unallocated rather than divide it
equally or charge the last work item without a declared allocation method.

Reports keep logical service queries, service transport attempts, handler observations and
provider attempts as separately named totals. The client and server views of one request
are not two logical queries. Do not add client wait, server elapsed, cache/refresh spans and
upstream request durations as elapsed savings; do not add request-detail measurements to a
process summary covering the same effects. Preserve deduplication identity, observation kind,
coverage, direct-versus-mediated origin and shared/unattributed costs through collection.

Required synthetic journey: 50 admitted same-scope queries coalesce into one three-page source
walk; the actual upstream count is three, not 50 or 150. A subsequent warm hit yields an
observed service call with zero covered upstream attempts. A forced passthrough plus retry
counts actual sends; a denied request makes zero provider attempts; a background poll remains
its own bucket; missing server data stays unknown. Concurrent handlers cannot share work
context or duplicate process CPU. A zero-provider stale answer is not evidence of a correct or
faster current answer. Test no diagnostic propagation when either consent/protocol option is
off, and preserve transport/auth/error semantics and all observation gaps.

/63 owns vocabulary/record/reducer fixtures; /64 owns optional opaque context forwarding at
existing launch/child seams; /65 supplies the reusable own-provider counter/timer (it does not
implement the unshipped service). Forge-neutral/39 owns typed service/client diagnostic
context, /37–38 refresh/poll/event attribution, /40–41 served handler/executor/passthrough,
/42–43 query/cache outcomes and /45 the actual served journey. Those service briefs remain
independently implementable with compatible fixtures until v3 integration; their activation
gates stand. Collection/01–02 and /06–08 retain the same distinctions in import/replay/reports.

## Delivery, compatibility and acceptance

| Owner | Required delivery |
|---|---|
| desktools-v2/61 | Preserve completed perf-v2 implementation and Evidence. No retroactive v3 acceptance. |
| desktools-v2/63 | perf-v3 schema, typed context/resolver, profile/privacy boundary, local writer and mixed-version reader, deterministic fixtures. No DB/API prerequisite. |
| desktools-v2/64 | Desk/agent launch and shared CLI/env propagation, child-ID lifecycle, maintained producer inventory and documented gaps. |
| desktools-v2/65 | Existing provider/HTTP observations for timing and point cost with honest coverage; no requests for measurement. |
| desktools-v2/62; statusgen/26 | Adopt v3 in their unfinished producer work, preserving v1/v2 and own-process boundaries; prove actual records reach the reader. |
| performance-collection/01–02 | Shared meanings, aliased import of v1/v2/v3 and producer identity/coverage; no second local instrument. |
| performance-collection/03–09 | Preserve dimensions, units, nullable coverage, consent, deduplication and parent/child exclusions through storage, transport, reporting and qualification. |

Consumer sweep at `2e02111ed` (active briefs matching perfrec/perf-v2 or the existing
performance-priority routing): desk-tools/38 and desktools-v2/18,21,33,56,58–60 retain
their original scope and reuse these shared seams; /17 qualifies the shared CLI flow.
Completed desk-tools/23–25 and desktools-v2/16,20,61 retain their implementation/acceptance
and Evidence; only /61's successor scheduling metadata changes. Forge-neutral/37–45
retain their service scope and own-process observation/coverage obligations; /45 qualifies
the combined path. Other CLI children inherit cli-contract.md; /64 inventories unsupported
boundaries instead of claiming every standalone binary is migrated. Prior research snapshots,
release notes, fixtures and dated evidence remain historical, not v3 delivery obligations.

Local attribution's critical path is `61 (implemented) → 63 → 64`; request observation is
`63 → 65` in parallel. Qualified desk workload /62 follows 58,59,60,63,64,65;
statusgen/26 follows 63/64; collection/02 follows its own /01 and 63/64/65. A local
record-to-reader journey precedes all optional database/API work. This contract does not
unpark performance-collection, reopen completed briefs, or delay unrelated credential work.

Acceptance must traverse launcher → parsed tool → instrumented child → local records → report,
with two different work items, explicit override/clear, concurrent children, retry/resume,
unknown parent and invalid context. Bypass the parser and prove the writer/decoder still
enforces profile/field restrictions. Synthetic transports prove retry/304/error counts,
body-close timing, measured-zero versus missing points, concurrent unrelated quota activity,
and zero additional provider calls. An uninstrumented child must remain a reported gap.
Mixed v1/v2/v3 fixtures prove old meanings, grouping and no fabricated attribution.

Match enabled/disabled workloads and record payload size, concurrency, p50/p95/max overhead,
bytes/drops and coverage. Retain the existing small-append target only for comparable fixtures;
no new universal budget is invented. Optional writer errors and malformed context preserve
the underlying stdout/stderr, exit, audit and required accounting. Implementation human gates
review concrete local disclosure and propagation evidence; deployment and remote activation
retain their existing separate operator controls.

## Perf-v4 successor — requested 21 October 2026

This dated amendment selects **perf-v4** for future selected-profile production,
replacing the earlier future-v3 writer direction after compatible readers ship.
It preserves the historical v3 specification, its implementation and all evidence.
The [future JSON architecture](proposals/non-ci-architecture/29-extensible-json-contracts.md)
applies the same compatibility pattern to the other owning domains. Delivery is
split into desktools-v2/67 (codec/readers) and /68 (producer cutover); /69 delivers
this design and the architecture census, not the runtime changes.

### Stable core and extension roster

The core marker is `schema: "perf-v4"`. Keep existing JSON spellings and semantics
for `ts`, `tool`, `verb`, `boundary`, `wallMs`, `cpu`, optional `cpuMs`,
`exit`, `result`, `sourceSHA`, `install`. Add mandatory `profile` (`tool` or
`workflow`), `observationKind` (the existing four-kind roster), `invocationId`
(the existing random ID grammar), and `extensions` (an object). All core fields
except `cpuMs` are required, exact-case and non-null; keep existing field-specific
validation. v1 process-lifetime CPU and v2 interval CPU retain separate meanings.
Core `role` is mandatory and follows v3 assigned-execution semantics, not the v1/v2 desk-role meaning:
tool profile requires the literal `none`; workflow profile carries the validated
assigned execution role (`coordinate`, `implement`, `review`, `verify`, `triage`,
`investigate`, `other`), or `none` when no assigned role is present. With supported
attribution version 1, its assigned `role` must equal the core value; an absent
attribution role requires core `none`. `deskRole` remains a distinct attribution
field and is never emitted by a tool-profile writer. An absent or unsupported
attribution extension cannot supply workflow filtering coverage from the core
role alone. No extension may change timing units, process boundaries or identity.

Each extension is `{ "version": 1, "data": { ... } }`. Exact registered names and
version-1 payloads are:

| Name | Payload and reuse |
|---|---|
| `process` | `phases`, `phaseOverlap`, `counters`, `unavailable` from the v2 wire shape with identical bounds, enums and measured/unavailable distinction |
| `attribution` | Existing `Attribution` payload except `invocationId`, which moves to the core; retain session/agent/parent/work fields, field coverage, MaxWork and MaxWorkBytes |
| `requests` | `requests` array of existing `RequestStats`, `requestDrops` and `requestCoverage`; retain group limits, bins, sampled-byte/point counts and coverage validation from v3 |
| `query` | `query`: optional existing `QueryObservation`; absence means unavailable, not a fabricated coverage field. Preserve its providerCoverage and exact query/source/coalescing vocabularies; no raw query, URL or error data |

The /67 machine-readable schema must enumerate every version-1 member from the
pinned v3/v2 reference structs and validation tables; this prose does not authorize
inventing a second observation model or weakening a field's constraints. Share the
canonical observation/reduction types and validation with historical decoders.

The selected writer always emits `process` and `requests` version 1, retaining
their explicit unavailable coverage when a seam cannot measure. It also emits
`query` version 1: a `query` observation is permitted only for service-client and
service-handler kinds. For process-summary and provider-work, an empty query
extension payload means **not applicable**; for a client or handler, the absence
of a query observation means **unavailable**, never an observed zero. The reader
uses the core observation kind to distinguish these cases; there is no invented
query coverage field and no fabricated observation to populate the extension. It emits
`attribution` version 1 only for workflow profile. A tool-profile writer has no
workflow linkage payload. Version-1 attribution must not repeat invocationId or
supply another profile. Every returned record remains per-process; the same
measurement is never dual-written in both v3 and v4.

Readers accept an absent known optional extension as unavailable. Unknown names
and unsupported versions are structurally checked, counted as unsupported and
skipped. This permits a future writer to evolve attribution while an older v4
reader still reports core timings. That reader cannot group/filter by skipped
attribution, charge unknown request points, or call absent process counters zero.
Known supported payloads with invalid fields invalidate the record; do not downgrade
malformed known data to an ignored extension. Invalid core/profile or workflow
linkage in a supported tool-profile payload also invalidates the record.

### Bounds, ownership and privacy

Proposed v4 limits are a JSON line smaller than 1 MiB including newline (matching
the existing reader scanner ceiling), at most eight extensions (four initial groups
plus room for additive groups), names matching `[a-z][a-z0-9-]{0,31}`, positive
integer versions no greater than 2147483647, and maximum JSON nesting depth 32.
The depth ceiling leaves ample room above the bounded context/request structures
while preventing attacker-controlled recursive decode. /67 tests must exercise the
largest legal typed record and every bound's boundary. If the inherited payload
roster cannot fit, revise this design and its tests together before writer cutover;
never truncate a record and report complete measurement.

Reject duplicate object keys at every depth, case-variant core/known-payload keys,
trailing JSON values, null version/data and non-object data. Unknown payloads are
bounded and discarded without echoing key names or data in diagnostics. Readers
may expose aggregate unsupported counts; they must not expose a generic raw extension
map. Perfrec is an observational reducer, not a lossless signed-record relay.

Keep a closed typed producer registry. A caller cannot write arbitrary extension
names, future versions, strings, paths or bytes through a public RawMessage API.
The local opt-in/profile, restrictive file permissions, seven-UTC-day retention,
identifier rules and export alias boundary remain unchanged. Reader tolerance is
not permission to persist new data. Future extensions need owned schemas, coverage
semantics, privacy review and conformance fixtures before producer registration.

### Compatibility and rollout

| Writer / data | New reader behavior |
|---|---|
| v1/v2/v3 historical line | Existing decoder, units, rejection behavior and report meanings remain unchanged |
| v4 with supported version-1 extensions | Strict typed decode into the shared observation model; usable core and supported facets |
| v4 with absent/unsupported optional extension | Usable independent core and supported facets; unavailable facet and aggregate unsupported coverage |
| v4 malformed supported extension, ambiguous keys, unknown core field or major | Reject line, count invalid/skipped with bounded diagnostics; never silently reinterpret |
| v4 consumed by a pre-v4 binary | May be rejected; distribute qualified readers before selecting the writer |

`deskperf` keeps legacy JSON groups and adds a distinct `perfV4` result only when
v4 observations are present. Mixed-history totals must not double-count process,
client, handler, provider or boot-step observations. An attribution filter requires
supported matching attribution; unavailable context cannot match or leak values.
Explicitly test conflict handling and same-install/invocation duplicate identities.

Legacy `--role` keeps its v1/v2 desk-role meaning and excludes all v3/v4 records,
including a v4 record whose core role happens to equal the filter. `--assigned-role`
uses the workflow-aware v3/v4 reducers and requires supported matching attribution
under workflow-profile permission; it never reinterprets a legacy desk role or
matches a v4 core role without usable attribution. A supplied legacy `--role`
therefore excludes v4 even when combined with `--assigned-role`. Mixed-history
fixtures must exercise both filters, both profiles, missing/unsupported attribution
and query not-applicable/unavailable cases without changing old report meanings.

First land the reader. Then qualify desktoken and verifyloop, the two tools observed
with retained local v3 records, through their real compiled producer/report paths.
The source census is broader: 68 desk producer entrypoints plus statusgen share the
recorder. /68 enumerates their current wrappers independently from the maintained
inventory, and changes the single shared selected-profile schema choice to v4.
Do not imply that only two source tools need compatibility or bypass other producer
adapters. Local record history is not an installed-fleet count.

No telemetry opt-in writes nothing. `ASSAY_TELEMETRY=1` without a selected profile
keeps existing default v2 behavior; explicit `tool|workflow` selects v4 after cutover.
Inherited identity alone does not select it. Preserve old-schema fixture construction
for compatibility tests. No production dual-write or silent downgrade is introduced.
Invalid explicit context retains existing redacted coverage handling without changing
the underlying tool result. Publish through ordinary release/checksum pins, with
reader capability established before consumer selection. No special release,
workflow dispatch, local binary install, network upload or performance-collection activation
is part of this delivery. Future collection explicitly adds v4 alongside legacy reads.

### Proof and reconciliation

/67 must test historical fixture stability, unknown optional versions with useful
core reporting, malformed known payload rejection, duplicate/bounds/privacy mutations,
and the actual writer → line reader → reducer → CLI JSON flow. An external Go consumer
must use the public codec without importing a desk internal package. /68 must test
compiled desktoken/verifyloop plus statusgen success/refusal/early-exit paths, child
recorder isolation, tool/workflow opt-in, disabled behavior and full producer census.
Use matched cold/warm fixtures to report p50/p95 and measurement counters; process-local
reuse is not cross-process startup savings and no overhead number is promised here.

Statusgen/26 retains its original Task 3 snapshot-reuse inventory and
TestInvocationSnapshotReuseParity, and Task 4 workloads/benchmarks. Desktools-v2/62
retains its integration and /58–65 prerequisites. Their unfinished format tasks need
explicit dated v4 amendments and reader capability dependencies before dispatch;
this design is not their implementation or verification. Completed /63–65 evidence
stays immutable. /68 additionally waits for /64 and /65 eligibility: a new format
cannot erase their outstanding verification obligations. Existing /60 merged source
and its independent verification requirement are also preserved.
