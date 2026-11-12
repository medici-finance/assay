---
brief: assay:assay:intake-class-routing:02
title: The scan publishes recorded handoffs and land stops a judged exit without one
why: >-
  A recorded intake judgment lives only in a local log; nothing writes the companion file and
  the placeholder pointer the board reads, so publication is a hand edit nobody makes. And an
  intake session can still record a judged exit with no judgment behind it, which is how
  several hundred issues were labelled and left. This brief has the scan publish what was
  recorded and has the land verb refuse the exit that would strand an issue.
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The scan would copy a judgment's free-text fields (evidence, required action, wake
  condition) from a local log into a committed file, in a repository that may be public. The
  human confirms where the scrub boundary sits for that automated copy, and confirms the
  refusal ruled as D2 in the scoping document.
decision-trigger: start
issues: []
outcome: none
schema: brief-v2
version: 1
id: d3c50cab-89f6-4bdc-9079-fccb194019ee
authored: "2026-11-12 by a strong-tier authoring session (read-only review)"
exec-tier: strong
exec-tier-why: "(a) how the scan receives the recorded judgments is not fully pre-specified; (b) correctness spans the land verb, the scan and the board's reader."
domain: complicated
sources:
  - "docs/streams/intake-class-routing/README.md §1.1, §4.2, §4.5, §5, §9 D2, §10"
  - "spec/intake-handoff-v1.md — consumed as shipped, not changed"
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): no writer of the placeholder's handoff keys or of a companion file exists outside tests"
consumers:
  - "statusgen/scanissues.go: follow-up intake-class-routing/02 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/main.go: follow-up intake-class-routing/02 (this brief; the scan's new input flag)"
  - "tools/desk/cmd/scanloop/lane.go: follow-up intake-class-routing/02 (this brief; passes the recorded judgments to the scan it already runs)"
  - "tools/desk/cmd/scanloop/landverb.go: follow-up intake-class-routing/02 (this brief; the refusal)"
  - "statusgen/nextup.go: out-of-scope (the admission rule is read, not changed; row 1 exercises it end to end)"
  - "plugins/assay/skills/intake-desk/SKILL.md: out-of-scope (the skill names the land command, whose flags do not change)"
---

# Brief 02 — The scan publishes recorded handoffs and land stops a judged exit without one

## Context

files: statusgen/scanissues.go, statusgen/main.go, statusgen/intakehandoffs.go, statusgen/intakehandoffs_test.go, tools/desk/cmd/scanloop/lane.go, tools/desk/cmd/scanloop/landverb.go, tools/desk/cmd/scanloop/handoffrecord.go, tools/desk/cmd/scanloop/handoff_test.go, changelog/intake-handoff-publish.md

facts:
- Read at `0665bfb77` (2026-11-12).
- `scanloop land --handoff` validates a judgment and appends it to the local `intake-handoffs-v1.jsonl` (`tools/desk/cmd/scanloop/handoffrecord.go:19`, `appendHandoffRecord`). It writes nothing under the stream root.
- The board assesses a placeholder from two frontmatter keys, `handoff:` (a bare file name ending `.json`) and `handoff-opened:`, plus that sibling file (`statusgen/placeholder.go:364`, `statusgen/intakehandoffs.go:17`).
- The scanner owns the placeholder file: it renders it (`statusgen/scanissues.go:338`), strips both keys when an issue is reactivated (`:1026`) and moves the companion with the placeholder on archive (`:1167`).
- The drain already runs the scan in the carrier worktree: `statusgen --root . --scan-issues` (`tools/desk/cmd/scanloop/lane.go:269`).
- The tracked exits are `placeholder`, `bug`, `finding`, `needs-decision`, `rejected-watching` (`tools/desk/cmd/scanloop/land.go:41`). Without `--handoff`, `land` records any of them.
- An accountable `rejected-watching` exit and an item parked on a `needs-decision` issue discharge without a companion (`spec/intake-handoff-v1.md`, "Observation retention and scheduling").
- Existing end-to-end tests: `TestIntakeHandoffDisposition` (`tools/desk/cmd/scanloop/handoff_test.go`) and `TestScanHandoffAdmission` (`statusgen/intakehandoffs_test.go`).
- Class of defect: a judgment that is recorded but never reaches the reader that schedules on it. Sites it can recur at: the issue lane's placeholders and the intake register's entries. This brief covers the issue lane; row 4 guards the register lane's existing behaviour.
- Single control the current design leans on: a session hand-editing two keys and a file. The layers here are independent: the scan publishes (statusgen), and the land verb refuses (scanloop). Either can fail without disabling the other.

layering: no new component. The placeholder file keeps one writer, the scanner; scanloop supplies recorded judgments to the scan it already dispatches and never edits a placeholder itself. Assessment stays the shared `deskcore/issueflow` call. No new process, store or verb.

design-fit:
  owner: statusgen scanner (placeholder file and companion); tools/desk/cmd/scanloop (the intake exit and judgment records)
  contract: none — intake-handoff-v1 is consumed as shipped; no registered meaning changes
  retires: ["hand publication of handoff companions and placeholder pointers"]
  weight: "verbs 0, flags +1 (the scan's judgments input), refusals +1 (judged exit with no handoff), rule-text lines 0"
  why-add: "Publication has no owner today, and the scanner is the placeholder's only writer, so the input belongs on the scan. The refusal replaces a prose rule that was not followed. Removal considered: dropping the admission requirement, which the scoping document §5 rejects."

## Human decision

An intake judgment carries three free-text fields: the evidence for the judgment, and for work waiting on someone outside the pipeline, the required action and the condition that wakes it. Today a person copies a judgment into the repository by hand and is told to scrub those fields first. This change makes the scan copy them automatically into a committed file. In a public repository that file is public. The board refuses to schedule on a judgment whose evidence field is empty, so the field cannot simply be dropped.

Separately, the change makes the tool refuse to record that an issue was judged a bug or given a placeholder when no judgment record accompanies it. That refusal was recommended when this work was planned and was ruled when the plan was merged, unless a different ruling was recorded then. It is stated here so the reader knows it ships in the same change; it is not the question being asked.

Options:
1. **Publish the fields as recorded.** The copy rides the same reviewed intake change as today, so the repository's existing outbound checks and the reviewer see it before merge. Nothing new is added. A careless evidence line can reach a draft pull request before anyone reads it.
2. **Publish typed references only.** In a repository the roster marks public, the scan refuses to publish a judgment whose evidence is not a typed reference (an issue, pull request, brief or finding id), and names the offending field. Free text stays local. Intake must write evidence as references in public repositories.

Default if no answer: option 2 after 7 days from the decision issue's filing. It is the narrower behaviour and can be widened later; option 1 cannot be undone for text already pushed.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not edit `spec/intake-handoff-v1.md`, `spec/intake-exit-v1.md` or `statusgen/nextup.go`.
- The scanner stays the only writer of a placeholder file.

## Task

1. Re-check the facts. If a writer of the placeholder's handoff keys already exists, report NEEDS_CONTEXT.
2. **Publish.** Give `statusgen --scan-issues` an input naming a file of recorded judgments. For each placeholder the scan writes or keeps, take the latest revision recorded for that issue's latest source event, write it beside the placeholder as `<placeholder stem>.handoff.json`, and set `handoff:` and `handoff-opened:`. Rules:
   - Publish only a judgment that decodes strictly and whose item and event time match the placeholder's issue. Anything else is left unpublished and named on stderr; the scan still exits 0 for the placeholders it could write and reports the count it skipped.
   - A later revision replaces the companion. An unchanged one produces no diff.
   - Reactivation and archive behaviour stay exactly as they are.
   - Apply the ruling from this brief's decision issue to the three free-text fields. Until it is ruled, implement option 2.
3. **Supply.** Have the drain pass the local judgment log to the scan it already runs. An absent log is not an error: the scan runs as today.
4. **Refuse.** In `scanloop land`, a `--exit bug` or `--exit placeholder` recorded for an issue with no `--handoff` is refused with exit 5, and the message names `--handoff` and `--handoff-reader`. `finding`, `needs-decision` and `rejected-watching` are unchanged. A mechanical exit written by the drain is unchanged.
5. **Tests.**
   - `TestScanPublishesHandoff` (planned) in `statusgen`: over a fixture root, a recorded direct repair is published by the scan and the placeholder becomes eligible through `eligibleBase`; the same record with a different event time is not published and the placeholder stays held; a second scan with no change writes nothing.
   - `TestScanPublishKeepsRegister` (planned) in `statusgen`: the intake register's handoff export is byte-identical before and after a publishing scan.
   - `TestLandRefusesBareJudgedExit` (planned) in `tools/desk/cmd/scanloop`: the two refused exits return exit 5 and write no exit record; the three exempt exits and a `--handoff` call still succeed.
   - `TestScanPublishScrubBoundary` (planned) in `statusgen`: under the ruled option, a judgment whose evidence is free prose is handled as that option says, in a root marked public and in one that is not.
6. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

Every row names a test this brief delivers. Fail-first: before the change, record rows 1 and 3 red.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishesHandoff$' . > "$o" && grep -q -- '--- PASS: TestScanPublishesHandoff ' "$o")` | exit 0 and the named PASS; a recorded direct repair reaches the board through the scan, and one with the wrong event time does not |
| 2 | check:ci +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLandRefusesBareJudgedExit$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestLandRefusesBareJudgedExit ' "$o")` | exit 0 and the named PASS; the two judged exits are refused with no record written, and the exempt exits still land |
| 3 | check:ci +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishScrubBoundary$' . > "$o" && grep -q -- '--- PASS: TestScanPublishScrubBoundary ' "$o")` | exit 0 and the named PASS; free-prose evidence is handled as the ruled option says in a public root and in a private one |
| 4 | check:ci +neighbour | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishKeepsRegister$' . > "$o" && grep -q -- '--- PASS: TestScanPublishKeepsRegister ' "$o")` | exit 0 and the named PASS; the register lane's export is unchanged by a publishing scan |
| 5 | check:ci +dereference | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$o" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$o")` | exit 0 and the named PASS; the existing admission behaviour still holds |
| 6 | check:ci | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$o")` | exit 0 and the named PASS; the existing writer behaviour still holds |
| 7 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:02` | exit 0; output contains "0 disproved" |

Failure modes mapped: publication the board cannot read → row 1; a stale judgment published against a newer event → row 1's negative case; the refusal not firing, or firing on an exempt exit → row 2; free text reaching a public repository against the ruling → row 3; the register lane changed by accident → row 4; regressions in the shipped path → rows 5 and 6. Whether the refusal message reads clearly is review-only.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). Reviewer records verdict + date in the stream README table. Human gate is MANDATORY.
