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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassExamples$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassExamples ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 62a6169dc364 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestClassPassFlagsPlant$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestClassPassFlagsPlant ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 62a6169dc364 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 712 && grep -c 'references/intake-class-pass.md' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:4355a46b19d3 | 2026-11-19 | assay-verifier-app[bot] @ 62a6169dc364 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/skillslint && GOWORK=off go test -count=1 ./...)` | pass exit=0 sandbox=container-netns | sha256:7abc332221d6 | 2026-11-19 | assay-verifier-app[bot] @ 62a6169dc364 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:7cfc256c7b4590e7e5b1a9f54296a203f927f0f91b9585705d579af019d71098
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7097 run=8bfc32f3ee0929568c79311aacff5a19180f6ae0419b4d20ba5bf46a397f9f7b source=62a6169dc364b3d2968b0ab3144c70eac9743778 brief=docs/streams/intake-class-routing/brief-04-work-the-owed-batch-by-class.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=8bfc32f3ee0929568c79311aacff5a19180f6ae0419b4d20ba5bf46a397f9f7b source=62a6169dc364b3d2968b0ab3144c70eac9743778 tree=sha256:7cfc256c7b4590e7e5b1a9f54296a203f927f0f91b9585705d579af019d71098 witness=sha256:41d27e13f7e8bded21817c33c9cc4cf0ce0883fe65e712d84d6400353806400b opened=2026-11-19T00:56:17Z closed=2026-11-19T00:59:08Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `statusgen --consumers --brief assay:assay:intake-class-routing:04` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 62a6169dc364 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7097 run=8bfc32f3ee0929568c79311aacff5a19180f6ae0419b4d20ba5bf46a397f9f7b source=62a6169dc364b3d2968b0ab3144c70eac9743778 brief=docs/streams/intake-class-routing/brief-04-work-the-owed-batch-by-class.md model=claude-opus-5-5 tier=strong

Row notes (observer: assay-verifier-app[bot], claude-opus-5-5, strong tier, 2026-11-19). The first table is the closed container-netns witness for rows 1 to 4, bytes verbatim, from the release-pinned verify-runner v1.0.87 image (`--network none`, read-only root, read-only module cache, GOPROXY=off, GOTOOLCHAIN=local) at merged main 62a6169dc364b3d2968b0ab3144c70eac9743778. The second table is the host witness for row 5. The two selections are disjoint and cover all five rows. Delivery: merge 58af60249fb8 (PR #6308), whose first parent is b14e2ff73cdc; the Verify table is unchanged between that base and this source.

- Rows 1 and 2: pass. A supplemental probe (no acceptance credit, same image and flags) printed `--- PASS: TestClassPassExamples` and `--- PASS: TestClassPassFlagsPlant` with its nine plant subtests. A second probe (no acceptance credit) on a throwaway tracked snapshot outside the home dropped the trailing ` direct` from the first `expect: observed repair direct` line of the class-pass reference. The row 1 command then fails: `line 289: outcome below expects "observed repair direct", not "observed repair"`, `--- FAIL: TestClassPassExamples`. Row 1 discriminates.
- Row 3: pass. The output hash sha256:4355a46b19d3 is the digest of `1` plus a newline, so the skill holds one pointer to the reference. The probe read 655 lines. Stale bound: the skill had 710 lines at authoring (0665bfb77), so 712 is that count plus 2. It shrank to 654 before delivery, so the 712 cap no longer detects growth beyond two lines. Growth at delivery, observed independently: 654 to 655 (one line), within the brief's two-line limit.
- Row 4: pass. The probe printed `ok  github.com/medici-finance/assay/tools/skillslint`.
- Row 5: could-not-run, recorded twice by the instrument. On this darwin host, verifyrun refuses check:ci rows because `unshare --net` does not exist. Inside the container the command exits 2, because the linked worktree's git directory is not mounted: `COULD-NOT-CHECK: git diff --name-only refs/remotes/origin/main...HEAD: exit status 128: fatal: gitfile does not point to a valid repository`. That container attempt was superseded by the disjoint run above and is not landed. A supplemental host probe of the bare row command (no acceptance credit, no sandbox) exits 2. It reports COULD-NOT-CHECK because the post-merge diff is empty, the committed Evidence has no Verification-Table marker, and the history search refuses this home's profile. A further supplemental host probe pinned the delivery explicitly (`--delivery 58af60249fb8e9195adb01296626a75649ed4f91 --base b14e2ff73cdc2bc2d31c5c242f94ac2f899442ed`, no acceptance credit, no sandbox) and exits 0 with `summary: 3 corroborated, 0 disproved, 1 unchecked`. The three fixed-here consumers corroborate. The author-brief out-of-scope entry is unchecked by design. On a full clone, the row's own command can derive the delivery from history: commit b3bbb256df36, merged by 58af60249fb8, carries the `Brief: intake-class-routing/04` trailer.

Risk-bearing value. The frontmatter risk answers are all present and all `no`, the brief is not irreversible, and the delivery touches no risk-classed path, so the fail-first trigger does not fire. No row carries a risk-bearing, live, mutating or end-to-end tag. Enumeration over the b14e2ff73cdc..58af60249fb8 diff and the Deliverables: the eight outcome names and the expect-line map in the scanloop class_examples_test.go (test-only, lines 30 to 47); the placeholder records in the class-pass reference (example data); and the row 3 cap 712 in this brief's Verify table. None is an irreversible value. The reference reuses the class trigger (3 counted instances, a 2nd merged fix, or a brittle-module first) unchanged; the brief forbids changing it, and the diff does not change it.
RISK-VALUE: DERIVED — row 3 skill line cap = 712 @ brief-04-work-the-owed-batch-by-class.md Verify row 3 — it is the skill's 710 lines at authoring (0665bfb77) plus the design-fit allowance of +2 rule-text lines. It is stale, because the skill was 654 lines at the delivery base. The +2 property was checked separately (654 to 655).

**VERIFY: BLOCKED** — (blocker: medici-finance/assay#6099, open: a bare `statusgen --consumers` row cannot pass post-merge without a delivery pin) rows 1 to 4 pass (4 of 5 obligations witnessed). Row 5 could not run in either admitted lane: check:ci needs `unshare --net`, which darwin lacks, and the container lane has no git history for head commands. No acceptance obligation was observed to fail. Needs a Linux runner with `unshare --net` and full git history, or a container lane that exposes history to head commands.


## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
