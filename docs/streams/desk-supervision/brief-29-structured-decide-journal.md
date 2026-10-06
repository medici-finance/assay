---
brief: assay:assay:desk-supervision:29
title: Decide journal as structured records, with observed outcomes joined later
why: >-
  When a desk loop asks a model to choose between a fixed set of moves, the log keeps only a
  flattened text line: no list of options, no confidence numbers, and no place to record what
  actually turned out to be right. Without those, no one can ever measure whether a cheap
  decision model is trustworthy enough to rely on, or compare one against another. This brief
  keeps each decision as a structured record and lets the real outcome be attached later
  without changing the original.
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
  (a) the record shape, the observation-attachment rule and the advice-to-journal plumbing for
  calibrated predictions are design decisions the facts do not fully fix; (b) correctness spans
  decide.go, decisionassessment.go, a new journal sink, a deskaudit verb pair and two specs;
  (c) a record that silently drops probabilities or abstentions passes a happy-path test and
  biases every later calibration measurement.
domain: complicated
outcome: none
sources:
  - "desk-supervision: Decide journal as structured records, with observed outcomes joined later"
  - "tools/desk/internal/deskkit/decide.md — the Decide contract (Journalled row: question, context digest, answer, justification, elapsed, outcome)"
  - "spec/decision-assessment-v1.md — the Prediction envelope (§4) and its projection into Decide (§6); §7 leaves calibration evaluation to graph-execution/12"
  - "docs/streams/graph-execution/brief-12-decision-evaluation.md — the evaluator (reliability, risk–coverage, abstention) that needs prediction records joined to adjudicated labels"
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models and statistical learning to the pipeline found the Decide journal is not analysable — vocabulary, default and probabilities are not recorded and outcomes cannot be joined"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): DecisionRecord (decide.go:225-238) is flattened by AuditJournal.Record into a detail string carrying only item/outcome/answer/elapsed/justification (decide.go:504-519) — vocabulary, default, prompt and detail are dropped; PredictionAdvisor projects a validated Prediction to Advice and discards the distribution (decisionassessment.go:239-259, 282-300); no observed-outcome store exists"
consumers:
  - "tools/desk/internal/deskkit/decide.go (Advice gains an optional Prediction; Decide builds and tees the structured record): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/decidejournal.go (planned — record + observation writers, reader, join): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/decisionassessment.go (PredictionAdvisor attaches the validated Prediction, including abstentions): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/decide.md (new section: decide-record-v1 and decide-observation-v1 — the canonical schema): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "spec/decision-assessment-v1.md (§6 gains step 4: the validated distribution reaches the journal record): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskaudit/main.go (verbs decide-observe and decide-export): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (deskaudit row + Decide journal paragraph): follow-up desk-supervision/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/commsloop/decide.go and tools/desk/cmd/commsgw/prosegate.go (the two Decide callers): out-of-scope (both pass a nil Journal, so the default journal change reaches them with no edit; Verify row 1 runs their packages' tests)"
  - "docs/records-and-retention.md (lists the two new local files): follow-up desk-supervision/35"
  - "graph-execution/12 evaluator (planned, not on main): out-of-scope (unimplemented; decide-export's joined line is the input shape it can read, and this brief changes nothing there)"
---

# Brief 29 — Decide journal as structured records, with observed outcomes joined later

## Context
files:
- **edit** `tools/desk/internal/deskkit/decide.go` — `Advice` gains `Prediction *Prediction` (optional,
  additive); `Decide` builds a `DecideRecordV1` beside the existing `DecisionRecord`; the nil-Journal
  default becomes a tee of `AuditJournal` (unchanged) and the new `StructuredJournal`.
- **add** `tools/desk/internal/deskkit/decidejournal.go` (planned) + `decidejournal_test.go` (planned) —
  record type, observation type, the two append-only writers, reader, and the join used by export.
- **edit** `tools/desk/internal/deskkit/decisionassessment.go` — `PredictionAdvisor.Advise` attaches
  the validated `Prediction` to the returned `Advice`, including on the abstention path.
- **edit** `tools/desk/internal/deskkit/decide.md` — new section "Structured journal" carrying both
  field tables (the canonical schema).
- **edit** `spec/decision-assessment-v1.md` — §6 step 4.
- **edit** `tools/desk/cmd/deskaudit/main.go` + `main_test.go` — verbs `decide-observe`, `decide-export`.
- **edit** `tools/desk/README.md` — deskaudit row and a short Decide-journal paragraph.
- **add** `changelog/desk-supervision-29.md` (planned).

facts (all read 2026-10-06 @ 1fbf1153f):
- `DecisionRecord` (`decide.go:225-238`) already holds ts, loop, item, prompt, detail, contextDigest,
  vocabulary, default, answer, justification, elapsed, outcome. The default sink `AuditJournal.Record`
  (`decide.go:504-519`) writes one `decide` verb line whose `Detail` packs only
  item/outcome/answer/elapsed/justification — vocabulary, default, prompt are lost.
- Outcome vocabulary is closed: `advised`, `default-disabled`, `default-no-advisor`, `default-budget`,
  `default-timeout`, `default-error`, `default-invalid`, `default-unjournalled` (`decide.go:241-260`).
- Journal-or-discard: an advised answer whose journal write fails is downgraded to the default and
  re-journalled as `default-unjournalled` (`decide.go:465-487`).
- The local store: `deskDir()` = `$HOME/.config/assay` (`killswitch.go:65-80`); the audit log is
  `audit.jsonl` there (`audit.go:88-94`), append-only, mode 0600. New files go in the same directory.
- `SessionTag()` (`audit.go:270-281`) reads `DESK_SESSION`, then the agent session id, else `unknown`.
- `PredictionAdvisor.Advise` (`decisionassessment.go:282-300`) validates the `Prediction`, then projects
  it through `ToAdvice` (`:239-259`), which keeps only the top label and a formatted justification; an
  abstention returns a zero `Advice`, which `Decide` records as `default-invalid` — indistinguishable
  from a malformed answer.
- Callers today: `tools/desk/cmd/commsloop/decide.go:202-208` and `tools/desk/cmd/commsgw/prosegate.go:164-170`,
  both with a nil-defaulting `Journal` field.
- `dispatch_ref` as defined by desk-supervision/28. A Decide call may run outside any dispatch; when the
  ref is not available the record omits it and joins by `session_tag` + `item` instead.
- single-point-of-failure: the observation writer's membership check (the record exists; the observed
  label is in that record's own vocabulary) — backed by `decide-export`'s independent re-validation on
  read, which skips and counts any record or observation that fails, however it got onto disk.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Record schema `decide-record-v1`** (one JSON line per consult, file `decide-records.jsonl` in
   `deskDir()`, append-only, 0600, written under the same flock discipline as `audit.jsonl`). Fields:
   `schema` (`decide-record-v1`), `record_id` (128-bit random hex), `ts`, `loop`, `item`,
   `question_id` (sha256 of prompt + vocabulary in order + default — the decision-class key; changes
   whenever the vocabulary changes, so calibration never pools across vocabularies), `prompt` (the
   fixed code-literal question text), `detail_digest`, `context_digest`, `vocabulary`, `default`,
   `answer`, `outcome`, `elapsed_ms`, `session_tag`, optional `dispatch_ref`, optional `advisor_tier`
   (closed set `any` / `strong`, from a new optional `Consult.AdvisorTier`; any other value → the
   field is omitted and the audit line notes it), and — only when the advisor supplied a `Prediction`
   that passes the check in Task 3 — `label_probabilities`, `shadow_labels`, `abstained`,
   `provider_version`, `calibrator_version`, `actual_backend`. **Never written:** raw context, raw
   detail, the advisor's justification text (it is model output that may echo untrusted context; the
   existing audit line keeps carrying it exactly as today), evidence content.
2. **Default journal = tee.** A nil `Consult.Journal` writes BOTH the unchanged `AuditJournal` line
   and the structured record; the tee returns an error if either write fails, so journal-or-discard
   now covers the structured record too (an advised answer with no structured record is downgraded
   to the default). A caller-supplied `Journal` is untouched; export `StructuredJournal` so a
   caller can compose it.
3. **Prediction reaches the record.** `Advice` gains `Prediction *Prediction`. `PredictionAdvisor`
   sets it after `ValidatePrediction` passes, on BOTH the answer path and the abstention path. In
   `Decide`, re-check it independently before recording: every label in the record's vocabulary,
   probabilities finite in [0,1] summing to 1 within `PredictionNormalizationTolerance`, no label in
   both maps (call `ValidatePrediction` with a request built from the question's vocabulary and the
   prediction's own subject). A failing prediction is not recorded (record carries
   `prediction_rejected: true`); the answer path is unchanged either way. An abstention is recorded
   as `abstained: true` with outcome `default-invalid`, so analysis can tell abstention from garbage.
4. **Observation schema `decide-observation-v1`** (file `decide-observations.jsonl`, same dir, same
   discipline). Fields: `schema`, `observation_id`, `record_id`, `observed_label` (must be in the
   referenced record's `vocabulary`), `observed_at` (RFC3339, not before the record's `ts`),
   `source` (closed set: `human-ruling`, `pipeline-outcome`, `adjudication`), `evidence_ref` (a typed
   reference only — `owner/repo#N`, `owner/repo@<40-hex sha>`, or `<stream>/<NN>`; anything else
   refused), `observer_role` (closed set: the five desk role slugs, or `human` — never a name).
   The original record is never rewritten; several observations per record are allowed.
5. **`deskaudit decide-observe --record <id> --label <L> --source <S> --ref <R> --role <role>`** —
   validates per Task 4 against the record read from disk and appends; every refusal exits 5 naming
   the field and leaves both files byte-identical.
6. **`deskaudit decide-export [--question <id>] [--since <RFC3339>] [--strict]`** — prints one joined
   JSON line per record: the record, its latest observation (by `observed_at`), `observation_count`,
   and `observations_conflict` (true when observations disagree). It re-validates every line it reads
   (schema tag, answer ∈ vocabulary, observation's record exists and label ∈ that vocabulary) and
   SKIPS failures, printing `skipped=<n>` to stderr; `--strict` makes any skip exit 5. This is the
   input an evaluator (reliability/ECE, risk–coverage over `label_probabilities` + `abstained`) reads.
7. **Docs.** decide.md "Structured journal" section with both field tables and the never-written
   list; decision-assessment-v1.md §6 step 4 ("a validated Prediction, including an abstention, is
   carried to the decide-record-v1 journal line; its numbers are recorded, never acted on beyond the
   projection in step 3"); README; changelog fragment.

Must NOT collect (binding, from the shared contract): raw PR/issue/comment text or justification
prose in the structured record; transcripts or prompts beyond the fixed code-literal question;
per-person figures (`session_tag` is a session id, `observer_role` a role slug); the files are local
desk state, never committed or published. These records are inputs to calibration, never targets for
ranking agents or people.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test ./tools/desk/internal/deskkit/... ./tools/desk/cmd/deskaudit/... ./tools/desk/cmd/commsloop/... ./tools/desk/cmd/commsgw/...` | exit 0 | check:ci |
| 2 | `go test ./tools/desk/internal/deskkit/ -run '^TestDecideRecord_NoRawText$' -v > "${TMPDIR:-/tmp}/b29-noraw.out" 2>&1 && grep -F -e '--- PASS: TestDecideRecord_NoRawText' "${TMPDIR:-/tmp}/b29-noraw.out"` | exit 0; a consult whose context, detail and justification each carry a distinct canary string writes a record containing none of the three canaries, and the digests equal sha256 of the inputs | check:ci +mutation |
| 3 | `go test ./tools/desk/internal/deskkit/ -run '^TestDecideRecord_PredictionCarried$' -v > "${TMPDIR:-/tmp}/b29-pred.out" 2>&1 && grep -F -e '--- PASS: TestDecideRecord_PredictionCarried' "${TMPDIR:-/tmp}/b29-pred.out"` | exit 0; subtests: PredictionAdvisor answer → record holds the full `label_probabilities` and `calibrator_version`; abstention → `abstained: true`, outcome `default-invalid`; plain Advisor → no probability fields; a hand-built Advice whose Prediction sums to 0.7 → `prediction_rejected: true`, no probabilities recorded | check:ci +mutation |
| 4 | `go test ./tools/desk/internal/deskkit/ -run '^TestDecideRecord_TeeJournalOrDiscard$' -v > "${TMPDIR:-/tmp}/b29-tee.out" 2>&1 && grep -F -e '--- PASS: TestDecideRecord_TeeJournalOrDiscard' "${TMPDIR:-/tmp}/b29-tee.out"` | exit 0; with the structured file made unwritable, an advised answer returns the default and the audit line reads `default-unjournalled` | check:ci +mutation |
| 5 | `go test ./tools/desk/cmd/deskaudit/ -run '^TestDecideObserve_Refusals$' -v > "${TMPDIR:-/tmp}/b29-obs.out" 2>&1 && grep -F -e '--- PASS: TestDecideObserve_Refusals' "${TMPDIR:-/tmp}/b29-obs.out"` | exit 0; subtests each exit 5 naming the field: unknown record id, label outside the record's vocabulary, unknown source, prose `--ref`, a non-role `--role`, `observed_at` before the record; both files byte-identical after every refusal | check:ci +mutation |
| 6 | `go test ./tools/desk/cmd/deskaudit/ -run '^TestDecideExport_JoinEndToEnd$' -v > "${TMPDIR:-/tmp}/b29-flow.out" 2>&1 && grep -F -e '--- PASS: TestDecideExport_JoinEndToEnd' "${TMPDIR:-/tmp}/b29-flow.out"` | exit 0; a real `Question.Decide` with a PredictionAdvisor (nil Journal), then `decide-observe` twice with different labels, then `decide-export` → one line carrying the probabilities, the later label, `observation_count: 2`, `observations_conflict: true`; the records file is byte-identical before and after both observations | check:ci +flow |
| 7 | `go test ./tools/desk/cmd/deskaudit/ -run '^TestDecideExport_RevalidatesOnRead$' -v > "${TMPDIR:-/tmp}/b29-reread.out" 2>&1 && grep -F -e '--- PASS: TestDecideExport_RevalidatesOnRead' "${TMPDIR:-/tmp}/b29-reread.out"` | exit 0; lines appended directly to the files (bypassing decide-observe): an observation with a label outside the vocabulary and a record whose answer is outside its vocabulary are skipped with `skipped=2` on stderr; `--strict` exits 5 | check:ci +mutation |
| 8 | `go test ./tools/desk/internal/deskkit/ -run '^TestAuditJournalWritesDecideLine$' -v > "${TMPDIR:-/tmp}/b29-audit.out" 2>&1 && grep -F -e '--- PASS: TestAuditJournalWritesDecideLine' "${TMPDIR:-/tmp}/b29-audit.out"` | exit 0 (the existing audit `decide` line is unchanged) | check:ci +neighbour |
| 9 | `go test ./tools/desk/internal/deskkit/ -run '^TestDecideRecordDocMatchesStruct$' -v > "${TMPDIR:-/tmp}/b29-doc.out" 2>&1 && grep -F -e '--- PASS: TestDecideRecordDocMatchesStruct' "${TMPDIR:-/tmp}/b29-doc.out"` | exit 0; the field names in decide.md's two schema tables equal the JSON tags of the two Go types, read by reflection | check:ci +dereference |
| 10 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Untrusted context or model justification leaks into the record | row 2 |
| Probabilities dropped in the projection, so calibration is impossible | rows 3, 6 |
| Abstentions recorded as plain invalid answers, so risk–coverage is wrong | row 3 |
| Advice taken with no structured record (biased sample) | row 4 |
| Observation rewrites or corrupts the original record | rows 5, 6 (byte-identical checks) |
| Observed label outside the vocabulary pollutes the join | rows 5, 7 (writer and reader independently) |
| Existing audit trail or callers break | rows 1, 8 |
| Docs describe a schema the code does not write | row 9 |
| `question_id` pools records across a vocabulary change | review-only (reviewer reads the digest inputs in the diff; a dedicated row would restate the implementation) |
| `dispatch_ref` source differs from desk-supervision/28's definition | review-only until 28 lands its definition |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
