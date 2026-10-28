# Verifyloop planning observation qualification

This follow-on implements the association-scan part of
[the verification performance specification](verifyloop-performance-spec.md).
It reuses gitcore and the perfrec perf-v3 recorder introduced for verifyloop in
[#3738](https://github.com/medici-finance/assay/pull/3738). Other provider, startup,
publication and future service work retains its existing owners.

The algorithm previously restarted checkout identity, HEAD and registry discovery for
each declared brief, then repeated tracking identity/HEAD for cross-repository items.
`SelectQueue` now gives the entire scan one pass-scoped reader. Work is proportional to
distinct roots, root/commit registries and root/commit/path declarations. Handles,
observed failures and parsed declarations are discarded at the end of the scan.
The scheduling source is reset at scan entry too. Blocker observations are keyed by
admitted repository, target kind and number. Default wake-input revisions are reused per
root/input, retaining receipt-scope validation. An all-rejected multi-root pass stops
before scanning or provider discovery while preserving incomplete coverage and exit 6.
No permanent cache, new service or alternative declaration parser is introduced.

## Reproducible neutral benchmark

`BenchmarkAssociationQueue` constructs a complete offline fixture: one tracking checkout,
one source checkout, both at fetched main. One third of briefs are legacy, one third declare
the tracking repository and one third declare the source repository. All rows are
implemented, have an empty Evidence section and resolve successfully. Fixture creation
and Git commits are outside timing; the measured operation is production `SelectQueue`,
without a forge source. This exercises planning, not preflight or execution admission.

Baseline: `c7afc51121d636c0a6200f454dc45287ad35e32f`. Use the benchmark and its fixture
helpers from this change with unchanged baseline production files, then with this change's
production files. The baseline was built using a Go overlay restoring those production
files from the named commit; test inputs and fixture helpers were identical.

From `tools/desk`, run each binary with the equivalent of:

```sh
go test ./cmd/verifyloop -run '^$' -bench '^BenchmarkAssociationQueue$' -benchtime=1x -count=5
```

Five alternating before/after samples per size, Apple M4 Max, Go 1.27.2, medians:

| Briefs | Baseline scan | Pass-scoped gitcore scan | Reduction |
|---|---:|---:|---:|
| 12 | 389.2 ms | 23.1 ms | 94.1% |
| 96 | 3,248.8 ms | 62.4 ms | 98.1% |

A separate perf-v3 observation of the complete 96-row fixture measured 448 local Git
subprocesses before and two after, with zero HTTP operations on both sides. The two
remaining subprocesses are the scoped origin reads, once per opened root. The final
record contains two root opens, one registry validation and 64 declaration resolutions.

These are synthetic component timings, not an end-to-end or fleet SLA. Filesystem cache,
repository object layout, brief mix and machine load affect results; the local benchmark
is not a cold-disk trial. The association-only benchmark makes no provider requests or point-cost claim. Normal
preflight, credential work, pending-Evidence reads, row execution and publication remain.

## Correctness and freshness

The regression suite compares production queue items against the original fresh reader
on all 96 fixture rows. The production scan still reads its named roots under hostile caller Git variables.
Include/worktree remote overrides agree with the fresh reader, preventing a declared
cross-repository item from being misclassified as same-repository. Gitcore handles object
and reference reads; effective origin is intentionally read once per opened root through Git. Telemetry records 64 declaration resolutions, two root opens and one
registry read for that fixture, using fixed labels only.

A second selection observes new source commits and recovers after a failed registry is
repaired. Immutable declaration keys separate commits, paths and roots, including linked
worktrees sharing objects. Returned declaration slices cannot mutate memoized results.
Open failures remain stable for one pass and are retried on a new pass.

After planning, move source or tracking HEAD, fetched main, origin, a declaration, the graph
registry or the configured source root. Both native and interim dispatch must refuse before
creating a worktree or invoking a feeder. Those checks still use the existing fresh Git
reader; the pass reader is never stored on a queue item or loop. The independent receipt,
source-closure and landing checks remain in place.

Planning and dispatch now use the same effective config scope for origin identity.
The temporary local-only configured-URL helper has been removed. Missing inputs remain
unavailable and invalid declarations retain their policy refusals. Lower-level object I/O
diagnostic text may name gitcore rather than a Git subprocess failure.

The gitcore object reader is currently qualified for SHA-1 repositories only. SHA-256
repositories fail closed as could-not-check; extending gitcore object-format support is a
separate migration-owner task, not a fabricated success here.

## Provider fan-in and avoided work

`TestVerifyPerformanceBlockerFanIn` composes the production factory, verifier custody,
Forge adapter, counted transport and perf-v3 observer. One hundred references to one
blocker produce one physical provider attempt. Issue/change, number and repository
collisions, unavailable results, caller diagnostics and next-selection refresh have
separate regressions.

Five alternating baseline/subject samples with an identical fake physical transport and
10 ms simulated latency per send measured:

| 100 references to one blocker | Baseline | Subject |
|---|---:|---:|
| Physical HTTP attempts | 100 | 1 |
| Median summed `blocker` phase | 1,089 ms | 12 ms |

This is controlled latency, not a measured GitHub response-time promise. REST quota cost
remains unsupported in the record; request savings do not establish money or GraphQL-point
savings. For a real pass, saved sends equal repeated references minus unique admitted
blocker targets; a queue with no repeated blockers gets no blocker-read saving.

A matched two-rejected-root fixture observed two scheduling HTTP attempts and an
unadmitted fallback item before, versus zero attempts and zero items after. Both reports
remain incomplete with two skipped roots and exit 6. Healthy-root and explicit-root
controls still scan. No root is silently treated as an empty healthy queue.

## Diagnostics

The existing `ASSAY_TELEMETRY=1 ASSAY_PERF_PROFILE=tool` opt-in records the new fixed phases
`association`, `association-root` and `association-registry` in perf-v3. Their counts show
committed declaration queries, distinct open attempts and registry validation attempts.
Nested timings are inclusive; do not add them to `queue-scan` or process wall time.
The blocker phase counts logical queries; the existing request observer counts physical
attempts. The roster now uses all 16 phase slots, with a regression preventing silent
truncation; future phase additions must consolidate labels or change the shared bound.
No new exporter, spool, metric schema or high-cardinality label is added. Missing provider
coverage remains missing; local reuse does not imply a provider-cache hit.

## Sibling call sites

| Caller | Treatment |
|---|---|
| `SelectQueue`, single-root and multi-root scans | One reader per scan, shared across all roots. |
| `scanAwaiting` used by verdict commands | Fresh pass per scan through the same scan helper. |
| Native/interim dispatch, attested receipt admission, `VerifierDeliverables` admission callers | Existing fresh Git reads and shared declaration policy retained. |
| Pending-Evidence source reads and forge observations | Existing owners/caches retained; other reuse work remains in the parent specification. |

## Validation

The original affected suites passed. The expanded follow-up also qualifies `cmd/muhar`
and the real mutation harness, including its required positive control. Mutations cover: omit the dispatch recheck, omit the pass's fetched-main gate, collapse declaration
revision keys, reread registries, collapse root keys, accept mutable revision keys, and
allocate a fresh reader per scanned item, blocker key/reuse/refresh, rejected-root scanning,
wake-input reuse and scoped origin identity. The unmodified baseline passes. The anchors and test
command are in `tools/desk/cmd/verifyloop/association-pass-mutations.json`.
