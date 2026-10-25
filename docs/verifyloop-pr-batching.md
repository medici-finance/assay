# Pending-Evidence PR metadata and file-list batching

This slice follows the per-pass pending-Evidence joins and ordered attestation
batching. It reduces the remaining two-request-per-verifier-PR discovery pattern;
it does not change scheduling policy, credential custody or execution admission.

## Read path and preserved checks

The existing optional native-GitHub capability fence admits the backend and its
known outbound decorator. GitLab, read-only and unknown decorators retain the
existing complete per-target reader. No new Forge interface method is added.

For two or more distinct verifier PRs in one admitted repository, chunks of at
most 32 use two ordered operations: first metadata, then complete file-list
observations. The later observation must carry the same PR number, head SHA and
changed-file count. Its connection total, explicit pagination proof and unique
file paths must agree. Missing/unknown data and per-alias errors remain uncertainty;
transport and unattributable errors stop later requests. A connection beyond 100
files uses the existing complete native walk, including its count proof, and must
still bind to the original head/count. Singleton populations retain the existing
native two-request path rather than adding GraphQL point charges without saving a
round trip.

The projection includes only fields used by pending-Evidence admission. File paths
and removed/renamed status are preserved; patches and rename ancestry are not
provided by this planning capability. General Forge readers used for risk checks,
publication and other consumers remain unchanged. Actor rendering and numeric
identity use the existing GraphQL reducer; PR mapping uses the existing REST-wire
mapper; complete file-count validation has one shared owner.

Only changes naming the pinned verifier login enter preparation. The existing
per-pass cache retains both results and errors. Evaluation still confirms the
numeric trusted author, independence from the implementer, same-repository
provenance, open/unmerged state, default base, list-to-metadata head binding,
complete file membership, exact-commit file content, exact attestation binding,
body/stamp history, outcome rows and wake inputs. Every proposed hold still performs
its independent current PR head/base/state read. Dispatch, execution and Evidence
publication retain their own admission and consequential rechecks. A new selection
discards all mutable observations; no persistent mutable cache is introduced.

## Repeatable busy verifier-PR benchmark

`BenchmarkPendingChangeBusyQueue` compares the actual native per-target reader
against the actual batch capability through local HTTP, production transport,
decoders and the shared completeness proof. Fixed populations are 1, 32 and 65 PRs,
each with two files. An explicit 1 ms service delay makes round-trip scaling
repeatable; it is a modeled service delay, not measured GitHub latency. The benchmark
asserts the request budget on every iteration and reports requests/op plus wall
ns/op and allocations. It exercises PR discovery, not a complete live desk or its
preflight, attestation, dispatch and publication costs. Separate planner regressions
prove positive holds, trust refusals, pass refresh, cached uncertainty and fresh
head checks. The live four-root comparison retains all normal preflight checks.

Run from `tools/desk`:

```sh
GOWORK=off go test ./internal/deskkit -run '^$' \
  -bench '^BenchmarkPendingChangeBusyQueue$' -benchtime=3x -count=5
GOWORK=off go run ./cmd/muhar \
  -spec cmd/verifyloop/pending-pr-batch-mutations.json -j 2
```

The request budgets are 2/64/130 for per-target reads and 2/2/6 for the selected
capability. Overflow adds complete native reads; unknown data never becomes an
empty list to meet a budget. Existing validator and per-pass mutation controls
remain in force. Fixtures are offline, contain public-neutral invented data and
need no token or provider access.

## Instrumentation and qualification

The two fixed documents join the existing closed same-operation cost inventory;
no cost-only query is sent. The existing phase, context, recorder and reader owners
remain unchanged. Record the schema actually emitted by the measured source;
perf-v4 selection remains its separately owned producer/reader migration.

Qualification must compare matched parent/subject sources, the same frozen four
root HEADs and contemporaneous provider state. Count native provider, Git preflight,
CLI identity and credential-child traffic separately before totaling them. Retain
partial/unsupported cost coverage, verify complete root admission and unchanged
root contents, and compare scheduling outputs and uncertainty. A drained live queue
cannot replace the fixed busy population or qualify a universal speedup claim.

## Qualification results (2026-10-21)

Matched source parent `aec9230c02756c7eccfbf61ddde66709dc4397c7` and
implementation `a4a2e58e63c0d6327b908dc6054ec858d1670f8b` were built with the
same Go toolchain and default transport. Four frozen root HEADs were admitted by
normal preflight on every run; tracked root contents remained unchanged. The run
order was parent, batch, batch, parent. All four complete JSON plans were exactly
equal: 266 awaiting, 84 dispatchable, 2 Evidence-only, 180 bucketed, including the
same 24 uncertain items. No execution or publication was performed.

| Run | Wall seconds | Total GitHub requests | Native REST | Native GraphQL |
| --- | ---: | ---: | ---: | ---: |
| baseline-a | 31.478 | 53 | 38 | 7 |
| subject-a | 16.319 | 25 | 8 | 9 |
| subject-b | 15.217 | 25 | 8 | 9 |
| baseline-b | 27.256 | 53 | 38 | 7 |

Total counts include four Git preflight advertisements and four CLI identity
probes per run. Credential children succeeded from cached credentials with zero
HTTP requests; native provider attempts matched independently counted physical
HTTP/2 sends, with zero recorder drops and only successful provider responses.
Debug headers and bodies were consumed in memory and discarded; only aggregate
counts and normal safe diagnostics were retained.

Mean wall time fell from **29.367 s to 15.768 s** (46.3% shorter, 1.86x), and total
requests from **53 to 25** (52.8% fewer). These are two observations per source,
not a latency guarantee: provider state and machine/network load vary. Per-phase
wall observations can overlap and must not be added together. Measured same-query
GraphQL points were 7 before and 9 after; REST point coverage is unsupported, so
this is not evidence of lower total rate-limit points. Both measured sources
actually emitted **perf-v3**, not perf-v4.

The offline benchmark ran five repetitions of three iterations per case:

| Verifier PRs | Per-target requests | Selected reader requests | Median before ms | Median after ms |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 2 | 2 | 2.855 | 2.892 |
| 32 | 64 | 2 | 87.763 | 4.052 |
| 65 | 130 | 6 | 221.058 | 15.521 |

The singleton result is unchanged within timing noise. Busy-population timings
reflect the explicit local 1 ms delay and isolate PR-discovery scaling; they must
not be presented as production end-to-end speedups. The benchmark asserts budgets
on every iteration. Full verifyloop/deskkit suites, focused race tests, vet and
documentation lint passed. The mutation harness had a green baseline, caught its
fresh-head positive control and caught all eight declared mutations.
