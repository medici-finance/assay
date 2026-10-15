---
brief: assay:assay:derived-board:09
title: incremental reconcile — forge reads scale with open PRs plus PRs changed since the last regen, never with total PR history
why: >-
  Forge requests are what is rate limited, and today every board regeneration spends them in
  proportion to the repository's whole PR history — on a measured consumer board, ~280
  requests to learn about 17 open PRs, growing with every PR ever merged, on every push.
  Reading only the open PRs and the PRs that changed since the last regeneration, and taking
  everything older from the witness ledger (brief 08), turns a cost that only grows into one
  that tracks the work actually in flight — single-digit requests on a quiet board.
wave: 3
depends: ["derived-board/08"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1969]
outcome: none
schema: brief-v2
version: 1
authored: 2026-10-15 by a reconcile performance investigation session
sources:
  - "reconcile --backfill profile, 2026-10-14/15, on a consumer board (1,936 PRs: 17 open): 301 forge requests per run, ~250 of them file-list reads of long-merged PRs"
  - "#1969 (the 20-page pull-list cap returns a truncated list as complete)"
  - "derived-board/08 (the ledger this brief reads)"
  - freshness-checked 2026-10-15 @ e7c208b00 (origin/main)
exec-tier: strong
exec-tier-why: "(b) the incremental read must derive exactly what the full read derives, across reconcile and the regen drift comparator; a subtle watermark error survives happy-path tests"
domain: complicated
consumers:
  - "statusgen/reconcile.go: follow-up derived-board/09 (this brief)"
  - "statusgen/regen.go (drift comparator): follow-up derived-board/09 (this brief)"
  - "statusgen/claimdecay.go: out-of-scope (claim decay reads branch-scoped pull-request state on its own, one query per 100 live branches, landed separately; it never reads the ledger)"
  - ".github/workflows (this repository's statusgen workflow, the scheduled --verify-ledger step): follow-up derived-board/09 (this brief; staged as a diff, pushed by the driver because App tokens lack workflow scope)"
  - "adopter regen workflows (pass --ledger, commit the ledger, add the scheduled --verify-ledger run): out-of-scope (named in the release note of the version that carries this brief; applied per repo through upgrade-assay)"
---

# Brief 09 — incremental reconcile

## Context
files:
- `statusgen/ghfetch.go` — `fetchOpenPulls`, `fetchPullsUpdatedSince(watermark)`; the full walk pages to the end or fails (no silent cap, #1969)
- `statusgen/reconcile.go` — `--ledger <path>` (incremental mode), `--verify-ledger`, the `forgeCost` JSON field
- `statusgen/regen.go` — the drift comparator reads PRs the same way
- `statusgen/witnessledger.go` (planned) — merge delta records into the ledger
- `statusgen/reconcileincremental_test.go` (planned)
- `changelog/derived-board-09-incremental-reconcile.md` (planned)

facts:
- The ledger (brief 08) holds every closed PR the fold could read as a witness, verified against local git on read, with a `watermark` = the largest `updated_at` the recording run saw.
- `GET /repos/{o}/{r}/pulls?state=open` costs one request per 100 open PRs. `GET /repos/{o}/{r}/pulls?state=all&sort=updated&direction=desc` returns PRs newest-updated first, so a walk can stop at the first PR whose `updated_at` is older than the watermark — one request per 100 PRs changed since the last run. A body edit, merge, close or reopen all bump `updated_at`.
- File lists are needed only for PRs the fold classifies (open PRs in scope, and delta PRs); ledger records already carry their authoring facts.
- Measured target (2026-10-15, board of 1,936 PRs, 17 open): incremental run ≈ 1 open-list request + 1 delta request + file lists for the in-scope open/delta PRs + the open-issue pages — against 301 today.

single-point-of-failure: the watermark, together with the ledger's forge-only fields — a PR changed before the watermark but missing from the ledger, or a forge-only field that is wrong, would not be re-read by an incremental run. Behind it: (1) the delta walk overlaps the watermark by a fixed margin (re-reads PRs updated up to 1 hour before it); (2) brief 08's hand-edit lint refuses any pull request that changes the ledger, and its read-time checks re-fetch every record whose git-witnessed fields disagree, and refuse the whole ledger on a count mismatch or a raised watermark; (3) a ledger more than 7 days old (`recordedAt`) is not used — the run does the full read and re-records — so a wrong record lives at most 7 days with no scheduled workflow required; (4) `--verify-ledger` re-derives from a full read and exits non-zero on any difference, the independent check a scheduled run adds on top; (5) `--ledger` absent keeps today's full read, unchanged.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `reconcile --ledger <path>`: read the ledger (brief 08). If it is unusable, or its `recordedAt` is more than 7 days old (`ledgerMaxAge`, a named constant), do the full read. If usable: re-fetch the records brief 08's read returned in `reread`, fetch open PRs, fetch PRs updated since `watermark − 1h`, fold over (ledger records, overridden by any delta record for the same PR) ∪ delta ∪ open; read file lists only for open PRs in scope and delta PRs. If the ledger is missing, unparseable or fails local verification: print one NOTICE naming why, and do the full read. With `--record-ledger`, write the merged ledger and the new watermark.
2. The `--backfill` branch/body match reads candidates from ledger `backfillRefs` plus delta and open PRs — the same candidate set the full read builds.
3. `reconcile --verify-ledger <path>`: full read, derive both ways, print every brief whose cell differs, exit non-zero on any difference. This is the out-of-band check a scheduled (e.g. weekly) workflow runs.
4. Full mode pages the pull list to the end; a walk stopped by any bound is a could-not-check, never a complete list (#1969).
5. `--json` gains `forgeCost: {requests: <n>, mode: "incremental"|"full", fallbackReason: "…"}` so every run reports what it spent.
6. regen's drift comparator takes the same incremental read when given the ledger.
7. Add a scheduled `--verify-ledger` step to this repository's own statusgen workflow (weekly), staged in the PR as a diff. App tokens cannot push workflow files, so the driver pushes that one file. Name the hand-off in the PR body. The 7-day maximum age (Task 1) bounds the exposure whether or not the step has landed.
8. Changelog fragment, including the workflow change adopters make.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test -count=1 -run TestReconcileIncremental ./statusgen/...` | exit 0 — on a recorded-response fixture of 2,500 closed PRs (all in the ledger), 3 open and 2 updated since the watermark, the run makes no `state=all` page beyond the delta stop and no file-list read for a ledger PR, and its JSON equals the full-read JSON | check:ci +flow |
| 2 | `go test -count=1 -run TestReconcileIncrementalCost ./statusgen/...` | exit 0 — `forgeCost.requests` stays the same when the fixture's closed history grows from 500 to 5,000 PRs with open and delta fixed | check:ci |
| 3 | `go test -count=1 -run TestReconcileLedgerFallback ./statusgen/...` | exit 0 — a missing, malformed, or locally unverifiable ledger produces the NOTICE and the full read's exact JSON; forcing the incremental path on an unverifiable ledger makes the test fail | check:ci +mutation |
| 4 | `go test -count=1 -run TestReconcileLedgerMaxAge ./statusgen/...` | exit 0 — a ledger whose `recordedAt` is more than 7 days old produces the full read's exact JSON and a re-recorded ledger. A record in brief 08's `reread` set is fetched from the forge, and its recorded fields are not used. Removing the age check makes the test fail | check:ci +mutation |
| 5 | `go test -count=1 -run TestReconcileWatermarkEdit ./statusgen/...` | exit 0 — a merged PR whose body gains a `Brief:` trailer after the ledger was recorded (its `updated_at` moves past the watermark) is re-read and changes the derived cell | check:ci +neighbour |
| 6 | `go test -count=1 -run TestFetchAllPullsNoSilentCap ./statusgen/...` | exit 0 — a fixture with more pages than any bound returns lookedAt=false with a reason, never a short list as complete | check:ci |
| 7 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/sg" . && "${TMPDIR:-/tmp}/sg" reconcile --backfill --root .. --repo medici-finance/assay --record-ledger "${TMPDIR:-/tmp}/wl.jsonl" --json > "${TMPDIR:-/tmp}/full.json" && "${TMPDIR:-/tmp}/sg" reconcile --verify-ledger "${TMPDIR:-/tmp}/wl.jsonl" --backfill --root .. --repo medici-finance/assay` | exit 0 — a live read of this repository: the ledger-derived and full-read results agree (GITHUB_TOKEN set) | check +dereference |
| 8 | `statusgen --root . --consumers --brief derived-board/09` | exit 0 (routing claims corroborated against the diff) | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
