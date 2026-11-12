---
brief: assay:assay:intake-class-routing:01
title: Record and publish a handoff for every judged issue
why: >-
  Intake judges an issue, comments and labels it, and stops. Since 2026-10-26 a placeholder is
  schedulable only after an accountable repair judgment is recorded and published, and none
  has been, so judged issues never reach a worker. The step is in the intake skill and was
  skipped; this brief makes it a short numbered step backed by a procedure whose examples a
  test executes.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
schema: brief-v2
version: 1
id: 9c6ecc61-2d17-4953-9617-81aa1d448343
authored: "2026-11-12 by a strong-tier authoring session (read-only review)"
exec-tier: strong
exec-tier-why: "(a) the procedure's wording and example set are design choices; (b) the examples must agree with the shipped assessment in another module."
domain: complicated
sources:
  - "docs/streams/intake-class-routing/README.md §4.1, §4.2, §4.5, §9 D1, §10"
  - "spec/intake-handoff-v1.md — consumed as shipped, not changed"
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): no reference file for the handoff procedure exists; the skill passage is prose only"
consumers:
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up intake-class-routing/01 (this brief; flips to fixed-here when the implementation edits the path)"
  - ".cursor/skills/intake-desk/SKILL.md: follow-up intake-class-routing/01 (this brief; generated harness copy of the canonical skill)"
  - ".cursor/assay-managed.json: follow-up intake-class-routing/01 (this brief; generated harness content digest)"
---

# Brief 01 — Record and publish a handoff for every judged issue

## Context

files: plugins/assay/references/intake-handoff.md, plugins/assay/skills/intake-desk/SKILL.md, .cursor/skills/intake-desk/SKILL.md, .cursor/assay-managed.json, tools/desk/cmd/scanloop/procedure_examples_test.go, tools/desk/cmd/scanloop/testdata/handoff-procedure-plant.md, changelog/intake-handoff-procedure.md

facts:
- Read at `0665bfb77` (2026-11-12). Re-establish each with the command given.
- Admission rule: `statusgen/nextup.go:594` holds a `placeholder-v1` brief out of Next-up unless `issueflow.RepairDispositionSettled` is true (`grep -n RepairDispositionSettled statusgen/nextup.go`).
- Settled means assessed `observed`, route `repair`, and the successor equal to the source issue (`deskcore/issueflow/intake.go`, `AssessIntakeHandoff` and `RepairDispositionSettled`).
- Writer: `scanloop land --item <owner/repo#N> --opened <time> --kind <reason> --exit <slug> --artifact <typed ref> --tier any|strong --handoff <file> --handoff-reader intake-handoff-v1` (`tools/desk/cmd/scanloop/landverb.go:66`). It appends to the local `intake-handoffs-v1.jsonl` and writes nothing into the stream root.
- The board reads only a JSON file beside the placeholder, named by the placeholder's `handoff:` key, with the event time in `handoff-opened:` (`statusgen/placeholder.go:364`, `statusgen/intakehandoffs.go:17`). The file name must be a bare name ending `.json`.
- The intake skill is 710 lines (`wc -l plugins/assay/skills/intake-desk/SKILL.md`). The handoff rule is the passage headed "Accountable handoff (version-negotiated)": both paragraphs, lines 280 to 312 at this commit, ending "they publish as written."
- Three sentences in that passage are controls, not procedure, and must survive the edit: the scrub instruction for a public repository ("Before committing a companion to a public repository, scrub its free-text `evidence`, `required_action` and `wake_condition` fields; they publish as written.", lines 311-312); "The local history alone is not an assignment authority." (line 287); and "Do not promote raw placeholders or open a new issue merely to acknowledge an observation." (lines 297-298). The handoff spec states the scrub rule too (`grep -n 'scrub' spec/intake-handoff-v1.md`).
- A placeholder's brief id is `issue-loop/issue-<N>` for the scan's home repository and `issue-loop/<name>-issue-<N>` for any other (`placeholderStem`, `statusgen/scanissues.go:277`; the stream name is fixed at `:39`). A filter on the dispatch queue therefore keys on the stream, not on the id's shape.
- The default dispatch view cannot show that a stream has no eligible row. It is a score-ranked selection capped at a span of 20 and at most 4 rows per stream; the per-stream figure is a maximum, not a reservation, and the fill stops when the span is full (`statusgen/nextup.go:25`, `:57`, and the pass-1 loop at `:1096`). A correctly admitted placeholder can be outranked by every higher-priority row. The uncapped read is `statusgen --root <root> --next-up --span 100000`: with the span out of the way, a stream's rows are limited only by its own cap. That cap starts at 4 and is lowered by the stream's claimed items, by a declared `max-concurrent` and by an active drive's per-stream cap (`:1039` to `:1077`). The same output counts every eligible row a stream cap held back, in `heldByStreamCap` (`statusgen/dispatchqueue.go:55`). So that read shows an `issue-loop` row whenever one is eligible, unless a cap held it, and `heldByStreamCap` at 0 rules that out whatever lowered the cap. The output is one line of compact JSON, so a line count of the match is 1 or 0.
- Shared references live in `plugins/assay/references/` and carry an `assay:harnesslint non-matrix-reference` comment (see `milestone-authoring.md`, line 3).
- Class of defect this closes: a procedure step that exists only as prose inside a long skill body and has no executable example. The guard is a test that reads the examples.
- Single control this design leans on: the session following the procedure. An independent tool-side layer is a separate brief in this stream; this brief does not depend on it.

design-fit:
  owner: plugins/assay/skills/intake-desk (the procedure); record semantics stay with spec/intake-handoff-v1.md
  contract: none — procedure text only; no registered meaning changes
  retires: ["the prose-only 'Accountable handoff' passage in the intake skill"]
  weight: "verbs 0, flags 0, refusals 0, skill rule-text lines 0 or fewer, one new reference file"
  why-add: "The step was skipped inside a 710-line body and executable examples cannot live in a skill body. Removal considered: deleting the passage outright, which leaves no procedure at all."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not edit `spec/intake-handoff-v1.md` or `spec/intake-exit-v1.md`.

## Task

1. Re-check the facts above against the tree you are on. If a reference file for this procedure already exists, or the writer's flags differ, report NEEDS_CONTEXT.
2. Write `plugins/assay/references/intake-handoff.md` (planned). It states, in this order:
   - when a judged issue needs a companion and the two cases that need none (an accountable `rejected-watching` exit; an item parked on a `needs-decision` issue);
   - the judgment table from the scoping document §4.1, one row per case;
   - one complete example record per row, each in a fenced `json` block, each preceded by a line `expect: <state> <route>` with a trailing ` direct` when the record is a direct repair. Use placeholder values such as `example-org/example#12`; no real issue, login or installation name;
   - the exact `scanloop land` command for one item;
   - publication by hand: write the companion beside the placeholder as `<placeholder stem>.handoff.json`, set `handoff:` to that file name and `handoff-opened:` to the event time, and commit both in the same reviewed intake change as the placeholder. This step carries the scrub requirement itself, in these words: "Before committing a companion to a public repository, scrub its free-text `evidence`, `required_action` and `wake_condition` fields; they publish as written." It adds that a repository whose visibility the session cannot read is treated as public, and cites the handoff spec for the rule;
   - read-back: the next pass no longer prints the issue as owed, and the dispatch queue, read uncapped as in Verify row 5, lists a direct repair;
   - failure handling from the scoping document §4.5, including that a session below strong tier never records a `needs-scoping` judgment;
   - every sentence of the retired skill passage that Task 3 does not keep in the skill: the replay stop condition, the read-back lines, the successor rules and the register view. Nothing in the passage is dropped; each sentence ends in the skill or in this reference.
3. In `plugins/assay/skills/intake-desk/SKILL.md`, replace the "Accountable handoff (version-negotiated)" passage with at most twelve lines: the rule in one sentence, the step's place in the judgment loop, a pointer to the reference, and the three control sentences named in the facts, kept in the skill with their wording unchanged (the scrub instruction, "not an assignment authority" and "Do not promote raw placeholders"). They are not moved to the reference: a session that never opens the reference must still read them. Add one sentence to item 1 of "The judgment half" saying the item is not routed until its handoff is recorded and published. The skill must not grow.
4. Regenerate the generated harness copy and digest with the repository's own generator; do not hand-edit them.
5. Add `TestHandoffProcedureExamples` (planned) in `tools/desk/cmd/scanloop/`: it reads every `expect:` line and the fenced record after it from the reference, decodes the record with `issueflow.DecodeIntakeHandoff`, assesses it with facts that match the record, and fails when state, route or the direct-repair result differ from the `expect:` line, or when the reference holds no example.
6. Add `TestHandoffExamplesFlagPlant` (planned): it runs the same reader over `testdata/handoff-procedure-plant.md` (planned), which holds one record marked `expect: observed repair direct` whose successor is a different issue, and passes only when the reader reports that record.
7. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows 1 and 2 name tests this brief delivers. Row 5 is the live half of the stream's critical-path head and is witnessed by someone who did not implement, on an installation at v1.0.62 or later.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffProcedureExamples$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestHandoffProcedureExamples ' "$o")` | exit 0 and the named PASS; every example in the reference decodes and assesses as its `expect:` line says |
| 2 | check:ci +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffExamplesFlagPlant$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestHandoffExamplesFlagPlant ' "$o")` | exit 0 and the named PASS; the planted record that claims a direct repair with a different successor is reported |
| 3 | check:ci | tree | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 710 && grep -c 'references/intake-handoff.md' plugins/assay/skills/intake-desk/SKILL.md` | exit 0; prints 1 or more; the skill did not grow and points at the reference |
| 4 | check:ci | tree | `(cd tools/skillslint && GOWORK=off go test -count=1 ./...)` | exit 0; output contains "ok" |
| 5 | check +flow | host:intake-installation | `statusgen --root . --next-up --span 100000 \| grep -c '"stream": *"issue-loop"'` | run in the root that holds the placeholder: prints 1, after one direct repair was recorded and published by the reference's steps; before that step the same command printed 0 on this installation. The read is uncapped by span because the default view's 20 rows can omit an eligible placeholder, and the filter is on the stream because a home-repository placeholder's id is `issue-loop/issue-<N>`. The witness also records the same output's `heldByStreamCap`: a printed 0 with `heldByStreamCap` at 0 is a fail, and a printed 0 with it above 0 is could-not-check |
| 6 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:01` | exit 0; output contains "0 disproved" |
| 7 | check:ci +neighbour | tree | `(k=$(tr '\n' ' ' < plugins/assay/skills/intake-desk/SKILL.md) && r=$(tr '\n' ' ' < plugins/assay/references/intake-handoff.md) && for p in 'scrub its free-text' 'not an assignment authority' 'Do not promote raw placeholders'; do case "$k" in *"$p"*) ;; *) echo "skill lost: $p"; exit 1;; esac; done && for p in 'scrub its free-text' 'spec/intake-handoff-v1.md'; do case "$r" in *"$p"*) ;; *) echo "reference lacks: $p"; exit 1;; esac; done)` | exit 0 and no output; the shortened skill still carries the scrub instruction and both guard sentences, and the reference states the scrub requirement and cites the spec. Fails when any is absent |

Failure modes mapped: a wrong-but-plausible example in the procedure → row 1; the example reader silently matching nothing → row 2 and row 1's no-example failure; the skill growing or losing its pointer → row 3; a stale generated copy → row 4; the writer and the board disagreeing on a real installation → row 5; the scrub instruction or a guard sentence lost when the passage is shortened, or the reference's publication step written without it → row 7. Whether the procedure reads clearly to a session is review-only.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
