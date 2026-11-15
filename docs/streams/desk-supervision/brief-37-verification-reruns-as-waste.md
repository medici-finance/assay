---
brief: assay:assay:desk-supervision:37
title: Verification re-runs as waste — every attempt on the outcome record, joined to measured cost
why: >-
  When a verification has to be run again (because the tools were built from source, the
  environment broke a row, or the credit check failed), the extra runs leave no record. Only the
  final verdict is stored. So nobody can say "brief N needed four verification runs and three of
  them were wasted", or how much of the cost and time already measured for that brief went to
  runs that earned nothing. This brief records every attempt and its disposition on the outcome
  record, joins each attempt to the cost and time rows the desk already measures (it adds no new
  cost measure), and turns "how often did we re-run this, and what did the waste cost" into one
  query.
wave: 0
depends: []
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4163]
schema: brief-v2
version: 1
authored: 2026-10-22 by an authoring session for the coordinator desk (issue 4163, with the driver's correction of the same day)
exec-tier: strong
exec-tier-why: >-
  (b) one new field crosses five components that must agree: the deskkit validator, the
  deskevidence writer, two tolerant readers (statusgen and verifyloop) and a perf-history join
  in deskperf; (c) the honest-cost property (never apportion, never print zero for
  "not measured") is a regression a happy-path test cannot catch.
domain: complicated
outcome: none
sources:
  - 'desk-supervision: Verification re-runs as waste — every attempt on the outcome record, joined to measured cost'
  - "issue #4163 (question from the driver): a verifier run that earned no credit leaves no structured trace; the outcome record has no attempts field and perf rows have no verification-attempt mark. Case: graph-execution/02 on 2026-10-22, one dispatch producing an uncredited source-built 9/9 table, a discarded installed-tools table (row 5 failed for an environmental reason), and a final credited table plus one `blocked` outcome record"
  - "driver correction 2026-10-22, relayed by the coordinator desk: perf already measures desk cost and the total time a brief takes, and tool costs are credited there. Add NO new token, model or wall-cost field. Reuse those measures and centre the brief on uncredited work as waste: record every attempt with its disposition and a join key to the existing cost rows, derive the per-brief re-run count and the uncredited share of the already-measured cost and time, and make 'brief N needed X runs, Y uncredited, cost Z' one query"
  - "docs/performance-attribution-spec.md — workflow-profile attribution (`agentId`, `role`, typed `work` refs, local-only, 7 UTC-day retention, raw work refs never exported) and the perf-v4 successor section; this brief reads those rows and adds no field or extension to them"
  - "desktools-v2/63 and desktools-v2/64 (implemented): the workflow profile and its propagation through `deskdispatch` child contexts; desktools-v2/67 and desktools-v2/68 (todo): the perf-v4 envelope, whose readers stay compatible with the rows this brief reads"
  - "desk-supervision/34 (todo): `usage-v1` token and wall counts per worker dispatch; review and verify dispatches are explicitly out of its scope, so verifier token cost stays could-not-check here"
  - "graph-execution/07 (implemented): `statusgen --flow` per-brief implemented-to-verified dwell, which remains the brief's verification lead time; docs/streams/continuing-operations/cockpit.md shows lead time observed and service time unknown until actual evidence exists"
  - "desk-supervision/16 and desk-supervision/27 (implemented): verify-wake-v1 receipt, its join keys (`started`, `finished`, `rows_total`, `dispatch_ref`) and redaction rules"
  - freshness-checked 2026-10-22 @ 866a169ae (origin/main)
design-fit:
  owner: tools/desk/internal/deskkit (verifyoutcomejoin.go owns the outcome-record join-key shape)
  contract: "none — docs/contracts.md has no semantic-owner row for verify-outcome records; the field contract stays docs/streams/desk-supervision/verify-wake-v1.md, which this brief amends"
  retires: ["prose-only accounts of discarded and uncredited verification runs in Evidence notes and PR bodies (the issue's case), which become one structured field"]
  weight: "+1 optional record key (attempts, 6 sub-keys, 2 closed reason sets), +1 read-only deskperf verb with 3 flags, +5 refusals (attempt shape), +~40 rule-text lines across verify-wake-v1.md, the verifier kit and the verify-desk skill; 0 new cost measures, 0 new perf fields or extensions, 0 new stores"
  why-add: "The attempt history exists only inside one verifier dispatch and is lost when it ends; the outcome record is the one durable per-dispatch record the verifier already writes, so attempts belong there. Considered instead: a perf-v4 dispatch-id extension (the issue's proposal), rejected because perf rows are local-only, pruned after 7 days and need a privacy review per new extension, so a re-run count kept there would vanish within a week; and new cost fields on the record, rejected by the driver because perf already measures that cost."
consumers:
  - "tools/desk/internal/deskkit/verifyoutcomejoin.go (joinTopKeys, ParseJoinInput, checkRunMetadataShape gain `attempts`): fixed-here"
  - "tools/desk/internal/deskkit/verifyattempts.go (attempt type, validator, per-brief rollup): fixed-here"
  - "tools/desk/cmd/deskevidence/outcomerecord.go (the writer accepts and lands `attempts` unchanged): fixed-here"
  - "tools/desk/cmd/deskperf/ (new read-only verb `reruns`; the package header's read-scope sentence is amended): fixed-here"
  - "statusgen/verifyoutcomes_additive_test.go (the additive-key pin gains `attempts`): fixed-here"
  - "tools/desk/cmd/verifyloop/outcomerecords_test.go (a record carrying `attempts` still plans and wakes as before): fixed-here"
  - "tools/desk/cmd/deskdispatch/references/verifier-prompt.md (fixed-shape attempt reporting): fixed-here"
  - "plugins/assay/skills/verify-desk/SKILL.md (the dispatch step passes the brief as the perf work ref; the outcome-record step carries `attempts`): fixed-here"
  - "docs/streams/desk-supervision/verify-wake-v1.md and docs/streams/verify-outcomes/README.md (field table and register description): fixed-here"
  - "perfrec/ and the perf-v4 envelope (desktools-v2/67, desktools-v2/68): out-of-scope (read only; this brief adds no perf field, extension or producer change, and joins on the workflow attribution those briefs already keep)"
  - "statusgen --flow (graph-execution/07): out-of-scope (the brief's verification lead time stays there; this report does not recompute or duplicate it)"
  - "verifier dispatch token usage (desk-supervision/34's deferred review/verify follow-up): out-of-scope (34 records worker dispatches only; until that follow-up lands, verifier token cost is printed as could-not-check, never estimated)"
  - "verifier dispatches that ended without landing any outcome record: out-of-scope (no record exists to carry attempts; the report says so in one line rather than counting them as zero)"
---

# Brief 37 — Verification re-runs as waste — every attempt on the outcome record, joined to measured cost

## Context
files:
- **edit** `tools/desk/internal/deskkit/verifyoutcomejoin.go`: `attempts` joins `joinTopKeys` (line 130); `ParseJoinInput` (line 149) decodes it; `checkRunMetadataShape` (line 264) calls the new validator.
- **add** `tools/desk/internal/deskkit/verifyattempts.go` and `tools/desk/internal/deskkit/verifyattempts_test.go`: the `VerifyAttempt` type, `CheckAttempts`, `RollupAttempts` (per-brief counts over every record), and `PerfBriefRef` (the qualified perf work ref for a brief).
- **edit** `tools/desk/cmd/deskevidence/outcomerecord.go` and `tools/desk/cmd/deskevidence/outcomerecord_test.go`: the writer lands `attempts` as validated.
- **add** `tools/desk/cmd/deskperf/reruns.go` and `tools/desk/cmd/deskperf/reruns_test.go`; **edit** `tools/desk/cmd/deskperf/main.go` (verb dispatch, usage text, package header).
- **add** `tools/desk/cmd/deskperf/testdata/reruns/`: the graph-execution/02-shaped fixture (two outcome records, one perf day file).
- **edit** `statusgen/verifyoutcomes_additive_test.go` and `tools/desk/cmd/verifyloop/outcomerecords_test.go`: neighbour rows.
- **edit** `tools/desk/cmd/deskdispatch/references/verifier-prompt.md`, `plugins/assay/skills/verify-desk/SKILL.md`, `docs/streams/desk-supervision/verify-wake-v1.md`, `docs/streams/verify-outcomes/README.md`, `tools/desk/README.md`.
- **add** `changelog/desk-supervision-37.md`.

facts:
- **The outcome record today (read 2026-10-22 @ 866a169ae).** One JSON object per landed
  verdict under `docs/streams/verify-outcomes/<stream>/<NN>-*.json`, written only by
  `deskevidence --outcome-record` (`tools/desk/cmd/deskevidence/outcomerecord.go`; order:
  `deskkit.CheckRecordKeys` → future-ts check → `deskkit.ParseJoinInput` → receipt →
  `redactOutcomeRecord` in `outcomejoin.go` → `tieOutcomeTarget`). Records are immutable and keys
  are additive (`verify-wake-v1.md` lines 36-38). The join keys `started`, `finished` (RFC3339
  UTC, `started <= finished <= ts`) and `rows_total` already exist (`verify-wake-v1.md` Fields
  table, lines 40-63). A verifier dispatch writes ONE record, so a record is a dispatch; earlier
  runs inside that dispatch appear only in prose, if at all (issue #4163).
- **Readers tolerate unknown keys.** `deskkit.ParseRecord` (`verifyoutcomes.go` line 79) reads
  only `brief`/`ts`; `ReadVerifyOutcomes` (line 181) returns every record; `LatestPerBrief`
  (line 298) keeps one. verifyloop's `readVerifyOutcomeRecords` (`tools/desk/cmd/verifyloop/briefscan.go`)
  uses plain `json.Unmarshal`. statusgen pins additive keys as invisible in
  `TestVerifyOutcomes_AdditiveKeys` (`statusgen/verifyoutcomes_additive_test.go`). Old records
  therefore stay valid with no migration: every new key is optional.
- **Credit is already defined; this brief only records it.** verify-desk `SKILL.md`
  (around lines 196-230): a row is credited only on `statusgen verifyrun --bracket-digest
  --witness <brief> --credit` printing `credit=credited`; `not-credited` and `could-not-check`
  are not credit. §"Qualifying a repaired verifier installation" (line 236): a locally built
  binary in an admitted run is never credit. These are the uncredited reason classes below.
- **Cost and time are already measured; reuse, never re-measure.** Workflow-profile perf rows
  (`docs/performance-attribution-spec.md` lines 61-82; desktools-v2/63, desktools-v2/64) carry
  `role` (`verify` for a `--kit verifier` dispatch, `tools/desk/cmd/deskdispatch/dispatch.go` line 233), typed `work` refs
  (`perfrec.WorkRef{Kind, Repo, Brief, ...}`, `perfrec/context.go` line 65), a fresh random
  `agentId` per dispatch (`perfrec.ChildContext`, `perfrec/lifecycle.go` line 13), and the
  measures `deskperf report --perf-profile workflow` already sums: `perfrec.V3Group`'s
  `sumOwnProcessWallMs`, `sumOwnCpuMs` (`perfrec/reduce.go` line 14) and the forge request
  rollups (`deskkit.SummarisePerfV3RequestsByTool`, `perfday.go` line 259). The rows are
  local-only and pruned to 7 UTC days (`tools/desk/cmd/deskperf/main.go` header). `deskdispatch` passes a
  work ref to the child only when `--perf-work` is given (`dispatch.go` lines 237-241); the
  verify-desk skill does not pass it today, so verifier perf rows currently carry no brief.
- **A brief work ref takes the qualified id only.** `WorkRef.Brief` must match `briefRE`
  (`perfrec/context.go` line 124): `<a>:<b>:<stream>:<N>`, four colon-separated segments and a
  number with no leading zero (in-tree examples: `statusgen/perfrecord_test.go` line 785 uses
  `assay:assay:statusgen:26`). A `<stream>/<NN>` value fails `validWork`, and the resolver then
  drops the whole `work` field with coverage `invalid` (`perfrec/context.go` lines 282-286). The row is still
  written, with no brief, and nothing is refused at dispatch time. A `V3Filter.Work` in that form is
  refused outright (`perfrec/reduce.go` line 87). The brief file's frontmatter `brief:` value is
  the qualified id except for zero padding: `assay:assay:desk-supervision:37` is valid as written,
  but `assay:assay:graph-execution:02` (the frontmatter of graph-execution/02) fails `briefRE`
  and becomes `assay:assay:graph-execution:2`.
  The brief's total verification lead time is graph-execution/07's `statusgen --flow` dwell;
  token cost per dispatch is desk-supervision/34's `usage-v1`, worker dispatches only.
- **Join key choice.** The attempt joins to perf rows by `(work ref with kind=brief, repo = the
  root's repo, brief = the qualified id from PerfBriefRef; role=verify; row ts within
  [attempt.started, attempt.finished])`. This puts no perf identifier (`agentId`,
  `sessionId`) on the record, so nothing local-only reaches a register, public or private.
  Verifier claims are exclusive per brief, so two verifier dispatches for one brief do not
  overlap. When they appear to (more than one distinct `agentId` inside one attempt window),
  the attempt's cost is reported `ambiguous`, never split.
- **Refusal convention.** `deskkit.Refused(...)` → exit 5; unverifiable → exit 6
  (`exitcodes.go`). `deskperf` treats an empty perf history as a reason-named refusal, never an
  all-zeroes report (`tools/desk/cmd/deskperf/main.go` `load`).
- single-point-of-failure: the deskkit attempt validator (`CheckAttempts`) is what keeps a
  malformed or self-contradicting attempt list off the register. Behind it is the reader:
  `deskperf reruns` re-validates every stored record with the same pure function and reports a
  failing record as `invalid` (path named) instead of counting it. The two fail at different
  times (write vs read) for different reasons (a writer bypass or hand-landed record is caught only
  at read). Verify row 4 proves the reader with the writer bypassed.

layering: no new component. The validator and rollup are pure functions in deskkit next to
the existing join-key checks; `deskevidence` stays the only writer; `deskperf` stays the
read-only perf reader and gains one verb that reads the register plus the perf day files it
already reads (Task 1, Task 4; Verify rows 1-6).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Add no token, model, wall-time or cost field to the outcome record, and no field or extension to perf rows (driver correction, 2026-10-22).
- Never estimate: a measure that is missing, pruned or ambiguous is printed as `could-not-check` or `ambiguous` with its reason, never as zero and never apportioned.

## Task
1. **Attempt shape and validator (`tools/desk/internal/deskkit/verifyattempts.go`).**
   `attempts` is an optional list on the record, 1 to 16 entries. Each entry has exactly these keys:
   - `attempt`: integer; the list is numbered 1..n contiguously in order.
   - `disposition`: `credited` | `uncredited` | `discarded`.
   - `reason`: absent for `credited`. For `uncredited` it is one of `local-build` (a locally built
     binary in an admitted run), `not-credited` (`--credit` printed `credit=not-credited`) or
     `credit-unchecked` (`credit=could-not-check`). For `discarded` it is one of `env-fault` (an
     environmental cause failed rows; the run was thrown away and re-run) or `superseded`
     (replaced by a later attempt for any other stated cause).
   - `cause_ref`: optional; same grammar and checks as `blocker_ref`; REQUIRED when `reason` is `env-fault`.
   - `started`, `finished`: RFC3339 UTC with `started <= finished`. Attempts do not overlap and
     ascend: `record.started <= a[1].started`, `a[i].finished <= a[i+1].started`,
     `a[n].finished <= record.finished`. A record with `attempts` must therefore carry
     `started`/`finished`.
   - `rows_run`: integer, `1 <= rows_run <= rows_total`. A record with `attempts` must carry `rows_total`.

   Cross-entry rules: at most one `credited` entry, and when present it is the last one; a record
   whose `outcome` is `verified` (or legacy `pass`) and carries `attempts` must end in a
   `credited` entry. Unknown or case-variant keys inside an entry are refused, as
   `ParseJoinInput` already refuses them at top level. Every refusal names the attempt
   number and the rule. Add `attempts` to `joinTopKeys` and call `CheckAttempts` from
   `checkRunMetadataShape`. Absent `attempts` remains valid. No redaction change: the entries
   carry no identifier beyond `cause_ref`, which the existing public-repo gate already checks like
   `blocker_ref`.

   In the same file, add `PerfBriefRef(frontmatterID string) (string, error)`. It is pure. It takes a
   brief file's frontmatter `brief:` value (`<a>:<b>:<stream>:<NN>`), drops the leading zeros of
   the number, and returns the result only when it matches perfrec's `briefRE`. Otherwise it
   refuses, naming the value. It is the single mapping from a brief to its perf work ref. The
   report join (step 4) and the skill's dispatch value (step 5) both use it, so the writer and
   reader cannot drift apart.
2. **Writer (`tools/desk/cmd/deskevidence/outcomerecord.go`).** `--outcome-record` lands
   `attempts` byte-for-byte as validated. A refusal from step 1 is exit 5 and writes nothing.
3. **Per-brief rollup (`deskkit.RollupAttempts`).** Over EVERY record for a brief
   (`ReadVerifyOutcomes`, not `LatestPerBrief`), produce: `runs` (the sum of attempt counts; a
   record with no `attempts` counts as 1 run with disposition `unrecorded`, because its internal
   re-runs cannot be checked), counts by disposition and by reason, and per attempt its window.
   Elapsed attempt time is `finished - started` from the record. It is observed, not a new measure, and
   is summed per disposition.
4. **Report verb `deskperf reruns --root <repo> [--brief <stream>/<NN>] [--days N] [--json]`.**
   It reads the register under `--root` plus the perf day files `deskperf report` already reads.
   It writes nothing and opens no network connection; amend the package header's read-scope sentence to
   say so. With `--brief`, it prints one line of the form
   `<stream>/<NN>: <runs> verification runs, <U> uncredited, <D> discarded, <R> unrecorded`. Row 2
   pins it exactly as `graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded`. A cost block follows. Without it, it prints one row per brief with
   at least one record, sorted by `runs` descending. Cost is NOT a new measure. For each attempt it
   selects workflow perf rows with `role=verify`, a row `ts` inside the attempt window, and a
   `work` ref with `kind` `brief`, `repo` equal to the root's repo, and `brief` equal to
   `PerfBriefRef` of the frontmatter `brief:` value of the brief file that `--brief <stream>/<NN>`
   names. For example, `desk-supervision/37` becomes `assay:assay:desk-supervision:37` and
   `graph-execution/02` becomes `assay:assay:graph-execution:2`. It compares only those three
   fields: `briefUuid` and `primary` are ignored, so the comparison is not `perfrec`'s
   whole-struct `workEqual`. A `<stream>/<NN>` work ref never matches, because perfrec never
   stores one. If the brief file is missing or `PerfBriefRef` refuses, the verb exits 6 and names
   the path or value. It sums them
   with the existing `perfrec.V3Group` and request-rollup reducers. It then prints the total, the
   uncredited-plus-discarded part, and that part's share. Honest states:
   - an attempt window older than the perf retention or outside `--days` → `could-not-check (pruned)`;
   - perf logging off or no workflow rows in range → `could-not-check (no perf rows)`, the same
     reason text `load` uses;
   - more than one distinct `agentId` in one window → that attempt `ambiguous`, excluded from
     both sums and listed;
   - matching rows outside every attempt window → `unassigned`, summed separately and never
     folded into either side;
   - verifier token cost → `could-not-check (desk-supervision/34 records worker dispatches only)`;
   - dispatches that landed no record → one fixed line saying they are not counted.

   `--json` emits the same fields with three-state values (a number, or `{"state": "...",
   "reason": "..."}`). The verb exits 6 when the register has no record for `--brief`.
5. **Verifier kit and skill.** `verifier-prompt.md` gains a short fixed-shape "Attempts" section:
   every run of the Verify table, full or partial, in this dispatch is one attempt. Record its
   window, rows run, and disposition and reason from the closed sets in step 1. Discarding a run never
   deletes its entry. `plugins/assay/skills/verify-desk/SKILL.md`: the dispatch step passes
   `--perf-work '{"kind":"brief","repo":"<owner/repo>","brief":"<qualified-brief-id>"}'` (the
   existing flag, `tools/desk/internal/cli/performance.go`). The skill states that
   `<qualified-brief-id>` is the brief file's frontmatter `brief:` value with the number's
   leading zeros dropped (`PerfBriefRef`, step 1). Its examples are `assay:assay:desk-supervision:37`,
   and `assay:assay:graph-execution:2` for graph-execution/02. It also states that a `<stream>/<NN>`
   value is silently dropped by perfrec. The outcome-record step includes `attempts`. Neither changes what is credited.
6. **Docs.** Add `attempts` to the Fields table in `verify-wake-v1.md` with the step 1 rules
   and to the register description in `docs/streams/verify-outcomes/README.md`. Add a `deskperf
   reruns` line to `tools/desk/README.md`. Add a changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/deskperf/... ./cmd/verifyloop/... > "${TMPDIR:-/tmp}/b37-pkgs.out" 2>&1 && grep -F -e 'cmd/deskperf' "${TMPDIR:-/tmp}/b37-pkgs.out"` | exit 0; the printed `cmd/deskperf` line begins `ok` | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_GraphExecution02Fixture$' -v > "${TMPDIR:-/tmp}/b37-fixture.out" 2>&1 && grep -F -e '--- PASS: TestReruns_GraphExecution02Fixture' "${TMPDIR:-/tmp}/b37-fixture.out"` | exit 0. The fixture register holds one record for `graph-execution/02` (outcome `blocked`) with attempts `[uncredited local-build, discarded env-fault, credited]`, plus one earlier legacy record with no `attempts`. The fixture perf day file holds rows inside each window, each carrying the work ref `{"kind":"brief","repo":"medici-finance/assay","brief":"assay:assay:graph-execution:2"}`. Output line equals `graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded`. The cost block's uncredited-plus-discarded `sumOwnProcessWallMs` equals the sum of the fixture rows inside attempts 1-2 exactly, and its share is printed. Verifier tokens print `could-not-check` | check:ci +flow +dereference |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckAttempts$' -v > "${TMPDIR:-/tmp}/b37-check.out" 2>&1 && grep -F -e '--- PASS: TestCheckAttempts' "${TMPDIR:-/tmp}/b37-check.out"` | exit 0. Each subtest is refused naming its rule: a gap in numbering; `credited` not last; two `credited`; `verified` outcome ending in `discarded`; `env-fault` without `cause_ref`; `credited` with a `reason`; overlapping windows; an attempt outside the record's `started`/`finished`; `rows_run` above `rows_total`; 17 entries; an unknown key (`tokens`); a case-variant key (`Disposition`). A record with no `attempts` passes. Mutation: with the credited-last rule removed, the `credited` not last subtest fails | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_InvalidStoredRecordNotCounted$' -v > "${TMPDIR:-/tmp}/b37-reader.out" 2>&1 && grep -F -e '--- PASS: TestReruns_InvalidStoredRecordNotCounted' "${TMPDIR:-/tmp}/b37-reader.out"` | exit 0. A hand-placed record (writer bypassed) whose attempts put `credited` first is listed `invalid` with its path and excluded from `runs`. Mutation: with the reader's re-validation removed (`deskperf reruns` no longer calls `CheckAttempts` on stored records), the record is counted and the test fails | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_NeverEstimates$' -v > "${TMPDIR:-/tmp}/b37-honest.out" 2>&1 && grep -F -e '--- PASS: TestReruns_NeverEstimates' "${TMPDIR:-/tmp}/b37-honest.out"` | exit 0. Subtests: an empty perf dir prints cost `could-not-check (no perf rows)` while the counts are still printed; a window older than `--days` prints `could-not-check (pruned)`; two distinct `agentId`s in one window mark that attempt `ambiguous`, which is excluded from both sums; a row for the brief outside every window is `unassigned`. In no subtest does the JSON carry a numeric `0` for a could-not-check measure. Mutations: replacing the missing-perf state with `0` fails the empty-dir subtest, and splitting an ambiguous window's rows evenly across its agents fails the ambiguous subtest | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -v > "${TMPDIR:-/tmp}/b37-writer.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b37-writer.out" && grep -F -e 'attempts' "${TMPDIR:-/tmp}/b37-writer.out"` | exit 0. The existing table gains named subtests whose names contain `attempts`: a valid attempts list lands byte-for-byte, and an invalid one exits 5 with nothing written | check:ci |
| 7 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b37-statusgen.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b37-statusgen.out"` | exit 0. The pinned additive-key record now includes `attempts`, and the board output is byte-identical to the same record without it. The command runs from `statusgen/`, its own module; the repo root has no `go.mod`. The form matches desk-supervision/27 row 6 | check:ci +neighbour |
| 8 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -v > "${TMPDIR:-/tmp}/b37-verifyloop.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b37-verifyloop.out"` | exit 0. A `blocked` record carrying `attempts` yields the same plan bucket and wake decision as the same record without it | check:ci +neighbour |
| 9 | `grep -F -e 'local-build' docs/streams/desk-supervision/verify-wake-v1.md && grep -F -e 'local-build' tools/desk/cmd/deskdispatch/references/verifier-prompt.md && grep -F -e '"brief":"<qualified-brief-id>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'assay:assay:graph-execution:2' plugins/assay/skills/verify-desk/SKILL.md && ! grep -F -e '"brief":"<stream>/<NN>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'deskperf reruns' tools/desk/README.md` | exit 0. Each positive grep prints its line, and the negative grep prints nothing. None of the positive strings is in its file at 866a169ae. The skill's `--perf-work` example carries the qualified placeholder and the zero-stripped worked example, never the short form | check:ci |
| 10 | `test -f tools/desk/internal/deskkit/verifyattempts.go && ! grep -n -i -e 'json:"[a-z_]*token' -e 'json:"[a-z_]*cost' -e 'json:"[a-z_]*wall' -e 'json:"[a-z_]*model' tools/desk/internal/deskkit/verifyattempts.go` | exit 0; no output. No attempt field names a token, cost, wall-time or model measure (driver correction) | check:ci |
| 11 | `statusgen --consumers --brief desk-supervision/37 --base $(git merge-base origin/main HEAD)` | exit 0; every `follow-up desk-supervision/37` entry reads CORROBORATED (the implementation edits each named path); the four out-of-scope entries stay UNCHECKED and reviewer-owned; completeness is not proved | check:ci |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfBriefRef$' -v > "${TMPDIR:-/tmp}/b37-ref.out" 2>&1 && grep -F -e '--- PASS: TestPerfBriefRef' "${TMPDIR:-/tmp}/b37-ref.out" && go test ./cmd/deskperf/ -run '^TestReruns_JoinsQualifiedWorkRefOnly$' -v > "${TMPDIR:-/tmp}/b37-join.out" 2>&1 && grep -F -e '--- PASS: TestReruns_JoinsQualifiedWorkRefOnly' "${TMPDIR:-/tmp}/b37-join.out"` | exit 0. `TestPerfBriefRef`: `assay:assay:desk-supervision:37` maps to itself, `assay:assay:graph-execution:02` maps to `assay:assay:graph-execution:2`, and `desk-supervision/37` and `a:b:c:0` are refused. Each mapped value, set as a `perfrec` work ref, resolves with work coverage `measured`. `TestReruns_JoinsQualifiedWorkRefOnly`: perf rows are written through the real `perfrec` context resolver with `--perf-work` as the skill passes it. Rows from the qualified ref are joined to the attempt. Rows written with `"brief":"graph-execution/02"` carry no work, and the attempt reports `could-not-check (no perf rows)`, never a cost. Mutation: with the join comparing the `<stream>/<NN>` id instead of `PerfBriefRef`'s, the first case joins nothing and the test fails | check:ci +mutation |

### Pre-mortem
| Failure | Caught by |
|---|---|
| The report prints `0` cost when perf logging was off, so waste reads as free | row 5 |
| Ambiguous perf rows are split evenly across attempts, which turns an estimate into a figure | row 5 (ambiguous subtest) |
| A verified record lands with its credited run not last, so the waste count is wrong | rows 3, 4 |
| The new key breaks the board or the wake planner on old or new records | rows 7, 8 |
| Someone adds `tokens` or `wall_ms` to an attempt "for convenience" and duplicates perf | rows 3 (unknown key), 10 |
| The verify-desk never passes `--perf-work`, so every cost block is could-not-check in production | row 9 (skill line); the report's own reason text names the missing work ref |
| `--perf-work` carries `<stream>/<NN>` or a zero-padded number, so perfrec silently drops the work ref and no attempt ever joins a cost row | rows 9 (skill pins the qualified placeholder, never the short form), 12 (mapping and join through the real resolver) |
| Verifiers stop reporting discarded runs because it looks bad | no row — review-only (a reporting norm stated in the verifier kit; the count is a measure of environment waste, never of the verifier) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — 2026-11-02 at cd08e97833cb (claude-opus-5-5)

First verification of this brief, on merged main at cd08e97833cb (detached home, admitted by deskdispatch --check-verifier, attestation issue medici-finance/assay#5344). The implementation is PR medici-finance/assay#5329, merge 84a4a4400803 (first parent 45b5095f0c56): commits 8c37812ab (the feature) and d4d93c20f (fixture stream ids neutralised). No implementation file changed between that merge and cd08e97833cb. The runner took no part in the implementation.

- **How the rows ran.** All 12 rows are check:ci. They ran inside one closed admission bracket: the host opened it at 21:55:26Z, then statusgen v1.0.70 ran every row with --sandbox=container-netns in a --network none container of the pinned verify-runner v1.0.70 image for linux/arm64 (digest on the Container-Image line below). The home, the repository's git directory and the Go module cache were mounted read-only, the environment carried KUBECONFIG=/dev/null and GOPROXY=off and no forge credential, and the witness directory, a Go build cache and a scratch TMPDIR were the only writable mounts. The container finished at 22:03:11Z and the host closed the bracket at 22:09:17Z. The bracket digest check reads verdict=match.
- **Uncredited steps, disclosed.** After the credited container run finished, and while the bracket was still open (22:04Z to 22:17Z), diagnostic runs and the mutation runs below ran in separate --network none containers of the same image. They used the home read-only or a scratch export of cd08e97833cb outside the home, and never touched the home or the witness. None of them is acceptance credit.
- **Row 1** exited 1 (witness hash sha256:e3b0c44298fc, the empty grep output). Of its four packages, cmd/verifyloop and internal/deskkit/untrustcorpus were ok; internal/deskkit, cmd/deskevidence and cmd/deskperf failed on four tests, none in a file the implementation changed and none exercising it. The causes are all container-environment: TestOverrideIdentityResolvesRosterLogin panics with a nil pointer because user.Current() fails when the container uid has no passwd entry; TestBeaconStoreConcurrentProcessesPreserveFields ("concurrent.json: no such file or directory") and TestLandedApprovalContext ("lost repository authority: origin=\"\" err=gitcore: remote-url origin: no URL configured") fail only with TMPDIR on the bind-mounted scratch directory and pass on a container tmpfs; TestV4ReadReportFlow rebuilds deskperf through Go's default HOME cache layout, so with GOPROXY=off it cannot see the module cache ("module lookup disabled by GOPROXY=off"). A diagnostic rerun of row 1's exact command, with only the environment changed (a passwd entry for the container uid, TMPDIR on tmpfs, the module cache at HOME's default path), exited 0: every package ok, and the printed line is `ok  	github.com/medici-finance/assay/tools/desk/cmd/deskperf	27.508s`. That rerun is diagnostic only. Row 1 is could-not-run in this container, not a failure of this deliverable.
- **Row 11** exited 2 as written. Run post-merge, `$(git merge-base origin/main HEAD)` is HEAD itself, so the tool refused: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:37 is not in the diff against cd08e97833cbf31395e302adf8922fe6b0bde137, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." The row is a pre-merge form (the medici-finance/assay#4560 class). A supplemental run with the delivering merge's first parent as base (`--base 45b5095f0c5674c60027f131c3ed1f31f36b9d95`) exited 0 with `summary: 9 corroborated, 0 disproved, 4 unchecked, 0 brief(s) claiming nothing`, and the four UNCHECKED entries are the four out-of-scope consumers, which matches the Expect cell. No credit. Row 11 is could-not-run as written: a check-definition gap.
- **Rows 2 to 10 and 12** pass. Row 3's test covers all 12 refusal subtests the Expect cell names, and a record with no attempts passes. Expect-cell drift, not a defect: rows 2 and 12 still name `graph-execution/02` in their Expect cells, but commit d4d93c20f moved the fixture to `example-stream/02`. Row 2's test now asserts `example-stream/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded` with work ref `assay:assay:example-stream:2`, which is the same shape and the same figures. Row 12's test still maps `assay:assay:graph-execution:02` to `assay:assay:graph-execution:2` and `assay:assay:desk-supervision:37` to itself, and it refuses `example-stream/37` and `a:b:c:0` (the short form is `example-stream/37` there, not `desk-supervision/37`).
- **Mutations (rows 3, 4 and 5).** The brief's own manifest tools/desk/cmd/deskperf/reruns-mutations.json (one control and 8 mutants) was applied one entry at a time to the scratch export, by exact single-occurrence replacement. Each entry ran through the manifest's test command (`go test -count=1 -timeout 300s -run '^(TestCheckAttempts|TestPerfBriefRef|TestReruns_.*)$' ./internal/deskkit ./cmd/deskperf`) and was then restored; every restore's sha256 matched the original. The unmutated baseline exited 0 before the runs and again after the last restore. The control and all 8 mutants were killed (exit 1). Row 3: the credited-last mutant fails `TestCheckAttempts/credited_not_last`. Row 4: the reader-skips-revalidation mutant fails `TestReruns_InvalidStoredRecordNotCounted`. Row 5: the missing-perf-as-0 mutant fails `TestReruns_NeverEstimates/empty_perf_dir`; the ambiguous-window mutant fails `TestReruns_NeverEstimates/two_agentIds_in_one_window`; the pruned-window mutant fails `window_older_than_--days`. The other mutants (short join key, out-of-window row folded in, CPU printed as 0) are killed by the row 2, 5 and 12 tests. The manifest's ambiguous mutant counts the window into both sums, which is not the same as the Expect cell's "split evenly", but it trips the same subtest.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/deskperf/... ./cmd/verifyloop/... > "${TMPDIR:-/tmp}/b37-pkgs.out" 2>&1 && grep -F -e 'cmd/deskperf' "${TMPDIR:-/tmp}/b37-pkgs.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_GraphExecution02Fixture$' -v > "${TMPDIR:-/tmp}/b37-fixture.out" 2>&1 && grep -F -e '--- PASS: TestReruns_GraphExecution02Fixture' "${TMPDIR:-/tmp}/b37-fixture.out"` | pass exit=0 sandbox=container-netns | sha256:259faf07a032 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckAttempts$' -v > "${TMPDIR:-/tmp}/b37-check.out" 2>&1 && grep -F -e '--- PASS: TestCheckAttempts' "${TMPDIR:-/tmp}/b37-check.out"` | pass exit=0 sandbox=container-netns | sha256:11a0ab17c1b5 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_InvalidStoredRecordNotCounted$' -v > "${TMPDIR:-/tmp}/b37-reader.out" 2>&1 && grep -F -e '--- PASS: TestReruns_InvalidStoredRecordNotCounted' "${TMPDIR:-/tmp}/b37-reader.out"` | pass exit=0 sandbox=container-netns | sha256:60e39bbc25c6 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_NeverEstimates$' -v > "${TMPDIR:-/tmp}/b37-honest.out" 2>&1 && grep -F -e '--- PASS: TestReruns_NeverEstimates' "${TMPDIR:-/tmp}/b37-honest.out"` | pass exit=0 sandbox=container-netns | sha256:236df6e0396f | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -v > "${TMPDIR:-/tmp}/b37-writer.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b37-writer.out" && grep -F -e 'attempts' "${TMPDIR:-/tmp}/b37-writer.out"` | pass exit=0 sandbox=container-netns | sha256:d9e3b0e437c8 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b37-statusgen.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b37-statusgen.out"` | pass exit=0 sandbox=container-netns | sha256:2b98a98b1bff | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -v > "${TMPDIR:-/tmp}/b37-verifyloop.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b37-verifyloop.out"` | pass exit=0 sandbox=container-netns | sha256:1ea9dea132c0 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -F -e 'local-build' docs/streams/desk-supervision/verify-wake-v1.md && grep -F -e 'local-build' tools/desk/cmd/deskdispatch/references/verifier-prompt.md && grep -F -e '"brief":"<qualified-brief-id>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'assay:assay:graph-execution:2' plugins/assay/skills/verify-desk/SKILL.md && ! grep -F -e '"brief":"<stream>/<NN>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'deskperf reruns' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:e903f37caa0a | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -f tools/desk/internal/deskkit/verifyattempts.go && ! grep -n -i -e 'json:"[a-z_]*token' -e 'json:"[a-z_]*cost' -e 'json:"[a-z_]*wall' -e 'json:"[a-z_]*model' tools/desk/internal/deskkit/verifyattempts.go` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --consumers --brief desk-supervision/37 --base $(git merge-base origin/main HEAD)` | fail exit=2 sandbox=container-netns | sha256:3261d34c316d | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfBriefRef$' -v > "${TMPDIR:-/tmp}/b37-ref.out" 2>&1 && grep -F -e '--- PASS: TestPerfBriefRef' "${TMPDIR:-/tmp}/b37-ref.out" && go test ./cmd/deskperf/ -run '^TestReruns_JoinsQualifiedWorkRefOnly$' -v > "${TMPDIR:-/tmp}/b37-join.out" 2>&1 && grep -F -e '--- PASS: TestReruns_JoinsQualifiedWorkRefOnly' "${TMPDIR:-/tmp}/b37-join.out"` | pass exit=0 sandbox=container-netns | sha256:6f38ea70967c | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:646b4e70934f4e4fd4018087336bea3f81a6aff3dec573f40fd5bf07adc7887e
Container-Image: sha256:c556f436ff2789b07b2097d50a9d7b5f1266b09f3c40f51b4f02e52bbac7f217
Verification-Attestation: medici-finance/assay#5344 run=efbe457958ec888f276b4e0f61ab7b7885cc66838abca238c47e72cef2db58f2 source=cd08e97833cbf31395e302adf8922fe6b0bde137 brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=efbe457958ec888f276b4e0f61ab7b7885cc66838abca238c47e72cef2db58f2 source=cd08e97833cbf31395e302adf8922fe6b0bde137 tree=sha256:646b4e70934f4e4fd4018087336bea3f81a6aff3dec573f40fd5bf07adc7887e witness=sha256:509282da1de815e8d73ec93de03a3a7da1ec8cd151d09985d6a470327a11fdd4 opened=2026-11-02T21:55:26Z closed=2026-11-02T22:09:17Z before=receipt after=check-verifier

Supplemental runs (diagnostic, no acceptance credit):

| Probe | Command | Observed | Date | Runner |
|-------|---------|----------|------|--------|
| S1 (row 1, environment only) | row 1's exact command; container with a passwd entry for its uid, TMPDIR on tmpfs, module cache at HOME's default path | exit 0; internal/deskkit, untrustcorpus, cmd/deskevidence, cmd/deskperf and cmd/verifyloop all ok; printed `ok  	github.com/medici-finance/assay/tools/desk/cmd/deskperf	27.508s` | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| S2 (row 11, delivery base) | `statusgen --consumers --brief desk-supervision/37 --base 45b5095f0c5674c60027f131c3ed1f31f36b9d95` | exit 0; `summary: 9 corroborated, 0 disproved, 4 unchecked, 0 brief(s) claiming nothing`; the 4 UNCHECKED are the out-of-scope consumers | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| S3 (rows 3 to 5, mutations) | reruns-mutations.json, control and 8 mutants, each through the manifest test command | baseline exit 0; control and all 8 mutants killed, exit 1; after restore exit 0 | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |

Risk-bearing literal values at cd08e97833cb. The frontmatter risk answers are all no (gate: model), but the diff touches deskkit, a risk-classed path, so the procedure runs. The enumeration covered the non-test Go diff of 45b5095f0c56..84a4a4400803 (verifyattempts.go, reruns.go, deskperf main.go and report.go, deskevidence outcomerecord.go, verifyoutcomejoin.go) plus the brief's Deliverables. Every value is reversible by an edit and a release.

RISK-VALUE: DERIVED — MaxAttempts = 16 @ tools/desk/internal/deskkit/verifyattempts.go:47 — the brief's Task 1 bounds the list at 1 to 16 entries; row 3's 17-entry subtest pins it.

RISK-VALUE: DERIVED — rows_run bound `rows < 1 || rows > rowsTotal` @ tools/desk/internal/deskkit/verifyattempts.go:235 — the brief states 1 <= rows_run <= rows_total; row 3's rows_run-above-rows_total subtest pins it.

RISK-VALUE: DERIVED — `--days` default deskkit.PerfMaxDays @ tools/desk/cmd/deskperf/reruns.go:140 — PerfMaxDays = perfRetentionDays (tools/desk/internal/deskkit/perfread.go:32), the seven-UTC-day perf retention; no eighth day exists to read. A window older than it prints could-not-check (pruned), never 0 (row 5, and the pruned-window mutant).

RISK-VALUE: DERIVED — closed reason and disposition sets @ tools/desk/internal/deskkit/verifyattempts.go:31 — credited, uncredited, discarded (unrecorded only in the rollup) and the reasons local-build, not-credited, credit-unchecked, env-fault, superseded are the sets the brief's Task 1 names.

RISK-VALUE: N/A — no timeout, host, endpoint, file mode, permission or credential-handling literal is introduced. Refusals reuse the existing exit codes 5 and 6.

**VERIFY: BLOCKED** — 10 of 12 rows pass at cd08e97833cb in one closed v1.0.70 linux/arm64 container bracket (verdict=match), and the manifest's control and all 8 mutants are killed. No obligation of this brief was observed failing. Row 1 is could-not-run in this container: four tests outside the implementation fail for environment reasons (no passwd entry for the container uid, a bind-mounted TMPDIR, a HOME-default module cache under GOPROXY=off), and the same command with only those changed exits 0, which is diagnostic and not credit. Row 11 is could-not-run as written: it is a pre-merge consumers form that cannot corroborate post-merge (medici-finance/assay#4560 class), and with the delivering merge's base it reads 9 corroborated, 0 disproved, 4 unchecked, which is not credit.

### Non-implementer verifier run — 2026-11-06 at 8f5bb307de31 (claude-opus-5-5)

Container witness for rows 1 to 10 and 12, in one closed admission bracket (verify-runner v1.0.75, --network none, sandbox=container-netns):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/deskperf/... ./cmd/verifyloop/... > "${TMPDIR:-/tmp}/b37-pkgs.out" 2>&1 && grep -F -e 'cmd/deskperf' "${TMPDIR:-/tmp}/b37-pkgs.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_GraphExecution02Fixture$' -v > "${TMPDIR:-/tmp}/b37-fixture.out" 2>&1 && grep -F -e '--- PASS: TestReruns_GraphExecution02Fixture' "${TMPDIR:-/tmp}/b37-fixture.out"` | pass exit=0 sandbox=container-netns | sha256:67d6845958c0 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckAttempts$' -v > "${TMPDIR:-/tmp}/b37-check.out" 2>&1 && grep -F -e '--- PASS: TestCheckAttempts' "${TMPDIR:-/tmp}/b37-check.out"` | pass exit=0 sandbox=container-netns | sha256:11a0ab17c1b5 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_InvalidStoredRecordNotCounted$' -v > "${TMPDIR:-/tmp}/b37-reader.out" 2>&1 && grep -F -e '--- PASS: TestReruns_InvalidStoredRecordNotCounted' "${TMPDIR:-/tmp}/b37-reader.out"` | pass exit=0 sandbox=container-netns | sha256:c53732ec3913 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_NeverEstimates$' -v > "${TMPDIR:-/tmp}/b37-honest.out" 2>&1 && grep -F -e '--- PASS: TestReruns_NeverEstimates' "${TMPDIR:-/tmp}/b37-honest.out"` | pass exit=0 sandbox=container-netns | sha256:6738b008e891 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -v > "${TMPDIR:-/tmp}/b37-writer.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b37-writer.out" && grep -F -e 'attempts' "${TMPDIR:-/tmp}/b37-writer.out"` | pass exit=0 sandbox=container-netns | sha256:2200d8ab79d5 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b37-statusgen.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b37-statusgen.out"` | pass exit=0 sandbox=container-netns | sha256:f86c59366680 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -v > "${TMPDIR:-/tmp}/b37-verifyloop.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b37-verifyloop.out"` | pass exit=0 sandbox=container-netns | sha256:497a86587e5c | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -F -e 'local-build' docs/streams/desk-supervision/verify-wake-v1.md && grep -F -e 'local-build' tools/desk/cmd/deskdispatch/references/verifier-prompt.md && grep -F -e '"brief":"<qualified-brief-id>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'assay:assay:graph-execution:2' plugins/assay/skills/verify-desk/SKILL.md && ! grep -F -e '"brief":"<stream>/<NN>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'deskperf reruns' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:e903f37caa0a | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -f tools/desk/internal/deskkit/verifyattempts.go && ! grep -n -i -e 'json:"[a-z_]*token' -e 'json:"[a-z_]*cost' -e 'json:"[a-z_]*wall' -e 'json:"[a-z_]*model' tools/desk/internal/deskkit/verifyattempts.go` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfBriefRef$' -v > "${TMPDIR:-/tmp}/b37-ref.out" 2>&1 && grep -F -e '--- PASS: TestPerfBriefRef' "${TMPDIR:-/tmp}/b37-ref.out" && go test ./cmd/deskperf/ -run '^TestReruns_JoinsQualifiedWorkRefOnly$' -v > "${TMPDIR:-/tmp}/b37-join.out" 2>&1 && grep -F -e '--- PASS: TestReruns_JoinsQualifiedWorkRefOnly' "${TMPDIR:-/tmp}/b37-join.out"` | pass exit=0 sandbox=container-netns | sha256:07483c7fa504 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:cee7fc06673423aacdd6178185b8bf59b910a9e0528dc2d47cc6aa437ab30ed0
Container-Image: sha256:732786a33ebfa6dc1d80d396781df1f31c5331f67a8c0bd97518785f5a72b173
Verification-Attestation: medici-finance/assay#5602 run=ebd5220c8b8731779fbd0df5fabd4efbb2a0589a081ce1c335668c595cd72d7f source=8f5bb307de31c64b7c96e38d631926fec8c69211 brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ebd5220c8b8731779fbd0df5fabd4efbb2a0589a081ce1c335668c595cd72d7f source=8f5bb307de31c64b7c96e38d631926fec8c69211 tree=sha256:cee7fc06673423aacdd6178185b8bf59b910a9e0528dc2d47cc6aa437ab30ed0 witness=sha256:eb6f20b24442559d64802df15dd2a79e7b37c17d08cf2d6f327bd21fb8788013 opened=2026-11-06T16:45:43Z closed=2026-11-06T16:51:30Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `statusgen --consumers --brief desk-supervision/37 --base $(git merge-base origin/main HEAD)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5602 run=ebd5220c8b8731779fbd0df5fabd4efbb2a0589a081ce1c335668c595cd72d7f source=8f5bb307de31c64b7c96e38d631926fec8c69211 brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong

The host ran row 11 with `--rows 11` after the bracket closed (table above). Fail-first on the host printed `--fail-first: no risk-bearing row in this table; every row is recorded not-selected at base` (the frontmatter risk answers are all no, and no row is risk-bearing). The implementation (PR medici-finance/assay#5329, merge 84a4a4400803) is unchanged at 8f5bb307de31, apart from two refusal-message strings in verifyoutcomejoin.go (4617a298b).

- **Rows 2 to 10 and 12** pass in the bracket, which closed at 16:51:30Z with verdict=match. The Expect-cell drift on rows 2 and 12 (`graph-execution/02` vs fixture `example-stream/02`) is unchanged from the 2026-11-02 run.
- **Row 1** exited 1 in the container, for environment reasons outside this deliverable. Diagnostic run D1 (no credit, after the close) showed that 5 failing internal/deskkit tests stop at `fatal: gitfile does not point to a valid repository: /work/.git` or at an unresolvable origin forge. The home is a linked worktree whose git directory is not in the container (medici-finance/assay#4322, medici-finance/assay#4327). The sixth failure, TestV4ReadReportFlow in cmd/deskperf, rebuilds through HOME's default module cache and fails with `module lookup disabled by GOPROXY=off` (medici-finance/assay#4683). D2 changed only those two things: the git common dir was mounted read-only at its host path, and the module cache was linked at HOME's default path. In D2, cmd/deskperf, cmd/deskevidence, cmd/verifyloop and untrustcorpus are ok. internal/deskkit fails one test: TestReviewCallerRollback, the pr-monitor.test.sh case classified-unwritable-hold, "a surviving prior artifact is explicitly rejected, never left silently". That fixture makes a directory unwritable with `chmod 555`, and the container runs as root, which ignores the mode. That test is outside this brief's diff. Row 1 is not run in this container for these environment reasons. It is not a failure of this deliverable.
- **Row 11** is could-not-run on the darwin host (no `unshare --net`), and it is a pre-merge form (medici-finance/assay#4560 class). In supplemental S1, run as written from the home's git, the merge-base is HEAD. Because the uncommitted Evidence edit is present, it exits 0 vacuously with `0 corroborated, 0 disproved, 13 unchecked`, which is the hazard medici-finance/assay#4560 records. In S2, with `--base 45b5095f0c5674c60027f131c3ed1f31f36b9d95`, it exits 0 with `summary: 9 corroborated, 0 disproved, 4 unchecked, 0 brief(s) claiming nothing`; the 4 unchecked are the out-of-scope consumers. Neither S1 nor S2 earns credit.

| Probe | Command | Observed | Date | Runner |
|-------|---------|----------|------|--------|
| D1 (row 1, as bracketed) | row 1's go test, same container mounts | exit 1; deskkit 6 tests fail, deskperf TestV4ReadReportFlow fails; deskevidence, verifyloop, untrustcorpus ok | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) |
| D2 (row 1, environment only) | row 1's exact command; git common dir mounted read-only, module cache at HOME's default path | exit 1; only TestReviewCallerRollback (root ignores chmod 555) fails; the other four packages ok | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) |
| S1 (row 11, as written) | row 11's command from the home, uncommitted Evidence present | exit 0; `0 corroborated, 0 disproved, 13 unchecked` (vacuous) | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) |
| S2 (row 11, delivery base) | `statusgen --consumers --brief desk-supervision/37 --base 45b5095f0c56…` | exit 0; `9 corroborated, 0 disproved, 4 unchecked` | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — MaxAttempts = 16 @ tools/desk/internal/deskkit/verifyattempts.go:47 — the brief's Task 1 bound of 1 to 16 entries; row 3 pins it.

RISK-VALUE: DERIVED — rows_run bound `rows < 1 || rows > rowsTotal` @ tools/desk/internal/deskkit/verifyattempts.go:235 — 1 <= rows_run <= rows_total per the brief; row 3 pins it.

RISK-VALUE: DERIVED — `--days` default deskkit.PerfMaxDays @ tools/desk/cmd/deskperf/reruns.go:140 — resolves to perfrec.RetentionDays = 7 (perfrec/perfrec.go:50), the perf retention window; row 5 pins the pruned-window refusal.

RISK-VALUE: DERIVED — closed disposition and reason sets @ tools/desk/internal/deskkit/verifyattempts.go:31 — exactly the sets the brief's Task 1 names.

RISK-VALUE: N/A — no timeout, host, endpoint, file mode or permission literal is introduced; each value above is reversible by an edit and a release.

**VERIFY: BLOCKED** — 10 of 12 rows pass at 8f5bb307de31 in one closed v1.0.75 container bracket (verdict=match). No obligation of this brief was observed failing. Row 1 is not run, for container-environment reasons: a linked-worktree gitdir, the module cache, and a root-only chmod fixture outside this brief. Row 11 is could-not-run as written, a check-definition gap in the medici-finance/assay#4560 class.
### Non-implementer verifier run — 2026-11-10 at 612750a12406 (claude-opus-5-5)

Container witness for rows 1 to 10 and 12, in one closed admission bracket (verify-runner v1.0.81 pinned by release digest, --network none, sandbox=container-netns, a fresh --module-cache):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/deskperf/... ./cmd/verifyloop/... > "${TMPDIR:-/tmp}/b37-pkgs.out" 2>&1 && grep -F -e 'cmd/deskperf' "${TMPDIR:-/tmp}/b37-pkgs.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_GraphExecution02Fixture$' -v > "${TMPDIR:-/tmp}/b37-fixture.out" 2>&1 && grep -F -e '--- PASS: TestReruns_GraphExecution02Fixture' "${TMPDIR:-/tmp}/b37-fixture.out"` | pass exit=0 sandbox=container-netns | sha256:67d6845958c0 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckAttempts$' -v > "${TMPDIR:-/tmp}/b37-check.out" 2>&1 && grep -F -e '--- PASS: TestCheckAttempts' "${TMPDIR:-/tmp}/b37-check.out"` | pass exit=0 sandbox=container-netns | sha256:11a0ab17c1b5 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_InvalidStoredRecordNotCounted$' -v > "${TMPDIR:-/tmp}/b37-reader.out" 2>&1 && grep -F -e '--- PASS: TestReruns_InvalidStoredRecordNotCounted' "${TMPDIR:-/tmp}/b37-reader.out"` | pass exit=0 sandbox=container-netns | sha256:c53732ec3913 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_NeverEstimates$' -v > "${TMPDIR:-/tmp}/b37-honest.out" 2>&1 && grep -F -e '--- PASS: TestReruns_NeverEstimates' "${TMPDIR:-/tmp}/b37-honest.out"` | pass exit=0 sandbox=container-netns | sha256:b0b8f0190064 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -v > "${TMPDIR:-/tmp}/b37-writer.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b37-writer.out" && grep -F -e 'attempts' "${TMPDIR:-/tmp}/b37-writer.out"` | pass exit=0 sandbox=container-netns | sha256:2200d8ab79d5 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b37-statusgen.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b37-statusgen.out"` | pass exit=0 sandbox=container-netns | sha256:f86c59366680 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -v > "${TMPDIR:-/tmp}/b37-verifyloop.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b37-verifyloop.out"` | pass exit=0 sandbox=container-netns | sha256:497a86587e5c | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -F -e 'local-build' docs/streams/desk-supervision/verify-wake-v1.md && grep -F -e 'local-build' tools/desk/cmd/deskdispatch/references/verifier-prompt.md && grep -F -e '"brief":"<qualified-brief-id>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'assay:assay:graph-execution:2' plugins/assay/skills/verify-desk/SKILL.md && ! grep -F -e '"brief":"<stream>/<NN>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'deskperf reruns' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:e903f37caa0a | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -f tools/desk/internal/deskkit/verifyattempts.go && ! grep -n -i -e 'json:"[a-z_]*token' -e 'json:"[a-z_]*cost' -e 'json:"[a-z_]*wall' -e 'json:"[a-z_]*model' tools/desk/internal/deskkit/verifyattempts.go` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfBriefRef$' -v > "${TMPDIR:-/tmp}/b37-ref.out" 2>&1 && grep -F -e '--- PASS: TestPerfBriefRef' "${TMPDIR:-/tmp}/b37-ref.out" && go test ./cmd/deskperf/ -run '^TestReruns_JoinsQualifiedWorkRefOnly$' -v > "${TMPDIR:-/tmp}/b37-join.out" 2>&1 && grep -F -e '--- PASS: TestReruns_JoinsQualifiedWorkRefOnly' "${TMPDIR:-/tmp}/b37-join.out"` | pass exit=0 sandbox=container-netns | sha256:07483c7fa504 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#5990 run=0460f1b73fdaaa431c57d8363026d676fd4abd25a916af00c5f97835a4131ce1 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=0460f1b73fdaaa431c57d8363026d676fd4abd25a916af00c5f97835a4131ce1 source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:945740437067097d2e1f9904b1c8b69e624b7f2290eaa8a30fb0aa289b298f93 opened=2026-11-10T07:53:39Z closed=2026-11-10T07:57:31Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `statusgen --consumers --brief desk-supervision/37 --base $(git merge-base origin/main HEAD)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5990 run=0460f1b73fdaaa431c57d8363026d676fd4abd25a916af00c5f97835a4131ce1 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong

The host ran row 11 with `--rows 11` after the bracket closed (table above). The implementation is PR medici-finance/assay#5329, merge 84a4a4400803. Its delivered code is unchanged at 612750a12406 except for the outcome-record dry-run route in outcomerecord.go (4fd8de15a and 840e1c067), which come from later briefs and add no risk-bearing literal.

- **Rows 2 to 10 and 12** pass in the bracket. The bracket opened at 07:53:39Z and closed at 07:57:31Z, and `--bracket-digest` reports verdict=match. `--bracket-digest --credit` prints credit=could-not-check, because the witness file is not in a git work tree. That check runs again once the Evidence lands.
- **An earlier bracket in this dispatch was discarded and re-run.** It opened at 07:43:14Z and closed at 07:48:51Z, with verdict=match. Rows 2 to 10 and 12 passed in it too. Row 1 failed partly because TestV4ReadReportFlow (cmd/deskperf) rebuilds deskperf through HOME's default module cache and fails with `module lookup disabled by GOPROXY=off` (medici-finance/assay#4683). The re-run bracket also mounted the provisioned cache read-only at HOME's default module path. Its witness is the one above.
- **Row 1** exits 1 in the container, for an environment reason outside this deliverable. In diagnostic D3, row 1's go test runs in the same container with the same mounts but outside verifyrun, and it exits 0: all five packages are ok. D4 adds only the git variables that verifyrun puts into every admitted row (GIT_NO_REPLACE_OBJECTS, GIT_ATTR_NOSYSTEM, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT, GIT_ASKPASS, and GIT_CONFIG_COUNT=2 pinning grep.patternType and grep.extendedRegexp). With those added, it exits 1. One test fails: TestLandedApprovalContext in cmd/deskevidence, with `lost repository authority: origin="" err=gitcore: remote-url origin: no URL configured`. gitcore's OriginConfigInjected treats any non-zero GIT_CONFIG_COUNT as injected command-priority config and withholds origin authority, so the test's fixture loses its origin. This is the class medici-finance/assay#2715 records: git-fixture tests that fail under verifyrun's admitted row env. That test is not in this brief's diff. medici-finance/assay#5356 had attributed this same test to a bind-mounted TMPDIR. D3 and D4 both use a tmpfs TMPDIR, and only the admitted git env decides the result.
- **Row 11** is could-not-run on the darwin host (no `unshare --net`), and it is a pre-merge form (medici-finance/assay#4560 class): from the home, `git merge-base origin/main HEAD` is HEAD itself, so the diff it judges is empty. In supplemental S2, with `--base 45b5095f0c5674c60027f131c3ed1f31f36b9d95` (the first parent of the delivering merge), it exits 0 with `summary: 9 corroborated, 0 disproved, 4 unchecked, 0 brief(s) claiming nothing`. The 4 unchecked entries are the out-of-scope consumers. S2 earns no credit.

| Probe | Command | Expected | Observed | Date | Runner |
|-------|---------|----------|----------|------|--------|
| D1 (row 1, first bracket's mounts; no credit) | row 1's go test packages, plain shell in the same container | diagnostic only | exit 1; cmd/deskperf TestV4ReadReportFlow fails with `module lookup disabled by GOPROXY=off`; deskkit, untrustcorpus, deskevidence, verifyloop ok | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |
| D3 (row 1, re-run bracket's mounts; no credit) | row 1's go test packages, plain shell in the same container, outside verifyrun | diagnostic only | exit 0; all five packages ok | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |
| D4 (row 1, D3 plus verifyrun's admitted git env; no credit) | as D3, with the admitted GIT_* variables set | diagnostic only | exit 1; only cmd/deskevidence TestLandedApprovalContext fails (origin authority withheld under GIT_CONFIG_COUNT=2) | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |
| S2 (row 11, delivery base; no credit) | `statusgen --consumers --brief desk-supervision/37 --base 45b5095f0c56…` | diagnostic only | exit 0; `9 corroborated, 0 disproved, 4 unchecked` | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |

Risk-bearing values: I enumerated the literals in the delivered diff of PR medici-finance/assay#5329 (verifyattempts.go, the deskperf reruns command, the deskevidence outcome writer, statusgen's outcome reader and the verifyloop record join), ranked them by how much a wrong value would cost, and derived the top entries.

RISK-VALUE: DERIVED — MaxAttempts = 16 @ tools/desk/internal/deskkit/verifyattempts.go:47 — the brief's Task 1 bounds the attempts list at 1 to 16 entries; row 3's 17-entry subtest pins it.

RISK-VALUE: DERIVED — rows_run bound `rows < 1 || rows > rowsTotal` @ tools/desk/internal/deskkit/verifyattempts.go:235 — the brief states 1 <= rows_run <= rows_total; row 3 pins both ends.

RISK-VALUE: DERIVED — `--days` default deskkit.PerfMaxDays @ tools/desk/cmd/deskperf/reruns.go:140 — PerfMaxDays = perfRetentionDays (tools/desk/internal/deskkit/perfread.go:32) = perfrec.RetentionDays (tools/desk/internal/deskkit/perf.go:56) = 7 (perfrec/perfrec.go:50), the perf retention window; no eighth day exists to read, and row 5 pins the pruned-window refusal.

RISK-VALUE: DERIVED — closed disposition and reason sets @ tools/desk/internal/deskkit/verifyattempts.go:31 — credited, uncredited and discarded, with the reasons local-build, not-credited, credit-unchecked, env-fault and superseded: exactly the sets the brief's Task 1 names.

RISK-VALUE: N/A — no timeout, host, endpoint, file mode, permission or credential-handling literal is introduced; refusals reuse the existing exit codes, and each value above can be reversed by an edit and a release.

**VERIFY: BLOCKED** — 10 of 12 rows pass at 612750a12406 in one closed v1.0.81 container bracket (verdict=match). No obligation of this brief was observed failing. Row 1 exits 1 only under verifyrun's admitted git env: TestLandedApprovalContext loses origin authority under GIT_CONFIG_COUNT (medici-finance/assay#2715 class), a test outside this brief, and the same command without that env exits 0, which is diagnostic and not credit. Row 11 is could-not-run as written, a pre-merge consumers form in the medici-finance/assay#4560 class.
### Non-implementer verifier run — 2026-11-12 at 680102a08777 (claude-opus-5-5)

This pass verifies merged main at 680102a08777. The home was detached and admitted by deskdispatch --check-verifier (attestation medici-finance/assay#6299). The implementation is PR medici-finance/assay#5329, merge 84a4a4400803, first parent 45b5095f0c56. Since the 2026-11-10 source 612750a12406, none of the brief's core files has changed: verifyattempts.go, verifyoutcomejoin.go, reruns.go, the reruns fixtures, outcomerecord.go and the two neighbour tests. The deskperf usage text and report.go changed for another brief's forge-tag labels, and the verify-desk skill and the desk README gained text that touches none of row 9's strings. The runner took no part in the implementation.

All 12 rows ran in one closed admission bracket: verify-runner v1.0.83 pinned by digest, --network none, sandbox=container-netns, and a fresh module cache that the open step filled. The cache was mounted read-only at both GOMODCACHE and HOME's default module path. The bracket opened at 20:58:28Z and closed at 21:02:23Z, and `--bracket-digest` reports verdict=match.

Verification-Attestation: medici-finance/assay#6299 run=02d876ebc37f14c8b9e4106e1fe63c27e74b83d1f703debf2dcec24e6f610dcc source=680102a0877755feab9ef85dc4e1d3f52a137a54 brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong

Bracket witness (rows 1-12):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/deskperf/... ./cmd/verifyloop/... > "${TMPDIR:-/tmp}/b37-pkgs.out" 2>&1 && grep -F -e 'cmd/deskperf' "${TMPDIR:-/tmp}/b37-pkgs.out"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_GraphExecution02Fixture$' -v > "${TMPDIR:-/tmp}/b37-fixture.out" 2>&1 && grep -F -e '--- PASS: TestReruns_GraphExecution02Fixture' "${TMPDIR:-/tmp}/b37-fixture.out"` | pass exit=0 sandbox=container-netns | sha256:e698b508db85 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckAttempts$' -v > "${TMPDIR:-/tmp}/b37-check.out" 2>&1 && grep -F -e '--- PASS: TestCheckAttempts' "${TMPDIR:-/tmp}/b37-check.out"` | pass exit=0 sandbox=container-netns | sha256:11a0ab17c1b5 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_InvalidStoredRecordNotCounted$' -v > "${TMPDIR:-/tmp}/b37-reader.out" 2>&1 && grep -F -e '--- PASS: TestReruns_InvalidStoredRecordNotCounted' "${TMPDIR:-/tmp}/b37-reader.out"` | pass exit=0 sandbox=container-netns | sha256:c53732ec3913 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskperf/ -run '^TestReruns_NeverEstimates$' -v > "${TMPDIR:-/tmp}/b37-honest.out" 2>&1 && grep -F -e '--- PASS: TestReruns_NeverEstimates' "${TMPDIR:-/tmp}/b37-honest.out"` | pass exit=0 sandbox=container-netns | sha256:0aeb58d46fb2 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -v > "${TMPDIR:-/tmp}/b37-writer.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b37-writer.out" && grep -F -e 'attempts' "${TMPDIR:-/tmp}/b37-writer.out"` | pass exit=0 sandbox=container-netns | sha256:2200d8ab79d5 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b37-statusgen.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b37-statusgen.out"` | pass exit=0 sandbox=container-netns | sha256:f86c59366680 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -v > "${TMPDIR:-/tmp}/b37-verifyloop.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b37-verifyloop.out"` | pass exit=0 sandbox=container-netns | sha256:497a86587e5c | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -F -e 'local-build' docs/streams/desk-supervision/verify-wake-v1.md && grep -F -e 'local-build' tools/desk/cmd/deskdispatch/references/verifier-prompt.md && grep -F -e '"brief":"<qualified-brief-id>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'assay:assay:graph-execution:2' plugins/assay/skills/verify-desk/SKILL.md && ! grep -F -e '"brief":"<stream>/<NN>"' plugins/assay/skills/verify-desk/SKILL.md && grep -F -e 'deskperf reruns' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:e903f37caa0a | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -f tools/desk/internal/deskkit/verifyattempts.go && ! grep -n -i -e 'json:"[a-z_]*token' -e 'json:"[a-z_]*cost' -e 'json:"[a-z_]*wall' -e 'json:"[a-z_]*model' tools/desk/internal/deskkit/verifyattempts.go` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --consumers --brief desk-supervision/37 --base $(git merge-base origin/main HEAD)` | fail exit=2 sandbox=container-netns | sha256:4aeb4b29e648 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfBriefRef$' -v > "${TMPDIR:-/tmp}/b37-ref.out" 2>&1 && grep -F -e '--- PASS: TestPerfBriefRef' "${TMPDIR:-/tmp}/b37-ref.out" && go test ./cmd/deskperf/ -run '^TestReruns_JoinsQualifiedWorkRefOnly$' -v > "${TMPDIR:-/tmp}/b37-join.out" 2>&1 && grep -F -e '--- PASS: TestReruns_JoinsQualifiedWorkRefOnly' "${TMPDIR:-/tmp}/b37-join.out"` | pass exit=0 sandbox=container-netns | sha256:07483c7fa504 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:f248930a0b58b0881b8dd4d1a4ee31e42201ff3a11b9e2dcc2f255b3264942b9
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6299 run=02d876ebc37f14c8b9e4106e1fe63c27e74b83d1f703debf2dcec24e6f610dcc source=680102a0877755feab9ef85dc4e1d3f52a137a54 brief=docs/streams/desk-supervision/brief-37-verification-reruns-as-waste.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=02d876ebc37f14c8b9e4106e1fe63c27e74b83d1f703debf2dcec24e6f610dcc source=680102a0877755feab9ef85dc4e1d3f52a137a54 tree=sha256:f248930a0b58b0881b8dd4d1a4ee31e42201ff3a11b9e2dcc2f255b3264942b9 witness=sha256:ae0b926ee2e24a26dd95f4713ac41d82cb269a40ee39ab60c85df612345add9e opened=2026-11-12T20:58:28Z closed=2026-11-12T21:02:23Z before=receipt after=check-verifier

- **Rows 2 to 10 and 12** pass in the bracket.
- **Row 1** exits 1 for an environment reason outside this deliverable. In no-credit diagnostics, row 1's go test ran in the same image with the same mounts but outside verifyrun. Four packages were ok: internal/deskkit, internal/deskkit/untrustcorpus, cmd/deskperf and cmd/verifyloop. One test failed: TestLandedApprovalContext in cmd/deskevidence (landed_test.go:537, `lost repository authority: origin="" err=gitcore: remote-url origin: no URL configured`). The failure appears with verifyrun's admitted git env (GIT_CONFIG_COUNT=2) and also with only GIT_CONFIG_COUNT=1. The 2026-11-10 pass saw the same command exit 0 with no GIT_CONFIG variables set. That test belongs to the closure-snapshot fix, not to this brief. Attached to the class issue medici-finance/assay#2715.
- **Row 11** exits 2 because the form is pre-merge. From the detached home, `git merge-base origin/main HEAD` is HEAD itself, so statusgen prints `COULD-NOT-CHECK: assay:assay:desk-supervision:37 is not in the diff against 680102a08777…`. In supplemental S2, `--base 45b5095f0c56…` (the delivering merge's first parent) exits 0 with `9 corroborated, 0 disproved, 4 unchecked`. The 4 unchecked entries are the out-of-scope consumers. S2 earns no credit. Attached to the class issue medici-finance/assay#6099.
- **Attempts.** An earlier bracket in this dispatch (opened 20:53:46Z, closed 20:57:46Z) was superseded. Its HOME mount left HOME unwritable, so the roster config was never installed. It gave the same row results, and it is kept on the outcome record as a superseded attempt.

| Probe | Command | Expected | Observed | Date | Runner |
|-------|---------|----------|----------|------|--------|
| D1 (row 1, admitted git env; no credit) | row 1's go test packages, plain shell in the same container, with the admitted GIT variables | diagnostic only | exit 1; only cmd/deskevidence TestLandedApprovalContext fails (origin URL missing); four other packages ok | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| D2 (row 1, safe.directory only; no credit) | as D1, with GIT_CONFIG_COUNT=1 safe.directory in place of the admitted git env | diagnostic only | exit 1; the same single test fails with the same message | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| S1 (row 11 as written, host; no credit) | row 11's command on the host from the home | diagnostic only | exit 2; COULD-NOT-CHECK, the brief is not in the diff against 680102a08777 | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| S2 (row 11, delivery base; no credit) | `statusgen --consumers --brief desk-supervision/37 --base 45b5095f0c56…` | diagnostic only | exit 0; `9 corroborated, 0 disproved, 4 unchecked` | 2026-11-12 | assay-verifier-app[bot] @ 680102a08777 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Risk-bearing values: the risk fields are all no and none is irreversible. The literals in the delivered diff of PR medici-finance/assay#5329 were enumerated anyway, covering verifyattempts.go, the deskperf reruns command, the deskevidence outcome writer, statusgen's outcome reader and the verifyloop record join. They were ranked, and the top entries were derived. Line numbers were re-read at 680102a08777.

RISK-VALUE: DERIVED — MaxAttempts = 16 @ tools/desk/internal/deskkit/verifyattempts.go:47 — Task 1 bounds the attempts list at 1 to 16 entries; row 3's 17-entry subtest pins it.
RISK-VALUE: DERIVED — rows_run bound `rows < 1 || rows > rowsTotal` @ tools/desk/internal/deskkit/verifyattempts.go:235 — Task 1 states 1 <= rows_run <= rows_total; row 3 pins the upper end.
RISK-VALUE: DERIVED — `--days` default deskkit.PerfMaxDays @ tools/desk/cmd/deskperf/reruns.go:140 — PerfMaxDays = perfRetentionDays (tools/desk/internal/deskkit/perfread.go:32) = perfrec.RetentionDays (tools/desk/internal/deskkit/perf.go:60) = 7 (perfrec/perfrec.go:50). That is the perf retention window, and no eighth day exists to read. Row 5 pins the pruned-window state.
RISK-VALUE: DERIVED — closed disposition and reason sets @ tools/desk/internal/deskkit/verifyattempts.go:31-44 — credited, uncredited and discarded, with the reasons local-build, not-credited, credit-unchecked, env-fault and superseded. These are exactly the sets Task 1 names.
RISK-VALUE: N/A — enumeration over the same diff found no timeout, host, endpoint, file mode, permission or credential-handling literal; refusals reuse the existing exit codes, and every value above can be reversed by an edit and a release.

**VERIFY: BLOCKED** — 10 of 12 rows pass at 680102a08777 in one closed v1.0.83 container bracket (verdict=match). No obligation of this brief was observed failing. Row 1 exits 1 only through TestLandedApprovalContext, a test outside this brief that loses its fixture origin under any inherited GIT_CONFIG_COUNT (medici-finance/assay#2715 class). Row 11 cannot give a verdict as written, because it is a pre-merge consumers form (medici-finance/assay#6099 class).

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
