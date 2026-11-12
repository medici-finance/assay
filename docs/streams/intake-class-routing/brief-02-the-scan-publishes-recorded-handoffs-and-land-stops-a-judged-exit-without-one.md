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
  - "statusgen/principal.go: out-of-scope (the existing fail-closed public-form reader is called, not changed)"
  - "statusgen/nextup.go: out-of-scope (the admission rule is read, not changed; row 1 exercises it end to end)"
  - "plugins/assay/skills/intake-desk/SKILL.md: out-of-scope (the skill names the land command, whose flags do not change)"
---

# Brief 02 — The scan publishes recorded handoffs and land stops a judged exit without one

## Context

files: statusgen/scanissues.go, statusgen/main.go, statusgen/intakehandoffs.go, statusgen/intakehandoffs_test.go, tools/desk/cmd/scanloop/lane.go, tools/desk/cmd/scanloop/landverb.go, tools/desk/cmd/scanloop/handoffrecord.go, tools/desk/cmd/scanloop/handoff_test.go, changelog/intake-handoff-publish.md

facts:
- Read at `0665bfb77` (2026-11-12).
- `scanloop land --handoff` validates a judgment and appends it to the local `intake-handoffs-v1.jsonl` (`appendHandoffRecord`, `tools/desk/cmd/scanloop/handoffrecord.go:43`; the file name is the constant at `:19`). It writes nothing under the stream root.
- The board assesses a placeholder from two frontmatter keys, `handoff:` (a bare file name ending `.json`) and `handoff-opened:`, plus that sibling file (`statusgen/placeholder.go:364`, `statusgen/intakehandoffs.go:17`).
- The scanner owns the placeholder file: it renders it (`statusgen/scanissues.go:338`), strips both keys when an issue is reactivated (`:1026`) and moves the companion with the placeholder on archive (`:1167`).
- The drain already runs the scan in the carrier worktree: `statusgen --root . --scan-issues` (`tools/desk/cmd/scanloop/lane.go:269`).
- The tracked exits are `placeholder`, `bug`, `finding`, `needs-decision`, `rejected-watching` (`tools/desk/cmd/scanloop/land.go:41`). Without `--handoff`, `land` records any of them.
- An accountable `rejected-watching` exit and an item parked on a `needs-decision` issue discharge without a companion (`spec/intake-handoff-v1.md`, "Observation retention and scheduling").
- `statusgen` already decides whether a repository is public, and decides it fail-closed. It parses the roster's repository list with its visibility tokens (`scanEffectiveConfig`, `statusgen/rosterconfig.go:713`), and `onBehalfOfPublicForm` (`statusgen/principal.go:74`) answers public for everything except a repository the roster explicitly marks private: an unlisted repository, an entry with no visibility token and an empty name all answer public. Its comment requires it to stay in sync with the desk tools' reader. The scan uses this reader; a second input for the same decision would be a second path that can disagree with it.
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
2. **Publish typed references only.** A repository is treated as public unless the roster marks it private; one the roster does not list, or whose marking cannot be read, counts as public. In a public repository the scan publishes a judgment only when all three free-text fields are safe: the evidence must be a typed reference (an issue, pull request, brief or finding id), and the required action and the wake condition must be empty. A judgment that fails this is not published; the scan names the field and reports the judgment as unpublished, and the text stays local. In practice a judgment about work waiting on someone outside the pipeline is never published to a public repository, because its action and wake condition are free text. That holds back nothing a worker could start: such a judgment does not make an issue schedulable. Intake must write evidence as references in public repositories.

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
   - Apply the ruling from this brief's decision issue to the three free-text fields: `evidence`, `required_action` and `wake_condition`. Until it is ruled, implement option 2, and implement it fail-closed through the reader `statusgen` already has: the scan asks `onBehalfOfPublicForm` about the root's own repository, the one the companion is committed to, and adds no visibility flag, environment variable or second roster parse. That repository is the roster's home repository, which the scan already reads through `scanHomeRepo` (`statusgen/scanissues.go:35`). It is never the issue's source repository: one scan writes placeholders for every repository in its scan list into the one root, so a source repository marked private says nothing about where its companion is published. An unset home repository is the empty name, which the reader answers as public, so a scan run with no roster at all, as in CI, treats the root as public. Under option 2 a public root publishes a judgment only when `evidence` is a typed reference and `required_action` and `wake_condition` are both empty; anything else is named on stderr with its field and counted as unpublished.
3. **Supply.** Have the drain pass the local judgment log to the scan it already runs. An absent log is not an error: the scan runs as today. The drain passes nothing about visibility; the scan decides that itself, as Task 2 says.
4. **Refuse.** In `scanloop land`, a `--exit bug` or `--exit placeholder` recorded for an issue with no `--handoff` is refused with exit 5, and the message names `--handoff` and `--handoff-reader`. `finding`, `needs-decision` and `rejected-watching` are unchanged. A mechanical exit written by the drain is unchanged.
5. **Tests.**
   - `TestScanPublishesHandoff` (planned) in `statusgen`: over a fixture root, a recorded direct repair is published by the scan and the placeholder becomes eligible through `eligibleBase`; the same record with a different event time is not published and the placeholder stays held; a second scan with no change writes nothing.
   - `TestScanPublishKeepsRegister` (planned) in `statusgen`: the intake register's handoff export is byte-identical before and after a publishing scan.
   - `TestLandRefusesBareJudgedExit` (planned) in `tools/desk/cmd/scanloop`: the two refused exits return exit 5 and write no exit record; the three exempt exits and a `--handoff` call still succeed.
   - `TestScanPublishScrubBoundary` (planned) in `statusgen`, under the ruled option (option 2 until ruled), over three roots, each configured through the roster the existing reader parses: one marked public, one marked private and one the roster does not list. It covers a repair judgment whose evidence is free prose, a repair judgment whose evidence is a typed reference, and an external-prerequisite judgment with a typed-reference evidence and free-text `required_action` and `wake_condition`. In the public root and in the unmarked root only the typed-reference repair is published, and each of the other two is named with its field and counted as unpublished. In the private root all three are published. Two further cases pin which repository decides, each using an issue whose source repository is marked differently from the root: a public root that scans an issue from a repository marked private publishes only the typed-reference repair, and a private root that scans an issue from a repository marked public publishes all three.
6. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

Every row names a test this brief delivers. Fail-first: before the change, record rows 1 and 3 red.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishesHandoff$' . > "$o" && grep -q -- '--- PASS: TestScanPublishesHandoff ' "$o")` | exit 0 and the named PASS; a recorded direct repair reaches the board through the scan, and one with the wrong event time does not |
| 2 | check:ci +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLandRefusesBareJudgedExit$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestLandRefusesBareJudgedExit ' "$o")` | exit 0 and the named PASS; the two judged exits are refused with no record written, and the exempt exits still land |
| 3 | check:ci +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishScrubBoundary$' . > "$o" && grep -q -- '--- PASS: TestScanPublishScrubBoundary ' "$o")` | exit 0 and the named PASS; in a public root and in an unmarked root, free-prose evidence and an external-prerequisite judgment's free-text action and wake condition are not published and are named; in a private root they are. The root's own marking decides both ways when the issue's source repository is marked differently |
| 4 | check:ci +neighbour | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishKeepsRegister$' . > "$o" && grep -q -- '--- PASS: TestScanPublishKeepsRegister ' "$o")` | exit 0 and the named PASS; the register lane's export is unchanged by a publishing scan |
| 5 | check:ci +dereference | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$o" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$o")` | exit 0 and the named PASS; the existing admission behaviour still holds |
| 6 | check:ci | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$o")` | exit 0 and the named PASS; the existing writer behaviour still holds |
| 7 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:02` | exit 0; output contains "0 disproved" |

Failure modes mapped: publication the board cannot read → row 1; a stale judgment published against a newer event → row 1's negative case; the refusal not firing, or firing on an exempt exit → row 2; free text reaching a public repository against the ruling, through any of the three fields, through a root the roster does not list, or through a decision keyed on the issue's source repository instead of the root's → row 3; the register lane changed by accident → row 4; regressions in the shipped path → rows 5 and 6. Whether the refusal message reads clearly is review-only.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). Reviewer records verdict + date in the stream README table. Human gate is MANDATORY.
