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
  - "spec/registers-v1.md (§5.2 format: the optional triage stamp and `scoped → F-<slug>`; new §5.4 exit mapping): fixed-here"
  - "statusgen/registerentries.go (intakeEntry gains three optional keys; the strict key set is derived from the struct): fixed-here"
  - "statusgen/main.go (the --intake-exits flag): fixed-here"
  - "tools/desk/cmd/scanloop/{adapter,exitrecord,landverb}.go (record writer called from Land, and the land verb; land.go's ExitOf and auditExit are reused unchanged): fixed-here"
  - "tools/desk/cmd/scanloop/main.go (the land verb and usage text): fixed-here"
  - "plugins/assay/skills/intake-desk/SKILL.md (exits table naming, triage stamp, land verb step): fixed-here"
  - "tools/desk/README.md (scanloop section): fixed-here"
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
- **add** `spec/intake-exit-v1.md` (first planned in this stream's directory; moved because `tools/desk` ships and its schema test must not name a withheld stream path) — the record schema: fields
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
- **The register is committed to git; the issue-lane file is not.** The intake register
  (`docs/streams/intake/`) is tracked, so in a public adopter repo anything written to an entry's
  frontmatter is public. That is why every identity-bearing value the record or the stamp can carry
  (`triager_role`, `triaged-by`) is a CLOSED role set, never a free slug: a slug pattern admits any
  lowercase person login. The closed set is the five canonical desk loop names
  (`deskkit.canonicalLoopNames`, `tools/desk/internal/deskkit/loopnames.go`) — `the-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `intake-desk` — plus the neutral token `driver`
  for a stamp a human wrote by hand. It is spelled as a literal in BOTH modules (statusgen cannot
  import deskkit) and pinned to the schema doc by row 6. This is the same shape desk-supervision/29's
  `observer_role` and desk-supervision/32's `ruler` already use.
- single-point-of-failure: the record writer's closed-key and closed-vocabulary validation (no
  title, body, author, person login or vendor model name can enter a record or a register stamp) —
  backed by the schema-match test in BOTH modules (row 6), which fails on any added JSON key or any
  difference in the role set, by the register-side lint on `triaged-by` (a different component from
  the scanloop writer, and the only guard on a hand-edited entry), and by the review gate on the
  implementation diff.

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
   - `triager_role` — one of the CLOSED role set: `the-desk`, `worker-desk`, `pr-review-desk`,
     `verify-desk`, `intake-desk`, or `driver` (a hand-written stamp). Never a login, a name or any
     other slug; the set is listed in the schema doc's vocabulary table and both writers and the
     register lint check membership, not a pattern;
   - `triager_tier` — `any` | `strong` for `judgment`, `none` for `mechanical`; never a vendor
     model name (the public stamp label is the only place a model name appears, and this record
     does not repeat it);
   - `opened`, `triaged` — RFC3339 (`opened` <= `triaged`); `opened` may be empty when unknown;
   - `kind` — one of the classifier reasons `new-issue`, `update`, `unreadable-placeholder-state`,
     `no-scan-target`, `scan-target-outside-write-boundary` (`classify`, `adapter.go:303-326`), or
     empty for intake;
   - `trust` — the admission state for issue items, empty for intake;
   - `session_tag`, `dispatch_ref` — optional join keys (desk-supervision/28); `dispatch_ref` must
     parse against desk-supervision/28's grammar. The record file is local state (never committed),
     so the clear `dispatch_ref` is allowed there and nowhere on a public surface.
   State the NEVER-recorded list: titles, bodies, comments, author or assignee logins, human reply
   prose, prompts or transcripts. These records are never used to rank people or agents.
2. **Issue-lane writer (scanloop).** In `exitrecord.go`: an `IntakeExitRecord` struct with exactly
   the keys above; `Validate()` refusing (`deskkit.Refused`, exit 5) an unknown exit, a missing
   artifact where required, an artifact that is not a typed ref, a tier outside the set, a
   `triager_role` outside the closed role set (a login-shaped value such as `alice-dev` included), a
   `kind` outside the classifier's reasons (`new-issue`, `update`, `unreadable-placeholder-state`,
   `no-scan-target`, `scan-target-outside-write-boundary`), a `dispatch_ref` that does not parse
   against desk-supervision/28's grammar, `detail` on the wrong exit, `opened` after `triaged`; `appendExitRecord` writing one line to
   `<deskDir>/intake-exits.jsonl` under the existing audit lock discipline. Call it from `Land` for
   every member it records (mechanical: `decided_by: mechanical`, `triager_tier: none`), AFTER that
   member's audit `land` line. A write failure is returned, never swallowed, but only once every
   member is landed and audited: a refused or unwritable record costs that member its record, never
   its audit line and never another member's exit. Dry-run writes nothing.
3. **Judgment exits get a landing verb.** `scanloop land --item <owner/repo#N> --exit <exit>
   --artifact <ref> --tier any|strong [--opened <RFC3339>] [--detail rejected|watching] [--kind <reason>]
   [--dispatch-ref <ref>]` — the session runs it after routing a parked item by hand, with the
   command the EMITTED report prints for that item. `--kind` takes
   only the classifier reasons listed in step 2 and `--dispatch-ref` only a value that parses
   against desk-supervision/28's grammar; anything else is refused (exit 5) before anything is
   written. It goes through `ExitOf(ExitUnrouted, exit)`, refuses a second DIFFERENT exit for an
   inbound EVENT already in the record file (the key is the item, compared without regard to case,
   plus `--opened`; same exit again = exit 0 noop), writes the existing audit `land` line and then
   the record (`decided_by: judgment`, `triager_role` = the canonical name of the running loop,
   taken from the `$DESK_LOOP` the session declared and refused if unset or not in the closed role
   set).
   Both `--kind` and `--dispatch-ref` are self-reported labels for analysis only; nothing gates on
   them.
4. **Register writer (statusgen).** Add `Triaged` (`triaged`), `TriagedBy` (`triaged-by`),
   `TriagerTier` (`triager-tier`) to `intakeEntry`. When present: `triaged` must parse as a date or
   RFC3339; `triaged-by` must be a MEMBER of the closed role set (`the-desk`, `worker-desk`,
   `pr-review-desk`, `verify-desk`, `intake-desk`, `driver`) — never matched by a pattern, so a
   login-shaped value such as `alice-dev` is a PROBLEM; `triager-tier` must be `any` or `strong` —
   each violation a `--lint` PROBLEM naming the entry and value (for `triaged-by`, the PROBLEM names
   the entry and the rule, and quotes the offending value only up to the closed set's length so a
   long pasted string is not echoed back). Absence is legal (forward-only; no
   backfill). `statusgen --intake-exits --json --root <dir>` emits one `intake-exit-v1` line per
   entry whose disposition is triaged, mapped: `scoped` + `scoped-to` a stream or `<stream>/<NN>` →
   `placeholder`; `scoped` + `scoped-to` `issue #NN` → `bug`; `scoped` + `scoped-to` `F-<slug>` →
   `finding`; `decision-needed` → `needs-decision` (artifact = the entry's `decision-issue` value rendered as
   the typed ref `#N`; an entry with no `decision-issue` is counted `unmapped`); `watching` /
   `rejected` → `rejected-watching` with `detail`; legacy `adopted` → `placeholder`. It ends with a
   summary object `{"stamped": N, "unstamped": M, "unmapped": K}` so coverage is visible; an
   unmappable disposition is counted, never guessed. The register is the
   directory `docs/streams/intake/` AND the index `docs/streams/INTAKE.md`: if NEITHER exists the
   export is could-not-check (exit 6), not an empty export. An index with no per-entry files (this
   repo) is a readable register with zero entries: exit 0, summary counts all 0.
5. **Spec.** §5.2 lists the three optional keys; new §5.4 carries the mapping table from step 4 and
   says the record schema lives in `intake-exit-v1.md`. The four-value disposition grammar is
   unchanged except that `scoped-to` may name an `F-<slug>`.
6. **Skill + docs.** intake-desk: the exits table gains the code slug per row; the intake-lane triage
   commit sets `triaged`, `triaged-by` (the running loop's canonical name, or `driver` when a human
   triages by hand), `triager-tier` alongside `disposition`; after routing a
   parked issue item, run `scanloop land`. Update `tools/desk/README.md`; add the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | exit 0; subtests: unknown exit → refused; missing artifact on `bug` → refused; free-text artifact → refused; vendor model name as tier → refused; a login-shaped `triager_role` (`alice-dev`) → refused while each member of the closed role set is accepted; `kind` outside the classifier reasons → refused; `dispatch_ref` not matching desk-supervision/28's grammar → refused; `detail` on `bug` → refused; `opened` after `triaged` → refused; valid mechanical and judgment records → accepted | check:ci +mutation |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | exit 0; subtests: each disposition form maps to its exit; an unmappable value counts in `unmapped`; a bad `triager-tier` is a lint PROBLEM; a login-shaped `triaged-by` (`alice-dev`) is a lint PROBLEM, while each closed-set member lints clean and so does an absent key; an entry with none of the new keys lints clean and still counts as untriaged/triaged exactly as before (intake-debt neighbour) | check:ci +mutation +neighbour |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | exit 0; subtests: first land writes one record + one audit line; same exit again → exit 0, no second line; different exit for the same event → exit 5, file unchanged; `unrouted` → exit 5; `$DESK_LOOP` set to a login-shaped value → exit 5, file unchanged; `--kind free-text` or a malformed `--dispatch-ref` → exit 5, file unchanged | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | exit 0; two offline passes: pass 1 (two new issues) writes one `decided_by: mechanical` record per batch member; pass 2 sees one of them again as an update, parks it and prints its `land` command with `--opened`; that `land` writes the `judgment` record beside the item's earlier mechanical one, and a different exit for the same event is refused; no line contains the keys `title`, `author` or `body`, nor the fixture's author login | check:ci +flow |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | exit 0; each module's test reads the fields table of the schema doc `spec/intake-exit-v1.md` and fails if its record type's JSON keys, or its closed role set, differ from it in either direction (two writers, one schema, one vocabulary) | check:ci +flow +mutation |
| 7 | `statusgen --intake-exits --json --root .` | exit 0; the last line is the summary object (in this repo, which has the `docs/streams/INTAKE.md` index but no per-entry intake files, all three counts are 0) — not an empty output and not exit 6 | check |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` (on an operator's desk host, after the first real `scanloop land`) | one line `<item> <exit> <artifact>`; `gh issue view` on the printed item and artifact shows both exist, and for `needs-decision` the artifact carries the `needs-decision` label, for `bug` the `bug` label | gate:model +dereference |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | exit 0, prints 4; a refused or unreadable record in a drain pass still lands and audits every batch member (exit 5 or 6 naming the item); an undeclared `$DESK_LOOP` is refused through the real command path; a land whose audit write failed leaves no record and the retry writes both; the same event spelt in a different case or offset is one event, a later event of the same item takes its own exit | check:ci +mutation |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A record carries a title, author login or vendor model name (sensitive text leaks into analysis data) | rows 2, 5, 6 |
| A person's login lands in the committed register (`triaged-by`) or the record (`triager_role`) because the value was checked by pattern, not by membership | rows 2, 3, 4 (a login-shaped value refused at the register lint AND at both writers) |
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
### Independent verification — 2026-10-11

**VERIFY: FAIL** — BLOCKED: eight required check:ci rows could not execute, row 9 lacks its real operator-host prerequisite, and static validator mismatches require repair. No check:ci test failure or PASS is claimed.

Runner: assay-verifier-app[bot]; actual model gpt-6.1-sol; tier any; merged source 18d752a4c9318e27bcfd49fa44f9cfb62923b691.

Expected behavior was grounded from the brief Task/Verify and the pre-implementation register spec before implementation/source tests were read. Independent isolated schema, flow and suite clusters received only the brief text and their assigned rows; static reads are separated from test results.

Execution witness from statusgen verifyrun --dry-run, exit 2. The generated Runner principal, command, result, output hash, date and source are retained exactly; the published role identity annotation is part of the witness authority chain.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 | sha256:6949ea13ef1f | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | fail exit=2 | sha256:f649715f3cb7 | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2662 run=646a9aae149b103915cb9200d053806e030dabad9597bd9ca3db99c6c37470fe source=18d752a4c9318e27bcfd49fa44f9cfb62923b691 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=gpt-6.1-sol tier=any


| Verify row | Expected | Observed | Date / runner |
|---|---|---|---|
| 1 | Both Go-module suites exit 0 | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 2 | Closed vocabulary, typed refs, times and dispatch joins validate or refuse exactly as Task 2 declares | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 3 | Disposition mapping, complete stamps, invalid present keys and neighbour debt behavior match Task 4 | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 4 | One record plus audit; same event/exit noop; conflicting exit and invalid declared role or labels refuse without writes | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 5 | Two offline passes yield mechanical then judgment records and emitted opened event; no sensitive text keys or fixture login | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 6 | Both writers match all sixteen JSON keys and six roles in the schema | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 7 | Index-only register emits a zero-count summary and exits 0 | PASS; exit=0; {"stamped":0,"unstamped":0,"unmapped":0} | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 8 | Every consumers routing is corroborated against implementation diff | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 9 | Real operator judgment record exists; item and typed artifact dereference with appropriate labels | BLOCKED; exit=2; jq: error: Could not open file $HOME/.config/assay/intake-exits.jsonl: No such file or directory; null null null | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 10 | Four event tests pass: preserve audit on record refusal/unreadability, unset loop refusal, audit retry, event case/offset equality and later-event independence | BLOCKED; exit=- (unexecuted); No command output: row did not execute. Required unshare --net sandbox is unavailable on Darwin. | 2026-10-11 assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |

The row-9 witness preserves fail exit=2 as written; its cause is an absent operator-host record rather than an observed implementation failure. No item/artifact was available for a forge dereference. The missing prerequisite is [#2666](https://github.com/medici-finance/assay/issues/2666). The check:ci executor prerequisite is [#2665](https://github.com/medici-finance/assay/issues/2665).

Static findings, filed as [#2668](https://github.com/medici-finance/assay/issues/2668): the scanloop dispatch-ref regex requires a double-hyphen and has no prefix/whole-ref length cap, while the authoritative dispatch validator accepts a bounded 1–226 printable-ASCII prefix and caps strings at 256 bytes; present-empty or whitespace triage keys are skipped despite the present-value validation contract. The real land command reaches the existing unknown-loop Guard first and returns exit 6, while row 4 specifies exit 5; reconcile the contract/check without weakening Guard. Collected unreadable-record errors lose member IDs, while row 10 expects each affected item to be named. These are source comparisons, not executed test outcomes.

Risk enumeration covered every introduced/changed literal across all 23 delivering-change paths and Task/Deliverables: 1616 string, raw-string and numeric entries, plus resolved inherited exit/classifier/admission literals and numeric bounds inside patterns. Privacy/identity and permission literals rank first because later edits cannot recall disclosure; routing/join literals rank next because historical records require correction; reversible operational bounds rank last.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — Task steps 1/4 require exactly the five canonical desk roles plus neutral driver; the canonical loop registry supplies the five desk names and driver denotes a hand-written stamp. Membership bounds identity to roles without admitting a person's arbitrary slug. This is a spec derivation, not a passing test claim.

RISK-VALUE: DERIVED — recordCreateMode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — octal 6 grants the owner read+write, and zero group/other bits grant nobody else access. Local dispatch/session joins stay in desk state instead of public records. This matches the new schema's explicit 0600 requirement and the established audit-state restriction.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — equivalence to authoritative desk-supervision/28 cannot be derived: the canonical validator accepts a bounded 1–226 printable-ASCII claim key without requiring double-hyphen, and caps the total ref at 256 bytes. This copied literal requires double-hyphen and omits those bounds. Filed as medici-finance/assay#2668.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so both writers can be concatenated under one record schema; it is not an implementation-selected version.

RISK-VALUE: DERIVED — JSON keys = ["schema", "source", "item", "repo", "exit", "detail", "artifact", "decided_by", "triager_role", "triager_tier", "opened", "triaged", "kind", "trust", "session_tag", "dispatch_ref"] @ tools/desk/cmd/scanloop/exitrecord.go:38 and statusgen/intakeexits.go:32 — Task step 1 exhaustively declares exactly these sixteen keys. No title/body/comment/author key is needed to join the inbound event, chosen exit, typed artifact, role/tier and time, so these exclusions follow from the record's text-free purpose.

RISK-VALUE: DERIVED — trackedExits = ["placeholder", "bug", "finding", "needs-decision", "rejected-watching"] @ tools/desk/cmd/scanloop/land.go:24 — Task step 1 and the pre-existing front-door taxonomy define exactly five exits; unrouted is a pending state rather than a sixth outcome. Reusing the existing ledger vocabulary preserves the already-established routing authority.

Wake: operator completes #2665 for rows 1–6, 8, 10 and #2666 for row 9; worker or brief author resolves #2668 for contracts of rows 2, 3, 4 and 10 without weakening the stop guard. The brief stays implemented until independent verification resolves all mandatory rows.

### Independent verification — 2026-10-15

**VERIFY: BLOCKED** — row 9 could-not-check: its forge dereference half (an issue read of the printed item and artifact, with label check) needs a network call, which this run's offline envelope forbids; its record half ran (exit 0, one judgment line). Environment, not an implementation failure: rows 1–8 and 10 pass at merged source 8efe5579d925. 9 of 10 rows pass.

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk); tier any; non-implementer. Merged source 8efe5579d9253813835db650d45180eea76cf33e, detached worktree. Window 2026-10-15T04:11:40Z to 2026-10-15T04:26Z. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7; one later change to scanloop (#2946) is included at this source.

Expected results were written down from the brief's Task and Verify sections before any test or diff was read. Every row ran as its own invocation with HOME, XDG_CONFIG_HOME, GOCACHE and TMPDIR pointed at a scratch directory outside the worktree; GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null (no network). Toolchain go1.27.2 darwin/arm64. Rows 7 and 8 used statusgen built from this source (go build of the statusgen module into the scratch directory), not a pinned release binary, since the flag under test ships in this change.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | exit 0 | exit 0; ok statusgen 414.236s, ok statusgen/contexthandoff, ok statusgen/evaluation, ok statusgen/streamview, ok tools/desk/cmd/scanloop 58.337s | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | exit 0; refusal and acceptance subtests as listed in Verify row 2 | exit 0; --- PASS: TestExitRecord_Validate plus 45 PASS subtests, among them refused/unknown exit, refused/missing artifact on bug, refused/free-text artifact, refused/vendor model as tier, refused/login-shaped role alice-dev, accepted/role for each of the six closed-set members, refused/kind outside classifier, refused/bad dispatch ref, refused/detail on bug, refused/opened after triaged, accepted/valid mechanical, accepted/valid judgment | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | exit 0; mapping, unmapped count, tier and role lint PROBLEMs, closed-set members and absent keys clean, intake-debt neighbour unchanged | exit 0; --- PASS: TestIntakeExits_MappingAndStamp plus 30 PASS subtests: 11 maps cases, 4 unmapped cases, bad triager-tier and bad triaged lint PROBLEMs, login-shaped triaged-by alice-dev lint PROBLEM, long triaged-by not echoed, six closed-set members lint clean, absent keys lint clean, intake-debt neighbour unchanged, no register is could-not-check, index with no entries is a zero summary | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | exit 0; first land writes one record and one audit line; same exit noop; different exit, unrouted, login-shaped loop, free-text kind, bad dispatch-ref each refused | exit 0; --- PASS: TestLandVerb_OneExitPerItem plus 10 PASS subtests: first land writes one record and one audit line, same exit again is a no-op, different exit, unrouted, role alice-dev, role unset, free-text kind, bad dispatch-ref, vendor tier, free-text artifact | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | exit 0; two offline passes, mechanical then judgment records, no title, author or body key | exit 0; --- PASS: TestDrainPass_WritesExitRecords (4.00s) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | exit 0; both modules match the schema doc's keys and role set | exit 0; --- PASS: TestIntakeExitSchema_MatchesDoc printed once for statusgen and once for scanloop | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | `statusgen --intake-exits --json --root .` | exit 0; last line is the summary with all three counts 0 | exit 0; stdout is one line, {"stamped":0,"unstamped":0,"unmapped":0}; stderr carries only the assay-config banner | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | `statusgen --consumers --root .` | exit 0; every consumers routing corroborated by the implementation diff | exit 0; "consumers: no brief files in the diff against 8efe5579d9253813835db650d45180eea76cf33e — nothing to corroborate" (the default base is the source itself, so this exit 0 is vacuous). Supplementary run against the delivering change, statusgen --consumers --root . --base 0b1f1295c8390345df6b384539b98190785e69a5 --brief desk-supervision/33: exit 0, "summary: 7 corroborated, 0 disproved, 4 unchecked"; the 4 unchecked are the follow-up and three out-of-scope routings, each "unchanged since the merge-base" as their routings claim | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` (on an operator's desk host, after the first real `scanloop land`) | one line item exit artifact; a forge read shows both exist and the bug or needs-decision label | jq half: exit 0, one line of the shape owner/repo#N bug owner/repo#N (item and artifact are the same issue, in a repository that is not public, so its name is withheld here). The operator file holds 323 records, all judgment, all source issue, triager_role intake-desk on every one, tiers any 195 and strong 128, and its key union is exactly the 16 schema keys. Dereference half: could-not-check, because this run is offline and a forge issue read is a network call. Re-run on a networked desk host: gh issue view N -R owner/repo --json number,labels on the printed item and artifact | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | exit 0, prints 4 | exit 0; prints 4 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Risk-bearing value. The brief's risk answers are all no and it is not irreversible; the enumeration was still done over every literal the delivering change introduces in tools/desk/cmd/scanloop/exitrecord.go and statusgen/intakeexits.go plus the Task's own vocabularies: the schema name, the record file name intake-exits.jsonl, decided_by values, the three tiers, the six-member role set, the five classifier kinds, the admission states, the item, intake id, repo, session tag (1 to 128 characters) and five typed-artifact patterns, the dispatch_ref pattern, the 0o600 modes on the lock and record files, the 60-second lock wait, the lint clip length (the longest role, 14 characters), and the reused five-exit set. Ranked: identity and privacy literals first, since a leaked login in a committed register cannot be recalled by an edit; join-key grammar next, since malformed historic records need correction; the lock wait, clip length and session-tag bound last as reversible knobs.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — Task steps 1 and 4 fix the set as the five canonical desk loop names plus the neutral driver token; the five match the canonical loop registry in tools/desk/internal/deskkit/loopnames.go, and membership (not a pattern) is what keeps any person's login out of the committed register.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — owner read and write, no group or other bits; the schema doc requires 0600 because the file may carry a clear dispatch_ref, which Task step 1 allows only in local state.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so the two writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [placeholder, bug, finding, needs-decision, rejected-watching] @ tools/desk/cmd/scanloop/land.go:41 — Task step 1 names exactly these five code slugs; unrouted is a pending state, refused as an exit by rows 2 and 4.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — Task steps 2 and 3 require dispatch_ref to parse against the desk-supervision/28 grammar, whose validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement. This copy requires a double hyphen and a narrower charset and has no length bound, so it is not equivalent; the gap is the one already filed as #2668. Reversible: the value is an analysis-only label that nothing gates on.

Verification-Attestation: medici-finance/assay#3008 run=a5d66d716927b931174920bd86ad3f050eab8ed8f568611864ea7c77f7ed645f source=8efe5579d9253813835db650d45180eea76cf33e brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=any
### Independent verification — 2026-10-29

**VERIFY: FAIL** — row 9: the newest judgment record is a needs-decision exit whose artifact, #2985, does not carry the needs-decision label (removed on 2026-10-27, a day before the record was written); row 1 exited 1 because the statusgen package hit go test's default 10-minute deadline on a loaded host (#4391), with no assertion failure in that package. Rows 2–8 and 10 pass. 8 of 10 rows pass.

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer. Merged source 711d8740e8caf8aa561113ec54c97fc06c021784. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7. Later changes to the delivered scanloop and statusgen files up to this source (#4413, #3261, #3120, #3065, #3051 and one unnumbered policy commit) leave every risk-bearing literal below unchanged.

Rows 1–8 and 10 ran in one closed admission bracket: host open-bracket, then the pinned verify-runner v1.0.65 image (by digest, network none, read-only root, every capability dropped) running statusgen verifyrun with the container-netns sandbox against a byte-identical clone of the home, then host close-bracket. The bracket digest check reports verdict=match. Row 9 ran on the operator desk host through statusgen verifyrun, with no forge token in its environment. Its forge dereference was a separate read-only issue read as the verifier App. That read is the forge, not a cluster or production endpoint.

Row 1 diagnosis (supplemental, no credit). The witness keeps only an output hash, so the row's two halves were re-run in the same image with their output kept. The statusgen half printed "panic: test timed out after 10m0s" for the root statusgen package at 601.4 s, while a coverage-bookkeeping test was running. No test in that package reported a failure first. In the same run the workref package's public-import-boundary test failed with "production consumer tools/desk: signal: killed" after 77 s. That is the process being killed under memory and CPU pressure, not an assertion. Every other statusgen sub-package passed. Five other verifier containers were sharing the 6-CPU engine. This is the same failure class as #4391. In the witnessed row the && short-circuited the scanloop half after the statusgen half exited 1. Run alone, it fails four trust-gate tests ("cannot resolve which forge serves medici-finance/assay") when the clone's origin remote is a local path. With origin set to the home's own GitHub URL it passes (ok, 14.6 s). So that was an artifact of the clone, not of the code.

Row 9 dereference. The jq line names one public issue as both item and artifact: medici-finance/assay#2985 needs-decision medici-finance/assay#2985. The sha256 of that line is the witness hash 09fce5bf21c8. The issue read shows #2985 exists and is OPEN, with only the raised-by:desk label. Its label events show needs-decision added on 2026-10-15T02:09:50Z and removed by the desk App on 2026-10-27T22:14:51Z. The record says intake-desk, judgment, kind update, triaged 2026-10-28T20:49:43Z, so the needs-decision exit was recorded after the artifact had lost its label. This is the same contract gap #2862 records for bug exits: the land verb writes an exit without checking that the artifact carries the matching label. The operator file holds 343 records, all judgment.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | fail exit=1 sandbox=container-netns | sha256:e177a369f31e | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | pass exit=0 sandbox=container-netns | sha256:35f01ba61304 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:126acf2caa81 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | pass exit=0 sandbox=container-netns | sha256:c3eb85d58e40 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | pass exit=0 sandbox=container-netns | sha256:13ac83173225 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | pass exit=0 sandbox=container-netns | sha256:55999241c271 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:97d7cdf4e132 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:8512eb233cb1 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | pass exit=0 sandbox=container-netns | sha256:7de1555df0c2 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:6becf45565b3507f2a6d7a9bace7779b5382ebc55eaf8ac5e6e9ad08f2a4ef9c
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4879 run=15f486ece7ed08ff2b15b13550652124dbbd2825a70c7a8d83953dccd0ef48f5 source=711d8740e8caf8aa561113ec54c97fc06c021784 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=15f486ece7ed08ff2b15b13550652124dbbd2825a70c7a8d83953dccd0ef48f5 source=711d8740e8caf8aa561113ec54c97fc06c021784 tree=sha256:6becf45565b3507f2a6d7a9bace7779b5382ebc55eaf8ac5e6e9ad08f2a4ef9c witness=sha256:f00840c7f795eb21908985b3e1e31c4658b6a321cf0c0ffd297dd78bfdb0e600 opened=2026-10-29T20:29:14Z closed=2026-10-29T20:49:17Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:09fce5bf21c8 | 2026-10-29 | assay-verifier-app[bot] @ 711d8740e8ca (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4879 run=15f486ece7ed08ff2b15b13550652124dbbd2825a70c7a8d83953dccd0ef48f5 source=711d8740e8caf8aa561113ec54c97fc06c021784 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-10-29, the verifier App at 711d8740e8ca):

- Row 1 — expected exit 0. Observed: exit 1 in the container bracket; diagnosis: the root statusgen package hit go test's default 10m deadline (601.4 s) on a loaded host, and the workref boundary test's child was killed; no assertion failure in the root package (#4391).
- Row 2 — expected exit 0; the refusal and acceptance subtests of Verify row 2. Observed: exit 0 in the container bracket.
- Row 3 — expected exit 0; mapping, unmapped, lint and neighbour subtests. Observed: exit 0 in the container bracket.
- Row 4 — expected exit 0; land subtests. Observed: exit 0 in the container bracket.
- Row 5 — expected exit 0; two offline passes. Observed: exit 0 in the container bracket.
- Row 6 — expected exit 0; both modules match the schema doc. Observed: exit 0 in the container bracket.
- Row 7 — expected exit 0; summary with all three counts 0. Observed: exit 0 in the container bracket; a separate probe of the same image printed {"stamped":0,"unstamped":0,"unmapped":0}.
- Row 8 — expected exit 0. Observed: exit 0 in the container bracket; the default base is the source itself, so the probe printed "nothing to corroborate" (vacuous).
- Row 9 — expected one item exit artifact line; both exist; a needs-decision artifact carries needs-decision. Observed: jq exit 0, line medici-finance/assay#2985 needs-decision medici-finance/assay#2985; #2985 exists, OPEN, labels raised-by:desk only, needs-decision removed 2026-10-27T22:14:51Z; obligation fails.
- Row 10 — expected exit 0, prints 4. Observed: exit 0 in the container bracket.

Risk-bearing value. The brief's risk answers are all no and it is not irreversible. The enumeration still covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies. That is the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern, the 0o600 lock and record modes, and the reversible knobs (60-second lock wait, clip length, session-tag bound). Ranked: identity and privacy literals first, because a login that leaks into the committed register cannot be recalled by an edit. The join-key grammar comes next, and the reversible knobs last, with no derivation needed. Each literal was re-read at this source.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver token, exactly the schema doc's closed role set. Membership, not a pattern, keeps a person's login out of the register.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — owner read and write, no group or other bits. The schema doc requires 0600 because the file may carry a clear dispatch_ref, which is allowed only in local state.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so both writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement. This copy requires a double hyphen, uses a narrower charset and has no length bound, so the two are not equivalent. Still open as #2668. Reversible: it is an analysis-only label that nothing gates on.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
