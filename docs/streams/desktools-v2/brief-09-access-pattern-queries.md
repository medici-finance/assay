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
version: 1
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
| 6 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb9.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb9.txt` | exit 0; count NOT HIGHER than the `desktools-v2/02` line in `docs/streams/desktools-v2/forge-ban-baseline.txt` — adding typed ops introduces no reach-past site (the query documents stay inside the backends) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — read-only typed operations added behind the seam; no
identity binding, no capability removed; the query documents stay inside the backends, which
the ban-lint independently enforces). Row 3 dereferences the single-snapshot freshness claim,
row 4 the "no raw query in the signature" contract, row 5 the measured cost (calls+points, not
a memory assertion), row 6 the ban-lint neutrality. Reviewer records verdict + date in the
stream README table.
