---
brief: assay:assay:statusgen:15
title: Extract canonical eligibility and coverage evaluation API
why: "Workflow and projection consumers need the same eligibility and coverage answers without launching statusgen or copying its evaluator."
wave: 1
depends: []
unblocks: ["graph-execution/19"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-08 by statusgen newbrief
sources:
  - "#2395 — library-first API planning"
  - docs/library-first.md
  - "freshness-checked 2026-10-08 @ 403b8ec8c26b21b85d88b5faea4cff619550e602"
exec-tier: strong
exec-tier-why: Extraction must preserve cross-component evidence and eligibility semantics while separating existing filesystem effects.
domain: complicated
consumers:
  - "statusgen/eligibility.go, statusgen/coverage.go, statusgen/nextup.go, statusgen/drivefrontier.go: follow-up statusgen/15"
  - "statusgen/evaluation, statusgen/go.mod, docs/contracts.md, tools/desk/internal/arch: follow-up statusgen/15"
  - "workflow/bindings: follow-up graph-execution/19"
  - "projection consumers outside this repository: out-of-scope (adopter pins and consumer rollout remain adopter-owned)"
id: a82664a9-c0a8-456f-b152-66214afff2e1
---

# Brief 15 — Extract canonical eligibility and coverage evaluation API

## Context

files: `statusgen/evaluation/` (new), `statusgen/eligibility.go`, `statusgen/coverage.go`,
`statusgen/nextup.go`, `statusgen/drivefrontier.go`, their tests and model-loading helpers,
`statusgen/testdata/evaluation/` (new), `statusgen/evaluation_contract_test.go` (new),
`docs/contracts.md`, `tools/desk/internal/arch/` (existing owner/marker checks),
`.github/workflows/assay-statusgen.yml`, `docs/library-first.md`,
`changelog/statusgen-evaluation-library.md` (new).

facts (403b8ec8c26b21b85d88b5faea4cff619550e602, 2026-10-08): eligibility and coverage
are implemented in package main; eligibility imports os/filepath and coverage includes git
observation. Offline does not mean pure. `statusgen/streamview` already contains importable
wire types. `statusgen/graphcontract` does not exist; graph-execution/19 formerly owned a
second extraction. It now consumes this brief. The existing coverage binding seam accepts
explicit bindings; this brief adds no production workflow-store binding.

layering: statusgen owns the meaning; a pure evaluation package receives normalized models,
observations and an explicit evaluation context. Existing command adapters load files/git
observations and map results. Task 1–3 and Verify 1–4 enforce the boundary.
design-fit:
  owner: statusgen/evaluation (extracted from statusgen, same semantic owner)
  contract: S-eligibility
  retires: [package-main-only evaluator implementation, graph-execution/19 independent extraction]
  weight: "verbs 0, flags 0, refusals 0; one importable API, no duplicate evaluator"
  why-add: "A supported package replaces process-only access; exporting the whole CLI would retain effects and prevent reuse."

Read first: `docs/library-first.md`, `docs/contracts.md` (S-eligibility and S-delivery),
`docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`,
`docs/streams/graph-execution/brief-19-durable-instance-store.md`.

## Ground rules

Work in an isolated branch with offline fixtures. No live providers, release, merge or rollout.
Keep existing command/wire behavior. Stop at implemented; verification is independent.
Effort L is deliberate: eligibility and coverage share models and caller mappings; one atomic
extraction avoids two temporary model owners. Re-cut before implementation if the dependency
inventory requires unrelated parser, policy or full-board migration.

## Task

1. Capture pre-extraction golden verdicts/reasons from existing fixtures, including unresolved
   cross-repo gates, stale acceptance, wrong revision, missing evidence and absent versus declared
   but unresolved pattern bindings. Inventory the transitive inputs; move filesystem/git loading
   into adapters. Do not make formerly unavailable forge refs resolvable as part of extraction.
2. Move the canonical evaluation functions and minimal immutable input/result types into
   `statusgen/evaluation`. Public entrypoints are typed EvaluateEligibility and EvaluateCoverage
   operations over supplied observations; no global environment, implicit clock, network, process,
   filesystem discovery, token or write operation. Preserve distinct revision kinds and reasons.
   Update existing statusgen callers in the same change. Remove the old implementations; thin
   mapping wrappers are allowed, independent recalculation is not.
3. Add a direct-package consumer fixture and compare it with the real statusgen CLI on identical
   inputs. Eligibility still does not grant admission; coverage still does not grant publication.
   Update semantic-owner/marker checks to the actual moved owner without weakening ceilings.
   Add transitive capability checks with planted violations and an effect-trapping offline flow.
4. Qualify external-module consumption through a temporary local module proxy with `GOWORK=off`,
   no relative replace and networking disabled. Pin/release documentation must cover the statusgen
   module's public API and evaluator identity. Extend existing CI path coverage for every file the
   cross-module checks read. Document the API, migration and provenance; add the changelog fragment.

## Verify

The named tests below are deliverables, not existing evidence. Each must run production code,
assert independent expected results and retain a fail-first mutation; no skipped/missing test passes.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | exit 0; named PASS; Same frozen inputs produce equal CLI/library verdicts and reasons; changing a mapping makes the test fail. |
| 2 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | exit 0; named PASS; Missing/wrong-revision/stale/unresolved evidence holds; known no-binding remains distinct; treating unknown as satisfied fails. |
| 3 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | exit 0; named PASS; Transitive process/network/filesystem/credential dependencies refused; planted indirect import fails, pure control passes. |
| 4 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | exit 0; named PASS; External module builds and exercises both APIs with GOWORK=off, no relative replace and local proxy only; unsupported API expectation fails. |

## Pre-mortem

A moved function can change missing-data meaning (row 2), leave a second implementation or
bad mapping (row 1 and owner checks), hide effectful helpers (row 3), or build only in the
workspace (row 4). API ergonomics and L scope remain review judgments.

## Evidence

<!-- Non-implementer records merged revision, commands, results and retained red controls. -->
<!-- Non-implementer records merged revision, commands, results and retained red controls. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2663 run=bcd98070405617fdef9d2fa5bdca93f6d74d161132d0211adaa7ab6889f1809c source=18d752a4c9318e27bcfd49fa44f9cfb62923b691 brief=docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md model=gpt-6.1-sol tier=any


**VERIFY: BLOCKED** — all four required check:ci rows were not run; the witness executor returned exit 2 because the network-off sandbox is unavailable on this Darwin host. No implementation failure was observed. Status remains implemented.

Environment blocker: [assay#2665](https://github.com/medici-finance/assay/issues/2665). Wake: an operator provides a supported Linux verifier with the required network-off sandbox and records an explicit recheck.

RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22 — the behavior-preserving extraction must retain the existing 12-character witness tokens (pre-extraction statusgen/verifyrun.go:150); graph-execution/03 records that format, and the revision-token floor already used it. This static derivation does not clear the blocked execution rows.

Risk metadata is explicitly all no and gate is model. Full diff-wide literal enumeration and remaining derivations are incomplete in this blocked pass; no PASS or sign-off is claimed.
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 946fd67330e5 (4/4 rows PASS; control and 13/13 mutations fail their named test)

Verification-Attestation: medici-finance/assay#2934 run=8408ed48b3dfb616de0551a185c2eac325ae214e514d5dee7fe9a9f3ddebd2df source=946fd67330e5bbc6acc0fe3e0bbe9521d7786caa brief=docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md model=claude-opus-5-5 tier=strong

A fresh non-implementer verifier ran every Verify command exactly as written. The home was a detached worktree at merged main 946fd67330e5, admitted by `deskdispatch --check-verifier` (rc 0). Host: darwin, go1.27.2, git 2.56.0. The run was offline: GOPROXY=off, GOSUMDB=off, and modules came from the local cache only. The network-off witness sandbox the check:ci class uses for hermetic re-execution does not exist on this host (#2665). These rows ran directly, and CI's network-off re-execution remains the hermetic arm. The delivering change is #2552, followed by #2693. Before the rows, the verifier grounded on that diff. The old evaluators are gone from package main: its eligibility.go and coverage.go are now thin mappers that call `evaluation.EvaluateEligibility` and `evaluation.EvaluateCoverage`. The verifier also ran a separate mutation sweep on a git-archived copy of the statusgen and forgeread modules at the same commit, applying each entry in the shipped evaluation-mutations.json. The baseline passed. The control (eligibility adapter swaps edge type and reason) and all 13 mutations were caught, each by the test its row names: parity ×3, uncertain-evidence ×5, boundary ×3, external-consumer ×2.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | pass exit=0 | PASS: Test Evaluation Library CLI Parity (14.80s). Mutations: the control and 3 adapter-mapping mutations fail this test | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | pass exit=0 | PASS: Test Evaluation Uncertain Evidence Holds (3.94s). Mutations: all 4 unknown-treated-as-satisfied coverage mutations (no witness, wrong revision, unreadable row, brief not found) fail this test | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | pass exit=0 | PASS: Test Evaluation Pure Dependency Boundary (0.98s). Mutations: os/exec import, os import and a shallow walker each fail this test | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | pass exit=0 | PASS: Test Evaluation External Consumer (4.40s). Mutations: an added Admit API and a tools/desk manifest path each fail this test | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |

Risk metadata is explicitly all no, the gate is model, and the item is not irreversible. Every literal the diff introduces or moves was enumerated (package constants in the evaluation package). Each one equals its pre-extraction value or is a new label. All are reversible by an edit and a redeploy, and every bound fails safe to a hold.

RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22 — Task 2 requires a behavior-preserving extraction. The pre-extraction value was treeSHALen = 12 (verifyrun.go:150 before #2552), the width every recorded witness token carries. minRevisionTokenLen = TreeSHALen (coverage.go:489) keeps the same floor.
RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105 — it equals the pre-extraction maxLandingCandidates = 1000 (coverage_inputs.go:729 before #2552). A search past the bound returns could-not-check (holds), so the bound can only withhold work, never release it.
RISK-VALUE: DERIVED — EvaluatorVersion = "statusgen-evaluation/1" @ statusgen/evaluation/identity.go:13 — this is the first generation of a newly introduced identity, so it starts at 1. It names the rules and carries no authority.

**VERIFY: PASS** — 4/4 rows exit 0 with the named PASS. The control and all 13 shipped mutations fail their named test.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3071 run=e33fa83e8492e0e1e3c9ca469ea9de4be6ad80db42eac8e96296e470cdbf4c42 source=a39ad90497c10cdc16413695c3ce9b74c2b098d6 brief=docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md model=claude-opus-5-5 tier=any

### Non-implementer verifier run — 2026-10-15 claude-opus-5-5 verifier at a39ad90497c1 (4/4 rows PASS by hand, network-off; control and 13/13 shipped mutations caught by the suite)

A fresh non-implementer verifier re-ran the Verify table at merged main a39ad90497c1. The home was a detached worktree admitted by `deskdispatch --check-verifier` (rc 0, binding above). Host: darwin, go1.27.2, git 2.56.0. `statusgen verifyrun` ran first on the clean tree and wrote the witness rows above. It exited 2 because its check:ci sandbox needs `unshare --net`, which darwin lacks (#2665).

Every row then ran by hand, verbatim from the Verify table, under macOS sandbox-exec. The profile was `(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))`. Before the runs, the verifier proved the profile works. Outside the sandbox, an HTTPS request to the Go module proxy returned 200. Inside it, the same request failed to resolve the host (curl exit 6), and a request by IP to 1.1.1.1 failed to connect (curl exit 7). A loopback HTTP server stayed reachable from inside (200). Rows ran with a throwaway HOME, the real build and module caches, GOPROXY=off, GOSUMDB=off and GOTOOLCHAIN=local.

Since the earlier PASS at 946fd67330e5, the evaluation package, its contract and scenario tests, and the fixtures have not changed. The mutation manifest changed only its go.mod anchor (#2944). The command adapter's change is #2998: the coverage adapter now wraps its git port in a per-evaluation memo, which caches failed reads as failed. The rows exercise that adapter path.

| # | Command | Expected | Observed | Date runner |
|---|---------|----------|----------|-------------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | exit 0; named PASS; mapping change fails | exit 0 — "--- PASS: TestEvaluationLibraryCLIParity (9.89s)"; no SKIP lines. Mutations: the control and the 3 parity adapter mutations each fail this test | 2026-10-15 assay-verifier-app[bot] @ a39ad90497c1 |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | exit 0; named PASS; unknown-as-satisfied fails | exit 0 — "--- PASS: TestEvaluationUncertainEvidenceHolds (3.01s)"; no SKIP lines. Mutations: 4 of the 5 shipped uncertain-evidence mutations fail this test (brief not found, no witness, wrong revision, unreadable row). The fifth (same-repo issue gate read as satisfied) passes this test; it fails rows 1 and 4's tests. See the note below | 2026-10-15 assay-verifier-app[bot] @ a39ad90497c1 |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | exit 0; named PASS; planted indirect import fails | exit 0 — "--- PASS: TestEvaluationPureDependencyBoundary (1.16s)"; no SKIP lines. Mutations: the os/exec import, the os import and the shallow walker each fail this test | 2026-10-15 assay-verifier-app[bot] @ a39ad90497c1 |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | exit 0; named PASS; unsupported API fails | exit 0 — "--- PASS: TestEvaluationExternalConsumer (3.52s)"; the consumer resolved through a file:// proxy with GOWORK=off inside the network-off sandbox. Mutations: the added Admit API and the manifest path under the internal tool module each fail this test | 2026-10-15 assay-verifier-app[bot] @ a39ad90497c1 |

Mutation sweep. The sweep ran on a git-archived copy of the statusgen and forgeread modules at a39ad90497c1, outside the home and under the same sandbox. It applied every entry in the shipped evaluation-mutations.json, one at a time, with the manifest's five-test command plus -v. The baseline passed 5/5. The control and all 13 mutations turned the suite red (rc 1), and the restored copy passed 5/5 again. Twelve were caught by the test their category names.

Note on mutation 5 ("uncertain evidence: a forge-backed gate is treated as satisfied"): this one changes only the same-repo issue path. It was caught by TestEvaluationLibraryCLIParity (frozen verdicts drift for forge-backed-gates) and TestEvaluationExternalConsumer (alpha/03 turns eligible), not by TestEvaluationUncertainEvidenceHolds. The cause is the forge-backed-gates scenario: its brief alpha/02 carries both a same-repo issue gate and a cross-repo issue gate, and that test checks only the verdict and the first hold. The untouched cross-repo hold keeps alpha/02 held. Mutating both forge-backed paths together does fail it ("forge-backed-gates:alpha/02: verdict "eligible", want "held""). So the regression class is caught by the Verify suite. The precise gap: row 2's own test does not isolate the same-repo issue path. The earlier run at 946fd67330e5 had the same tests and listed 4 mutations under row 2. This is a test-precision follow-up, not a row failure.

Risk metadata is explicitly all no, the gate is model, and the item is not irreversible. The enumeration covered every package-level literal in statusgen/evaluation: bounds, state and result labels, reference patterns and the identity. The numeric bounds equal their values before #2552. The labels are names, not authorities. Every value is reversible by an edit and a redeploy, and every bound fails safe to a hold.

RISK-VALUE: DERIVED — TreeSHALen = 12 @ statusgen/evaluation/coverage_api.go:22 — the extraction must preserve behavior. The value before #2552 was treeSHALen = 12 (statusgen/verifyrun.go:150 at 1235e8304^), the width every recorded witness token carries. minRevisionTokenLen = TreeSHALen (statusgen/evaluation/coverage.go:489) keeps the same floor, so a shorter token proves nothing.
RISK-VALUE: DERIVED — hexRevisionRe = `^[0-9a-fA-F]{12,40}$` @ statusgen/evaluation/coverage.go:503 — identical to the pre-extraction pattern (statusgen/coverage.go:725 at 1235e8304^). The lower bound 12 is TreeSHALen, and the upper bound 40 is a full SHA-1 object name. A token outside the pattern is never credited.
RISK-VALUE: DERIVED — DefaultMaxLandingCandidates = 1000 @ statusgen/evaluation/coverage_repo.go:105 — equals the pre-extraction maxLandingCandidates = 1000 (statusgen/coverage_inputs.go:729 at 1235e8304^). A search past the bound returns could-not-check, which holds, so the bound can only withhold work.
RISK-VALUE: DERIVED — EvaluatorVersion = "statusgen-evaluation/1" @ statusgen/evaluation/identity.go:13 — the first generation of a newly introduced identity, so it starts at 1. It is unchanged since 946fd67330e5, names the rules, and carries no authority.

**VERIFY: PASS** — 4/4 rows exit 0 with the named PASS under a proven network-off sandbox. The control and 13/13 shipped mutations fail the suite; 12/13 fail the test their category names, and mutation 5 is caught by rows 1 and 4 instead of row 2 (precision note above).

### Network-off execution witness — 2026-10-19 claude-opus-5-5 verifier at 4b15482af5c6 (4/4 rows pass, sandbox=container-netns)

A fresh non-implementer verifier was admitted by `deskdispatch --check-verifier` (rc 0) against merged main 4b15482af5c6c71b9312ca7df0f23b219de3f840. The binding is in the Verification-Attestation line below. The execution witness ran under the driver's 2026-10-19 interim-runner ruling (recorded on the private house tracker). The runner was a local docker container started with `--network none` and the engine's default seccomp profile. statusgen and the desk tools were built from the cloned tree inside it, and `statusgen verifyrun --sandbox container-netns` ran non-dry with GOPROXY=off and GOTOOLCHAIN=local. verifyrun exited 0 and wrote the witness table below.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | pass exit=0 sandbox=container-netns | sha256:9aeb5c7332b1 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | pass exit=0 sandbox=container-netns | sha256:f7199ac1a095 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | pass exit=0 sandbox=container-netns | sha256:12dacdaee356 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | pass exit=0 sandbox=container-netns | sha256:6a764910a93b | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3490 run=0fb94e6086281a7faabdcaeb2f1de6f012b8dfca6a5931633f45733ec8ddec67 source=4b15482af5c6c71b9312ca7df0f23b219de3f840 brief=docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md model=claude-opus-5-5 tier=strong

The substance is the existing by-hand Evidence block of 2026-10-15 at source a39ad90497c10cdc16413695c3ce9b74c2b098d6. In it all 4 rows passed network-off, and the control and 13/13 shipped mutations were caught by the suite. This pass adds the container execution witness to that substance.

**VERIFY: PASS** — all 4 Verify rows now carry a passing network-off execution witness (exit 0, sandbox=container-netns) at 4b15482af5c6, on top of the 2026-10-15 by-hand PASS.

## Review

Gate: model. This is behavior-preserving offline extraction, not an authority change. Any
proposal to change evidence acceptance, identity or admission is separate work with its own gate.
