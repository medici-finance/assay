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
RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement. This copy requires a double hyphen, uses a narrower charset and has no length bound, so the two are not equivalent. Still open as #2668. Reversible: it is an analysis-only label that nothing gates on.

### Independent verification — 2026-10-30

**VERIFY: FAIL** — row 9: the newest judgment record is still the needs-decision exit on #2985, and #2985 still does not carry the needs-decision label (removed 2026-10-27, a day before the record was written; the #2862 land-verb label gap). Row 1 exited 1 because verifyrun's admitted row environment sets GIT_CONFIG_COUNT and the statusgen git read-parity and revision tests refuse that environment profile (#2715); the same command passes outside verifyrun. Rows 2–8 and 10 pass. 8 of 10 rows pass.

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer. Merged source 40925e8233c8a63d70682e77ca333e665f1f955e. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7. Since the previous run's source 711d8740, the only change to a delivered file is in statusgen/main.go (a gate-scores row type), which leaves every risk-bearing literal below unchanged.

Rows 1–8 and 10 ran in one closed admission bracket: host open-bracket, then the pinned verify-runner v1.0.66 image (release index sha256:29494cbeca300a29295a2eeed7398500675cb63c7911d660a08c899e8584d175, run natively as its linux/arm64 manifest, the Container-Image below; network none, read-only root, every capability dropped, non-root user) running the installed statusgen v1.0.66 verifyrun with the container-netns sandbox against the home mounted read-only, then host close-bracket. The bracket digest check reports verdict=match. SG_OLD_BIN pointed at the checksum-verified v1.0.52 statusgen baseline release asset, as CI's fetch-baseline step provisions it, because the witness skew tests fail rather than skip without it. An earlier bracket in this run is superseded and not landed: its container carried verifier-added git config and no baseline, both runner errors on my side. Row 9 ran on the operator desk host through statusgen verifyrun, with no forge token in its environment. Its forge dereference was a separate read-only issue read as the verifier App.

Row 1 diagnosis (supplemental, no credit). The witness keeps only an output hash, so the row's two halves were re-run in the same image and environment outside verifyrun with output kept: every statusgen package passed (exit 0) and the scanloop half passed (exit 0). The same statusgen half was then re-run with exactly the git variables verifyrun's admitted row environment appends (GIT_NO_REPLACE_OBJECTS, GIT_ATTR_NOSYSTEM, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT, GIT_ASKPASS, and GIT_CONFIG_COUNT=2 with the two grep keys). It exited 1: the root statusgen package failed TestGoGitSessionLock, TestGoGitReadParity, TestRevisionCallerFlow, TestRevisionOrdinaryNestedPrefix, TestRevisionPortProfiles, TestStaleFailHistoryQueryReuse and TestFlowCard, each reporting the "native discovery/config/shallow environment profile" or its fallback; every other statusgen package passed. That is open issue #2715 (git-fixture tests fail under verifyrun's admitted row env), a check-environment defect in the witness harness, not a failure of the delivered intake-exit code.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | fail exit=1 sandbox=container-netns | sha256:c8bd8546325f | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | pass exit=0 sandbox=container-netns | sha256:c106fb6c2f79 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:a8f1fe69fe69 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | pass exit=0 sandbox=container-netns | sha256:6f38a7544752 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | pass exit=0 sandbox=container-netns | sha256:7827e784182d | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | pass exit=0 sandbox=container-netns | sha256:55999241c271 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:2039cc614730 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:323fc3f3fec5 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | pass exit=0 sandbox=container-netns | sha256:7de1555df0c2 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465
Container-Image: sha256:097db7d3e20415bc7a2c524250e2ad89158ed93a31a7d8cd861e6cd38fc3b3b4
Verification-Attestation: medici-finance/assay#4954 run=78d4f0dbfc282777ecf75262c28ff2230f46fd50ec1d120b41323ecbf73c7bcc source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=78d4f0dbfc282777ecf75262c28ff2230f46fd50ec1d120b41323ecbf73c7bcc source=40925e8233c8a63d70682e77ca333e665f1f955e tree=sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465 witness=sha256:a9e74983aaaaa8befb7883567400701ceccb23f5b89fd28c9851107d7b6b1126 opened=2026-10-30T02:27:32Z closed=2026-10-30T02:32:53Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:09fce5bf21c8 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4954 run=78d4f0dbfc282777ecf75262c28ff2230f46fd50ec1d120b41323ecbf73c7bcc source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Row 9 dereference. The jq line names one public issue as both item and artifact: medici-finance/assay#2985 needs-decision medici-finance/assay#2985, the same line (witness hash 09fce5bf21c8) as the previous run; no newer judgment record has been written. The record says intake-desk, judgment, kind update, triaged 2026-10-28T20:49:43Z. The issue read shows #2985 exists and is OPEN, with only the raised-by:desk label. Its label events show needs-decision added on 2026-10-15T02:09:50Z and removed by the desk App on 2026-10-27T22:14:51Z, so the needs-decision exit was recorded after the artifact had lost its label. This is the contract gap #2862 records: the land verb writes an exit without checking that the artifact carries the matching label. The operator file holds 343 records, all judgment.

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-10-30, the verifier App at 40925e8233c8):

- **Row 1** (2026-10-30): `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` — expected exit 0; observed exit 1 in the container bracket. Outside verifyrun, same image and environment, both halves exit 0; with verifyrun's admitted git variables added, seven root-package git tests refuse the environment profile (#2715). No delivered intake-exit test failed.
- **Row 2** (2026-10-30): TestExitRecord_Validate — expected exit 0 with the refusal and acceptance subtests; observed exit 0 in the container bracket.
- **Row 3** (2026-10-30): TestIntakeExits_MappingAndStamp — expected exit 0 with mapping, unmapped, lint and neighbour subtests; observed exit 0 in the container bracket.
- **Row 4** (2026-10-30): TestLandVerb_OneExitPerItem — expected exit 0 with the land subtests; observed exit 0 in the container bracket.
- **Row 5** (2026-10-30): TestDrainPass_WritesExitRecords — expected exit 0 with two offline passes; observed exit 0 in the container bracket.
- **Row 6** (2026-10-30): TestIntakeExitSchema_MatchesDoc in both modules — expected exit 0; observed exit 0 in the container bracket.
- **Row 7** (2026-10-30): `statusgen --intake-exits --json --root .` — expected exit 0 with a summary line; observed exit 0 in the container bracket.
- **Row 8** (2026-10-30): `statusgen --consumers --root .` — expected exit 0; observed exit 0 in the container bracket.
- **Row 9** (2026-10-30): the jq line plus a read-only issue read of item and artifact — expected one item exit artifact line, both exist, and a needs-decision artifact carries needs-decision; observed jq exit 0, line medici-finance/assay#2985 needs-decision medici-finance/assay#2985; #2985 exists and is OPEN with labels raised-by:desk only, needs-decision removed 2026-10-27T22:14:51Z; obligation fails.
- **Row 10** (2026-10-30): TestExitEvent_ — expected exit 0, prints 4; observed exit 0 in the container bracket.

Risk-bearing value. The brief's risk answers are all no and it is not irreversible. The enumeration still covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies: the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern, the 0o600 lock and record modes, and the reversible knobs (60-second lock wait, clip length, session-tag bound). Ranked: identity and privacy literals first, because a login that leaks into the committed register cannot be recalled by an edit; the join-key grammar next; the reversible knobs last, with no derivation needed. Each literal was re-read at this source.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver token, exactly the schema doc's closed role set. Membership, not a pattern, keeps a person's login out of the register.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — owner read and write, no group or other bits. The schema doc requires 0600 because the file may carry a clear dispatch_ref, which is allowed only in local state.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so both writers' records concatenate under one schema.

### Independent verification — 2026-11-08

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer, under desk tools v1.0.78. Merged source ac53f30aad5e91b717cb9790efef8c4a8917e661. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7, an ancestor of this source. Since source 8592383261fb (a run on 2026-11-06 whose outcome was never landed, so it is void), no delivered file has changed: the scanloop package and statusgen/intakeexits.go are byte-identical. The only change in the delivered modules is tools/desk go.mod and go.sum, which raise several requirements (for example mattn/go-isatty v0.0.20 to v0.0.24 and go-logr/logr v1.4.3 to v1.4.4) and add new ones (jackc/pgx/v5 v5.12.0, pressly/goose/v3 v3.29.0 and the in-repo workflow module among them).

Rows 1–8 and 10 ran in one closed admission bracket: host open-bracket, then the pinned verify-runner v1.0.75 image (release index sha256:732786a33ebfa6dc1d80d396781df1f31c5331f67a8c0bd97518785f5a72b173, run natively as its linux/arm64 manifest, the Container-Image below; network none, read-only root, every capability dropped, non-root user) running the installed statusgen v1.0.75 verifyrun with the container-netns sandbox against the home mounted read-only, then host close-bracket. The bracket digest check reports verdict=match. SG_OLD_BIN pointed at a v1.0.52 statusgen baseline built for linux/arm64 from the v1.0.52 release commit in the release workflow's from-source mode, because the witness skew tests fail rather than skip without it.

Reading the witness. The image's baked Go module cache predates this tree's tools/desk go.mod. Before any row, verifyrun's module-cache check reported the gap (thirteen requirements, among them go-winio v0.6.3 against v0.6.2, go-isatty v0.0.24 against v0.0.20, and pgpassfile, pgx/v5 and goose/v3 absent) and recorded row 1 could-not-run on it. Rows 2, 4, 5, 6 and 10 send go test output to a file and print only the grep result, so the output that check reads was empty and the witness records them as fail exit=1; the output hash e3b0c44298fc on rows 2, 4, 5 and 10 is the hash of empty output.

Diagnosis (supplemental, no credit). Row 2's go test, re-run in the same image and environment outside verifyrun with its output kept, stops before any test: "open /go/pkg/mod/cache/download/github.com/mattn/go-isatty/@v/v0.0.24.lock: read-only file system", then "FAIL github.com/medici-finance/assay/tools/desk/cmd/scanloop [setup failed]". Every tools/desk row builds that same package, so rows 2, 4, 5 and 10 and the tools/desk half of row 6 stopped on module resolution before their checks ran. Row 6's output is not empty, which fits its statusgen half printing its result line before the tools/desk half stopped. None of these rows observed the delivered code failing. Rows 3, 7 and 8 do not build tools/desk and passed in the bracket.

Supplemental host observation (no acceptance credit, not sandboxed). On the operator desk host, from an export of this source with a throwaway HOME, GOPROXY=off and -mod=readonly, one go test run of the scanloop package over the tests the tools/desk rows name exited 0: TestExitRecord_Validate, TestLandVerb_OneExitPerItem, TestDrainPass_WritesExitRecords, TestIntakeExitSchema_MatchesDoc and four TestExitEvent_ tests passed. This shows the delivered tests still compile and pass against the moved requirements; it gives no row credit.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | could-not-run exit=1 sandbox=container-netns — image module cache does not match go.sum (github.com/jackc/pgx/v5 v5.12.0 vs none; github.com/mattn/go-isatty v0.0.24 vs v0.0.20; github.com/pressly/goose/v3 v3.29.0 vs none): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:8ee34e191218 | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:99e9df6a662f | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | fail exit=1 sandbox=container-netns | sha256:1e0c85581540 | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:bbbe22bc2d80 | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:8025b5f6ba5b | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:140de362d04dafa6fc5821dad725698fcc739d49aa854b2aaaf80c0e9293d19a
Container-Image: sha256:576cb2e49407447769b79a32af370a23e1c108d62842b8f1772a809d1d3b7617
Verification-Attestation: medici-finance/assay#5731 run=45a85d2ceb2de096e0d0186a075ad19810d7da87c7105006777db734f6b4a5ad source=ac53f30aad5e91b717cb9790efef8c4a8917e661 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=45a85d2ceb2de096e0d0186a075ad19810d7da87c7105006777db734f6b4a5ad source=ac53f30aad5e91b717cb9790efef8c4a8917e661 tree=sha256:140de362d04dafa6fc5821dad725698fcc739d49aa854b2aaaf80c0e9293d19a witness=sha256:a96e6b35ddebb334c7cabc380af4a499c7187e7267f5b2421f95e287349f8ea6 opened=2026-11-08T01:01:44Z closed=2026-11-08T01:03:26Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:da5475834c93 | 2026-11-08 | assay-verifier-app[bot] @ ac53f30aad5e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5731 run=45a85d2ceb2de096e0d0186a075ad19810d7da87c7105006777db734f6b4a5ad source=ac53f30aad5e91b717cb9790efef8c4a8917e661 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Row 9 ran on the operator desk host through statusgen verifyrun, with no forge credential variable set in its environment. Its record half ran: jq exited 0 and printed one line naming the newest judgment record, a bug exit whose item and artifact are the same issue on the operator's private house tracker; it is not named here because this repository is public. The operator file holds 562 records, all judgment, unchanged since 2026-11-03, so this is the same record the void 2026-11-06 run read. The record is newer than the #2985 needs-decision record the 2026-10-30 section examined.

Row 9 dereference: not run in this dispatch. The issue read of the item and artifact, with its bug-label check, is a forge read, and this dispatch reserves every forge read to the desk. The earlier blocker, #2862, still stands in code: the scanloop package, which holds the land verb, is unchanged since the 2026-10-30 source, so landing an exit still does not check that the artifact carries the matching label. Whether this artifact now carries the bug label is the desk's read to make.

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-11-08, the verifier App at ac53f30aad5e):

- **Row 1** (2026-11-08): the statusgen and scanloop go test command — expected exit 0; observed could-not-run exit=1 in the container bracket, the image's module cache lacking this tree's tools/desk requirements before the check ran.
- **Row 2** (2026-11-08): TestExitRecord_Validate — expected exit 0; witness fail exit=1 with empty output; the diagnosis above shows go test stopping at package setup on a module the image lacks, so no delivered assertion ran.
- **Row 3** (2026-11-08): TestIntakeExits_MappingAndStamp — expected exit 0; observed exit 0 in the container bracket.
- **Row 4** (2026-11-08): TestLandVerb_OneExitPerItem — expected exit 0; witness fail exit=1 with empty output, the same setup stop as row 2.
- **Row 5** (2026-11-08): TestDrainPass_WritesExitRecords — expected exit 0; witness fail exit=1 with empty output, the same setup stop as row 2.
- **Row 6** (2026-11-08): TestIntakeExitSchema_MatchesDoc in both modules — expected exit 0; witness fail exit=1; its tools/desk half builds the same scanloop package and stops the same way.
- **Row 7** (2026-11-08): the intake-exits JSON report — expected exit 0; observed exit 0 in the container bracket.
- **Row 8** (2026-11-08): the consumers report — expected exit 0; observed exit 0 in the container bracket.
- **Row 9** (2026-11-08): the jq line plus an issue read of item and artifact — record half observed exit 0 with one line; dereference half not run, as above.
- **Row 10** (2026-11-08): TestExitEvent_ — expected exit 0, prints 4; witness fail exit=1 with empty output, the same setup stop as row 2.

Fail-first: no row is risk-bearing. The brief's risk answers are all no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag (the row tags are mutation, flow, neighbour and dereference), so verifyrun's fail-first selection holds no row and none was run.

Risk-bearing value. The brief's risk answers are all no and it is not irreversible. The enumeration covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies: the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern and the 0o600 record mode, with the reversible knobs (lock wait, clip length, session-tag bound) ranked last. Each literal was re-read at this source; none changed since the 2026-10-30 source.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver name, exactly the schema doc's closed role set.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — owner read and write, no group or other bits, as the schema doc requires for a file that may carry a clear dispatch_ref.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so both writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement, so the two grammars are not equivalent; routed to #2668.

**VERIFY: BLOCKED** — rows 1, 2, 4, 5, 6 and 10 stopped on Go module resolution in the pinned verify-runner v1.0.75 image, whose baked module cache predates this tree's tools/desk go.mod (environment; re-run needs an image whose cache matches this source); row 9's dereference half is not run because this dispatch reserves forge reads to the desk, and the #2862 land-verb label gap still stands in code. Rows 3, 7 and 8 pass and row 9's record half passes; no delivered check was observed failing. 3 of 10 rows pass.
### Independent verification — 2026-11-10

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer, under desk tools v1.0.81. Merged source 612750a12406f19a96a83e6ea7687d794f772f9f. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7, an ancestor of this source. Since the 2026-11-08 source ac53f30aad5e91b717cb9790efef8c4a8917e661, the delivered intake-exit files are byte-identical: the scanloop exit-record, land-verb and land files and statusgen/intakeexits.go. The scanloop package did change elsewhere. One commit (45bb73ef8, "fix(scanloop): run fails closed on an unarmed monitor or refused probe") touches its forge, monitor and run files and the monitor tests, and tools/desk go.mod gains three lines.

Rows 1–8 and 10 ran in one closed admission bracket. The host opened the bracket. The pinned verify-runner v1.0.81 image then ran the image's statusgen v1.0.81 verifyrun with the container-netns sandbox, against the home mounted read-only, and the host closed the bracket. The image is pinned by its release index sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59 and runs natively as its linux/arm64 manifest, the Container-Image below. The container ran with network none, a read-only root, every capability dropped, a non-root user, a fresh module cache (the Module-Cache below), GOPROXY=off and -mod=readonly. The bracket digest check reports verdict=match. SG_OLD_BIN pointed at a v1.0.52 statusgen baseline, built for linux/arm64 from the v1.0.52 release commit in the release workflow's from-source mode, because the witness skew tests fail rather than skip without it. This image's module cache matches the tree, so the 2026-11-08 module-resolution stop did not recur: every tools/desk row built and ran.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | fail exit=1 sandbox=container-netns | sha256:f657306260a4 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | pass exit=0 sandbox=container-netns | sha256:af7feb2c47bb | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:422f309d071a | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | pass exit=0 sandbox=container-netns | sha256:6f38a7544752 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | pass exit=0 sandbox=container-netns | sha256:7827e784182d | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | pass exit=0 sandbox=container-netns | sha256:55999241c271 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:97d7cdf4e132 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:ab9297245c77 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | pass exit=0 sandbox=container-netns | sha256:7de1555df0c2 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:41ac76c853fbb5b5feebd9b1f302de531d879080e9dafe10b086691751de6f74
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#5982 run=93f68c553227356e9ba45ac0e10498d4ae63955f90ac909b81a472ec73b902bb source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=93f68c553227356e9ba45ac0e10498d4ae63955f90ac909b81a472ec73b902bb source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:d4f95a1c0fe29ed75cce87e606846b983333bc93778a5f6bc1954a85b6ff6eb0 opened=2026-11-10T07:00:50Z closed=2026-11-10T07:04:20Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:da5475834c93 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5982 run=93f68c553227356e9ba45ac0e10498d4ae63955f90ac909b81a472ec73b902bb source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Row 9 ran on the operator desk host through statusgen verifyrun --rows 9 --dry-run, with no forge credential variable in its environment. The jq record half exited 0 with output hash sha256:da5475834c93, the same hash the 2026-11-08 run recorded. The operator file still holds 562 records, all judgment, and its newest record dates from 2026-11-03. The newest judgment record is a bug exit whose item and artifact are the same issue, on the operator's private house tracker. That issue is not named here because this repository is public.

Row 9 dereference (a forge read under the verifier App identity, outside verifyrun). The item and artifact issue exists and is OPEN. Its labels are help wanted and raised-by:worker. Its label event history shows only those two labels, both added when it was opened on 2026-10-30, and no bug label at any point. The row's Expect requires a bug-exit artifact to carry the bug label, so row 9 FAILS on observed absence. This is the gap #2862 records, and it still stands in code: the land verb is unchanged and still lands a bug exit without checking the artifact's labels.

Row 1 diagnosis (supplemental, no credit). Row 1's command was re-run in the same image and environment outside verifyrun, with its output kept. The scanloop half printed ok. The statusgen half failed only in the root package, on 27 failing test lines from the TestAutonomyGates, TestGateRollupsBatched, TestGateReadFitsDeadline and TestGateRollupOverflow tests. Each reports cause "gh-failed". Every other statusgen package printed ok. These tests put a fake forge CLI first on PATH, but statusgen's stored-login guard refuses every forge CLI launch, the fake included, when no forge credential variable is set and the tree holds the verifier attestation file or the admitted-row git pins are present. The verifier home holds that file. This is the open test-isolation defect #5014, in statusgen's autonomy gate-share tests and outside this brief's delivered code. The delivered statusgen intake-exit tests pass in rows 3 and 6.

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-11-10, the verifier App at 612750a12406):

- **Row 1** (2026-11-10): the statusgen and scanloop go test command. Expected exit 0. Observed fail exit=1 in the container bracket: the statusgen autonomy gate-share tests fail under the admitted-verifier environment (#5014), and the scanloop half passes.
- **Row 2** (2026-11-10): TestExitRecord_Validate. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 3** (2026-11-10): TestIntakeExits_MappingAndStamp. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 4** (2026-11-10): TestLandVerb_OneExitPerItem. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 5** (2026-11-10): TestDrainPass_WritesExitRecords. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 6** (2026-11-10): TestIntakeExitSchema_MatchesDoc in both modules. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 7** (2026-11-10): the intake-exits JSON report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 8** (2026-11-10): the consumers report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 9** (2026-11-10): the jq line plus an issue read of item and artifact. The record half observed exit 0 with one line. The dereference half found the bug-exit artifact without the bug label, so the row fails (#2862).
- **Row 10** (2026-11-10): TestExitEvent_. Expected exit 0, printing 4. Observed exit 0 in the container bracket.

Fail-first: no row is risk-bearing. The brief's risk answers are all no. No Verify row carries a risk-bearing, live, mutating or end-to-end tag (the row tags are mutation, flow, neighbour and dereference). So verifyrun's fail-first selection holds no row, and none was run.

Risk-bearing value. The brief's risk answers are all no, and the brief is not irreversible. The enumeration covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies: the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern and the 0o600 record mode. The reversible knobs (lock wait, clip length, session-tag bound) ranked last. Each literal was re-read at this source, and none changed since the 2026-11-08 source.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver name. This is exactly the schema doc's closed role set.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:383 — owner read and write, no group or other bits. The schema doc requires this for a file that may carry a clear dispatch_ref.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so that both writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement, so the two grammars are not equivalent. Routed to #2668.

**VERIFY: FAIL** — row 9's dereference half observed the newest bug-exit artifact without the bug label: the land verb still records a bug exit without checking the artifact's labels (#2862). Row 1 fails because statusgen's autonomy gate-share tests refuse their fake forge CLI under the admitted-verifier environment, a test-isolation defect outside the delivered code (#5014); its scanloop half passes. Rows 2–8 and 10 pass in a closed container-netns bracket. 8 of 10 rows pass.
### Independent verification — 2026-11-12

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer, under desk tools v1.0.83. Merged source 680102a0877755feab9ef85dc4e1d3f52a137a54. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7, an ancestor of this source. Since the 2026-11-10 source 612750a12406f19a96a83e6ea7687d794f772f9f, statusgen/intakeexits.go and the scanloop land file are byte-identical. One commit (8ee3b6a33, "fix(desk): name the audit lock holder on a busy lock; keep audit-read failure distinct from a blind poll") touches the scanloop exit-record, observations and run files and adds audit-read tests. In the exit-record file it replaces the local audit-lock loop with the shared desk audit lock, so the writer's lock wait now comes from that shared lock's 60-second bound. The record schema, roles, exits, dispatch_ref pattern and record mode are unchanged.

Rows 1–8 and 10 ran in one closed admission bracket. The host opened the bracket. The pinned verify-runner v1.0.83 image then ran the image's statusgen v1.0.83 verifyrun with the container-netns sandbox, against the home mounted read-only, and the host closed the bracket. The image is pinned by its digest and runs natively as its linux/arm64 manifest, the Container-Image below. The container ran with network none, a read-only root, every capability dropped, a non-root user, a fresh module cache (the Module-Cache below), GOPROXY=off and -mod=readonly. The bracket digest check reports verdict=match. SG_OLD_BIN pointed at a v1.0.52 statusgen baseline, built for linux/arm64 from the v1.0.52 release commit, because the witness skew tests fail rather than skip without it.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | fail exit=1 sandbox=container-netns | sha256:a90bc45ce530 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | pass exit=0 sandbox=container-netns | sha256:32db5cd0f789 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:b45fa94248f2 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | pass exit=0 sandbox=container-netns | sha256:6f38a7544752 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | pass exit=0 sandbox=container-netns | sha256:7827e784182d | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | pass exit=0 sandbox=container-netns | sha256:55999241c271 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:4017a683bf20 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:73c08ec8f327 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | pass exit=0 sandbox=container-netns | sha256:7de1555df0c2 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:f248930a0b58b0881b8dd4d1a4ee31e42201ff3a11b9e2dcc2f255b3264942b9
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6294 run=edb35c661f921341db66bc51634a2bba92a3ad78b89c6a8382eb8b88e1857c0c source=680102a0877755feab9ef85dc4e1d3f52a137a54 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=edb35c661f921341db66bc51634a2bba92a3ad78b89c6a8382eb8b88e1857c0c source=680102a0877755feab9ef85dc4e1d3f52a137a54 tree=sha256:f248930a0b58b0881b8dd4d1a4ee31e42201ff3a11b9e2dcc2f255b3264942b9 witness=sha256:b8c5e7601e09e0fae0b7295065f62b6b188e8e87717ff4b33829227634a4555f opened=2026-11-12T20:38:19Z closed=2026-11-12T20:41:04Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:b17ebdd34e8e | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6294 run=edb35c661f921341db66bc51634a2bba92a3ad78b89c6a8382eb8b88e1857c0c source=680102a0877755feab9ef85dc4e1d3f52a137a54 brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Row 9 ran on the operator desk host through statusgen verifyrun --rows 9 --dry-run, with no forge credential variable in its environment. The jq record half exited 0 and printed one line. Its output hash, sha256:b17ebdd34e8e, differs from the 2026-11-10 hash because the operator file has grown: it now holds 590 records, all judgment, and its newest record was triaged on 2026-11-12 by the intake desk at tier strong. That newest judgment record is a bug exit whose item and artifact are the same issue, on the operator's private house tracker. The issue is not named here because this repository is public.

Row 9 dereference (a forge read under the verifier App identity, outside verifyrun). The item and artifact issue exists and is OPEN. Its labels are bug, raised-by:reviewer, impact:low, risk:med and effort:s. Its label event history shows raised-by:reviewer added when it was opened, and bug added with the three triage labels at 2026-11-12T19:57:37Z, five seconds before the record's triage time. The row's Expect is that both issues exist and that a bug-exit artifact carries the bug label, and it holds, so row 9 passes. The land verb still does not check an artifact's labels before it records an exit (#2862 stays open). This record passes because the triager labelled the issue before landing, not because the land verb enforced it.

Row 1 diagnosis (supplemental, no credit). The witness keeps only an output hash, so row 1's two halves were re-run in the same image and container settings outside verifyrun, with output kept. The scanloop half printed ok. The statusgen half was run twice. With no git variables injected, it failed only in the root package, on 11 top-level tests (27 failing test lines) from the TestAutonomyGates, TestGateRollupsBatched, TestGateReadFitsDeadline and TestGateRollupOverflow families. Each reports cause "gh-failed": the stored-login guard refuses the tests' fake forge CLI when no forge credential variable is set and the tree holds the verifier attestation file. That is the open test-isolation defect #5014, the same set the 2026-11-10 run found. With exactly the git variables verifyrun's admitted row environment appends (GIT_NO_REPLACE_OBJECTS, GIT_ATTR_NOSYSTEM, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT, GIT_ASKPASS, and GIT_CONFIG_COUNT=2 with the two grep keys), it also failed only in the root package, on 32 top-level tests. They are those 11 plus 21 that refuse the injected git configuration: the acceptance-transition, snapshot-approval, derived-history, cross-source coverage, expect-transition, flow-card and go-git session and read-parity tests. Each reports "injected Git configuration is unsupported" or the "native discovery/config/shallow environment profile" fallback. That is the open #2715 class, which has grown since #2715 was filed. Every other statusgen package printed ok in both runs. Both classes are outside the delivered intake-exit code; rows 3 and 6 run the delivered statusgen intake-exit tests, and they pass. This run's row 1 blocker is filed as #6298, with the command, the output and both test lists.

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-11-12, the verifier App at 680102a08777):

- **Row 1** (2026-11-12): the statusgen and scanloop go test command. Expected exit 0. Observed fail exit=1 in the container bracket. That result is an instrument result, not an observed defect in the deliverable. Only the root statusgen package fails, in two test-isolation classes that the verifier environment triggers (#5014 and #2715), and both are outside the delivered intake-exit code. The scanloop half and every other statusgen package pass. The row's obligation stays uncovered and is recorded as blocked in #6298.
- **Row 2** (2026-11-12): TestExitRecord_Validate. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 3** (2026-11-12): TestIntakeExits_MappingAndStamp. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 4** (2026-11-12): TestLandVerb_OneExitPerItem. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 5** (2026-11-12): TestDrainPass_WritesExitRecords. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 6** (2026-11-12): TestIntakeExitSchema_MatchesDoc in both modules. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 7** (2026-11-12): the intake-exits JSON report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 8** (2026-11-12): the consumers report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 9** (2026-11-12): the jq line plus an issue read of item and artifact. The record half observed exit 0 with one line. The dereference half found the item and artifact OPEN, with the bug label on the bug-exit artifact, so the row passes.
- **Row 10** (2026-11-12): TestExitEvent_. Expected exit 0, printing 4. Observed exit 0 in the container bracket.

Fail-first: no row is risk-bearing. The brief's risk answers are all no. No Verify row carries a risk-bearing, live, mutating or end-to-end tag (the row tags are mutation, flow, neighbour and dereference). So verifyrun's fail-first selection holds no row, and none was run.

Risk-bearing value. The brief's risk answers are all no, and the brief is not irreversible. The enumeration covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies: the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern and the 0o600 record mode. The reversible knobs (lock wait, clip length, session-tag bound) ranked last. Each literal was re-read at this source. The only change since the 2026-11-10 source is that the record-mode line moved from line 383 to line 366 and the lock wait now comes from the shared desk audit lock (60 seconds), a reversible knob.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver name. This is exactly the schema doc's closed role set.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:366 — owner read and write, no group or other bits. The schema doc requires this for a file that may carry a clear dispatch_ref.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so that both writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement, so the two grammars are not equivalent. Routed to #2668.

RISK-VALUE: DERIVED — exitLockWait = deskkit.AuditLockWait = 60 * time.Second @ tools/desk/cmd/scanloop/exitrecord.go:268 and tools/desk/internal/deskkit/auditlock.go:27 — the exit writer now shares the desk-wide audit lock bound rather than a local one. This is a reversible knob, ranked last.

**VERIFY: BLOCKED** — no delivered obligation was observed failing. Rows 2–10 pass, 9 of 10. Row 1 exits 1 in the closed container-netns bracket for instrument causes only. Its scanloop half and every statusgen package except the root package pass. The root package fails in two test-isolation classes that the verifier environment triggers. The autonomy gate-share tests refuse their fake forge CLI when the home holds the verifier attestation file (#5014). The git-fixture and acceptance-transition tests refuse the admitted row environment's injected git configuration (#2715). Both are outside the delivered intake-exit code. The blocker is recorded in #6298, kind environment. Its next actor is a worker on #5014 and #2715; after those land, row 1 is re-run. Row 9 now passes: the newest bug-exit artifact carries the bug label. The land verb still does not enforce that label (#2862).

### Independent verification — 2026-11-13

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong), non-implementer, under desk tools v1.0.83. Merged source f9c836b11e51bd5f9661b0da40f7a63ed1c18f0e. Delivering change: #2424, merged as bfef0dab238ad5aad468f736d78daa8c4c236dd7, an ancestor of this source. Since the 2026-11-12 source 680102a0877755feab9ef85dc4e1d3f52a137a54, the delivered intake-exit files are byte-identical: the scanloop exit-record, land-verb and land files, statusgen/intakeexits.go, statusgen/registerentries.go and spec/intake-exit-v1.md. The scanloop package changed elsewhere. Two commits (f97421582, "fix(scanloop): a fresh scan carrier leaves out what open carriers carry", and d8815f6c7, "fix(desk): read known-kind GitLab numbers through the typed operations") add the open-carrier exclusion to the scan dispatch (the new carriers file, and the adapter, lane, discover, run and trust files) without touching the record writer or the land verb.

Rows 1–8 and 10 ran in one closed admission bracket. The host opened the bracket. The pinned verify-runner v1.0.83 image then ran the image's statusgen v1.0.83 verifyrun with the container-netns sandbox, against the home mounted read-only, and the host closed the bracket. The image is pinned by its digest and runs natively as its linux/arm64 manifest, the Container-Image below. The container ran with network none, a read-only root, every capability dropped, a non-root user, a fresh module cache (the Module-Cache below), GOPROXY=off and -mod=readonly. The bracket digest check reports verdict=match. SG_OLD_BIN pointed at a v1.0.52 statusgen baseline, built for linux/arm64 from the v1.0.52 release commit, because the witness skew tests fail rather than skip without it. The first close step, run from outside the home, refused with two identical brief paths (#5182); the same close step run from inside the home appended the Admission-Bracket line.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && go test ./...) && (cd tools/desk && go test ./cmd/scanloop/...)` | fail exit=1 sandbox=container-netns | sha256:fb8ce8657ad4 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitRecord_Validate$' -v > "${TMPDIR:-/tmp}/b33-validate.out" 2>&1 && grep -F -e '--- PASS: TestExitRecord_Validate' "${TMPDIR:-/tmp}/b33-validate.out"` | pass exit=0 sandbox=container-netns | sha256:ffd63cabe7c5 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test . -run '^TestIntakeExits_MappingAndStamp$' -v > "${TMPDIR:-/tmp}/b33-map.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExits_MappingAndStamp' "${TMPDIR:-/tmp}/b33-map.out"` | pass exit=0 sandbox=container-netns | sha256:07157ae343ed | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestLandVerb_OneExitPerItem$' -v > "${TMPDIR:-/tmp}/b33-land.out" 2>&1 && grep -F -e '--- PASS: TestLandVerb_OneExitPerItem' "${TMPDIR:-/tmp}/b33-land.out"` | pass exit=0 sandbox=container-netns | sha256:6f38a7544752 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestDrainPass_WritesExitRecords$' -v > "${TMPDIR:-/tmp}/b33-flow.out" 2>&1 && grep -F -e '--- PASS: TestDrainPass_WritesExitRecords' "${TMPDIR:-/tmp}/b33-flow.out"` | pass exit=0 sandbox=container-netns | sha256:7827e784182d | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && go test . -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sg-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sg-schema.out") && (cd tools/desk && go test ./cmd/scanloop/ -run '^TestIntakeExitSchema_MatchesDoc$' -v > "${TMPDIR:-/tmp}/b33-sl-schema.out" 2>&1 && grep -F -e '--- PASS: TestIntakeExitSchema_MatchesDoc' "${TMPDIR:-/tmp}/b33-sl-schema.out")` | pass exit=0 sandbox=container-netns | sha256:55999241c271 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --intake-exits --json --root .` | pass exit=0 sandbox=container-netns | sha256:4017a683bf20 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | pass exit=0 sandbox=container-netns | sha256:cc63dbf0dbaf | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/scanloop/ -run '^TestExitEvent_' -v > "${TMPDIR:-/tmp}/b33-event.out" 2>&1 && grep -c -E -e '^--- PASS: TestExitEvent_' "${TMPDIR:-/tmp}/b33-event.out"` | pass exit=0 sandbox=container-netns | sha256:7de1555df0c2 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:ddf3f991f55d0a522bcbc08983a0f8ff908262ab3891e6b58cb7c43f76d46bfa
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6406 run=fac969c2778338384555ec070d5f31844d667902defba1975e1124d69e705f3b source=f9c836b11e51bd5f9661b0da40f7a63ed1c18f0e brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=fac969c2778338384555ec070d5f31844d667902defba1975e1124d69e705f3b source=f9c836b11e51bd5f9661b0da40f7a63ed1c18f0e tree=sha256:ddf3f991f55d0a522bcbc08983a0f8ff908262ab3891e6b58cb7c43f76d46bfa witness=sha256:c12e29ce4edb8e9c766e9f89e8e58e2b71795b7fee47a3a8e603d1157cd6d65d opened=2026-11-13T07:02:02Z closed=2026-11-13T07:04:24Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 9 | `jq -rs 'map(select(.decided_by == "judgment")) \| last \| "\(.item) \(.exit) \(.artifact)"' "$HOME/.config/assay/intake-exits.jsonl"` | pass exit=0 | sha256:25e617a2c0f5 | 2026-11-13 | assay-verifier-app[bot] @ f9c836b11e51 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6406 run=fac969c2778338384555ec070d5f31844d667902defba1975e1124d69e705f3b source=f9c836b11e51bd5f9661b0da40f7a63ed1c18f0e brief=docs/streams/desk-supervision/brief-33-intake-exit-record.md model=claude-opus-5-5 tier=strong

Row 9 ran on the operator desk host through statusgen verifyrun --rows 9 --dry-run, with no forge credential variable in its environment. The jq record half exited 0 and printed one line. Its output hash, sha256:25e617a2c0f5, differs from the 2026-11-12 hash because the operator file has grown: it now holds 634 records, all judgment, and its newest record was triaged on 2026-11-13 by the intake desk at tier strong. That newest judgment record is a bug exit whose item and artifact are the same issue, medici-finance/assay#2337.

Row 9 dereference (a forge read under the verifier App identity, outside verifyrun). Issue #2337 exists and is OPEN. Its labels are bug, raised-by:verifier, impact:low, risk:low and effort:s. Its label event history shows bug added at 2026-10-07T12:51:53Z, three seconds after the issue was opened, and never removed. The row's Expect is that both the item and the artifact exist and that a bug-exit artifact carries the bug label. That holds, so row 9 passes. The land verb still does not check an artifact's labels before it records an exit (#2862 stays open); this record passes because the issue already carried the label.

Row 1 diagnosis (supplemental, no credit). The witness keeps only an output hash, so row 1's two halves were re-run in the same image and container settings outside verifyrun, with exactly the git variables verifyrun's admitted row environment appends (GIT_NO_REPLACE_OBJECTS, GIT_ATTR_NOSYSTEM, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT, GIT_ASKPASS, and GIT_CONFIG_COUNT=2 with the two grep keys), and with output kept. The scanloop half printed ok. The statusgen half failed only in the root package, on 32 top-level tests, the same set the 2026-11-12 run found. Eleven are from the TestAutonomyGates, TestGateRollupsBatched, TestGateReadFitsDeadline and TestGateRollupOverflow families and report cause "gh-failed": the stored-login guard refuses the tests' fake forge CLI when no forge credential variable is set and the tree holds the verifier attestation file (#5014). The other 21 are the acceptance-transition, snapshot-approval, derived-history, cross-source coverage, expect-transition, flow-card and go-git session and read-parity tests, and each reports "injected Git configuration is unsupported" or the "native discovery/config/shallow environment profile" fallback (#2715). Every other statusgen package printed ok. Both classes are outside the delivered intake-exit code, and both issues are still open. Rows 3 and 6 run the delivered statusgen intake-exit tests, and they pass. The row 1 blocker stays recorded in #6298.

Per-row observations, checked against the witness tables above, which carry each row's Date and Runner (2026-11-13, the verifier App at f9c836b11e51):

- **Row 1** (2026-11-13): the statusgen and scanloop go test command. Expected exit 0. Observed fail exit=1 in the container bracket. That result is an instrument result, not an observed defect in the deliverable. Only the root statusgen package fails, in the two test-isolation classes the verifier environment triggers (#5014 and #2715), both outside the delivered intake-exit code. The scanloop half and every other statusgen package pass. The row's obligation stays uncovered and is recorded as blocked in #6298.
- **Row 2** (2026-11-13): TestExitRecord_Validate. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 3** (2026-11-13): TestIntakeExits_MappingAndStamp. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 4** (2026-11-13): TestLandVerb_OneExitPerItem. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 5** (2026-11-13): TestDrainPass_WritesExitRecords. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 6** (2026-11-13): TestIntakeExitSchema_MatchesDoc in both modules. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 7** (2026-11-13): the intake-exits JSON report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 8** (2026-11-13): the consumers report. Expected exit 0. Observed exit 0 in the container bracket.
- **Row 9** (2026-11-13): the jq line plus an issue read of item and artifact. The record half observed exit 0 with one line. The dereference half found the item and artifact OPEN, with the bug label on the bug-exit artifact, so the row passes.
- **Row 10** (2026-11-13): TestExitEvent_. Expected exit 0, printing 4. Observed exit 0 in the container bracket.

Fail-first: no row is risk-bearing. The brief's risk answers are all no. No Verify row carries a risk-bearing, live, mutating or end-to-end tag (the row tags are mutation, flow, neighbour and dereference). So verifyrun's fail-first selection holds no row, and none was run.

Risk-bearing value. The brief's risk answers are all no, and the brief is not irreversible. The enumeration covered every literal the delivering change introduces in the scanloop exit-record, land and statusgen intake-exits files, plus the Task's vocabularies: the schema name, the sixteen record keys, the six-member role set, the five tracked exits, the dispatch_ref pattern and the 0o600 record mode. The reversible knobs (lock wait, clip length, session-tag bound) ranked last. Each literal was re-read at this source, and none changed since the 2026-11-12 source; every line number below is unchanged.

RISK-VALUE: DERIVED — intakeExitRoles = ["the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"] @ tools/desk/cmd/scanloop/exitrecord.go:72 and statusgen/intakeexits.go:61 — the five canonical desk loop names (canonicalLoopNames, tools/desk/internal/deskkit/loopnames.go:77) plus the neutral driver name. This is exactly the schema doc's closed role set.

RISK-VALUE: DERIVED — record file mode = 0o600 @ tools/desk/cmd/scanloop/exitrecord.go:366 — owner read and write, no group or other bits. The schema doc requires this for a file that may carry a clear dispatch_ref.

RISK-VALUE: DERIVED — intakeExitSchema = "intake-exit-v1" @ tools/desk/cmd/scanloop/exitrecord.go:29 and statusgen/intakeexits.go:27 — Task step 1 fixes this literal so that both writers' records concatenate under one schema.

RISK-VALUE: DERIVED — trackedExits = [ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching] @ tools/desk/cmd/scanloop/land.go:41 — the five exits of the schema doc's exit field. Unrouted is a pending state, never an exit.

RISK-VALUE: NAMED, NOT DERIVED — dispatchRefRe = `^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/cmd/scanloop/exitrecord.go:100 — the authoritative desk-supervision/28 validator at tools/desk/internal/deskkit/verifyoutcomejoin.go:32 takes a 1 to 226 byte printable-ASCII prefix with no double-hyphen requirement, so the two grammars are not equivalent. Routed to #2668.

RISK-VALUE: DERIVED — exitLockWait = deskkit.AuditLockWait = 60 * time.Second @ tools/desk/cmd/scanloop/exitrecord.go:268 and tools/desk/internal/deskkit/auditlock.go:27 — the exit writer shares the desk-wide audit lock bound. This is a reversible knob, ranked last.

**VERIFY: BLOCKED** — no delivered obligation was observed failing. Rows 2–10 pass, 9 of 10. Row 1 exits 1 in the closed container-netns bracket for instrument causes only. Its scanloop half and every statusgen package except the root package pass. The root package fails in the same two test-isolation classes as on 2026-11-12. The autonomy gate-share tests refuse their fake forge CLI when the home holds the verifier attestation file (#5014). The git-fixture and acceptance-transition tests refuse the admitted row environment's injected git configuration (#2715). Both are outside the delivered intake-exit code, and both are still open. The blocker stays recorded in #6298, kind environment. Its next actor is a worker on #5014 and #2715; after those land, row 1 is re-run. Row 9 passes again: the newest bug-exit artifact carries the bug label. The land verb still does not enforce that label (#2862).

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
