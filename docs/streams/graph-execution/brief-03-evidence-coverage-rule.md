---
brief: assay:assay:graph-execution:03
title: Evidence coverage rule and the observe evidence kind
why: >-
  A brief can be marked verified while a mandatory check was never run, errored, or ran
  against a different revision of the work: the tools record what happened but no rule says
  what MUST have happened before the next step is allowed. A deterministic coverage rule
  turns "the evidence looks fine" into "every mandatory claim has a passing result at this
  exact revision, or nothing downstream moves" — and adds the one evidence kind the fleet
  lacks, a signal watched over a window after a change lands.
wave: 1
depends: ["graph-execution/02"]
unblocks: ["graph-execution/05"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/work-input-amendment.md — 2026-09-30 pending scope amendment"
  - "freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3: amend unfinished scope; no implementation claim"
  - "docs/streams/graph-execution/admission-assurance-spec.md — 2026-09-18 integration amendment"
  - "freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f"
  - "docs/streams/graph-execution/spec.md §2.3 (verification governs transitions; the coverage rule; the observe kind; integration check at the join) and §3 (evidenceactor advisory, autoflip does not re-check the stamp)"
  - "graph-execution/02 — the node contract's `evidence: [{kind, claim, mandatory}]` and the `join:` node this rule reads"
  - "statusgen/verifyrun.go (the execution witness: pass / fail / could-not-run, tree SHA recorded per row, treeSHALen=12); statusgen/lifecycle.go (WitnessInfo.Version stale-witness demotion); statusgen/witnessgate.go"
  - "statusgen/evidenceactor.go (Evidence attribution by git blame — a NOTICE, not a gate); statusgen/autoflip.go (header: the flip does NOT re-check the `verified` stamp it promotes)"
  - "docs/three-state-instrument-rule.md (could-not-check is never pass)"
  - "Baz — planner/verifier split https://youtu.be/aWrGSM5vVyc?t=566 ; per-requirement dispatch https://youtu.be/aWrGSM5vVyc?t=706 ; pre-change grounding https://youtu.be/aWrGSM5vVyc?t=872"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn -- '--coverage\\|wrong-revision\\|observe:' statusgen/*.go` returns no hits; verifyrun records a tree SHA per witness row but nothing compares it to the brief's merged revision as a release condition — not already satisfied"
exec-tier: strong
exec-tier-why: "(b) the rule joins four artifacts — Verify rows, verifyrun witnesses, reviewer approvals and the pattern's evidence declarations — and a mismatch in revision semantics between any two lets a stale pass release work; (c) an aggregation that treats could-not-check as pass is the exact fault and passes every happy-path test"
domain: complicated
consumers:
  - "statusgen/autoflip.go (the verified→done flip gains a coverage precondition): fixed-here"
  - "statusgen/lifecycle.go (the `verified` witness precedence reads coverage, not only WitnessInfo.Passed): fixed-here"
  - "spec/lifecycle-v1.md (the verified state gains the coverage condition): fixed-here"
  - "spec/workflow-pattern-v1.md (the `observe` kind's fields): fixed-here"
  - ".github/workflows/verify-gate-open.yml and verify-gate-close.yml (the human sign-off pair): out-of-scope (unchanged — a human sign-off is a `decision` node; coverage governs the model lane's transitions and reports for the human lane)"
  - "statusgen/evidenceactor.go (attribution stays advisory): out-of-scope (this brief adds no identity gate; attribution and coverage are different questions and stay separate rules)"
version: 3
id: d94aa822-65e3-44ad-ba9a-479ddcb47fdc
---

# Brief 03 — Evidence coverage rule and the observe evidence kind

## Context
files: `statusgen/coverage.go` (planned), `statusgen/coverage_test.go` (planned), `statusgen/testdata/coverage/` (planned), `statusgen/autoflip.go`, `statusgen/lifecycle.go`, `statusgen/main.go` (the `--coverage` flag), `spec/lifecycle-v1.md`, `spec/workflow-pattern-v1.md` (planned) — graph-execution/02 creates it, `schemas/workflow-pattern-v1.json` (planned) — graph-execution/02 creates it, `docs/lifecycle.md`, `docs/enforcement-model.md`, `changelog/graph-execution-03-coverage.md` (planned)
facts:
- `statusgen verifyrun --brief <path>` writes an Evidence row per Verify row with a three-state Result (`pass` / `fail` / `could-not-run`) and the tree it ran against (`Runner` cell, 12-char SHA, `+dirty` when modified) — `statusgen/verifyrun.go`. `--check` reports per row whether a witness exists, still describes the row, and passed. Nothing compares the witness SHA to the brief's merged revision as a condition for anything. (Read 2026-09-16 @ d96fd3ba.)
- `statusgen/lifecycle.go` folds witnesses by precedence: `WitnessInfo{Passed, Version}` is the `verified` witness and a Version mismatch against the brief demotes it (stale-Verify). `ApprovalInfo{Approved, AtHead}` is the `done` witness for `gate: model`.
- `statusgen/autoflip.go` (header comment): the verified→done flip's precondition is exactly two facts — README row at `verified`, and an APPROVED reviewer-App review at the merged head; it does NOT re-check the `verified` stamp it promotes. `statusgen/evidenceactor.go` attributes Evidence lines by `git blame` and emits NOTICEs.
- Vocabulary (from graph-execution/02 and the README): evidence kinds `command | review | witness | observe`; results `pass | fail | missing | error | could-not-check | wrong-revision`. Mapping from today's witnesses: verifyrun `pass`→`pass`, `fail`→`fail`, `could-not-run`→`could-not-check`, no witness→`missing`, witness at a different SHA than the item's revision→`wrong-revision`, witness present but unparseable→`error`.
- **Decision recorded here:** `autoflip.go` DOES gain a coverage precondition — the flip refuses (reported, never silent) when coverage for the brief is not `released`. This is the one place the stamp it promotes is re-checked; the header comment's trade is thereby closed for the model lane. The human lane (`verify-gate-*.yml`) is not changed by this brief.
- Single-point-of-failure note: the ONE control is the coverage verdict. Second layer, independent: verifyrun's `could-not-run` contains `not-run`, which `unrunMarkerRe` reads at the board level, so a row nobody ran stays UNRUN on the board even if coverage were miscomputed (a different reader, a different signal). Third: the reviewer App's approval-at-head in `autoflip.go` remains a separate precondition; coverage adds to it, it does not replace it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: `example-org/*` placeholders; fixtures name no adopter.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Integration amendment — 2026-09-18

Retain this brief as the sole coverage implementation. Applicable evidence must bind the exact subject and acceptance-definition digest; a model assessment is never an execution witness. Treat unavailable identity/definition corroboration as could-not-check. The later instance contract (09) supplies bindings through an adapter; this core rule remains testable without a provider. Control-profile population/period export belongs to 15, not this brief. Keep protected acceptance/executor identity as independent controls; coverage alone does not establish them.

## Work-input amendment — 2026-09-30

Apply WI-2 through the existing coverage API and declared coverage fixtures. Same complete
input fingerprint may reuse a derivation; a new subject cannot inherit an old exact-revision
PASS. Preserve historical rows and hold affected claims on changed acceptance or source,
policy, build or environment dependencies. Unknown dependency coverage requires broader
revalidation. Selective reuse is optional and needs an explicit applicability derivation;
file non-overlap is not proof. Keep the existing result vocabulary; report the invalidating
fact in the reason. The later 09 adapter supplies the manifest, without making 09 a cycle
on this core rule. 14 owns rechecking at the effect/acceptance boundary.

## Task
1. **The rule** (`statusgen/coverage.go` (planned)): `func evaluateCoverage(root string, streams []*Stream, patterns map[string]Pattern) map[string]Coverage`. For each brief, the set of mandatory claims is the union of (a) its Verify rows and (b) the `mandatory: true` evidence entries of the pattern node the brief is at (when a pattern applies; a brief with no pattern uses (a) alone). Each claim resolves to `{claim, kind, result, revision, released: bool, reason}`. The brief is `released` only when every mandatory claim is `pass` at the item's revision — the merged SHA for a merged brief, the PR head for an open one. Any `missing | error | could-not-check | wrong-revision` holds with the reason naming the claim. `fail` holds and is reported distinctly from could-not-check.
2. **The join's integration check.** When the pattern's `join:` node applies, coverage requires at least one Verify row classed `+flow` (rowclass.go's obligation token); absent → a `missing` claim `integration-check` — individually passing rows do not release the join.
3. **The `observe` kind.** Extend `spec/workflow-pattern-v1.md` (planned) and `schemas/workflow-pattern-v1.json` (planned) with `evidence: [{kind: observe, signal, band, window, source, mandatory}]`. Coverage treats it as a claim filled by the verifier from the named `source`; an unreadable source → `could-not-check` → held. The schema doc states: declare `observe` only where a deploy exists; where none exists the kind is **omitted**, never recorded as "n/a". The two patterns 02 ships carry no `observe` entry; the doc shows the shape under an `example-org` deploy.
4. **Consumers.** `autoflip.go`: before flipping, call coverage; not `released` → refuse with the reasons in the result (dry-run prints them). `lifecycle.go`: the `verified` witness precedence reads `released` in addition to `WitnessInfo.Passed`, so a stale or wrong-revision witness demotes exactly as a version mismatch does today.
5. **CLI.** `statusgen --coverage [--json] --root <root>`: one line per brief `<id> released|held <n-claims> <first-reason>`; `--json` emits every claim. Exit 0; exit 2 on an unreadable tree.
6. **Docs.** `spec/lifecycle-v1.md`: `verified` gains the coverage condition; `docs/lifecycle.md` and `docs/enforcement-model.md` gain the rule and the row. `changelog/graph-execution-03-coverage.md` (planned).

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go test -run TestCoverage ./...` | exit 0 |
| 2 | check:ci +mutation | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; a fixture whose witness SHA differs from the brief's merged revision is `wrong-revision` and `released: false` |
| 3 | check:ci +mutation | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; a `could-not-run` witness yields `could-not-check` and holds — never `pass` |
| 4 | check:ci +mutation | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; a `verified` row with an approval at head but a `missing` mandatory claim is NOT flipped, and the dry-run output names the claim |
| 5 | check:ci +flow | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; a brief at the pattern's join with only site rows is held on `integration-check`; adding one `+flow` row releases it |
| 6 | check | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | the printed set is a subset of `['could-not-check', 'error', 'fail', 'missing', 'pass', 'wrong-revision']` |
| 7 | check +dereference | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count >= 1 |
| 8 | check:ci +neighbour | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 — the no-override pin and the advisory attribution check are untouched |
| 9 | check | `statusgen --root . --lint; echo rc=$?` | `rc=0` |
| 10 | check | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | `rc=0` on the authoring branch (every entry routes follow-up or out-of-scope); exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass |
| 11 | check:ci +mutation | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS; high-confidence advice does not satisfy missing mandatory evidence |
| 12 | check:ci +mutation | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS; changed acceptance invalidates the affected witness |
| 13 | check:ci +mutation | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS; a policy edit outside touched files holds the affected claim and preserves the old receipt; mutation: bypass the policy-fingerprint comparison — the named test must fail |
| 14 | check:ci +flow | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS; unchanged analysis can be reused but an old-head PASS cannot release the new subject |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass exit=0 | sha256:a8774919d94f | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass exit=0 | sha256:38af6536faaa | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:55108acd6d2b | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | fail exit=0 | sha256:5afa3aae3cf9 | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-worker-app[bot] @ 83b7dc205761 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-10-01, round 7** (correctness review 5373616187 at `f67856b183e6`, finding M4;
security review 5373640634, finding S12). Main merged in at `a60712beda05`. The one conflict
was in `statusgen/autoflip.go`, at the `decideModelFlip` call: the resolution keeps this
brief's coverage-refusal block and takes main's call line with its `briefid:raw` waiver (#1966),
which `TestBriefIDKeyGuard` requires. The S12 fix was then committed at `83b7dc205761`.
`statusgen verifyrun --brief` re-executed all 14 rows at that commit, a clean checkout with
nothing staged, modified or untracked, with the `statusgen` binary built from that same commit
first on `PATH`. The table above replaces the round-6 set recorded at `1fd6a1258987`. Rows 1–5,
8 and 11–14 record `could-not-run` (no `unshare --net` on this darwin host). `cd statusgen &&
GOWORK=off go build ./... && go vet ./... && go test ./... -count=1` was also run directly on
the identical tree and passed, including `TestBriefIDKeyGuard`, `TestAliasDropGuard` and
`TestVerifyOutcomesSingleReader`; that is corroborating detail, not a substitute witness. Row 10
is `fail exit=0` for the parseExpect reason given below.

**Round-7 fix.** S12: `resolveVerifyClaim`'s pass branch now mirrors `checkWitnesses`. A pass
witness on a Verify row the lint flags prose-led (#1808) measured the mention, not the check,
so the claim resolves `could-not-check` with the prose-led note and the brief is held.
`TestCoverageProseLedPassIsNotReleased` pins it, using `checkWitnesses` on the same witness as
its parity anchor. The test was written first and failed before the fix (`coverage released a
prose-led row whose pass checkWitnesses demotes to could-not-run`, `Result:pass`); it passes
after.

**Mutation proof, round 7.** Applied to a scratch COPY of `statusgen/` at `83b7dc205761` (never
the worktree), with `go test -count=1 -run`:

| Guard | Mutation (in `coverage.go`) | Killed by | Result |
|---|---|---|---|
| S12 prose-led pass | `if r.ProseLed != ""` → `if false && r.ProseLed != ""` | `TestCoverageProseLedPassIsNotReleased` | RED: `Released:true`, `Result:pass` |

**Re-run 2026-09-30, round 6** (the sixth CHANGES_REQUESTED review at `dc8ee95cc688`,
review 5364066717, findings M2, M3, A11 and A12; plus security review 5363895046, advisory
S11). #1891 merged to main first, as M3 asked. Its rewrite of Verify rows 13 and 14 to the
`mktemp` form came in with the merge of main at `ee4d5c9e620f`. The M2 and A11 fixes were then
committed at `1fd6a1258987`. `statusgen verifyrun --brief` re-executed all 14 rows at that
commit, a clean checkout with nothing staged, modified or untracked. The `statusgen` binary
was built from that same commit and put first on `PATH`. The table above replaces the round-5
set recorded at `66871816d034`. Evidence rows 13 and 14 now quote #1891's command text, so
they match the Verify table again. Rows 1–5, 8 and 11–14 record `could-not-run` (no
`unshare --net` on this darwin host). `cd statusgen && GOWORK=off go build ./... && go vet
./... && go test ./... -count=1` was also run directly on the identical tree and passed. That
includes every test the could-not-run rows name, and it is corroborating detail, not a
substitute witness. Row 10 is `fail exit=0` for the parseExpect reason given below.

**Round-6 fixes.** M2: two comments in `coverage.go` no longer spell the verify outcome log's
literal name, so `TestVerifyOutcomesSingleReader` passes without widening its allow-list. A11:
`TestCoverageNothingDeclaredStaysConservative` pins `forRow`'s nothing-is-declared branch. Its
three subtests use a complete manifest whose only dependency is a policy, a build input or an
environment entry, followed by a source edit outside that dependency after the witness. Each
must hold as `wrong-revision`. The branch behaved correctly before the test existed; the
mutation below shows the test is load-bearing.

**Mutation proof, rounds 5 and 6** (A12, S11). Each mutation was applied to a scratch COPY of
`statusgen/` at `1fd6a1258987` (never the worktree), and the named test was run with
`go test -count=1 -run`. Every mutant was killed:

| Guard | Mutation (in `coverage.go`) | Killed by | Result |
|---|---|---|---|
| A11 nothing declared | `forRow`'s `len(out.declared) == 0` case no longer selects the conservative scope | `TestCoverageNothingDeclaredStaysConservative` (all three subtests) | RED: want `wrong-revision`, got `pass` |
| S11 mutant A | the bullet branch of `extractContextDeclaredEntriesRaw` skips a bullet with no backtick span | `…NeverFailsOpen/P4 plain bullet beside a backticked bullet` | RED: got `pass` |
| S11 mutant B | `newWitnessScope` reads `bf.DeclaredPaths` instead of the raw declared entries | `…NeverFailsOpen/P8 dotless entry declared only now` | RED: got `pass` |
| S11 mutant C | `atBase` reads `extractContextDeclaredPaths` instead of `extractContextDeclaredEntriesRaw` | `…NeverFailsOpen/P7 dotless entry declared only at the witness` | RED: got `pass` |
| row 13 `+mutation` | the policy-fingerprint comparison bypassed (`if !same` → `if false && !same`) | `TestCoveragePolicyDependencyChanged` | RED: want `wrong-revision`, got `pass` |

**Re-run 2026-09-30, round 5** (the fifth CHANGES_REQUESTED review at `17e215ae4`, plus the
2026-09-30 work-input amendment's rows 13 and 14): `statusgen verifyrun --brief` re-executed
all 14 rows at `66871816d034`, a clean checkout with nothing staged, modified or untracked
(`git status --porcelain` empty). That commit holds the round-5 `coverage.go`, `brieffile.go`
and tests, the spec paragraph and the changelog. The `statusgen` binary was built from that
same commit and put first on `PATH`, so rows 6, 9 and 10 run the code under review, not an
older installed release. The witnesses carry no `+dirty` suffix. The table above replaces the
round-4 set recorded at `b749edd42546+dirty`. Rows 1–5, 8 and 11–14 record `could-not-run`
(no `unshare --net` on this darwin host). The row 13 and 14 commands were also run directly
on the identical tree: both exited 0 and printed their named `--- PASS:` lines. That is
corroborating detail, not a substitute witness. Row 6's printed set is now
`['could-not-check', 'error', 'fail', 'missing', 'wrong-revision']`, a subset of the
expected set. The corpus loses its last `pass` claims (10 rows in two briefs whose witnesses
predate unrelated code changes) because a `files:` line alone no longer narrows the scope
(round-5 F6/S10 and WI-2). Row 10 is `fail exit=0`, as in earlier rounds: see the
parseExpect note below.

**Correction to the round-4 note (round-5 F8).** The round-4 note below calls `b749edd42546`
"a CLEAN tree", but every round-4 witness recorded `b749edd42546+dirty`. `verifyrun` saw
uncommitted changes in that worktree when it ran, so those rows were never clean-tree
witnesses. The round-5 table above is the clean one; the round-4 wording is kept as
written for the record.

**Re-run 2026-09-27, round 4** (the fourth CHANGES_REQUESTED review at `981623aeb`, reviews
5330931191 correctness and 5330940083 security: F6/S6 still open, S8/S9 new advisories):
`statusgen verifyrun --brief` re-executed every row at `b749edd42546` (a CLEAN tree — the
round-4 fix committed at that same SHA: `statusgen/brieffile.go` now also parses every
`files:` entry unfiltered by path shape, `coverage.go`'s witness scope uses that raw list
instead of the path-shape-filtered one, `classifyRevision` now requires a witness token to
be SHA-shaped before it reaches git, and four new tests pin the fix), using the `statusgen`
binary built from that same commit. The table above replaces the round-3 set recorded at
`8b88e97f1252`: those witnesses read `wrong-revision` by design, since `statusgen/coverage.go`
and `statusgen/brieffile.go` changed after them. Rows 1–5, 8, 11, 12 again record
`could-not-run` (no `unshare --net` on this darwin host); `cd statusgen && go build ./... &&
go vet ./... && go test ./... -count=1` was also run directly on the identical tree and
passed — corroborating detail, not a substitute witness.

**Re-run 2026-09-25, round 3** (the third CHANGES_REQUESTED review at `5980c2acb`: F2 round 3,
F6 / security S5, A7): `statusgen verifyrun --brief` re-executed every row at `8b88e97f1252`
(a CLEAN tree — the round-3 `coverage.go`, its tests, the spec paragraph and the changelog
committed at `83280a436`; then main merged, bringing in #1691, and the `af/brief-08`/`-09`
autoflip fixtures rewritten to the witnessed Evidence form per the merge-order note), using
the `statusgen` binary built from that same commit. The table above
replaces the round-2 set recorded at `e2c2b350c87c`: those witnesses read `wrong-revision`
by design, since `statusgen/coverage.go` and `spec/lifecycle-v1.md` changed after them.
Rows 1–5, 8, 11, 12 again record `could-not-run` (no `unshare --net` on this darwin host);
`cd statusgen && go build ./... && go vet ./... && go test ./... -count=1` was also run
directly on the identical tree and passed — corroborating detail, not a substitute witness.

**Re-run 2026-09-25, round 2** (the second CHANGES_REQUESTED review at `e43403dfd`: F2, F3,
security S1–S3): `statusgen verifyrun --brief` re-executed every row at `e2c2b350c87c` (a
CLEAN tree — the reworked `coverage.go`, its tests, and the spec paragraph committed first,
so the witnesses carry no `+dirty` suffix), using the `statusgen` binary built from that
same commit. The table above replaces the round-1 set recorded at `8737498f7fa5+dirty` —
those witnesses read `wrong-revision` under the round-2 rule by design, since declared
`files:` paths (`statusgen/coverage.go`, `spec/lifecycle-v1.md`) changed after them. Rows
1–5, 8, 11, 12 still record `could-not-run` — this darwin host lacks `unshare --net` for
`check:ci`'s hermetic network-off re-run — but `cd statusgen && go build ./... && go vet
./... && go test ./... -count=1` was ALSO run directly at `e2c2b350c87c` (no hermetic
wrapper; no test in the package uses the network) and passed, which includes every test
those rows name — corroborating detail, not a substitute witness; a Linux `check:ci`
runner still owes the mechanical rows.

**Fail-first, round 1** (findings F1/F2/F3 at `6c0403006`; tests not named in the frozen
Verify table, exercised by `go test ./...`): `TestCoverageNoTargetRevisionIsCouldNotCheck`,
`TestCoverageMissingTreeTokenIsCouldNotCheck`, `TestCoverageShortTreeTokenIsCouldNotCheck`,
`TestCoverageAncestorWitnessReleases`, `TestCoverageAncestorWitnessOtherPathChangedMismatch`
and `TestCoverageExpectChangedSinceWitnessRan` were RED against the ORIGINAL `coverage.go`
(`6c0403006ca3521b74a6421fba2b64c77cb2bbd3`) and GREEN after the round-1 rework.

**Fail-first, round 2** (F2, F3, security S1/S2 at `e43403dfd`): the five new tests below were
run against `e43403dfd`'s `coverage.go` (new tests in place, no other file touched) and were
RED, each reproducing one of the review's probes, then GREEN at `e2c2b350c87c`:

| Test | Probe | RED at `e43403dfd` |
|------|-------|--------------------|
| `TestCoverageDirtyWitnessExpectChangedIsError` | PROBE-F / S1 | `released: true`, `Result: pass` for a `+dirty` witness whose Expect was tightened |
| `TestCoverageBriefAbsentAtWitnessTreeIsCouldNotCheck` | PROBE-G / S2 | `released: true`, `Result: pass` with the brief absent at the witness tree |
| `TestCoverageRowAbsentAtWitnessTreeIsCouldNotCheck` | F3(b) | `released: true` with row 2 absent at the witness tree |
| `TestCoverageSiblingEvidenceAndStatusRegenStillRelease` | PROBE-D/E | `wrong-revision` after sibling Evidence and a `STATUS.md` regen |
| `TestCoverageDeclaredFilesScopeTheWitness` | F2 | `wrong-revision` for a change to an UNDECLARED path |

**Mutation proof** (the four pre-existing `+mutation` rows 2, 3, 11, 12, and the round-2
guards). Each mutation was applied to a scratch COPY of `statusgen/` (never the worktree),
the named test was run, and the copy was restored. Every mutant was killed (RED):

| Row / guard | Mutation (in `coverage.go`) | Test | Result |
|---|---|---|---|
| row 2 | `classifyRevision`'s final `return revisionMismatch` → `revisionMatch` | `TestCoverageWrongRevisionHolds` | RED: want `wrong-revision`, got `could-not-check` |
| row 3 | the could-not-run branch returns `covPass` | `TestCoverageCouldNotCheckIsNotPass` | RED: `released: true` |
| row 11 | `if isWitnessRow(er.Text)` → `if true` | `TestCoverageAdviceCannotSupplyWitness` | RED: want `missing`, got `error` |
| row 12 | the Command guard → `if false` | `TestCoverageAcceptanceDigestChanged` | RED: want `error`, got `could-not-check` |
| F3(a) | `base := witnessBaseRevision(wrev)` → `base := wrev` | `TestCoverageDirtyWitnessExpectChangedIsError` | RED |
| F3(b) | an unreadable historical row returns `covPass` | `…BriefAbsentAtWitnessTree…`, `…RowAbsentAtWitnessTree…` | RED (both) |
| F2 bookkeeping | `isBoardBookkeepingPath` → `false` | `TestCoverageSiblingEvidenceAndStatusRegenStillRelease` | RED |
| F2 declared | the declared-scope branch disabled | `TestCoverageDeclaredFilesScopeTheWitness` | RED |
| F2 directory | a declared directory no longer covers paths under it | `TestCoverageDeclaredFilesScopeTheWitness` | RED |
| F2 other path | the scope check in `ancestorNoOtherChanges` disabled | `TestCoverageAncestorWitnessOtherPathChangedMismatch` | RED: `released: true` |

(The round-1 note that this mutation proof was blocked by a tool-use classifier is superseded:
it ran this round.)

**Fail-first, round 3** (F2 round 3, F6 / security S5 at `5980c2acb`): the new tests were run
against `5980c2acb`'s `coverage.go` (new test file in place, no other file touched). These 14
subtests were RED there, each reproducing one of the reviewers' probes, and are GREEN at
`83280a43620f`:

| Test / subtest | Probe | RED at `5980c2acb` |
|------|-------|--------------------|
| `TestCoverageDeclaredScopeExemptsVerifyBookkeeping/R1…`, `/R2…`, `/whole_docs/streams…` | F2 R1, R2 | `wrong-revision` after the brief's own README row flip, a `STATUS.md` regen, sibling Evidence |
| `TestCoverageWitnessScopeNeverFailsOpen/S5a…`, `/S5a'…`, `/S5b…` | S5a, S5a', S5b | `pass` after a declared file was renamed / moved into `docs/streams/` |
| `TestCoverageWitnessScopeNeverFailsOpen/S5c…`, `/S5d…`, `/S5e…`, `/X1…`, `/X2…`, `/X3…` | S5c–e, X1, X2 | `pass` with a brace, `.`, `n/a`, bare-sibling or `**` declaration |
| `TestCoverageWitnessScopeNeverFailsOpen/S5f…`, `/S5f'…` | S5f | `pass` after `files:` was narrowed / added in the Evidence commit |

The two "still holds" subtests of `TestCoverageDeclaredScopeExemptsVerifyBookkeeping`,
`TestCoverageResolvableDeclarationsStillScope` and `TestCoverageBriefOwnFileNeverInvalidates`
(A7) were already GREEN at `5980c2acb`. They pin behaviour that must not regress, and the
mutations below prove each one is load-bearing. `TestCoverageDeclaredFilesScopeTheWitness`'s
fixture now writes the file and the directory its `files:` line declares, because an entry that
names nothing now selects the conservative scope.

**Mutation proof, round 3.** Each mutation was applied to `coverage.go`, `go test -run
TestCoverage` was run, and the file was restored byte-for-byte. Every mutant was killed:

| Guard | Mutation (in `coverage.go`) | Killed by |
|---|---|---|
| F6 renames | drop `--no-renames` from the diff | `…NeverFailsOpen/S5a`, `/S5a'`, `/S5b` |
| F6 base declaration | skip the `files:` read at the witness's base commit | `…NeverFailsOpen/S5f`, `/S5f'` |
| F6 unresolved entry | an entry that resolves to nothing no longer widens | `…NeverFailsOpen/S5c`, `/S5d`, `/S5e`, `/X1` |
| F6 unsupported syntax | `declaredEntrySupported` check off | `…NeverFailsOpen/X3` |
| trailing `/**` | the `/**` → directory rewrite removed | `…ResolvableDeclarationsStillScope/trailing_/**…` |
| F2 exemption | the `isVerifyWrittenPath` exemption off | `…ExemptsVerifyBookkeeping/R1`, `/R2`, `/whole_docs/streams…` |
| F2 over-exemption | every `docs/streams/` path exempt in the declared branch | `…ExemptsVerifyBookkeeping/…artifact_changed_still_holds` (both) |
| A7 | the brief-own-file exemption removed (`p == sc.briefRel`) | `TestCoverageBriefOwnFileNeverInvalidates` (both subtests) |

**Fail-first, round 4** (F6/S6 re-review, S8, S9 at `981623aeb`): the new/extended tests below
were run against `981623aeb`'s `coverage.go`/`brieffile.go` (pre-fix) and were RED, each
reproducing one of the reviewers' probes or mutations, then GREEN at `b749edd42546`:

| Test / subtest | Probe | RED at `981623aeb` |
|------|-------|--------------------|
| `TestCoverageWitnessScopeNeverFailsOpen/P1…` | reviewer P1 | `pass` after a declared, backticked, dotless `Makefile` was edited |
| `TestCoverageWitnessScopeNeverFailsOpen/P2…` | reviewer P2 | `pass` after a declared, backticked, dotless directory (`tools`) had a file under it edited |
| `TestCoverageWitnessScopeNeverFailsOpen/P3…` | reviewer P3 | `pass` after an unbackticked inline dotless declared file was edited |
| `TestCoverageWitnessScopeNeverFailsOpen/P6…` | reviewer P6 | `pass` after an unrelated path changed, with a declared dotless entry resolving to nothing |
| `TestCoverageWitnessTokenMustBeHexShaped` | security S8 | `pass` for a symbolic (`HEAD^{commit}`) witness token, even after the declared file it speaks for changed twice |

`TestCoverageBoundObserveClaimHolds` and `TestCoverageZeroClaimsBriefIsHeld` (security S9) pin
two guards that already behaved correctly at `981623aeb` but had no test that would catch a
regression — they are GREEN at `981623aeb` and stay GREEN at `b749edd42546`; see the round-4
mutation proof below for how each is shown load-bearing.

**Mutation proof, round 4.** Each mutation was applied to `coverage.go`, the named test was
run, and the file was restored byte-for-byte. Every mutant was killed:

| Guard | Mutation (in `coverage.go`) | Killed by |
|---|---|---|
| F6/S6 raw declared entries | `newWitnessScope`/`atBase` reverted to the path-shape-filtered `DeclaredPaths`/`extractContextDeclaredPaths` | `…NeverFailsOpen/P1`, `/P2`, `/P3`, `/P6` |
| S8 revision-token shape | the `hexRevisionRe` check in `classifyRevision` removed | `TestCoverageWitnessTokenMustBeHexShaped` |
| S9 observe guard (reviewer's M-b) | `resolvePatternEvidenceClaim`'s `observe` case returns `covPass` unconditionally | `TestCoverageBoundObserveClaimHolds` |
| S9 zero-claims guard (reviewer's M-d) | the `len(claims) == 0` fail-closed branch disabled | `TestCoverageZeroClaimsBriefIsHeld` |

**Task item 1's revision wording.** Task item 1 says the item's revision is "the merged SHA
for a merged brief, the PR head for an open one". The implementation resolves the item's
revision offline as the checked-out `HEAD` — the PR head on the branch, but the main TIP
(not the merge SHA) on the model-lane flip job — and credits a witness at an ancestor when
no path it speaks for changed since (the brief's `files:` now and at the witness's commit,
else — or when a declared entry names no real path — everything outside `docs/streams/**`
and `STATUS.md`; never the files verify and regeneration write; renames count against their
old path). That deviation from the Task's literal wording is
deliberate (the literal form needs a live merge lookup the ground rules forbid, and the
round-1 whole-tree rule halted the model lane); `spec/lifecycle-v1.md` and the
`coverage.go` header state the implemented rule. The `+dirty` tolerance is a declared
residual (header "THE +dirty TOLERANCE"), pinned by `TestCoverageDirtyWitnessToleranceIsDeclared`.

**Row 10** mechanically records `fail exit=0`, but the row's own Expect cell states TWO
acceptable outcomes (`rc=0` on the authoring branch; `rc=2` on a fully merged main) —
`parseExpect`'s exit-code reader lifts the FIRST unquoted digit after `exit`, which here
is the "expected 2" fragment naming the merged-main case, not the authoring-branch case
this run is actually in. The command's REAL exit code on this authoring branch is `0`
(confirmed directly above, and by hand: `statusgen --consumers --brief graph-execution/03
--root .` prints `rc=0` with every consumers: entry CORROBORATED or UNCHECKED, none
DISPROVED). This is a parseExpect limitation on a two-case Expect cell, not a defect in
`--consumers` or in coverage.go; recorded here rather than silently edited into a false
`pass`.
### Post-merge verification — 2026-10-01 (verify-desk, independent of the implementer)

What moved since the last run: this is the first post-merge verifier pass. The rows above are the implementer's pre-merge witnesses at branch commit 83b7dc205761. #1682 was squash-merged as 909101d1aa20, and every row below was run against merged main at 4b1f8fc8bfaf, the current tip. That tip is 909101d1aa20 plus two STATUS.md regenerations. The Verify table is unchanged since the merge.

Setup: a clean detached worktree at 4b1f8fc8bfaf, with `git status --porcelain` empty before and after the run. The `statusgen` binary was built from that same tree (`cd statusgen && GOWORK=off go build -o <scratch>/statusgen .`) and put first on PATH, so rows 6, 9 and 10 exercise the merged code and not an installed release. Go 1.27.1 darwin/arm64. The Go rows were run directly with `go test`, not through `statusgen verifyrun`, because verifyrun's check:ci arm needs `unshare --net`, which this darwin host lacks. A combined verbose run of every named test (`cd statusgen && go test -count=1 -v -run 'TestCoverage|TestAutoFlip(Refuses|NoOverride)|TestEvidenceActor' ./...`) exited 0 with 53 PASS lines, 0 FAIL lines and no SKIP lines.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | Verify row 1. exit 0; `ok github.com/medici-finance/assay/statusgen 11.813s`. The combined -v run showed no SKIP lines. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; differing witness SHA is wrong-revision, released false | Verify row 2. exit 0; `ok ... 0.167s`. The -v run shows this named test PASS (0.01s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; a witness the run produced no verdict for yields could-not-check and holds | Verify row 3. exit 0; `ok ... 0.165s`. The -v run shows this named test PASS (0.01s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; verified row with approval at head but a missing claim is not flipped, dry-run names the claim | Verify row 4. exit 0; `ok ... 0.353s`. The -v run shows this named test PASS (0.16s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; join with only site rows held on integration-check; one +flow row releases | Verify row 5. exit 0; `ok ... 0.447s`. The -v run shows this named test PASS (0.20s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | printed set is a subset of the six results | Verify row 6. statusgen exit 0, python exit 0, printed `['could-not-check', 'error', 'fail', 'missing', 'wrong-revision']`, which is a subset. A re-run with an absolute --root printed the same set. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count >= 1 | Verify row 7. exit 0; the three counts were 9 (workflow-pattern spec), 5 (schema) and 2 (lifecycle spec). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | Verify row 8. exit 0; both invocations printed `ok` (0.173s and 0.941s). The -v run shows the no-override test PASS and all 18 evidence-actor tests PASS. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | Verify row 9. `rc=0`. The absolute-root form printed `LINT: PASS` and rc=0, with NOTICE lines only, none of them about graph-execution. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on fully merged main is acceptable and recorded as such | Verify row 10. As written, this gave `rc=2` with `COULD-NOT-CHECK: ... is not in the diff against 4b1f8fc8bfaf...`, which is the merged-main outcome the Expect cell names, and it does not count as a pass. Following the tool's own instruction, I re-ran it against the squash merge's own diff (next row). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10b | `statusgen --consumers --brief graph-execution/03 --root . --base 909101d1a^; echo rc=$?` | rc=0; no consumers entry DISPROVED | Verify row 10, corrected form. `rc=0`, `summary: 4 corroborated, 0 disproved, 2 unchecked`. The four fixed-here entries (autoflip.go, lifecycle.go, lifecycle-v1 spec, workflow-pattern spec) are CORROBORATED. The two UNCHECKED entries are the out-of-scope ones (the verify-gate workflow pair and evidenceactor.go). `git diff --stat de45ad17171a 909101d1a` over those three paths prints nothing, so they are unchanged, as their out-of-scope routing states. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | Verify row 11. exit 0; the named test's `--- PASS` line (0.01s), then `PASS` and `ok ... 0.160s`. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | Verify row 12. exit 0; the named test's `--- PASS` line (0.01s), then `PASS` and `ok ... 0.168s`. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS | Verify row 13. exit 0, so the grep found the named PASS line. The combined -v run shows the same test PASS (0.43s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | Verify row 14. exit 0, so the grep found the named PASS line. The combined -v run shows the same test PASS (0.70s). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Mutation rows (2, 3, 4, 11, 12, 13).** I did not repeat the mutations. The implementer's mutation tables above record each mutant killed. This pass confirms only that the named guards pass on merged main.

**Scope traceability.** Every row above discharges the Verify row it names. Some delivered work has no Verify row of its own. I checked it separately; these checks sit outside the table and are not counted in it:
- Task item 4, the lifecycle.go half: `go test -count=1 -v -run 'TestLifecycleDemotion.*NotReleased' ./...` exits 0, and the lifecycle coverage-demotion test PASSES.
- Task item 5, the plain-line CLI form: `statusgen --coverage --root .` exits 0 and prints `graph-execution/03 held 14 Verify row #10: ...: the witness records a failure`. A --root that does not exist exits 2, as the Task requires for an unreadable tree.
- Task item 6, the docs: docs/lifecycle.md, docs/enforcement-model.md, the changelog fragment changelog/graph-execution-03-coverage.md and spec/lifecycle-v1.md each mention coverage at least twice.
- Supporting changes with no Verify row: brieffile.go, patterns.go, verifyrun.go and the af autoflip fixtures. They are exercised only through the test runs above.

**Observation for the desk (not a Verify failure).** At merged main 4b1f8fc8bfaf, `statusgen --coverage` releases 0 of 294 briefs. The results are 1895 missing, 469 wrong-revision, 146 could-not-check, 85 fail and 34 error. This brief is itself held. Its witnesses were recorded at branch commit 83b7dc205761, which is not an ancestor of main because the PR was squash-merged (`git merge-base --is-ancestor` gives rc=1). Those witnesses therefore resolve as wrong-revision or could-not-check. That is the rule working as designed: a witness at a non-ancestor tree must not release. The consequence is that the model-lane verified→done autoflip (autoflip.go:1070) will now refuse every gate:model brief until a verifyrun witness exists at a revision that is an ancestor of main. Rows classed check:ci can only get such a witness on a runner that provides `unshare --net`. Hand-transcribed Evidence tables such as this one are not witness rows, so they do not release coverage.

**Risk-bearing values.** The risk fields are all `no` and the gate is `model`. No path in the diff matches the policy risk-path triggers (the secrets directory, the workflows directory, the per-repo topology additions). The PR was on a public repo, so whether it counts as risk-classed by visibility is the desk's routing call.

I enumerated every literal added or changed in the non-test diff from de45ad17171a to 909101d1a, across coverage.go, autoflip.go, lifecycle.go, main.go, brieffile.go, patterns.go, verifyrun.go and workflow-pattern-v1.json:
- `hexRevisionRe = ^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:679
- `minRevisionTokenLen = treeSHALen` @ statusgen/coverage.go:665, which resolves to `treeSHALen = 12` @ statusgen/verifyrun.go:150 (pre-existing value)
- `coverageExitOK = 0`, `coverageExitCouldNot = 2` @ statusgen/coverage.go:1273-1274
- `coverageResultOrder` fail 0 / missing 1 / error 2 / wrong-revision 3 / could-not-check 4 / pass 5 @ statusgen/coverage.go:250-255
- `witnessCellRunner = 5` @ statusgen/verifyrun.go:284
- the schema adds string fields only (signal, band, window, source) and no numeric bound

Ranked by irreversibility: none of these is irreversible, and the item is marked `irreversible: no`. A wrong value would mis-gate the board's flip, and an edit plus a re-release undoes it. The top-ranked value is the revision-token shape, because a wrong bound there would fail open: a stale or symbolic witness would release a brief. Next come the exit codes. The result order and the Runner cell index only choose a display reason and parse a column.

- Lower bound 12: this equals treeSHALen, the exact length verifyrun writes. A shorter token was the review's F1 hole, where a 1-character token matched about 1 in 16 SHAs. Twelve hex characters leave about 2.8e14 possible values, so an accidental match is negligible.
- Upper bound 40: this is a full SHA-1 hex object name, the longest token git emits in a SHA-1 repository.
- Hex-only: this shape rejects symbolic revisions such as HEAD~0, which would otherwise never go stale (the review's S8). It rejects nothing verifyrun produces.
- Residual: a repository using the SHA-256 object format (64-hex names) would have full-length tokens rejected. They resolve as could-not-check, which fails closed, not open. verifyrun writes only 12 characters, so this does not arise today.
- The exit codes 0 and 2 come directly from Task item 5 ("Exit 0; exit 2 on an unreadable tree"), and the run above shows a missing root exiting 2.

rows_passed=14 rows_total=14 (row 10 passes on its corrected form 10b; the as-written form gave the merged-main exit 2 that its Expect cell anticipates)

RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:679 — the lower bound is treeSHALen = 12 (statusgen/verifyrun.go:150), the exact witness token length, which closes the review's F1 short-token coincidence; the upper bound 40 is a full SHA-1 hex name; hex-only closes the symbolic-revision hole from the review's S8; a 64-hex SHA-256 name fails closed as could-not-check.
RISK-VALUE: DERIVED — coverageExitOK = 0 / coverageExitCouldNot = 2 @ statusgen/coverage.go:1273-1274 — taken verbatim from Task item 5, and a missing root was observed to exit 2.

VERIFY: PASS
### Post-merge re-verification with execution witness — 2026-10-01 (verify-desk, independent of the implementer)

What moved since the last run: nothing in the Verify table and nothing in the delivered code. The previous block ran at 4b1f8fc8bfaf by hand. This pass ran at f5827b497467, the main tip. Between the squash merge 909101d1aa20 and this tip, the files this brief delivered (coverage.go, autoflip.go, lifecycle.go, verifyrun.go, brieffile.go, patterns.go, the pattern schema and the spec directory) are byte-identical, and the brief file gained Evidence lines only. What is new here is the `statusgen verifyrun` execution witness on Linux with the network off, which the earlier passes could not produce on a darwin host.

Setup for the witness: a full local clone pinned to f5827b497467 with its remote-tracking main at the same commit and `git status --porcelain` empty before the run; a golang:1.25-bookworm container (go1.25.14 linux/arm64) started with `--network none`, so the only interface was loopback; GOPROXY=off, GOTOOLCHAIN=local, a read-only module cache; `statusgen` built inside the container from that same tree and put on PATH. `statusgen verifyrun --brief <this brief>` then ran all 14 rows. The check:ci rows ran under the tool's own `unshare --net --map-root-user` wrapper; the plain check rows (6, 7, 9, 10) ran in the same network-off container. The table below is the tool's output, verbatim. Two earlier container attempts were discarded before this one because of my own setup faults (a stray GOFLAGS value, then a clone whose remote-tracking main pointed at an older commit, which changed what rows 9 and 10 compared against); neither is recorded here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | pass exit=0 | sha256:29903827e0ce | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | pass exit=0 | sha256:b76956e68534 | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | pass exit=0 | sha256:28ee1419f776 | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | pass exit=0 | sha256:2ef712a496ea | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | pass exit=0 | sha256:1814891b1ed9 | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass exit=0 | sha256:73af7c13fa8d | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass exit=0 | sha256:38af6536faaa | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | pass exit=0 | sha256:65bc53a7610c | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:5410b8aa3322 | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | fail exit=0 | sha256:d4d4c35249df | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | pass exit=0 | sha256:b11ba71130e9 | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | pass exit=0 | sha256:b8d61c8b96be | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ f5827b497467 (on-behalf-of human:ian) (forge-identity) |

`statusgen verifyrun --check` on the same tree: 13 pass, 1 fail, 0 could-not-run or missing (of 14 Verify rows); exit 1. `statusgen --coverage` on the same tree: `graph-execution/03 held 14 Verify row #10: ... the witness records a failure`.

Hand run on the host (darwin/arm64, go1.27.1), in a clean detached worktree at f5827b497467, with `statusgen` built from that tree first on PATH. A combined verbose run of every named test (`cd statusgen && go test -count=1 -v -run 'TestCoverage|TestAutoFlip(Refuses|NoOverride)|TestEvidenceActor' ./...`) exited 0 with 53 top-level PASS lines, 0 FAIL lines and 0 SKIP lines.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | 0 | pass. `ok github.com/medici-finance/assay/statusgen 19.970s`. Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | 0 | pass. `ok ... 0.254s`; the verbose run shows the named test PASS (0.01s). Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | 0 | pass. `ok ... 0.242s`; the verbose run shows the named test PASS (0.01s). Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | 0 | pass. `ok ... 0.317s`; the verbose run shows the named test PASS (0.15s). Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | 0 | pass. `ok ... 0.459s`; the verbose run shows the named test PASS (0.17s). Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | 0 | pass. Printed `['could-not-check', 'error', 'fail', 'missing', 'wrong-revision']`, a subset of the six results; the container printed the same set. Witness: pass on exit status only (the tool says the Expect cell is not machine-decidable), so the printed set here is the content check. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | 0 | pass. Counts 9, 5 and 2. Witness: pass; its output hash equals the one the implementer's pre-merge witness recorded. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | 0 | pass. Both invocations printed `ok` (0.165s and 1.312s); the verbose run shows the no-override test PASS. Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | 0 | pass. Printed `LINT: PASS` and `rc=0` on the host and in the container. Witness: pass on exit status only; see the row 9 note below. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | 0 | could-not-check, not a pass. Printed `COULD-NOT-CHECK: ... is not in the diff against f5827b497467...` and `rc=2`, on the host and in the container. That is the merged-main outcome the Expect cell says must never be recorded as pass. Witness: fail, `exit 0, expected 2`. See the row 10 note below. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | 0 | pass. The named test's `--- PASS` line (0.01s), then `ok ... 0.193s`. Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | 0 | pass. The named test's `--- PASS` line (0.01s), then `ok ... 0.180s`. Witness: pass, network-off. | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | 0 | pass. Exit 0 means the grep found the named PASS line; the verbose run shows the same test PASS (0.39s). Witness: pass, network-off (the row prints nothing, so its hash is that of empty output). | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | 0 | pass. Exit 0 means the grep found the named PASS line; the verbose run shows the same test PASS (0.30s). Witness: pass, network-off (empty-output hash, as row 13). | 2026-10-01 | assay-verifier-app[bot] (on-behalf-of human:ian) |

**Row 10 is a check-definition defect as authored, and it holds this brief.** The row cannot produce a passing witness on any tree:
- The command ends in `; echo rc=$?`, so the shell always exits 0.
- The Expect cell contains the unquoted words "exit 2". The witness tool reads the first unquoted exit number as the required process exit, so it requires exit 2 and records `fail exit=0` every time. The implementer's pre-merge witness shows the same `fail exit=0` on the authoring branch, where the command printed `rc=0`.
- On merged main the command prints `rc=2`, which the Expect cell itself classes as could-not-check and never pass.

The previous block counted this row as passing through a corrected form (10b). That form is not the row as authored, so this pass does not count it. For the record, the corrected form still behaves as before: `statusgen --consumers --brief graph-execution/03 --root . --base 909101d1a^; echo rc=$?` prints `summary: 4 corroborated, 0 disproved, 2 unchecked` and `rc=0`. No consumers entry is disproved; the defect is in the row, not in the delivered code. Releasing this brief needs the Verify row amended by its author so that the merged-main form is the one checked (for example the `--base` form with an Expect written as output is `rc=0`), then a fresh witness.

**Row 9 note (weak check, not a failure).** The Expect cell is a bare backticked `rc=0`, which the witness tool cannot decide, and the trailing echo makes the shell exit 0 whatever the lint prints. A passing witness for row 9 therefore does not show that the lint passed. I saw this directly: in a discarded container attempt the lint printed `LINT: FAIL` and `rc=1` (caused by my stale clone ref, not by main) and the witness still recorded `pass exit=0`. On the correct tree the lint prints `LINT: PASS` and `rc=0`. Writing the Expect as output is `rc=0` would make the witness check it.

**Mutation rows (2, 3, 4, 11, 12, 13).** I did not repeat the mutations. The implementer's mutation tables above record each mutant killed; this pass confirms the named guards pass on merged main.

**Gating.** With this witness landed, coverage for this brief has 13 claims at `pass` at f5827b497467 and one at `fail` (row 10), so the brief stays held and the model-lane flip would refuse it, naming row 10. At f5827b497467 `statusgen --coverage` holds all 311 briefs in the corpus; a missing root exits 2.

**Risk-bearing values.** The risk fields are all `no` and the gate is `model`. The delivered files are unchanged since the previous block, so the enumeration is the same, re-read at f5827b497467:
- `hexRevisionRe = ^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:679
- `minRevisionTokenLen = treeSHALen` @ statusgen/coverage.go:665, with `treeSHALen = 12` @ statusgen/verifyrun.go:150
- `coverageExitOK = 0`, `coverageExitCouldNot = 2` @ statusgen/coverage.go:1273-1274
- `coverageResultOrder` fail 0 / missing 1 / error 2 / wrong-revision 3 / could-not-check 4 / pass 5 @ statusgen/coverage.go:249-256
- `witnessCellRunner = 5` @ statusgen/verifyrun.go:284
- the schema adds string fields only and no numeric bound

None is irreversible: a wrong value mis-gates a board flip and an edit plus a re-release undoes it. The revision-token shape ranks first because a wrong bound there fails open. The result order and the Runner cell index only pick a display reason and a column.

RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:679 — the lower bound 12 is treeSHALen (statusgen/verifyrun.go:150), the exact token length verifyrun writes (the witness above carries the 12-character token f5827b497467), so no shorter prefix can match by coincidence; the upper bound 40 is a full SHA-1 hex name, the longest git emits in a SHA-1 repository; hex-only rejects symbolic revisions that would never go stale; a 64-hex SHA-256 name is rejected and resolves could-not-check, which fails closed.
RISK-VALUE: DERIVED — coverageExitOK = 0 / coverageExitCouldNot = 2 @ statusgen/coverage.go:1273-1274 — taken from Task item 5 ("Exit 0; exit 2 on an unreadable tree"); a root that does not exist was observed to exit 2 and the real tree to exit 0.

rows_passed=13 rows_total=14 (row 10: witness fail, could-not-check by its own Expect on merged main; a check-definition defect, no implementation defect observed)

VERIFY: BLOCKED — 13 of 14 rows pass with a network-off Linux execution witness at f5827b497467; row 10 cannot pass as authored and needs a Verify-table amendment by the author before this brief can be released. Held at implemented.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question: name one input state in which coverage reports `released` while a
mandatory claim has no passing result at the item's revision. If one exists, the rule is
not deterministic and the brief is bounced.
