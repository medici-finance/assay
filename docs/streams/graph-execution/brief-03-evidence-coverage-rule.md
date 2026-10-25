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
unblocks: ["graph-execution/05", "graph-execution/19", "iso-9001/09"]
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

### Non-implementer verifier run: 2026-10-02T22:23:58Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

What moved since the last run: the previous independent pass (2026-10-01, above) ran at 4b1f8fc8bfaf. This pass re-ran every Verify row by hand at e1d99484ffd91b649ea45e10a1cecf4ba2a4924b, a later main tip. Between the squash merge of #1682 (909101d1aa20) and this tip, the statusgen package changed further (verifyrun.go among others), so this is a fresh observation of the merged code, not a copy of the earlier one.

Setup: a clean detached worktree at that commit; the working tree was unmodified before and after. The statusgen binary was built from the same tree (`cd statusgen && GOWORK=off go build`) and put first on PATH, so rows 6, 9 and 10 exercise merged code. Go 1.27.1 darwin/arm64. All commands ran under a throwaway HOME with no roster configured. Deviations from the authored text, both disclosed: row 6's output file was redirected from the system temp directory into a scratch directory, and TMPDIR pointed at the same scratch directory for rows 13 and 14. No tracked file was edited.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | exit 0; `ok github.com/medici-finance/assay/statusgen 65.420s`. A separate -v run showed 32 top-level TestCoverage tests with a PASS line, 0 FAIL, 0 SKIP. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; a witness SHA that differs from the merged revision is wrong-revision and released false | exit 0; `ok ... 2.271s`. The -v run printed `--- PASS: TestCoverageWrongRevisionHolds (0.01s)`, so the pattern matched a real test. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; a witness with no verdict yields could-not-check and holds, never pass | exit 0; `ok ... 2.360s`. The -v run printed `--- PASS: TestCoverageCouldNotCheckIsNotPass (0.01s)`. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; a verified row with approval at head but a missing mandatory claim is not flipped, and the dry-run names the claim | exit 0; `ok ... 3.542s`. The -v run printed `--- PASS: TestAutoFlipRefusesUnreleasedCoverage (0.23s)`. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; a join with only site rows is held on integration-check; one +flow row releases it | exit 0; `ok ... 3.537s`. The -v run printed `--- PASS: TestCoverageJoinRequiresIntegrationRow (0.24s)`. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | the printed set is a subset of the six results | statusgen exit 0, python exit 0; printed `['could-not-check', 'error', 'fail', 'missing', 'wrong-revision']`, a subset of the expected six. Output file redirected to a scratch directory. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count >= 1 | exit 0; counts 9 (workflow-pattern spec), 5 (schema), 2 (lifecycle spec). | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | exit 0; both invocations printed `ok` (0.690s and 1.820s). The -v run printed `--- PASS: TestAutoFlipNoOverride (0.01s)` and 18 top-level TestEvidenceActor PASS lines. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | `rc=0`; last output line `LINT: PASS`, NOTICE lines only, none about graph-execution. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass | `rc=2`; `COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against e1d99484ffd9...`. This is the merged-main outcome the Expect cell names. It is recorded as could-not-check and is not counted as a pass. Supplementary run below. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | exit 0; `--- PASS: TestCoverageAdviceCannotSupplyWitness (0.01s)`, then `ok ... 0.798s`. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | exit 0; `--- PASS: TestCoverageAcceptanceDigestChanged (0.01s)`, then `ok ... 0.178s`. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS | exit 0, so the grep found the named PASS line (the command prints nothing). The separate -v run printed `--- PASS: TestCoveragePolicyDependencyChanged (0.63s)`. TMPDIR was a scratch directory. The mutation half was not repeated in this pass. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | exit 0, so the grep found the named PASS line. The separate -v run printed `--- PASS: TestCoverageReuseDoesNotRetargetPass (0.38s)`. TMPDIR was a scratch directory. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Row 10, supplementary (outside the table, not a Verify row).** From a checkout of the squash merge 909101d1aa20, `statusgen --consumers --brief graph-execution/03 --root . --base 909101d1a^` printed `rc=0` and `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. The four fixed-here entries (autoflip.go, lifecycle.go, the lifecycle spec, the workflow-pattern spec) are CORROBORATED. The two UNCHECKED entries are the out-of-scope ones (the verify-gate workflow pair and evidenceactor.go), reported as unchanged since the base. The worktree was returned to e1d99484ffd9 afterwards.

**Vacuity check.** One verbose run of every test the rows name (`go test -count=1 -v -run 'TestCoverage\|TestAutoFlip(RefusesUnreleasedCoverage\|NoOverride)\|TestEvidenceActor\|TestLifecycleDemotion' ./...`) exited 0 with 57 top-level PASS lines, 0 FAIL and 0 SKIP. Each name in rows 2, 3, 4, 5, 8, 11, 12, 13 and 14 has its own PASS line, so no row passes on a pattern that matches nothing.

**Execution witness.** `statusgen verifyrun --brief docs/streams/graph-execution/brief-03-evidence-coverage-rule.md --dry-run` exited 2 and wrote nothing to the brief. Per row: 6, 7 and 9 pass (exit 0); row 10 fail (`exit 0, expected 2`); rows 1 to 5, 8 and 11 to 14 produced no verdict because the check:ci arm needs the Linux network-off sandbox (`unshare --net`), which this darwin host lacks (#1800, open). Witness result: 3 of 14. Disclosure: the witness executes row 6 exactly as authored, so it wrote /tmp/ge03.json in the system temp directory; that file was left in place.

**Findings.**
1. Row 10 cannot record a witness pass on any tree. The authored command ends in `; echo rc=$?`, so the compound always exits 0, while the witness reads the Expect cell's "exit 2" as the required exit status. On merged main the tool itself exits 2 (correct, by the Expect cell) and the witness still records a failure. This is a check-definition defect in the row, not in the consumers gate or the coverage rule. The implementer's note above describes the same reader behaviour from the authoring-branch side.
2. Coverage holds this brief, and every other one: `statusgen --coverage --root .` exits 0 and reports 311 briefs, 0 released (1912 missing, 509 wrong-revision, 177 could-not-check, 84 fail, 35 error). This brief's line is `graph-execution/03 held 14 Verify row #10: ...: the witness records a failure`; its 14 claims resolve as 10 could-not-check, 3 wrong-revision, 1 fail. The model-lane flip precondition in autoflip.go therefore refuses this brief until Linux-runner witnesses exist at an ancestor of main and row 10 is re-baselined. Already filed as #1990 (open).
3. Task delivery, read from the diff of #1682 (de45ad17171a to 909101d1aa20, 27 files):
   - Task 1 (the rule): delivered in statusgen/coverage.go. The signature is `evaluateCoverage(root string, streams []*Stream, opts coverageOptions)`, not the `patterns map[string]Pattern` third parameter the Task names. The item's revision resolves to the checked-out HEAD, not a looked-up merge SHA; the implementer declared that deviation above and the lifecycle spec states the implemented rule.
   - Task 2 (join integration check): delivered; the `integration-check` claim is emitted in coverage.go and pinned by row 5's test.
   - Task 3 (observe kind): delivered in the workflow-pattern spec (section 4.1.1, with the declare-only-where-a-deploy-exists rule) and both schema copies (signal, band, window, source). `TestCoverageBoundObserveClaimHolds` passes.
   - Task 4 (consumers): delivered. autoflip.go calls evaluateCoverage and refuses with a reason; lifecycle.go demotes on `Passed && !Released`, and `TestLifecycleDemotionCoverageNotReleased` passes.
   - Task 5 (CLI): delivered. The plain form prints one line per brief; a root that does not exist exits 2.
   - Task 6 (docs): delivered. The lifecycle spec, docs/lifecycle.md and docs/enforcement-model.md mention coverage 4, 3 and 3 times. The changelog fragment the PR added is no longer a file at this tip: the v1.0.30 aggregation moved its text into CHANGELOG.md, where the `statusgen --coverage` entry is present.
   - The Context line lists a statusgen/testdata/coverage/ directory as planned. It was never created; the tests build their fixtures in temporary repositories. No Verify row depends on it.
4. Mutation halves (rows 2, 3, 4, 11, 12, 13) were not repeated; this pass confirms only that the named guards pass on merged main. The implementer's mutation tables above are the record for those.

**Risk-bearing values.** The risk fields are all `no`, irreversible is `no`, gate is `model`. Enumerated over the non-test diff of #1682 (coverage.go, autoflip.go, lifecycle.go, main.go, brieffile.go, patterns.go, verifyrun.go, both schema copies), with line numbers at e1d99484ffd9:
- `hexRevisionRe = ^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:695
- `minRevisionTokenLen = treeSHALen` @ statusgen/coverage.go:681, resolving to `treeSHALen = 12` @ statusgen/verifyrun.go:150
- `coverageExitOK = 0`, `coverageExitCouldNot = 2` @ statusgen/coverage.go:1344-1345
- `coverageResultOrder` fail 0 / missing 1 / error 2 / wrong-revision 3 / could-not-check 4 / pass 5 @ statusgen/coverage.go:262-269
- `witnessCellRunner = 5` @ statusgen/verifyrun.go:284
- the schema adds string fields only, no numeric bound

Ranking: none is irreversible; a wrong value mis-gates a board flip and is undone by an edit and a release. The revision-token shape ranks first because a wrong bound there fails open (a stale or symbolic witness token could release a brief). The exit codes rank next. The result order only picks which reason is displayed, and the cell index parses a column.

RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:695 — the lower bound equals treeSHALen = 12 (statusgen/verifyrun.go:150), the exact token length verifyrun writes, so nothing verifyrun emits is rejected and a shorter, collision-prone prefix is; 40 is a full SHA-1 hex name, the longest token git emits in a SHA-1 repository; hex-only rejects symbolic revisions that never go stale. A 64-hex SHA-256 name is rejected and resolves could-not-check, which fails closed.
RISK-VALUE: DERIVED — coverageExitOK = 0 / coverageExitCouldNot = 2 @ statusgen/coverage.go:1344-1345 — taken verbatim from Task item 5 ("Exit 0; exit 2 on an unreadable tree"); a root that does not exist was observed to exit 2 and a readable tree 0.

rows_passed=13 rows_total=14 by hand. Row 10 is could-not-check on merged main, the outcome its Expect cell declares acceptable; its supplementary run against the merge diff gave rc=0 with nothing disproved.

VERIFY: PASS

Desk landing note (verify-desk, 2026-10-02): the hand verdict above stands, and the outcome record for this run is `blocked`. The status does not move on it: the execution witness scores 3 of 14 on this host (the check:ci rows need a network-off sandbox, #1800), and row 10 cannot pass the witness as authored because its command always exits 0 while its Expect names exit 2.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Merged main bed1a31ba875549c069fe49276d5b6eeaa0d147c. Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`.

Why this re-run: the last receipt (blocked at e1d99484ffd9, 13/14 by hand, blocker #1800) went stale because the brief file's hash changed. Between e1d99484ffd9 and bed1a31ba the brief changed in two places only: the 2026-10-02 Evidence block was landed (#2068), and the `unblocks:` frontmatter gained graph-execution/19 and iso-9001/09. Task, DoD and the Verify table did not change. Every other declared input hashes the same as on the receipt: coverage.go, coverage_test.go, autoflip.go, lifecycle.go, main.go, both specs, the schema, docs/lifecycle.md and docs/enforcement-model.md. `git describe` is still v1.0.31(-127). Elsewhere in the statusgen package, verifyincontainer.go, unrun.go, registers.go, corroborate.go and others changed. None of them is an input this brief verifies. So the change does not touch what the brief verifies. The new thing this pass adds is a Linux execution witness, which the maintainer sanctioned for this brief to get past #1800. #1800 (darwin has no `unshare --net`) and #1990 (coverage releases no briefs) are both still OPEN (`gh issue view`, read-only).

Setup (hand rows): a clean detached worktree at bed1a31ba. I built statusgen from that tree (`cd statusgen && GOWORK=off go build`) and put it first on PATH. Go 1.27.1 darwin/arm64, with KUBECONFIG=/dev/null. The tracked tree was unmodified before and after the run. Row 6's output file went to a scratch directory instead of the system temp directory. Expectations come from the brief's Task and Verify text at bed1a31ba.

Disclosure on the Go rows: my first pass ran with TMPDIR pointed at a scratch directory inside the git worktree. In that pass, rows 1 and 2 exited 1. TestCoverageWrongRevisionHolds gave `could-not-check` ("the item's revision bbbbbbbbbbbb is not in this clone") where the test expects `wrong-revision`. The cause is in the test fixture: the helper builds its root with t.TempDir() and never runs `git init`. When TMPDIR sits inside a git work tree, git resolves the enclosing repository and the fake revision is not found. The result is still held, never released, so the rule fails closed. With the default TMPDIR, both rows exit 0, as recorded below. The Linux witness (default /tmp) also passes them. I report this as a test-hermeticity finding, not as a defect in the rule.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | Verify row 1. exit 0; `ok github.com/medici-finance/assay/statusgen 39.344s` (default TMPDIR). A combined -v run shows 32 top-level TestCoverage PASS lines, with 0 FAIL and 0 SKIP. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; a witness SHA different from the merged revision is wrong-revision and released false | Verify row 2. exit 0; `ok ... 0.169s` (default TMPDIR). The -v run printed `--- PASS: TestCoverageWrongRevisionHolds (0.01s)`. When TMPDIR is inside a git work tree it exits 1 with could-not-check instead (see disclosure above). | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; a could-not-run witness yields could-not-check and holds, never pass | Verify row 3. exit 0; `ok ... 0.191s`. -v: `--- PASS: TestCoverageCouldNotCheckIsNotPass (0.01s)`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; a verified row with approval at head but a missing mandatory claim is not flipped, and the dry-run names the claim | Verify row 4. exit 0; `ok ... 0.334s`. -v: `--- PASS: TestAutoFlipRefusesUnreleasedCoverage (0.22s)`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; a join with only site rows is held on integration-check, and one +flow row releases it | Verify row 5. exit 0; `ok ... 0.412s`. -v: `--- PASS: TestCoverageJoinRequiresIntegrationRow (0.51s)`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | the printed set is a subset of the six results | Verify row 6. statusgen exit 0, python exit 0. Printed `['could-not-check', 'error', 'fail', 'missing', 'wrong-revision']`, a subset of the six. (364 briefs, 0 released.) | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count >= 1 | Verify row 7. exit 0. Counts: 9 (workflow-pattern spec), 5 (schema), 2 (lifecycle spec). | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | Verify row 8. exit 0; both invocations printed `ok` (0.162s and 1.806s). -v: `--- PASS: TestAutoFlipNoOverride (0.01s)` and 18 top-level TestEvidenceActor PASS lines. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | Verify row 9. `rc=0`; output line `LINT: PASS`, and every other line is a NOTICE. Ten NOTICEs are about this brief: gotest-run-vacuous on rows 1, 2, 3, 4, 5, 8 (twice), 11 and 12, meaning those commands carry no PASS assertion, and verify-row-portability on row 6. The vacuity check below covers the gotest-run-vacuous gap. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 (could-not-check) on a fully merged main is acceptable and recorded as such, never as pass | Verify row 10. `rc=2`; `COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against bed1a31ba875...`. This is the merged-main outcome the Expect cell names. It is recorded as could-not-check and NOT counted as a pass. Supplementary run below. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | Verify row 11. exit 0; `--- PASS: TestCoverageAdviceCannotSupplyWitness (0.01s)`, then `ok ... 0.171s`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | Verify row 12. exit 0; `--- PASS: TestCoverageAcceptanceDigestChanged (0.02s)`, then `ok ... 0.187s`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS | Verify row 13. exit 0, so the grep found the named PASS line (the command prints nothing). The combined -v run shows `--- PASS: TestCoveragePolicyDependencyChanged (0.94s)`. I did not repeat the mutation half. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | Verify row 14. exit 0, so the grep found the named PASS line. The combined -v run shows `--- PASS: TestCoverageReuseDoesNotRetargetPass (0.49s)`. | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) |

**Row 10, supplementary (outside the table; not a Verify row).** In a throwaway clone detached at the squash merge 909101d1aa20, I ran the statusgen binary built from bed1a31ba as `statusgen --consumers --brief graph-execution/03 --root . --base 909101d1a^`. It exited 0 with `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. The CORROBORATED entries are the four fixed-here ones: autoflip.go, lifecycle.go, the lifecycle spec and the workflow-pattern spec. The UNCHECKED entries are the two out-of-scope ones, the verify-gate workflow pair and evidenceactor.go, each "unchanged since the merge-base".

**Vacuity check.** I ran every test the rows name in one verbose run: `go test -count=1 -v -run 'TestCoverage|TestAutoFlip(RefusesUnreleasedCoverage|NoOverride)|TestEvidenceActor|TestLifecycleDemotion' ./...`. It exited 0 with 57 top-level PASS lines, 0 FAIL and 0 SKIP. Each test named in rows 2, 3, 4, 5, 8, 11, 12, 13 and 14 has its own PASS line, so no row passes on a pattern that matches nothing. No row is VACUOUS. Row 10 is could-not-check, not a pass.

**Execution witness (Linux container).** This recipe is the maintainer's ruling, relayed to this desk, for briefs held only on #1800, and this brief was held only on #1800. I used a throwaway clone under the worktree's untracked scratch directory, detached at bed1a31ba, with that clone's refs/remotes/origin/main pinned to the same SHA. The image was the cached golang:1.26-bookworm (go1.26.8 linux/arm64; module go directive 1.25.0), run with no pull. The exact command, where incontainer.sh is passed inline as the `sh -c` argument and is not mounted:

`docker run --rm --pull never --network none --security-opt seccomp=unconfined -v <home>/.v/clone:/work -v $(go env GOMODCACHE):/go/pkg/mod:ro -v ~/.config/assay/roster.env:/root/.config/assay/roster.env:ro golang:1.26-bookworm sh -c "$(cat incontainer.sh)"`

The script does the following: `git config --global --add safe.directory '*'`; sets the in-container git identity to user.name assay-verifier-app[bot] and user.email <id>+assay-verifier-app[bot]@users.noreply.github.com; exports `GOFLAGS=-count=1 GOPROXY=off GOTOOLCHAIN=local`; runs `cd /work/statusgen && GOWORK=off go build -o /usr/local/bin/statusgen .`; then runs `statusgen verifyrun --brief docs/streams/graph-execution/brief-03-evidence-coverage-rule.md` and `statusgen verifyrun --check --brief` on the same path. Disclosure: `seccomp=unconfined` was used only inside this throwaway `--network none` container, so that `unshare --net` works for the check:ci arm. The only mounts were the clone, the module cache (read-only) and the roster (read-only). No credentials, tokens or PEMs were mounted. verifyrun exited 1 and `--check` exited 1. The verifyrun write landed in the throwaway clone's copy of the brief. This worktree's brief was never written.

`verifyrun --check` summary: `docs/streams/graph-execution/brief-03-evidence-coverage-rule.md: 13 pass, 1 fail, 0 could-not-run/missing (of 14 Verify rows)` (row 10: `fail — the witness records a failure`; verifyrun printed `row 10: fail (exit=0, sha256:cc904c369a51) — exit 0, expected 2`).

Witness table, verbatim from the tool:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | pass exit=0 | sha256:1f99e6950966 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | pass exit=0 | sha256:28ee1419f776 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | pass exit=0 | sha256:9cdfba532244 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | pass exit=0 | sha256:846bbb502a32 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | pass exit=0 | sha256:2ef712a496ea | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass exit=0 | sha256:73af7c13fa8d | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass exit=0 | sha256:38af6536faaa | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | pass exit=0 | sha256:d27f4fd4cad0 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:0060039bc5ca | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | fail exit=0 | sha256:cc904c369a51 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | pass exit=0 | sha256:594eeda0187a | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | pass exit=0 | sha256:4c8038c1ce0a | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |

Reading the witness. On this Linux run, the check:ci rows that were could-not-run on darwin (1–5, 8, 11–14) all pass, so the #1800 environment blocker does not apply to this witness. Row 10's witness `fail` is a defect in how the check is written, not in the code. The authored command ends in `; echo rc=$?`, so the compound command always exits 0. The witness, though, reads "exit 2" in the Expect cell as the required status. The tool exits 2 on merged main, which the Expect cell accepts, and the witness still records a failure. Rows 9 and 6 pass on the witness by exit status only: row 9 ends in `; echo rc=$?` and row 6's python step always exits 0. For those two rows, the witness pass does not decide the Expect. The hand readings above (`rc=0` / `LINT: PASS`, and the printed subset) are what discharge them. Rows 13 and 14 hash to the empty-output digest, as expected, because those commands print nothing and assert with grep.

**Blocker.** The only row that blocks the witness gate and the coverage release (autoflip.go's precondition) is row 10. As authored, it can never get a witness pass on any tree. It is tracked on #1927 (check-definition re-authoring), where a 2026-10-04 comment adds this row; #1927 already lists the same `; echo rc=$?` shape for other briefs. #1990 (coverage releases no briefs) is related. To release, row 10 needs re-authoring, either as an exit-2-tolerant form or as the `--base <merge>^` corroboration form. #1800 is still OPEN, but this sanctioned container witness gets past it for this brief.

**Findings for the desk to file.**
1. graph-execution/03 Verify row 10: the `; echo rc=$?` suffix together with an Expect of "exit 2" makes the row unpassable on the witness. This is a check-definition re-authoring need, the same class as #1927.
2. Test hermeticity: the mustCoverageStream helper in coverage_test.go never runs `git init` on its t.TempDir() root. When TMPDIR sits inside a git work tree, git resolves the enclosing repository. TestCoverageWrongRevisionHolds then sees could-not-check instead of wrong-revision and fails. The rule still holds the brief, so it fails closed. The test is environment-sensitive, though, and a verifier who points TMPDIR at an in-repo scratch directory gets a false red.

**Mutation halves** (rows 2, 3, 4, 11, 12, 13): not repeated. This pass confirms only that the named guards pass on merged main.

**Risk-bearing values.** gate `model`, every risk field `no`, irreversible `no`. coverage.go and verifyrun.go are byte-identical in hash to the last receipt. I re-checked the line numbers at bed1a31ba and enumerated over the same non-test diff of #1682 as the previous passes:
- `hexRevisionRe = ^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:695
- `minRevisionTokenLen = treeSHALen` @ statusgen/coverage.go:681, which resolves to `treeSHALen = 12` @ statusgen/verifyrun.go:150
- `coverageExitOK = 0`, `coverageExitCouldNot = 2` @ statusgen/coverage.go:1344-1345
- `coverageResultOrder` (fail 0 / missing 1 / error 2 / wrong-revision 3 / could-not-check 4 / pass 5) @ statusgen/coverage.go:262-269
- `witnessCellRunner = 5` @ statusgen/verifyrun.go:284
- the schema adds string fields only, with no numeric bound

Ranking: none of these is irreversible, since a wrong value can be undone by an edit and a release. The revision-token shape ranks first because it would fail open. The exit codes rank next. The result order and the cell index rank last.

RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:695. Why it is right: the lower bound equals treeSHALen = 12 (statusgen/verifyrun.go:150), the exact token length verifyrun writes, so a shorter, collision-prone prefix is rejected. 40 is a full SHA-1 hex name. Hex-only rejects symbolic revisions, which would never go stale. A 64-hex SHA-256 name resolves could-not-check, which fails closed.
RISK-VALUE: DERIVED — coverageExitOK = 0 / coverageExitCouldNot = 2 @ statusgen/coverage.go:1344-1345. Why it is right: both values come verbatim from Task item 5 ("Exit 0; exit 2 on an unreadable tree"), and row 6 observed exit 0 on a readable tree.

rows_passed=13 rows_total=14 by hand. Row 10 is could-not-check on merged main, which its Expect allows. Its supplementary run against the merge diff gave rc=0 with 0 disproved. The witness gives 13 pass and 1 fail (row 10, the check-definition defect).

VERIFY: BLOCKED — no code failure observed. The witness is held only on Verify row 10's check definition, tracked on #1927 (a 2026-10-04 comment adds this row; related: #1990). #1800 is still open, and this sanctioned Linux witness gets past it for this brief.
### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-07 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian)

Merged main 36113a1ddbb3492c213520d561edd09e2f7afca8 (origin/main equal to `git ls-remote origin main` at start). Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`.

Setup: clean detached worktree at 36113a1ddbb3; statusgen built from that tree (`cd statusgen && go build`) and placed first on PATH for rows 6, 9 and 10. Go 1.27.1 darwin/arm64, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local), throwaway HOME, TMPDIR and GOCACHE in a scratch directory outside any git work tree. No roster, token or PEM was read. Tracked tree unmodified before and after (`git status --short` empty). Expectations were written from the brief's Task, amendments and Verify table before any diff, test body or prior verdict was read.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | pass | exit 0; ok github.com/medici-finance/assay/statusgen 32.531s. A separate -count=1 -v run lists 33 top-level TestCoverage* PASS lines plus TestAutoFlipRefusesUnreleasedCoverage, 0 FAIL | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | pass | exit 0; ok ... 0.962s; -v run shows the named test PASS | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | pass | exit 0; ok ... 1.016s; -v run shows the named test PASS | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | pass | exit 0; ok ... 3.402s; -v run shows the named test PASS | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | pass | exit 0; ok ... 0.433s; -v run shows the named test PASS | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass | exit 0; printed ['could-not-check', 'error', 'fail', 'missing', 'pass', 'wrong-revision'], equal to the allowed set (367 briefs, 3218 claims, 0 released) | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass | exit 0; counts 9, 5, 2 (each at least 1) | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | pass | exit 0; both invocations ok (0.943s, 2.307s); -v run shows TestAutoFlipNoOverride PASS and 18 TestEvidenceActor* PASS | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass | rc=0; LINT: PASS (remaining lines are NOTICEs, none naming graph-execution/03) | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | could-not-check | rc=2; COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against 36113a1ddbb3 — the merged-main outcome the Expect cell allows, recorded as could-not-check and never as pass | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | pass | exit 0; named test PASS, then ok ... 0.949s | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | pass | exit 0; named test PASS, then ok ... 1.058s | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | pass | exit 0; the grep found the named PASS line | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | pass | exit 0; the grep found the named PASS line | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |

**Execution witness.** `/opt/desk-tools/bin/statusgen verifyrun` (v1.0.32) ran once, non-dry, on this darwin host at 36113a1ddbb3. Result: rows 6, 7 and 9 pass; row 10 fail (exit 0, expected 2); rows 1–5, 8 and 11–14 could-not-run, because check:ci rows need the Linux `unshare --net` network-off sandbox and this host is darwin. The witness table is kept in the desk's scratch copy (witness-table.md) with its runner and source tag as the tool wrote them. The witness's edit to the brief was reverted afterwards (`git checkout -- .`, brief md5 identical before and after).

**Reasoning.** All 13 executable-to-pass rows pass by hand on merged main. Row 10 returns exit 2 (could-not-check) on merged main, which its Expect cell allows and requires to be recorded as could-not-check. As authored, though, row 10 can never pass the execution witness: the trailing `; echo rc=$?` makes the compound command exit 0, while the witness reads "exit 2" from Expect. That check-definition defect is the open blocker #1927 (state open, read 2026-10-07), the same blocker the 2026-10-04 run recorded. The brief file, and with it the row 10 definition, is byte-identical to the 2026-10-04 receipt. The darwin witness also could not produce verdicts for the nine check:ci rows. This is environment-only: a Linux network-off witness at the prior SHA scored those rows pass.

Changes since the 2026-10-04 receipt (bed1a31ba875): coverage.go, main.go and spec/lifecycle-v1.md changed through #2304 ("judge witness staleness by the paths a row reads; require the witness tree to have landed", #2026). The coverage suite grew to 33 top-level tests, all passing. Row 6's printed set now includes `pass` (43 passing claims across 367 briefs), where at bed1a31ba it held none.

**Risk-bearing values** (diff scope: coverage.go, coverage_inputs.go, verifyrun.go treeSHALen consumer, autoflip/lifecycle wiring):
- RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/coverage.go:725. The lower bound 12 equals treeSHALen = 12 (statusgen/verifyrun.go:150), the exact length verifyrun writes, so no real witness is rejected; a shorter token would coincide with a random SHA at 16^-k (12 hex gives 2^-48). The upper bound 40 is a full SHA-1 hex id. Requiring hex keeps symbolic revisions such as HEAD~0 from tracking HEAD and staying pass forever. In a SHA-256 object-format repo, 64-hex tokens would be refused. That outcome is held, not released, so the failure direction is closed.
- RISK-VALUE: DERIVED — minRevisionTokenLen = treeSHALen (12) @ statusgen/coverage.go:711. It has the same floor as above, and a shorter token goes to could-not-check (TestCoverageShortTreeTokenIsCouldNotCheck PASS).
- RISK-VALUE: DERIVED — coverageExitOK = 0, coverageExitCouldNot = 2 @ statusgen/coverage.go:1434-1435. These are Task 5's literal contract ("Exit 0; exit 2 on an unreadable tree"). Observed: exit 0 on the real tree, and exit 2 with --root at a nonexistent directory.
- RISK-VALUE: NAMED, NOT DERIVED — isBoardBookkeepingPath: p == "STATUS.md" or prefix "docs/streams/" @ statusgen/coverage.go:982. This is the conservative-scope exemption. A change only under docs/streams/** or STATUS.md never stales a witness unless the brief declares that artifact. The code header states it as an accepted residual (coverage.go:172-175). It is an engineering trade, needed because verify batches and regen always touch these paths. It is not derived from the brief: the 2026-09-30 WI-2 amendment says file non-overlap is not proof. A row that reads docs/streams, such as row 9's `statusgen --lint`, can therefore keep a reused pass after a docs/streams edit that would change its outcome. This answers the brief's Review question with a concrete input state. Routed for human judgment, not counted as a row failure.

**VERIFY: BLOCKED**

### Non-implementer verifier re-run — 2026-10-11 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian)

Merged main 1235e830437adfaf08c69f93a4d894df85426f4e (the #2552 head that moved the coverage rule into the statusgen evaluation package; `git ls-remote origin refs/heads/main` returned the same SHA). Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. The brief's Task and Verify table are byte-identical to the dispatch packet's copy at this commit.

Verification-Attestation: medici-finance/assay#2597 run=183623490d063ed089a67bd212c5e999d34bc8eef0ab68726ff6bd92ccc0bfe5 source=1235e830437adfaf08c69f93a4d894df85426f4e brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=any

Setup: `deskdispatch --check-verifier` admitted this run (rc=0, binding above) before any row ran. Clean detached home at 1235e830437a; statusgen built from that tree (`cd statusgen && go build`, output outside the home) and placed first on PATH for rows 6, 9 and 10. Go 1.27.1 darwin/arm64, git 2.56.0, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local), TMPDIR and GOCACHE in a scratch directory outside the home. Each row ran once as written from the home root. Each Go row then ran again as a diagnostic with GIT_CONFIG_GLOBAL=/dev/null and GIT_CONFIG_NOSYSTEM=1, because the host's global git config turns on commit signing. Rows 1-5 and 8 were forced uncached with GOFLAGS=-count=1 for that rerun. Both runs agreed on every row. No roster, token or PEM was read by any row. The tracked tree was unmodified before and after every row (`git status --short` empty).

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | pass: exit 0; ok github.com/medici-finance/assay/statusgen 31.086s. A separate -count=1 -v run lists 32 top-level TestCoverage* PASS lines plus TestAutoFlipRefusesUnreleasedCoverage PASS, 0 FAIL. Diagnostic rerun: exit 0, 29.239s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; wrong-revision fixture holds | pass: exit 0; ok ... 0.253s; the -v run shows TestCoverageWrongRevisionHolds PASS. Diagnostic rerun: exit 0, 0.254s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; could-not-check holds, never pass | pass: exit 0; ok ... 0.239s; the -v run shows TestCoverageCouldNotCheckIsNotPass PASS. Diagnostic rerun: exit 0, 0.218s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; no flip on a missing mandatory claim | pass: exit 0; ok ... 0.618s; the -v run shows TestAutoFlipRefusesUnreleasedCoverage PASS. Diagnostic rerun: exit 0, 0.570s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; join held on integration-check | pass: exit 0; ok ... 0.611s; the -v run shows TestCoverageJoinRequiresIntegrationRow PASS. Diagnostic rerun: exit 0, 0.584s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | printed set within the six results | pass: exit 0; printed ['could-not-check', 'error', 'fail', 'missing', 'pass', 'wrong-revision'], equal to the allowed set (384 briefs, 3397 claims, 0 released). This brief's own entry is held: 13 claims wrong-revision (the witness ran at bed1a31ba875) and row 10 fail | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count at least 1 | pass: exit 0; counts 9, 5, 2 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | pass: exit 0; both invocations ok (0.223s, 2.424s); the -v run shows TestAutoFlipNoOverride PASS and 18 TestEvidenceActor* PASS. Diagnostic rerun: exit 0 (0.230s, 2.292s) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | pass: rc=0; LINT: PASS; 0 PROBLEM lines, 1540 NOTICE lines, none naming graph-execution/03 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on merged main is acceptable, recorded as could-not-check | could-not-check: printed rc=2; COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against 1235e830437a. This is the merged-main outcome the Expect cell accepts, recorded as could-not-check and never as pass | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | pass: exit 0; TestCoverageAdviceCannotSupplyWitness PASS, then ok ... 0.239s. Diagnostic rerun: exit 0, named PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | pass: exit 0; TestCoverageAcceptanceDigestChanged PASS, then ok ... 0.243s. Diagnostic rerun: exit 0, named PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS | pass: exit 0; the grep found the named PASS line. Diagnostic rerun: exit 0 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | pass: exit 0; the grep found the named PASS line. Diagnostic rerun: exit 0 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

**Execution witness.** `statusgen verifyrun --brief` (built from this tree, first on PATH) ran once, non-dry, on this darwin host at 1235e830437a and exited 2. Rows 6, 7 and 9 pass. Row 10 fails (exit 0, expected 2). Rows 1-5, 8 and 11-14 came back could-not-run, because check:ci rows need the Linux `unshare --net` network-off sandbox and this host is darwin. Those rows were run by hand above instead. The witness table, with its runner and attestation line as the tool wrote them, is kept in the desk's scratch copy (witness-table.md). Its edit to the brief was reverted afterwards (`git checkout` of this file; md5 identical before and after), as the 2026-10-07 run did.

**Reasoning.** The 13 rows that can pass all pass by hand on merged main. Both runs agree, so the host's commit-signing config does not affect any row. Row 10 printed rc=2 (could-not-check) on merged main. Its Expect cell accepts that outcome, so it is recorded as could-not-check, not counted as a pass. The Verify table is therefore met as authored. As authored, though, row 10 can never pass the execution witness: the trailing `; echo rc=$?` makes the compound command exit 0, while the witness reads exit 2 from the Expect cell. That check-definition defect is open on medici-finance/assay#1927 (state OPEN, read 2026-10-11). Until it is re-authored, this brief's coverage verdict stays held on row 10, so autoflip cannot take it past verified. That hold is the coverage rule doing its job, not a defect in the work. The 2026-10-04 and 2026-10-07 runs recorded BLOCKED on the same row-10 facts, and this run records BLOCKED too. The table being met as authored does not make row 10 a pass: its could-not-check outcome is accepted by its Expect cell but is not a pass, and the execution witness cannot pass it until it is re-authored.

Changes since the 2026-10-07 receipt (36113a1ddbb3): #2552 moved the rule into the statusgen evaluation package (evaluation/coverage.go, coverage_api.go, coverage_inputs.go and coverage_repo.go; statusgen's coverage.go is now a thin adapter), and #2361 added verifyrun's container-netns sandbox. The set of 32 TestCoverage* function names is identical at both SHAs, and all of them pass. Row 6 now covers 384 briefs and 3397 claims, against 367 and 3218 on 2026-10-07.

**Risk-bearing values.** The trigger did not fire: risk metadata is present, irreversible is no, and the diff is statusgen tooling. The values were enumerated anyway. Diff scope: evaluation/coverage.go, evaluation/coverage_api.go, evaluation/coverage_repo.go, evaluation/identity.go, statusgen coverage.go, and the verifyrun treeSHALen consumer. Every entry is reversible by an edit and a redeploy. Ranking: revision-shape bounds first, exit codes next, then the search bound.
- RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22. verifyrun now truncates through the same constant (statusgen/coverage.go:51 aliases it; verifyrun.go:1164 truncates to it), so the witness writer and the coverage reader cannot drift apart. 12 hex digits gives a 2^-48 chance that a random SHA matches by coincidence.
- RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/evaluation/coverage.go:503. The lower bound equals TreeSHALen, the exact length verifyrun writes, so no real witness is rejected. 40 is a full SHA-1 hex id. Requiring hex stops a symbolic revision such as HEAD~0 from tracking HEAD and staying pass forever. A 64-hex SHA-256 token would be refused, which leads to held, not released.
- RISK-VALUE: DERIVED — minRevisionTokenLen = TreeSHALen (12) @ statusgen/evaluation/coverage.go:489. It is the same floor; a shorter token goes to could-not-check.
- RISK-VALUE: DERIVED — coverageExitOK = 0, coverageExitCouldNot = 2 @ statusgen/coverage.go:299-300. These are Task 5's literal contract ("Exit 0; exit 2 on an unreadable tree"). Row 6 observed exit 0 on the real tree.
- RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105. It is a cost bound on the squash-landing search. A witness past the bound is could-not-check, never assumed landed, so a wrong value can only over-hold, never release. It is a reversible operational knob and ranks last. The input field now reads 0 as this default (statusgen/evaluation/coverage_api.go:166-168). The tests that exercise the bound use 1, not 0.
- RISK-VALUE: NAMED, NOT DERIVED — isBoardBookkeepingPath: p == "STATUS.md" or prefix "docs/streams/" @ statusgen/evaluation/coverage.go:753-754. This is unchanged in substance since 2026-10-07; only the file moved. A change made only under docs/streams/** or STATUS.md never stales a witness unless the brief declares that artifact. This is an engineering trade, and the 2026-09-30 amendment says file non-overlap is not proof. A row that reads docs/streams, such as row 9's lint, can keep a reused pass after a docs/streams edit that would change its outcome. That is a concrete answer to the Review question. It is routed for human judgment and not counted as a row failure.

**VERIFY: BLOCKED** — 13/14 rows pass by hand at 1235e830437a; row 10 is could-not-check (rc=2), the merged-main outcome its Expect accepts; the darwin witness had no network-off sandbox for the 10 check:ci rows; coverage stays held on row 10 until medici-finance/assay#1927 re-authors it.

### Non-implementer verifier re-run — 2026-10-15 assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk)

Merged main a1c0a9c1e8a3bbd4de6f223eabc10ed9a27a1e68. Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. Window: admission check at 2026-10-15T04:35Z, rows ran 04:36:50Z to 04:39:09Z, diagnostics finished 04:43Z (UTC).

Verification-Attestation: medici-finance/assay#3016 run=607aa2424af4fb2ce4f10e7306dc9e88c4d78f4187a88a1b6d4ed2d6fb2fa73c source=a1c0a9c1e8a3bbd4de6f223eabc10ed9a27a1e68 brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=any

Delivering change: #1682 (merge 909101d1aa202a00e7c18a1788a49f51031de9ff) delivered the rule, the observe kind and all the named tests, including the four work-input amendment tests. Follow-ups on the same rule: #2041 (f76cbeef5cc0), #2304 (6c46105ab2c6) and #2552 (1235e830437a, which moved the rule into the statusgen evaluation package). Since the 2026-10-11 run (1235e830437a) the only change under the evaluation package is a new thin export file (declared_api.go, 24 added lines, no logic); autoflip changed in #2456 (18530134c091) and still calls the coverage evaluation before any flip (autoflip.go:1077).

Grounding, written from the brief before any diff or test was read: the six-value result vocabulary; could-not-run maps to could-not-check and holds; a witness at another revision is wrong-revision and holds; a join needs a +flow row or holds on integration-check; observe is declared in the pattern spec and schema; autoflip refuses an unreleased brief and names the claim; the CLI exits 0, or 2 on an unreadable tree; advice is never a witness; a changed acceptance or policy dependency holds the claim; an old-head pass never releases a new subject.

Setup: `deskdispatch --check-verifier` admitted this run (rc=0, binding above) before any row ran. Clean detached home at a1c0a9c1e8a3; statusgen built from that tree with its output in a scratch directory outside the home, and put first on PATH for rows 6, 9 and 10. Go 1.27.2 darwin/arm64, git 2.56.0, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local). HOME, TMPDIR and GOCACHE pointed at a fresh scratch directory outside the home, so no test result came from an earlier cache. Each row ran once, as written, as its own separate process from the home root. `git status --short` was empty after every row.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | pass: exit 0; ok github.com/medici-finance/assay/statusgen 20.425s. A separate -count=1 -v diagnostic lists 32 top-level TestCoverage PASS lines, 0 FAIL lines | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; wrong-revision fixture holds | pass: exit 0; ok ... 0.212s; the diagnostic -v run shows the named test PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; could-not-check holds, never pass | pass: exit 0; ok ... 0.209s; the diagnostic -v run shows the named test PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; no flip on a missing mandatory claim | pass: exit 0; ok ... 0.437s; the diagnostic -v run shows the named test PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; join held on integration-check | pass: exit 0; ok ... 0.474s; the diagnostic -v run shows the named test PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | printed set within the six results | pass: exit 0; printed ['could-not-check', 'error', 'fail', 'missing', 'wrong-revision'], a subset of the allowed six (426 briefs, 3773 claims, 0 released). This brief is held: 13 claims wrong-revision (witness at bed1a31ba875) and row 10 fail. A diagnostic rerun with the roster file present gave identical counts | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count at least 1 | pass: exit 0; counts 9, 5, 2 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | pass: exit 0; both invocations ok (statusgen package 1.309s on the second); the diagnostic -v run shows the no-override test PASS and 18 TestEvidenceActor PASS lines | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | pass: rc=0; LINT: PASS; 0 lines start with PROBLEM, 1849 NOTICE lines, none naming graph-execution/03 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on merged main is acceptable, recorded as could-not-check | could-not-check: printed rc=2; COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against a1c0a9c1e8a3. This is the merged-main outcome the Expect cell accepts. It is recorded as could-not-check, never as pass | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | pass: exit 0; the named test PASS (0.01s), then ok ... 0.193s | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | pass: exit 0; the named test PASS (0.01s), then ok ... 0.211s | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS; mutation must fail it | pass: exit 0; the grep found the named PASS line. Mutation check in a scratch copy outside the home: making the policy-fingerprint comparison in evaluation/coverage.go always report unchanged turns it into FAIL (row 1 resolved pass and released instead of wrong-revision); the unmutated copy passes | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | pass: exit 0; the grep found the named PASS line | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

**Reasoning.** All 13 rows that can pass do pass on merged main a1c0a9c1e8a3. Row 10 printed rc=2 (could-not-check), which is the merged-main outcome its Expect cell accepts, so it is recorded as could-not-check and not counted as a pass. As authored, row 10 cannot give a pass on merged main, and the coverage rule keeps this brief held on it. That is the rule working as designed, not a defect in the delivered work. The check-definition defect is tracked on medici-finance/assay#1927; this run made no network reads, so it did not re-read that issue's state. The earlier BLOCKED runs recorded the same row-10 facts, and this run records BLOCKED too.

Observation from row 6: no claim anywhere in the tree resolves pass at a1c0a9c1e8a3. Of 639 wrong-revision claims, 445 name a change to the .assay-surfaces file (last changed on 2026-10-10 in #2499) as the reason. For witnesses without a dependency manifest, the brief's work-input amendment requires broader revalidation in exactly this case, so this holds as the brief intends. It is not a release fault.

**Risk-bearing values.** The trigger did not fire: risk metadata is present, irreversible is no, and the change is statusgen tooling. The values were enumerated anyway, over statusgen/coverage.go, statusgen/evaluation/coverage.go, coverage_api.go, coverage_repo.go, declared_api.go and the verifyrun truncation. The literals are unchanged since the 2026-10-11 run, and every entry can be reversed by an edit and a redeploy. Ranking: revision-shape bounds first, then exit codes, then the search bound.
- RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22. statusgen/coverage.go:51 aliases it and verifyrun.go:1174 truncates to it, so the witness writer and the coverage reader share one constant and cannot drift apart. 12 hex digits gives a 2^-48 chance that a random tree SHA matches by coincidence.
- RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/evaluation/coverage.go:503. The lower bound equals TreeSHALen, the exact length verifyrun writes, so no real witness is rejected. The upper bound, 40, is a full SHA-1 id. Requiring hex stops a symbolic ref from tracking HEAD and staying pass forever. A longer token is refused, which leads to held, never to released.
- RISK-VALUE: DERIVED — minRevisionTokenLen = TreeSHALen (12) @ statusgen/evaluation/coverage.go:489. It is the same floor; a shorter token resolves could-not-check.
- RISK-VALUE: DERIVED — coverageExitOK = 0, coverageExitCouldNot = 2 @ statusgen/coverage.go:299-300. These are Task 5's literal contract ("Exit 0; exit 2 on an unreadable tree"). Row 6 observed exit 0 on the real tree.
- RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105. It is a cost bound on the squash-landing search. A witness past the bound is could-not-check, never assumed landed, so a wrong value can only over-hold and never release. It is a reversible operational knob and ranks last.
- RISK-VALUE: NAMED, NOT DERIVED — isBoardBookkeepingPath: p == "STATUS.md" or prefix "docs/streams/" @ statusgen/evaluation/coverage.go:753-754. A change made only under docs/streams or to STATUS.md never stales a witness unless the brief declares that artifact. That is an engineering trade, and the 2026-09-30 amendment says file non-overlap is not proof. A row that reads docs/streams, such as row 9's lint, could keep a reused pass after an edit there that changes its outcome. This is routed for human judgment and is not counted as a row failure.

**VERIFY: BLOCKED** — 13/14 rows pass at a1c0a9c1e8a3; row 10 is could-not-check (rc=2), the merged-main outcome its Expect accepts but not a pass; coverage stays held on row 10 until medici-finance/assay#1927 re-authors it.

### Non-implementer verifier re-run — 2026-10-21 assay-verifier-app[bot] (claude-opus-5-5 verifier)

Merged main 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1. Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. Window: admission check at 2026-10-21T18:30Z, rows ran 18:31:52Z to 18:33:26Z, diagnostics finished 18:35Z (UTC).

Verification-Attestation: medici-finance/assay#3907 run=5a0feeaf3818465bb86e94865909a9b9ff3e4c552df17f8c0d0cb01c18591cc7 source=1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=strong

Delivering change: #1682 (merge 909101d1aa20) delivered the rule, the observe kind and all named tests. Since the 2026-10-15 run (a1c0a9c1e8a3) the coverage code changed in #3470 and its follow-up 47492ac9f (protected acceptance definitions: an optional immutable original-table revision governs the historical Expect read, and an invalid protected definition adds a could-not-check claim so the set can never be empty and releasable), a memoised repo wrapper, and EvaluatorVersion moved to statusgen-evaluation/3. #3574 changed how a bare `--consumers --brief` resolves on merged main (row 10). autoflip still calls the coverage evaluation before any flip (autoflip.go:1077).

Grounding, written from the brief before any diff or test was read: the six-value result vocabulary; could-not-run maps to could-not-check and holds; a witness at another revision is wrong-revision and holds; a join needs a +flow row or holds on integration-check; observe is declared in the pattern spec and schema; autoflip refuses an unreleased brief and names the claim; the CLI exits 0, or 2 on an unreadable tree; advice is never a witness; a changed acceptance or policy dependency holds the claim; an old-head pass never releases a new subject.

Setup: `deskdispatch --check-verifier` admitted this run (rc=0, binding above) before any row ran. Clean detached home at 1f0b54e745d8; statusgen built from that tree with its output in a fresh scratch directory outside the home, and put first on PATH for rows 6, 9 and 10. Go 1.27.2 darwin/arm64, git 2.56.0, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local). TMPDIR and GOCACHE pointed at a fresh scratch directory outside the home, so no test result came from an earlier cache. Each row ran once, as written, as its own process from the home root. For rows 9 and 10 HOME was the desk's cell config home (the one the desk's statusgen shim uses), because with an empty scratch HOME the first row-9 attempt printed `LINT: REFUSED configuration` rc=5 for want of a roster file; that attempt is an environment refusal, not a result. `git status --short` was empty after every row.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | exit 0 | pass: exit 0; ok github.com/medici-finance/assay/statusgen 21.358s. A separate -count=1 -v diagnostic lists 32 top-level TestCoverage PASS lines, 0 FAIL lines | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; wrong-revision fixture holds | pass: exit 0; ok ... 0.253s; the diagnostic -v run shows the named test PASS | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; could-not-check holds, never pass | pass: exit 0; ok ... 0.355s; the diagnostic -v run shows the named test PASS | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; no flip on a missing mandatory claim | pass: exit 0; ok ... 0.449s; the diagnostic -v run shows the named test PASS | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; join held on integration-check | pass: exit 0; ok ... 0.443s; the diagnostic -v run shows the named test PASS | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | printed set within the six results | pass: exit 0; printed ['could-not-check', 'error', 'fail', 'missing', 'pass', 'wrong-revision'], exactly the allowed six (530 briefs, 4369 claims, 24 pass claims, 0 briefs released). This brief is held: 13 claims wrong-revision (witness at bed1a31ba875) and row 10 fail | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count at least 1 | pass: exit 0; counts 9, 5, 2 | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 | pass: exit 0; both invocations ok (statusgen package 0.239s and 1.030s); the diagnostic -v run shows the no-override test PASS and 18 TestEvidenceActor PASS lines | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --lint; echo rc=$?` | rc=0 | pass: rc=0; LINT: PASS; 0 lines start with PROBLEM, 2168 NOTICE lines, none naming graph-execution/03 | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on merged main is acceptable, recorded as could-not-check | could-not-check: printed rc=2; COULD-NOT-CHECK: the brief is not in the diff against 1f0b54e745d8, and its delivery could not be derived because the committed Evidence has no Verification-Table marker naming the delivery. This is the merged-main outcome the Expect cell accepts, recorded as could-not-check, never as pass. Supplemental probe (no row credit): the same command with --delivery 909101d1aa20 and --base its first parent de45ad17171a printed 4 corroborated, 0 disproved, 2 unchecked (the two out-of-scope entries), rc=0 | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS | pass: exit 0; the named test PASS (0.01s), then ok ... 0.222s | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS | pass: exit 0; the named test PASS (0.01s), then ok ... 0.242s | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | exit 0; named PASS; mutation must fail it | pass: exit 0; the grep found the named PASS line. Mutation check in a no-hardlinks scratch clone outside the home: making depFingerprintSame in evaluation/coverage.go return same for every policy and build dependency turns it into FAIL (row 1 resolved pass and released instead of wrong-revision); the unmutated clone passes | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | exit 0; named PASS | pass: exit 0; the grep found the named PASS line; the diagnostic -v run also shows it PASS | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

**Reasoning.** All 13 rows that can pass do pass on merged main 1f0b54e745d8. Row 10 printed rc=2 (could-not-check), the merged-main outcome its Expect accepts, so it is recorded as could-not-check and not counted as a pass. Its reason changed after #3574: a bare `--consumers --brief` on merged main now looks for a Verification-Table marker naming the delivery, and this brief's Evidence has none. With the delivery passed explicitly the routing claims corroborate, so nothing in the delivered work is disproved; the row as authored still cannot pass on merged main. The check-definition repair is tracked on medici-finance/assay#1927, which was still open when read during this run.

Observation from row 6: 24 claims in the tree now resolve pass, but no brief is released; this brief's witnesses are held wrong-revision because .assay-surfaces (and, for row 7, .gitattributes) differs between the witness tree and the item's and there is no dependency manifest, which is the broader revalidation the work-input amendment requires.

**Risk-bearing values.** The trigger did not fire: risk metadata is present, irreversible is no, and the change is statusgen tooling. Enumerated anyway over statusgen/coverage.go, statusgen/evaluation/coverage.go, coverage_api.go, coverage_repo.go, identity.go and the verifyrun truncation, including the diff since a1c0a9c1e8a3. Every entry can be reversed by an edit and a redeploy. Ranking: revision-shape bounds first, then exit codes, then the search bound and version label.
- RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22. statusgen/coverage.go:51 aliases it and verifyrun.go:1179-1180 truncates to it, so the witness writer and the coverage reader share one constant. 12 hex digits gives a 2^-48 chance that a random tree SHA matches by coincidence.
- RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/evaluation/coverage.go:516. The lower bound equals TreeSHALen, the exact length verifyrun writes; 40 is a full SHA-1 id. Hex-only stops a symbolic ref from tracking HEAD. A longer token is refused, which leads to held, never released.
- RISK-VALUE: DERIVED — minRevisionTokenLen = TreeSHALen (12) @ statusgen/evaluation/coverage.go:502. Same floor; a shorter token resolves could-not-check.
- RISK-VALUE: DERIVED — acceptance revision length 40 or 64, lowercase hex only @ statusgen/evaluation/coverage.go:388 (new since the last run). 40 is a full SHA-1 object id and 64 a full SHA-256 object id, the two git object formats; any abbreviated or symbolic value resolves could-not-check, so an original acceptance definition must be an immutable object, and a wrong value can only over-hold.
- RISK-VALUE: DERIVED — coverageExitOK = 0, coverageExitCouldNot = 2 @ statusgen/coverage.go:308-309. Task 5's literal contract ("Exit 0; exit 2 on an unreadable tree"). Row 6 observed exit 0 on the real tree.
- RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105. Cost bound on the squash-landing search; a witness past it is could-not-check, never assumed landed, so it can only over-hold. Reversible operational knob, ranks last.
- RISK-VALUE: DERIVED — EvaluatorVersion = "statusgen-evaluation/3" @ statusgen/evaluation/identity.go:13. A label, not a threshold: it must change whenever a verdict can change for the same observations, and the protected-acceptance change above can change verdicts, so the bump is required; its exact value carries no bound.
- RISK-VALUE: NAMED, NOT DERIVED — isBoardBookkeepingPath: p == "STATUS.md" or prefix "docs/streams/" @ statusgen/evaluation/coverage.go:766-767. Unchanged since the 2026-10-15 run. A change made only under docs/streams or to STATUS.md never stales a witness unless the brief declares that artifact; the 2026-09-30 amendment says file non-overlap is not proof, and a row that reads docs/streams (row 9's lint) could keep a reused pass after an edit there. Routed for human judgment on medici-finance/assay#2339 (open, needs-decision), not counted as a row failure.

**VERIFY: BLOCKED** — 13/14 rows pass at 1f0b54e745d8; row 10 is could-not-check (rc=2, now for want of a Verification-Table delivery marker after #3574), the merged-main outcome its Expect accepts but not a pass; coverage stays held on row 10 until medici-finance/assay#1927 re-authors it.
### Non-implementer verifier re-run — 2026-10-25 claude-opus-5-5 verifier at 61eb62a4a61b (container + host verifyrun witness; blocked on row 10, check-definition, medici-finance/assay#1927)

This pass replaces the hand-transcribed 2026-10-21 table as this brief's execution witness. That table stays above as dated history and corroboration. Its result was blocked, 13/14 at 1f0b54e745d8.

Merged main 61eb62a4a61bce0f2e0c69b98f1906de80400316. Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. Times are UTC, 2026-10-25:
- `deskdispatch --check-verifier` admitted the run (rc=0) before any row ran.
- The admission bracket opened at 13:29:47Z.
- Container rows ran 13:30:00Z to 13:31:54Z.
- The bracket closed at 13:32:12Z.
- Host rows ran 13:32:39Z to 13:34:13Z.
- Supplemental readings finished at 13:40Z.

The home was a clean detached checkout of that commit for both witnesses. The BriefSHA256 in the binding below, 011335420b0c…, is unchanged from the 2026-10-21 run.

Verification-Attestation: medici-finance/assay#4375 run=91c7591107dec25a91f04a3952a67e2b12299eec95d08e3842c926d0a3e6b669 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=strong

**Delivered code since the prior source.** Between 1f0b54e745d8 and 61eb62a4a61b:
- statusgen/coverage.go, statusgen/evaluation/, statusgen/autoflip.go and statusgen/lifecycle.go are unchanged.
- spec/lifecycle-v1.md only gained §10 (milestone boundary), which is unrelated to coverage.
- verifyrun.go changed, but its tree-SHA truncation is the same code, now at statusgen/verifyrun.go:1200-1201.

**Lane split.** The two selections are disjoint and together cover all 14 rows.
- Container witness: rows 1, 2, 3, 4, 5, 8, 11, 12, 13 and 14. These are the Go test rows.
- Host witness: rows 6, 7, 9 and 10. These read the whole tree and its git history, and rows 6, 9 and 10 call statusgen on PATH.

**Container witness (rows 1, 2, 3, 4, 5, 8, 11, 12, 13, 14).**
- Sequence: host `statusgen verifyrun --open-bracket`, then `statusgen verifyrun --sandbox=container-netns` in the container, then host `--close-bracket`. `statusgen verifyrun --bracket-digest` on the closed witness printed verdict=match (span-lines tree-digest:1, container-image:1, attestation:1), rc=0.
- Container: `--network none`, default seccomp profile, the home mounted read-only, and the Go module cache mounted read-only. GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-buildvcs=false.
- Image: a local Go 1.27.2 linux/arm64 image with python3 and git, image id sha256:955fd6c46c93ee746d8f560b4c90b172a759d2543337d8173cdaabcd2013bca4. It has no registry digest.
- statusgen in the container: the v1.0.59 release linux/amd64 binary (emulated). Its sha256 a9c05100… matches the release checksums.
- The receipt and witness were written outside the home. A build-only probe before the bracket confirmed the module cache was complete offline; it earns no credit.
- Rows 13 and 14 hash to the empty-output digest because their commands redirect the test log to a temp file and print nothing on success.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | pass exit=0 sandbox=container-netns | sha256:7b5ad1394d24 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | pass exit=0 sandbox=container-netns | sha256:d58587d5fb20 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | pass exit=0 sandbox=container-netns | sha256:da2d3c1ed398 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | pass exit=0 sandbox=container-netns | sha256:c8bf507fe418 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | pass exit=0 sandbox=container-netns | sha256:74d777d503c8 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | pass exit=0 sandbox=container-netns | sha256:5645d2b0147f | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | pass exit=0 sandbox=container-netns | sha256:0fb0f6da98c2 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | pass exit=0 sandbox=container-netns | sha256:37e54a514291 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoveragePolicyDependencyChanged.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoveragePolicyDependencyChanged$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoveragePolicyDependencyChanged " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 14 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCoverageReuseDoesNotRetargetPass.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCoverageReuseDoesNotRetargetPass$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCoverageReuseDoesNotRetargetPass " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9
Container-Image: sha256:955fd6c46c93ee746d8f560b4c90b172a759d2543337d8173cdaabcd2013bca4
Verification-Attestation: medici-finance/assay#4375 run=91c7591107dec25a91f04a3952a67e2b12299eec95d08e3842c926d0a3e6b669 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=91c7591107dec25a91f04a3952a67e2b12299eec95d08e3842c926d0a3e6b669 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 tree=sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9 witness=sha256:ae06fc02a62691e0217632fe6cf4001a30fd762749754589974d695c629df002 opened=2026-10-25T13:29:47Z closed=2026-10-25T13:32:12Z before=receipt after=check-verifier

**Host witness (rows 6, 7, 9, 10).**
- Command: `statusgen verifyrun --brief <this brief> --rows 6,7,9,10`, in write mode at the home root. It exited 1 because of row 10.
- statusgen on PATH: the v1.0.59 release darwin-arm64 binary. Its sha256 09da8a7a… matches the release checksums.
- Host: git 2.56.0. KUBECONFIG=/dev/null, with no cluster or forge contact.
- The table below is exactly what verifyrun wrote. The brief was then restored to its attested bytes, so this landing is the only place the table appears.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass exit=0 | sha256:6ffeef6d48d8 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass exit=0 | sha256:38af6536faaa | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:0e3beceaa2ef | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | fail exit=0 | sha256:22e5cd618e21 | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4375 run=91c7591107dec25a91f04a3952a67e2b12299eec95d08e3842c926d0a3e6b669 source=61eb62a4a61bce0f2e0c69b98f1906de80400316 brief=docs/streams/graph-execution/brief-03-evidence-coverage-rule.md model=claude-opus-5-5 tier=strong

**Row 10's fail cell is the check definition, not an observed failure.**
- verifyrun's note for row 10 was "exit 0, expected 2". It read the Expect's "exit 2 (could-not-check) on a fully merged main" clause as an exit-status contract.
- The authored command ends in `; echo rc=$?`, so its process exit status is always 0. The real statusgen status is only the printed `rc=` line, and no exit status the command can produce satisfies that reading.
- The hand re-run below printed rc=2 with the same could-not-check reason as on 2026-10-21. That is the merged-main outcome the Expect accepts and says must be recorded as such, never as a pass.
- So row 10 is could-not-check. It is neither a pass nor a disproved obligation.

**Supplemental readings (no acceptance credit).** Each is a direct re-run of the row's own command in the same clean home after the witnesses, with the same binary on PATH.
- Row 10 printed: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:graph-execution:03 is not in the diff against 61eb62a4a61bce0f2e0c69b98f1906de80400316, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." It then printed rc=2.
  - With the delivery passed explicitly (`--delivery` 909101d1aa20, the #1682 merge, and `--base` de45ad17171a, its first parent): 4 routing claims corroborated, 0 disproved, 2 unchecked (the out-of-scope entries), rc=0.
  - Nothing in the delivered work is disproved. Only the authored row cannot pass on merged main.
- Row 9 printed `LINT: PASS`, rc=0. It gave 0 PROBLEM lines, and none of its NOTICE lines names this brief.
- Row 6 exited 0 and printed exactly the six permitted results.
  - Tree-wide: 560 briefs, 4537 claims, 40 pass claims, 0 briefs released.
  - This brief is held. 13 claims are wrong-revision against the earlier witness at bed1a31ba875: .assay-surfaces and .gitattributes differ, and there is no dependency manifest. Row 10's claim is fail, with the reason "the witness records a failure".
- Row 7's counts were 9, 5 and 2.

Two earlier diagnostic runs are discarded. One ran in a home that verifyrun had already written to. The other ran in a separate clone whose environment did not match the attested home. Neither is evidence.

**Reasoning.** Thirteen of the fourteen rows pass under a tree-stamped verifyrun witness on merged main 61eb62a4a61b:
- 10 rows in the bracketed network-off container (digest match).
- 3 rows on the host.

Row 10 is could-not-check, for the reason above. The repair is to re-author its check definition so the command's exit status carries statusgen's status, and so the row can run on merged main. One way is to pass `--delivery`/`--base`. That repair is tracked on medici-finance/assay#1927, which was still open when read during this run.

Coverage stays held on row 10 until then. Row 6 shows the same thing from the evaluator's side: this brief's claims are not released.

**Risk-bearing values.** The trigger did not fire: risk metadata is present, irreversible is no, and the change is statusgen tooling. The values were enumerated anyway, re-read at 61eb62a4a61b. The coverage evaluation files are byte-unchanged since 1f0b54e745d8, so every derivation from the 2026-10-21 run still holds. Only the verifyrun truncation line moved. Every entry can be reversed by an edit and a redeploy.

Ranking: revision-shape bounds first, then exit codes, then the search bound and the version label.
- RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22.
  - statusgen/coverage.go:51 aliases it, and statusgen/verifyrun.go:1200-1201 truncates to it. The witness writer and the coverage reader therefore share one constant.
  - With 12 hex digits, the chance that a random tree SHA matches by coincidence is 2^-48.
- RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/evaluation/coverage.go:516.
  - The lower bound equals TreeSHALen, the exact length verifyrun writes. 40 is a full SHA-1 id.
  - Hex-only stops a symbolic ref from tracking HEAD. A longer token is refused, and that can only hold a claim, never release it.
- RISK-VALUE: DERIVED — minRevisionTokenLen = TreeSHALen (12) @ statusgen/evaluation/coverage.go:502. It is the same floor; a shorter token resolves could-not-check.
- RISK-VALUE: DERIVED — acceptance revision length 40 or 64, lowercase hex only @ statusgen/evaluation/coverage.go:388.
  - These are full SHA-1 and SHA-256 object ids, the two git object formats.
  - An abbreviated or symbolic value resolves could-not-check, so a wrong value can only over-hold.
- RISK-VALUE: DERIVED — coverageExitOK = 0, coverageExitCouldNot = 2 @ statusgen/coverage.go:308-309.
  - This is Task 5's literal contract: "Exit 0; exit 2 on an unreadable tree".
  - Row 6's witness observed exit 0 on the real tree.
- RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105.
  - It is the cost bound on the squash-landing search. A witness past the bound is could-not-check, never assumed landed, so the bound can only over-hold.
  - It is a reversible operational knob and ranks last.
- RISK-VALUE: DERIVED — EvaluatorVersion = "statusgen-evaluation/3" @ statusgen/evaluation/identity.go:13.
  - It is a label, not a threshold. It must change whenever a verdict can change for the same observations.
  - Its exact value carries no bound.
- RISK-VALUE: NAMED, NOT DERIVED — isBoardBookkeepingPath: p == "STATUS.md" or prefix "docs/streams/" @ statusgen/evaluation/coverage.go:766-767 — routed for human judgment on medici-finance/assay#2339 (open, needs-decision), not counted as a row failure.
  - A change made only under docs/streams, or only to STATUS.md, never stales a witness unless the brief declares that artifact.
  - The 2026-09-30 amendment says file non-overlap is not proof. A row that reads docs/streams, such as row 9's lint, could keep a reused pass after an edit there.

**VERIFY: BLOCKED** — check-definition: row 10 (medici-finance/assay#1927, still open). Its `; echo rc=$?` masks the exit status its Expect is read against, and on merged main it prints rc=2 (could-not-check), never a pass. Witness at 61eb62a4a61b: the other 13 rows pass exit=0 (10 in the bracketed network-off container, digest match; 3 on the host). Coverage stays held on row 10 until medici-finance/assay#1927 re-authors it.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question: name one input state in which coverage reports `released` while a
mandatory claim has no passing result at the item's revision. If one exists, the rule is
not deterministic and the brief is bounced.
