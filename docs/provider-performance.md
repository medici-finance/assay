# Own provider observations

Desktools-v2/65 composes the shared `perfrec` request sink with the existing
counting transport. Callers pass an invocation-owned `RequestObserverFor` made
from the actual parsed context, session and bounded origin into `ForgeFor` or
`ResolveForge`. Backends retain their existing credential and outbound checks.
There is no ambient measurement setter, new sender or monitoring request.

The existing idempotent counting leaf observes each actual send, including retry,
304, transport failure and cancellation. An adapter's local refusal or cache
replay above that leaf observes no send. The body wrapper forwards the original
bytes, read errors and Close calls without reading ahead or buffering. Timing
ends at body EOF, Close or terminal error. Bytes are those consumed before that
boundary; an early Close does not establish complete payload consumption.
Unfinished or killed requests may have no terminal observation. A missing sample
is a coverage gap. Request time is work and cannot be added to process wall time.

The shared recorder owns the 64-group bound, 32-bin mergeable duration histogram,
drop counts, profile validation, permissions, retention and report reduction.
The adapter supplies only closed class/protocol/resource/outcome values and
numeric scalars. It never records request URLs, queries, headers, bodies, tokens
or free-form errors. Parent processes observe their own requests; child requests
require the child's own observer. An uninstrumented child remains unknown.

## Actual GraphQL cost

With an admitted observer, known compiled GitHub read queries against the exact
public GitHub API base add `_assayPerf:rateLimit{cost}` to that same operation.
The GitLab trust read against the exact public GitLab base similarly adds
`_assayPerf:queryComplexity{score}`. Both metadata fields are documented by their
providers: [GitHub point value](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api#returning-the-point-value-of-a-query)
and [GitLab query complexity](https://docs.gitlab.com/api/graphql/getting_started/#query-complexity).
Business selections and variables remain unchanged. There is no extra request,
capability probe, retry fallback or additional credential lookup.

The existing backend decode supplies scalar metadata from the bytes it already
consumes. A second JSON decode extracts only this bounded metadata; it adds CPU
work, not another body read. GitHub points and GitLab complexity retain distinct
units. Returned numeric zero is measured; missing/null metadata is unknown;
malformed metadata is failed. REST/auth cost is unsupported. This adapter emits
no estimates. Rate-limit remaining/used/reset gauges are never differenced or
summed as actual charges. Unknown/failed samples contribute no invented zero.

Custom operations, mutations, enterprise and custom provider schemas receive no
query modification and retain unknown cost. Optional missing metadata does not
fail a business operation. The qualified public schemas can still reject an
operation; its original provider error remains the result, with unknown cost.
No recovery send is performed for telemetry.

## Current production coverage

`desktoken` explicitly passes its selected session into all six current auth HTTP
leaves: installation discovery, token exchange, coverage installation/repository
reads, GitLab rotation and self-check (including the existing previous-token
diagnostic). These requests use class `authentication`, separate from provider
operations, without increasing legacy forge-call counters. Cached credentials,
local refusal and `--no-rotate` can establish zero auth sends when the complete
invocation inventory is selected. Consent off keeps the original client.

Native backends observe provider requests only when their caller passes the
explicit observer. Existing uncomposed tools/custom clients remain gaps; a new
transport wrapper is not evidence of complete fleet coverage. Hot-path
composition in deskboard/deskdispatch and the future credential provider remains
desktools-v2/62; issueboard/deskinbox integration is desk-tools/42. Statusgen
retains its own provider owner and does not import the authenticated backend.
Future service/refresh/poll/repair producers pass their own bounded origin;
none is inferred from a tool name. No future service or cache is introduced here.

Deskboard's operational `charged` field remains the historical
requests-minus-304 proxy. It is not actual GraphQL points. Its quota-floor
admission, default floor and ETag/cache behavior are unchanged.

## Qualification and limitations

The three named `/65` regressions use real backend/transport/record/report paths;
the auth fixture runs the actual parser and existing auth leaves. The committed
`forgeperf-mutations.json` plants missing leaf observation, headers-only timing,
nested double counting, fabricated zero cost, omitted auth observation and
counting above the real board cache/admission adapter, and lost GitHub cost
metadata behind the existing JSON response sanitizer. All seven controls must
fail their named test. The sanitizer remains in place; a per-operation capture
links the existing decode to the actual observed response without changing
response headers or business bytes.

Matched fake-provider samples alternate recording on/off, 20 cycles each,
100 requests/cycle, concurrency 1. REST uses a 17-byte JSON response; the
GraphQL variant also includes same-operation metadata selection/extraction and
its larger response. Enabled samples include aggregation and local append.
The final matched run measured REST wall ns p50/p95/max 267958/632125/642500
off and 517542/866084/1117542 on; summed CPU was 17 ms off and 22 ms on.
GraphQL measured 459292/807709/919250 off and 865292/1510625/1764750 on;
summed CPU was 19 ms off and 31 ms on. Both modes made exactly 2000 sends
per protocol. Both use actual JSON content types and preserve the GitHub SDK's
response sanitizer. These are local instrumentation costs, with coarse CPU
granularity and no real network latency, provider rate pressure,
wide-concurrency or fleet overhead claim. Test output reports fresh samples;
these historical figures are not acceptance thresholds.

Removing the observer argument or deselecting the optional profile restores the
prior measurement path. There is no migration of stored legacy records, pin
change, operational activation or release in this change. Independent verification
still owns acceptance after shared prerequisites and this implementation land.

Protocol labels currently recognize the native public GraphQL paths only. An
enterprise or custom subpath can remain labelled REST even when its API uses
GraphQL; its cost is still unknown, never inferred from that label. This is a
coverage limitation rather than qualified enterprise protocol attribution.

## verifyloop scheduling consumer

verifyloop composes `RequestObserverFor` at its existing verifier-role forge resolution
for blocker and pending-Evidence reads. The context is selected before those reads from
the existing explicit environment profile. With no profile, legacy recording stays perf-v2;
no observer is installed. Selected profiles reuse the bounded perf-v3 own-request duration,
protocol/resource/outcome and GraphQL-cost observations above.

The invocation also records a fixed phase roster and separates `plan` from `verdict`,
including both dry-run shorthands. Phases are inclusive, can overlap, and are never added
to wall time. Preflight's separate transports, auth/row subprocesses and verifier children
remain coverage gaps; this integration does not mark the complete request inventory
measured. Cache/waiter counters remain unavailable. No endpoint or subject labels, new
requests, cache, parser flags or scheduling changes are introduced. See the
[draft performance extension](verifyloop-performance-spec.md) for the boundaries and proposed
local/future follow-ons; diagnostic integration alone does not approve those changes.

For a supported build containing this integration, an explicit local perf-v3 tool run is:

```sh
ASSAY_TELEMETRY=1 ASSAY_PERF_PROFILE=tool verifyloop plan --root . --json
deskperf report --days 1 --tool verifyloop --json
```

Use `ASSAY_PERF_PROFILE=workflow` only when local work/session attribution is wanted;
workflow grouping in the existing reporter additionally requires `--perf-profile workflow`.
The tool profile carries no workflow subjects. A real plan retains its normal preflight
and authorized scheduling reads; the example is not an offline test recipe.

The verifyloop performance regression suite exercises both selected v3 profiles, the
legacy/off paths, pending-admission parity and signed/unsigned verdict phases. It runs
the production callback and a built executable; scheduling requests pass through the
production factory and resolver with fixture custody and a fake physical HTTP transport.
`performance-mutations.json` uses the existing muhar harness to pin observer propagation,
early context selection, main entrypoint wiring, phase coverage and dry-run attribution.
