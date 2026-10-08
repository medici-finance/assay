---
brief: assay:assay:measured-status:02
title: "Derive MinCorpus for the learned riskscore model against its 15-feature events-per-variable floor, or record the rationale — and pin it with a test"
why: >-
  The learned defect-prediction model trusts itself over the heuristic fallback once the
  labeled corpus reaches MinCorpus=40, but standard events-per-variable guidance for a
  15-feature model would put that floor near 150. Either 40 is a deliberately conservative
  under-threshold (safe because the heuristic absorbs the gap) or it is an un-derived guess
  that graduates the model too early; either way the value must be computed from the feature
  count, not typed as a bare literal.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1171]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#1171 — MinCorpus=40 (learned.go) not derived against the 15-feature model's events-per-variable guidance"
  - "qualgen/riskscore/learned.go — Config.MinCorpus and DefaultConfig (MinCorpus:40); the under-corpus three-state threshold below which the score is heuristic-only"
  - "qualgen/riskscore/features.go — the 15-element feature vector MinCorpus must be derived against"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — MinCorpus:40 is a bare literal in DefaultConfig with no events-per-variable derivation in code or the quality/15 brief"
exec-tier: strong
exec-tier-why: derives a model-graduation threshold from the feature vector — an error graduates the learned model early or never, and it compounds through every score consumed downstream
domain: complicated
value: med
consumers:
  - "qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here"
version: 1
id: 81c2396f-a641-49ce-ae17-9683f990dfa2
---

# Brief 02 — Derive MinCorpus against the feature vector

## Context
files:
- `qualgen/riskscore/learned.go` — `Config.MinCorpus`, `DefaultConfig()`.
- `qualgen/riskscore/features.go` — the feature vector whose length is the derivation input.
- `qualgen/riskscore/learned_test.go` — add the derivation-pinning test.
facts:
- feature vector length today = 15 (`features.go`); MinCorpus today = 40 (`DefaultConfig`).
- events-per-variable (EPV) is the standard floor: `MinCorpus >= EPV * featureCount`; a
  conservative logistic-model EPV is ~10, giving ~150 for 15 features.
- below MinCorpus the model does not train and the score is emitted heuristic-only with a
  could-not-learn status (a three-state read), so an under-set floor over-trusts the learned
  model and an over-set one keeps the heuristic longer — the value is a real tradeoff, not cosmetic.
- `qualgen/riskscore` is buildable/testable from fixtures without a live corpus.
- `qualgen` is its own Go module (`qualgen/go.mod`, no root `go.mod` in this repo); tests and
  builds run from `qualgen/` (`cd qualgen && go test ./riskscore/ …`), not from the repo root.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Derive MinCorpus from the feature count. Preferred: make MinCorpus a function of
   `len(features)` and a named `EventsPerVariable` constant (`MinCorpus = EventsPerVariable *
   featureCount`), so the floor tracks the vector automatically when a feature is added or
   removed. Choose and DOCUMENT the EPV value with its rationale.
2. If the team's decision is that 40 is a deliberately conservative under-threshold justified
   by the heuristic fallback (rather than the EPV floor), then instead record that rationale
   as a `// Derivation:` block on `MinCorpus` naming why the EPV floor is intentionally not
   applied and what condition would raise it — and add the same pinning test against the
   documented value. Do not leave a bare literal either way.
3. Add a test `TestMinCorpusDerivedFromFeatureCount` (planned) that recomputes the floor from
   `len(features)` and the documented EPV (or asserts the documented conservative value and
   its stated invariant), and fails if `MinCorpus` drifts from its derivation.
4. Add a FLOW test `TestMinCorpusGovernsLearnedSwitch` (planned) that trains against a corpus
   one below the derived floor (asserts heuristic-only / could-not-learn) and one at the floor
   (asserts the learned model trains), proving the derived value actually governs the
   heuristic↔learned switch end to end — MinCorpus is a shared default consumed by the Train
   under-corpus gate.
5. **Fail-first (rule 9).** Before landing, temporarily set the derived/documented value one
   below its own correct output (e.g. drop `EventsPerVariable` by one) and confirm
   `TestMinCorpusDerivedFromFeatureCount` (planned) goes red; restore and confirm green. Record the
   red-then-green run under `## Evidence` (this repo has no `mutations.json` harness under
   `qualgen/`, so the proof is a one-time hand-mutation, not a corpus entry — same convention
   used across the repo's other `Fail-first (rule 9)` sections, e.g.
   `docs/streams/apps-installer/brief-01-role-app-indirection.md`).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | exit 0 (the test's own `--- PASS:` line is present, so a renamed or missing test cannot pass vacuously) | check +dereference |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | exit 0 (a corpus one below the derived floor stays heuristic-only/could-not-learn and one at the floor trains — the value actually governs the switch end to end) | check +flow |
| 3 | `cd qualgen && go build ./riskscore/` | exit 0 | check |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | exit 0 (a written derivation exists next to the value) | check |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | exit 0; output does not contain "DISPROVED" (the fixed-here consumer routing is corroborated, not contradicted) | check |

## Evidence
Option taken (implementer record, 2026-10-02): **Task 1 — derive the floor.**
`EventsPerVariable = 10` and `DerivedMinCorpus()` = `EventsPerVariable * len(FeatureNames())`
= 10 x 15 = 150; `DefaultConfig().MinCorpus` calls it. The `// Derivation:` comments on the
constant and on `DerivedMinCorpus` state that the floor counts total labeled examples (both
classes) per predictor, so meeting it is a necessary, weaker condition than the EPV rule,
which counts rarer-outcome events per predictor; a rarer-outcome event floor is not added
here (#1171 stays open for that question).

Fail-first (Task 5), implementer's hand-mutations of `qualgen/riskscore/learned.go`, run from
`qualgen/` against the working tree on top of `d67b2128b4c7`, each restored from a saved
copy (`cmp` identical) before the next:

| Mutation | Test | Red (quoted) | Green after restore |
|---|---|---|---|
| A — `EventsPerVariable` 10 -> 9 (the derived value one below its correct output) | `TestMinCorpusDerivedFromFeatureCount` | `--- FAIL: TestMinCorpusDerivedFromFeatureCount` / `learned_test.go:330: EventsPerVariable = 9, documented derivation says 10` | `--- PASS` |
| B — `Train` gate `len(examples) < 40` instead of `< cfg.MinCorpus` | `TestMinCorpusGovernsLearnedSwitch` | `--- FAIL: TestMinCorpusGovernsLearnedSwitch` / `learned_test.go:358: Train with 149 examples (floor 150) must refuse as under-corpus, got <nil>` | `--- PASS` |

After restoring, the full package passes (`go test ./riskscore/ -count=1`: `ok`), and
`go vet ./riskscore/` and `gofmt -l riskscore` are clean.

Verify row 2 re-authored (check-definition fix, same assertion, narrower match): as first
written, `... -v 2>&1 | grep -q 'PASS'` exits 141 on 3 of 3 runs under `bash -o pipefail`,
the shell `verifyrun` runs every row in — `grep -q` exits at the first match and `go test`
takes SIGPIPE. The row now reads the whole stream with `grep -c` and matches the test's own
`--- PASS:` line: exit 0 on 3 of 3 runs at this head, and exit 1 with mutation B applied
(prints `0`), so the row goes red when the switch is not governed by the floor.

Implementer's run of the Verify table follows as an execution witness (`statusgen
verifyrun`). The independent verifier re-runs on merged main; this record does not set
`verified`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run TestMinCorpusDerivedFromFeatureCount -count=1` | pass exit=0 | sha256:19a150b66c99 | 2026-10-02 | assay-worker-app[bot] @ ecca15f0e94d (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run TestMinCorpusGovernsLearnedSwitch -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-worker-app[bot] @ ecca15f0e94d (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-worker-app[bot] @ ecca15f0e94d (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-worker-app[bot] @ ecca15f0e94d (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | pass exit=0 | sha256:024a79653f52 | 2026-10-02 | assay-worker-app[bot] @ ecca15f0e94d (on-behalf-of human:ian) (forge-identity) |

Verify rows 1 and 2 re-authored again (same class, check-definition): `statusgen --root .
--lint` raised `gotest-run-vacuous` on row 1 — `go test -run <name>` exits 0 with `[no tests
to run]` when the test is missing. Reproduced: with `TestMinCorpusDerivedFromFeatureCount`
renamed in a scratch edit, the old row 1 printed `ok ... [no tests to run]` and exited 0.
Row 1 now asserts the test's own `--- PASS:` line like row 2, and both selectors are
anchored (`^...$`). Under `bash -o pipefail`: row 1 exit 0 at this head, exit 1 with
mutation A applied, exit 1 with the test renamed; row 2 exit 0 at this head, exit 1 with
mutation B applied. Every scratch edit was restored before the witness below, which
supersedes the table above for rows 1 and 2 (that table stays as the log of what ran).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-worker-app[bot] @ 65ea85b7ff98 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-worker-app[bot] @ 65ea85b7ff98 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-worker-app[bot] @ 65ea85b7ff98 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-worker-app[bot] @ 65ea85b7ff98 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | pass exit=0 | sha256:b1e69629fd94 | 2026-10-02 | assay-worker-app[bot] @ 65ea85b7ff98 (on-behalf-of human:ian) (forge-identity) |
### Non-implementer verifier run — 2026-10-06 claude-opus-5-5[1m]

Verified SHA: merged main `fc5afe37f9e48681b39007ef694ac73fc4ef562e` (implementation landed in a11ac4167, #2054; squash parent a7b4bed48). The runner is not the implementer. It ran in its own detached temporary worktree cut from origin/main, run time 2026-10-05 ~20:45Z UTC. Toolchain: host go1.27.1 darwin/arm64 (the qualgen module pins go 1.25.0), statusgen v1.0.32. Offline: `KUBECONFIG=/dev/null`, no endpoint contacted. Every row ran under `bash -o pipefail`.

Grounding, written from the brief text before the implementer's tests were read: Task 1 expects a named `EventsPerVariable` constant of about 10, with `MinCorpus = EventsPerVariable * len(features)`. The tree has 15 features (`FeatureNames()` in `qualgen/riskscore/features.go` lists 15 names, and `JITFeatures.Vector()` returns 15 entries), so the expected floor is 10 x 15 = 150. The tree carries exactly that. `EventsPerVariable = 10`, `DerivedMinCorpus()` returns `EventsPerVariable * len(FeatureNames())`, `DefaultConfig().MinCorpus` calls it, and the bare 40 is gone. The Train gate (`len(examples) < cfg.MinCorpus`) did not change and reads the derived value through `cfg`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | prints `1`; stream shows `--- PASS: TestMinCorpusDerivedFromFeatureCount (0.00s)`, `ok .../qualgen/riskscore`. Fail-first re-done by this runner: with EventsPerVariable 10 -> 9 (scratch edit, restored with git checkout) the row exits 1 and prints `0`, `learned_test.go:330: EventsPerVariable = 9, documented derivation says 10` | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | prints `1`; `--- PASS: TestMinCorpusGovernsLearnedSwitch (0.00s)`. Fail-first re-done: with the Train gate hard-coded to `< 40` the row exits 1 and prints `0`, `learned_test.go:358: Train with 149 examples (floor 150) must refuse as under-corpus, got <nil>`, so the floor really governs the switch | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | no output | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | match present; Derivation blocks sit on EventsPerVariable (learned.go line 39) and on Config.MinCorpus (line 68) | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | fail exit=2 (could-not-check) | `--consumers: COULD-NOT-CHECK: assay:assay:measured-status:02 is not in the diff against fc5afe37f9e4..., so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` Expect (exit 0) is not met as written: on merged main the default base is main, so there is no diff to read. Diagnostic only, not the row: with `--base a7b4bed48` (the squash parent of the implementing commit) the tool exits 0 with `0 corroborated, 0 disproved, 1 unchecked`. The one consumers entry, the Train gate, is unchanged text that reads the changed `cfg.MinCorpus`, and nothing is disproved. Same check-definition class as #1915 | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |

Execution witness (`statusgen verifyrun`, non-dry, clean tree at fc5afe37f9e4):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | fail exit=2 | sha256:98fe379a1224 | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |

Risk-bearing value. The brief's risk answers are all present and all `no`, and the item is not irreversible, so the fail-safe trigger does not fire. The enumeration was done anyway.

The scope covered was the a11ac4167 diff to `qualgen/riskscore/learned.go` and `qualgen/riskscore/learned_test.go`, plus the Deliverables. It found these literals:

- `EventsPerVariable = 10` @ qualgen/riskscore/learned.go:46
- `DerivedMinCorpus() = EventsPerVariable * len(FeatureNames())` @ qualgen/riskscore/learned.go:60. Its count input is the 15-entry list at qualgen/riskscore/features.go:162, so the value is 150. It replaces the removed `MinCorpus: 40`.
- `documentedEPV = 10` @ qualgen/riskscore/learned_test.go:328 (the test pin)
- the test-fixture sizes `floor+5` and seed `11` @ qualgen/riskscore/learned_test.go:351. These are not risk-bearing.

Ranking: EventsPerVariable is first. If it is wrong, the learned model graduates over the heuristic too early or too late, and that error reaches every downstream score. The breakage is reversible with a one-constant edit and a redeploy. Epochs, LearningRate and L2 were not changed by the diff and are out of scope.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression (`sigmoid` at learned.go:396, used by Train and Score). 10 events per variable is the conventional floor for logistic regression (Peduzzi et al. 1996), and it is the value the brief's own fact names (`MinCorpus >= EPV * featureCount`, EPV ~10). The resulting floor of 10 x 15 = 150 was recomputed independently from both FeatureNames and Vector. Caveat, documented in the code and left open on #1171: the floor counts TOTAL labeled rows, not rarer-outcome events, so it is a necessary but weaker condition than true EPV.

Result: 4 of 5 rows pass. Row 5 fails as written: exit 2, could-not-check, because the instrument has no diff to read on merged main (class #1915). The substance holds: the floor is derived, pinned and fail-first proven, and the base-pinned diagnostic shows 0 disproved. Status stays `implemented` until row 5 is re-baselined.

VERIFY: FAIL
### Non-implementer verifier re-run — 2026-10-07 claude-opus-5-5[1m]

Verified SHA: merged main `91f04b81ba064394aa121a4940cf11a138b55402` (implementation landed in a11ac4167, #2054; its parent a7b4bed48). The runner is not the implementer. It ran in its own detached temporary worktree cut from origin/main. Toolchain: host go1.27.1 darwin/arm64, statusgen v1.0.32. Offline: `KUBECONFIG=/dev/null`, no endpoint contacted. Every row ran under `bash -o pipefail`.

Re-check of the previous FAIL (2026-10-06, row 5 exit 2 could-not-check): row 5 still fails as written on current main. Neither the row text nor the statusgen version changed since that run. #2300 re-derived the `--consumers` rows to the post-merge pinned form for build-less-brittle 05, 09, 11, 12 and 13 (#1915), but it did not touch this brief, so no merged fix has cleared row 5. Note for re-authoring: the delivering commit a11ac4167 carries no `Brief:` trailer, so the #2300 trailer lookup copied verbatim would resolve nothing and fail closed. The re-authored row has to resolve the delivering change another way, for example from the `(measured-status/02)` subject or from #2054.

Grounding was written from the brief text and the main source before the tests were read. It expected `EventsPerVariable` of about 10 and `MinCorpus = EventsPerVariable * len(features)` = 10 x 15 = 150, with no bare 40. Main matches. EventsPerVariable = 10 is at learned.go line 46. DerivedMinCorpus() returns EventsPerVariable * len(FeatureNames()) (lines 59-61). DefaultConfig's MinCorpus calls DerivedMinCorpus() (line 81). The Train gate reads cfg.MinCorpus (line 185). FeatureNames() lists 15 names (features.go lines 161-168).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | prints `1`. Fail-first re-done by this runner: with EventsPerVariable changed from 10 to 9 (scratch edit, restored with git checkout) the test prints `0` and exits 1, with `learned_test.go:330: EventsPerVariable = 9, documented derivation says 10` and `--- FAIL: TestMinCorpusDerivedFromFeatureCount` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | prints `1` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | no output | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | match present; Derivation blocks are on EventsPerVariable (learned.go line 39) and on Config.MinCorpus (line 68) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | fail exit=2 (could-not-check) | `--consumers: COULD-NOT-CHECK: assay:assay:measured-status:02 is not in the diff against 91f04b81ba06..., so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` The Expect (exit 0) is not met as written, because on merged main there is no diff to read (class #1915). A separate diagnostic run, which is not the row: with the tree at a11ac4167 and `--base a7b4bed48` the tool exits 0 with `0 corroborated, 0 disproved, 1 unchecked`. The single consumers entry, the Train gate, is UNCHECKED, and nothing is disproved | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Risk-bearing value. All of the brief's risk answers are present and all are `no`, and the item is not irreversible, so the fail-safe trigger does not fire. The enumeration was done anyway, over the a11ac4167 changes to the riskscore learned layer and its test plus the Deliverables:
- `EventsPerVariable = 10` @ qualgen/riskscore/learned.go:46
- `DerivedMinCorpus() = EventsPerVariable * len(FeatureNames())` @ qualgen/riskscore/learned.go:60. Its input is 15 names at qualgen/riskscore/features.go:162-167, so it gives 150. It replaces the removed `MinCorpus: 40`.
- `documentedEPV = 10` @ qualgen/riskscore/learned_test.go:328 (test pin)
- fixture sizes `floor+5` and seed `11` @ qualgen/riskscore/learned_test.go:351 (not risk-bearing)

EventsPerVariable ranks first. If it is wrong, the learned model graduates too early or too late, and every downstream score inherits the error. The error can be undone with one constant edit and a redeploy.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression (sigmoid at learned.go:396, used in Train at :221 and Score at :250). 10 events per variable is the conventional logistic-regression floor (Peduzzi et al. 1996) and is the value the brief's own fact names. 10 x 15 = 150 was recomputed from FeatureNames. Caveat: the floor counts total labeled rows, not rarer-outcome events, so it is a necessary but weaker condition. The code documents this, and #1171 tracks it.

Result: 4 of 5 rows pass. Row 5 still fails as written (exit 2, could-not-check, class #1915), and no merged fix has cleared it. Status stays `implemented` until row 5 is re-authored to the post-merge pinned form.

VERIFY: FAIL

Filed: #1915 (row 5 instance not covered by #2300; issuecomment-6029774563)

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
