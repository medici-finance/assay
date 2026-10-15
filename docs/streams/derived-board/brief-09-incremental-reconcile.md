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
  - "ci/staged-workflows/assay-statusgen.yml (regen_all passes --ledger; weekly --verify-ledger job): follow-up derived-board/09 (this brief; the edit lands in the staged copy, which a maintainer promotes because App tokens lack workflow scope)"
  - "adopter regen workflows (pass --ledger, commit the ledger, add the scheduled --verify-ledger run): out-of-scope (named in the release note of the version that carries this brief; applied per repo through upgrade-assay)"
---

# Brief 09 — incremental reconcile

## Context
files:
- `statusgen/ghfetch.go` — `fetchOpenPulls`, `fetchPullsUpdatedSince(watermark)`
- `statusgen/reconcile.go` — `--ledger <path>` (incremental mode), `--verify-ledger`, the `forgeCost` JSON field
- `statusgen/regen.go` — the drift comparator reads PRs the same way
- `statusgen/witnessledger.go` (planned) — merge delta records into the ledger
- `statusgen/reconcileincremental_test.go` (planned)
- `ci/staged-workflows/assay-statusgen.yml` — `regen_all` (brief 08 Task 8) passes `--ledger`; a weekly scheduled `--verify-ledger` job. The staged copy is what the job-text tests read and a maintainer promotes it; the `.github/workflows/` copy is not edited here
- `statusgen/reconcilejob_test.go` — `TestReconcileJobTriggerAndScope` pins exactly one cron and the `reconcile` job's `if:`; amended in the same PR for the weekly cron
- `statusgen/reconcilejobledger_test.go` (planned) — extended with the `--ledger` pin and the weekly job (Verify 8)
- `changelog/derived-board-09-incremental-reconcile.md` (planned)

facts:
- The ledger (brief 08) is `.statusgen/witness-ledger.jsonl`. It holds every closed PR the fold could read as a witness, witnessed against local git on read, with `watermark` set to the largest `updated_at` the recording run saw. Brief 08 Task 8 gives the `regen` job `fetch-depth: 0`, `pull-requests: read`, and one `regen_all` function used by both the first attempt and the retry.
- `GET /repos/{o}/{r}/pulls?state=open` costs one request per 100 open PRs. `GET /repos/{o}/{r}/pulls?state=all&sort=updated&direction=desc` returns PRs newest-updated first, so a walk can stop at the first PR whose `updated_at` is older than the watermark: one request per 100 PRs changed since the last run. A body edit, merge, close or reopen all bump `updated_at`.
- File lists: a ledger record whose `files` is a list or `"delivers"` needs no read. A ledger record whose `files` is `"not-fetched"` needs exactly one read, and only when the fold needs its authoring answer (its brief is movable, or it is a backfill candidate). Open PRs in scope and delta PRs are read as today.
- Brief 08's merge-shape witness (squash: `M^1..M`; merge commit: `merge-base(M^1, M^2)..M^2`) keeps two-parent merges out of `reread`. Only records with a forge status git cannot map (`copied`, `changed`, `unchanged`, the 3,000-file cap) or a failed witness are re-fetched; `forgeCost` reports how many.
- The full pull walk's silent 20-page cap is removed by #3013 (#1969), which also adds `TestFetchAllPullsNoSilentCap`. If #3013 lands first, row 6 below cites that test as already passing.
- Measured target (2026-10-15, board of 1,936 PRs, 17 open): an incremental run ≈ 1 open-list request + 1 delta request + file lists for in-scope open/delta PRs and needed `"not-fetched"` records + rereads + the open-issue pages, against 301 for today's full read.

single-point-of-failure: the watermark, together with the ledger's forge-only fields — a PR changed before the watermark but missing from the ledger, or a forge-only field that is wrong, is not re-read by an incremental run. Brief 08's hand-edit lint detects that at review (and prevents it once `lint` is a required check, brief 08 Task 9). Behind it the rest DETECT or BOUND: (1) the delta walk overlaps the watermark by 1 hour (boundary races); (2) brief 08's read-time witnessing re-fetches every record git can disprove, merged or closed; (3) the full-read age — read from git (brief 08 Task 4: the newest writer-committed, ledger-touching `Ledger-Full-Read: yes` commit), never from a header field — makes any ledger older than 7 days unusable, so a wrong forge-only field or a deleted record lives at most 7 days; (4) `--verify-ledger` re-derives from a full read and exits non-zero on any difference, and on an unusable ledger (Task 3: it never falls back to the full read, so it never compares a full read with itself), scheduled independently of every incremental run; (5) `--ledger` absent keeps today's full read.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `reconcile --ledger <path>`: read the ledger (brief 08).
   - **Full read instead** when the ledger is missing, unparseable or unusable, or when brief 08's git-witnessed `fullReadAt` is more than 7 days old (`ledgerMaxAge`, a named constant). Print one NOTICE naming why.
   - **Otherwise, incremental:** re-fetch the records in brief 08's `reread` set, fetch open PRs, and fetch PRs updated since `watermark − 1h`. Fold over (ledger records, overridden by any delta record for the same PR) ∪ delta ∪ open. Read file lists for open PRs in scope, for delta PRs, and for any `"not-fetched"` ledger record whose authoring answer the fold needs. Read none for a ledger record whose list or `"delivers"` is recorded.
   - **Re-recording** (`--record-ledger`): the merged ledger and the new watermark. A delta PR that is reopened (now open), or no longer in scope (trailer and every backfill ref removed), loses its ledger record. A `"not-fetched"` record that was read this run is recorded with its list.
   - **Commit message** (brief 08 Task 6): `--commit-message-file` carries `Ledger-Full-Read: yes` only after a full read. An incremental re-record carries no trailer, so it never extends the age.
2. The `--backfill` branch/body match reads candidates from ledger `backfillRefs` plus delta and open PRs: the same candidate set the full read builds.
3. `reconcile --verify-ledger <path> --ledger-writer <email>`: read the ledger under brief 08 Task 4's witnessing (the writer email is required there), do a full read, derive both ways, print every brief whose cell differs, and exit non-zero on any difference.
   - **An unusable ledger fails; it never falls back.** If the ledger is missing, unparseable or unusable under brief 08 Task 4 (shallow clone, header/record mismatch, a watermark past its commit, a future-dated full-read commit, or `--ledger-writer` absent or empty), `--verify-ledger` exits non-zero and names the reason. It does NOT apply Task 1's full-read fallback, because that would derive "both ways" from the same full read and always agree. The 7-day age is not a refusal here: it bounds what an incremental run may trust, and an aged ledger that differs from a full read is exactly what this verb exists to catch.
4. Full mode pages the pull list to the end; a walk stopped by any bound is a could-not-check (#1969, delivered by #3013 if it lands first).
5. `--json` gains `forgeCost: {requests: <n>, rereads: <n>, notFetchedReads: <n>, mode: "incremental"|"full", fallbackReason: "…"}`.
6. regen's drift comparator takes the same incremental read when given the ledger.
7. **Workflow: an edit to the staged copy, `ci/staged-workflows/assay-statusgen.yml`, that a maintainer promotes** (brief 08 Task 8 states why). The edit composes with the classified push already in the `regen` commit step; it adds no push path.
   - In the `regen` job's "Commit STATUS.md if it changed" step, `regen_all` (brief 08 Task 8) adds `--ledger "$LEDGER"` to its `reconcile --record-ledger` call. The existence guards on both ledger pathspecs, the restore on a failed run, the preflight before the commit and before every retry, and the `nonff`-only retry are brief 08's and stay as written. That one flag is what makes every push-to-main regen incremental; the checkout is already full-depth.
   - Add a weekly `schedule`-triggered job (with `LEDGER=.statusgen/witness-ledger.jsonl` set, as in brief 08 Task 8) running `reconcile --verify-ledger "$LEDGER" --ledger-writer 319272446+assay-board-writer[bot]@users.noreply.github.com --backfill --root . --repo "$GITHUB_REPOSITORY"` (the writer email is the same literal brief 08 Task 8 passes to `--record-ledger`; without it brief 08 Task 4 makes the ledger unusable) with the read-only token (`contents`, `pull-requests`, `issues`: read), no App token, and `fetch-depth: 0`, failing the job on a difference. The workflow already has an hourly cron (`17 * * * *`), a `schedule` concurrency group and a `reconcile` job guarded on `github.event_name == 'schedule'`. So every scheduled job tests `github.event.schedule` against its own cron: the weekly job runs only on the weekly cron, and the hourly `reconcile` job no longer runs on it. The same PR amends `TestReconcileJobTriggerAndScope`, which pins exactly one cron and the `reconcile` job's `if:`, to the two-cron shape.
   - The hourly `reconcile` job is unchanged: it keeps the full read by design. Only `regen_all` and the weekly job read the ledger.
   - A missing ledger fails the weekly job loudly (Task 3), with no special case in the job: until the first push-to-main regen under the promoted workflow commits one, there is nothing to verify, and a green job would claim otherwise.
   - Name the hand-off in the PR body. The 7-day maximum age (Task 1) bounds the exposure whether or not the weekly job has landed. Until the promoted workflow carries this edit and brief 08's, every push-to-main regen is the full read.
8. Changelog fragment, including the workflow change adopters make.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test -count=1 -run TestReconcileIncremental ./statusgen/...` | exit 0 — on a recorded-response fixture of 2,500 closed PRs in the ledger (lists recorded, plus some `"not-fetched"`), 3 open and 2 updated since the watermark: no `state=all` page beyond the delta stop, no file-list read for a ledger PR with a recorded list, exactly one read for each `"not-fetched"` record the fold needs and none for the rest, and JSON equal to the full read's | check:ci +flow |
| 2 | `go test -count=1 -run TestReconcileIncrementalCost ./statusgen/...` | exit 0 — `forgeCost.requests` stays the same when the fixture's closed history grows from 500 to 5,000 PRs (squash and merge-commit records mixed, none in `reread`) with open and delta fixed | check:ci |
| 3 | `go test -count=1 -run TestReconcileLedgerFallback ./statusgen/...` | exit 0 — a missing, malformed or unusable ledger produces the NOTICE and the full read's exact JSON. Forcing the incremental path on an unusable ledger fails. Under `--verify-ledger` the same unusable ledgers (missing, malformed, shallow clone, `--ledger-writer` absent or empty) exit non-zero naming the reason and never print a match; a mutation that falls back to the full read under `--verify-ledger` and exits 0 fails that case | check:ci +mutation |
| 4 | `go test -count=1 -run TestReconcileLedgerMaxAge ./statusgen/...` | exit 0 — a ledger whose git-witnessed full read is more than 7 days old produces the full read's exact JSON and a re-recorded ledger with a trailer-carrying message. A record in `reread` is fetched from the forge, and its recorded fields are not used. Removing the age check fails | check:ci +mutation |
| 5 | `go test -count=1 -run TestReconcileWatermarkEdit ./statusgen/...` | exit 0 — a merged PR whose body gains a `Brief:` trailer after recording (its `updated_at` moves past the watermark) is re-read and changes the derived cell. A delta PR whose trailer was removed, or that was reopened, loses its ledger record on re-record | check:ci +neighbour |
| 6 | `go test -count=1 -run TestFetchAllPullsNoSilentCap ./statusgen/...` | exit 0 — a walk that hits the bound returns lookedAt=false with a reason, never a short list as complete (lands with #3013 if that merges first) | check:ci |
| 7 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/sg" . && cd .. && S=$(mktemp -d "${TMPDIR:-/tmp}/wl.XXXXXX") && W="319272446+assay-board-writer[bot]@users.noreply.github.com" && git clone -q --no-local . "$S/r" && mkdir -p "$S/r/.statusgen" && "${TMPDIR:-/tmp}/sg" reconcile --backfill --root "$S/r" --repo medici-finance/assay --record-ledger "$S/r/.statusgen/witness-ledger.jsonl" --commit-message-file "$S/msg" --ledger-writer "$W" && git -C "$S/r" add .statusgen/witness-ledger.jsonl && GIT_COMMITTER_NAME=assay-board-writer GIT_COMMITTER_EMAIL="$W" git -C "$S/r" -c user.name=assay-board-writer -c user.email="$W" commit -q -F "$S/msg" && "${TMPDIR:-/tmp}/sg" reconcile --verify-ledger "$S/r/.statusgen/witness-ledger.jsonl" --ledger-writer "$W" --backfill --root "$S/r" --repo medici-finance/assay` | exit 0 — a live read of this repository, run in a scratch full-depth clone where the recorded ledger is committed under the writer identity with the `Ledger-Full-Read: yes` trailer from the same run, so the ledger is usable and derived from. The ledger-derived and full-read results agree, and no fallback NOTICE is printed (a fallback here is a failure; GITHUB_TOKEN set) | check +dereference |
| 8 | `go test -count=1 -timeout 300s -run TestReconcileJobVerifyLedger ./statusgen/...` | exit 0 — job-text test against the staged YAML, in the style of `TestReconcileJob*`: (a) the "Commit STATUS.md if it changed" step's `regen_all` passes `--ledger` and still sources `pushcred.sh`, preflights before the commit and before the retry, and pushes with `pushcred_push`; (b) a weekly cron exists beside the hourly `17 * * * *`; the weekly job's `if:` names `github.event.schedule` with that cron, holds exactly `contents`, `pull-requests`, `issues` read, references no App token, checks out `fetch-depth: 0`, and runs `reconcile --verify-ledger` with a non-empty `--ledger-writer` and no `|| true`; (c) the hourly `reconcile` job's `if:` excludes the weekly cron; (d) executing the weekly job's run text against a stub `statusgen` that exits 1 fails the job, and exit 0 passes it. Mutations: drop `--ledger` from `regen_all`; drop the weekly job's `github.event.schedule` test; drop its `--ledger-writer`; give the weekly job the App token; swallow the exit code. Each fails its case | check:ci +mutation |
| 9 | `statusgen --root . --consumers --brief derived-board/09` | exit 0 (routing claims corroborated against the diff) | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
