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
  - "adopter regen workflows (commit the ledger with the generated READMEs): out-of-scope (the release note of the version carrying brief 09 names the one-line workflow change; adopters apply it through upgrade-assay)"
---

# Brief 08 — witness ledger

## Context
files:
- `statusgen/witnessledger.go` (planned) — record type, read, write, local verification
- `statusgen/witnessledger_test.go` (planned)
- `statusgen/reconcile.go` — the `--record-ledger <path>` flag (write side only)
- `statusgen/briefauthoring.go` — expose the per-PR authoring facts the ledger stores
- `docs/streams/derived-board/spec.md` — a short section naming the ledger as a generated, single-writer surface
- `changelog/derived-board-08-witness-ledger.md` (planned)

facts:
- Forge reads per `reconcile --backfill` run today (measured 2026-10-15, board of 1,936 PRs): `GET /pulls?state=all` pages (one per 100 PRs, plus a second identical walk — fixed separately), one `GET /pulls/{n}/files` per candidate PR (~250), `GET /issues?state=open` pages (~11). No comment or review endpoint is read by reconcile.
- What the fold reads from a pull (`ghfetch.go` `ghPull`): number, state, draft, body (for the single `Brief:` trailer, and the `--backfill` branch/body forms), merged_at, merge_commit_sha, head sha, head ref. From a file list (`briefauthoring.go`): only whether every path is an authoring path (stream README, brief file, changelog fragment) and which brief files it ADDS.
- Merged PRs are immutable except for a later edit of the body, which bumps the PR's `updated_at` (brief 09 re-reads anything whose `updated_at` passed the watermark).
- Of 255 briefs with a trailer PR on the measured board, 77 have more than one merged trailer PR (delivery plus rework/pin follow-ups); the ledger stores every one, never "the" PR.
- Precedent for a generated JSONL surface under `docs/streams/`: `docs/streams/verify-outcomes.jsonl`.

single-point-of-failure: the ledger's records being exactly what a full forge read would return — behind it: (1) every record re-verified against LOCAL git each run (merge sha present and reachable from the default branch; zero forge cost); (2) brief 09's delta read re-fetches any PR whose `updated_at` moved past the watermark; (3) brief 09's scheduled `--verify-ledger` full re-derivation fails on any difference; (4) a missing, unparseable or unverifiable ledger means a full read, never a partial one.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define the ledger: `docs/streams/witness-ledger.jsonl`, one header line (`{"schema":"witness-ledger-v1","repo":…,"watermark":<max updated_at seen>,"recordedAt":…}`) then one line per closed PR the fold could ever read as a witness — a merged or closed PR that carries exactly one `Brief:` trailer, OR whose head ref or body names any brief in a declared `--backfill` form. Per record: `number`, `state` (merged|closed), `mergeSha`, `headSha`, `headRef`, `mergedAt`, `updatedAt`, `briefTrailer` (or empty), `backfillRefs` (every `<stream>/<NN>` the backfill forms match in head ref + body, extracted by the SAME matcher `containsBackfillForm` uses), and the authoring facts: `authoringOnlyPaths` (the full path list when every changed path is an authoring path) or `delivers: true` (when any is not), or `files: "unread"` when the list could not be read or proved complete. Open PRs are NEVER recorded.
2. Sort records by number; write deterministically (same inputs → byte-identical file). No PR bodies are stored — only the extracted refs.
3. `reconcile --record-ledger <path>` (requires `--backfill`, online): after a successful run, write the ledger from the pulls and file lists the run already read. A run that could not look (any `unread`, any refusal) writes nothing and exits non-zero — a partial ledger is never written.
4. `readWitnessLedger(path, root)`: parse, then verify every record locally — the merge sha resolves (`git cat-file -e`) and is reachable from the default branch for `merged` records. Any failure → the whole ledger is reported unusable with the first failing record named (brief 09 then falls back to a full read). No forge call.
5. Name the ledger in `docs/streams/derived-board/spec.md` as a generated surface with one writer (the regen step that commits the generated READMEs), alongside the Briefs table.
6. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test -count=1 -run TestWitnessLedger ./statusgen/...` | exit 0 — round trip; deterministic bytes; open PRs never recorded; a run with any `unread` writes nothing | check:ci |
| 2 | `go test -count=1 -run TestWitnessLedgerFoldParity ./statusgen/...` | exit 0 — on a recorded-response fixture, the fold over (ledger records + the open PRs) derives byte-identical `reconcile --backfill --json` to the fold over the full pull and file-list read, including a brief with two merged trailer PRs and an authoring-only PR | check:ci +flow |
| 3 | `go test -count=1 -run TestWitnessLedgerTamper ./statusgen/...` | exit 0 — a record whose merge sha is absent from local git, or not reachable from the default branch, makes `readWitnessLedger` report the ledger unusable naming that PR; removing the reachability check makes the test fail | check:ci +mutation |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/sg" . && "${TMPDIR:-/tmp}/sg" reconcile --backfill --root .. --repo medici-finance/assay --record-ledger "${TMPDIR:-/tmp}/wl.jsonl" --json > /dev/null && head -1 "${TMPDIR:-/tmp}/wl.jsonl"` | exit 0; the header line names `witness-ledger-v1` and `medici-finance/assay` (a live read of this repository, GITHUB_TOKEN set) | check +dereference |
| 5 | `statusgen --root . --consumers --brief derived-board/08` | exit 0 (routing claims corroborated against the diff) | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
