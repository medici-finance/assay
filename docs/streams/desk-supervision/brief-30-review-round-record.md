---
brief: assay:assay:desk-supervision:30
title: Review round record — per-round timing, reviewer tier and finding transitions
why: >-
  The desk already counts review rounds per finding class, but it keeps no record of when each
  round happened, which strength of model reviewed it, or how many findings it opened, closed or
  saw disputed. Without that, nobody can ask "do PRs that need three rounds fail verify more
  often?", "does a strong-tier reviewer settle a class in fewer rounds?" or "how long does a
  worker take to answer a finding?". This brief derives one record per reviewer round from the
  review threads the desk already writes, and adds the one fact the thread lacks — the reviewer's
  tier — at the moment the review is posted.
wave: 1
depends: ["desk-supervision/28"]
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
  (b) correctness spans the deskpost write path (stamp composition, planted-stamp refusal, the
  forge-side idempotency digest), the deskkit fold that already counts rounds, and a new reviewloop
  reader; (c) a stamp that silently changes deskpost's duplicate-review detection survives every
  happy-path test.
domain: complicated
outcome: none
sources:
  - "docs/streams/desk-supervision/review-finding-v1.md — the record this brief extends (fields, derivation guarantees, Reactor consumption)"
  - "docs/streams/desk-supervision/brief-19-review-finding-continuity.md — delivered the ledger, the per-class round machine and `reviewloop plan --records`; this brief adds a transcript of that same fold, not a second counter"
  - "docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md — read for overlap: it governs review scope, records nothing per round; no overlap"
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models and statistical learning to the pipeline found review rounds cannot be joined to reviewer tier, timing or verify outcome"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): deskkit.ForgeRecord carries no timestamp and no reviewer tier (tools/desk/internal/deskkit/reviewfinding.go:340-370); the reviewloop payload record has no time field (tools/desk/cmd/reviewloop/findingcontinuity.go:34-43); deskpost review already reads the attested dispatch tier (tools/desk/cmd/deskpost/review.go:315-327) and discards it after the floor decision; no `review-round` string exists anywhere under tools/"
consumers:
  - "tools/desk/internal/deskkit/reviewfinding.go (ForgeRecord gains At and ReviewerTier; DeriveRoundRecords): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/reviewround.go (planned — the stamp composer/parser and RoundRecord type): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskpost/review.go (appends the stamp to the posted body; refuses a planted stamp; strips it in reviewBodyDigest): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/reviewloop/findingcontinuity.go and main.go (payload `at`; the `rounds` subcommand): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/desk-supervision/review-finding-v1.md (new Round records section): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (reviewloop and deskpost rows): follow-up desk-supervision/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskreply (worker replies): out-of-scope (a worker reply carries no reviewer tier; the reader ignores a stamp on a worker-role record, so deskreply needs no change)"
  - "tools/desk/cmd/reviewloop/plan.go (the existing ledger render): out-of-scope (rounds are a separate read-only subcommand; plan output stays byte-identical)"
  - "tools/desk/cmd/deskcalibrate (reversal-rate mining): out-of-scope (it keeps reading its own sampled verdicts; adopting round records as its input is a later analysis decision, not this brief)"
  - "assembling the --records payload from the forge: out-of-scope (no tool builds it today and reviewloop makes no forge call by contract; a payload builder is deferred to an unauthored brief)"
---

# Brief 30 — Review round record — per-round timing, reviewer tier and finding transitions

## Context
files:
- **add** `tools/desk/internal/deskkit/reviewround.go` (planned) — `RoundRecord`, `ReviewRoundStampV1`,
  `AppendReviewRoundStamp`, `ParseReviewRoundStamp`, `DeriveRoundRecords`.
- **add** `tools/desk/internal/deskkit/reviewround_test.go` (planned).
- **edit** `tools/desk/internal/deskkit/reviewfinding.go` — `ForgeRecord` gains `At string` and
  `ReviewerTier string`; the fold exposes per-record transitions to `DeriveRoundRecords` (no change
  to `DeriveLedger`'s result).
- **edit** `tools/desk/cmd/deskpost/review.go` + `review_test.go` — append the stamp; refuse a
  planted one; keep the forge-side duplicate check neutral to the stamp.
- **edit** `tools/desk/cmd/reviewloop/findingcontinuity.go`, `main.go` — optional `at` on each
  payload record; new read-only `rounds` subcommand.
- **add** `tools/desk/cmd/reviewloop/rounds_test.go` (planned), `tools/desk/cmd/reviewloop/testdata/rounds-thread.json` (planned).
- **edit** `docs/streams/desk-supervision/review-finding-v1.md` — a "Round records" section.
- **edit** `tools/desk/README.md` — reviewloop and deskpost rows.
- **add** `changelog/desk-supervision-30.md` (planned).

facts (2026-10-06 @ 1fbf1153f; re-read the named lines at pickup):
- The round machine already exists and is the only one: `deskkit.DeriveLedger`
  (`tools/desk/internal/deskkit/reviewfinding.go:609`) folds forge records in `Seq` order; a reviewer
  record completes a round only after an intervening worker response (`applyReviewer`, :800-866;
  the poll guard is `if !cs.pendingResponse`); a worker response arms the middle leg
  (`applyWorker`, :875-900). `RoundCap = 3` (:431). This brief must not add a second counter or
  change the cap.
- `ForgeRecord` (:340-370) carries Seq, Kind, Role, Actor, Head, Verdict, Lane, Block — no event
  time, no tier. The reviewloop payload record `FindingRecord`
  (`tools/desk/cmd/reviewloop/findingcontinuity.go:34-43`) likewise has no time field;
  `ReadRecords` (:65-98) lifts it and fails closed (exit 6) on an empty or malformed payload.
- `reviewloop` is read-only by contract: "spawns nothing, writes nothing, makes no GitHub call"
  (`tools/desk/README.md:34`). It is therefore NOT a writer; the only reviewer-side write boundary
  is `deskpost review`.
- `deskpost review` already reads the dispatcher-attested stamp for its model floor
  (`tools/desk/cmd/deskpost/review.go:315-327`, `deskkit.ModelCapabilityFloor` → `FloorDecision{Outcome, State, Stamp}`,
  `tools/desk/internal/deskkit/modelfloor.go:156-161`; `ModelStamp{Model, Tier}`,
  `modelstamp.go:179-182`). The tier vocabulary is `strong` / `any` (`modelfloor.go:166-175`).
- `deskpost review` already appends a tool-owned trailer to the POSTED body only, keeping the
  idempotency digest on the caller's body (`deskkit.AppendOnBehalfOf`, `review.go:190-197`,
  `principal.go:413`). Its forge-side duplicate check compares `reviewBodyDigest` of each existing
  review against the caller body's digest (`appReviewExistsAt`, `review.go:607`; `reviewBodyDigest`,
  :659-663).
- The forge records each review's `submitted_at` (`tools/desk/cmd/deskpost/github.go:494`;
  GraphQL `submittedAt`, `tools/desk/internal/deskkit/forge_github.go:784`) and each comment's
  `created_at`. These are the round timestamps; no tool-side clock is trusted for them.
- Join key: a round record names `repo`, `pr` and `head`; it joins to the deskpost audit row
  (`deskkit.Entry`: repo, pr, headSHA, sessionTag — `tools/desk/internal/deskkit/audit.go:66-86`)
  and through that to the dispatch record, `dispatch_ref` as defined by desk-supervision/28. The
  round record itself does not carry `dispatch_ref`: it is derived from forge records, which do not
  hold the claim id, and the forge thread of a public PR must not carry a local claim key.
- The public `dispatched-model:<slug>` label already exposes a model slug on a stamped PR; this
  record does NOT copy it — tier slugs only.
- single-point-of-failure: the stamp's provenance (only `deskpost review` writes it). Layers behind
  it: (1) deskpost refuses a caller body that already contains a stamp, so an agent cannot plant a
  tier; (2) the reader honours a stamp only on a reviewer-role record and only with a value from
  the closed vocabulary, else reports `unknown` and could-not-check; (3) the dispatch label
  timeline on the PR stays an independent cross-check (Verify row 9).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **The stamp (deskkit).** Add `ReviewRoundStampV1{ReviewerTier string}` rendered as an HTML
   comment `<!-- assay:review-round:v1 {"reviewerTier":"<tier>"} -->`, with
   `AppendReviewRoundStamp(body []byte, tier string) ([]byte, error)` and
   `ParseReviewRoundStamp(body string) (tier string, present bool, err error)`. Closed value set:
   `strong`, `any`, `unattested`. Anything else — including any model slug — is an error from
   both functions.
2. **The writer (deskpost review).** After the model-floor decision, compute the tier:
   `fd.Stamp.Tier` when `fd.State == deskkit.ModelStamped`, else `unattested`. Append the stamp to
   the POSTED body after the on-behalf-of trailer, so the posted body is, in order: the caller body,
   the on-behalf-of trailer exactly as `deskkit.AppendOnBehalfOf` renders it today, then a newline
   and the stamp as the LAST line of the body; `dig` and every idempotency key stay keyed on
   the caller body. Before any network call, refuse (exit 5, naming the marker) a caller body that
   already contains `assay:review-round:v1`. Make `appReviewExistsAt`'s comparison strip a trailing
   stamp from each existing review body before digesting: remove the final line only when it is
   byte-exactly the stamp shape of step 1 (one comment, one `reviewerTier` key, a value from the
   closed set) and preceded by a newline, and nothing else. Everything the digest compares after
   that — the on-behalf-of trailer included — is untouched, so the duplicate-detection outcome for
   every input is exactly what it is today.
3. **The record type and fold (deskkit).** `ForgeRecord` gains `At string` (RFC3339 forge event
   time) and `ReviewerTier string`. Add `DeriveRoundRecords(records []ForgeRecord) ([]RoundRecord, []string)`
   that runs the same fold as `DeriveLedger` (share the code path; do not re-implement the round
   rules) and emits one `RoundRecord` per (reviewer record × lane-scoped class that record touches):
   - `schema: "review-round/v1"`, `lane`, `class` (bare), `reviewSeq`, `head`, `reviewedAt`,
     `responseAt` (time of the latest worker record that armed this round; empty on an opening
     verdict), `reviewerTier`;
   - `roundAfter` (the class's round count after this record), `completedRound` (true only when
     the fold counted a round here), `opening` (true on the class's opening verdict), `poll` (true
     when the fold treated it as a poll), `held` (class at the cap after this record);
   - transition COUNTS for the class at this record: `opened`, `resolved`, `fixedSinceLast`,
     `disputedSinceLast`, `promoted` (advisory→blocking), `stillOpenBlocking`.
   `reviewerTier` is the parsed stamp on a reviewer-role record; a reviewer record without a
   stamp reads `unknown`; a stamp on a worker-role record is ignored and reported in the second
   return value (could-not-check notes). A record with no `At` gets empty time fields and a
   could-not-check note — never a zero time. `DeriveLedger`'s output must stay unchanged.
4. **Never collected.** A `RoundRecord` carries no finding `failure` / `explanation` /
   `resolution` / `evidence` text, no body text or digest of prose, no finding IDs, and no actor
   login — counts per class and per round only. Tier slugs only, never a model slug. These records
   are for aggregate analysis per class, tier and PR; they are never used to rank a person or an
   agent — state this in the docs section.
5. **The reader (reviewloop).** `FindingRecord` gains optional `at`; `ReadRecords` lifts it and
   parses the stamp from `Body` into `ReviewerTier`. The reader honours ONLY the stamp that is the
   final line of the body, byte-exact to step 1's shape; a stamp-shaped comment earlier in a body
   (planted in findings prose, or a variant with extra keys, other whitespace or a second stamp) is
   ignored and the record reads `unknown` with a could-not-check note, so a planted variant can
   never win over the real trailing one. Add `reviewloop rounds --records <thread.json>`:
   prints one JSON line per `RoundRecord` (with `repo` and `pr` from the payload) to stdout, the
   could-not-check notes to stderr, exit 6 on an empty/malformed payload (reuse `ReadRecords`). It
   stays read-only and makes no forge call. `plan` is untouched.
6. **Docs + changelog.** review-finding-v1.md: a "Round records" section (stamp format, the
   closed tier set, the record fields, the never-collected list, the join to the audit row and
   desk-supervision/28, that the forge thread is the store — no separate journal). README rows for
   reviewloop (`rounds`) and deskpost (the stamp). Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/reviewloop/... ./cmd/deskpost/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDeriveRoundRecords_Transcript$' -v > "${TMPDIR:-/tmp}/b30-transcript.out" 2>&1 && grep -F -e '--- PASS: TestDeriveRoundRecords_Transcript' "${TMPDIR:-/tmp}/b30-transcript.out"` | exit 0; subtests: opening verdict → `opening` true, `roundAfter` 0; review→response→re-review → `completedRound` true and `responseAt` set; a re-review with no response → `poll` true, `roundAfter` unchanged; a fourth round → `held` true; `DeriveLedger` on the same records yields the same `Rounds` and `Held` as before this change | check:ci |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDeriveRoundRecords_NoProseNoLogin$' -v > "${TMPDIR:-/tmp}/b30-noprose.out" 2>&1 && grep -F -e '--- PASS: TestDeriveRoundRecords_NoProseNoLogin' "${TMPDIR:-/tmp}/b30-noprose.out"` | exit 0; the marshalled records for a fixture whose findings carry distinctive failure/resolution/evidence strings, IDs and an actor login contain none of those strings | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskpost/ -run '^TestReviewRoundStamp_Writer$' -v > "${TMPDIR:-/tmp}/b30-writer.out" 2>&1 && grep -F -e '--- PASS: TestReviewRoundStamp_Writer' "${TMPDIR:-/tmp}/b30-writer.out"` | exit 0; subtests: attested `strong` stamp → posted body ends with the `strong` stamp; no attestation → `unattested`; caller body containing `assay:review-round:v1` → refused exit 5 with zero HTTP calls; the posted body never contains the model slug | check:ci +mutation |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReviewRoundStamp_ReaderTrust$' -v > "${TMPDIR:-/tmp}/b30-reader.out" 2>&1 && grep -F -e '--- PASS: TestReviewRoundStamp_ReaderTrust' "${TMPDIR:-/tmp}/b30-reader.out"` | exit 0; subtests: stamp on a worker-role record → ignored, could-not-check note; stamp value outside the closed set (a model slug) → `unknown` + note; reviewer record without stamp → `unknown`; a valid-looking stamp earlier in the body followed by prose, or a trailing stamp with an extra key or altered whitespace → `unknown` + note; two stamps → `unknown` + note; record without `At` → empty times + note | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskpost/ -run '^TestAppReviewExistsAt_StampNeutral$' -v > "${TMPDIR:-/tmp}/b30-dedup.out" 2>&1 && grep -F -e '--- PASS: TestAppReviewExistsAt_StampNeutral' "${TMPDIR:-/tmp}/b30-dedup.out"` | exit 0; for each case in the existing appReviewExistsAt table, the (dup, why) result is identical whether or not the existing review body carries a trailing stamp, and the table includes a case whose existing body ends with the real on-behalf-of trailer followed by the stamp (the shape row 4 proves deskpost posts), so the strip is exercised on the production layout and the identity cannot hold vacuously; a stamp-shaped line that is NOT the last line is left in place and changes the digest | check:ci +neighbour |
| 7 | `cd tools/desk && go test ./cmd/reviewloop/ -run '^TestRoundsFlow_StampToRecord$' -v > "${TMPDIR:-/tmp}/b30-flow.out" 2>&1 && grep -F -e '--- PASS: TestRoundsFlow_StampToRecord' "${TMPDIR:-/tmp}/b30-flow.out"` | exit 0; a reviewer body built with `deskkit.AppendReviewRoundStamp` (the function row 4 proves deskpost calls) goes through `ReadRecords` and `DeriveRoundRecords` and comes out with that `reviewerTier`; the fixture `testdata/rounds-thread.json` bodies equal the composer's output | check:ci +flow |
| 8 | `cd tools/desk && go build -o "${TMPDIR:-/tmp}/b30-reviewloop" ./cmd/reviewloop && "${TMPDIR:-/tmp}/b30-reviewloop" rounds --records cmd/reviewloop/testdata/rounds-thread.json > "${TMPDIR:-/tmp}/b30-rounds.jsonl" && jq -e -s 'map(select(.schema == "review-round/v1" and .reviewerTier == "strong" and .completedRound == true)) \| length >= 1' "${TMPDIR:-/tmp}/b30-rounds.jsonl"` | exit 0; prints `true` | check +flow |
| 9 | `for n in $(gh pr list -R medici-finance/assay --state merged --limit 40 --json number --jq '.[].number'); do gh api "repos/medici-finance/assay/pulls/$n/reviews" --jq '.[] \| select(.body \| contains("assay:review-round:v1")) \| "\(.commit_id) \(.submitted_at)"' \| sed "s/^/$n /"; done` (on main, after deskpost with this change has posted reviews) | at least one line; for one printed PR, the stamped `reviewerTier` in that review's body equals the `dispatched-tier:` label the PR carried at that `submitted_at` (`gh api repos/medici-finance/assay/issues/<that PR>/timeline`), or is `unattested` when no dispatcher-applied tier label was present | gate:model +dereference |
| 10 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A second round counter is written that disagrees with the ledger | row 2 (DeriveLedger result unchanged; transcript rounds equal ledger rounds) |
| Polls are recorded as rounds, inflating round counts | row 2 (poll subtest) |
| Finding prose or a reviewer login leaks into the analysis record | row 3 |
| An agent plants `"reviewerTier":"strong"` in its own review body | row 4 (planted stamp refused) + row 5 (reader rejects off-vocabulary values) |
| A vendor model name lands in a public review body | row 4 (no model slug in posted body) + row 5 |
| The appended stamp breaks deskpost's duplicate-review detection | row 6 |
| deskpost writes the stamp but the reader parses a different shape, so every tier reads `unknown` | row 7 + row 8 |
| The stamped tier is not the tier the dispatcher attested | row 9 (dereferenced against the PR's label timeline) |
| Missing forge times silently become zero times and poison latency analysis | row 5 (`At` absent subtest) |
| The `--records` payload is never assembled in practice, so no round records are ever produced | no row — recorded out-of-scope in `consumers:`; a payload builder is a separate brief |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
