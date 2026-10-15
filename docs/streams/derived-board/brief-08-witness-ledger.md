---
brief: assay:assay:derived-board:08
title: witness ledger — the board writer records every merged-PR fact the fold reads, so a merged PR is read from the forge once, ever
why: >-
  `statusgen reconcile` re-reads every pull request a repository has ever had, and the
  changed-file list of a few hundred of them, on every board regeneration — about 280
  rate-limited forge requests per run on a consumer board of ~1,900 PRs, of which 17 were
  open. A merged PR never changes, so every one of those reads re-derives an answer the
  previous run already had. Recording the merged-PR facts the fold reads, once, in a
  generated ledger is what lets brief 09 make the forge cost follow the OPEN work instead of
  the whole history, which only grows.
wave: 2
depends: ["derived-board/03"]
unblocks: ["derived-board/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
schema: brief-v2
version: 1
authored: 2026-10-15 by a reconcile performance investigation session
sources:
  - "reconcile --backfill profile, 2026-10-14/15, on a consumer board (1,936 PRs: 17 open, 1,753 merged, 166 closed): 301 forge requests per run — 40 pull-list pages (the list paged twice), 250 per-PR file-list reads, 11 open-issue pages"
  - "#2615 (the earlier reconcile slowdown — whole-board lint per moved row)"
  - "#1969 (fetchAllPulls silently truncates at 20 pages)"
  - "docs/streams/derived-board/spec.md (single-writer generated surfaces)"
  - freshness-checked 2026-10-15 @ e7c208b00 (origin/main)
exec-tier: strong
exec-tier-why: "(a) the record schema and the exactness argument are design work; (b) correctness is a cross-component property — the fold must derive the same cells from ledger + delta as from a full read"
domain: complicated
consumers:
  - "statusgen/reconcile.go: follow-up derived-board/08 (this brief adds the --record-ledger write flag; flips to fixed-here when the implementation lands)"
  - "statusgen/reconcile.go (read side): follow-up derived-board/09 (reads the ledger)"
  - "statusgen/regen.go (drift comparator, same PR read): follow-up derived-board/09"
  - ".github/workflows/assay-statusgen.yml (board-writer job writes and commits the ledger): follow-up derived-board/08 (this brief; staged as a diff, pushed by the driver because App tokens lack workflow scope)"
  - "adopter regen workflows (commit the ledger with the generated READMEs): out-of-scope (the release note of the version carrying brief 09 names the one-line workflow change; adopters apply it through upgrade-assay)"
---

# Brief 08 — witness ledger

## Context
files:
- `statusgen/witnessledger.go` (planned) — record type, read, write, local verification
- `statusgen/witnessledger_test.go` (planned)
- `statusgen/reconcile.go` — the `--record-ledger <path>` flag (write side only)
- `statusgen/main.go` — wire the ledger hand-edit check into `--lint` beside `checkReadmeTables`
- `.github/workflows/assay-statusgen.yml` — the board-writer job (the sole writer, `assay-board-writer[bot]`) runs the ledger write and adds the file to the commit it already pushes. App tokens lack workflow scope, so this file is staged as a diff and the driver pushes it
- `statusgen/briefauthoring.go` — expose the per-PR authoring facts the ledger stores
- `docs/streams/derived-board/spec.md` — a short section naming the ledger as a generated, single-writer surface
- `changelog/derived-board-08-witness-ledger.md` (planned)

facts:
- Forge reads per `reconcile --backfill` run today (measured 2026-10-15, board of 1,936 PRs): `GET /pulls?state=all` pages (one per 100 PRs, plus a second identical walk — fixed separately), one `GET /pulls/{n}/files` per candidate PR (~250), `GET /issues?state=open` pages (~11). No comment or review endpoint is read by reconcile.
- What the fold reads from a pull (`ghfetch.go` `ghPull`): number, state, draft, body (for the single `Brief:` trailer, and the `--backfill` branch/body forms), merged_at, merge_commit_sha, head sha, head ref. From a file list (`briefauthoring.go`): only whether every path is an authoring path (stream README, brief file, changelog fragment) and which brief files it ADDS.
- Merged PRs are immutable except for a later edit of the body, which bumps the PR's `updated_at` (brief 09 re-reads anything whose `updated_at` passed the watermark).
- Of 255 briefs with a trailer PR on the measured board, 77 have more than one merged trailer PR (delivery plus rework/pin follow-ups); the ledger stores every one, never "the" PR.
- Precedent for a generated JSONL surface under `docs/streams/`: `docs/streams/verify-outcomes.jsonl`.

single-point-of-failure: the ledger only ever changing through the board writer, which commits it directly on the default branch, never through a pull request. Of the layers behind it, only (1) PREVENTS an edit; the rest DETECT or BOUND one, and each is named for what it does. (1) Prevention: a `--lint` PROBLEM, run in CI on every pull request, when a branch's diff against `refs/remotes/origin/main` touches the ledger (the rule-47 shape of `checkReadmeTables`). (2) Witnessing: every record whose claim git can check is checked on every read, and a failure sends that PR back to the forge (Task 4). (3) Corruption detection, not tamper prevention: the header's `recordCount`/`maxNumber` catch an accidental truncation, but they are self-attested in the same file, so a deliberate delete-and-decrement passes them; deletion is prevented only by (1) and bounded by (4). (4) Bounded exposure: the age of the last FULL read is taken from git (the committer time of the newest default-branch commit carrying `Ledger-Full-Read: yes`), never from a field in the file, and brief 09 refuses a ledger older than 7 days. (5) Independent detection: brief 09's scheduled `--verify-ledger`. The field table in Task 4 says, field by field, which layer prevents and which only bounds.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define the ledger: `docs/streams/witness-ledger.jsonl`, one header line (`{"schema":"witness-ledger-v1","repo":…,"watermark":<max updated_at seen>,"recordedAt":… (informational only; never read for any decision),"recordCount":<n>,"maxNumber":<largest PR number recorded>}`), then one line per closed PR. Open PRs are NEVER recorded. Per record:
   - **Identity:** `number`, `state` (merged|closed), `mergeSha`, `headSha`, `headRef`, `mergedAt`, `updatedAt`, `briefTrailer` (the single `Brief:` trailer, or empty).
   - **Backfill refs, independent of the brief set:** `backfillRefs` is a list of `{run, sep, next}` entries, one per occurrence in head ref + body of a `/` or `-` followed by a digit. `run` is the maximal run of `[A-Za-z0-9._-]` ending just before the separator; `next` is the following three characters. The ledger is pushed directly, past merge-time content gates, so it stores the minimum PR-sourced text: `run` is cut to its last 64 characters (a stream name is far shorter), no entry is stored unless `next` starts with two digits, and no title, body or label is stored. At read time a brief `<stream>/<NN>` matches a record iff some entry has `stream` as a suffix of `run` and `NN` as a prefix of `next`. Because stream names use only run characters, that is exactly `containsBackfillForm(headRef or body, stream, NN)`, so a brief authored AFTER the record was written still matches correctly. The brief set is never consulted when recording. Store no bodies.
   - **Authoring facts, the classifier's own inputs:** `files` is either (a) the list `[{filename, status, previous_filename}]` exactly as `GET /pulls/{n}/files` returned it, when every path is an authoring path; (b) `"delivers"`, when any path is not (the PR can never be authoring-only for any brief); or (c) `"unread"`, when this run did not read the list. Today's full run reads file lists only for candidate PRs, so most records are `"unread"`. A run that later needs the authoring answer for an `"unread"` record fetches that one list and records it. The ledger never forces a read the full run would not make.
   - **Scope:** a closed PR is recorded when it carries a trailer, or when `backfillRefs` is non-empty or its head ref contains a separator-digit form. That is a brief-set-independent superset of every PR the fold or the backfill could ever read as a witness.
2. Sort records by number; write deterministically (same inputs → byte-identical file). No PR bodies are stored — only the extracted refs.
3. `reconcile --record-ledger <path>` (requires `--backfill`, online): after a successful run, write the ledger from the pulls and file lists the run already read. A run that could not look (any `unread`, any refusal) writes nothing and exits non-zero — a partial ledger is never written.
4. `readWitnessLedger(path, root)`: parse, then verify locally, with no forge call:
   - **Shallow clone.** If the clone is shallow, the ledger is unusable (could-not-check, so a full read). "The last commit that touched the ledger" would resolve to the shallow boundary, and the age in Task 4's last item could not be read.
   - **Corruption detection.** The header's `recordCount` equals the number of records, and the largest record number equals `maxNumber`. A mismatch makes the ledger unusable. This is self-attested, so name it corruption detection and never tamper prevention.
   - **Merged records.** The merge sha resolves (`git cat-file -e`) and is reachable from the default branch, and its first-parent commit subject ends `(#<number>)`. The file list is then checked against `git diff-tree -r --no-renames --name-status <sha>^1 <sha>`, with renames expanded the way `--no-renames` reports them: REST `added` must be git `A`, `modified` `M`, `removed` `D`, and `renamed` must be a `D` of `previous_filename` plus an `A` of `filename`. Every git entry must be accounted for. A `"delivers"` record must show at least one non-authoring path.
   - **Closed records (`state: closed`, empty `mergeSha`/`mergedAt`).** No first-parent commit on the default branch may carry the PR's number: neither a subject ending `(#<number>)` nor a `Merge pull request #<number> ` subject. One `git log --first-parent --format=%s` pass builds that set for the whole ledger. A record claiming closed for a PR the history shows merged is the "rewrite merged as closed" tamper, and it goes in `reread`.
   - **`reread` set.** Any record that fails a check, or whose merge commit has more than one parent (the first-parent diff is then not the PR's own change), goes in `reread`. The caller re-fetches exactly those PRs from the forge and never trusts their recorded fields. The verdict always comes from the recorded REST fields; git only witnesses them.
   - **Watermark.** `watermark` may not be later than the committer time of the newest commit that touched the ledger, or the ledger is unusable. A lowered watermark only causes extra delta reads, which is the safe direction.
   - **Age of the last full read.** The newest first-parent commit on the default branch that touched the ledger AND carries the trailer `Ledger-Full-Read: yes`. Its committer time is returned as `fullReadAt`. A header field is never used for this. Incremental writes (brief 09) commit without the trailer, so they never refresh the age. A future-dated committer time (later than now plus 5 minutes) makes the ledger unusable.
   - **Field by field.** The function's doc comment carries this table verbatim:

     | Field | Prevented by | Witnessed (re-read on failure) | Bounded by |
     |---|---|---|---|
     | `number`, `mergeSha`, `state: merged` | lint (1) | reachability + subject `(#N)` (2) | — |
     | `state: closed` | lint (1) | no first-parent commit for `#N` (2) | — |
     | `files` list / `"delivers"` | lint (1) | first-parent `diff-tree` (2) | — |
     | `files: "unread"` | — | — (not a claim; fetched on demand) | — |
     | `briefTrailer`, `backfillRefs`, `headRef`, `headSha`, `mergedAt`, `updatedAt` | lint (1) | none, forge-only | full-read age (4); `--verify-ledger` (5) |
     | record presence (a deleted record) | lint (1) | none (the header count is corruption detection only) | full-read age (4); `--verify-ledger` (5) |
     | `watermark` | lint (1) | upper bound: committer time of the ledger commit | full-read age (4) |
     | full-read age | git trailer + committer time | future time rejected | — |
5. Hand-edit lint, the rule-47 shape: `--lint` reports a PROBLEM when the branch's diff against `refs/remotes/origin/main` adds, removes or modifies `docs/streams/witness-ledger.jsonl`, naming the board writer as the file's only writer. Where the diff base cannot be resolved (shallow clone, no `origin/main`), report a could-not-check NOTICE, as the existing branch-diff gates do, and never pass silently. Wire it into `main.go` beside `checkReadmeTables`.
6. The writer. Today the board-writer job in `.github/workflows/assay-statusgen.yml` commits `STATUS.md` only, and no `reconcile` step writes to the default branch. Add a step to that job, before its commit: `statusgen reconcile --backfill --root . --repo "$GITHUB_REPOSITORY" --record-ledger docs/streams/witness-ledger.jsonl`. Do the forge READ with the job's read-only default `GITHUB_TOKEN` (`contents: read`, `pull-requests: read`), as the existing reconcile job does. The board-writer App token is minted only for the push, never for the read. `git add` the ledger beside `STATUS.md`. A full read commits with the trailer `Ledger-Full-Read: yes` (Task 4's age source); an incremental write (brief 09) commits without it. A non-zero `reconcile` exit leaves the previous ledger in place and does not fail the STATUS.md commit. Stage the workflow edit as a diff and hand it to the driver to push, naming the hand-off in the PR body. Until it lands no ledger exists, and brief 09 does the full read, which is today's behaviour.
7. Name the ledger in `docs/streams/derived-board/spec.md` as a generated surface with one writer (the board-writer job above, committing directly on the default branch), alongside the Briefs table. Name the four layers and which fields each one covers.
8. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test -count=1 -run TestWitnessLedger ./statusgen/...` | exit 0 — round trip; deterministic bytes; open PRs never recorded; a run with any `unread` writes nothing | check:ci |
| 2 | `go test -count=1 -run TestWitnessLedgerBackfillRefsExact ./statusgen/...` | exit 0 — property test over generated head refs, bodies and brief ids, including briefs not present when the record was written: the read-time match over `backfillRefs` equals `containsBackfillForm` on the original text for every pair. A recorder that consulted the brief set, or dropped the suffix rule, fails | check:ci +mutation |
| 3 | `go test -count=1 -run TestWitnessLedgerUnreadFiles ./statusgen/...` | exit 0 — a record whose `files` is `"unread"` makes exactly one file-list read when (and only when) the fold needs its authoring answer, and the re-recorded ledger carries the list; a renamed-file PR round-trips `status` and `previous_filename` and classifies the same as the live read | check:ci |
| 4 | `go test -count=1 -run TestWitnessLedgerFoldParity ./statusgen/...` | exit 0 — on a recorded-response fixture, the fold over (ledger records + the open PRs) derives byte-identical `reconcile --backfill --json` to the fold over the full pull and file-list read, including a brief with two merged trailer PRs and an authoring-only PR | check:ci +flow |
| 5 | `go test -count=1 -run TestWitnessLedgerTamper ./statusgen/...` | exit 0 — each tamper is caught: (a) a merge sha absent from local git or unreachable from the default branch, or whose commit subject names a different PR number, puts that record in `reread`; (b) an edited `files` list or a `"delivers"` that disagrees with the merge commit's first-parent diff puts it in `reread`; (c) a deleted record (count mismatch) or a raised `watermark` makes the ledger unusable. Removing any one of those checks makes its case fail | check:ci +mutation |
| 6 | `go test -count=1 -run TestWitnessLedgerClosedTamper ./statusgen/...` | exit 0 — a merged record rewritten as `state: closed` with empty `mergeSha`/`mergedAt` lands in `reread`, because the first-parent history carries `(#N)`. A genuinely closed PR does not. With the closed-record check removed, the rewrite derives a demoted cell and the test fails | check:ci +mutation |
| 7 | `go test -count=1 -run TestWitnessLedgerFullReadAge ./statusgen/...` | exit 0 — `fullReadAt` comes from the newest `Ledger-Full-Read: yes` commit, not the header. A future-dated header `recordedAt` changes nothing. A later commit without the trailer does not refresh it. A future-dated committer time, or a shallow clone, makes the ledger unusable. Keying the age on the header field fails the test | check:ci +mutation |
| 8 | `go test -count=1 -run TestWitnessLedgerHandEditLint ./statusgen/...` | exit 0 — on a fixture repo, a branch whose diff against `origin/main` edits one ledger line makes `--lint` exit non-zero with a PROBLEM naming the ledger and its single writer. The same branch without the ledger change lints clean. With the check unwired, the edited branch lints clean and the test fails | check:ci +mutation |
| 9 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/sg" . && "${TMPDIR:-/tmp}/sg" reconcile --backfill --root .. --repo medici-finance/assay --record-ledger "${TMPDIR:-/tmp}/wl.jsonl" --json > /dev/null && head -1 "${TMPDIR:-/tmp}/wl.jsonl"` | exit 0; the header line names `witness-ledger-v1` and `medici-finance/assay` (a live read of this repository, GITHUB_TOKEN set) | check +dereference |
| 10 | `statusgen --root . --consumers --brief derived-board/08` | exit 0 (routing claims corroborated against the diff) | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
