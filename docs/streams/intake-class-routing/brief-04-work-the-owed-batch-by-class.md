---
brief: assay:assay:intake-class-routing:04
title: Work the owed batch by class
why: >-
  Intake judges issues one symptom at a time, so the same mechanism is filed, labelled and
  fixed again and again while the tracker grows. Each pass already hands intake the oldest
  owed issues as a batch. Grouping that batch by mechanism, matching each group to work that
  already exists, and writing one class brief where none does, turns many stranded symptoms
  into one unit a worker can pick up and close together.
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
id: 2fa82b92-eb57-45ef-b2b8-4c404efc89e5
authored: "2026-11-12 by a strong-tier authoring session (read-only review)"
exec-tier: strong
exec-tier-why: "(a) the grouping and matching procedure is a design the facts do not pre-specify; (b) its example records must agree with the shipped assessment in another module."
domain: complicated
sources:
  - "docs/streams/intake-class-routing/README.md §4.3, §4.4, §4.5, §4.6, §9 D3 and D4, §10"
  - "docs/streams/build-less-brittle/brief-04-intake-by-error-class.md — the class record, counting and trigger, reused unchanged"
  - "spec/intake-handoff-v1.md — consumed as shipped, not changed"
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): the intake skill's class decision runs per new issue only; no procedure groups a batch"
consumers:
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up intake-class-routing/04 (this brief; flips to fixed-here when the implementation edits the path)"
  - ".cursor/skills/intake-desk/SKILL.md: follow-up intake-class-routing/04 (this brief; generated harness copy of the canonical skill)"
  - ".cursor/assay-managed.json: follow-up intake-class-routing/04 (this brief; generated harness content digest)"
  - "plugins/assay/skills/author-brief/SKILL.md: out-of-scope (class briefs use its existing rules 9d and 14 and design-fit; nothing is added there)"
---

# Brief 04 — Work the owed batch by class

## Context

files: plugins/assay/references/intake-class-pass.md, plugins/assay/skills/intake-desk/SKILL.md, .cursor/skills/intake-desk/SKILL.md, .cursor/assay-managed.json, tools/desk/cmd/scanloop/class_examples_test.go, tools/desk/cmd/scanloop/testdata/class-pass-plant.md, changelog/intake-class-pass.md

facts:
- Read at `0665bfb77` (2026-11-12).
- Each intake pass presents retained, unjudged issues oldest first, up to a per-repository bound, as `intake:` lines under `EMITTED` (`tools/desk/cmd/scanloop/observations.go`, `run.go:419`). That set is "the owed batch".
- The intake skill's class decision (item 1 of "The judgment half") already defines: the `error-class` issue titled `class: <mechanism>`, the instance block and its fields, counting (only `confirmed-defect` and `false-positive`, one per `incident-group`), and the `design-owed` trigger (3 counted instances, or the class's 2nd merged fix, or the first in a `brittle`-marked module). It is applied to one new issue at a time.
- Read 2026-11-12 on this repository's tracker: 11 `error-class` issues in total, none opened after 2026-10-24, 2 `design-owed`.
- The handoff record's `duplicate` disposition takes route `duplicate` and a successor that is a brief or an issue; `needs-scoping` takes route `authoring`, `author_tier: strong` and a successor brief declaring strong execution, and the writer refuses it from a session below strong tier (`deskcore/issueflow/intake.go`, `tools/desk/cmd/scanloop/handoffrecord.go:100`).
- A brief closes its member issues through its `issues:` list (`spec/brief-v1.md`); a class brief is authored with `author-brief`, whose rule 9(d) makes it strong-tier and whose rule 14 requires a class-guard row.
- Shared references live in `plugins/assay/references/` and carry an `assay:harnesslint non-matrix-reference` comment.
- Class of defect this closes: symptoms of one mechanism routed as unrelated items. The guard is the example set, executed by a test, covering all four outcomes.
- Single control this design leans on: the session's judgment when grouping. Review of the class brief's pull request is the independent second layer; a wrong grouping is visible there as a brief whose members do not share its stated mechanism.

design-fit:
  owner: plugins/assay/skills/intake-desk (the class decision)
  contract: none — reuses the class record of build-less-brittle/04 and intake-handoff-v1 unchanged
  retires: []
  weight: "verbs 0, flags 0, refusals 0, skill rule-text lines +2 at most, one new reference file"
  why-add: "The existing class decision fires only on a new arrival, so the backlog is never grouped. The procedure cannot live in the skill body without growing it. Removal considered: lowering the trigger instead, which the scoping document D3 rejects as unnecessary."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not change the class trigger, the instance block's fields, the duplicate rule (evidence merged first; a strong worker marks; an independent pass closes) or the trust gate.
- Do not edit `spec/intake-handoff-v1.md` or the `author-brief` skill.

## Task

1. Re-check the facts. If a batch grouping procedure already exists, report NEEDS_CONTEXT.
2. Write `plugins/assay/references/intake-class-pass.md` (planned). It states, in this order:
   - **When.** Before judging the owed batch item by item, on a strong-tier session. A session below strong tier does not group; it records only direct repairs, duplicates of existing work it can cite, and external waits, and leaves the rest owed.
   - **Group.** By mechanism, using the owning module, the labels and the open `error-class` issues. Quarantined or unblessed text is never read into a group. A single unrelated item goes straight to its own judgment.
   - **Step 1, match existing work.** Where to look and in what order: open class issues; briefs and streams whose title, `why:` or `files:` name the mechanism or module; approved or routed specs; and, where the installed tools support it, the context discovery the implementation pickup reference describes. A match means each member is recorded `duplicate` to that owner, and the owner's author is asked on the owner's own tracker item to add the member numbers to its `issues:`. Intake does not edit another brief.
   - **Step 2, apply the trigger.** Count the group exactly as the class decision counts. State plainly that a backlog group meeting the threshold is `design-owed` on this pass.
   - **Step 3, author the class brief.** With `author-brief`, in the same reviewed change as the members' judgments. The brief states the mechanism in one line, the owning module, every member in `issues:`, every site the mechanism can recur at, the existing or planned work it was matched against and the result, what it retires, and a class-guard Verify row that fails on a planted second instance. Members are recorded `duplicate` to the brief.
   - **Step 4, release the rest.** Below the trigger: open or update the class issue with one instance block per member, and record each member as its own direct repair.
   - **What grouping never does.** Merge evidence, mark a duplicate on the tracker, close an issue, dispatch a worker or take a claim.
   - **Partial results.** A class brief without its members' judgments is incomplete and the members stay owed; an unreadable write is could-not-check.
   - **Examples.** One worked batch covering all four outcomes: a group matched to an existing brief, a group that meets the trigger, a pair below it, and one quarantined item. For every record the batch produces, a fenced `json` block preceded by a line `expect: <state> <route>` with a trailing ` direct` for a direct repair; for the quarantined item, a line `expect: none`. Placeholder values only.
3. In `plugins/assay/skills/intake-desk/SKILL.md`, add at most two lines to the class decision in item 1 of "The judgment half": the owed batch is grouped before it is judged, with a pointer to the reference. Touch no other section.
4. Regenerate the generated harness copy and digest with the repository's own generator.
5. Add `TestClassPassExamples` (planned) in `tools/desk/cmd/scanloop/`: it reads every `expect:` line and the record after it, decodes and assesses each record as the shipped code does, and fails when any differs from its `expect:` line, when a `duplicate` example's successor is not a brief or issue reference, when fewer than the four outcomes are present, or when the quarantined item has a record.
6. Add `TestClassPassFlagsPlant` (planned): it runs the same reader over `testdata/class-pass-plant.md` (planned), which holds a quarantined item that was given a `repair` record, and passes only when the reader reports it.
7. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassExamples$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassExamples ' "$o")` | exit 0 and the named PASS; the worked batch's records decode and assess as stated, and all four outcomes are present |
| 2 | check:ci +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassFlagsPlant$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassFlagsPlant ' "$o")` | exit 0 and the named PASS; a quarantined item that was routed is reported |
| 3 | check:ci | tree | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 712 && grep -c 'references/intake-class-pass.md' plugins/assay/skills/intake-desk/SKILL.md` | exit 0; prints 1 or more; the skill grew by two lines at most and points at the reference |
| 4 | check:ci | tree | `(cd tools/skillslint && GOWORK=off go test -count=1 ./...)` | exit 0; output contains "ok" |
| 5 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:04` | exit 0; output contains "0 disproved" |

Failure modes mapped: an example record the shipped code would not accept → row 1; an outcome missing from the procedure → row 1; quarantined text routed → row 2; the skill growing or losing its pointer → row 3; a stale generated copy → row 4. Whether a real session groups well is not checkable here: it is judged at review of each class brief it produces, and measured by milestone m-02 in the scoping document.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
