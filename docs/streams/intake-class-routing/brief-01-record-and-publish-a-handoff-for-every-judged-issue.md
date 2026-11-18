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
- The default dispatch view cannot show that a stream has no eligible row. It is a score-ranked selection capped at a span of 20 and at most 4 rows per stream; the per-stream figure is a maximum, not a reservation, and the fill stops when the span is full (`statusgen/nextup.go:25`, `:57`, and the pass-1 loop at `:1096`). A correctly admitted placeholder can be outranked by every higher-priority row. The uncapped read is `statusgen --root <root> --next-up --span 100000`: with the span out of the way, a stream's rows are limited only by its own cap. That cap starts at 4 and is lowered by the stream's claimed items, by a declared `max-concurrent` and by an active drive's per-stream cap (`:1039` to `:1077`). The same output counts every eligible row a stream cap held back, in `heldByStreamCap` (`statusgen/dispatchqueue.go:55`). An active drive can also withhold a drive-boosted row past its worker floor, counted separately in `heldByDriveWorkerCap` and omitted from the output when 0 (`statusgen/dispatchqueue.go:71`, `:186`). So that read shows an `issue-loop` row whenever one is eligible, unless something held it, and both counts at 0 rule that out. Both counts are totals for the root, not per stream, so a non-zero count is inconclusive even when another stream caused it. The output is one line of compact JSON, so a line count of the match is 1 or 0.
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
| 5 | check +flow | host:intake-installation | `statusgen --root . --next-up --span 100000 \| grep -c '"stream": *"issue-loop"'` | run in the root that holds the placeholder: prints 1, after one direct repair was recorded and published by the reference's steps; before that step the same command printed 0 on this installation. The read is uncapped by span because the default view's 20 rows can omit an eligible placeholder, and the filter is on the stream because a home-repository placeholder's id is `issue-loop/issue-<N>`. The witness also records the same output's `heldByStreamCap` and `heldByDriveWorkerCap` (absent means 0): a printed 0 with both at 0 is a fail, and a printed 0 with either above 0 is could-not-check, because both are totals for the root |
| 6 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:01` | exit 0; output contains "0 disproved" |
| 7 | check:ci +neighbour | tree | `(k=$(tr '\n' ' ' < plugins/assay/skills/intake-desk/SKILL.md) && r=$(tr '\n' ' ' < plugins/assay/references/intake-handoff.md) && for p in 'scrub its free-text' 'not an assignment authority' 'Do not promote raw placeholders'; do case "$k" in *"$p"*) ;; *) echo "skill lost: $p"; exit 1;; esac; done && for p in 'scrub its free-text' 'spec/intake-handoff-v1.md'; do case "$r" in *"$p"*) ;; *) echo "reference lacks: $p"; exit 1;; esac; done)` | exit 0 and no output; the shortened skill still carries the scrub instruction and both guard sentences, and the reference states the scrub requirement and cites the spec. Fails when any is absent |

Failure modes mapped: a wrong-but-plausible example in the procedure → row 1; the example reader silently matching nothing → row 2 and row 1's no-example failure; the skill growing or losing its pointer → row 3; a stale generated copy → row 4; the writer and the board disagreeing on a real installation → row 5; the scrub instruction or a guard sentence lost when the passage is shortened, or the reference's publication step written without it → row 7. Whether the procedure reads clearly to a session is review-only.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Verification 2026-11-18 at 283b024f3331

Independent post-merge verification of merged main 283b024f33318f1d811b8e0f42df6f7f63608fa8 by a non-implementer verifier: model claude-opus-5-5, tier strong. The delivery is PR #6309 (merge 8a1afee596d8, first parent 83f7b9ebd512). Pre-work admission: `deskdispatch --check-verifier` exited 0 before any row ran, binding run c43d97e3133f, source 283b024f3331, model claude-opus-5-5 and tier strong. The Verify table is not protected. Re-derivation (`deskpathguard rederive`) found 7 rows, every one from-base, and 0 author-added.

Verification-Attestation: medici-finance/assay#6892 run=c43d97e3133fbdff2fb0636b6f1cadfd20446bf03ce32225ff09fdc4f82832a0 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 brief=docs/streams/intake-class-routing/brief-01-record-and-publish-a-handoff-for-every-judged-issue.md model=claude-opus-5-5 tier=strong

Lanes. Rows 1, 2, 3, 4, 6 and 7 are check:ci rows. They ran in the container lane under one admission bracket, `statusgen verifyrun --rows 1,2,3,4,6,7`, in three steps:

1. `--open-bracket` on the host, with a new empty module cache.
2. `--sandbox=container-netns` inside `docker run --rm --network none --read-only` on the release-pinned runner image `ghcr.io/medici-finance/verify-runner:v1.0.87@sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1`. The digest is read from the v1.0.87 release asset verify-runner-image.txt, whose sha256 matches the release checksums.txt.
3. `--close-bracket` on the host.

Inside the container the user was non-root (uid 501). The home, its git directory and the module cache were mounted read-only. The module proxy was off and the toolchain local. The build cache and temp directory were container-local. HOME held only a roster copy, with no key, App configuration or forge credential. The declared verifier Git environment was exported. Only the witness directory was writable. `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`.

Row 5 is class `check +flow` with env host:intake-installation, so it is not a container row. It is recorded below as could-not-run.

Attempts. Attempt 1 (14:33:54Z) was refused at container admission before any row ran: `container admission refused; no Verify rows executed: GOSUMDB=off; go.sum verification stays on in the container`. It was superseded by attempt 2, the same run without that variable. Attempt 2 (14:34:20Z to 14:35:26Z) is the witness below.

Container witness, the closed witness file's bytes:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffProcedureExamples$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestHandoffProcedureExamples ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffExamplesFlagPlant$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestHandoffExamplesFlagPlant ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 710 && grep -c 'references/intake-handoff.md' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:4355a46b19d3 | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/skillslint && GOWORK=off go test -count=1 ./...)` | pass exit=0 sandbox=container-netns | sha256:fa56c79c027b | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --consumers --brief assay:assay:intake-class-routing:01` | fail exit=2 sandbox=container-netns | sha256:05ff11b146b2 | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `(k=$(tr '\n' ' ' < plugins/assay/skills/intake-desk/SKILL.md) && r=$(tr '\n' ' ' < plugins/assay/references/intake-handoff.md) && for p in 'scrub its free-text' 'not an assignment authority' 'Do not promote raw placeholders'; do case "$k" in *"$p"*) ;; *) echo "skill lost: $p"; exit 1;; esac; done && for p in 'scrub its free-text' 'spec/intake-handoff-v1.md'; do case "$r" in *"$p"*) ;; *) echo "reference lacks: $p"; exit 1;; esac; done)` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:1efe6c181cc7f3cd68aeef2ed8ec2ef27c832d9b0ca1ecad663a759e8ba6f410
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6892 run=c43d97e3133fbdff2fb0636b6f1cadfd20446bf03ce32225ff09fdc4f82832a0 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 brief=docs/streams/intake-class-routing/brief-01-record-and-publish-a-handoff-for-every-judged-issue.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c43d97e3133fbdff2fb0636b6f1cadfd20446bf03ce32225ff09fdc4f82832a0 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 tree=sha256:1efe6c181cc7f3cd68aeef2ed8ec2ef27c832d9b0ca1ecad663a759e8ba6f410 witness=sha256:1ff4ce65a3f33aaa995caf855da64b69b0d9880e51efb959d522d761656ddd48 opened=2026-11-18T14:32:40Z closed=2026-11-18T14:38:39Z before=receipt after=check-verifier

Row 6 disposition. The tool-written cell reads `fail exit=2`, but the command did not check anything. Its own output under the declared verifier Git environment is: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:intake-class-routing:01 is not in the diff against 283b024f33318f1d811b8e0f42df6f7f63608fa8, and the brief's delivery could not be derived: ... the history cannot be searched through the library read path (native discovery/config/shallow environment profile) ... No routing claim was corroborated and none was disproved." So the "0 disproved" obligation was not observed to fail. It is could-not-check, and this pass counts it as could-not-run.

I then ran a supplemental diagnostic in the same image and mounts, outside the bracket, changing one variable per run. It carries no acceptance credit. `GIT_CONFIG_COUNT=0` alone produces the COULD-NOT-CHECK exit 2. Without it, the same command exits 0 with "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". The declared verifier environment must stay as declared, so the row stays held. It is attached to medici-finance/assay#6851, the same class: the zero config count selects a native Git profile the library refuses.

Row 5, the live witness. It could not run: its precondition is unmet on every installation.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 5 | `statusgen --root . --next-up --span 100000 \| grep -c '"stream": *"issue-loop"'` | prints 1 in the root that holds the placeholder, after one direct repair was recorded and published by the reference's steps | could-not-run: precondition unmet. No direct repair has been recorded and hand-published on any installation, so no root holds a published companion to read. At the four stream roots of one adopting installation, read at their main, there are 0 committed handoff companions and 0 placeholders with a handoff key. Recording and publishing a judgment is an intake act through a reviewed intake change, not a verifier act. Blocker filed: medici-finance/assay#6898 | 2026-11-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong) (on-behalf-of human:ian) |

Supplemental observation for row 5 (no acceptance credit). I ran the row's command with installed statusgen v1.0.87 at a read-only snapshot of each of that installation's four roots' main. It printed 0 at every root. At the root that holds the issue-loop placeholders, `heldByStreamCap` was 0 and `heldByDriveWorkerCap` was absent (0). That is the row's stated "before" state, the same as README §1.1. It is not the "after" witness the row owes.

Supplemental observations. These carry no acceptance credit and are not execution witnesses for any row.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 6-diag | the row 6 command in the same image and mounts, outside the bracket, with one Git variable changed per run | diagnostic only | exit 2 COULD-NOT-CHECK only when GIT_CONFIG_COUNT=0 is set; with it unset, exit 0 and "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" | 2026-11-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong) (on-behalf-of human:ian) |
| 5-before | the row 5 command, run with installed statusgen v1.0.87 at a read-only snapshot of each of one installation's four stream roots' main | diagnostic only (the row's "before" state) | exit 0 and printed 0 at every root; at the root that holds the issue-loop placeholders, heldByStreamCap 0 and heldByDriveWorkerCap absent (0); 0 committed handoff companions and 0 placeholders with a handoff key | 2026-11-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong) (on-behalf-of human:ian) |

Risk-bearing values. Every risk answer is "no" and none is irreversible. The diff touches procedure text and tests only. Enumerated over the delivering commits 35324857dd02 and d29fc9130e66:

- `n != 6` @ tools/desk/cmd/scanloop/procedure_examples_test.go:100
- `n != 3` @ :128
- `len(problems) != 3` @ :131
- planted defect lines 8, 31 and 50 @ :137-139

Row 3 also pins the skill bound `-le 710`; the skill is 655 lines at this commit. All are reversible test or procedure constants.

- RISK-VALUE: DERIVED — `n != 6` @ tools/desk/cmd/scanloop/procedure_examples_test.go:100 — the reference's judgment table in §2 has exactly 6 rows, and it carries exactly 6 expect labels (reference lines 68, 91, 114, 137, 160 and 183), one per row as Task 2 requires.
- RISK-VALUE: DERIVED — planted lines 8, 31 and 50 @ tools/desk/cmd/scanloop/procedure_examples_test.go:137-139 — they equal the three expect labels in the plant file (lines 8, 31 and 50). Line 50 is the brief's planted direct repair with a different successor, and the test requires it reported exactly once.

Tally: 7 rows. 5 passed (1, 2, 3, 4, 7) in the closed container bracket. 2 could-not-run (5: live precondition unmet, medici-finance/assay#6898; 6: could-not-check under the declared verifier Git environment, medici-finance/assay#6851). No obligation was observed to fail.

**VERIFY: BLOCKED** — rows 5 and 6 could-not-run (5: no direct repair has been recorded and published on any installation; 6: consumers gate COULD-NOT-CHECK under GIT_CONFIG_COUNT=0); rows 1, 2, 3, 4 and 7 pass.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
