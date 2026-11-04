---
brief: assay:assay:milestone-support:05
title: "Build the combined milestone and brief dependency graph"
why: "A milestone must describe completion work without accidentally making every contributor wait for its final acceptance prerequisites."
wave: 3
depends: ["milestone-support/03", "milestone-support/04"]
unblocks: ["milestone-support/06", "milestone-support/07", "milestone-support/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 6ad9458b-3e56-4969-8dd1-366164f55037
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/topology.go: fixed-here (inspected: interprets no milestone relationship; nothing to retire)"
  - "statusgen/briefv2.go: fixed-here (GraphEdge carries a pre-resolved milestone start-gate state)"
  - "statusgen/evaluation/: fixed-here (BuildMilestoneGraph; the evaluator reads resolved edges as given)"
  - "statusgen/testdata/milestone-support/05/: fixed-here (mixed-graph fixture tree)"
  - "docs/dependency-graph-design.md: fixed-here (section 3.8)"
  - "docs/streams/milestone-support/brief-06-bind-milestone-acceptance-to-existing-evidence-and-decisions.md: follow-up milestone-support/06 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-07-evaluate-milestone-readiness-through-the-canonical-eligibility-api.md: follow-up milestone-support/07 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-09-preserve-workflow-subjects-and-human-rulings-across-aliases.md: follow-up milestone-support/09 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 05 — Build the combined milestone and brief dependency graph

## Context

files: `statusgen/topology.go`, `statusgen/briefv2.go`, `statusgen/evaluation/`, `statusgen/testdata/milestone-support/05/` (planned), `docs/dependency-graph-design.md`, `changelog/milestone-support-05.md` (planned), `statusgen/testdata/milestone-support/05-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen graph resolution owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen graph resolution"
  contract: S-eligibility
  retires: ["duplicated interpretation or implicit state inference at the named existing consumer boundary"]
  weight: "New binaries 0; new services 0; required pure API/schema/fixture changes only. Measure actual rule/flag/refusal deltas at implementation review; no current weight reduction is claimed."
  why-add: "Reuse the existing semantic owner and public module. This records a missing shared meaning; moving it into a helper process or copying another parser would add authority or drift without benefit."

single-point-of-failure: incorrect shared identity or capability interpretation; retain independent current-subject, authority and consumer checks and plant faults with the outer check bypassed where Verify names that boundary.

## Read first

- [Implementation scope and consumer census](README.md#implementation-scope).
- [Milestone contract](../../../spec/milestone-v1.md).
- [Brief contract and compatibility](../../../spec/brief-v1.md).
- [Stream contract](../../../spec/stream-v1.md).
- [Library-first direction](../../library-first.md).
- [Existing semantic owners](../../contracts.md).
- [Dependency waves and adoption holds](README.md).

## Ground rules

- Own isolated branch and draft implementation PR. Do not merge, flip status/spec/stream/PR readiness, dispatch work, cut a tag, change installed pins or contact a cluster.
- Scope approval and stream activation take effect on the maintainer's merge of PR #4182; this brief has no completion evidence. A non-implementer owns post-merge verification; never fill Evidence from authorship or a fixture expectation.
- Refresh main, active claims/PRs, module imports, source contracts and consumer inventory at pickup. Record brief/source revisions, actual public API availability, coverage/unknowns and continue/adapt/hold in the implementation handoff. No obsolete active claim is overwritten.
- Preserve existing credentials, independent human/risk gates, UUIDs, archived evidence and exact-subject meanings. No ambient-auth fallback or automatic bulk migration.
- Shared resolution and capability assessment MUST be direct library calls, not subprocess calls to statusgen, desk-tools or a new helper executable. External I/O remains in existing adapters.

## Task

1. Extend the existing graph/resolver with typed completion, milestone-prerequisite and explicit brief-start edges. Resolve pinned source revisions through an injected read-only source port, preserving root and UUID identity; no online forge client belongs in the pure graph package.
2. Derive reverse membership and deduplicate shared briefs from canonical contribution lists. Keep brief waves based on existing brief dependencies and expose capability gates separately. Do not create an implicit M1-to-M2 chain or inherit every milestone prerequisite onto each brief.
3. Detect direct/indirect cross-root cycles over completion and execution relations, including a brief requiring its own milestone. Report known blockers alongside coverage unknowns; withdrawn or missing work never satisfies an edge. Retire duplicated graph interpretation at consuming sites.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneMixedGraph$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneMixedGraph ' "$milestone_verify_out")` | exit 0 and named PASS; 11 starts independently, 12 needs Y, and Z remains final-only; deleting a prerequisite or blanket-inheriting a gate fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneGraphUnknowns$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneGraphUnknowns ' "$milestone_verify_out")` | exit 0 and named PASS; self/indirect cross-root cycles fail; aliases deduplicate and missing pinned revisions remain unknown rather than falling back. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->
### Non-implementer verifier run — 2026-11-04 assay-verifier-app (verify-desk, strong tier, gate:model) — **VERIFY: PASS**

Admitted home at assay 7f61cff94b18 (detached, equal to origin/main), attestation medici-finance/assay#5488. statusgen v1.0.73 on the host and in the verify-runner image. Both rows ran in the network-off container lane inside a closed admission bracket (`--bracket-digest` verdict=match, span-lines tree-digest:1, container-image:1, attestation:1); the darwin host has no `unshare --net`. `deskdispatch --check-verifier` exited 0 before and after the run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneMixedGraph$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneMixedGraph ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ 7f61cff94b18 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneGraphUnknowns$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneGraphUnknowns ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ 7f61cff94b18 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:f1f8a0ebeb8be2206d08a9df4679ec424311c242db143bd6d6ee22b616bfdac1
Container-Image: sha256:100dde7f1f3a8be53ae45e9c1ff21fcd9fc82760595c91c422670db36c25d8d9
Verification-Attestation: medici-finance/assay#5488 run=4f805489ce5040ff7c47254ba033c833a07d1b4b5de1d71e66e54bab7f8b43d8 source=7f61cff94b188d46b4e38e6259798d85bb5d4510 brief=docs/streams/milestone-support/brief-05-build-the-combined-milestone-and-brief-dependency-graph.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=4f805489ce5040ff7c47254ba033c833a07d1b4b5de1d71e66e54bab7f8b43d8 source=7f61cff94b188d46b4e38e6259798d85bb5d4510 tree=sha256:f1f8a0ebeb8be2206d08a9df4679ec424311c242db143bd6d6ee22b616bfdac1 witness=sha256:bef977406010a9c1fa8979ebd1570ca5b33a66e41c35fd3e59e9efacfed6e87b opened=2026-11-04T23:27:56Z closed=2026-11-04T23:28:39Z before=receipt after=check-verifier

Supplemental, no acceptance credit: declared mutation manifest statusgen/testdata/milestone-support/05-mutations.json (20 cases), run with its own runner, one case at a time, in a scratch git copy of source 7f61cff94b18, inside the same verify-runner image with no network.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | python3 statusgen/testdata/milestone-support/05-mutate.py --name blanket-inherited-gate | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT blanket-inherited-gate: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S2 | python3 statusgen/testdata/milestone-support/05-mutate.py --name uuid-dropped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT uuid-dropped: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S3 | python3 statusgen/testdata/milestone-support/05-mutate.py --name prerequisite-deleted | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT prerequisite-deleted: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S4 | python3 statusgen/testdata/milestone-support/05-mutate.py --name record-prerequisites-dropped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT record-prerequisites-dropped: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S5 | python3 statusgen/testdata/milestone-support/05-mutate.py --name start-gate-dropped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT start-gate-dropped: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S6 | python3 statusgen/testdata/milestone-support/05-mutate.py --name blocked-read-as-unknown | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT blocked-read-as-unknown: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S7 | python3 statusgen/testdata/milestone-support/05-mutate.py --name resolved-edge-ignored | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT resolved-edge-ignored: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S8 | python3 statusgen/testdata/milestone-support/05-mutate.py --name unloaded-record-as-none | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT unloaded-record-as-none: TestMilestoneMixedGraph | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S9 | python3 statusgen/testdata/milestone-support/05-mutate.py --name self-cycle-not-refused | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT self-cycle-not-refused: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S10 | python3 statusgen/testdata/milestone-support/05-mutate.py --name cycle-gate-untyped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT cycle-gate-untyped: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S11 | python3 statusgen/testdata/milestone-support/05-mutate.py --name milestone-edge-dropped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT milestone-edge-dropped: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S12 | python3 statusgen/testdata/milestone-support/05-mutate.py --name alias-double-counted | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT alias-double-counted: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S13 | python3 statusgen/testdata/milestone-support/05-mutate.py --name pinned-revision-fallback | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT pinned-revision-fallback: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S14 | python3 statusgen/testdata/milestone-support/05-mutate.py --name unknown-as-no-dependency | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT unknown-as-no-dependency: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S15 | python3 statusgen/testdata/milestone-support/05-mutate.py --name withdrawn-satisfies | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT withdrawn-satisfies: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S16 | python3 statusgen/testdata/milestone-support/05-mutate.py --name key-collision-picks-a-winner | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT key-collision-picks-a-winner: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S17 | python3 statusgen/testdata/milestone-support/05-mutate.py --name record-collision-picks-one | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT record-collision-picks-one: TestMilestoneGraphUnknowns | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S18 | python3 statusgen/testdata/milestone-support/05-mutate.py --name lattice-requalifies-every-path | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT lattice-requalifies-every-path: TestMilestoneLatticeStaysLinear | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S19 | python3 statusgen/testdata/milestone-support/05-mutate.py --name closure-not-deduplicated | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT closure-not-deduplicated: TestMilestoneLatticeStaysLinear | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |
| S20 | python3 statusgen/testdata/milestone-support/05-mutate.py --name gate-detail-uncapped | named baseline PASS, then named FAIL under the plant | exit 0; baseline PASS then CAUGHT gate-detail-uncapped: TestMilestoneLatticeStaysLinear | 2026-11-04 | assay-verifier-app[bot] (model verifier, supplemental) (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — maxGateDetail = 20 @ statusgen/evaluation/milestonegraph.go:409 — a display cap on start-gate hold-reason detail; the milestone node keeps the full list. Reversible knob.
RISK-VALUE: DERIVED — EvaluatorVersion = "statusgen-evaluation/5" @ statusgen/evaluation/identity.go:13 — bumped from /4 because edges now carry a pre-resolved milestone start-gate state.

**VERIFY: PASS** — both Verify rows exit 0 with their named PASS lines (TestMilestoneMixedGraph, TestMilestoneGraphUnknowns) at source 7f61cff94b18 in the closed container bracket; all 20 declared mutations turned their named test red (supplemental, no acceptance credit). Risk fields are all no and gate is model, so the README row flips implemented → verified.

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
