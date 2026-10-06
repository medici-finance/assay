---
brief: assay:assay:desk-supervision:33
title: Intake exit record — every triaged item lands one structured disposition
why: >-
  The front-door desk decides, for every incoming issue or idea, which of five ways it leaves
  (becomes a brief, a bug, a finding, a human decision, or is rejected/watched). That is the
  cheapest, most repetitive judgment the pipeline makes, so it is the first one worth handing to
  a cheaper model, but only once we can measure how the current desk decides. Today the judged
  exits are written nowhere: the scan loop records only the mechanical exits as a flattened
  audit string, the judgment items are parked and routed by hand, and the intake register this
  repo is told to keep holds zero entries and no triage stamp. This brief makes each triage leave
  one small, text-free record (what came in, which exit, who decided at what tier, when, and
  what it became) in one schema, from both the issue lane and the idea register.
wave: 0
depends: []
unblocks: ["desk-supervision/35"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (pipeline analysis records)
exec-tier: strong
exec-tier-why: >-
  (b) one record schema is written by two writers in two Go modules (statusgen for the in-git
  intake register, scanloop for the issue lane) and named by the canonical spec and the
  intake-desk skill; a writer that drifts from the schema, or that copies a title or an author
  login into a record, passes its own happy-path test.
domain: complicated
outcome: none
sources:
  - "spec/registers-v1.md §5 (INTAKE register, lines 243-273 @ 1fbf1153f) — the mandated per-entry disposition field this brief extends with a triage stamp"
  - "plugins/assay/skills/intake-desk/SKILL.md — the five tracked exits table (lines 31-42) and the intake-lane four triage exits (lines 408-433)"
  - "tools/desk/cmd/scanloop/land.go — the closed five-exit set and the per-pass ExitLedger (lines 22-41, 121-200)"
  - "docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md — sibling record brief (join-key shape, tier-slug rule, refusal convention)"
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models to the pipeline named the five-exit triage call as the first shadow target for a cheap decision model, and found no labelled record of it"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): docs/streams/intake/ does not exist in this repo and docs/streams/INTAKE.md reads 'No intake entries yet'; intakeEntry (statusgen/registerentries.go:24-41) has no triage-time, triager or tier key; scanloop's production run wires no Feeder, so every judgment item is parked (adapter.go:466-467, run.go:267-272) and never reaches Land"
consumers:
  - "spec/registers-v1.md (§5.2 format, §5.3 rules; new §5.4 exit mapping): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/registerentries.go (intakeEntry gains three optional keys; the strict key set is derived from the struct): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/main.go (the --intake-exits flag): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/scanloop/land.go and adapter.go (record writer called from Land): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/scanloop/main.go (the land verb and usage text): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/intake-desk/SKILL.md (exits table naming, triage stamp, land verb step): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (scanloop section): follow-up desk-supervision/33 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/records-and-retention.md (lists the new record and its retention): follow-up desk-supervision/35 (the records page brief owns every row of that table)"
  - "statusgen/intake_alarm.go (isUntriagedDisposition, the intake-debt line): out-of-scope (it reads only `disposition`; the three new keys are additive and do not change which entries count as untriaged — Verify row 3 keeps a neighbour case)"
  - "statusgen/graph.go (intake nodes): out-of-scope (graph nodes carry id and title only; no exit or triage field is added to the graph)"
  - "the deskkit audit log's existing `scanloop land` line (auditExit): out-of-scope (kept unchanged for the rate/idempotency readers that replay it; the structured record is written beside it, not in place of it)"
---

# Brief 33 — Intake exit record — every triaged item lands one structured disposition

## Context
files:
- **edit** `spec/registers-v1.md` — §5.2: three OPTIONAL frontmatter keys on an intake entry
  (`triaged`, `triaged-by`, `triager-tier`); new §5.4 "Exit record": the disposition → exit
  mapping table and a pointer to the record schema doc.
- **add** `docs/streams/desk-supervision/intake-exit-v1.md` (planned) — the record schema: fields
  table, closed vocabularies, the never-recorded list, where each writer puts records.
- **edit** `statusgen/registerentries.go` (+ test) — `intakeEntry` gains `Triaged`, `TriagedBy`,
  `TriagerTier`; value checks when present.
- **add** `statusgen/intakeexits.go` (planned) + `statusgen/intakeexits_test.go` (planned) — the
  disposition → exit mapping and the `--intake-exits --json` export.
- **edit** `statusgen/main.go` — register the `--intake-exits` flag.
- **edit** `tools/desk/cmd/scanloop/land.go`, `adapter.go` (+ tests) — build and append an
  `intake-exit-v1` record in `Land`, beside the existing `auditExit` line.
- **add** `tools/desk/cmd/scanloop/exitrecord.go` (planned) + `exitrecord_test.go` (planned) — the
  record type, validation and append; `landverb.go` (planned) + `landverb_test.go` (planned) — the
  `scanloop land` verb for judgment exits.
- **edit** `tools/desk/cmd/scanloop/main.go` — dispatch and usage for `land`.
- **edit** `plugins/assay/skills/intake-desk/SKILL.md` — the triage steps (lines 31-42 table,
  408-433 intake lane): stamp the intake entry; run `scanloop land` after routing a judgment item.
- **edit** `tools/desk/README.md` — scanloop section: the `land` verb and the record file.
- **add** `changelog/desk-supervision-33.md` (planned).

facts:
- Five exits, code spelling (the closed set the record uses): `placeholder`, `bug`, `finding`,
  `needs-decision`, `rejected-watching` — `trackedExits`, `tools/desk/cmd/scanloop/land.go:41`
  @ 1fbf1153f. The skill's table spells them `spec/brief · bug/issue · finding · needs-decision ·
  rejected/watching` (SKILL.md:33-39); `placeholder` is the spec/brief exit. The record uses the
  code spelling; the skill table gains the code slug beside each name.
- scanloop `Land` (adapter.go:495-527) resolves ONE exit via `ExitOf` (land.go:96-119, two
  disagreeing exits → `deskkit.Refused`, exit 5), records it in the per-pass `ExitLedger`
  (land.go:141-155, in memory only), then writes `deskkit.Entry{Tool:"scanloop", Verb:"land",
  Result:<exit>, Detail:"<lane> -> <artifact>"}` (land.go:192-200). No tier, no source, no
  opened/triaged timing, no decided-by; the artifact is inside a free string.
- Judgment items never reach `Land` in production: `Dispatch` returns `awaitingRoutingError` when
  `Feeder` is nil (adapter.go:466-467), `run` parks the item (run.go:267-272), and only tests set a
  `Feeder` (batch_test.go:301, drainpass_test.go:118). So the five-way JUDGMENT — the label a
  decision model needs — is the one exit never recorded. Mechanical exits are computed by
  `classify` (adapter.go:303-326: a new issue → scan-carrier lane → `placeholder`).
- `judgmentItem` payload carries `author` and `trust` (adapter.go:330-344). `author` is a person's
  login and MUST NOT enter the record; `trust` (the admission state) may.
- The audit log and the desk state dir: `deskDir()` = `$HOME/.config/assay` (or the test override),
  `tools/desk/internal/deskkit/killswitch.go:65-80`; audit at `<deskDir>/audit.jsonl`
  (audit.go `auditPath`). The issue-lane record file is `<deskDir>/intake-exits.jsonl`.
- Intake register: per-entry files under `docs/streams/intake/` (flat or the five subdirs
  `new/ decision-needed/ watching/ completed/ rejected/`, registerentries.go:43-50). Frontmatter keys
  are STRICT — derived by reflection from `intakeEntry` (registerentries.go:242-245, enforced at
  :328), so a new key must be a struct field or it is a parse error. Disposition values are
  `new | decision-needed | watching | scoped | rejected` (+ legacy `adopted`), with the target in
  `scoped-to` (registerentries.go:24-41, 58-73).
- This repo has NO `docs/streams/intake/` directory; `docs/streams/INTAKE.md` holds no entries
  (checked 2026-10-06 @ 1fbf1153f). The register half of this brief is therefore exercised on
  fixtures here and on real entries only in an adopter tree that keeps one.
- Join keys: the issue-lane record carries `session_tag` (the same value as the audit Entry's
  `sessionTag`) and an optional `dispatch_ref` as defined by desk-supervision/28. A record that
  cannot know `dispatch_ref` joins by `item` (`owner/repo#N`) instead.
- single-point-of-failure: the record writer's closed-key validation (no title, body, author or
  vendor model name can enter a record) — backed by the schema-match test in BOTH modules (row 6),
  which fails on any added JSON key, and by the review gate on the implementation diff.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Schema doc** (`intake-exit-v1.md`). One JSON object per triaged item, keys exactly:
   - `schema` = `"intake-exit-v1"`;
   - `source` — `issue` | `intake`;
   - `item` — `owner/repo#N` (issue) or `I-<slug>` (intake); `repo` — `owner/repo`;
   - `exit` — one of the five code slugs above; `detail` — `rejected` | `watching`, present only
     when `exit` is `rejected-watching`;
   - `artifact` — a typed ref only: `<stream>/<NN>`, `owner/repo#N`, `#N`, `F-<slug>`, or a scan PR
     ref; REQUIRED unless `exit` is `rejected-watching`;
   - `decided_by` — `mechanical` | `judgment`;
   - `triager_role` — a roster role slug (e.g. `issue-loop`), never a person;
   - `triager_tier` — `any` | `strong` for `judgment`, `none` for `mechanical`; never a vendor
     model name (the public stamp label is the only place a model name appears, and this record
     does not repeat it);
   - `opened`, `triaged` — RFC3339 (`opened` <= `triaged`); `opened` may be empty when unknown;
   - `kind` — the classifier reason (`new-issue`, `update`, …) or empty for intake;
   - `trust` — the admission state for issue items, empty for intake;
   - `session_tag`, `dispatch_ref` — optional join keys (desk-supervision/28).
   State the NEVER-recorded list: titles, bodies, comments, author or assignee logins, human reply
   prose, prompts or transcripts. These records are never used to rank people or agents.
2. **Issue-lane writer (scanloop).** In `exitrecord.go`: an `IntakeExitRecord` struct with exactly
   the keys above; `Validate()` refusing (`deskkit.Refused`, exit 5) an unknown exit, a missing
   artifact where required, an artifact that is not a typed ref, a tier outside the set, `detail`
   on the wrong exit, `opened` after `triaged`; `appendExitRecord` writing one line to
   `<deskDir>/intake-exits.jsonl` under the existing audit lock discipline. Call it from `Land` for
   every member it records (mechanical: `decided_by: mechanical`, `triager_tier: none`). A write
   failure is returned, never swallowed (same rule as `auditExit`). Dry-run writes nothing.
3. **Judgment exits get a landing verb.** `scanloop land --item <owner/repo#N> --exit <exit>
   --artifact <ref> --tier any|strong [--detail rejected|watching] [--kind <reason>]
   [--dispatch-ref <id>]` — the session runs it after routing a parked item by hand. It goes
   through `ExitOf(ExitUnrouted, exit)`, refuses a second DIFFERENT exit for an item already in the
   record file (same exit again = exit 0 noop), writes the record (`decided_by: judgment`,
   `triager_role` = the loop's roster role) and the existing audit `land` line.
4. **Register writer (statusgen).** Add `Triaged` (`triaged`), `TriagedBy` (`triaged-by`),
   `TriagerTier` (`triager-tier`) to `intakeEntry`. When present: `triaged` must parse as a date or
   RFC3339; `triaged-by` must match `^[a-z][a-z0-9-]*$`; `triager-tier` must be `any` or `strong` —
   each violation a `--lint` PROBLEM naming the entry and value. Absence is legal (forward-only; no
   backfill). `statusgen --intake-exits --json --root <dir>` emits one `intake-exit-v1` line per
   entry whose disposition is triaged, mapped: `scoped` + `scoped-to` a stream or `<stream>/<NN>` →
   `placeholder`; `scoped` + `scoped-to` `issue #NN` → `bug`; `scoped` + `scoped-to` `F-<slug>` →
   `finding`; `decision-needed` → `needs-decision` (artifact = `decision-issue`); `watching` /
   `rejected` → `rejected-watching` with `detail`; legacy `adopted` → `placeholder`. It ends with a
   summary object `{"stamped": N, "unstamped": M, "unmapped": K}` so coverage is visible; an
   unmappable disposition is counted, never guessed. A missing register is could-not-check (exit 6),
   not an empty export.
5. **Spec.** §5.2 lists the three optional keys; new §5.4 carries the mapping table from step 4 and
   says the record schema lives in `intake-exit-v1.md`. The four-value disposition grammar is
   unchanged except that `scoped-to` may name an `F-<slug>`.
6. **Skill + docs.** intake-desk: the exits table gains the code slug per row; the intake-lane triage
   commit sets `triaged`, `triaged-by`, `triager-tier` alongside `disposition`; after routing a
   parked issue item, run `scanloop land`. Update `tools/desk/README.md`; add the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | exit 0; subtests: unknown exit → refused; missing artifact on `bug` → refused; free-text artifact → refused; vendor model name as tier → refused; `detail` on `bug` → refused; `opened` after `triaged` → refused; valid mechanical and judgment records → accepted | check:ci +mutation |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | exit 0; subtests: each disposition form maps to its exit; an unmappable value counts in `unmapped`; a bad `triager-tier` is a lint PROBLEM; an entry with none of the new keys lints clean and still counts as untriaged/triaged exactly as before (intake-debt neighbour) | check:ci +mutation +neighbour |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | exit 0; subtests: first land writes one record + one audit line; same exit again → exit 0, no second line; different exit for the same item → exit 5, file unchanged; `unrouted` → exit 5 | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | exit 0; an offline pass over a fixture inbound file (one new issue, one update) writes one `decided_by: mechanical` record per batch member, then `land` on the parked update writes the `judgment` record; no line contains the keys `title`, `author` or `body`, nor the fixture's author login | check:ci +flow |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | exit 0; each module's test reads the fields table of the schema doc `docs/streams/desk-supervision/intake-exit-v1.md` (planned) and fails if its record type's JSON keys differ from it in either direction (two writers, one schema) | check:ci +flow +mutation |
| 7 | `statusgen --intake-exits --json --root .` | exit 0; the last line is the summary object (in this repo, which has no per-entry intake files, all three counts are 0) — not an empty output and not exit 6 | check |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` (on an operator's desk host, after the first real `scanloop land`) | one line `<item> <exit> <artifact>`; `gh issue view` on the printed item and artifact shows both exist, and for `needs-decision` the artifact carries the `needs-decision` label, for `bug` the `bug` label | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A record carries a title, author login or vendor model name (sensitive text leaks into analysis data) | rows 2, 5, 6 |
| The two writers drift to different key sets, so records cannot be concatenated | row 6 |
| Judgment exits are still never written because the skill never runs `land` | row 9 (a real judgment record exists); skill wording stays review-only |
| `land` lets one item take two exits across sessions | row 4 |
| New intake keys break the intake-debt count or reject existing entries | row 3 (neighbour case) |
| Mapping guesses an exit for an unknown disposition, polluting labels | row 3 (`unmapped` count) |
| Export reports "zero exits" on a tree it could not read | row 7 (summary line, exit 6 only on a missing register) |
| A record names an artifact that does not exist or is the wrong kind | row 9 dereferences one; full artifact existence checking on write is not done (a forge read per record) — review-only |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
