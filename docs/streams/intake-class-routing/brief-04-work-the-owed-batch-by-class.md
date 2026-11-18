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
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here"
  - ".cursor/skills/intake-desk/SKILL.md: fixed-here"
  - ".cursor/assay-managed.json: fixed-here"
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

Non-implementer verification on merged main c5df2f7f09bb355c9fcd4aad94994a781112c191 (delivered by b3bbb256df36, first parent 340714fa6cfb). Gate: model; all four risk answers no. Tool: installed statusgen v1.0.87. Admission: `deskdispatch --check-verifier` exit 0 before the rows and again before this edit. All five rows are `check:ci` rows. The host is darwin and cannot sandbox them with `unshare --net`, so they ran in a closed container-netns admission bracket (`--network none`, read-only tree, read-only module cache, `GOPROXY=off`, `GOTOOLCHAIN=local`) on the release-pinned verify-runner v1.0.87 image. `--bracket-digest` reports `verdict=match`. The witness file's bytes follow verbatim.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassExamples$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassExamples ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassFlagsPlant$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassFlagsPlant ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 712 && grep -c 'references/intake-class-pass.md' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:4355a46b19d3 | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/skillslint && GOWORK=off go test -count=1 ./...)` | pass exit=0 sandbox=container-netns | sha256:232d6bddd06c | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --consumers --brief assay:assay:intake-class-routing:04` | fail exit=2 sandbox=container-netns | sha256:aa3bcf73e0c9 | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:7c7e183bae944b0f117d30fa67dcb4c314e684db1bf5f58d432e001ea5a208ec
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7059 run=971cd42382815abdc592f7555187b5b63c9d7399d1aaf5821584b9b41cbb3fdd source=c5df2f7f09bb355c9fcd4aad94994a781112c191 brief=docs/streams/intake-class-routing/brief-04-work-the-owed-batch-by-class.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=971cd42382815abdc592f7555187b5b63c9d7399d1aaf5821584b9b41cbb3fdd source=c5df2f7f09bb355c9fcd4aad94994a781112c191 tree=sha256:7c7e183bae944b0f117d30fa67dcb4c314e684db1bf5f58d432e001ea5a208ec witness=sha256:66aa345bd699d673ba46c31490c1c5b8d78cbab92a6c9fa85fb3eeb778c3a9c1 opened=2026-11-18T21:10:39Z closed=2026-11-18T21:12:13Z before=receipt after=check-verifier

Row 5 is could-not-check, not a disproved claim. Its witness cell reads `fail exit=2`; `statusgen --consumers` exit 2 means "the diff could not be taken", not exit 1 (a claim DISPROVED). In the container the tool printed `COULD-NOT-CHECK: git diff --name-only refs/remotes/origin/main...HEAD: exit status 128: fatal: gitfile does not point to a valid repository: /work/.git`. The admitted home is a linked worktree whose gitdir is not mounted in the bracket. A supplemental host probe of the same command (no acceptance credit) also exited 2: `COULD-NOT-CHECK: assay:assay:intake-class-routing:04 is not in the diff against c5df2f7f09bb355c9fcd4aad94994a781112c191, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery, and the history cannot be searched through the library read path (native discovery/config/shallow environment profile)`. On merged main HEAD equals origin/main, so the row as authored has no diff to read and no recorded delivery to fall back on. It needs either a committed delivery marker or an explicit delivery and base, which is a reviewed row-definition repair and needs fresh admission.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 5 | `statusgen --consumers --brief assay:assay:intake-class-routing:04` | exit 0; output contains "0 disproved" | could-not-run as admitted: exit 2, COULD-NOT-CHECK in the container (gitdir of the linked-worktree home not mounted) and on the host (empty diff at merged main; no committed Verification-Table marker; history read path unavailable). No claim corroborated or disproved by the row as written. | 2026-11-18 | assay-verifier-app[bot] @ c5df2f7f09bb (claude-opus-5-5) (on-behalf-of human:ian) |

Supplemental host probe, labelled diagnostic, no acceptance credit: `statusgen --consumers --brief assay:assay:intake-class-routing:04 --delivery b3bbb256df36b1bfeec8911040c079d7652fe72b --base 340714fa6cfbc3ddde81dc16df6fa84bc0b295a4` exited 0 with `summary: 3 corroborated, 0 disproved, 1 unchecked` (the three fixed-here consumers CORROBORATED; the author-brief out-of-scope entry UNCHECKED as a reviewer judgement).

Row notes. Rows 1 and 2 discriminate: the test reads every expect line and its record in the reference, decodes and assesses each with the shipped issueflow code, requires every outcome group, and the plant test plus nine named mutations each must be reported. Row 3: the skill was 710 lines at the delivery's base and 711 after it (the bound 712 is base plus two); it is 655 lines at this head after later edits and still carries the pointer.

Risk-bearing value. Risk metadata is present with all four answers no, the item is not irreversible, and the delivery touches only a skill, its generated copy and digest, a reference, a test, a test fixture and a changelog fragment. Enumeration over the delivered diff b3bbb256df36 and the Deliverables:
- Verify row 3 line bound = 712 @ brief line 93 — reversible (a doc-size guard; edit and re-run).
- design-fit weight "skill rule-text lines +2 at most" @ brief line 58 — reversible.
- minimum expect lines = 4 (`n < 4`) @ class_examples_test.go line 173 — reversible test floor; it matches the brief's four required outcomes.
- design-owed trigger = 3 counted instances / second merged fix @ intake-class-pass.md line 90 — restated, not changed (the brief forbids changing it); it is the shipped class decision's value, intake-desk SKILL.md line 353.

RISK-VALUE: DERIVED — design-owed trigger = 3 counted instances @ plugins/assay/references/intake-class-pass.md:90 — it restates the existing class decision (intake-desk SKILL.md:353) unchanged, as the brief's ground rules require; the reference introduces no new threshold. The other entries are reversible documentation and test bounds, ranked last and needing no derivation.

**VERIFY: BLOCKED** — rows 1-4 pass in the closed container bracket (verdict=match); row 5 could-not-run as admitted (consumers instrument COULD-NOT-CHECK at merged main); no obligation observed failed.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
