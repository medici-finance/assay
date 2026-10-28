# Pending Evidence observation reuse

This is the bounded pending-Evidence and attestation follow-on to
[verifyloop-performance-spec.md](verifyloop-performance-spec.md). It extends the existing
planning-pass owner; it introduces no service, persistent cache, credential owner,
concurrency, preflight exception or automatic row execution.

## Observation and evaluation boundaries

Each admitted repository's existing `pendingChanges` value now owns:

- File bytes and parsed Evidence bindings keyed by repository, exact commit and path.
  Regular-file proof remains mandatory. Mutable references are refused; unavailable
  observations are retained only for the current selection. Returned bytes are copied.
- Validated scheduling receipts, including failures, keyed by repository and attestation
  number. The verifier role, resolved forge and active validation policy belong to the
  enclosing pass. A new `SelectQueue` clears the owner along with its forge observations.
- A brief-to-change index built from complete changed-file reads already performed.
  The first exhaustive walk supplies it. An early proven hold retains lazy reading.
  Unreadable verifier changes remain candidates for every brief, in original order;
  removed files and non-verifier changes establish no membership. Incomplete population
  still makes absence unknown.

Per-item implementer independence, source association, exact Evidence binding, observed
source/plan, receipt scope and wake evaluation run for each item. A cached receipt is
scheduling evidence, never execution or publication authority.

## Attestation consolidation and retained rechecks

`ReadVerifierAttestation` now supplies its first typed issue observation to the existing
exact-target/body-history validation. The shared predicate checks the same target kind,
dispatcher authority, exact title/body and complete, known, unedited body history.

A **second issue read remains after body-history validation**. It rechecks target,
canonical body and closed state, and supplies current labels before the complete stamp
history read. `AttestedModelStampOf` still establishes the dispatcher model/tier pair.
A successful one-page validation therefore needs two issue reads and both provenance
walks: four provider operations rather than five. Removing the remaining issue read
would lose the mutation controls, so it is deliberately retained.

`StampTimelineFor` and `attestationRecord` retain their existing fresh-read behavior for
other callers. The extracted helpers consume typed observations without changing those
callers' trust predicates or partial-error results. Execution and Evidence admission
continue through their own fresh checks. A proposed pending hold still rereads PR
head/base/state after immutable file evaluation, on every lookup, including receipt hits.

## Matched qualification

The saved comparison uses one fixed 266-item, four-root snapshot. All roots passed normal
preflight, both plans were complete with exit 0, and all plan values and prompts matched.
Operational identities and raw plans are retained privately; only aggregate measurements
appear here. Both diagnostic builds used the same compiler, CPU/runtime profiler and
HTTP write-attempt observer.

| Observation | Parent | Reuse |
|---|---:|---:|
| Total GitHub HTTP requests, including preflight | 280 | 187 |
| Scheduling provider requests | 272 | 179 |
| Exact-commit file requests | 49 | 27 |
| Attestation issue-object requests | 117 | 62 |
| Attestation body-history requests | 39 | 31 |
| Attestation label-history requests | 39 | 31 |
| Measured GraphQL points | 42 | 34 |
| Instrumented wall time, first pair (parent then reuse) | 78.590 s | 58.631 s |
| Instrumented wall time, reverse-order repeat | 120.825 s | 299.382 s |

The 93-request reduction is explained by 22 file reads, eight repeated five-operation
attestation validations, and one issue read for each of 31 distinct validations.
The first pair saved 25.4% wall time, but the reverse-order repeat was slower despite
the same request reduction and identical plan. No stable live wall-time improvement is
established by these samples. REST quota units remain unsupported, and no monetary
saving is inferred.

The total counter covers native provider/Git HTTP, credential-child HTTP and the real
preflight GH identity probe. The normal credential cache was warm; no credential-child
HTTP occurred in this pair. Four Git discovery requests and four identity requests stayed
unchanged. Only GraphQL reads used POST. Build/provisioning/inspection traffic is outside
the measured invocation. The existing perf-v3 native request coverage remains partial;
the diagnostic outer counter does not relabel it as complete.

## Hotspot findings

A previous sequential pair took 120 versus 221 seconds while sending the same 280
requests; its recorded provider-request duration grew from 109 to 212 seconds. That
observation alone could not establish a local-code regression. Unchanged release source
subsequently completed in 81 seconds with exactly the same plan and request count.

CPU/runtime profiles and HTTP traces locate the current hotspot in serial provider
response waiting. In the new parent sample, response-header time was 69.853 seconds,
including 69.613 seconds waiting for the first response byte after the write. Combined
DNS/connect/TLS time was about 0.2 seconds; 274 of 276 native requests reused a connection.
CPU was 1.885 seconds. These observations do not identify whether remote service,
network, contention or throttling caused the earlier latency swing. Goroutine wait
profiles overlap and must not be summed into process wall time.

The slow optimized repeat used 2.451 seconds of CPU in 299.382 seconds wall time. The
runtime blocking profile attributes 201.02 seconds on the planning path to
`net/http/internal/http2.transportResponseBody.Read` waiting on its data pipe, with
`GitHubForge.doJSON` above it. The full observed provider-request duration was 290.198
seconds; response-header time was a smaller part. This distinguishes response-body waits
from CPU work and from merely waiting for headers. The root cause of those transfer
stalls is unresolved. No HTTP protocol or transport default is changed by this PR.

The optimization reduces the repeated work rather than introducing parallel provider
traffic. Remaining distinct attestation/trust/stamp reads and distinct pending changes
retain their existing semantics. The subsequent [ordered attestation batching
change](verifyloop-attestation-batching.md) consolidates those distinct reads while
retaining their four observation boundaries. Conditional transport and served observations
remain separate work in the parent specification.

## Reproduction and safety controls

Run inside `tools/desk`, with a canonical temporary directory on macOS:

```sh
TMPDIR=/private/tmp GOWORK=off go test ./cmd/verifyloop ./internal/deskkit
TMPDIR=/private/tmp GOWORK=off go test -race ./cmd/verifyloop ./internal/deskkit \
  -run 'TestPendingReuse|TestPendingFileReuse|TestPendingAttestationReuse|TestPendingIndex|TestVerifierObservation'
TMPDIR=/private/tmp GOWORK=off go run ./cmd/muhar \
  -spec cmd/verifyloop/pending-reuse-mutations.json -j 2
```

The mutation harness has a green baseline and caught positive control. All eleven
mutations are caught: lost PR-head reconciliation, lost consequential attestation read,
stale pass, repository/commit cache collisions, repeated receipt reads, mutable byte
leakage, non-exact refs, lost unknown-file membership and skipped body-edit history.
Existing pending-source, actor/id independence, partial coverage, scope, wake, cross-repo,
model/stamp and dispatch checks run through the unchanged production path.

Instrumentation continues using perfrec's existing perf-v3 tool profile. The fixed phase
roster stays within its existing bound. `attestation.n` counts actual receipt validations;
`pending-evidence.n` still counts item evaluations. Neither implies a persistent/provider
cache hit. No new telemetry schema, cache counters or attribution knobs are introduced.
