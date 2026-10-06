---
brief: assay:assay:desk-supervision:31
title: Persist CI check results the desk tools already read
why: >-
  The desk tools read every pull request's CI results many times a day to decide what to flip,
  review or report, then throw them away. So no one can later ask "which checks flake, how often,
  and how long do they take?" or "did a PR that went red twice also fail verify?". Keeping one
  small local line per finished check run, taken from reads the tools already make, answers those
  questions at no extra forge cost.
wave: 0
depends: []
unblocks: ["desk-supervision/35"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (pipeline analysis records)
exec-tier: strong
exec-tier-why: >-
  (b) one decorator change reaches every desk tool that reads CI, and the attempt key must agree
  between two different forge read shapes (REST check-runs and the GraphQL rollup); (c) the
  recorder sits on the read path of flip and board decisions, so a recorder that blocks, panics or
  alters the returned value would change a gate verdict while every happy-path test stays green.
domain: complicated
outcome: none
sources:
  - "docs/streams/desk-supervision/README.md — the records-for-later-analysis set (desk-supervision/28–35); this brief is the CI-check-history record"
  - "research note (driver-held, 2026-10-06): an independent review of the pipeline's records found CI results are read but never kept, so per-check flake rate and duration cannot be computed, and CI history cannot be joined to review rounds or verify outcomes"
  - "docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md — the join-key convention (repo + PR + head SHA) this record shares"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): no CI-result persistence exists — every check read in tools/desk goes through deskkit Forge.ChecksAtHead (tools/desk/internal/deskkit/forge.go:1499) or the rollup carried on ListOpenChanges / ReviewQueueSnapshot (forge.go:724-760, forge_github.go:692, :719, :827-858), and the results are only reduced to in-memory verdicts"
consumers:
  - "tools/desk/internal/deskkit/cicheckhistory.go (planned — record type, writer, reader): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/outboundforge.go (the single Forge decorator gains three recording read overrides; header classification comment): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/forge.go (RollupNode gains ID): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/forge_github.go (both rollup queries select databaseId; ghOpenChange maps it): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/outbound_test.go (obClass table, if it must name the overridden reads): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/desk-supervision/ci-check-v1.md (planned — canonical record schema): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (a short 'CI check history' paragraph linking the schema): follow-up desk-supervision/31 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/records-and-retention.md (lists the new record): follow-up desk-supervision/35"
  - "tools/desk/cmd/deskflip, deskboard, deskpost, deskautolane, deskmonitor, deskroster, deskdisposition (the CI readers): out-of-scope (they read through the Forge that ResolveForge returns, so they gain recording without a code change; their returned values are unchanged — Verify row 5 proves it)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: out-of-scope (its ChecksAtHead and rollup already populate the fields the record needs; a GitLab CheckRun with no ID falls back to the timestamp attempt key defined in Task 2)"
  - "statusgen/autonomy.go (shells out to `gh pr view --json statusCheckRollup` over merged PRs): out-of-scope (a separate binary outside the desk Forge seam, reading a retrospective window; adding a second writer there would split the record — deferred to an unauthored follow-up if its window proves useful)"
---

# Brief 31 — Persist CI check results the desk tools already read

## Context
files:
- **add** `tools/desk/internal/deskkit/cicheckhistory.go` (planned) + `cicheckhistory_test.go` (planned) — the `CICheckRecord` type, the writer `recordCIChecks`, the reader `LoadCIChecks`.
- **edit** `tools/desk/internal/deskkit/outboundforge.go` — `*outboundForge` overrides `ChecksAtHead`, `ListOpenChanges`, `ReviewQueueSnapshot`: delegate, then record on success. Update the header's classification comment.
- **edit** `tools/desk/internal/deskkit/forge.go` — `RollupNode` gains `ID string` (same meaning and rendering as `CheckRun.ID`, via `checkRunID`).
- **edit** `tools/desk/internal/deskkit/forge_github.go` — add `databaseId` to the `...on CheckRun{…}` selection of `ghOpenChangesQuery` and `ghReviewQueueQuery`; decode and map it in `ghOpenChange`.
- **edit** `tools/desk/internal/deskkit/outbound_test.go` — only if `obClass` must name an overridden read.
- **add** `docs/streams/desk-supervision/ci-check-v1.md` (planned) — the canonical record schema.
- **edit** `tools/desk/README.md` — one paragraph: what is recorded, where, and the schema link.
- **add** `changelog/desk-supervision-31.md` (planned).

facts:
- One construction site: `deskkit.ResolveForge` (`tools/desk/internal/deskkit/forgeresolve.go:743-775`) returns every backend wrapped by `OutboundChecked` (`outboundforge.go:39`); `TestForgeSingleConstructionSite` (`forgeresolve_test.go:285`) pins it and the forgeban rule stops cmd packages from building a backend. The decorator is therefore the ONE choke point every desk reader passes through.
- CI reads in the desk tools, all through that Forge (checked 2026-10-06 @ 1fbf1153f):
  `ChecksAtHead` — deskflip `checksAtHeadOnce` (`tools/desk/cmd/deskflip/flip.go:1761`), deskpost `combinedStatusAt`/`checkRunsAt` (`tools/desk/cmd/deskpost/forgeclient.go:257-288`), deskautolane (`tools/desk/cmd/deskautolane/lane.go:373`, `:885`), deskboard health and zero-CI (`tools/desk/cmd/deskboard/health.go:171`, `zeroci.go:239`);
  rollup on `ListOpenChanges` / `ReviewQueueSnapshot` — deskboard (`board.go:207`, `:282`), deskmonitor (`pr.go:100`), deskroster (`roster.go:304`), deskdisposition (`verbs.go:257`).
- Shapes: `ChecksAtHead` (`forge.go:388`) carries `Statuses []StatusContext` (`forge.go:343`: State, Context, CreatedAt) AND `CheckRuns []CheckRun` (`forge.go:355`: ID, Name, Status, Conclusion, StartedAt, CompletedAt). `RollupNode` (`forge.go:724`) unions both (Typename `CheckRun` | `StatusContext`) and today has NO ID; `OpenChange` (`forge.go:744`) carries Number and HeadSHA for the rollup.
- A commit-status gate (any CI that posts a commit status rather than a check run — e.g. a required external scanner) arrives ONLY as a `StatusContext`; a record that keeps check runs alone misses it.
- `CheckRun.ID` is per EXECUTION: a re-run of the same named check is a new ID (`forge.go:356-364`). GitHub's combined-status endpoint returns the latest status per context, so for a status the only per-execution handle is `CreatedAt`.
- Local state dir: `deskDir()` (`tools/desk/internal/deskkit/killswitch.go:65`) = `~/.config/assay/` (or the test override); the audit log is `audit.jsonl` there with daily rotation `rotateIfNeeded` (`audit.go:172`) and `segmentPattern` (`audit.go:101`) matching only `audit.jsonl.<date>` — a new `ci-checks.jsonl` cannot be swept into audit segments.
- `dispatch_ref` as defined by desk-supervision/28 is NOT known at a CI read; this record joins by `repo` + `head_sha` (+ `pr` when the read carried one).
- single-point-of-failure: the decorator override — if a future change returns a Forge that bypasses `OutboundChecked`, recording silently stops. Backed by `TestForgeSingleConstructionSite` (an independent static test of the construction site) and by Verify row 6 (a real tool run must leave records).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Schema doc** `ci-check-v1.md`: one JSON object per line, fields exactly —
   `schema` (`"ci-check-v1"`), `observed_at` (RFC3339 UTC, when the tool read it), `tool`
   (`CanonicalToolKeyOr` of the running binary), `repo` (`owner/name`), `head_sha`, `pr` (int,
   omitted when the read carried no PR), `kind` (`check-run` | `status`), `name` (check name or
   status context), `attempt`, `conclusion` (lowercased forge value), `started_at`,
   `completed_at` (RFC3339 or omitted), `duration_s` (int, check-run only, omitted when either
   stamp is missing). State the dedupe key `(repo, head_sha, kind, name, attempt)`, the
   never-collected list (step 6), retention (step 5) and that it is local operational state, never
   committed.
2. **Attempt key.** check-run: `CheckRun.ID` / `RollupNode.ID` when non-empty, else
   `t:<started_at>`; status: `t:<created_at>`. An entry with no usable key is not recorded. Add
   `databaseId` to the CheckRun fragment of both GraphQL rollup queries and map it through
   `checkRunID` into `RollupNode.ID`, so the REST and GraphQL reads of one run produce the SAME key.
   This adds a field to an existing request, never a request.
3. **Only terminal results.** Record a check-run only when `status == completed`; a status only
   when `state` is `success`, `failure` or `error`. Pending/queued/expected entries and the GitLab
   `GitLabRollupUnmapped` sentinel are skipped (they would later conflict with the same key).
4. **Recorder in the decorator.** `*outboundForge` overrides `ChecksAtHead(repo, sha)`,
   `ListOpenChanges(repo)` and `ReviewQueueSnapshot(repo)`: call the inner method, and ONLY when
   it returns no error, hand the entries to `recordCIChecks` (head SHA = `sha` for ChecksAtHead,
   `OpenChange.HeadSHA` + `Number` for rollups). The overrides return the inner result and error
   UNCHANGED — the same pointer, never a copy — and recording is best-effort: a write failure,
   lock timeout or panic inside the recorder (recovered) never changes the return value or adds
   latency beyond a bounded lock wait (≤ 2 s, then the batch is dropped with one stderr line per
   process). The recorder makes NO forge call.
5. **Writer + dedupe (two layers).** `recordCIChecks` appends to `<deskDir>/ci-checks.jsonl`
   (dir 0700, file 0600) under its own lock file `ci-checks.lock`; it skips keys already written by
   this process (in-memory set) or present in the live file (read under the lock), and rotates the
   live file daily to `ci-checks.jsonl.<YYYY-MM-DD>` by the same rule as `rotateIfNeeded`. The tools
   never delete segments (retention = the adopter's; say so in the schema doc). `LoadCIChecks`
   reads all segments and dedupes again on the key, so a cross-day or cross-process duplicate on
   disk never reaches an analysis.
6. **Never collected** (enforced by the record type having no such field): check output, title,
   summary, annotations, log text, details/target URLs, the actor or app that posted it, any
   session transcript; no per-person field at all. Names and conclusions only.
7. **Docs + changelog.** `tools/desk/README.md` paragraph; changelog fragment (bullets: the record,
   its location, that it costs no extra forge reads).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCICheckHistory_RecordsBothKinds$' -v > "${TMPDIR:-/tmp}/b31-kinds.out" 2>&1 && grep -F -e '--- PASS: TestCICheckHistory_RecordsBothKinds' "${TMPDIR:-/tmp}/b31-kinds.out"` | exit 0; a fake inner Forge serves one completed check run, one queued check run, one `success` status context and one `pending` status at one head; exactly two records land, one `kind:"check-run"` with `duration_s` and one `kind:"status"` | check:ci |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCICheckHistory_Dedupe$' -v > "${TMPDIR:-/tmp}/b31-dedupe.out" 2>&1 && grep -F -e '--- PASS: TestCICheckHistory_Dedupe' "${TMPDIR:-/tmp}/b31-dedupe.out"` | exit 0; subtests: the same run read twice by ChecksAtHead → one line; read once by ChecksAtHead and once via the ListOpenChanges rollup → one line (same attempt key); a re-run (new ID, same name) → two lines; a duplicate written across a day rotation → `LoadCIChecks` returns one | check:ci +mutation |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCICheckHistory_NoExtraForgeReads$' -v > "${TMPDIR:-/tmp}/b31-reads.out" 2>&1 && grep -F -e '--- PASS: TestCICheckHistory_NoExtraForgeReads' "${TMPDIR:-/tmp}/b31-reads.out"` | exit 0; a `GitHubForge` against an `httptest` server is called through `OutboundChecked` for ChecksAtHead, ListOpenChanges and ReviewQueueSnapshot; the server's request count is identical with the recorder enabled and with the state dir unwritable, and records land in the enabled case | check:ci +mutation |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCICheckHistory_ReadUnchangedOnRecorderFailure$' -v > "${TMPDIR:-/tmp}/b31-fail.out" 2>&1 && grep -F -e '--- PASS: TestCICheckHistory_ReadUnchangedOnRecorderFailure' "${TMPDIR:-/tmp}/b31-fail.out"` | exit 0; subtests: unwritable state dir, held lock, recorder panic → each override returns the inner result pointer and error unchanged; an inner read error → no record and the same error returned | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestFlip_LeavesCICheckRecords$' -v > "${TMPDIR:-/tmp}/b31-flow.out" 2>&1 && grep -F -e '--- PASS: TestFlip_LeavesCICheckRecords' "${TMPDIR:-/tmp}/b31-flow.out"` | exit 0; a deskflip run whose Forge comes from the production construction path (`OutboundChecked` over a fake inner backend, state dir overridden) leaves records for the head it evaluated, and its verdict equals the verdict with recording disabled | check:ci +flow |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCICheckRecord_NoFreeTextFields$' -v > "${TMPDIR:-/tmp}/b31-fields.out" 2>&1 && grep -F -e '--- PASS: TestCICheckRecord_NoFreeTextFields' "${TMPDIR:-/tmp}/b31-fields.out"` | exit 0; reflection over `CICheckRecord`'s JSON tags equals exactly the field list in `ci-check-v1.md` (the test reads the doc's field table), so adding an output/URL/actor field fails | check:ci +mutation |
| 8 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -v > "${TMPDIR:-/tmp}/b31-site.out" 2>&1 && grep -F -e '--- PASS: TestForgeSingleConstructionSite' "${TMPDIR:-/tmp}/b31-site.out"` | exit 0 (the choke point the recorder relies on is still the only construction site) | check:ci +neighbour |
| 9 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |
| 10 | `jq -r 'select(.kind=="check-run" and (.attempt\|test("^[0-9]+$"))) \| "\(.repo) \(.attempt) \(.conclusion)"' ~/.config/assay/ci-checks.jsonl` (on a desk machine after one `deskboard` run on the built binary) | at least one line; for one printed `<repo> <attempt> <conclusion>` triple, `gh api repos/<repo>/check-runs/<attempt> --jq .conclusion` prints that same conclusion and `--jq .head_sha` equals the record's `head_sha` | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Commit-status gates are silently dropped (only check runs recorded) | row 2 |
| REST and GraphQL reads of one run produce different keys, doubling every record | row 3 (cross-shape subtest) |
| A pending run is recorded, then its completed result is deduped away | row 2 (pending skipped) + row 3 |
| Recording adds a forge call (e.g. fetching run details to fill duration) | row 4 |
| A full disk or held lock makes a flip/board verdict fail or change | row 5 + row 6 |
| A reader path bypasses the decorator, so nothing is ever recorded | row 6 (real flip path) + row 8 |
| Someone later adds a URL, output or actor field | row 7 |
| Records claim a conclusion the forge never reported | row 10 |
| A recorded check name itself carries sensitive text | review-only — names are repo CI configuration, already visible to anyone who can read the PR; the file is local 0600 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
