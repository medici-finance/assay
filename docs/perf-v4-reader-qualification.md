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
| First decode/reduce, ms | 5.126 | 5.675 |
| Warm p50, ms | 5.026 | 5.584 |
| Warm p95, ms | 5.293 | 5.970 |
| Own CPU over all 16 passes, ms | 91.000 | 105.000 |

The extensible reader costs more in this fixture; no performance improvement is
claimed for the wire migration. The independent issueboard/lint/verifyrun changes
retain their matched original measurements until supported selected producers
can be composed with both source variants.

## Controls exercised

- CI quiet-home guard reproduced the new compiled fixture starting Go under a
  scratch home without the existing telemetry-child isolation. The fixture now
  uses shared test-only `gitquiet/goquiet.Cmd` and checks for stray files; the
  original guard remains unchanged. Assay recording and consent are unaffected.
- Actual typed writer, file reader, compiled deskperf JSON/text/day report and
  external Go consumer; mixed historical data, permission filters, STOP refusal
  and unavailable observation coverage remain distinct.
- Internal review reproduced padded LF/CRLF equality-bound lines being admitted;
  the raw-byte public decoder and native file splitter now retain those bytes.
  Below/equal/above-limit regressions retain this boundary. Multi-line regressions also
  keep valid records after an oversized LF/CRLF line, including multi-buffer and
  unterminated oversized tails; each oversized line contributes one skip.
- Duplicate/case aliases, late schema downgrade, malformed known facets, bounded
  unknown optional facets, source identity conflict and cross-schema collisions
  fail closed without exposing discarded extension names or payloads.
- Shared reducer and per-record coverage reject integer overflow; exact maximum
  valid totals remain valid. Historical validator/control assertions remain.
- Canonical local mutation baseline and distinct control are healthy: eleven new
  codec/identity mutations, four raw-line adapter mutations, three overflow mutations,
  eight retained v3 mutations and six command-report mutations are caught. No survivors or
  could-not-mutate are counted as passing. This is local harness qualification,
  not an assertion that every spec is enforced by the CI truth matrix.
- Full shared-library and actual deskperf suites, race/vet, relevant adapter and
  cross-module CI-trigger checks, mutation anchors, repository lint and the
  committed test ledger are required before publication. Independent correctness,
  security and postmerge verification remain separate; Evidence is unchanged.

## Review follow-up

Correctness review 5643845827 at the original reader head requested changes for
multi-line truncation and omitted command-view v4 exclusion counts. Both were
reproduced before repair. The bounded splitter now drains an oversized line and
resumes at its next LF without increasing the allocation bound; command text
prints v4 duplicate, conflict and invalid counts. The shared reader also retains
the published command-report repair from PR #4352, including its four regressions
and two added mutation controls. Main's canonical forge adapter row and this
reader's performance ownership row both survive the actual documentation conflict.

Decode refusals, including over-bound lines, contribute to the file reader's
`skipped` count, rather than the reducer's `Invalid` count. V4 request counters
appear in the separate perf-v4 projections; the existing pooled and by-day forge
counts do not include v4. They must not be read as all-schema provider totals.
Command-view saturation and unavailable attribution under workflow filters remain
nonblocking review observations; this repair does not change their semantics.

The earlier codec/reducer fixture remains scoped to its measured implementation;
no new whole-tool or selected-producer performance sample is claimed here.

A follow-up boundary regression checks the historical final line without LF:
its largest accepted legacy record still reaches the legacy decoder. The drain
threshold uses the scanner's full existing buffer, while the v4 decoder retains
its strict byte limit including LF. The pre-split reader passes this regression;
the initial drain threshold failed it and the corrected threshold passes it.
No allocation, decoder, recorder or admission-control limit was increased.
