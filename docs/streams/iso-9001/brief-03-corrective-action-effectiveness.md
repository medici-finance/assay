---
brief: assay:assay:iso-9001:03
title: A finding closes on a fired control — the corrective-action effectiveness record
why: >-
  A findings entry records that a corrective action was taken. `resolved: yes` means the work
  landed; it does not mean the failure mode can no longer occur, and nothing in the schema
  carries the difference. The register already holds the nature of the nonconformity and the
  action; it does not hold the result. The corpus already applies the right rule to its own
  audits — a finding closes only when its check goes green, never on a fix commit — and
  already demands, of any brief that adds a check, a mutation row proving the check goes RED.
  This brief generalises that shape to the register, so that closing a finding requires naming
  the command that re-establishes the failure mode is gone, the date it was run, and who ran
  it.
wave: 1
depends: ["iso-9001/01"]
unblocks: ["iso-9001/06", "iso-9001/09"]
effort: M
exec-tier: strong
exec-tier-why: >-
  A schema addition plus a severity policy over an inherited corpus. The transition scoping is
  the judgement — a rule that fires retroactively on every landed finding manufactures
  false positives and is the fastest route to an exemption file.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`statusgen/registerentries.go` — the findings entry schema today: `id`, `date`, `title`, `affects`, `ack`, `resolved`, the `parked-until` / `parked-by` / `parked-reason` triple, and the optional `class` / `control` pair. No effectiveness field, no date, no runner."
  - "`statusgen/findingcontrol.go` — the nearest existing thing: a recurring-class finding must name a landed control before the class counts closed. It is a NOTICE, it is scoped to recurring-class findings only, and it checks that a control LANDED rather than that it FIRES. Its header states its own escalation posture: advisory this phase, a hard error is a later, separate decision."
  - "`docs/brief-rules.md` rule 16 — a brief that adds a CHECK must include a mutation-test Verify row: revert the fix or break the guarded thing, run the check, confirm it goes RED. Where a corrective action is 'add a control', effectiveness is already demonstrated; this brief generalises the shape rather than inventing one."
  - "`docs/mistake-proofing.md` §3 D1 (a control must be shown to fire) and §3 D4 (warnings do not compose — land advisory, census, then flip fatal on a named condition; do not add a permanent NOTICE)."
  - "The parked triple as the in-tree precedent for a group of keys that are all required together, with a named human as the authority: `parked-until` + `parked-by` + `parked-reason`, 'a park is a snooze, not a mute'."
  - "depends iso-9001/01: the per-control evidence row shape — the control, the injected error, the verdict, the date, the tool version — is defined there. This brief reuses it for a finding's effectiveness record rather than inventing a second shape for the same idea."
  - "The standard-side reading: the corrective-action clause asks the organisation to evaluate the need to eliminate the cause, determine whether similar nonconformities exist or could occur elsewhere, review the EFFECTIVENESS of the action taken, and retain records of the nature of the nonconformity, the actions taken and THE RESULTS. The most-written finding against it is a correction recorded as a corrective action, followed by a record closed on the day the action was implemented with no later effectiveness check."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — `git grep -n effectiveness -- statusgen/registerentries.go statusgen/findingcontrol.go` returns nothing; the field does not exist."
version: 1
id: 4001327b-51c1-4468-8ea9-3c471ce5f177
---

# Brief 03 — the effectiveness record

## Context

single-point-of-failure: after this brief, the lint's presence obligation is the control
standing between a finding closed on a fix commit and a clean register. It is deliberately a
thin one — it proves an effectiveness record EXISTS and was attributed and dated, not that
the named command actually re-establishes anything. The independent second layer is
unchanged and must stay: the reviewer question "does this command genuinely fail if the
failure mode returned?" is asked at a different time, by a different actor, on different
evidence. This brief adds a floor; the failure message must say so.

files:
- `statusgen/registerentries.go` (implementation home) — the findings entry schema.
- `statusgen/findingcontrol.go` — the severity path for a recurring-class finding, which this
  brief extends from "a control landed" to "a control was shown to fire".
- `docs/registers.md` — the schema documentation for the new keys.

facts:
- **Three keys, required together, or none.** Mirror the parked triple exactly:
  `effectiveness:` (the command, verbatim, that re-establishes the failure mode is gone),
  `effectiveness-date:` and `effectiveness-by:` (a `human:<name>` or a runner identity, in the
  same shape the Verified cell already uses). One or two of the three present is a hard
  error, the same way a half-filled park is: a partial record is worse than no record because
  it reads as one.
- **The severity must be transition-scoped by the finding's own `date:`.** A finding dated on
  or after the field's introduction and carrying `resolved: yes` owes the triple as a
  PROBLEM; an earlier finding owes it as a NOTICE. The inherited corpus is not made fatal by
  this change. Encode the boundary date as a named constant with a comment, not an inline
  literal, so a reviewer can find and argue with it.
- **`findingcontrol` escalates in the same change, and only for the class it already covers.**
  Once the triple exists, a `class: recurring` finding with `resolved: yes` and a landed
  `control:` but no effectiveness record is the exact gap that check was written to surface —
  promote it there from NOTICE to PROBLEM, under the same date scoping. Leave one-off
  findings alone: `findingcontrol`'s own header says an absent `class:` reads as one-off and
  must never be flagged, and widening that here would be a second decision hiding inside this
  one.
- **Do not add a `cause:` field in this brief.** Root-cause capture is a real gap and it is a
  different obligation with a different failure mode (a cause stated as a restatement of the
  symptom passes any presence check). Presence of a `cause:` string proves nothing, so a lint
  obligation on it would be proofing judgement, which D7 forbids. Name it as out of scope in
  the coverage note.
- **This check must survive the firing audit.** `--lint-audit` flags a rule that has never
  fired as a retirement candidate. This rule needs its own positive control — which is the
  rule applied to itself, and is the point.
- **Rule-tag every emitted message.** New PROBLEM/NOTICE lines carry a stable `[rule-tag]`
  bracket token, the convention `statusgen/lintaudit.go` already extracts; an untagged line
  falls into the unattributed bucket and is invisible to the firing audit.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Add the three optional keys to the findings entry struct** in
   `statusgen/registerentries.go`, beside the parked triple, with a comment stating what each
   is for and that the three are required together.
2. **Enforce the all-or-nothing rule**: one or two of the three present is a hard error naming
   which are missing. Reuse the parked triple's existing message shape rather than inventing a
   second dialect.
3. **Add the closure obligation**, transition-scoped by the finding's `date:` against a named
   constant: `resolved: yes` on or after the boundary without the triple is a PROBLEM; before
   it, a NOTICE. Carry a rule-tag.
4. **Promote the `findingcontrol` path for recurring-class findings** from NOTICE to PROBLEM,
   under the same date scoping, when the effectiveness record is absent. Do not change the
   behaviour for `class:` absent or `class: one-off`.
5. **Write the failure message to be actionable and honest.** It names the finding ID, states
   which of the three keys are missing, states that the check verifies the record's PRESENCE
   and attribution and not whether the named command re-establishes anything, and points at
   rule 16's mutation row as the worked example of what a good one looks like.
6. **Document the keys in `docs/registers.md`** in the shared-conventions section, in the same
   register as the parked triple, with one sentence of reason each. Record the coverage
   boundary beside them (D6): root cause is deliberately not a field, and a present
   effectiveness command is not a demonstration that it fires.
7. **Positive control on this check — the rule applied to itself.** Tests that inject a
   `resolved: yes` finding after the boundary with no triple and assert a PROBLEM; the same
   finding with the triple asserting silence; a finding dated before the boundary asserting a
   NOTICE and not a PROBLEM; and a partial triple asserting the all-or-nothing error.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check +dereference | `git grep -n 'effectiveness' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE, inverts**: no such field at authoring (2026-08-25 @ `6871a3b`) |
| 2 | check +dereference | `git grep -nF 'yaml:"resolved"' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE**: the field the new obligation is keyed on still exists and was not renamed under the change |
| 3 | check +dereference | `git grep -n 'recurringClass' -- statusgen/findingcontrol.go` | exit 0 — **DEREFERENCE**: the recurring-class concept the severity promotion is scoped to is real, so the promotion is not a dangling reference |
| 4 | check | `git grep -n 'effectiveness' -- docs/registers.md` | exit 0 — the schema page documents the new keys; a lint that fails a pull request citing a rule needs the rule written down |
| 5 | check | `cd statusgen && go test . -count=1 -run '^TestEffectivenessTripleRequiredTogether$' -v > "${TMPDIR:-/tmp}/iso9001-03-row5.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessTripleRequiredTogether' "${TMPDIR:-/tmp}/iso9001-03-row5.out"` | exit 0 — one or two of the three keys present is a hard error naming the missing ones; all three, or none, is silent |
| 6 | check +mutation | `cd statusgen && go test . -count=1 -run '^TestEffectivenessMissingOnResolvedIsAProblem$' -v > "${TMPDIR:-/tmp}/iso9001-03-row6.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessMissingOnResolvedIsAProblem' "${TMPDIR:-/tmp}/iso9001-03-row6.out"` | exit 0 — **mutation row (rule 16), positive control**: a `resolved: yes` finding dated after the boundary with no triple is a PROBLEM; the same finding with the triple is silent. Disarm the obligation and this test goes RED |
| 7 | check | `cd statusgen && go test . -count=1 -run '^TestEffectivenessInheritedCorpusStaysAdvisory$' -v > "${TMPDIR:-/tmp}/iso9001-03-row7.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessInheritedCorpusStaysAdvisory' "${TMPDIR:-/tmp}/iso9001-03-row7.out"` | exit 0 — the promotion is transition-scoped: a finding dated before the boundary produces a NOTICE and never a PROBLEM |
| 8 | check +neighbour | `cd statusgen && go test . -count=1 -run '^TestFindingControlOneOffUnchanged$' -v > "${TMPDIR:-/tmp}/iso9001-03-row8.out" 2>&1 && grep -F -e '--- PASS: TestFindingControlOneOffUnchanged' "${TMPDIR:-/tmp}/iso9001-03-row8.out"` | exit 0 — **neighbour row (rule 17)**: `findingcontrol` shares the findings frontmatter reader; a finding with no `class:` or `class: one-off` behaves exactly as before this change |
| 9 | check | `cd statusgen && go test ./... -count=1` | exit 0 — the full lint suite passes |
| 10 | check | `git grep -cE -e 'presence' -e 'adequacy' -- statusgen/findingcontrol.go statusgen/registerentries.go statusgen/effectiveness.go` | exit 0; a non-zero count — the presence-not-adequacy boundary is stated in the source the failure message is built from |
| 11 | check | `cd statusgen && go run . --root .. --lint` | exit 0, `LINT: PASS` — no `PROBLEM:` line tagged `effectiveness-partial`, `effectiveness-missing` or `finding-control-unfired`, so the inherited register is not made fatal. **Census**: every line carrying either closure tag is a `NOTICE:` on a finding dated before 2026-10-08; at implementation (2026-10-08) there is exactly one — `NOTICE: [effectiveness-missing]` for `F-path-claim-tree` (resolved, dated 2026-09-02, no record). This row shows the tree is not made fatal; it does NOT show the closure rules fire (row 12 does) |
| 12 | check +mutation +flow | `cd statusgen && go test . -count=1 -run '^TestEffClosureLintEntryPoint$' -v > "${TMPDIR:-/tmp}/iso9001-03-row12.out" 2>&1 && grep -F -e '--- PASS: TestEffClosureLintEntryPoint' "${TMPDIR:-/tmp}/iso9001-03-row12.out"` | exit 0 — **mutation row (rule 16) at the enforcement point**: drives `--lint` over a temp tree. A post-boundary resolved finding with no record exits non-zero with one `PROBLEM: [effectiveness-missing]` line; the same finding dated before the boundary leaves the exit at 0 with a `NOTICE:`; a recurring finding with a landed control exits non-zero with `PROBLEM: [finding-control-unfired]`; a partial record exits non-zero with `PROBLEM: [effectiveness-partial]`. Delete the two closure-rule calls in `run()` (`statusgen/main.go`), or append their problems to the notices, and this test goes RED |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — 2026-10-08 assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian)

First verify pass on merged main. Subject: main aff2ef11c744, confirmed equal to the forge's main by an independent API read at run time. The implementing commit is 62c570391240 (#2374); the six commits between it and the subject touch none of the implementation files, the test file or docs/registers.md. All twelve rows were run by hand (go1.27.1 darwin/arm64, no network or cluster contact), then again by statusgen v1.0.32 verifyrun. The desk re-ran rows 10 and 11 itself at the same sha: exit 0 each, and `statusgen verifyrun --check` on the brief reads 12 pass, 0 fail, 0 could-not-run/missing.

| # | Command | Expect | Result (exit + real output line) | Date | Runner |
|---|---------|--------|----------------------------------|------|--------|
| 1 | `git grep -n 'effectiveness' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE, inverts**: no such field at authoring (2026-08-25 @ `6871a3b`) | PASS. exit 0; 4 lines, among them `statusgen/registerentries.go:172` declaring the `Effectiveness` field with yaml key `effectiveness,omitempty`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `git grep -nF 'yaml:"resolved"' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE**: the field the new obligation is keyed on still exists and was not renamed under the change | PASS. exit 0; one line, `statusgen/registerentries.go:147`, the `Resolved bool` field. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `git grep -n 'recurringClass' -- statusgen/findingcontrol.go` | exit 0 — **DEREFERENCE**: the recurring-class concept the severity promotion is scoped to is real, so the promotion is not a dangling reference | PASS. exit 0; 3 lines, among them `statusgen/findingcontrol.go:38:const recurringClass = "recurring"`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `git grep -n 'effectiveness' -- docs/registers.md` | exit 0 — the schema page documents the new keys; a lint that fails a pull request citing a rule needs the rule written down | PASS. exit 0; 14 lines in docs/registers.md, the first at line 42. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessTripleRequiredTogether$' -v > "${TMPDIR:-/tmp}/iso9001-03-row5.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessTripleRequiredTogether' "${TMPDIR:-/tmp}/iso9001-03-row5.out"` | exit 0 — one or two of the three keys present is a hard error naming the missing ones; all three, or none, is silent | PASS. exit 0; `--- PASS: TestEffectivenessTripleRequiredTogether (0.00s)`, with its 10 subtests passing. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessMissingOnResolvedIsAProblem$' -v > "${TMPDIR:-/tmp}/iso9001-03-row6.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessMissingOnResolvedIsAProblem' "${TMPDIR:-/tmp}/iso9001-03-row6.out"` | exit 0 — **mutation row (rule 16), positive control**: a `resolved: yes` finding dated after the boundary with no triple is a PROBLEM; the same finding with the triple is silent. Disarm the obligation and this test goes RED | PASS. exit 0; `--- PASS: TestEffectivenessMissingOnResolvedIsAProblem (0.00s)`. Mutation checked on a scratch copy: with the fatal route sent to notices the row exits 1 on `--- FAIL: TestEffectivenessMissingOnResolvedIsAProblem`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessInheritedCorpusStaysAdvisory$' -v > "${TMPDIR:-/tmp}/iso9001-03-row7.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessInheritedCorpusStaysAdvisory' "${TMPDIR:-/tmp}/iso9001-03-row7.out"` | exit 0 — the promotion is transition-scoped: a finding dated before the boundary produces a NOTICE and never a PROBLEM | PASS. exit 0; `--- PASS: TestEffectivenessInheritedCorpusStaysAdvisory (0.00s)`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test . -count=1 -run '^TestFindingControlOneOffUnchanged$' -v > "${TMPDIR:-/tmp}/iso9001-03-row8.out" 2>&1 && grep -F -e '--- PASS: TestFindingControlOneOffUnchanged' "${TMPDIR:-/tmp}/iso9001-03-row8.out"` | exit 0 — **neighbour row (rule 17)**: `findingcontrol` shares the findings frontmatter reader; a finding with no `class:` or `class: one-off` behaves exactly as before this change | PASS. exit 0; `--- PASS: TestFindingControlOneOffUnchanged (0.00s)`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `cd statusgen && go test ./... -count=1` | exit 0 — the full lint suite passes | PASS. exit 0; `ok github.com/medici-finance/assay/statusgen 82.274s` and `ok github.com/medici-finance/assay/statusgen/streamview 0.105s`; no FAIL line. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `git grep -cE -e 'presence' -e 'adequacy' -- statusgen/findingcontrol.go statusgen/registerentries.go statusgen/effectiveness.go` | exit 0; a non-zero count — the presence-not-adequacy boundary is stated in the source the failure message is built from | PASS. exit 0; counts `statusgen/effectiveness.go:4`, `statusgen/findingcontrol.go:1`, `statusgen/registerentries.go:1`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `cd statusgen && go run . --root .. --lint` | exit 0, `LINT: PASS` — no `PROBLEM:` line tagged `effectiveness-partial`, `effectiveness-missing` or `finding-control-unfired`, so the inherited register is not made fatal. **Census**: every line carrying either closure tag is a `NOTICE:` on a finding dated before 2026-10-08; at implementation (2026-10-08) there is exactly one — `NOTICE: [effectiveness-missing]` for `F-path-claim-tree` (resolved, dated 2026-09-02, no record). This row shows the tree is not made fatal; it does NOT show the closure rules fire (row 12 does) | PASS. exit 0; stdout is the single line `LINT: PASS`; no line starts `PROBLEM:`; exactly one line carries any of the three tags, a `NOTICE: [effectiveness-missing]` for `F-path-claim-tree` (dated 2026-09-02, resolved). | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test . -count=1 -run '^TestEffClosureLintEntryPoint$' -v > "${TMPDIR:-/tmp}/iso9001-03-row12.out" 2>&1 && grep -F -e '--- PASS: TestEffClosureLintEntryPoint' "${TMPDIR:-/tmp}/iso9001-03-row12.out"` | exit 0 — **mutation row (rule 16) at the enforcement point**: drives `--lint` over a temp tree. A post-boundary resolved finding with no record exits non-zero with one `PROBLEM: [effectiveness-missing]` line; the same finding dated before the boundary leaves the exit at 0 with a `NOTICE:`; a recurring finding with a landed control exits non-zero with `PROBLEM: [finding-control-unfired]`; a partial record exits non-zero with `PROBLEM: [effectiveness-partial]`. Delete the two closure-rule calls in `run()` (`statusgen/main.go`), or append their problems to the notices, and this test goes RED | PASS. exit 0; `--- PASS: TestEffClosureLintEntryPoint (0.15s)`, with its 5 subtests passing. Mutation checked on a scratch copy: with the two closure-rule calls removed from `run()` the row exits 1 on `--- FAIL: TestEffClosureLintEntryPoint`. | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |

- **The Verify table was edited by the implementing change.** #2374 is the only commit that touches this brief after authoring, and its diff to the brief is one hunk covering the Verify table alone. What moved, compared row by row against the table before that commit:
  - Rows 1, 2, 3, 4 and 9: a Class cell was added; command and Expect are byte-identical.
  - Rows 5, 6, 7 and 8: tightened. The earlier form ran an unanchored `-run` selector across `./...` and exited 0 when no test matched; the present form anchors the test name and requires its `--- PASS:` line. Row 9 still runs `./...`.
  - Row 10: one path was added to the searched set, so a match in the added file alone would now satisfy it. Not load-bearing here: the earlier command, run by hand at the subject sha, also exits 0, with one match in each of its two files.
  - Row 11: the command is unchanged; the Expect was relaxed from zero firings of the new rule on this repo's register to exit 0, `LINT: PASS`, no PROBLEM, and exactly one advisory NOTICE (`F-path-claim-tree`). The earlier wording would not hold at the subject sha, because the rule does fire once as a NOTICE. The exit-code half is the same before and after.
  - Row 12: added, a mutation row at the lint entry point.
  - No row was dropped (11 before, 12 after) and no exit code is neutralised.
- **Rows 6 and 12 claim a RED under mutation that their commands cannot show.** Both claims were checked on scratch copies of the statusgen directory, outside the worktree, and both hold as stated in the table above.

RISK-VALUE scope: all four risk answers are no. Enumeration covered the non-test Go diff of 62c570391240 (seven files under statusgen) and the brief's Deliverables. One bound was found, the boundary date; the other literals are a date layout string and three rule tags, which are identifiers, not bounds.

RISK-VALUE: DERIVED — effectivenessBoundary = "2026-10-08" @ statusgen/effectiveness.go:42 — the brief fixes the boundary as the date the field is introduced, and the field was introduced by 62c570391240, committed 2026-10-08 (the same calendar date in the committer's zone and in UTC). The comparison at statusgen/effectiveness.go:128 is inclusive, matching the brief's "on or after".

VERIFY: PASS — 12 of 12 rows meet Expect by hand and hold a pass witness; no defect was found in the deliverable.

**The board row is not changed by this run and stays `implemented`.** Because the implementing change edited the table this run executes (first bullet above), the status change is left to a maintainer. Review notes recorded internally; maintainer follow-up required (#2374).

The table below is the statusgen verifyrun witness for this run, verbatim (exit 0).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `git grep -n 'effectiveness' -- statusgen/registerentries.go` | pass exit=0 | sha256:569874b591f3 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 2 | `git grep -nF 'yaml:"resolved"' -- statusgen/registerentries.go` | pass exit=0 | sha256:bb35f889e367 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 3 | `git grep -n 'recurringClass' -- statusgen/findingcontrol.go` | pass exit=0 | sha256:4b4d06347113 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git grep -n 'effectiveness' -- docs/registers.md` | pass exit=0 | sha256:64d2ae4bbc2b | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessTripleRequiredTogether$' -v > "${TMPDIR:-/tmp}/iso9001-03-row5.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessTripleRequiredTogether' "${TMPDIR:-/tmp}/iso9001-03-row5.out"` | pass exit=0 | sha256:efff63f657df | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessMissingOnResolvedIsAProblem$' -v > "${TMPDIR:-/tmp}/iso9001-03-row6.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessMissingOnResolvedIsAProblem' "${TMPDIR:-/tmp}/iso9001-03-row6.out"` | pass exit=0 | sha256:eb8d6fa7d56e | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test . -count=1 -run '^TestEffectivenessInheritedCorpusStaysAdvisory$' -v > "${TMPDIR:-/tmp}/iso9001-03-row7.out" 2>&1 && grep -F -e '--- PASS: TestEffectivenessInheritedCorpusStaysAdvisory' "${TMPDIR:-/tmp}/iso9001-03-row7.out"` | pass exit=0 | sha256:4e51958d1d80 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test . -count=1 -run '^TestFindingControlOneOffUnchanged$' -v > "${TMPDIR:-/tmp}/iso9001-03-row8.out" 2>&1 && grep -F -e '--- PASS: TestFindingControlOneOffUnchanged' "${TMPDIR:-/tmp}/iso9001-03-row8.out"` | pass exit=0 | sha256:4c778e2452ec | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd statusgen && go test ./... -count=1` | pass exit=0 | sha256:5af066f075a9 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 10 | `git grep -cE -e 'presence' -e 'adequacy' -- statusgen/findingcontrol.go statusgen/registerentries.go statusgen/effectiveness.go` | pass exit=0 | sha256:3ba23beddc6a | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go run . --root .. --lint` | pass exit=0 | sha256:2203591b80dd | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test . -count=1 -run '^TestEffClosureLintEntryPoint$' -v > "${TMPDIR:-/tmp}/iso9001-03-row12.out" 2>&1 && grep -F -e '--- PASS: TestEffClosureLintEntryPoint' "${TMPDIR:-/tmp}/iso9001-03-row12.out"` | pass exit=0 | sha256:9842c60b0ba7 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (on-behalf-of human:ian) (forge-identity) |

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in
the stream README table. Reviewer questions specific to this brief: (1) is the boundary date a
named constant with a comment rather than an inline literal, and is the inherited corpus
genuinely left advisory? (2) does the check carry its OWN positive control — the rule applied
to itself? (3) is the `findingcontrol` promotion confined to `class: recurring`, with a test
proving one-off behaviour is unchanged? (4) does the failure message say which half it covers
— presence and attribution, not whether the named command re-establishes anything? (5) was
`cause:` genuinely left out rather than smuggled in as an unenforced key?
