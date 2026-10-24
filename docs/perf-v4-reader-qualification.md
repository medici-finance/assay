# Shared performance reader qualification

The canonical `perfrec` library owns the bounded perf-v4 codec, registered typed
facets, identity selection and common observation reduction. Desk adapters own file
access and report projection; statusgen keeps its execution and verification
authority. No online adapter, token resolver or generic raw payload crosses that
boundary. This brief implements the reader first; selected producer migration is
separate desktools-v2/68 work. Existing retained records are never rewritten.

## Local fixture measurement

Five alternating fresh-process pairs use the same executable and deterministic,
equivalent 200-observation inputs. Each process measures its first decode/reduce
and 15 warm repetitions, verifying exact record count and summed wall facts.
Input and executable hashes plus raw logs are retained with the implementation
artifacts. This is a local codec/reducer fixture, not a physically cold filesystem,
whole-tool speedup, production workload or collection activation.

| Metric (median across five processes) | Existing v3 | New v4 |
|---|---:|---:|
| Input bytes | 161000.000 | 140200.000 |
| First decode/reduce, ms | 5.612 | 7.074 |
| Warm p50, ms | 5.434 | 6.361 |
| Warm p95, ms | 6.001 | 8.331 |
| Own CPU over all 16 passes, ms | 101.000 | 121.000 |

The extensible reader costs more in this fixture; no performance improvement is
claimed for the wire migration. The independent issueboard/lint/verifyrun changes
retain their matched original measurements until supported selected producers
can be composed with both source variants.

## Controls exercised

- Actual typed writer, file reader, compiled deskperf JSON/text/day report and
  external Go consumer; mixed historical data, permission filters, STOP refusal
  and unavailable observation coverage remain distinct.
- Duplicate/case aliases, late schema downgrade, malformed known facets, bounded
  unknown optional facets, source identity conflict and cross-schema collisions
  fail closed without exposing discarded extension names or payloads.
- Shared reducer and per-record coverage reject integer overflow; exact maximum
  valid totals remain valid. Historical validator/control assertions remain.
- Canonical local mutation baseline and distinct control are healthy: ten new
  codec/identity mutations, three overflow mutations, eight retained v3 mutations
  and three retained command-report mutations are caught. No survivors or
  could-not-mutate are counted as passing. This is local harness qualification,
  not an assertion that every spec is enforced by the CI truth matrix.
- Full shared-library and actual deskperf suites, race/vet, relevant adapter and
  cross-module CI-trigger checks, mutation anchors, repository lint and the
  committed test ledger are required before publication. Independent correctness,
  security and postmerge verification remain separate; Evidence is unchanged.
