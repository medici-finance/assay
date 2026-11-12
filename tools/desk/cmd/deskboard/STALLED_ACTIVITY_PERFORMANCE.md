# Stalled-draft activity request reduction

Issue: [#6261](https://github.com/medici-finance/assay/issues/6261).
Baseline source: `0665bfb771788b07473c281b8875fc1806dab36d`.

## Existing work and selected repair

Issueboard already has bounded positive-closure and required escalation-comment
batching, and Full/work-ready/intake/inbox projections. The current full-board
regressions prove 41 required comment queries become 9 with identical output.
These preserve the work in #4360, #4622 and desk-tools/42's acceptance (#5179).
The separate perf-v4 measurement remains #5278. Its remaining per-issue REST
label-history reads are required inputs; #4622 explicitly excludes replacing them
with a GraphQL timeline whose deleted-label and completeness semantics have not
been qualified. This repair changes neither issueboard nor its selectors.

Stalled already reuses complete shared reviews (#5021). It still reads comments
once per blocked draft. This repair batches up to five existing comment windows
through the existing wrapped provider. It reads only Author and CreatedAt, using
the same first-100 window and actor normalization as ListComments. The original
author-clock reduction stays in fetchLastAuthorComment, without changes to its
authorship-site guard. The existing limitation beyond comment 100 is preserved,
not represented as a complete-history improvement.

Only open drafts with complete blocking-at-current-head review observations are
eligible for read-ahead. Incomplete reviews, opaque decorators and unsupported
providers retain the original guarded path. Invalid/missing alias identity or
comment data cannot become evidence of no activity. A located GraphQL failure
makes that change unassessable; transport or unlocated errors make the affected
batch unassessable. Neither path retries under another operation or credential.
Head attribution, behind-main comparison, enumeration, scope, caps and concurrency
limits remain. Results belong to one repository and one invocation.

This is a bounded adapter improvement along the shared-observation direction in
`docs/desk-workflow-efficiency-spec.md`. The standing selection work remains with
graph-execution/27; forge-neutral/37–43 retains mirror/source serving ownership.
There is no persistent cache, new service, caller cutover, release or repin.

## Matched complete-command fixture

The same TestStalledActivityCost runs against baseline and candidate. It counts
actual HTTP attempts through the production command and GitHub adapter. All nine
normalized complete reports match; only asOf, auditFirstTS and daysStalled are
normalized. The author-replied phase adds recent replies at the same PR head.

| Blocked drafts | Cold/repeated baseline → candidate | Author replied baseline → candidate | Comment requests baseline → candidate |
|---:|---:|---:|---:|
| 5 | 16 → 12 | 15 → 11 | 5 → 1 |
| 20 | 61 → 45 | 57 → 41 | 20 → 4 |
| 60 | 181 → 133 | 169 → 121 | 60 → 12 |

This is 25–28% fewer complete-sweep attempts in these fixtures, including every
head and applicable behind-main read. It is not a live wall-time, quota-point or
fleet savings claim. The complete fixture fails its request-count assertions on
baseline; the original mixed snapshot/failure fixture is retained and now also
rejects activity reads for ineligible changes.

## Validation and limits

- Complete deskboard and issueboard suites: PASS.
- Focused race tests for stalled, the new provider, author-skip guard and existing
  issueboard batching/selection cases: PASS.
- Vet for deskboard, issueboard and deskkit: PASS.
- New mutation spec: healthy baseline/control, 4 caught, no survivors or mutation
  failures. The existing stalled snapshot spec likewise catches all 4 mutants.
- Provider tests cover repository confinement, opaque decorator guards, duplicate/
  invalid identities, alias mismatch, null/missing data, partial/root errors,
  transport/body failures, bot/deleted authors and the 100-comment bound.
- The full deskkit suite encounters TestFlowWorldBind and TestHotfixSelfGate
  failures reproduced on unchanged baseline. Existing #5138 and #5225 track the
  temporary-path environment issue. These are not claimed as passing here.

The local security pass traces the unchanged custody/transport, owned decorator
capability, scope and alias identity checks. It found no new exploitable path in
this bounded read adapter. This author pass does not replace independent PR review.
