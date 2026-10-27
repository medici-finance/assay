---
brief: assay:assay:graph-execution:32
title: Investigate criterion-level evidence freshness
why: >-
  Evidence can still answer an older question after the governing acceptance criterion
  changes. Establish whether existing Assay controls expose that mismatch before adding
  another identity, mapping or freshness mechanism.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 6366a15f-eb0e-4917-8da8-bd5383096962
authored: 2026-10-16 by worker-desk (author-brief)
sources:
  - spec/registers-v1.md §6.4–6.5 — requirement citations do not establish criterion coverage
  - spec/lifecycle-v1.md §2.4 — revision and Verify-row binding of execution evidence
  - statusgen/requirements.go and statusgen/traceability.go — current requirement and brief join
  - statusgen/evaluation/coverage.go — existing Command, Expect and dependency applicability guards
  - docs/streams/graph-execution/brief-28-verifier-domain-contracts.md — planned domain owner; not a delivered prerequisite
  - https://github.com/Ask-Loft/loft/blob/9bed930a32cd6bdc09a7274f45a4c665daf305de/crates/loft-cli/skill/loft-spec/references/reconciling.md
  - freshness-checked 2026-10-16 @ d0580c5e5 — fresh origin/main; requirement criteria remain string entries, not independently keyed records
exec-tier: strong
exec-tier-why: Requires semantic coverage judgments across requirements, Verify rows, source revisions and execution witnesses.
domain: complicated
---

# Brief 32 — Investigate criterion-level evidence freshness

## Context

files: `docs/research/criterion-evidence-freshness/README.md` (planned),
`docs/research/criterion-evidence-freshness/cases.json` (planned),
`docs/research/criterion-evidence-freshness/replay.py` (planned),
`docs/research/criterion-evidence-freshness/results.json` (planned),
`changelog/criterion-evidence-freshness-study.md` (planned).

facts (d0580c5e5, 2026-10-16):
- `spec/registers-v1.md` §6 defines the existing requirement owner: `REQ-<slug>`, string
  acceptance criteria, `satisfies:` links and satisfaction claims. A link is not coverage.
- `statusgen/traceability.go` joins requirement references to brief status and Evidence;
  it does not define a criterion-to-Verify-row identity contract.
- `statusgen/evaluation/coverage.go` already guards changed Command/Expect text and source,
  policy, build and environment applicability. A requirement edit may already be caught
  conservatively by those inputs; demonstrate any residual gap rather than assume one.
- `statusgen/coverage_test.go` has tests for changed acceptance definitions and Expect text;
  `statusgen/coverage_inputs_test.go` covers dependency applicability. These are baseline controls.
- Loft's documented trace check stores a reviewed node hash and searches for test names;
  it does not execute tests. This is a comparison technique, not an adopted dependency.

The question is whether an upstream acceptance edit, with an unchanged Verify row and
unchanged test name, can leave a misleading current-satisfaction claim. Separate historical
proof of the old intent from acceptance of revised intent. No claim that a gap exists is
made by this brief. Inspect planned verifier-domain contracts /28–31 for ownership overlap,
but do not wait for those implementations to conduct this source-and-fixture investigation.

## Design fit

design-fit:
  owner: existing requirement register and coverage evaluator retain their meanings
  contract: none — investigation proposes no new normative contract
  retires: []
  weight: verbs 0, flags 0, refusals 0, runtime rule-text lines 0
  why-add: n/a; research artifacts compare extension of existing owners with no-change

## Read first

- `spec/registers-v1.md` §6, `spec/lifecycle-v1.md` §2.4, `spec/brief-v1.md` §7.
- `statusgen/requirements.go`, `traceability.go`, `statusgen/evaluation/coverage.go`,
  `statusgen/evaluation/coverage_api.go` and the baseline tests named in Context.
- `docs/streams/graph-execution/work-input-amendment.md` and /28–31's Context/Task
  contracts for the proposed domain and applicability owners. Their proposed state is
  not evidence of a shipped control.

## Ground rules

- Offline, synthetic fixtures in temporary repositories only; no production, provider,
  telemetry, installation, Git configuration or authority changes.
- Do not change runtime code, schemas, gates, lifecycle states, consumer pins or live skills.
- Preserve original evidence subjects and unknowns. A hash match, test-name match or
  model assessment never substitutes for an execution witness or semantic review.
- Stop at implemented; independent verification owns verified/done. Any implementation
  or schema proposal needs a separately reviewed follow-on.

## Task

1. Trace the actual ask → acceptance criterion → brief → Verify row → execution witness
   path at an immutable Assay revision. State which links are authored, checked, executed
   or absent, with source locations and exact commands. Compare the documented contracts
   with observed behaviour; unavailable evidence is could-not-check. Inspect existing
   tests and report which already cover each suspected failure mechanism.
2. Freeze `cases.json` before comparing alternatives. Include at least twelve explicit
   cases: criterion tightening; a new criterion; wording-only edit; criterion reorder;
   split/merge with duplicate text; retirement; inherited-constraint change; changed
   Command; changed Expect; a weakened test retaining its name; an unrelated edit; and
   an unavailable governing source. Give each an independent expected disposition and
   rationale before obtaining results. State whether it requires fresh semantic review,
   fresh execution, both, neither, or an explicit unknown. IDs here identify study cases,
   not a proposed production criterion scheme.
3. Deliver a standard-library Python replay driver that builds/invokes the real existing
   Assay instruments against those synthetic repositories, records exact commands,
   revisions, exit codes, results and tool versions, and compares saved results with a
   fresh replay. Keep build work offline and caches/temp files outside tracked content.
   No fabricated witness counts as executed evidence: distinguish parser fixtures from
   actual command runs. Observe baseline behaviour first. Where a gap remains, compare
   a bounded experimental mapping using existing requirement references/Verify-row IDs
   with the baseline; keep the experimental prediction separate from observed baseline
   results and do not wire it into the product. Reuse existing fixtures/APIs where possible.
4. Write the decision report: prefer no-change or clearer review guidance if adequate;
   otherwise compare conservative requirement-version binding with criterion-level
   identity/mapping. Address semantic versus cosmetic edits, reorder/duplicates,
   split/retirement, inherited obligations, mapping review and weakened tests. Identify
   invalidation scope, unknown-source handling, backward compatibility, author/reviewer
   effort and conflicts with /28–31. A hash alone cannot decide semantic equivalence.
   Recommend the smallest justified extension to existing owners, or explicitly no
   extension. Reconcile any difference between a satisfied register entry and actual
   current acceptance; do not silently redefine satisfaction or demote historical records.
5. Keep the source inventory, case rationale, raw replay results, decision and limits
   together in the research README. Record exact commands to reproduce with no Loft
   dependency. If follow-on work is justified, scope it and its authorization questions;
   do not implement or create a new gate, register or status system. Add the changelog.

## Interface contract

Inputs: immutable source revision, fixed synthetic cases and explicit expected dispositions.
Outputs: source-backed report, reproducible baseline observations, separately labelled
experimental predictions and a no-change / guidance-only / bounded-extension recommendation.
This is research evidence; it grants no implementation, approval or deployment authority.

## Verify

The replay driver and artifacts below are future deliverables, not commands claimed to pass
at authoring. The driver must execute each declared case, reject skipped/duplicate cases and
retain could-not-check rather than silently certifying unavailable evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference +flow | `python3 docs/research/criterion-evidence-freshness/replay.py --check` | exit 0; fresh actual baseline replay matches committed results for every declared case; source revision, commands and execution observations resolve; report-only predictions are distinguishable from measured outcomes |
| 2 | check +mutation | `python3 docs/research/criterion-evidence-freshness/replay.py --self-test` | exit 0; controls demonstrate nonzero rejection of tampered saved observations, skipped cases and a fabricated execution claim; unavailable source remains an explicit unknown rather than current acceptance |
| 3 | check +dereference +neighbour | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCoverageAcceptanceDigestChanged$' . > "$out" && GOWORK=off go test -count=1 -v -run '^TestCoverageExpectChangedSinceWitnessRan$' . >> "$out" && grep -F -- '--- PASS: TestCoverageAcceptanceDigestChanged ' "$out" && grep -F -- '--- PASS: TestCoverageExpectChangedSinceWitnessRan ' "$out")` | exit 0; both named baseline controls run and pass; report credits their existing protection rather than presenting it as a new criterion-freshness mechanism |

## Definition of Done

The independent reviewer can reproduce the cases and distinguish current behaviour,
experimental prediction, semantic judgment and unknowns. Every claimed uncovered mechanism
has a source-backed reproducer; a no-gap result is valid. The report names costs and the
owning extension point without enacting its recommendation. No existing record is rewritten
to make evidence fit a new requirement. No docs-site generation applies to these research-only
artifacts; executable results and report stay together.

## Pre-mortem

| Failure mode | Detection |
|---|---|
| An assumed gap is already caught by conservative applicability | rows 1 and 3 exercise actual baseline controls; independent review checks attribution |
| A polished report uses guessed results or omits inconvenient cases | rows 1 and 2 rerun observations and reject tampering/skips |
| A test-name link is presented as proof that the revised behaviour ran | row 2 rejects fabricated execution; review judges semantic adequacy against each case |
| Investigation silently introduces a new production gate or identity schema | review-only: inspect the complete diff against the research-only files contract |
| Case labels or a hash are mistaken for a semantic oracle | review-only: adjudicate the predeclared expected dispositions and boundary claims |

## Evidence

<!-- Independent verifier records each row, exact source revision, command, exit and observed result. -->

Verification-Attestation: medici-finance/assay#3862 run=524386b83e92546447a90179cf2476d1a138bd8c320f7209d646166240913a69 source=e16c3ed49c1ea518b5c131335dc5d926505410a2 brief=docs/streams/graph-execution/brief-32-investigate-criterion-level-evidence-freshness.md model=claude-opus-5-5 tier=strong

Run against merged main at the attested source, offline (KUBECONFIG=/dev/null), go1.27.2 darwin/arm64, python3; replay scratch under the runner's TMPDIR, outside the worktree.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `python3 docs/research/criterion-evidence-freshness/replay.py --check` | exit 0; fresh actual baseline replay matches committed results for every declared case; report-only predictions distinguishable | exit 0; "CLI JSON byte commitments: 17/42 identical; semantic replay matches"; "14 declared cases executed; original witnesses retain their subjects"; validate() enforces source revision, one executed verifyrun witness per case and kind=report-only for every experimental_prediction; unavailable case reports numeric=could-not-check | 2026-10-21 | assay-verifier-app[bot] @ e16c3ed49c1ea518b5c131335dc5d926505410a2 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `python3 docs/research/criterion-evidence-freshness/replay.py --self-test` | exit 0; nonzero rejection of tampered observations, skipped cases and fabricated execution; unavailable source stays unknown | exit 0; "identical-observations: accepted exit=0"; 8 planted controls each "rejected exit=1": tampered-observation, skipped-case, duplicate-case, fabricated-execution, unknown-as-pass ("unavailable governing source is not current acceptance"), prediction-as-measured, wrong-raw-commitment, changed-decoded-value | 2026-10-21 | assay-verifier-app[bot] @ e16c3ed49c1ea518b5c131335dc5d926505410a2 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCoverageAcceptanceDigestChanged$' . > "$out" && GOWORK=off go test -count=1 -v -run '^TestCoverageExpectChangedSinceWitnessRan$' . >> "$out" && grep -F -- '--- PASS: TestCoverageAcceptanceDigestChanged ' "$out" && grep -F -- '--- PASS: TestCoverageExpectChangedSinceWitnessRan ' "$out")` | exit 0; both baseline controls run and pass; report credits existing protection | exit 0; "--- PASS: TestCoverageAcceptanceDigestChanged (0.02s)"; "--- PASS: TestCoverageExpectChangedSinceWitnessRan (0.45s)"; research README section "Source inventory and the actual chain" credits these as the brief's row-3 existing controls, not a new mechanism | 2026-10-21 | assay-verifier-app[bot] @ e16c3ed49c1ea518b5c131335dc5d926505410a2 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Rows: 3/3 passed, 0 could-not-check.

Risk-bearing value enumeration (delivering change: the research commit for this brief, merged via PR #3307; scope = the five research/changelog files it added). Literals introduced: minimum case count 12 (replay.py line 473); JSON captures per case 3 (replay.py line 363); subprocess timeouts 60 / 30 / 120 / 10 seconds (replay.py lines 126, 332, 338, 447 and 455); pinned study source commit (cases.json line 3, an ancestor of the verified head). Ranked: all are reversible research-harness knobs; no runtime code, schema, gate or authority is changed (design-fit weight 0, risk all "no", not irreversible). Top entry derived:

RISK-VALUE: DERIVED — minimum declared cases = 12 @ docs/research/criterion-evidence-freshness/replay.py:473 — Task 2 of this brief enumerates exactly twelve mandatory case kinds (tightening, new, wording, reorder, split/merge, retirement, inherited, Command, Expect, weakened test, unrelated, unavailable), so 12 is the contract floor; the frozen set has 14 distinct ids, and 3 captures per case at line 363 matches the three statusgen --json calls each case issues (42 = 3 x 14 observed in row 1).

**VERIFY: PASS** — 3/3 rows at the attested source

### 2026-10-22 verify pass (source 05dc9fbc88f2, witnessed)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 docs/research/criterion-evidence-freshness/replay.py --check` | pass exit=0 | sha256:b688f1510c3a | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `python3 docs/research/criterion-evidence-freshness/replay.py --self-test` | pass exit=0 | sha256:0e10d12bd0ad | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCoverageAcceptanceDigestChanged$' . > "$out" && GOWORK=off go test -count=1 -v -run '^TestCoverageExpectChangedSinceWitnessRan$' . >> "$out" && grep -F -- '--- PASS: TestCoverageAcceptanceDigestChanged ' "$out" && grep -F -- '--- PASS: TestCoverageExpectChangedSinceWitnessRan ' "$out")` | pass exit=0 | sha256:d3d75f410e8f | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4031 run=0501d7379d0d35ebed3798c61a787a7dd2064c1e4e82b487ecc182b8e40ec085 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/graph-execution/brief-32-investigate-criterion-level-evidence-freshness.md model=claude-opus-5-5 tier=strong

The table above is the `statusgen verifyrun` execution witness (write mode, statusgen
v1.0.54), run once from the admitted detached home at source
05dc9fbc88f2b1b23cd78bebc8634c564327eca4 on a clean tree; all three rows ran on the
host under the witness and none was could-not-run, so every row rests on the witness.
The verifier then re-ran the same three commands by hand at the same source, only to
read the output the witness hashes, and compared it with each Expect cell:

- Row 1 (exit 0): the driver printed 14 declared case lines (unchanged, tighten, add,
  wording, reorder, split-duplicates, retire, inherited, command, expect, weaken-test,
  unrelated, unavailable, declared-tighten), each with a measured baseline result kept
  separate from a `prediction=` field, then "14 declared cases executed; original
  witnesses retain their subjects" and "semantic replay matches". The saved
  source_commit in results.json (2f56cf2892af) is an ancestor of the verified source.
  The unavailable case reports numeric=could-not-check with prediction=unknown.
- Row 2 (exit 0): identical observations accepted; tampered-observation, skipped-case,
  duplicate-case, fabricated-execution, unknown-as-pass, prediction-as-measured,
  wrong-raw-commitment and changed-decoded-value were each rejected with exit 1 and a
  named reason.
- Row 3 (exit 0): "--- PASS: TestCoverageAcceptanceDigestChanged" and
  "--- PASS: TestCoverageExpectChangedSinceWitnessRan" from the statusgen package.

Rows 1-3: 3/3 pass on the witness; the hand runs add no row credit and only confirm
the Expect wording.

Risk-bearing value. The risk frontmatter is all no and nothing is irreversible. The
enumeration covered the delivering change's diff (the five research files, the changelog
line, one CLI-migration exclusion entry and two desk-tool test files). Literals found: the
minimum case count 12 in the driver's case guard (replay.py line 473), and the subprocess
timeouts 60, 30, 120 and 10 seconds plus the witness `--timeout 10s` (replay.py lines 126,
332, 338, 447, 455, 297). The timeouts are reversible operational knobs, ranked last and
not derived.

RISK-VALUE: DERIVED — len(ids) < 12 minimum = 12 @ docs/research/criterion-evidence-freshness/replay.py:473 — Task 2 of this brief requires at least twelve explicit cases and lists exactly twelve required categories; the guard refuses fewer and refuses duplicates, and cases.json declares 14.

**VERIFY: PASS** — 3/3 rows pass on the `statusgen verifyrun` witness at 05dc9fbc88f2; no row rests only on a hand run.
The table above is the `statusgen verifyrun` execution witness (write mode, statusgen
v1.0.63), run once from the admitted detached home at source
47ef38f961aa1006ff296034d118630efdc177cb on a clean tree, offline, go1.27.2 darwin/arm64,
Python 3.15.0. All three rows ran on the host under the witness with exit 0, so every row
rests on the witness. The delivering change is the squash merge of #3307
(83c6988d097d), whose parent is 379310083ad5; the five research and changelog files it
added are byte-unchanged between that merge and the verified source. A
`statusgen verifyrun --fail-first --base 379310083ad58b10fcc41bed2e537925823bfbc7`
rehearsal reported that this table carries no risk-bearing row (the risk frontmatter is
all no and no row is tagged risk-bearing, live, mutating or end-to-end), so no base run is
owed.

The verifier then re-ran the same three commands by hand at the same source, only to read
the output the witness hashes, and compared it with each Expect cell:

- Row 1 (exit 0): "CLI JSON byte commitments: 17/42 identical; semantic replay matches",
  then 14 declared case lines (unchanged, tighten, add, wording, reorder,
  split-duplicates, retire, inherited, command, expect, weaken-test, unrelated,
  unavailable, declared-tighten), each with a measured baseline result kept separate from
  a `prediction=` field, then "14 declared cases executed; original witnesses retain
  their subjects". The unavailable case keeps an explicit unknown with
  prediction=unknown. The saved source_commit in cases.json (2f56cf2892af) is an ancestor
  of the verified source.
- Row 2 (exit 0): "identical-observations: accepted exit=0"; tampered-observation,
  skipped-case, duplicate-case, fabricated-execution, unknown-as-pass ("unavailable
  governing source is not current acceptance"), prediction-as-measured,
  wrong-raw-commitment and changed-decoded-value were each rejected with exit 1 and a
  named reason.
- Row 3 (exit 0): "--- PASS: TestCoverageAcceptanceDigestChanged" and
  "--- PASS: TestCoverageExpectChangedSinceWitnessRan" from the statusgen package.

Rows 1-3: 3/3 pass on the witness; the hand runs add no row credit and only confirm the
Expect wording.

Risk-bearing value. The risk frontmatter is all no, nothing is irreversible, and the
design-fit weight is zero. The enumeration covered the delivering change's whole diff
(the five research and changelog files, one CLI-migration exclusion entry and two
desk-tool test files). Literals found: the minimum case count 12 and the duplicate
refusal in the driver's case guard (replay.py line 473); three JSON captures per case
(replay.py line 363); the subprocess timeouts 60, 30, 120 and 10 seconds plus the witness
`--timeout 10s` (replay.py lines 126, 332, 338, 447, 455 and 297); and the pinned study
source commit (cases.json line 3). The timeouts are reversible research-harness knobs,
ranked last and left underived; the capture count is fixed by the three statusgen JSON
calls each case issues (42 = 3 x 14 in row 1).

RISK-VALUE: DERIVED — len(ids) < 12 minimum = 12 @ docs/research/criterion-evidence-freshness/replay.py:473 — Task 2 of this brief requires at least twelve explicit cases and lists exactly twelve required categories; the guard refuses fewer and refuses duplicates, and cases.json declares 14.

**VERIFY: PASS** — 3/3 rows pass on the `statusgen verifyrun` witness at 47ef38f961aa; no row rests only on a hand run.

## Review

Gate: model; synthetic research and a recommendation only. Prioritize contradictions and
unsupported coverage claims. Review expected dispositions independently; structural lint
cannot establish that a case's semantic oracle or the recommendation is adequate.
