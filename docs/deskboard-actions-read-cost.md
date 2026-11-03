# Actions sweep visibility read cost

`deskboard actions` now obtains repository visibility from the same live typed review-queue snapshot it already reads for every configured repository. That removes one duplicate repository-metadata HTTP request per supported repository. The policy-drift alarm still compares the actual observed visibility against configured policy; the scalar is consumed only within the current invocation.

The shared `ReviewQueue.Visibility` value is optional. The GitHub backend maps only the known public, private and internal enum values. A missing, empty or future value retains the explicit live `RepoVisibility` probe. GitLab retains that same fallback. A failed fallback still reports `NOT OBSERVED`, rather than a guessed clean result. A partial GraphQL response remains unusable even when it includes visibility.

No policy observation is added to the per-PR or HTTP cache. Existing author/reviewer trust, current-head and changed-file validation, complete pagination, branch-health checks, read concurrency, rate floors and classification/output ordering remain in place.

## Matched local fixture

The complete actions fixture covers ten configured private repositories with two approved open changes each, using the real typed GitHub HTTP implementation against a local stand-in. Every invocation starts in a fresh fixture home, runs a cache-cold sweep, then an unchanged second sweep. Five adjacent baseline/candidate pairs alternate order. Both test binaries use Go 1.27.2 and the same build flags and fixture source. The baseline is immutable commit `1304d40f5c32cae5136e87f6b1f0cbe5a008c72a`; only the test fixture is overlaid onto it.

| Fixture state | Baseline requests | Candidate requests | Baseline non-304 responses | Candidate non-304 responses |
|---|---:|---:|---:|---:|
| Cold | 90 | 80 | 90 | 80 |
| Unchanged second sweep | 50 | 40 | 10 | 10 |

Every pair produced the same classification and output semantics and the same exit result. Comparison excludes the clock presentation fields (`asOf`, `auditFirstTS`, row `openAge` and `approvedAge`) and the cost report, whose actual request counts are compared separately. Full raw output is retained with the private measurement evidence.

| Local actions elapsed | Baseline p50 / p95 | Candidate p50 / p95 |
|---|---:|---:|
| Cold | 102.9 / 249.9 ms | 118.2 / 449.7 ms |
| Unchanged | 14.0 / 16.7 ms | 12.5 / 46.2 ms |

The external fixture-process median, including both sweeps and test setup, was 151.6 ms before and 171.7 ms after. Its measured reaped process CPU median was 170.2 ms before and 154.3 ms after. The host and filesystem cache were uncontrolled; with five samples, p95 is the maximum. These mixed local timings establish no overall elapsed-time speedup or production savings.

HTTP attempts are request counts, not GitHub quota points or money. In particular, the unchanged fixture removes conditional 304 round trips while leaving its non-304 count unchanged. The result is ten fewer actual requests for this ten-repository fixture, with no measured point-cost claim.

Run the current fixture with `go test ./cmd/deskboard -run '^TestActionsCostFixture$' -count=1 -v` from the desk module. It contacts only its local HTTP stand-in.

## Consumers and controls

The GitHub snapshot producer supplies the new scalar; GitLab continues to report it unavailable. The outbound/recording and read-only forge wrappers return the same typed result without replacing it. The actions consumer uses the scalar for its existing policy-drift alarm. The separate review-observation consumer shares the queue fetch but does not add visibility to its observation format or authorization rules. Standalone `policydrift` continues its explicit live probe.

The fail-first actions controls cover duplicate metadata reads and a changing live visibility behind unchanged PR facts. Additional controls retain unavailable-value fallback, failed-fallback visibility, all three enum mappings, and rejection of partial GraphQL results. The existing exact-query test retains every scope/rollup/page-bound assertion and explicitly pins the additional repository scalar; its historical query-cost record remains historical. The canonical four-mutation harness has a healthy baseline, a distinct caught control, four caught mutations, zero survivors and zero could-not-mutate results, with exact source restoration. It is a local harness, not a new CI-matrix enforcement claim.

The full shared-suite aggregate and exact unchanged baseline control are reported separately in the PR; no deadline, scanner or check is changed to produce a passing result.
