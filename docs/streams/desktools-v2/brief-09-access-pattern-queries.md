---
brief: assay:assay:desktools-v2:09
title: purpose-built access-pattern query operations (one tuned snapshot, not N per-item calls)
why: >-
  The desk reads the same shapes over and over — the review-queue, head-shas for a set of PRs,
  a board sweep — as N generic per-item calls. That triples latency (N+1), burns the rate-limit
  budget (the recurring secondary-rate-limit blindness class, where a throttled desk reads empty
  and cannot tell empty from blind), and tears freshness: N sequential REST calls can straddle a
  main-advance so half see the old head and half the new. A typed access-pattern operation, each
  backend implementing it as ONE tuned query, collapses each of those into a single consistent
  snapshot — and because the query lives inside the backend, no GitHub-specific document leaks
  past the seam.
wave: 3
depends: ["desktools-v2/02"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 3 (PURPOSE-BUILT QUERIES) — the operation set, the rationale (latency / rate-limit / consistent snapshot), and the two cautions (cost budget, no raw query past the seam)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the N+1 read clusters routed to this brief"
  - "tools/desk/internal/deskkit/forge.go — the interface these typed ops extend (freeze rule: a new method lands with its consuming call site); they live in tools/desk/internal/deskkit, which stays internal (spec §2 Principle 2 — a second module reaches the seam through the deskread verb, never by import)"
  - "the ban-lint (desktools-v2/02) — the control that already forbids a raw query literal outside a backend; this brief adds ops, it does not relax that"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — forge.go already carries per-item ops (ListOpenChanges, ReviewsAtHead, ChecksAtHead, GetPullRequest); the access-pattern ops COMBINE these into one round-trip, they do not replace the interface"
consumers:
  - "tools/desk/internal/deskkit/forge.go + both backends (the new typed access-pattern ops + one tuned query per backend): follow-up desktools-v2/09 (this brief; flips to fixed-here when the ops land)"
  - "the one consumer migrated as proof (e.g. deskboard's board sweep / the head-sha review monitor): follow-up desktools-v2/09 (this brief; the freeze-rule call site that lands with the ops)"
  - "statusgen's scan (a prime later consumer of the snapshot ops): out-of-scope (desktools-v2/08 migrates statusgen's reads; adopting the access-pattern ops there is a later optimization, not this brief)"
exec-tier: strong
exec-tier-why: >-
  question (a) — the operation set and each query's shape are design decisions the facts do not
  pre-specify, and (b) — correctness spans the interface, two backend queries, and a migrated
  consumer, where a snapshot that omits a field or tears under pagination survives a naive test.
domain: complicated
version: 2
id: 264e1b84-3155-4369-8c68-059a35e70645
---

# Brief 09 — purpose-built access-pattern query operations

## Context

files:
- `tools/desk/internal/deskkit/forge.go` — the
  new typed access-pattern operations (e.g. `ReviewQueueSnapshot`, `HeadSHAsForChanges`,
  `BoardSweepRead`) and their typed result structs.
- `forge_github.go`, `forge_gitlab.go` — each backend implements each op as ONE tuned query
  (GitHub GraphQL document / GitLab equivalent), the document PRIVATE to the backend.
- The one consumer migrated as proof (the freeze-rule call site) — e.g. `deskboard`'s board
  sweep or the head-sha review monitor — switched from its N per-item calls to the new op.
- NEW `docs/streams/desktools-v2/query-cost.md` (planned) — the measured before/after call and
  GraphQL-point costs for the migrated consumer (one variable per row).

single-point-of-failure: none of the credential kind — read-only, no identity change. The
design control that matters is that a raw query never crosses the interface; the independent
backing layer is the ban-lint (`desktools-v2/02`), which reddens CI on a `pullRequest`/
`mergeRequest` query literal outside a backend — so even a consumer author cannot smuggle a
GitHub document past the seam. Interface typing and the CI ban catch the leak two different ways.

facts:
- Principle 3 rationale, each measured not assumed: (a) N+1 → one round-trip (latency);
  (b) fewer calls → rate-limit headroom (the secondary-rate-limit blindness class); (c) one
  GraphQL read = one consistent snapshot → no tear across a main-advance (the freshness
  property).
- CAUTION 1 — GitHub GraphQL has a query-cost / point budget. Each op's query is tuned to
  MINIMAL cost, not maximal fetch, and MEASURED: calls and points before/after, ONE variable
  changed at a time. A snapshot op that fetches more than a consumer needs is a regression, not
  a win.
- CAUTION 2 — raw queries never cross the `Forge` interface. Callers pass typed inputs and
  receive typed results; the GraphQL document is a private detail of the backend. This is the
  same rule the ban-lint enforces; this brief adds ops without relaxing it.
- The ops COMBINE existing per-item reads (`ListOpenChanges`, `ReviewsAtHead`, `ChecksAtHead`,
  `GetPullRequest`) into one round-trip; they do not remove the per-item ops (other callers
  still use them).
- Freeze rule: each new method lands with a consuming call site — this brief migrates exactly
  one consumer as proof; broader adoption (statusgen's scan, the full review desk) is later.
- Out of scope: migrating every consumer; changing any credential path; touching statusgen
  (that is `desktools-v2/08`).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add at least one typed access-pattern operation to the shared Forge (`ReviewQueueSnapshot`
   or `HeadSHAsForChanges` or `BoardSweepRead`), with a typed result struct — no raw query in
   the signature.
2. Implement it in each backend as ONE tuned query (GraphQL document / GitLab equivalent),
   the document private to the backend.
3. Migrate ONE existing consumer from its N per-item calls to the new op (the freeze-rule call
   site), keeping its output identical.
4. Measure and record cost: calls and GraphQL points before/after for the migrated consumer,
   one variable changed at a time, in the PR body — proving fewer calls at lower or bounded cost.
5. Prove the snapshot is consistent (single round-trip) with a test that would tear under N
   sequential reads.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/` | exit 0; the new op's backend tests + the migrated consumer's tests pass |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestAccessPatternSingleRoundTrip -v` | output contains the literal line `--- PASS: TestAccessPatternSingleRoundTrip` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) proving the op resolves in ONE round-trip / one consistent snapshot — the dereferencing row for the freshness claim |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoRawQueryInSignature -v` | output contains the literal line `--- PASS: TestForgeNoRawQueryInSignature` (same rule) asserting the access-pattern op signatures carry typed inputs/results only, no raw query string — the "no raw query crosses the seam" row |
| 5 | `sh -c 'for p in "calls before" "calls after" "points before" "points after"; do grep -qiF -- "$p" docs/streams/desktools-v2/query-cost.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo all-present'` | exit 0; prints `all-present` — each of the four measurements is checked SEPARATELY in `docs/streams/desktools-v2/query-cost.md` (planned), so one word repeated cannot stand in for a missing measurement |
| 6 | `D="${D:-05c937307aa6}"; test -n "$D" && git rev-parse -q --verify "$D^1^{commit}" >/dev/null && git rev-parse -q --verify "$D^{commit}" >/dev/null && { n0=; n1=; for r in "$D^1" "$D"; do t=$(mktemp -d) && git archive -o "$t.tar" "$r" && tar -xf "$t.tar" -C "$t" && cp tools/desk/scripts/forge-ban.sh "$t/tools/desk/scripts/forge-ban.sh" && sh "$t/tools/desk/scripts/forge-ban.sh" > "$t.out" 2>&1; n=$(sed -n 's/.*reach-around sites: \([0-9][0-9]*\).*/\1/p' "$t.out"); rm -rf "$t" "$t.tar" "$t.out"; if [ -z "$n" ]; then echo "$r NO-COUNT"; exit 1; fi; echo "$r reach-around sites: $n"; if [ -z "$n0" ]; then n0=$n; else n1=$n; fi; done; if [ "$n1" -le "$n0" ]; then echo "NOT-HIGHER $n0 -> $n1"; else echo "HIGHER $n0 -> $n1"; exit 1; fi; }` — run from a main checkout; `D` defaults to this brief's delivering commit on main, `05c937307aa6` (#1851), and a verifier re-verifying a later delivery sets `D` to that commit instead | exit 0; prints three lines, `<D>^1 reach-around sites: N0`, `<D> reach-around sites: N1`, then `NOT-HIGHER N0 -> N1`, with N1 NOT HIGHER than N0 — adding typed ops introduces no reach-past site (the query documents stay inside the backends). The command decides the verdict itself: it exits 1 on `HIGHER`, on a tree that prints no count, and on a `D` that does not resolve to a commit with a parent. Both trees are measured with the SAME current script, and the reference is the count at the delivering commit's own merge parent, not the `desktools-v2/02` line in `docs/streams/desktools-v2/forge-ban-baseline.txt`, so sites other PRs add or remove before or after cannot move the verdict (#1529; the line went stale twice, 53 then 60, before this row stopped reading it). Evidence cites `D`, the parent sha, N0 and N1 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Non-implementer verifier run — VERIFY: FAIL — 5/6 rows, row 6 (#1529) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `43420f7ecd743f5c930dc479f54f5ef5ca7b82ed` (HEAD == the forge's `commits/main`). Implementing commit 05c937307 (#1851). `gate: model`, all risk answers `no`. Offline, `KUBECONFIG=/dev/null`; no check:ci row. Status stays `implemented`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | PASS — exit 0, no output | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/` | ok, consumer tests pass | PASS — `ok .../internal/deskkit 70.157s`; the migrated consumer's package (`./cmd/deskboard/`, not in the row's command) also run: exit 0, `ok 59.089s` | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestAccessPatternSingleRoundTrip -v` | PASS | PASS — `--- PASS: TestAccessPatternSingleRoundTrip`, 3 subtests incl. the tear control | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoRawQueryInSignature -v` | PASS | PASS — `--- PASS: TestForgeNoRawQueryInSignature` | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | row 5 command verbatim (the `sh -c` loop over the four phrases in `docs/streams/desktools-v2/query-cost.md`) | all-present | PASS — `all-present` | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | row 6 command (forge-ban.sh output to a temp file, then `grep -oE 'reach-around sites: [0-9]+'`) | count NOT HIGHER than the `desktools-v2/02` baseline line | **FAIL** — `reach-around sites: 61` (desk 29, statusgen 32) against baseline `desktools-v2/02 53` | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE: DERIVED — forgeQueueReviewsCap = 100 @ tools/desk/internal/deskkit/forge_github.go:694 — equals the connection's `first:`, so the `hasNextPage` check and the length check agree; 100 is the GraphQL maximum; overflow falls back to `ReviewsAtHead`.
RISK-VALUE: DERIVED — reviews(first:100) @ tools/desk/internal/deskkit/forge_github.go:683 — the GraphQL maximum; `hasNextPage` is selected, so overflow is detected rather than read as complete.
RISK-VALUE: DERIVED — GitLab ReviewsComplete: false @ tools/desk/internal/deskkit/forge_gitlab.go:876 — the fail-closed direction under the `QueuedChange` contract; the consumer gates on it.
RISK-VALUE: DERIVED — query points 3 → 4 @ docs/streams/desktools-v2/query-cost.md:38 — recomputed from the query documents with the published formula (301 requests → 3 points, 401 → 4); computed, not read back live.

Findings: (F1, the FAIL) the item adds no reach-around site: `forge-ban.sh` on archives of 05c937307^ and 05c937307 both print 61. The +8 over the committed 53 predates this brief (brief-02's own Evidence recorded 61 on 2026-09-27). The row cannot pass until the baseline is refreshed or the row compares against the count at the merge's parent — the open question on #1529. (F2) Task 2 asks for one tuned query per backend; GitLab is explicitly degraded (per-item reads, every change reported incomplete, fail-closed) at forge_gitlab.go:857-879. No Verify row covers it. (F3) Query points are computed, not measured; `query-cost.md` itself calls for a live cost read. (F4) Row 2's command covers only `./internal/deskkit/` while its Expect includes the consumer's tests.
### Non-implementer verifier re-run — VERIFY: PASS — 6/6 rows — 2026-10-01 assay-verifier-app[bot]

What moved since the last run: #1870 (54dc9e703) refreshed the `desktools-v2/02` forge-ban baseline line to 63 and rewrote row 6 so that it no longer reads that line. It now compares the forge-ban count at the delivering commit `D` = 05c937307aa65274ae615d650d9d3c8db61b9810 (#1851) with the count at its merge parent 61d700db1712e4a2512904efdd8659a29e87ad40, using the same current script for both. That clears the previous run's only FAIL (F1). Rows 1-5 are unchanged. Two later commits (#1914, #1822) touched the deskkit backend files and deskboard, so rows 1-4 were re-run on the current head.

The runner is not the implementer. I ran from a detached, isolated worktree at merged main `b5e53a6a2dbc18763b3dfa45b8623c2b5e524389` (the fetched `refs/remotes/origin/main`). Everything ran offline with `KUBECONFIG=/dev/null`. go1.27.1 darwin/arm64. `gate: model`, and all four risk answers are `no`. Rows 1-5 were run from the checkout root with the `cd` shown. Row 6 was run verbatim from the checkout root, as its own note requires.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | Verify row 1: exit 0, no output | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -timeout 10m ./internal/deskkit/` | exit 0; the new op's backend tests and the migrated consumer's tests pass | Verify row 2: exit 0, `ok github.com/medici-finance/assay/tools/desk/internal/deskkit 90.387s`. Also ran for this row's Expect clause (the consumer package is outside the row's command): `cd tools/desk && go test -timeout 10m ./cmd/deskboard/` gave exit 0, `ok github.com/medici-finance/assay/tools/desk/cmd/deskboard 23.226s` | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestAccessPatternSingleRoundTrip -v` | output contains the literal `--- PASS:` line for the named test | Verify row 3: exit 0. The `--- PASS:` line for the selected test is present, with 3 subtests passing: snapshot is one round trip and consistent; control sequential per-item reads tear; snapshot reviews equal per-item reviews at one instant | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeNoRawQueryInSignature -v` | output contains the literal `--- PASS:` line for the named test | Verify row 4: exit 0. The `--- PASS:` line for the selected test is present (0.00s), then `PASS` and `ok` | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `sh -c 'for p in "calls before" "calls after" "points before" "points after"; do grep -qiF -- "$p" docs/streams/desktools-v2/query-cost.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo all-present'` | exit 0; prints `all-present` | Verify row 5: exit 0, `all-present` | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `D="${D:-05c937307aa6}"; test -n "$D" && git rev-parse -q --verify "$D^1^{commit}" >/dev/null && git rev-parse -q --verify "$D^{commit}" >/dev/null && { n0=; n1=; for r in "$D^1" "$D"; do t=$(mktemp -d) && git archive -o "$t.tar" "$r" && tar -xf "$t.tar" -C "$t" && cp tools/desk/scripts/forge-ban.sh "$t/tools/desk/scripts/forge-ban.sh" && sh "$t/tools/desk/scripts/forge-ban.sh" > "$t.out" 2>&1; n=$(sed -n 's/.*reach-around sites: \([0-9][0-9]*\).*/\1/p' "$t.out"); rm -rf "$t" "$t.tar" "$t.out"; if [ -z "$n" ]; then echo "$r NO-COUNT"; exit 1; fi; echo "$r reach-around sites: $n"; if [ -z "$n0" ]; then n0=$n; else n1=$n; fi; done; if [ "$n1" -le "$n0" ]; then echo "NOT-HIGHER $n0 -> $n1"; else echo "HIGHER $n0 -> $n1"; exit 1; fi; }` | exit 0; three lines ending `NOT-HIGHER N0 -> N1` | Verify row 6: exit 0, with `D` left at its default. Output: `05c937307aa6^1 reach-around sites: 61`, then `05c937307aa6 reach-around sites: 61`, then `NOT-HIGHER 61 -> 61`. `D` = 05c937307aa65274ae615d650d9d3c8db61b9810 (squash, one parent). Parent = 61d700db1712e4a2512904efdd8659a29e87ad40. N0 = 61, N1 = 61 | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Scope traceability.** Each row above discharges the Verify row with the same number. The extra `./cmd/deskboard/` run belongs to row 2's Expect clause (the migrated consumer's tests), not to any new scope.

**Risk-bearing value.** The fail-safe trigger does not fire: risk metadata is present, all answers are `no`, the item is not irreversible, and the diff touches no risk-classed path. The enumeration was done anyway. It covered the delivering diff (05c937307) in the non-comment code of forge.go, forge_github.go, forge_gitlab.go and the deskboard board.go, plus the Deliverables' cost record:

- `forgeQueueReviewsCap = 100` @ tools/desk/internal/deskkit/forge_github.go:699 (introduced)
- `reviews(first:100)` in `ghReviewQueueReviewsSel` @ tools/desk/internal/deskkit/forge_github.go:688 (introduced)
- `ReviewsComplete: false` for every GitLab change @ tools/desk/internal/deskkit/forge_gitlab.go:876 (introduced)
- review-queue cost `points before 3, points after 4` @ docs/streams/desktools-v2/query-cost.md:38 (introduced)
- Not introduced, so out of scope: `labels(first:100)` and `contexts(first:100)`, inherited unchanged from the pre-existing open-changes document at forge_github.go:669, and `prListLimit = 100` @ tools/desk/cmd/deskboard/board.go:80, where the diff changed only a message that mentions it.

Ranked by irreversibility, none of these values is irreversible. All are read-side bounds that an edit and a redeploy can undo. The first three rank highest because a wrong value could make the snapshot read as complete when it is not.

RISK-VALUE: DERIVED — forgeQueueReviewsCap = 100 @ tools/desk/internal/deskkit/forge_github.go:699 — 100 is GitHub GraphQL's maximum `first:` for a connection, and it equals the selection's own `first:100`. The completeness guard at forge_github.go:878 treats reviews as complete only when `hasNextPage` is false AND the node count is 100 or fewer, so an overflow is caught and never read as a complete set.
RISK-VALUE: DERIVED — reviews(first:100) @ tools/desk/internal/deskkit/forge_github.go:688 — this is the GraphQL maximum, and `pageInfo{hasNextPage}` is selected alongside it, so a truncated review list is visible to the guard rather than silent.
RISK-VALUE: DERIVED — ReviewsComplete = false @ tools/desk/internal/deskkit/forge_gitlab.go:876 — this is the fail-closed direction. The GitLab backend has no tuned snapshot, so it marks every change incomplete, and the consumer falls back to per-item reads instead of trusting an empty review set.
RISK-VALUE: DERIVED — points 3 -> 4 @ docs/streams/desktools-v2/query-cost.md:38 — recomputed here from the query documents with GitHub's published cost formula (sum of connection requests divided by 100, rounded). Open-changes: 1 (pullRequests) + 100 (labels) + 100 (commits last:1) + 100 (contexts) = 301, which gives 3. Review-queue adds 100 (reviews) = 401, which gives 4. This is computed, not read from a live rateLimit field.

Findings (carried forward and re-checked on this head; none of them fails a Verify row):
- (F2) Task 2 asks for one tuned query per backend. The GitLab backend is still an explicit degraded path at forge_gitlab.go:870-879: per-item reads through `ListOpenChanges`, with every change reported incomplete. No Verify row covers the GitLab half of Task 2. This is verified work that maps to no row.
- (F3) The query points in `docs/streams/desktools-v2/query-cost.md` are computed, not live-measured, and the file says so itself. No row requires a live measurement.
- (F4) Row 2's command covers only `./internal/deskkit/`, but its Expect clause also names the consumer's tests. The consumer package was run separately above and passed.

rows_passed=6 rows_total=6
RISK-VALUE: DERIVED — forgeQueueReviewsCap = 100 @ tools/desk/internal/deskkit/forge_github.go:699 (plus the three other DERIVED lines above)
VERIFY: PASS

## Review
Gate: model (all four risk answers no — read-only typed operations added behind the seam; no
identity binding, no capability removed; the query documents stay inside the backends, which
the ban-lint independently enforces). Row 3 dereferences the single-snapshot freshness claim,
row 4 the "no raw query in the signature" contract, row 5 the measured cost (calls+points, not
a memory assertion), row 6 the ban-lint neutrality. Reviewer records verdict + date in the
stream README table.
