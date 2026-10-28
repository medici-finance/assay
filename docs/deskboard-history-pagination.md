# Bounded reviewed-head observations

Deskboard's `RE-REVIEW` fallback remains authoritative whenever the reviewed-head
interval cannot be established completely. The shared GitHub adapter can now read
more than the former single-response commit/file windows without adding a second
review classifier or observation store.

## Placement and bounds

`GitHubForge.CompareRefs` collects up to ten numeric pages of 100 commits. A stable,
nonnegative declared total, unique commit identities, matching status/divergence
metadata and terminal pagination evidence are required for `CommitsComplete`.
A declared total above 1000 fails incomplete after the first response: reading
more pages cannot prove completeness within this budget. Further pages require both requested endpoints to be immutable commit identifiers;
shared callers passing moving branch names retain a conservative incomplete result.
Only the first page's aggregate file window is projected into `RefComparison.Files`:
that existing field is not newly claimed to be complete.

`GitHubForge.GetCommit` collects up to thirty numeric pages of 100 files. A branch
argument is pinned to the first response's immutable SHA before further pages. SHA
and parent ordering must remain stable, and filenames must be present and unique.
Rename sources are preserved. Missing file arrays, repeated data, transport/JSON
errors and continuation without progress never become a complete observation.
An exactly full final file page without a next link requires another bounded read;
GitHub's 3000-file ceiling therefore remains incomplete even at an exact boundary.

Every page uses the existing authenticated client, request observer and deskboard's
conditional transport/quota floor. Link headers signal continuation only: provider
URLs are never followed. No permission, identity, quota-floor, decision, approval,
trust, admission, breaker, audit or telemetry-consent setting changes.

Deskboard still walks only the first-parent chain and checks each commit's files.
Edits later reverted and merge conflict resolutions remain visible; unrelated
second-parent history is not fetched as commit detail. Its existing per-PR fact
store stores only complete intervals and retains its existing invalidation key.
No new disk cache, TTL, service or global mutable observation owner is introduced.
The future shared review view can consume these existing typed completeness values;
collection remains in the credentialed adapter and action derivation remains pure.

The limits are per operation, not a promise of a small whole-sweep request budget:
a long first-parent chain can still need many commit reads, and the existing sweep
quota floor can refuse the board before that work finishes. Such a refusal stays a
refusal. The incomplete-data fallback is not waived to obtain a cheaper result.

## Failure shape: what a caller gets back

Both methods are shared with callers that read only metadata, so a failure is split
by WHERE it happens:

| Failure | `GetCommit` | `CompareRefs` |
|---|---|---|
| Page-1 transport, HTTP or JSON error | error | error |
| Page-1 SHA differs from a requested immutable SHA | `Unverifiable` error | n/a |
| Later page changes the commit SHA/parents, or the compare total/status/ahead/behind | `Unverifiable` error | `Unverifiable` error |
| Later-page transport, HTTP (including 5xx) or JSON error | page-1 metadata, `FilesComplete=false` | page-1 status, `CommitsComplete=false` |
| Repeated or empty identity, continuation without progress, more commits than declared, oversized page, exhausted budget | page-1 metadata, `FilesComplete=false` | page-1 status, `CommitsComplete=false` |

Only an identity or metadata change between pages, where the metadata itself is in
doubt, and a page-1 failure, where there is no metadata yet, return an error. Every
other uncertainty keeps what page 1 returned and drops only the completeness flag.
The completeness consumers (deskboard `changedFilesBetween`, deskdispatch review
preparation, the deskevidence decision gate) already treat an incomplete result and an
error identically, as could-not-check, so the split changes no decision they make.

## Per-caller request cost

Before this change every `GetCommit` and `CompareRefs` call was one request. Now:

| Call | Requests per call |
|---|---|
| `GetCommit` on a commit touching n files | floor(n/100)+1, at most 30 (1 for n < 100; 2 for 100–199; 3 for 200–299) |
| `CompareRefs` with two immutable SHAs, declared total t ≤ 1000 | max(1, ceil(t/100)), at most 10 |
| `CompareRefs` with a declared total above 1000, or with a branch-name endpoint | 1 |

The metadata-only `GetCommit` callers pay the file pages even though they never read
the file list: deskpost (head author and head committed-at), deskflip reanchor,
deskautolane, deskevidence (commit reads, landed check, decision gate) and deskboard's
stalled sweep (one head-commit read per open PR). For them the added cost is
floor(n/100) requests per call, so a 250-file keep-current merge head costs 3 requests
where it cost 1. The same applies to `CompareRefs` callers that read only `Status`
with immutable endpoints, such as the deskevidence outcome binding: up to 9 extra
requests for an interval of up to 1000 commits. A later-page failure no longer costs
any of these callers the metadata it already had (see the table above). This is a
stated cost, not a measured one: the measurement below covers only the deskboard
interval fixture.

## Consumer routing

- Fixed here: shared GitHub `GetCommit` and `CompareRefs` pagination and evidence.
- Preserved callers: deskboard, deskdispatch review preparation, deskevidence,
  deskpost, deskflip and deskautolane use the same Forge methods. A page-1 failure
  and an identity or metadata change between pages are ordinary errors; every other
  uncertainty returns the page-1 metadata with the completeness field false (see
  §Failure shape). Callers that require file/history evidence must honor the
  completeness fields.
- Unchanged: GitLab adapter, aggregate compare-file semantics, review queue snapshot,
  actor/exact-head policies, local git readers and decision-admission sweep reuse.
- Existing perf-v4 codec/readers and producer migration belong to desktools-v2/67–68;
  this patch adds no recorder, schema, emitter migration or remote collection.

## Qualification and measurement

Native adapter regressions cover complete multi-page reads and exact boundaries;
short/full pages with and without a continuation signal; stable counts and identities;
renames, duplicate/missing entries, changing refs, malformed/cut responses, later-page
errors and bounded exhaustion. Native deskboard regressions exercise first-parent
classification and complete-only interval reuse, including retry after partial data.
The dedicated mutation spec qualifies with a green baseline, caught positive control,
18 caught faults, zero survivors and zero unmutatable faults (14 at the first revision;
the later-page metadata and oversized-page guards added four). The spec is run by hand
with `go run ./cmd/muhar -spec internal/deskkit/commithistory-mutations.json` from
`tools/desk`; it is not yet in a CI mutation matrix. All seven shared adapter
and consumer suites pass (deskkit 462.946s, deskboard 41.808s, deskdispatch 169.802s,
deskevidence 173.221s, deskpost 19.844s, deskflip 2.239s, deskautolane 1.078s). Focused race,
vet, source lint, mutation-anchor and committed test-ledger checks pass. No original
test or control is retired. The final fail-fast behavior and all consumer suites were run at the measured
source below. Earlier measurements, binaries and qualification remain preserved
as the initial revision; no prior evidence is relabeled as a repeated run.

Matched measurement uses the actual native adapter, first-parent reader and shared
`PerfRunMeasured` writer/reader with only remote HTTP replaced by a fixture. Fixture
requests have an injected 8 ms delay. Each variant is a fresh process; small and large
cohorts are measured separately. A complete read and an incomplete fallback are not
equivalent work, so extra large-cohort requests are reported as a completeness cost,
not an invocation-speed improvement or an estimate of reviewer/model savings.

**perf-v4 qualification pending:** baseline main 917772b3 contains the v4 design and
migration briefs but no v4 codec or producer. Actual records must retain their real
`perf-v3` schema; relabeling those records or creating a second v4 writer would not
satisfy the requested measurement. Re-run this fixture with the canonical v4 producer
once that separately owned migration lands. No v4, production or rollout claim is made.

Matched runs (five fresh processes per variant/cohort, alternating order):

| Interval fixture | HTTP before → after | Median wall ms | Observed nearest-rank p95 wall ms | Median own CPU ms | Evidence |
|---|---:|---:|---:|---:|---|
| Small cold: 1 commit, 2 files | 2 → 2 | 87 → 71 | 394 → 146 | 15 → 15 | complete → complete |
| Large cold: 251 commits, 301 files | 1 → 7 | 44 → 139 | 277 → 219 | 18 → 21 | incomplete → complete |
| Eight unchanged large-interval sweeps | 8 → 7 | 459 → 140 | 633 → 428 | 39 → 27 | incomplete → complete |
| Declared over budget: 1001 commits | 1 → 1 | 70 → 63 | 143 → 90 | 16 → 14 | incomplete → incomplete |

The repeat cohort replays one unchanged externally observed PR key through actual
fact-store save/load. All five request-count repetitions agree. It saves one interval
request across eight such sweeps (12.5%) and recovers complete evidence on all eight;
repo snapshots, checks/reviews and consequential rechecks remain outside this interval
fixture. There is no whole-board, reviewer-cost, fleet, rate-limit-point or production
speedup claim. The large cold cohort does additional work, not equivalent cheaper work.
Host scheduling was noisy (small-before wall samples span 33–394 ms);
these five-sample quantiles describe only the observations, not a latency guarantee.
CPU values and unsupported/unknown point coverage are retained without a general savings claim.

Baseline production: `917772b3ee4709bc47e5d0888cb5cc698444b13e`.
After source and identical fixture: `f7224bd1a1b66493b27dc6fab2cb3764828787ef`.
Before binary SHA256: `cac3ac37de2b33704be3d7e9ab0ad2461e9ca0865a5ebba224874217439a8621`.
After binary SHA256: `edbb9fc33c24abdb50c5c61ab6b93da1db766c9663645934d6c5a964346adf68`.
Each record's native HTTP attempts matched a separate fixture-server witness. No
historical record was relabeled and no v4 or repeated mutation/performance run is invented.

API contract: [GitHub commit and comparison endpoints](https://docs.github.com/en/rest/commits/commits).
