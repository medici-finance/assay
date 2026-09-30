# Access-pattern query cost — `ReviewQueueSnapshot` (brief 09)

The record Task 4 of [brief 09](brief-09-access-pattern-queries.md) asks for: calls and
GraphQL points before and after, for the one consumer migrated onto the new typed op, changed
**one variable at a time**.

**Consumer.** `deskboard`'s actions sweep (`sweepActionsRepo`), per watched repo. Before, it
read the open-PR list (`ListOpenChanges`, one GraphQL POST) and then each PR's reviews
(`ReviewsAtHead`, one REST `GET /pulls/{n}/reviews` per PR, more pages only past 100 reviews).
After, it reads both in one `ReviewQueueSnapshot` (one GraphQL POST). `N` below is the number
of open PRs in the repo, capped at 100 by both documents.

## How each number was obtained

- **Calls are measured.** Offline fakes count every request the code actually makes:
  `TestAccessPatternSingleRoundTrip` (deskkit) counts HTTP requests to a GitHub fake for the
  snapshot and for the list-then-per-item reads it replaces; `TestSweepReviewQueueSnapshotMatchesPerItem`
  (deskboard) counts per-item review reads the sweep makes on each path. Both run in CI.
- **Points are computed, not live-measured.** This change was built offline, with no call to
  a live forge, so no `rateLimit { cost }` was read back from the API. The numbers apply
  GitHub's published cost formula to the two query documents. The formula: add up the
  requests needed to fill each connection, assuming every `first`/`last` limit is reached,
  then divide by 100 and round. `TestReviewQueueQueryIsOpenChangesPlusReviews` pins that the
  two documents differ by the reviews selection ONLY, so the point difference is that
  connection and nothing else. A live read of `rateLimit { cost }` on each document is the
  confirming measurement.
- **Points only count GraphQL.** REST calls (the per-item review reads) draw on the separate
  REST request budget, not on GraphQL points. So each row lists both budgets.

## Rows — one variable changed per row

| # | What changed from the row above | calls before | calls after | points before | points after | REST requests (per sweep, per repo) |
|---|---------------------------------|--------------|-------------|---------------|--------------|-------------------------------------|
| 1 | baseline — list + per-item reviews | — | 1 + N | — | 3 | N |
| 2 | **query document only**: the list document gains `reviews(first:100)`; the consumer still reads per item | 1 + N | 1 + N | 3 | 4 | N |
| 3 | **consumer only**: the sweep uses the snapshot's reviews and drops the per-item reads | 1 + N | 1 | 4 | 4 | 0 |

Net, row 1 → row 3: **calls before 1 + N, calls after 1; points before 3, points after 4**
(GraphQL); REST requests N → 0.

### The point arithmetic

| Connection (per document, limits assumed reached) | requests |
|----------------------------------------------------|----------|
| `pullRequests(first:100)` | 1 |
| `labels(first:100)` — one per PR | 100 |
| `commits(last:1)` — one per PR | 100 |
| `contexts(first:100)` — one per commit (100 PRs × 1 commit) | 100 |
| **open-changes document** | **301 → 3 points** |
| `reviews(first:100)` — one per PR (snapshot only) | 100 |
| **review-queue document** | **401 → 4 points** |

Lowering the review page size would not lower this. The formula counts requests per
connection, not items per page, so `reviews(first:50)` still adds 100 requests.
`first:100` also matches the REST page size the per-item read walks, so neither path splits a
review set differently.

## Reading the trade (Caution 1)

- **Fewer calls, bounded cost.** Each sweep now makes 1 call per repo, not 1 + N. The GraphQL
  cost is a flat 4 points whatever N is (the documents cap N at 100). The one extra point
  replaces N REST requests. With 20 open PRs, that is 21 calls → 1 call, and 20 REST
  requests → 0, for +1 GraphQL point.
- **No over-fetch.** The snapshot asks for exactly the fields `ReviewsAtHead` maps: review
  id, author login/kind/id, state, commit oid, body and submitted time. The consumer's own
  row mapping consumes all of them except review and author ids, which the forge-neutral
  `Review` type carries for its other consumers.
- **Fallback.** A PR with more than 100 reviews comes back marked incomplete, and the sweep
  reads it per item exactly as before: +1 call (plus pages) for that PR only.
- **GitLab is unchanged.** The GitLab backend reports every change incomplete, so a GitLab
  repo's sweep makes the same per-item reads it made before (see the op's inventory row). This
  record measures the GitHub path only.
- **Consistency.** The snapshot is one read, so each PR's head and the reviews reduced against
  it come from the same instant. `TestAccessPatternSingleRoundTrip`'s control shows the
  list-then-per-item reads tearing against a forge that moves between requests.
