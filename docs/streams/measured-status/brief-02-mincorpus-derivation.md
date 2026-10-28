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
version: 2
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
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02 --delivery a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 --base a7b4bed48f3075336958b812617964306274f188` | exit 0; output does not contain "DISPROVED" (the fixed-here consumer routing is corroborated, not contradicted) | check |

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
### Non-implementer verifier run — 2026-10-12 gpt-6.1-sol

Merged source: 024b15e1c783d4f0694a215e4e32a8f94ce11e91. Fresh non-implementer, dispatcher admission #2683, selected tier any. Host macOS arm64; Go 1.27.1 (module requires 1.25.0), git 2.56.0, statusgen v1.0.39. All authored rows ran unchanged under Bash with pipefail and KUBECONFIG=/dev/null. No source, index, module or tool edits; no independent source-mutation claim is made. Task 5's earlier implementer record was read as an archival record, never as this run's observation.

| # | Command | Expected | Observed | Date and runner |
|---|---------|----------|----------|-----------------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | exit 0; named test PASS line present | exit 0; prints 1 | 2026-10-12 assay-verifier-app[bot] (gpt-6.1-sol) @ 024b15e1c783 |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | exit 0; below-floor fallback and at-floor learned switch | exit 0; prints 1 | 2026-10-12 assay-verifier-app[bot] (gpt-6.1-sol) @ 024b15e1c783 |
| 3 | `cd qualgen && go build ./riskscore/` | exit 0 | exit 0; no output | 2026-10-12 assay-verifier-app[bot] (gpt-6.1-sol) @ 024b15e1c783 |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | exit 0; written derivation present | exit 0; no output | 2026-10-12 assay-verifier-app[bot] (gpt-6.1-sol) @ 024b15e1c783 |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | exit 0; no DISPROVED | could-not-check, exit 2; COULD-NOT-CHECK: brief is not in the diff against 024b15e1c783d4f0694a215e4e32a8f94ce11e91; no entry was corroborated and none was disproved | 2026-10-12 assay-verifier-app[bot] (gpt-6.1-sol) @ 024b15e1c783 |

Execution witness: actual statusgen verifyrun --dry-run output below, retained for Evidence landing. Its literal `fail exit=2` row-5 label records the mismatch with Expect exit 0; the command's observed COULD-NOT-CHECK diagnostic means no consumer verdict was obtained. verifyrun itself exited 1. That label is not an observed riskscore defect.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | fail exit=2 | sha256:9ece27b27192 | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2683 run=50126dba0b55ca5917ab33aabeba75c1dfa678efb709f4ec38373feeec99c4c8 source=024b15e1c783d4f0694a215e4e32a8f94ce11e91 brief=docs/streams/measured-status/brief-02-mincorpus-derivation.md model=gpt-6.1-sol tier=any

Supplementary diagnostic only: the two unfiltered named tests print their own `--- PASS:` lines and the package reports `ok`. This diagnostic is not a replacement for either authored grep row. No supplemental --base command was used.

Independent derivation: the brief's facts choose an approximately 10-per-predictor multiplier and a 15-feature vector, so the contractual total-row floor is 150. Independently enumerating the production feature list yields ns, nd, nf, entropy, la, ld, lt, ndev, age, nuc, npd, rc, exp, class_agent, class_automation; the numeric Vector has the same 15 columns. DerivedMinCorpus multiplies EventsPerVariable by that feature-name count; DefaultConfig passes it to Train's strict below-floor comparison (learned.go:185), and ScoreChange converts refusal to the heuristic fallback (learned.go:298-307). Thus 149 labeled rows must refuse and 150 must permit training, subject to the existing two-class gate. The new tests inspect the independent Vector length and exercise both Train and ScoreChange.

Literal enumeration and ranking over delivery a11ac41674c09aadb2a940bc7d1edbcd5e4c1402's executable changes, its adjacent derivation comments, changed Verify assertions and named Task deliverables (archival result timestamps, SHAs, issue IDs and commit metadata are provenance, not configuration values):

| Rank | Identifier = literal and source | Consequence and reversibility |
|---|---|---|
| 1 | EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 | Governs graduation of every learned score; wrong setting prolongs heuristics or graduates early. Reversible by edit/redeploy. |
| 2 | documentedEPV = 10 @ qualgen/riskscore/learned_test.go:328 | Oracle pin; wrong value blesses a wrong multiplier. Reversible test edit. |
| 2 | documentedFeatureCount = 15, documentedMinCorpus = 150 @ qualgen/riskscore/learned.go:51; documented EPV approximately 10, floor approximately 150, features 15 @ docs/streams/measured-status/brief-02-mincorpus-derivation.md:45 | Comment/contract numeric assertions; recomputed from the vector rather than copied from a green test. |
| 3 | illustrativeDefectRate = 0.2, illustrativeEvents = 30, illustrativeEPV = two @ qualgen/riskscore/learned.go:56 | Explanatory example only; 0.2 x 150 = 30 and 30 / 15 = 2. No executed threshold. |
| 3 | replacedMinCorpus = 40 @ delivery a11ac4167 parent qualgen/riskscore/learned.go:47; historicalMinCorpus = 40 @ current qualgen/riskscore/learned.go:70; alternativeMinCorpus = 40 @ docs/streams/measured-status/brief-02-mincorpus-derivation.md:63 | Removed runtime threshold / historical and alternate-task rationale, never this default. |
| 3 | fixtureSurplus = 5, fixtureSeed = 11 @ qualgen/riskscore/learned_test.go:351; belowFloorOffset = 1 @ :355, :358, :364, :365 | Fixture coverage and deterministic generation; reversible test values. |
| 3 | fixturePath = "a/x.go" @ qualgen/riskscore/learned_test.go:332 | Creates a feature extraction fixture; reversible test data. |
| 3 | underCorpusReason = "under-corpus" @ qualgen/riskscore/learned_test.go:357; fallbackReason = "could-not-learn" @ :361 | Exact observation tokens in the flow oracle; reversible assertion text. |
| 3 | paperAuthority = "Peduzzi et al. 1996" @ qualgen/riskscore/learned.go:40; specAuthority = "§12" @ :41; configAuthority = "§3.2/§11" @ :67 | Adjacent rationale citations, not runtime knobs. |
| 3 | testSelectors = '^TestMinCorpusDerivedFromFeatureCount$' and '^TestMinCorpusGovernsLearnedSwitch$', testCount = 1, expectedExit = 0 @ docs/streams/measured-status/brief-02-mincorpus-derivation.md:87-88; derivationToken = 'Derivation:' @ :90; consumersBrief = 'assay:assay:measured-status:02', forbiddenOutput = "DISPROVED", expectedExit = 0 @ :91 | Named oracle bindings prevent missing-test vacuity and preserve consumer assertion; row 5 cannot obtain its evidence on merged main. |

FeatureNames' string column bindings and Vector values are unchanged by this delivery and were counted as dependency input. Existing Epochs=400, LearningRate=0.1 and L2=1e-3 are unchanged context, not changed literals. The runtime minimum is computed, not a literal 150; it is not misrepresented as a source literal.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the brief explicitly chooses an approximately 10 multiplier; 15 production predictors independently give a contractual total-corpus floor of 150. The [cited Peduzzi study](https://pubmed.ncbi.nlm.nih.gov/8970487/) supports 10 as its observed EPV boundary in that study, not a guarantee that 150 total rows suffice for this model. For a defect fraction p, the implied event count is 150p; true 10-event-per-predictor coverage would require about 150/p total rows (750 at p=0.2). The code explicitly documents this weaker total-row condition; #1171 carries the separate event-floor question. No statistical sufficiency claim is inferred from the passing tests.

RISK-VALUE: DERIVED — documentedEPV = 10 @ qualgen/riskscore/learned_test.go:328 — this is the independently documented contract multiplier; the test obtains count from Vector and checks FeatureNames agrees, avoiding self-confirmation solely through the same production count function.

**VERIFY: BLOCKED** — four authored rows passed; row 5 observed could-not-check (exit 2), not an observed implementation failure. No status advance. Check-definition instance attached to [medici-finance/assay#1915](https://github.com/medici-finance/assay/issues/1915#issuecomment-6115983645). Wake only after the Verify definition or applicable consumers instrument changes to obtain the delivery diff on merged main.

### Non-implementer verifier re-run — 2026-10-19 claude-opus-5-5[1m]

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 sandbox=container-netns | sha256:4355a46b19d3 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 sandbox=container-netns | sha256:4355a46b19d3 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | fail exit=2 sandbox=container-netns | sha256:3e4f11ddc077 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |

The table above is the execution witness for this re-run: `statusgen verifyrun --sandbox container-netns`, non-dry, in a throwaway clone of merged main `8eb028b49bc5e7d45e470cc25dc8b4f531605238`. Runner: a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47), `--network none`, sandbox=container-netns, default seccomp, GOPROXY=off, statusgen and the desk tools built from the tree. It ran under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). The runner is not the implementer. Dispatcher admission passed before and after the witness (medici-finance/assay#3545). The rows were also run by hand under `bash -o pipefail` in a separate scratch clone at the same sha (host go1.27.2 darwin/arm64, statusgen v1.0.51, `KUBECONFIG=/dev/null`). Rows 1 and 2 printed `1`, rows 3 and 4 exited 0, and row 5 exited 2.

The prior Evidence verdict was BLOCKED, recorded 12 October 2026 at sha 024b15e1c783 (row 5 could-not-check). Since then, c24f792be (medici-finance/assay#3428) rewrote row 5 to a delivery-pinned form. Then medici-finance/assay#3508 restored the original row text per the driver's ruling, so the current row 5 is the same command that the earlier run could not check. `qualgen/riskscore` is unchanged since the delivering commit a11ac4167.

Grounding, written from the brief before the witness ran: rows 1 and 2 should print `1` because both named tests exist and pass. Row 3 should build. Row 4 should find the `Derivation:` comments on EventsPerVariable and Config.MinCorpus. Row 5 should hit the same post-merge limit as before: on merged main the default base is HEAD, so the instrument has no delivery diff to read.

Row 5, classification: tool / check-definition defect (class medici-finance/assay#1915). It is not a deliverable defect and not environment-only. By hand with the network allowed, the row exits 2 with `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:measured-status:02 is not in the diff against 8eb028b49bc5e7d45e470cc25dc8b4f531605238, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.`, which is the same exit as in the container. This is a could-not-check: the instrument did not look, so it is not a pass. Separate diagnostic, which is not the row: `statusgen --root . --consumers --brief measured-status/02 --delivery a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 --base a7b4bed48f3075336958b812617964306274f188` exits 0 with `CORROBORATED  qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here` and `summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. So the consumer claim holds when the instrument is pointed at the delivery.

Risk-bearing value. The brief's risk answers are all present and all `no`, and the item is not irreversible, so the trigger does not fire. The enumeration was repeated anyway over the a11ac4167 riskscore diff, which is unchanged at this head. It found `EventsPerVariable = 10` @ qualgen/riskscore/learned.go:46, the floor `EventsPerVariable * len(FeatureNames())` @ qualgen/riskscore/learned.go:60 (15 features, so 150), the default wired at line 81 and the Train gate at line 185, and `documentedEPV = 10` @ qualgen/riskscore/learned_test.go:328.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression, and 10 per predictor is the conventional logistic floor and the value the brief's own fact names. 10 x 15 FeatureNames gives 150 total labeled rows. As the code documents, this is a weaker condition than a true rarer-outcome event floor, and medici-finance/assay#1171 tracks that question.

Verification-Attestation: medici-finance/assay#3545 run=7b85f0ecf5b4a7cf79d5ea019f8aca9e0f554c8a34fed11c958ecbdf92d462c6 source=8eb028b49bc5e7d45e470cc25dc8b4f531605238 brief=docs/streams/measured-status/brief-02-mincorpus-derivation.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — rows 1 to 4 pass in the witness and by hand. Row 5, as restored by medici-finance/assay#3508, still exits 2 (could-not-check) on merged main, because the consumers instrument has no delivery diff to read there (class medici-finance/assay#1915). The delivery-pinned diagnostic finds 0 disproved, but a could-not-check is not a pass, so the status does not advance.

Outcome record: owed, not on this branch. The owed receipt is the next admitted verifier pass's own record (`blocked`, check-definition, medici-finance/assay#1915). Per `docs/verify-outcome-rows.md` §"Producer migration: covered rows are not held rows", it carries `rows:[5]` (held rows only), `row_results` for all five rows (rows 1 to 4 `pass`, row 5 `could-not-run`), `rows_passed` 4 and `rows_total` 5. It supersedes `02-20261012T011455Z-0c5b83ba8296`, the newest earlier receipt for this brief, which holds row 5 under the same blocker. All earlier receipts stay unchanged as history.

### Non-implementer verifier re-run — 2026-10-21 claude-opus-5-5

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | exit 0 (the test's own `--- PASS:` line is present) | pass: exit 0, printed 1 (network-off container) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | exit 0 (one below the floor is could-not-learn, one at the floor trains) | pass: exit 0, printed 1 (network-off container) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd qualgen && go build ./riskscore/` | exit 0 | pass: exit 0, no output (network-off container) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | exit 0 | pass: exit 0 (network-off container) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | exit 0; no "DISPROVED" | could-not-check: exit 2, "COULD-NOT-CHECK: assay:assay:measured-status:02 is not in the diff against c82a8c5f807b..., and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Runner environment: docker image assay-verify:go1.27-py (go1.27.2 linux/arm64, git 2.47.3), `--network none`, GOPROXY=off, GOTOOLCHAIN=local, the source tree and module cache mounted read-only, statusgen built inside the container from the tree at this sha. The rows ran in a throwaway clone of merged main at c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a with refs/remotes/origin/main at the same sha, because the linked admitted home's git backlink does not resolve inside the container. Runner is not the implementer. Output log sha256:7d77d4f1989b (kept outside the tree).

Supplemental probes (no acceptance credit, same container): the pre-delivery table's row 1 command (`-run TestMinCorpusDerivedFromFeatureCount -count=1`) exits 0 with `ok`, and its row 2 command (`-run TestMinCorpusGovernsLearnedSwitch -count=1 -v`, piped to `grep -q 'PASS'`) exits 0. With EventsPerVariable hand-mutated from 10 to 9 in a throwaway copy, row 1's command prints 0 and exits 1 (`EventsPerVariable = 9, documented derivation says 10`). With the Train gate mutated from `<` to `<=`, row 2's command prints 0 and exits 1. Both rows therefore discriminate. qualgen/riskscore is unchanged since the delivering squash a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 (first parent a7b4bed48f3075336958b812617964306274f188); the whole riskscore package passes.

Row 5: the instrument now derives the delivery pin only from a committed `Verification-Table:` marker in this brief's Evidence (docs/consumer-delivery-checks.md), and none exists at this sha, so it did not look. This is the same tool/check-definition class as before (medici-finance/assay#1915), not a deliverable defect. The marker below names the delivery for the next pass once it is committed. Supplemental probe (no acceptance credit): this exact brief committed on top of c82a8c5f807b in a throwaway clone, with origin/main at that commit, makes the row 5 command exit 0 network-off with `CORROBORATED  qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here` and `summary: 1 corroborated, 0 disproved, 0 unchecked`. So row 5 should resolve on the next admitted pass after this Evidence lands.

Verification-Table: delivery=a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 base=a7b4bed48f3075336958b812617964306274f188 original=a7b4bed48f3075336958b812617964306274f188

Risk-bearing value. Frontmatter risk answers are all present and all `no`, not irreversible; enumeration done anyway over the a11ac4167 riskscore diff: `EventsPerVariable = 10` @ qualgen/riskscore/learned.go:46; floor `EventsPerVariable * len(FeatureNames())` @ qualgen/riskscore/learned.go:60 (15 names, so 150); `MinCorpus: DerivedMinCorpus()` @ qualgen/riskscore/learned.go:81; gate `len(examples) < cfg.MinCorpus` @ qualgen/riskscore/learned.go:185; `documentedEPV = 10` @ qualgen/riskscore/learned_test.go:328.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression, and 10 per predictor is the conventional logistic EPV floor (Peduzzi et al. 1996) and the value the brief's own fact names; 10 x 15 features = 150 total labeled rows. It counts total rows, not rarer-outcome events, which the code documents as a weaker condition (medici-finance/assay#1171).

Verification-Attestation: medici-finance/assay#3962 run=601383e826e9a6a1d84e10d9443197d6d74facf93b0131db036acf592f424f51 source=c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a brief=docs/streams/measured-status/brief-02-mincorpus-derivation.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — rows 1 to 4 pass network-off; row 5 is could-not-check (exit 2): no committed Verification-Table marker at this sha (class medici-finance/assay#1915). 4 of 5 rows passed.

### Non-implementer verifier re-run — 2026-10-23 claude-opus-5-5

Witness (installed `statusgen verifyrun` v1.0.56, host, admitted home detached at 4b7568de01248b514f48493efea7734ce0149823; throwaway HOME carrying a roster copy, real Go caches). Runner is not the implementer.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusDerivedFromFeatureCount$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusDerivedFromFeatureCount'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test ./riskscore/ -run '^TestMinCorpusGovernsLearnedSwitch$' -count=1 -v 2>&1 \| grep -c '^--- PASS: TestMinCorpusGovernsLearnedSwitch'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go build ./riskscore/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | pass exit=0 | sha256:f32c3af8eb1f | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4196 run=2f1aba480d397201dad557e9ac5729c6b6074ce2ed6970eccc43083c9500ab56 source=4b7568de01248b514f48493efea7734ce0149823 brief=docs/streams/measured-status/brief-02-mincorpus-derivation.md model=claude-opus-5-5 tier=strong

Observed per row (all five rows executed by the witness above, each exit 0): row 1 printed 1 (the test's own `--- PASS:` line); row 2 printed 1; row 3 no output; row 4 matched; row 5 output hash sha256:f32c3af8eb1f, which is byte-identical to a hand capture on a clean clone at this sha: `base a7b4bed48f30..a11ac41674c0 (immutable delivered snapshot)`, `CORROBORATED  qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here`, `summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. No DISPROVED. The instrument pins to the delivery named by the committed 2026-10-21 marker above. The same command on a tree with an uncommitted Evidence edit reads base 4b7568de0124 with the working tree and reports `UNCHECKED` (0 corroborated, exit 0). That is the unfailable shape the closure lint below refuses.

Supersedes: the 2026-10-22 run recorded on Evidence PR medici-finance/assay#4063 (PASS 5/5 at source 05dc9fbc88f2, head b4292b5d75dc) is superseded by this run per medici-finance/assay#4163. Its witness is not credited here. Its two blocking review findings (f-closure-1, f-row5-closure-1) are what this run re-checks.

Closure probes (read-only, no acceptance credit). They ran in a throwaway clone at this sha, outside the admitted home, with this brief plus a probe-only PASS marker and the marker line below. That README row 02 was flipped to verified only in the clone. The admitted home's stream index is unchanged.
- Home, unflipped, as recorded above: `statusgen verifyrun --check` exit 2, `refused — protected table: refusing: verdict mismatch: the latest recorded verdict is VERIFY: BLOCKED, not PASS`.
- Clone, unflipped, with a probe PASS marker: `verifyrun --check` reports rows 3 to 5 `pass — witness matches the row and passed`. It reports rows 1 and 2 `could-not-run — the witness records a different command than the Verify row now carries`. Summary: `3 pass, 0 fail, 2 could-not-run/missing (of 5 Verify rows)`. `statusgen --lint` exits 0, with row 5 `[consumer-delivery-range]` only a NOTICE.
- Clone, flipped (closing tree): `statusgen --lint` exits 1 with three PROBLEMs for this brief. The first is `Verify row 5 [consumer-delivery-range] consumer command has no explicit --base ... a PROBLEM because this branch closes the brief ...: an unfailable row cannot be the basis of a new closure`. The other two are `measured-status/02: protected acceptance row 1: could-not-run` and the same for row 2.

Blockers:
1. Row 5 (check-definition class, medici-finance/assay#1915, open at 2026-10-23). Re-pinning the row with `--delivery/--base` needs a ruling, because medici-finance/assay#3508 restored this text. It is not a verifier edit.
2. Protected original rows 1 and 2. The table at base a7b4bed48f30 carries the pre-delivery commands. Delivery a11ac4167 re-authored them (the implementer record above). The installed witness runs only the current table, so the original commands have no witness and closure refuses. Supplemental probe on this host under `bash -o pipefail`, no acceptance credit: original row 1 (`go test ./riskscore/ -run TestMinCorpusDerivedFromFeatureCount -count=1`) exits 0 with `ok`. Original row 2 (`... -v 2>&1 | grep -q 'PASS'`) exits 141 on 3 of 3 runs. That is SIGPIPE from `grep -q`, the defect the implementer recorded, not a test failure. A passing witness of the original row 2 command therefore needs an acceptance-transition path or a ruling (next actor: the driver, or the medici-finance/assay#1915 owner, for the check-definition class).

qualgen/riskscore is unchanged between delivery a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 and this sha (empty diff).

Verification-Table: delivery=a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 base=a7b4bed48f3075336958b812617964306274f188 original=a7b4bed48f3075336958b812617964306274f188

Risk-bearing value. The frontmatter risk answers are all present and all `no`, and the brief is not irreversible. Enumeration over the a7b4bed48f30..a11ac4167 qualgen diff, plus the Deliverables:
- `EventsPerVariable = 10` @ qualgen/riskscore/learned.go:46
- floor `EventsPerVariable * len(FeatureNames())` @ qualgen/riskscore/learned.go:60 (15 names, so 150)
- `MinCorpus: DerivedMinCorpus()` @ qualgen/riskscore/learned.go:81, replacing the removed literal `MinCorpus: 40`
- gate `len(examples) < cfg.MinCorpus` @ qualgen/riskscore/learned.go:185
- `documentedEPV = 10` @ qualgen/riskscore/learned_test.go:328
- test-only corpus sizing `floor+5`, seed `11`, `floor-1` in the flow test

Ranked: EventsPerVariable is first, because it sets when the learned model is trusted over the heuristic. It is reversible by a one-constant edit and a redeploy. The test literals rank last.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression, and 10 per predictor is the conventional logistic EPV floor (Peduzzi et al. 1996). It is also the value the brief's own fact names. 10 x 15 features = 150 total labeled rows. It counts total rows, not rarer-outcome events, which the code documents as a weaker condition (medici-finance/assay#1171).

**VERIFY: BLOCKED** — (blocker: medici-finance/assay#1915) all 5 current-table commands exit 0 in the witness, and row 5 corroborates. Acceptance accounting: rows 3 and 4 pass. Rows 1 and 2 are could-not-run as protected original obligations, because the original command has no witness. Row 5 is held: it is uncreditable for a closure, since it trips `[consumer-delivery-range]` on a closing tree. 2 of 5 acceptance obligations are credited.
Observed per row (all five current-table rows executed by the 2026-10-28 witness above, each exit 0; no row is risk-bearing, so the fail-first pass recorded every row not-selected at base): row 1 printed 1 (the test's own `--- PASS:` line); row 2 printed 1; row 3 no output; row 4 matched; row 5 (now carrying explicit `--delivery a11ac41674c0... --base a7b4bed48f30...`) reported `CORROBORATED  qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here` and `summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. No DISPROVED. qualgen/riskscore is unchanged between delivery a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 and this sha (empty diff).

Supplemental probes (no acceptance credit, throwaway clone outside the admitted home, mutations restored): with EventsPerVariable hand-mutated from 10 to 9, row 1's command prints 0 and exits 1; with the Train gate hard-coded to `< 40`, row 2's command prints 0 and exits 1. Both rows discriminate. The original (pre-delivery) row 2 command, piped to `grep -q 'PASS'` under `bash -o pipefail`, exits 141 (SIGPIPE) on 3 of 3 runs, unchanged from the 2026-10-23 pass.

Closure: `statusgen verifyrun --check` on this tree exits 2 with `refused — protected table: refusing: verdict mismatch: the latest recorded verdict is VERIFY: BLOCKED, not PASS`. In the throwaway clone with a probe-only PASS marker, `--check` exits 2 with `refused — protected table: current Verify section differs from immutable delivery`, and `statusgen verifyrun --acceptance-record` (dry run) with the marker below refuses with the same reason before any row runs. `statusgen --lint` exits 0 on both trees.

Blockers:
1. Row 5 was amended after delivery (medici-finance/assay#4419, amendment 38f3469dadc5) to add the explicit delivery pin. The protected-table evaluator accepts such an amendment only through a human-approved acceptance transition on a same-repository decision issue; none exists for this brief. Next actor: the driver.
2. Rows 1 and 2 were re-authored by the delivery itself (a11ac4167), so they still owe their original commands. Original row 2 can never exit 0 under pipefail (SIGPIPE from `grep -q`), and the documented transition path covers only rows whose delivered definition equals the original. Next actor: the driver, or the protected-table owner, for a ruling on how that obligation is discharged.

Verification-Table: delivery=a11ac41674c09aadb2a940bc7d1edbcd5e4c1402 base=a7b4bed48f3075336958b812617964306274f188 original=a7b4bed48f3075336958b812617964306274f188

Risk-bearing value. The frontmatter risk answers are all present and all `no`, and the brief is not irreversible. Enumeration over the a7b4bed48f30..a11ac4167 qualgen diff is unchanged from the 2026-10-23 pass: EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 ranks first (it sets when the learned model is trusted over the heuristic; reversible by a one-constant edit), then the derived floor @ :60, its use @ :81, the Train gate @ :185, and the test-only literals.

RISK-VALUE: DERIVED — EventsPerVariable = 10 @ qualgen/riskscore/learned.go:46 — the learned layer is a logistic regression (sigmoid @ qualgen/riskscore/learned.go:396), and 10 per predictor is the conventional logistic EPV floor (Peduzzi et al. 1996) and the value the brief's own fact names; 10 x 15 features = 150 total labeled rows. It counts total rows, not rarer-outcome events, which the code documents as a weaker condition (medici-finance/assay#1171).

**VERIFY: BLOCKED** — all 5 current-table commands exit 0 in the witness and row 5 corroborates, but closure refuses: `protected table: current Verify section differs from immutable delivery`. Row 5's post-delivery amendment has no human-approved acceptance transition, and protected original rows 1 and 2 have no witness on their original commands (original row 2 exits 141 under pipefail). 5 of 5 current rows pass; the closure is held for the driver's ruling.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
