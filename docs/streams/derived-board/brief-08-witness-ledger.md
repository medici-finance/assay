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
  - ".github/workflows/assay-statusgen.yml (regen job writes and commits the ledger; pull_request paths watch it): follow-up derived-board/08 (this brief; staged as a diff, pushed by the driver because App tokens lack workflow scope)"
  - "the default branch's required status checks (add lint): out-of-scope (a ruleset change only the driver makes; named in Task 9 and the PR body)"
  - "adopter regen workflows (commit the ledger with the generated READMEs): out-of-scope (the release note of the version carrying brief 09 names the one-line workflow change; adopters apply it through upgrade-assay)"
---

# Brief 08 — witness ledger

## Context
files:
- `statusgen/witnessledger.go` (planned) — record type, read, write, local witnessing, commit-message composition
- `statusgen/witnessledger_test.go` (planned)
- `statusgen/reconcile.go` — the `--record-ledger <path>` and `--commit-message-file <path>` flags (write side only)
- `statusgen/main.go` — wire the ledger hand-edit check into `--lint` beside `checkReadmeTables`
- `.github/workflows/assay-statusgen.yml` — the `regen` job (the sole writer, `assay-board-writer[bot]`) writes and commits the ledger, and the `pull_request` trigger watches the ledger path. App tokens lack workflow scope, so this file is staged as a diff and the driver pushes it
- `statusgen/briefauthoring.go` — expose the per-PR authoring facts the ledger stores
- `docs/streams/derived-board/spec.md` — a section naming the ledger as a generated, single-writer surface, carrying Task 4's field table
- `changelog/derived-board-08-witness-ledger.md` (planned)

facts:
- Forge reads per `reconcile --backfill` run (measured 2026-10-15 on a board of 1,936 PRs): `GET /pulls?state=all` pages (one per 100 PRs), one `GET /pulls/{n}/files` per candidate PR (~250), and `GET /issues?state=open` pages (~11). Reconcile reads no comment or review endpoint.
- What the fold reads from a pull (`ghfetch.go` `ghPull`): number, state, draft, body (for the single `Brief:` trailer and the `--backfill` branch/body forms), merged_at, merge_commit_sha, head sha and head ref. From a file list (`briefauthoring.go` `briefAuthoringOnly`) it reads each entry's filename, status and previous_filename.
- Merged PRs are immutable except for a later edit of the body, which bumps the PR's `updated_at`. Brief 09 re-reads anything whose `updated_at` passed the watermark.
- On the measured board, 77 of the 255 briefs with a trailer PR have more than one merged trailer PR. The ledger stores every one.
- This repository merges both ways: re-check on `refs/remotes/origin/main` with `git log --first-parent --merges --oneline`. As of 2026-10-15 there are 541 two-parent `Merge pull request #N from …` commits alongside squash commits whose subject ends `(#N)`. Rebase merges are allowed by the ruleset but unused in the last 400 first-parent commits.
- The `regen` job today (`.github/workflows/assay-statusgen.yml`): `permissions: contents: read`; a default (shallow) checkout; a commit guard on `git status --porcelain -- STATUS.md` only; and a retry path that runs `git reset --hard origin/main` and regenerates STATUS.md only. Loop guard 1 is the `push` path filter (`docs/**`), and STATUS.md lives outside it on purpose. Loop guard 2 is the `[skip-status-regen]` message token. Only `leak-sweep` is a required status check; `lint` is not.

single-point-of-failure: the ledger only ever changing through the board writer, which commits it directly on the default branch, never through a pull request. Behind it, five layers, each named for what it actually does. (1) Review-time detection: a `--lint` PROBLEM on every pull request whose diff touches the ledger. This is PREVENTION only once `lint` is a required status check, a ruleset change the driver makes (Task 9); until then a reviewer or merger can override it. (2) Witnessing: every record claim git can check is checked on every read, and a failure sends that PR back to the forge (Task 4). (3) Corruption detection, never tamper prevention: the header's `recordCount`/`maxNumber` are self-attested, so they catch a truncation but a deliberate delete-and-decrement passes them. (4) Bounded exposure: the age of the last full read comes from git (Task 4: the newest ledger-touching first-parent commit committed by the board-writer identity and carrying `Ledger-Full-Read: yes`), and brief 09 refuses a ledger older than 7 days. (5) Independent detection: brief 09's scheduled `--verify-ledger`. Task 4's field table states, field by field, which layer witnesses and which only bounds.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Ledger path and header.** The ledger is `LEDGER` = `.statusgen/witness-ledger.jsonl`. It sits outside `docs/**`, as STATUS.md sits outside it, so the writer's own commit cannot retrigger the push workflow (loop guard 1). The header line is `{"schema":"witness-ledger-v1","repo":…,"watermark":<max updated_at seen>,"recordCount":<n>,"maxNumber":<largest PR number>,"recordedAt":…}`. `recordedAt` is informational and never read for a decision. One line per closed PR follows; open PRs are NEVER recorded.
   - **Per-record fields:** `number`, `state` (merged|closed), `mergeSha`, `headSha`, `mergedAt`, `updatedAt`, and `briefTrailer` (the single `Brief:` trailer, or empty). No head ref, title, body or label is stored. The ledger is pushed past merge-time content gates, so it carries the minimum PR-sourced text.
   - **Backfill refs, independent of the brief set:** `backfillRefs` lists `{src, run, sep, next}`, computed separately over the head ref (`src: ref`) and the body (`src: body`). A run never spans the two, matching `containsBackfillForm` on each text. There is one entry per `/` or `-` that is followed by a digit. `run` is the maximal run of `[A-Za-z0-9._-]` ending just before the separator, cut to its last 64 characters; `next` is the following three characters. At read time a brief `<stream>/<NN>` matches iff some entry has `stream` as a case-sensitive suffix of `run` and `NN` as a prefix of `next`. This equals `containsBackfillForm` for stream names of 64 run characters or fewer and NN of 2–3 digits, the brief-id grammar; the brief says so in the record type's doc comment. A brief authored after the record still matches. The recorder never consults the brief set.
   - **Authoring facts:** `files` is either (a) the list `[{filename, status, previous_filename}]` exactly as `GET /pulls/{n}/files` returned it, when every path is an authoring path; or (b) `"delivers"` when any path is not; or (c) `"not-fetched"` when this run did not need the list. Most records are `"not-fetched"`; today's full run reads lists only for candidate PRs, and recording never forces a read. A later run that needs the answer for a `"not-fetched"` record fetches that one list (brief 09).
   - **Scope:** a closed PR is recorded when it carries a trailer or has any `backfillRefs` entry. That is a brief-set-independent superset of every PR the fold or the backfill could read as a witness.
2. Sort records by number and write deterministically: the same inputs give a byte-identical file.
3. `reconcile --record-ledger <path>` (requires `--backfill`, online) writes the ledger from the pulls and file lists the run already read, after a successful run. A run that could not look (any entry in the run's `unread` disclosures, meaning a FAILED read, or a whole-run refusal) writes nothing and exits non-zero, so a partial ledger is never written. `"not-fetched"` is not a failed read.
4. `readWitnessLedger(path, root)` parses the ledger, then witnesses it locally with no forge call.
   - **Unusable ledger** (caller does a full read): a shallow clone (the history the checks below need is cut); a header/record mismatch (Task 1 counts); `watermark` later than the committer time of the newest commit that touched the ledger; or a future-dated full-read commit (beyond now + 5 minutes).
   - **Merged records.** The merge commit `M = mergeSha` resolves and is reachable from the default branch on its first-parent line. Its subject names the PR: a squash subject ending `(#<number>)`, or a merge subject `Merge pull request #<number> from …`. The PR's own change is then computed by shape:
     - **One parent (squash):** `git diff-tree -r --no-renames --name-status M^1 M`.
     - **Two parents (merge commit):** `M^2` must equal `headSha`. Use `git diff-tree -r --no-renames --name-status $(git merge-base M^1 M^2) M^2`, the same three-dot diff the forge's file list reports.
   - **File-list witness.** A `files` list must match the PR's change with renames expanded the way `--no-renames` reports them: `added`↔`A`, `modified`↔`M`, `removed`↔`D`, and `renamed` = `D` of `previous_filename` + `A` of `filename`. Every git entry must be accounted for. A `"delivers"` record must show at least one non-authoring path. The forge statuses `copied`, `changed` and `unchanged`, and a list at the 3,000-file cap, have no git counterpart here, so those records go to `reread` (counted in brief 09's `forgeCost`).
   - **Closed records** (`state: closed`, empty `mergeSha`/`mergedAt`). No first-parent subject on the default branch may name the PR in either form above. One `git log --first-parent --format=%s` pass builds that set. A closed claim for a PR the history shows merged goes to `reread`. This witness covers the squash and merge-commit methods only. A rebase merge leaves no PR number on the default branch, so for a rebase-merged PR `state: closed` is forge-only (field table).
   - **`reread`.** A record that fails its witness is re-fetched by the caller from the forge, and its recorded fields are not used. The verdict always comes from the recorded REST fields; git only witnesses them.
   - **Full-read age** (`fullReadAt`). The committer time of the newest first-parent commit on the default branch that (a) touched `LEDGER`, (b) was committed by the board-writer identity (`--ledger-writer <email>`; the job passes `319272446+assay-board-writer[bot]@users.noreply.github.com`) and (c) carries the trailer `Ledger-Full-Read: yes`. A commit failing (a), (b) or (c) never counts. A PR's squash, merge or rebase commit is committed by the forge's web-flow identity, never the board writer, so a trailer smuggled through a PR's commit messages cannot move the age. No header field is read for it.
   - **Field table.** The function's doc comment carries this table verbatim, and `spec.md` reproduces it (Task 7):

     | Field | Witnessed on read (re-read on failure) | Bounded by |
     |---|---|---|
     | `number`, `mergeSha`, `state: merged` | reachability + subject names `#N` (squash or merge-commit) | — |
     | `state: closed` | no first-parent subject names `#N` (squash or merge-commit methods only) | rebase-merged PRs: full-read age (4), `--verify-ledger` (5) |
     | `files` list / `"delivers"` | git change by merge shape; unmapped statuses → `reread` | — |
     | `files: "not-fetched"` | not a claim; fetched on demand | — |
     | `headSha` (merge-commit records) | must equal `M^2` | squash records: (4), (5) |
     | `briefTrailer`, `backfillRefs`, `mergedAt`, `updatedAt` | none, forge-only | (4), (5) |
     | record presence (a deleted record) | none (header count is corruption detection only) | (4), (5) |
     | `watermark` | upper bound: committer time of the newest ledger commit | (4) |
     | full-read age | git: writer-committed, ledger-touching, trailer-carrying commit | future time rejected |

     Every row is subject to layer (1), review-time detection, which becomes prevention once `lint` is a required check (Task 9).
5. **Hand-edit lint (rule-47 shape).** `--lint` reports a PROBLEM when the branch's diff against `refs/remotes/origin/main` adds, removes or modifies `LEDGER`, naming the board writer as its only writer. An unresolvable diff base (shallow clone, no `origin/main`) is a could-not-check NOTICE, as for the existing branch-diff gates, and never a silent pass. Wire it beside `checkReadmeTables`.
6. **The trailer decision belongs to statusgen.** `reconcile --record-ledger <path> --commit-message-file <f>` writes the full commit message: `chore(status): regenerate [skip-status-regen]`, plus a blank line and `Ledger-Full-Read: yes` only when this run did a full read. An incremental run (brief 09) writes the message with no trailer. The workflow never composes the trailer itself.
7. **Spec.** Name the ledger in `docs/streams/derived-board/spec.md` as a generated surface with one writer (the `regen` job, committing directly on the default branch), alongside the Briefs table. Reproduce Task 4's field table, and the five layers with their roles from the single-point-of-failure line.
8. **The writer, a staged workflow diff the driver pushes.** In `.github/workflows/assay-statusgen.yml`:
   - **`regen` job permissions:** `contents: read` + `pull-requests: read`. The forge read uses this read-only default `GITHUB_TOKEN`; the board-writer App token is minted only for the push, as today.
   - **`regen` checkout:** `fetch-depth: 0`. Task 4 makes a shallow clone unusable.
   - **One `regen_all` shell function** runs `statusgen --root .`, then the ledger write. That is `GITHUB_TOKEN=<read token> statusgen reconcile --backfill --root . --repo "$GITHUB_REPOSITORY" --record-ledger LEDGER --commit-message-file "$RUNNER_TEMP/msg" --ledger-writer <writer email>`. If that run exits non-zero, `git checkout -- LEDGER` restores the previous ledger and the message falls back to the plain STATUS message. Both the first attempt and the retry path call `regen_all`, so a re-sync regenerates both files and recomputes the trailer.
   - **Commit guard and commit:** `git add STATUS.md LEDGER`, the guard on `git status --porcelain -- STATUS.md LEDGER`, then `git commit -F "$RUNNER_TEMP/msg"`.
   - **`pull_request` trigger paths:** add `LEDGER`, so the Task 5 lint runs on a PR that touches only the ledger.
   - Until the diff lands no ledger exists, and brief 09 does the full read, today's behaviour.
9. **Driver hand-off, named in the PR body.** Push the Task 8 workflow diff. Add the `lint` check to the default branch's required status checks: that ruleset change is what turns layer (1) from detection into prevention. Until it is made, the single-point-of-failure line's "detection" wording stands.
10. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test -count=1 -run TestWitnessLedger ./statusgen/...` | exit 0 — round trip; deterministic bytes; open PRs never recorded; no head ref or body stored; a run with a FAILED read (an `unread` disclosure) writes nothing, while a run whose records are `"not-fetched"` writes them | check:ci |
| 2 | `go test -count=1 -run TestWitnessLedgerBackfillRefsExact ./statusgen/...` | exit 0 — property test over generated head refs, bodies and brief ids (NN of 2–3 digits, streams of 64 characters or fewer), including briefs added after recording: the read-time match equals `containsBackfillForm` on each original text separately; a run spanning ref and body never matches. Recording over the concatenation, or consulting the brief set, fails | check:ci +mutation |
| 3 | `go test -count=1 -run TestWitnessLedgerFoldParity ./statusgen/...` | exit 0 — on a recorded-response fixture, the fold over (ledger records + the open PRs) derives byte-identical `reconcile --backfill --json` to the full read, including a brief with two merged trailer PRs and an authoring-only PR | check:ci +flow |
| 4 | `go test -count=1 -run TestWitnessLedgerMergeShapes ./statusgen/...` | exit 0 — on a fixture repo, a squash-merged and a merge-commit-merged PR both witness clean with no `reread` (merge-commit change taken from `merge-base(M^1, M^2)..M^2` and `M^2 == headSha`). A merge-commit record with the wrong `headSha` goes to `reread`. Using `M^1..M` for a two-parent merge fails the clean case | check:ci +mutation |
| 5 | `go test -count=1 -run TestWitnessLedgerTamper ./statusgen/...` | exit 0 — an unreachable merge sha, a subject naming another PR, or an edited `files` list puts the record in `reread`; an unmapped forge status (`copied`) goes to `reread`; a header/record count mismatch (corruption) or a watermark later than the ledger commit makes the ledger unusable. Removing any one check fails its case | check:ci +mutation |
| 6 | `go test -count=1 -run TestWitnessLedgerClosedTamper ./statusgen/...` | exit 0 — a merged record rewritten as `state: closed` with empty `mergeSha`/`mergedAt` goes to `reread` for both a squash subject `(#N)` and a merge subject `Merge pull request #N`. A genuinely closed PR does not. Removing the closed-record check fails | check:ci +mutation |
| 7 | `go test -count=1 -run TestWitnessLedgerFullReadAge ./statusgen/...` | exit 0 — `fullReadAt` is the newest writer-committed, ledger-touching, trailer-carrying commit. A trailer on a commit that does not touch the ledger is ignored, as is a trailer on a ledger commit with a web-flow committer, and a later writer commit without the trailer (incremental) does not refresh the age. A future-dated full-read commit, or a shallow clone, makes the ledger unusable. Dropping the path condition or the committer condition fails | check:ci +mutation |
| 8 | `go test -count=1 -run TestRecordLedgerCommitMessage ./statusgen/...` | exit 0 — a full run's message ends with `Ledger-Full-Read: yes` and an incremental run's carries no trailer; both carry `[skip-status-regen]`. Writing the trailer unconditionally fails | check:ci +mutation |
| 9 | `go test -count=1 -run TestWitnessLedgerHandEditLint ./statusgen/...` | exit 0 — a branch whose diff against `origin/main` edits one ledger line makes `--lint` exit non-zero with a PROBLEM naming the ledger and its single writer; without the ledger change it lints clean; with the check unwired the edited branch lints clean and the test fails | check:ci +mutation |
| 10 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/sg" . && "${TMPDIR:-/tmp}/sg" reconcile --backfill --root .. --repo medici-finance/assay --record-ledger "${TMPDIR:-/tmp}/wl.jsonl" --commit-message-file "${TMPDIR:-/tmp}/msg" --json > /dev/null && head -1 "${TMPDIR:-/tmp}/wl.jsonl" && tail -1 "${TMPDIR:-/tmp}/msg"` | exit 0; the header names `witness-ledger-v1` and `medici-finance/assay`, and the message's last line is `Ledger-Full-Read: yes` (a live full read of this repository, GITHUB_TOKEN set) | check +dereference |
| 11 | `statusgen --root . --consumers --brief derived-board/08` | exit 0 (routing claims corroborated against the diff) | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
