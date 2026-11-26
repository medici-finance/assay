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
decision-issue: 6364
ruling: "https://github.com/medici-finance/assay/issues/6364#issuecomment-6929236569"
design: DR-handoff-publish
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
- The repository a scan's delta is committed to is the drain's scan target (`ScanTarget`, `tools/desk/cmd/scanloop/adapter.go:30`), resolved once from `--scan-target` or the root checkout's own origin and failing closed when neither gives one (`resolveScanTarget`, `tools/desk/cmd/scanloop/plan.go:100`). It is not the roster's home repository: nothing in `scanloop` compares the two (`grep -n -e HomeRepo -e ASSAY_HOME_REPO tools/desk/cmd/scanloop/*.go` prints nothing), and one roster can serve several roots. The scanner is run as `statusgen --root . --scan-issues` and is not told the target today (`lane.go:269`).
- The scan target is a declared name; the bytes are pushed somewhere else when the two disagree. The carrier commits in a worktree and publishes through `deskpr`, which pushes to the one repository that worktree's origin names (the push-destination rule, `tools/desk/cmd/deskpr/pushdest.go:15`). An explicit `--scan-target` is accepted on its shape alone and never compared with the origin (`plan.go:101`). The two are the same repository only when the flag is not passed.
- Existing end-to-end tests: `TestIntakeHandoffDisposition` (`tools/desk/cmd/scanloop/handoff_test.go`) and `TestScanHandoffAdmission` (`statusgen/intakehandoffs_test.go`).
- Class of defect: a judgment that is recorded but never reaches the reader that schedules on it. Sites it can recur at: the issue lane's placeholders and the intake register's entries. This brief covers the issue lane; row 4 guards the register lane's existing behaviour.
- Single control the current design leans on: a session hand-editing two keys and a file. The layers here are independent: the scan publishes (statusgen), and the land verb refuses (scanloop). Either can fail without disabling the other.

layering: no new component. The placeholder file keeps one writer, the scanner; scanloop supplies recorded judgments to the scan it already dispatches and never edits a placeholder itself. Assessment stays the shared `deskcore/issueflow` call. No new process, store or verb.

design-fit:
  owner: statusgen scanner (placeholder file and companion); tools/desk/cmd/scanloop (the intake exit and judgment records)
  contract: none — intake-handoff-v1 is consumed as shipped; no registered meaning changes
  retires: ["hand publication of handoff companions and placeholder pointers"]
  weight: "verbs 0, flags +2 (the scan's judgments input and its commit target), refusals +1 (judged exit with no handoff), rule-text lines 0"
  why-add: "Publication has no owner today, and the scanner is the placeholder's only writer, so the input belongs on the scan. The refusal replaces a prose rule that was not followed. Removal considered: dropping the admission requirement, which the scoping document §5 rejects."

## Human decision

An intake judgment carries three free-text fields: the evidence for the judgment, and for work waiting on someone outside the pipeline, the required action and the condition that wakes it. Today a person copies a judgment into the repository by hand and is told to scrub those fields first. This change makes the scan copy them automatically into a committed file. In a public repository that file is public. The board refuses to schedule on a judgment whose evidence field is empty, so the field cannot simply be dropped.

Separately, the change makes the tool refuse to record that an issue was judged a bug or given a placeholder when no judgment record accompanies it. That refusal was recommended when this work was planned and was ruled when the plan was merged, unless a different ruling was recorded then. It is stated here so the reader knows it ships in the same change; it is not the question being asked.

Options:
1. **Publish the fields as recorded.** The copy rides the same reviewed intake change as today, so the repository's existing outbound checks and the reviewer see it before merge. Nothing new is added. A careless evidence line can reach a draft pull request before anyone reads it.
2. **Publish typed references only.** A repository is treated as public unless the roster marks it private; one the roster does not list, or whose marking cannot be read, counts as public. In a public repository the scan publishes a judgment only when all three free-text fields are safe: the evidence must be a typed reference (an issue, pull request, brief or finding id), and the required action and the wake condition must be empty. A judgment that fails this is not published; the scan names the field and reports the judgment as unpublished, and the text stays local. In practice a judgment about work waiting on someone outside the pipeline is never published to a public repository, because its action and wake condition are free text. That holds back nothing a worker could start: such a judgment does not make an issue schedulable. Intake must write evidence as references in public repositories.

Default if no answer: option 2 after 2026-11-20 (7 days from the decision issue's filing on 2026-11-13). It is the narrower behaviour and can be widened later; option 1 cannot be undone for text already pushed.

Ruled: option 2 (option A on the decision issue), recorded by the `ruling:` link in this brief's frontmatter, which points at the ruling comment that names `intake-class-routing/02`.

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
   - Apply the ruling from this brief's decision issue to the three free-text fields: `evidence`, `required_action` and `wake_condition`. Until it is ruled, implement option 2, and implement it fail-closed through the reader `statusgen` already has: the scan asks `onBehalfOfPublicForm` about the root's own repository, the one the companion is committed to, and adds no visibility flag, environment variable or second roster parse. That repository is the drain's scan target, which the drain supplies with the judgment log (Task 3); the scan passes that name, unchanged, to the reader. It is never the issue's source repository: one scan writes placeholders for every repository in its scan list into the one root, so a source repository marked private says nothing about where its companion is published. It is not the roster's home repository either: the two can differ, and a home repository marked private must not make a public commit target private. A scan given no commit target asks the reader about the empty name, which answers public, so a scan run by hand or in CI, or with no roster at all, publishes no free text. The supplied name is not trusted on its own, because the push goes to the root checkout's origin: the scan resolves the repository that origin's push destination names (`git remote get-url --push origin` in the root, the same read `deskpr` makes) and treats the root as public unless that repository is the supplied target. An origin that cannot be read, does not parse as a repository, or names a different repository answers public. So free text is published only when three things hold together: a target was supplied, the origin names the same repository, and the roster marks it private. Under option 2 a public root publishes a judgment only when `evidence` is a typed reference and `required_action` and `wake_condition` are both empty; anything else is named on stderr with its field and counted as unpublished.
3. **Supply.** Have the drain pass two things to the scan it already runs: the local judgment log, and the scan target it resolved and will commit to. An absent log is not an error: the scan runs as today. The drain passes the target's name only, never a visibility value; the scan decides visibility itself through the one reader, as Task 2 says.
4. **Refuse.** In `scanloop land`, a `--exit bug` or `--exit placeholder` recorded for an issue with no `--handoff` is refused with exit 5, and the message names `--handoff` and `--handoff-reader`. `finding`, `needs-decision` and `rejected-watching` are unchanged. A mechanical exit written by the drain is unchanged.
5. **Tests.**
   - `TestScanPublishesHandoff` (planned) in `statusgen`: over a fixture root, a recorded direct repair is published by the scan and the placeholder becomes eligible through `eligibleBase`; the same record with a different event time is not published and the placeholder stays held; a second scan with no change writes nothing.
   - `TestScanPublishKeepsRegister` (planned) in `statusgen`: the intake register's handoff export is byte-identical before and after a publishing scan.
   - `TestLandRefusesBareJudgedExit` (planned) in `tools/desk/cmd/scanloop`: the two refused exits return exit 5 and write no exit record; the three exempt exits and a `--handoff` call still succeed.
   - `TestScanPublishScrubBoundary` (planned) in `statusgen`, under the ruled option (option 2 until ruled), over three roots, each identified by the commit target supplied to the scan and configured through the roster the existing reader parses: one marked public, one marked private and one the roster does not list. It covers a repair judgment whose evidence is free prose, a repair judgment whose evidence is a typed reference, and an external-prerequisite judgment with a typed-reference evidence and free-text `required_action` and `wake_condition`. In the public root and in the unmarked root only the typed-reference repair is published, and each of the other two is named with its field and counted as unpublished. In the private root all three are published. Two further cases pin which repository decides, each using an issue whose source repository is marked differently from the root: a public root that scans an issue from a repository marked private publishes only the typed-reference repair, and a private root that scans an issue from a repository marked public publishes all three. Two more pin the commit target against the home repository: with the roster's home repository marked private and a supplied commit target that is public or not listed, free text is not published; and with no commit target supplied at all, free text is not published whatever the roster says. One more pins the declared target against the push destination: with a supplied target the roster marks private and a root whose origin names a different repository, public or not listed, free text is not published; the same holds when the origin cannot be read.
6. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

Every row names a test this brief delivers. Fail-first: before the change, record rows 1 and 3 red.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishesHandoff$' . > "$o" && grep -q -- '--- PASS: TestScanPublishesHandoff ' "$o")` | exit 0 and the named PASS; a recorded direct repair reaches the board through the scan, and one with the wrong event time does not |
| 2 | check:ci +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLandRefusesBareJudgedExit$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestLandRefusesBareJudgedExit ' "$o")` | exit 0 and the named PASS; the two judged exits are refused with no record written, and the exempt exits still land |
| 3 | check:ci +mutation | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishScrubBoundary$' . > "$o" && grep -q -- '--- PASS: TestScanPublishScrubBoundary ' "$o")` | exit 0 and the named PASS; in a public root and in an unmarked root, free-prose evidence and an external-prerequisite judgment's free-text action and wake condition are not published and are named; in a private root they are. The commit target's marking decides both ways when the issue's source repository is marked differently, a home repository marked private does not make another commit target private, a scan given no commit target publishes no free text, and a supplied target that the root's origin does not name publishes none either |
| 4 | check:ci +neighbour | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishKeepsRegister$' . > "$o" && grep -q -- '--- PASS: TestScanPublishKeepsRegister ' "$o")` | exit 0 and the named PASS; the register lane's export is unchanged by a publishing scan |
| 5 | check:ci +dereference | tree | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$o" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$o")` | exit 0 and the named PASS; the existing admission behaviour still holds |
| 6 | check:ci | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$o")` | exit 0 and the named PASS; the existing writer behaviour still holds |
| 7 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:02` | exit 0; output contains "0 disproved" |

Failure modes mapped: publication the board cannot read → row 1; a stale judgment published against a newer event → row 1's negative case; the refusal not firing, or firing on an exempt exit → row 2; free text reaching a public repository against the ruling, through any of the three fields, through a root the roster does not list, through a decision keyed on the issue's source repository or on the roster's home repository instead of the commit target, through a scan that was given no commit target, or through a declared target that differs from the repository the root's origin pushes to → row 3; the register lane changed by accident → row 4; regressions in the shipped path → rows 5 and 6. Whether the refusal message reads clearly is review-only.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Verification 2026-11-26 at a5f18d4b0c34

Post-merge verification at merged main a5f18d4b0c34 under desk tools v1.0.94 (claude-opus-5-5, strong tier), by a non-implementer verifier. The home was detached at a5f18d4b0c34 and admitted by deskdispatch --check-verifier (attestation medici-finance/assay#7802). The delivery is medici-finance/assay#6362, merge 1d5ed867e4b6 with first parent 4c4ac6e5d650; no later commit touches its delivered files, and the delivery left this Verify table unchanged. deskpathguard rederive reads 7 rows, all from-base at a5f18d4b0c34, 0 author-added. All 7 rows are check:ci and the host is darwin arm64 (go1.27.4, git 2.56.1), so they ran in one closed container-netns admission bracket: open on the host 2026-11-26T02:30:00Z to 02:30:28Z with a new module cache outside the home, the verify-runner v1.0.94 image by digest in a network-none container 02:30:32Z to 02:31:01Z, close on the host at 02:31:11Z from the same working directory. statusgen verifyrun --bracket-digest reports verdict=match. Declared verifier Git environment, KUBECONFIG=/dev/null, no forge credential in the run. The closed witness bytes follow unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishesHandoff$' . > "$o" && grep -q -- '--- PASS: TestScanPublishesHandoff ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLandRefusesBareJudgedExit$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestLandRefusesBareJudgedExit ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishScrubBoundary$' . > "$o" && grep -q -- '--- PASS: TestScanPublishScrubBoundary ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanPublishKeepsRegister$' . > "$o" && grep -q -- '--- PASS: TestScanPublishKeepsRegister ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$o" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --consumers --brief assay:assay:intake-class-routing:02` | pass exit=0 sandbox=container-netns | sha256:7165349da2a5 | 2026-11-26 | assay-verifier-app[bot] @ a5f18d4b0c34 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:670ddbcc2db0bd2607b190bd8c59b32d29ea87f948bd99a3b2790caf72828530
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7802 run=d305b3c9f42b6d48a34f7718e6c68f2204235c1689a48ec622355a59a9d38ae1 source=a5f18d4b0c34b0f8f307d52cdcd47d8b2f510f69 brief=docs/streams/intake-class-routing/brief-02-the-scan-publishes-recorded-handoffs-and-land-stops-a-judged-exit-without-one.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d305b3c9f42b6d48a34f7718e6c68f2204235c1689a48ec622355a59a9d38ae1 source=a5f18d4b0c34b0f8f307d52cdcd47d8b2f510f69 tree=sha256:670ddbcc2db0bd2607b190bd8c59b32d29ea87f948bd99a3b2790caf72828530 witness=sha256:6cd5791947b6e87d6c06a49c623ebb013f6ddedc7d322cdef862f9e2e7fa1bb7 opened=2026-11-26T02:30:28Z closed=2026-11-26T02:31:11Z before=receipt after=check-verifier

Row grading. Rows 2 to 7 exit 0 in the witness and meet their Expect: each test row's named PASS line is asserted by its own command, and row 7's Expect (output contains "0 disproved") is read in S7 below. Row 1 also exits 0 with its named PASS, but its Expect has two clauses: a recorded direct repair reaches the board through the scan, and one with the wrong event time does not. The delivering pull request's Desk-decided block, item 1, records that the scan publishes the latest recorded event for the issue and does not detect a stale record, because the scan's issue wire carries no event time to match against; the alternative it names is extending the issue wire with an event time, at the cost of one follow-up brief. The decision record docs/streams/decisions/DR-handoff-publish.md says the same. What the delivered test checks for that clause is that an older recorded event does not replace a newer published one (S1 shows the check discriminates). Whether that substitute discharges the event-time clause can only be settled by reading the Desk-decided block, so row 1 carries no acceptance credit and is recorded as could-not-run (check-definition): its grading needs either a reviewed check-definition repair of row 1 or the follow-up that adds an event time to the issue wire. No open issue covering this was found on the tracker (searched 2026-11-26); one is owed.

Supplemental host observations, no acceptance credit; each ran outside the witness, with output kept outside the home and the same Git environment. S1 to S4 ran on a git-archive snapshot of a5f18d4b0c34 extracted outside the home; the unmodified snapshot passes each named test first.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| S1 | TestScanPublishesHandoff on the snapshot with the older-event refusal disabled (statusgen/scanpublish.go line 514 guarded by false) | caught | FAIL; the older-event subtest reports Published:1 where the older record must be refused | 2026-11-26 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S2 | TestLandRefusesBareJudgedExit on the snapshot with the placeholder exit dropped from the land refusal (tools/desk/cmd/scanloop/landverb.go line 90) | caught | FAIL; exit = 0, want 5 (refused), and 4 record lines where 3 are expected | 2026-11-26 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S3 | TestScanPublishScrubBoundary on the snapshot with the origin-matches-target check removed (statusgen/scanpublish.go line 318) | caught | FAIL; external and prose fields published where published=false is expected | 2026-11-26 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S4 | TestScanPublishScrubBoundary on the snapshot with the roster marking read replaced by a constant private answer (statusgen/scanpublish.go line 321) | caught | FAIL; prose and external fields published where published=false is expected | 2026-11-26 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S7 | row 7's statusgen command in the admitted home, to read its output | exit 0 | summary: 4 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing; the 3 unchecked are this brief's out-of-scope routes (principal.go, nextup.go, the intake-desk skill); with no base in the command the tool derived the delivery from history as 1d5ed867e4b6 pinned to first parent 4c4ac6e5d650, the same delivery named above; it also prints PRODUCER-DELIVERY: COULD-NOT-CHECK, which row 7's Expect does not grade | 2026-11-26 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing values. sensitive-data is yes and gate is human; the act that cannot be undone is publishing judgment text to a public repository, since pushed text cannot be taken back. Enumerated over the non-test diff of the delivering merge (1d5ed867e4b6 against 4c4ac6e5d650) and re-read at a5f18d4b0c34: the public/private decision in scanRootPublic (statusgen/scanpublish.go lines 305 to 321), the closed-shape allowlist (typed-reference shapes at lines 62 to 65, field shapes at lines 80 to 85, evidence cap at 236, rows cap at 280), the judgment-log scanner buffer (65536 initial, 1024*1024 maximum, line 344), the companion file mode 0o644 (lines 551 and 560), the land refusal exit (tools/desk/cmd/scanloop/landverb.go line 90) and the scan's could-not-check exit 2 (statusgen/main.go). Ranked by irreversibility, the first two groups decide what reaches a public repository; the buffer, file mode and exit codes are reversible operational values and rank last.

- RISK-VALUE: DERIVED — push destinations required = 1 (len(dests) != 1 counts as public) @ statusgen/scanpublish.go:315 — the bytes go to the origin's push destination, so the roster marking read must belong to the one repository that receives them; with zero or several push URLs the receiving repository is not known, and counting the root as public fails closed. Reversible by an edit, and the error direction only withholds text.
- RISK-VALUE: DERIVED — empty commit target = onBehalfOfPublicForm("") @ statusgen/scanpublish.go:308 — with no declared target there is no repository whose marking can be read, and the reader returns public for any repository the roster does not mark private (statusgen/principal.go:76, not changed by this item), so a scan given no target publishes no free text, as the ruling's option A requires.
- RISK-VALUE: DERIVED — handoffTypedRefs shapes (stream/NN with 2 to 4 digits, owner/repo#N, #N, F-slug) @ statusgen/scanpublish.go:62 — these are the typed-reference shapes of spec/intake-exit-v1.md, and each is anchored and drawn from a character class with no space, so a value that matches cannot carry a sentence; this is what makes typed references only (option A) enforceable.
- RISK-VALUE: NAMED, NOT DERIVED — evidence length cap = 96 @ statusgen/scanpublish.go:236 — the decision record states the 96 but gives no reason for it, and the owner/repo#N shape at line 63 has no length bound of its own, so a valid long reference over 96 characters is held back. The cap only restricts publication (an over-cap value is counted unpublished and stays in the local log), so a wrong value withholds text and never leaks it; the reason for 96 belongs to the decision record's author.
- RISK-VALUE: NAMED, NOT DERIVED — verification rows cap = 64 @ statusgen/scanpublish.go:280 — stated in the decision record without a derivation; it bounds row count, not content, and an over-cap block is held back rather than published.
- RISK-VALUE: DERIVED — land refusal exit = deskkit.ExitRefused (5 @ tools/desk/internal/deskkit/exitcodes.go:51) @ tools/desk/cmd/scanloop/landverb.go:90 — a bug or placeholder exit without a judgment is refused by a compiled-in constraint, which is the meaning the desk tools give exit 5; row 2's test asserts 5. Reversible by an edit.

Attempts: one run of the Verify table, the container-netns bracket above (2026-11-26T02:30:00Z to 2026-11-26T02:31:11Z, 7 rows), uncredited, credit-unchecked; the desk re-derives credit after landing.

Tally: 7 Verify rows; 6 pass (rows 2 to 7); 1 could-not-run, check-definition (row 1, exit 0 but its event-time clause is graded only through the Desk-decided substitution); 0 fail.

**VERIFY: BLOCKED** — rows 2 to 7 pass at a5f18d4b0c34. Row 1 exits 0 with its named PASS, but its "one with the wrong event time does not" clause is met only through the delivering pull request's Desk-decided substitution (latest recorded event wins, no event-time match), so it carries no acceptance credit until a reviewed check-definition repair or the issue-wire event-time follow-up lands; no open issue covers it yet. Nothing observed fails the brief.

## Review
Gate: human (from frontmatter — sensitive-data is yes). Reviewer records verdict + date in the stream README table. Human gate is MANDATORY.
