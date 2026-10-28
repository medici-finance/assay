---
brief: assay:assay:desk-tools:36
title: "The scheduled board reconcile's PR class becomes generated output — auto-approve lane (default) or direct-CI single-writer, per the driver's ruling on #2688"
why: >-
  The scheduled board reconcile (statusgen reconcile --apply, landing as
  `chore(board): reconcile` PRs by app/assay-board-writer) writes DERIVED state: Status-cell
  flips each witnessed by a merged PR carrying the brief's `Brief:` trailer, plus regen of
  the generated tables. It is the same artifact class as STATUS.md regen — which lands by
  CI single-writer with no PR at all — yet it currently consumes a dispatched reviewer slot
  and an authorization-needed human gate on every cron tick, reviewing content a reviewer
  cannot improve: the flips are derived, and the tool already self-holds any row that would
  lint. The driver ruled on #2688 (2026-10-11): the class is generated output; rule it into
  the auto-approve lane, or convert it to direct-CI single-writer — mechanism choice
  delegated to this brief. The residual risks the old review nominally covered (stale
  source reads, a wrong witness) are guarded better by the tooling than by a model pass:
  the reconcile's wouldApply lint, the derived-source discipline, and branch protection.
  This is a control-scope change (review-coverage narrowing) and exists ONLY because the
  human ruling on #2688 authorizes it.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [2688]
schema: brief-v2
authored: 2026-10-11 by a house authoring session at the driver's direction (ruling recorded on assay#2688)
sources:
  - "assay#2688 — the driver's ruling this brief implements: generated class, auto-approve lane or direct-CI, mechanism delegated. This brief cannot weaken the ruling's guards (self-hold, witness discipline, branch protection)."
  - ".github/workflows/ — the reconcile PR's own workflow (the wouldApply/reconcile machinery, #2440/#2441 era) is where the class's landing path lives today; the auto-lane's membership config and R-8 enactment line (docs/streams/issue-flow/rulings.md) are where a lane opt-in lands."
  - "CLAUDE.md-level auto-lane doctrine: the reviewer App merges driver-opted-in areas (never stream briefs) only via deskautolane merge, inert until R-8's Sign-off line holds an Enact: R-8 acceptance — never by hand, never on a session's own reading of lane membership."
  - "freshness-checked 2026-10-11 @ a666ceb1e (origin/main) — brief-36 free; #2651 (the class's first instance) still open under standing rules; the reconcile PR class has zero prior merges."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: the mechanism choice, the lane/config
  edit, the R-8 enactment line, and the runsheet leg must compose with the auto-lane's
  own inertness rules — and the Verify rows must prove lane membership the same way
  deskautolane itself reads it, not by prose.
consumers:
  - "the auto-approve lane's membership config (wherever deskautolane reads opted-in areas — the worker locates it and cites the exact file here at implementation): fixed-here (the board-reconcile PR class: match on title `chore(board): reconcile` + author app/assay-board-writer, scoped to nothing broader)"
  - "docs/streams/issue-flow/rulings.md (planned, NEW — the register does not exist on main today; the implementation CREATES it under either mechanism, because the ruled record needs a repo home either way): fixed-here (the R-8 enactment line for this area, citing #2688 in its ratified form; under mechanism A the Sign-off line completes per the lane's own discipline, under B it records the conversion and ships EMPTY — inert)"
  - "the reconcile workflow / ruleset: fixed-here ONLY if mechanism B (direct-CI) is chosen — a human runsheet carries any .github/workflows/ or ruleset change (the worker App lacks the workflows scope); under mechanism A no workflow file changes"
  - "docs/runsheets/reconcile-lane-hunk.md (planned): fixed-here under mechanism B (the exact workflow/ruleset diff + one-line apply command)"
version: 1
id: 7e2c4a18-d50b-4f61-9a3e-64c8f2d1b7a3
---

# Brief 36 — The reconcile PR class becomes generated output

## Context

The driver ruled on #2688: the scheduled board reconcile's PRs are generated output; they
belong in the auto-approve lane or on a direct-CI single-writer path. Open #2651 — the
class's first instance — merges under the standing rules regardless of this brief; the
ruling governs the class from this brief's landing forward.

**Mechanism (the ruled fork, default A):**

- **A (default) — auto-approve lane.** The reconcile PR class joins deskautolane's
  driver-opted-in areas: membership matched on title + author (nothing broader), the R-8
  enactment line added citing #2688. The PR form stays (audit trail), the reviewer slot and
  the authorization-needed human gate stop consuming on this class. Choose A unless the
  lane machinery makes the membership match awkward — it is the smaller change and touches
  no workflow file.
- **B — direct-CI single-writer.** The reconcile lands on main the way STATUS.md regen
  does: the workflow pushes, no PR. Requires the ruleset bypass for the board-writer
  identity and a main-guard carve-out; every workflow/ruleset edit rides a human runsheet
  (the worker App cannot push them). Choose B only if A is structurally awkward — record
  the reason in the PR body.

**Boundaries this brief holds:**

1. **Nothing weakens the class's own guards** — the reconcile's wouldApply self-hold, the
   merged-PR witness discipline, and branch protection all stay exactly as they are.
2. **The lane membership is narrow** — title `chore(board): reconcile` AND author
   `app/assay-board-writer`, never a prefix or an owner match that could catch sibling
   PRs. If the lane config cannot express this precisely, that is a reason to pick B, not
   to widen the match.
3. **Never stream briefs** — the auto-lane's standing exclusion; this brief changes the
   reconcile class only.
4. **#2651 itself is out of scope** — it merges under standing rules; the first
   post-landing reconcile tick is the ruling's caught-by.

## Task

**Task 1 — locate and read the lane machinery.** Find where deskautolane reads its
opted-in areas and how the R-8 enactment line binds (docs/streams/issue-flow/rulings.md);
cite both exact paths in the PR body. Decide A vs B per the Context fork and say which,
with the reason, in the PR body's first line.

**Task 2 (A) — the lane membership + enactment.** Add the reconcile class to the lane
config (narrow match, boundary 2) and the R-8 sign-off line citing #2688.

**Task 2 (B, only if A is awkward) — the runsheet.** Author
docs/runsheets/reconcile-lane-hunk.md (planned) with the exact workflow/ruleset diff and
one-line apply command; the worker touches zero workflow files.

**Task 3 — changelog fragment.** changelog/desk-tools-36.md (planned), one bullet.

files:
- the lane config + rulings.md (Task 2A; exact paths cited at implementation)
- docs/runsheets/reconcile-lane-hunk.md (planned) — Task 2B only
- changelog/desk-tools-36.md (planned) — Task 3

## Verify (executable — no prose-only DoD items)

Row 1 is discriminating under either mechanism; rows 2–3 apply per mechanism chosen; row 4
passes today and must still pass. Run from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | prints `ok`: the register the implementation CREATES carries the R-8 enactment line citing the ruling. FAILS today: the register does not exist (grep exits 2 on the missing file — the fail-state is the absent artifact, stated as such). |
| 2 | check | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | prints `ok` (mechanism A only; under B record not-applicable with the reason): the lane config carries the title match — AND the PR body demonstrates the REAL admission predicate against the installed deskautolane: the exact-title + exact-author case ADMITS, and the two negatives (same title, different author; near-miss title, correct author) each REFUSE, quoting the verb's own output. A bare title grep passes on an unused or mis-authored entry and proves nothing — round 1 demonstrated exactly that false pass. |
| 3 | check +flow | READ FROM THE FORGE after the first scheduled reconcile tick following this brief's merge: the reconcile PR either does not exist (B: direct-CI landing on main) or exists merged by the reviewer App via deskautolane (A) with no dispatched reviewer verdict and no authorization-needed label. | The ruling's caught-by: the class stops consuming a reviewer slot and a human gate. Could-not-check at authoring: no post-landing tick has run. |
| 4 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | prints `ok`: the tree lints clean. Passes today and must still pass. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 2 | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | could-not-run exit=- — unsubstituted placeholder(s) <the lane-config path cited in the PR body> — the row cannot run as written | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 3 | `READ FROM THE FORGE after the first scheduled reconcile tick following this brief's merge: the reconcile PR either does not exist (B: direct-CI landing on main) or exists merged by the reviewer App via deskautolane (A) with no dispatched reviewer verdict and no authorization-needed label.` | fail exit=2 | sha256:80dd39947a6e | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3031 run=77aed9fc601cc5ab900c3b39fae5cf7c34e8b5c6033ee016bf6d5ee81dc2ce01 source=bbf8e351624538c2a03159cd5a58116fb97fc293 brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=any

**Verifier pass — 2026-10-15, dispatched non-implementer verifier (verify-desk), merged main
bbf8e3516.** The delivery (PR #2706, merged 2026-10-13) chose mechanism B — direct-CI
single-writer: the rulings register with R-8 recorded unsigned, plus a human runsheet carrying
the serialized workflow conversion diff for the driver to apply. Each row was run by hand and
again under the execution witness above.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | grep -rq '2688' on the issue-flow rulings register, then echo ok | prints ok | exit 0, prints `ok`; the register exists and cites 2688 twice (R-8 section text); its R-8 Sign-off line (line 36) is empty, so the lane is enacted for no area | 2026-10-15 verify-desk verifier (claude-opus-5-5) |
| 2 | deskautolane --help, then grep the lane config for the reconcile title | prints ok — mechanism A only; under B not applicable, with the reason | NOT APPLICABLE under mechanism B: the delivery chose B, so no lane-config entry exists and no lane-config path was cited; the row's command carries an unsubstituted placeholder (the lane-config path), so the witness above marks it as unexecutable as written. Neighbour check: `deskautolane --help` exits 0. The register's R-8 text states the class does not enter the lane | 2026-10-15 verify-desk verifier (claude-opus-5-5) |
| 3 | forge read after the first scheduled reconcile tick following the merge | reconcile PR absent (B: direct-CI landing on main) or merged via deskautolane (A), with no reviewer verdict and no authorization-needed label | FAIL as observed: post-merge scheduled reconcile ticks have run (assay-statusgen schedule runs succeeded 2026-10-14T19:20Z and 23:11Z), and the reconcile still lands as a pull request — the board/reconcile branch tip 8807c807b (`chore(board): reconcile 2026-10-14`, committed 2026-10-14T23:13Z) heads PR #2651, still OPEN, author app/assay-board-writer, labelled authorization-needed. Cause: the direct-CI conversion is not applied on main — `git apply --check` of the runsheet diff still exits 0 at bbf8e3516 (the three target files are unchanged), and the runsheet itself states the remaining act is the driver's apply. Tracked on the open ruling issue #2688; not a code defect | 2026-10-15 verify-desk verifier (claude-opus-5-5) |
| 4 | build statusgen to a temp path, run --lint, grep LINT: PASS | prints ok | exit 0, prints `ok`; lint tail `LINT: PASS` (NOTICE lines only, none for desk-tools) | 2026-10-15 verify-desk verifier (claude-opus-5-5) |

**Risk-bearing value.** Enumeration over the delivery diff (changelog fragment, rulings
register, issue-flow README, staged-workflows README, the runsheet and its diff file, stream
index row): the item carries risk metadata (all four fields `no`, not irreversible). Literals
the diff introduces: the R-8 Sign-off line, empty, at the rulings register line 36 (an
authority binding); the runsheet's pinned diff digest sha256 f4e3e0df93e1…a866cd8 and byte
count 112,747 at the runsheet lines 29–30; the patch base ca0dad5c4 at the runsheet line 34.
The workflow literals inside the diff file are not live (the diff is unapplied). Ranked: the
empty Sign-off line first (a wrong value would enact the auto-approve lane, reversible by an
edit but a review-coverage grant); the digest/size second; the base last.

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36 — the ruling on #2688 delegated the mechanism, the delivery chose B (direct-CI, the class never enters the lane), so nothing may be enacted; the register's own header states an empty Sign-off line grants nothing.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30 — recomputed at bbf8e3516: `shasum -a 256` gives the same digest and `wc -c` gives 112747 bytes, so the pin matches the bytes the driver would apply.

**VERIFY: FAIL** — row 3: the reconcile class still lands as PR #2651 (open, authorization-needed) after post-merge ticks, because the runsheet's direct-CI conversion awaits the driver's apply (blocked on a human act, tracked on #2688); rows 1 and 4 PASS, row 2 not applicable under mechanism B.

**Re-verify pass, 2026-10-18.** Run by a dispatched non-implementer verifier (verify-desk) at merged
main 6dfb616ef. Verification-Attestation: medici-finance/assay#3396
run=ca035a175f23923903119a940e1328a1b7671ee51d79dac2536d56c1d279a901
source=6dfb616ef5463fa886e89ab7b07803d30006c58a
brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5
tier=strong. Each row was run by hand in the admitted detached home.

Verification-Attestation: medici-finance/assay#3396 run=ca035a175f23923903119a940e1328a1b7671ee51d79dac2536d56c1d279a901 source=6dfb616ef5463fa886e89ab7b07803d30006c58a brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | grep -rq '2688' on the issue-flow rulings register, then echo ok | prints ok | exit 0, prints `ok`. The R-8 section cites 2688, and its Sign-off line (line 36) is still empty. | 2026-10-18 verify-desk verifier (claude-opus-5-5) |
| 2 | deskautolane --help, then grep the lane config for the reconcile title | prints ok under mechanism A; under B, not applicable with the reason | NOT APPLICABLE under mechanism B. The delivery chose B, so no lane config entry exists and no lane config path was cited, and the row's placeholder has no value to substitute. Neighbour check: `deskautolane --help` exits 0. Outside workflows, tests, the runsheet diff and markdown, the reconcile title appears only in the deskreconcile verb sources. | 2026-10-18 verify-desk verifier (claude-opus-5-5) |
| 3 | forge read after the first scheduled reconcile tick following the merge | reconcile PR absent under B, or merged via deskautolane under A, with no reviewer verdict and no authorization-needed label | FAIL as observed. Scheduled assay-statusgen runs succeeded at 2026-10-18T09:32Z, 13:58Z and 17:19Z, and the reconcile still lands as a pull request. PR #3200 is `chore(board): reconcile`, author app/assay-board-writer, head board/reconcile, an OPEN draft created 2026-10-16T16:36Z with its newest commit at 2026-10-18T17:21Z and no reviews. #2651 was closed unmerged. The live workflow still has the step "Push board/reconcile and open or refresh its draft PR" (line 803). New since the 2026-10-15 pass: the runsheet diff no longer applies. `git apply --check` exits 1 at ci/staged-workflows/assay-statusgen.yml:478, and the reverse check fails too, so the diff is not already applied. A per-commit probe puts the break at d21ff890e (#3010). The diff's sha256 and size still match the pin. Attached to the ruling issue #2688 (comment 6254444767) as help wanted: re-serialize against current main, then the driver applies it. | 2026-10-18 verify-desk verifier (claude-opus-5-5) |
| 4 | build statusgen to a temp path, run --lint, grep LINT: PASS | prints ok | exit 0, prints `ok`. Lint tail is `LINT: PASS` (NOTICE lines only). The binary was built outside the home. | 2026-10-18 verify-desk verifier (claude-opus-5-5) |

**Risk-bearing value.** The enumeration covers the same delivery diff as the 2026-10-15 pass: the
changelog fragment, the rulings register, the issue-flow README, the staged-workflows README, the
runsheet with its diff file, and the stream index row. That diff introduces three literals: the
R-8 Sign-off line (empty, at the rulings register line 36), the diff pin (sha256 and 112,747 bytes,
at the runsheet lines 29-30), and the patch base ca0dad5c4 (runsheet line 34). Ranked: the Sign-off
line first, then the pin, then the base.

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at 6dfb616ef.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at 6dfb616ef: `shasum -a 256` and `wc -c` give 112747 bytes and the same digest.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's claim that they apply cleanly at the tip stopped holding at d21ff890e. Re-serializing against a new base is owed (help wanted on #2688), and a verifier cannot author it.

**VERIFY: FAIL** — row 3: the reconcile class still lands as PR #3200 after post-merge ticks. The direct-CI conversion is not applied, and its carried diff no longer applies to main (it breaks from d21ff890e), so it has to be re-serialized before the driver can apply it (tracked on #2688). Rows 1 and 4 PASS; row 2 is not applicable under mechanism B.

**Re-verify pass, 2026-10-19 (desk tools v1.0.50).** Run by a dispatched non-implementer verifier
(verify-desk) at merged main 50bef70d5. Each row was run by hand in the admitted detached home; the
statusgen binary was built outside it.

Verification-Attestation: medici-finance/assay#3441 run=19978fe5225c4c23ea3320bb252022c44580303b1d8324a0d55bda101daa3065 source=50bef70d518db5d73a6a2defd956f7bfcf346024 brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | grep -rq '2688' on the issue-flow rulings register, then echo ok | prints ok | exit 0, prints `ok`. The R-8 section cites 2688, and its Sign-off line (line 36) is still empty. | 2026-10-19 verify-desk verifier (claude-opus-5-5) |
| 2 | deskautolane --help, then grep the lane config for the reconcile title | prints ok under mechanism A; under B, not applicable with the reason | NOT APPLICABLE under mechanism B. The delivery chose B, so no lane config entry exists, no lane config path was cited, and the row's placeholder has no value to substitute. Neighbour check: `deskautolane --help` exits 0 (v1.0.50 build, sourceSHA 90198079). Outside workflows, ci, the runsheet and markdown, the reconcile title appears only in the deskreconcile verb sources and the statusgen reconcile job test. | 2026-10-19 verify-desk verifier (claude-opus-5-5) |
| 3 | forge read after the first scheduled reconcile tick following the merge | reconcile PR absent under B, or merged via deskautolane under A, with no reviewer verdict and no authorization-needed label | FAIL as observed. Scheduled assay-statusgen runs succeeded at 2026-10-18T20:37Z, 23:08Z and 2026-10-19T01:13Z, and the reconcile still lands as a pull request. PR #3200 (`chore(board): reconcile`, author app/assay-board-writer, head board/reconcile) is an OPEN draft with no reviews and no labels; its head 2e18aad90 (`chore(board): reconcile 2026-10-19`) was pushed at 2026-10-19T01:14:54Z by that tick. The live workflow still has the step "Push board/reconcile and open or refresh its draft PR" (line 811). Since 6dfb616ef, three commits changed the live and staged statusgen workflows (#3385, #3380, #3430), all regen-job fixes; none is the conversion. The runsheet diff still does not apply: `git apply --check` exits 1 at ci/staged-workflows/assay-statusgen.yml:478, and the reverse check exits 1, so the diff is not already applied. Its sha256 and size still match the pin. No re-serialized diff and no driver act have appeared on #2688 since the 2026-10-18 pass. | 2026-10-19 verify-desk verifier (claude-opus-5-5) |
| 4 | build statusgen to a temp path, run --lint, grep LINT: PASS | prints ok | exit 0, prints `ok`. The lint tail is `LINT: PASS`, with 0 PROBLEM lines (NOTICE lines only; desk-tools/36's four notices are the row-2 placeholder, no consumers row, an absent outcome, and a could-not-check risk×files cross-read for the missing Context files line). | 2026-10-19 verify-desk verifier (claude-opus-5-5) |

**Risk-bearing value.** The enumeration covers the same delivery diff (PR #2706) as the earlier passes.
That diff introduces three literals, ranked: the R-8 Sign-off line (empty, rulings register line 36),
the diff pin (sha256 and 112,747 bytes, runsheet lines 29-30), and the patch base ca0dad5c4
(runsheet line 34). No commit since 6dfb616ef has changed the rulings register or the runsheet.

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at 50bef70d5.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at 50bef70d5 with `shasum -a 256` and `wc -c`: same digest, 112747 bytes.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's clean-apply claim still fails at the tip. A re-serialization against a current base is still owed (help wanted on #2688), and a verifier cannot author it.

**VERIFY: FAIL** — row 3: the failure still holds at 50bef70d5 with the v1.0.50 tools. The 2026-10-19T01:13Z tick refreshed reconcile draft PR #3200. The direct-CI conversion is unapplied, and its carried diff still does not apply to main, so it must be re-serialized before the driver can apply it (tracked on #2688). Rows 1 and 4 PASS; row 2 is not applicable under mechanism B.

**Re-verify pass, 2026-10-21.** Run by a dispatched non-implementer verifier (verify-desk) at merged
main 8e7bce689. Each row was run by hand in the admitted detached home; the statusgen binary and
lint output were written outside it.

Verification-Attestation: medici-finance/assay#3892 run=120ad84da5241560abd836f50bfc963e0d7eee95b29decff994d63af02fd7efe source=8e7bce68999af4f6bf6c3151cc2103c3e55dd224 brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | prints ok | exit 0, prints `ok`. The R-8 section cites 2688; its Sign-off line (line 36) is still empty. | 2026-10-21 | assay-verifier-app[bot] @ 8e7bce68999af4f6bf6c3151cc2103c3e55dd224 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | prints ok under mechanism A; under B, not applicable with the reason | could-not-check: not applicable under mechanism B. The delivery chose B, so no lane config entry exists and no lane config path was cited; the placeholder has no value to substitute. Neighbour check: `deskautolane --help` exits 0 (build sourceSHA 0f6d392a). | 2026-10-21 | assay-verifier-app[bot] @ 8e7bce68999af4f6bf6c3151cc2103c3e55dd224 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `gh pr list -R medici-finance/assay --head board/reconcile --state all --limit 6 --json number,title,state,isDraft,author,mergedBy,labels` | reconcile PR absent under B, or merged via deskautolane under A, with no reviewer verdict and no authorization-needed label | FAIL as observed. Scheduled assay-statusgen runs succeeded at 2026-10-20T22:16Z, 2026-10-21T00:38Z, 05:59Z, 11:42Z and 17:07Z, and the reconcile still lands as a pull request. PR #3200 (`chore(board): reconcile`, author app/assay-board-writer) took dispatched reviewer verdicts (assay-reviewer-app CHANGES_REQUESTED twice on 2026-10-20, APPROVED 2026-10-21T09:38Z), carried the approval-needed label, and was merged at 2026-10-21T09:44Z by a human account, not via deskautolane. The 17:07Z tick then opened a new draft, PR #3873 (`chore(board): reconcile`, head 92ee58da9, `chore(board): reconcile 2026-10-21`), with no reviews yet. The live workflow still has the step "Push board/reconcile and open or refresh its draft PR" (line 823). The runsheet diff still does not apply: `git apply --check` exits 1 (fails on the live workflow at line 679 and the staged twin at line 478), and the reverse check exits 1, so it is not already applied. Its sha256 and size still match the pin. The ruling issue #2688 is open, last updated 2026-10-18; no re-serialized diff and no driver act. | 2026-10-21 | assay-verifier-app[bot] @ 8e7bce68999af4f6bf6c3151cc2103c3e55dd224 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | prints ok | exit 0, prints `ok`. Lint tail is `LINT: PASS`, 0 lines starting PROBLEM (NOTICE lines only; this brief's three notices are the row-2 placeholder, no consumers row, and an absent outcome). | 2026-10-21 | assay-verifier-app[bot] @ 8e7bce68999af4f6bf6c3151cc2103c3e55dd224 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

**Risk-bearing value.** The enumeration covers the same delivery diff (PR #2706) as the earlier passes:
the changelog fragment, the rulings register, the issue-flow README, the staged-workflows README, the
runsheet with its diff file, and the stream index row. It introduces three literals, ranked: the R-8
Sign-off line (empty, rulings register line 36), the diff pin (sha256 and 112,747 bytes, runsheet
lines 29-30), and the patch base ca0dad5c4 (runsheet line 34). No commit since 50bef70d5 has changed
the rulings register, the runsheet or its diff file.

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at 8e7bce689.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at 8e7bce689 with `shasum -a 256` and `wc -c`: same digest, 112747 bytes.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's clean-apply claim still fails at the tip. A re-serialization against a current base is still owed (help wanted on #2688), and a verifier cannot author it.

**VERIFY: FAIL** — row 3: at 8e7bce689 the reconcile class still lands as a pull request. PR #3200 consumed dispatched reviewer verdicts and a human merge, and the 2026-10-21T17:07Z tick opened PR #3873. The direct-CI conversion is unapplied and its carried diff still does not apply to main (tracked on #2688). Rows 1 and 4 PASS; row 2 is not applicable under mechanism B.

**Re-verify pass, 2026-10-21 (second).** Run by a dispatched non-implementer verifier (verify-desk) at
merged main af92fe827. Each row was run by hand in the admitted detached home; the statusgen binary and
lint output were written outside it. 46 commits landed since 8e7bce689; none changed the rulings
register, the runsheet, its diff file, or the live or staged statusgen workflow.

Verification-Attestation: medici-finance/assay#3984 run=0b37e3c671cffad39fc6b3c556044866f8f44bb20b7f45fcc497248b918a2912 source=af92fe827146129bc50af0df2ddb1cbc82fe0bfe brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | prints ok | exit 0, prints `ok`. The R-8 section cites 2688; its Sign-off line (line 36) is still empty. | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | prints ok under mechanism A; under B, not applicable with the reason | could-not-check: not applicable under mechanism B. The delivery chose B, so no lane config entry exists and no lane config path was cited; the placeholder has no value to substitute. Neighbour check: `deskautolane --help` exits 0 (v1.0.54 tools, sourceSHA 0f6d392a). | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `gh pr list -R medici-finance/assay --head board/reconcile --state all --limit 6 --json number,title,state,isDraft,author,mergedBy,labels` | reconcile PR absent under B, or merged via deskautolane under A, with no reviewer verdict and no authorization-needed label | FAIL as observed. Scheduled assay-statusgen runs succeeded at 2026-10-21T17:07Z and 20:36Z (and earlier), and the reconcile still lands as a pull request. PR #3873 (`chore(board): reconcile`, author app/assay-board-writer) is an OPEN draft, no reviews and no labels yet; its head 3cc4d0f35 (`chore(board): reconcile 2026-10-21`, committed 21:09Z) was refreshed by the 20:36Z tick. PR #3200 remains merged by a human account with the approval-needed label, not via deskautolane. The live workflow still has the step "Push board/reconcile and open or refresh its draft PR" (line 823). The runsheet diff still does not apply: `git apply --check` exits 1 (live workflow at line 679, staged twin at line 478), and the reverse check exits 1, so it is not already applied. Its sha256 and size still match the pin. Ruling issue #2688 is open; its newest comment is the prior verifier relay, with no re-serialized diff and no driver act. | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | prints ok | exit 0, prints `ok` (binary output directory pointed at verifier scratch outside the home). Lint exits 0, tail `LINT: PASS`, 0 lines starting PROBLEM (NOTICE lines only). | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

**Risk-bearing value.** The enumeration covers the same delivery diff (PR #2706) as the earlier passes:
the changelog fragment, the rulings register, the issue-flow README, the staged-workflows README, the
runsheet with its diff file, and the stream index row. It introduces three literals, ranked: the R-8
Sign-off line (empty, rulings register line 36), the diff pin (sha256 and 112,747 bytes, runsheet
lines 29-30), and the patch base ca0dad5c4 (runsheet line 34).

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at af92fe827.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at af92fe827 with `shasum -a 256` and `wc -c`: same digest, 112747 bytes.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's clean-apply claim still fails at the tip. A re-serialization against a current base is still owed (help wanted on #2688), and a verifier cannot author it.

**VERIFY: FAIL** — row 3: at af92fe827 the reconcile class still lands as a pull request; the 20:36Z tick refreshed draft PR #3873. The direct-CI conversion is unapplied and its carried diff still does not apply to main (tracked on #2688). Rows 1 and 4 PASS; row 2 is not applicable under mechanism B.

**Re-verify pass, 2026-10-22.** Run by a dispatched non-implementer verifier (verify-desk) at merged
main 05dc9fbc8, which equals forge main. The table below is the execution witness, written by
`statusgen verifyrun` from the admitted detached home on a clean tree; the prose after it records
the hand checks.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | could-not-run exit=- — unsubstituted placeholder(s) <the lane-config path cited in the PR body> — the row cannot run as written | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `READ FROM THE FORGE after the first scheduled reconcile tick following this brief's merge: the reconcile PR either does not exist (B: direct-CI landing on main) or exists merged by the reviewer App via deskautolane (A) with no dispatched reviewer verdict and no authorization-needed label.` | fail exit=2 | sha256:80dd39947a6e | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4050 run=bda3d59be9a5abf0c460a5ee00595eb1c06f9741df5f712029f0d86e63019254 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

**Hand checks at 05dc9fbc8.** 137 commits landed since af92fe827; none changed the rulings
register, the runsheet, its diff file, or the live or staged statusgen workflow. No row is classed
check:ci, so no container run was needed.

- Row 1: pass. The command exits 0 and prints `ok`. The register cites 2688 twice in the R-8
  section, and its Sign-off line (line 36) is still empty.
- Row 2: could-not-check: not applicable under mechanism B. The delivery chose B, so no lane
  config entry exists and no lane config path was cited; the placeholder has no value to
  substitute, and the witness records the row as could-not-run for that reason. Neighbour check:
  `deskautolane --help` exits 0 (build sourceSHA 0f6d392a).
- Row 3: FAIL as observed. Scheduled assay-statusgen runs succeeded at 2026-10-21T17:07Z, 20:36Z
  and 23:59Z, and the reconcile still lands as a pull request. PR #3873 (`chore(board): reconcile`,
  authored by the board-writer App) is an OPEN draft whose head b1140868c
  (`chore(board): reconcile 2026-10-22`, committed 2026-10-22T00:01Z) was pushed by the 23:59Z
  tick. It now carries the authorization-needed label and the dispatched-tier and dispatched-model
  labels, so it has entered the review lane; it has no reviews yet. PR #3200 remains merged by a
  human account, not via deskautolane. The live workflow still has the step "Push board/reconcile
  and open or refresh its draft PR" (line 823). The runsheet diff still does not apply:
  `git apply --check` exits 1 (live workflow at line 679, staged twin at line 478), and the
  reverse check exits 1, so it is not already applied. Ruling issue #2688 is open; its newest
  comment is the prior verifier relay, with no re-serialized diff and no driver act.
- Row 4: pass. The command exits 0 and prints `ok`; the binary was built to verifier scratch
  outside the home. A separate lint run with the same binary exits 0, tail `LINT: PASS`, 0 lines
  starting PROBLEM (NOTICE lines only; this brief's three notices are the row-2 placeholder, no
  consumers row, and an absent outcome).

**Risk-bearing value.** The enumeration covers the same delivery diff (PR #2706) as the earlier
passes: the changelog fragment, the rulings register, the issue-flow README, the staged-workflows
README, the runsheet with its diff file, and the stream index row. It introduces three literals,
ranked: the R-8 Sign-off line (empty, rulings register line 36), the diff pin (sha256 and 112,747
bytes, runsheet lines 29-30), and the patch base ca0dad5c4 (runsheet line 34).

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at 05dc9fbc8.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at 05dc9fbc8 with `shasum -a 256` and `wc -c`: same digest, 112747 bytes.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's clean-apply claim still fails at the tip. A re-serialization against a current base is still owed (help wanted on #2688), and a verifier cannot author it. Question filed: #4129.

**VERIFY: FAIL** — row 3: at 05dc9fbc8 the reconcile class still lands as a pull request; the 23:59Z tick refreshed draft PR #3873, which now carries the authorization-needed label. The direct-CI conversion is unapplied and its carried diff still does not apply to main (tracked on open issue #2688). Rows 1 and 4 PASS; row 2 is could-not-check (not applicable under mechanism B).
**Re-verify pass, 2026-10-27.** Run by a dispatched non-implementer verifier (verify-desk) at merged
main 0f2b208ce. The table below is the execution witness, printed by `statusgen verifyrun` (v1.0.63)
from the admitted detached home on a clean tree; the prose after it records the hand checks.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | could-not-run exit=- — unsubstituted placeholder(s) <the lane-config path cited in the PR body> — the row cannot run as written | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `READ FROM THE FORGE after the first scheduled reconcile tick following this brief's merge: the reconcile PR either does not exist (B: direct-CI landing on main) or exists merged by the reviewer App via deskautolane (A) with no dispatched reviewer verdict and no authorization-needed label.` | fail exit=2 | sha256:80dd39947a6e | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4678 run=f147ebbe24eab4523d622566c2eace7d1bb254aec4e01e310b3a0c7081d12688 source=0f2b208cebc30af31269e7482b8b9159b602867d brief=docs/streams/desk-tools/brief-36-reconcile-generated-class.md model=claude-opus-5-5 tier=strong

**Hand checks at 0f2b208ce.** No row is classed check:ci, so no container run was needed. No row
is risk-bearing (all four risk fields are `no` and no row carries a risk-bearing, live, mutating
or end-to-end tag), so no fail-first run was required. Since 05dc9fbc8, no commit changed the
rulings register, the runsheet or its diff file. Four commits changed the live or staged statusgen
workflow (877716f60, 0c87f6511, 53c285cf3, 689d461d0); none of them is the direct-CI conversion.

- Row 1: pass. The command exits 0 and prints `ok`. The register cites 2688 twice in the R-8
  section, and its Sign-off line (line 36) is still empty.
- Row 2: could-not-check: not applicable under mechanism B. The delivery chose B, so no lane
  config entry exists and no lane config path was cited; the placeholder has no value to
  substitute, and the witness records the row as could-not-run for that reason. Neighbour check:
  `deskautolane --help` exits 0 (build sourceSHA 125611dfb).
- Row 3: FAIL as observed. The witness's exit 2 comes from running the prose cell as a shell
  command, so it is not the observation; the forge read is. Scheduled assay-statusgen runs
  succeeded at 2026-10-27T02:01Z, 14:25Z and 20:03Z, and the reconcile still lands as a pull
  request. PR #3873 (`chore(board): reconcile`, authored by the board-writer App) is an OPEN
  draft whose head c23e7fd29 (`chore(board): reconcile 2026-10-27`, committed
  2026-10-27T20:05Z) was pushed by the 20:03Z tick. It carries the authorization-needed label
  and the dispatched-tier and dispatched-model labels, and it has two dispatched reviewer
  verdicts (CHANGES_REQUESTED, 2026-10-27T21:46Z). So the class still consumes a reviewer slot
  and a human gate. PR #3200 remains merged by a human account, not via deskautolane. The live
  workflow still has the step "Push board/reconcile and open or refresh its draft PR" (line 825).
  The runsheet diff still does not apply: `git apply --check` exits 1 (live workflow at line
  679, staged twin at line 478), and the reverse check exits 1, so it is not already applied.
  Ruling issue #2688 is open, last updated 2026-10-21; question #4129 (re-derive the runsheet
  base) is open with no comments. No re-serialized diff and no driver act has appeared.
- Row 4: pass. The command exits 0 and prints `ok`; the binary was built to verifier scratch
  outside the home.

**Risk-bearing value.** The enumeration covers the same delivery diff (PR #2706, squash commit
2c1653f35, parent edc387436) as the earlier passes: the changelog fragment, the rulings register,
the issue-flow README, the staged-workflows README, the runsheet with its diff file, and the
stream index row. It introduces three literals, ranked: the R-8 Sign-off line (empty, rulings
register line 36), the diff pin (sha256 and 112,747 bytes, runsheet lines 29-30), and the patch
base ca0dad5c4 (runsheet line 34).

- RISK-VALUE: DERIVED — R-8 Sign-off = (empty) @ docs/streams/issue-flow/rulings.md:36. Mechanism B keeps the class out of the lane, so nothing may be enacted, and the line is still empty at 0f2b208ce.
- RISK-VALUE: DERIVED — reconcile-lane-hunk.diff sha256 = f4e3e0df93e1d7d47088021c61cf2e232cd4b9ca91564e919cb8687f9a866cd8, size = 112,747 @ docs/runsheets/reconcile-lane-hunk.md:29-30. Recomputed at 0f2b208ce with `shasum -a 256` and `wc -c`: same digest, 112747 bytes.
- RISK-VALUE: NAMED, NOT DERIVED — runsheet base = ca0dad5c4162a7203167340c49d4c241e14060bb @ docs/runsheets/reconcile-lane-hunk.md:34. The base is accurate for the bytes, but the runsheet's clean-apply claim still fails at the tip. A re-serialization against a current base is still owed (open question #4129, ruling issue #2688), and a verifier cannot author it.

**VERIFY: FAIL** — row 3: at 0f2b208ce the reconcile class still lands as a pull request; the 2026-10-27T20:03Z tick refreshed draft PR #3873, which carries the authorization-needed label and two dispatched reviewer verdicts. The direct-CI conversion is unapplied and its carried diff still does not apply to main (tracked on open issues #2688 and #4129). Rows 1 and 4 PASS; row 2 is could-not-check (not applicable under mechanism B).

## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the lane
membership exactly the ruled class (title + author), with no widening? (2) does the R-8
enactment line cite #2688 and leave the auto-lane inert until its own sign-off discipline
is satisfied? (3) are the class's own guards (self-hold, witness discipline, branch
protection) demonstrably untouched? (4) if mechanism B was chosen, does the runsheet carry
the exact diff, and does the worker's diff touch zero workflow files? (5) is #2651
untouched by this change?
