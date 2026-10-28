# Ordered attestation batching for verifyloop

The pending-Evidence reuse change removed repeated reads of the same attestation, but
one distinct successful attestation still required four provider requests. In the
earlier four-root profile, 31 distinct attestations accounted for 124 of 187 total
GitHub requests. This change collects that distinct population before validating it.

## Composition and authority

`VerifyLoop.SelectQueue` supplies the existing scheduling source with its eligible brief
paths. Collection itself makes no network request. At the first pending-Evidence read
for a repository, preparation uses the same admitted Forge, complete PR/file listings,
trusted independent verifier-author/source predicates and exact-commit file cache as lazy
evaluation. Only references in relevant verifier-authored brief files enter the batch.
Historical bindings are retained; source age or a newest-only heuristic cannot establish
that an old failed/blocked receipt is irrelevant. An incomplete file listing still
contributes uncertainty in ordinary evaluation.

`deskkit.VerifierAttestationBatchReader` supports the native GitHub backend and its known
outbound decorator. It does not unwrap arbitrary wrappers or read-only credential fences.
GitLab and unsupported decorators keep the existing lazy per-target reader. The batch
reader uses `GitHubForge.doJSON`, the existing go-gh client, custody and request observer.
It introduces no credential owner, service, transport setting or global cache.

For each chunk of at most 32 distinct issue numbers, the reader performs four separate
GraphQL operations, in order:

1. Typed issue observations, exact title/body/author identity and complete current labels.
2. The canonical issue-trust selection and reducer, including explicit body-edit provenance
   and the existing bounded comment-history completeness rule.
3. New issue observations after body history, including current body, closed state and labels.
4. Label application/removal history after current labels, including actor identity.

An invocation-local adapter supplies these values to the shared
`ReadVerifierAttestation` validator, whose admission predicates are unchanged. Exact target/title/body/run/source/brief/model/tier,
unedited-body provenance, closed state and dispatcher stamp checks retain their shared
owners. The adapter implements only the three read capabilities consumed by the validator,
with no embedded Forge. A new validator dependency requires an explicit adapter change
at compile time. The adapter cannot send an unexpected extra request. Errors and missing aliases
are target-specific uncertainty; unattributable GraphQL or transport errors stop further
batch requests. Provider error messages are not echoed. An overflowing label connection
uses the existing complete native reader; its errors never become empty labels/history.
The second issue observation is never replaced by the first.

Preparation stores receipt results and errors in the existing per-pass receipt map.
Normal per-item evaluation still checks the exact Evidence binding, original/current
brief inputs, outcome records, dependency membership and wake predicate. Every proposed
hold still reconciles live PR head/base/state after file evaluation. New selection clears
all mutable observations. Dispatch, execution and Evidence publication retain their
independent admission and current-state reads.

## Instrumentation and qualification

The existing fixed 16-phase perf-v3 roster is retained. Preparation runs inside the first
per-item `pending-evidence` scope. `attestation` times an admission-reader invocation:
a native bulk invocation can cover multiple records, while the fallback remains one
invocation per record. Its `n` must not be interpreted as the number of records or requests.
Physical attempts, bytes and actual GraphQL points remain the transport observer's facts.
The three fixed batch documents join the existing closed cost-query inventory; targets
are variables and no provider text expands that inventory. Legacy consent and unknown
custom/enterprise-schema cost behavior are unchanged.

The measured parent is `fb7d8bb04ba46eddf8e75867e61620c19a4f74fe`; the measured
subject code is `0758413cfc581ba96ee83421ebd21bdf69d813c6`. Both used native default
HTTP settings, matching standard-library counters/runtime profiling and normal verifier
custody. CPU-heavy tests were stopped during measurement. The following table measures that subject revision. A subsequent review follow-up
narrows the validator adapter to an explicit read interface and adds boundary coverage;
it does not alter request construction, observation order or admission predicates.

Measured parent/subject/subject/parent on the same four frozen roots, with normal preflight and custody:

| Run | Wall | Total GitHub HTTP attempts | Measured GraphQL points |
|---|---:|---:|---:|
| Parent | 61.987 s | 189 | 34 |
| Batch | 28.367 s | 69 | 7 |
| Batch repeat | 26.580 s | 69 | 7 |
| Parent repeat | 62.517 s | 190 | 34 |

The 190-call repeat includes one normal token-refresh request. The other parent/subject difference is exactly 120 scheduling requests: 124 attestation reads become four. One additional verifier PR since the earlier profile adds two reads to both versions, explaining 69 rather than the prototype's 67-call budget. All four runs produced exactly identical complete 266-item plans, admitted all four roots, preserved all 28 uncertainties, and left their frozen HEADs/tracked files unchanged. Mean wall time fell 55.9% (2.27×); this is a repeated observation for this population, not a universal network-latency guarantee.

The plans contained 81 dispatchable items, 2 Evidence-only, 183 bucketed and zero
pending holds. Scheduling requests fell 181 → 61; the remaining eight common calls were
four Git preflight advertisements and four identity probes. Parent CPU samples were
2.169 / 2.095 seconds, versus 1.586 / 1.489 for batching. The attestation phase fell
36.445 / 36.387 seconds → 2.370 / 2.581. Mean total wall time was 62.252 → 27.474 seconds.
Provider response bytes fell from about 4.30 MB to 3.44 MB. All scheduling responses were
2xx and request drops were zero. REST point cost remains unsupported and partial recorder
coverage remains explicit. No monetary saving is inferred. Raw plans, operational root
identities, counter traces and profiles are retained privately.

The repeatable request reduction is 63.5% against the matched 189-call parent, or
120 scheduling requests in either pair. The original 280-call profile predates both
reuse and batching and is historical context, not this change's wall-time control.
Overflowing connections can add requests above this population's measured 69; the
complete native fallbacks are retained for correctness. The real mutation harness has
a green baseline, caught positive control, all thirteen mutations caught and zero
could-not-mutate. Full planner/shared-library suites, focused race coverage, documentation
lint and public-surface scan qualify the change.

The collection step can observe more relevant files up front than a successful early
lazy hold would. That bounded tradeoff is explicit: it excludes unrelated/non-verifier
files, keeps the original evaluation order, and qualifies positive holds separately from
the zero-hold live population. No local candidate-pruning heuristic was added.

Focused regressions cover per-target errors, missing provenance, edit/reopen/relabel
changes across stages, pagination fallbacks, scope fences, bounded chunks, candidate
scoping, missing batch results, pass refresh and fresh PR-head reconciliation. The
mutation recipe is `tools/desk/cmd/verifyloop/pending-batch-mutations.json`.

Review follow-up confirmation at `044a1cd9ed9d47c65214056dfc74a9a7007760b9`:
normal preflight again admitted all four frozen roots; the complete 266-item plan was
exactly equal to the original qualification. Total GitHub attempts remained 69,
scheduling attempts 61 and observed GraphQL points 7. Tool wall time was 29.436 seconds
(CPU 1.678 seconds), using the same counters, profiles and native transport defaults.
This is a single follow-up confirmation, separate from the paired timing comparison above.
The expanded mutation recipe catches all thirteen defects, including the adapter's third
issue read, invalid repository slug, foreign receipt repository and a valid change-kind
reference. The subsequent test correction uses a parsed PR URL to reach the kind guard;
it changes no production code.
