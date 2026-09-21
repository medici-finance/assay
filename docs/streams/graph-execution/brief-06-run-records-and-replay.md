---
brief: assay:assay:graph-execution:06
title: Run records and the replay/learning loop
why: >-
  When a run fails today, what is left is a transcript and a memory. Nothing records, in a
  form a machine can replay, which pattern was chosen and why, which edge blocked, what a
  tool returned, or who intervened. This brief gives every run a structured record, turns
  one failed run into a replay fixture, proposes one reviewed pattern revision from it, and
  proves the revision fixes the motivating cases without changing the ones it did not touch.
  A human decides what ships; the loop proposes, it never promotes.
wave: 3
depends: ["graph-execution/05", "graph-execution/09"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/admission-assurance-spec.md — 2026-09-18 integration amendment"
  - "freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f"
  - "docs/streams/graph-execution/spec.md §2.5 (run record → recurring failure → replay fixture → small change → regression over motivating cases and holdouts → reviewed next version; outcome and permitted method scored independently) and §4 (one incident-derived improvement replayed over its motivating cases and unchanged holdouts)"
  - "graph-execution/05 (the eight fixtures and the `statusgen experiment` harness this brief replays through; the failed run its replay fixture derives from) and graph-execution/02 (`spec/workflow-pattern-v1.md` (planned) — the `supersedes:` and `version:` keys a v2 pattern file carries)"
  - "schemas/desksupervise-status-v1.json (the JSON-schema convention this repo uses for a machine-readable contract: `$id` under schemas/, `additionalProperties: false`, every field three-state per docs/three-state-instrument-rule.md) and schemas/brief-v2.json"
  - "Replit: trace-to-PR at https://youtu.be/J8XxVnqUjYE?t=260, the human ship/wait/drop decision at https://youtu.be/J8XxVnqUjYE?t=292 — the reason promotion is a review, never an automatic step"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `ls schemas | grep -i run` returns nothing and `grep -rln 'run-record' docs statusgen schemas` returns only this stream's spec and README — no run-record schema or replay path exists; not already satisfied"
exec-tier: strong
exec-tier-why: "(a) the run-record field set and the replay-fixture derivation are design decisions the facts do not fully pre-specify; (b) correctness is cross-artifact — a pattern revision, its regression run and the holdout assertion must agree, and a loop that scores outcome only would silently reward a route that broke a constraint."
domain: complex
version: 2
id: 227c272b-dc4c-4528-9066-9e8db14d223d
---

# Brief 06 — Run records and the replay/learning loop

## Context
files: `schemas/run-record-v1.json` (planned), `statusgen/runrecord.go` (planned) (writer + validator), `statusgen/runrecord_test.go` (planned), `statusgen/replay.go` (planned) (`statusgen replay` subcommand), `statusgen/replay_test.go` (planned), `statusgen/experiment.go` (planned) (emits a run record per case; from graph-execution/05), `statusgen/testdata/graph-execution/replay/` (planned) (the fixture derived from one failed run), `spec/workflow-patterns/<name>-v2.yaml` (planned) (the one revision), `docs/streams/graph-execution/replay-01-report.md` (planned), `docs/lifecycle.md` (one paragraph: a pattern revision is a reviewed PR, never a promotion), `changelog/graph-execution-06-run-records-and-replay.md` (planned)
facts:
  - `domain: complex` because which failure recurs, and therefore which revision is worth proposing, is not knowable before the 05 fixtures have run — the brief probes (run, record), senses (which case failed and why), responds (one revision). The record schema and the regression rule are ordered work; the choice of revision is not.
  - Run record fields (the contract, fixed here so 05's harness and any later trial emit the same shape): `schema` (const `run-record-v1`), `run_id`, `attempt_id`, `pattern` `{id, version}`, `selected_because` (an enumerated reason: `declared | default | only-candidate`), `nodes[]` each `{id, kind, eligibility: {verdict, reason}, blocked_edge, tool_outcome: {command, exit, key_output}, evidence_refs[]}`, `interventions[]` each `{kind: hold | override | retry | stop, by_role}` — role NAME only (`desk | reviewer | verifier | worker | human`), never a login or a person; `environment` `{model, statusgen_version, tool_versions{}}`. Every field is three-state: `n/a` or `could-not-check` where unreadable, never a default zero or empty string that reads as a fact. No free-text reasoning field exists in the schema, by design (spec §2.5).
  - Replay fixture: `statusgen replay --record <run-record.json> --root <fixture>` (planned) re-runs the 05 harness over the fixture named in the record with the record's pattern version and asserts the same per-node eligibility and tool outcomes; a divergence is reported per node. The fixture under `testdata/graph-execution/replay/` (planned) is derived from ONE failed run of the 05 experiment (whichever case the report records as `fail`; if every case passed, the implementer injects a deterministic fault into a copy of case 4 — wrong-revision evidence — and says so in the report).
  - Regression rule: a revision `<name>-v2.yaml` (planned) with `supersedes: <name>-v1` passes only when (i) every motivating case (the replay fixture plus any 05 case that failed for the same reason) now passes, AND (ii) every unchanged holdout (the 05 cases the revision does not claim to affect) produces a run record byte-identical in `nodes[].eligibility` and `nodes[].tool_outcome` to its v1 record. A revision that changes a holdout is a red result, even if the holdout still "passes".
  - Two scores, never one: `outcome` (did the case reach its expected released state) and `permitted_method` (did every node's applied effects ⊆ the pattern's declared effects and did no gate node get bypassed). The replay report shows both columns; a run can pass outcome and fail method, and that row is RED. Task success alone is not the objective (spec §2.5).
  - Promotion is a pull request. The v2 file lands in a draft PR with the replay report; the review is the ship/wait/drop decision. No verb, flag or workflow promotes a revision to the selected default; `docs/lifecycle.md` says so in one paragraph.
  - Single-point-of-failure note: the ONE control is the holdout byte-identity assertion (ii). Second, independent layer: the `permitted_method` column reddens a revision that reaches the outcome by widening authority even when every holdout is untouched; third, out-of-band: the 02 pattern lint (`pattern-effect-exceeds-role`) runs on the v2 file in CI regardless of what the replay says.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: records, fixtures and the report carry `example-org/*` placeholders and role names only; no login, no adopter repo, no adopter number.
- Exactly ONE pattern revision is proposed in this brief. A second observed failure is a finding entry, not a second revision.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Integration amendment — 2026-09-18

This is the single run-record/replay schema owner. Use 09 instance and canonical Cell/work/node identities; record immutable input/acceptance/policy references, effect receipts, assessment/policy references when present, source freshness and provenance. Add compatible optional extension fields or version the schema with explicit reader refusal for mandatory semantics. Sensitive payloads are permissioned references, not embedded transcripts. Do not copy this record into a second migration schema. Learned selection is only a recorded reason tied to a separate assessment; policy still controls admission.

## Task
1. **Schema.** `schemas/run-record-v1.json` (planned) per the facts; `$id` under `schemas/`, `additionalProperties: false`, `required` lists every top-level field. `statusgen/runrecord.go` (planned): `WriteRunRecord` and `ValidateRunRecord`; the validator refuses a record with an unknown field, a non-enumerated `selected_because` or `interventions[].kind`, or a `by_role` that is not one of the five role names.
2. **Emit.** `statusgen experiment` (from 05) writes one run record per case to `--records-dir` (planned flag); `TestExperimentEmitsValidRecords` (planned) validates every emitted record against the schema.
3. **Replay.** `statusgen replay --record <file> --root <fixture> [--pattern-version N] [--json]` (planned): re-runs and diffs per node; exit 0 only on no divergence; a could-not-check input is reported and fails the run.
4. **Fixture + revision.** Derive `statusgen/testdata/graph-execution/replay/` (planned) from one failed 05 run (facts). Author `spec/workflow-patterns/<name>-v2.yaml` (planned) with `supersedes:` and a one-line `changed:` rationale. Run the 02 pattern lint over it.
5. **Regression.** `statusgen replay --regress --pattern <name> --from v1 --to v2` (planned): motivating cases must pass; holdouts must be byte-identical in the two fields named in the facts; both `outcome` and `permitted_method` columns printed. `TestReplayHoldoutDriftIsRed` (planned) mutates a holdout's expected record and asserts red; `TestReplayMethodFailIsRed` (planned) injects an extra applied effect into a passing outcome and asserts red.
6. **Report + docs.** `docs/streams/graph-execution/replay-01-report.md` (planned): the regression table as the binary prints it (between `<!-- replay:table:begin -->`/`<!-- replay:table:end -->` markers), which case motivated the revision, what changed, the two scores per case, and the honest limit that one revision over eight offline cases is a demonstration of the loop, not evidence that it improves real work. One paragraph in `docs/lifecycle.md`. `changelog/graph-execution-06-run-records-and-replay.md` (planned).

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go test -run 'TestRunRecord' ./... && go test -run 'TestReplay' ./...` | exit 0 |
| 2 | check:ci +dereference | `cd statusgen && go run . experiment --all --records-dir /tmp/ge06-records > /dev/null && go run . runrecord validate --dir /tmp/ge06-records; echo rc=$?; ls /tmp/ge06-records` | `rc=0` and 8 `.json` files listed — every emitted record validates against the committed schema, not against the writer's own idea of it |
| 3 | check:ci +mutation | `cd statusgen && go test -run 'TestReplayHoldoutDriftIsRed' -v ./...` | exit 0; output contains `holdout drift` — a revision that touches an untouched case reddens |
| 4 | check:ci +mutation | `cd statusgen && go test -run 'TestReplayMethodFailIsRed' -v ./...` | exit 0; output contains `permitted_method: fail` alongside `outcome: pass` — the two scores are independent and the method score alone reddens the row |
| 5 | check +dereference | `cd statusgen && go run . replay --regress --pattern $(basename -s -v2.yaml ../spec/workflow-patterns/*-v2.yaml) --from v1 --to v2 --markdown > /tmp/ge06-table.md; awk '/<!-- replay:table:begin -->/,/<!-- replay:table:end -->/' ../docs/streams/graph-execution/replay-01-report.md > /tmp/ge06-committed.md; diff /tmp/ge06-table.md /tmp/ge06-committed.md; echo rc=$?` | `rc=0` — the committed regression table is byte-identical to a fresh run; a report claiming a pass the binary does not reproduce fails here (exactly one `*-v2.yaml` exists, per the Ground rules, so the glob resolves to one name) |
| 6 | check +dereference | `grep -c '"reasoning"' schemas/run-record-v1.json; grep -c 'additionalProperties": false' schemas/run-record-v1.json` | first count 0 (no free-text reasoning field), second ≥ 1 — the schema forbids the field class spec §2.5 rules out, rather than merely omitting it |
| 7 | check +neighbour | `cd statusgen && go run . patterns --lint` | exit 0 — the v2 pattern file passes the 02 lint (`pattern-effect-exceeds-role`) independently of what the replay reports |
| 8 | check +neighbour | `cd statusgen && go test -run 'TestExperiment' ./...` | exit 0 — the 05 harness still passes with record emission added |
| 9 | check +flow | `cd statusgen && go run . experiment --root testdata/graph-execution/replay --records-dir /tmp/ge06-one > /dev/null; go run . replay --record /tmp/ge06-one/*.json --root testdata/graph-execution/replay; echo rc=$?` | `rc=0` — a record written by one run replays cleanly through the other subcommand: emit → validate → replay is one path, not three |
| 10 | check +dereference | `grep -n -i 'promot' docs/lifecycle.md` | ≥ 1 line and it states the revision lands as a reviewed pull request; a lifecycle doc that describes an automatic promotion fails the reviewer's reading |
| 11 | check:ci +flow | `cd statusgen && go test -count=1 -v -run TestRunRecordInstanceReferencesRoundTrip ./...` | exit 0; named test PASS; instance, subject and optional assessment references survive emit/validate/replay |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The reviewer's questions: (1) was the replay fixture derived from a run the 05 report actually records as failed, or from an injected fault — and does the report say which? (2) does the v2 revision reach its outcome by narrowing a declaration (acceptable) or by widening an effect set (row 4 must have caught it)? (3) is there exactly one revision?
