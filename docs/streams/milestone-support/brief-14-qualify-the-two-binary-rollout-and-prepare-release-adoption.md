---
brief: assay:assay:milestone-support:14
title: "Qualify the two-binary rollout and prepare release adoption"
why: "Source builds and shared imports are not proof that a packaged adopter has mutually compatible readers and safe migration behavior."
wave: 8
depends: ["milestone-support/10", "milestone-support/13"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 77fd5e65-b855-4e12-8fb7-ce184c0cc383
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
  - "statusgen/testdata/milestone-support/14/: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskinstall/: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/cursorbundle/: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "containers/: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "Makefile: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "docs/milestone-adoption.md: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/paired-versions.yaml: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "spec/milestone-v1.md: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
  - "spec/brief-v1.md: follow-up milestone-support/14 (this delivery; replace with fixed-here in implementation)"
---

# Brief 14 — Qualify the two-binary rollout and prepare release adoption

## Context

files: `statusgen/testdata/milestone-support/14/` (planned), `tools/desk/cmd/deskinstall/`, `tools/desk/internal/cursorbundle/`, `containers/`, `Makefile`, `docs/milestone-adoption.md` (planned), `plugins/assay/paired-versions.yaml`, `spec/milestone-v1.md`, `spec/brief-v1.md`, `changelog/milestone-support-14.md` (planned), `statusgen/testdata/milestone-support/14-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: cross-module qualification and release preparation owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "cross-module qualification and release preparation"
  contract: none — existing index does not yet name this exact typed record/projection; update its existing owner row in the implementation, not a second registry.
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

1. Build an offline candidate-bundle conformance matrix through the actual statusgen and desk-tools entrypoints. Include both new, old/new mixed and unsupported-required pairs; old/new aliases, exact/missing revision pins, shared contributors, cycles, parked/missing roots, final-only gates, revoked/wrong-subject acceptance and unchanged legacy fixtures must behave consistently.
2. Qualify two temporary adopter roots with supported stream opt-in, one cross-root capability and independently useful outcomes. Demonstrate the producer split without editing consumers, preserve legacy aliases/history, and prove missing acceptance remains outstanding after all contributors are done. This is a disposable synthetic pilot, not activation of real streams or acceptance of cellctl.
3. Verify the candidate source/artifact bundle includes every sibling module needed by GOWORK=off consumers, Docker/build contexts and generated skill copies. Local replace success is not proof of published module availability. Record exact candidate commits/tool versions/capability sets, unresolved publishing prerequisites and the actual release-time module pins required; do not invent future tags/checksums.
4. Prepare the adopter upgrade/runbook and rollback/hold conditions using existing pin/version tools. Reader/generator/dispatch/report support must be present before enabling prefixed writes or milestone gates. Prepare candidate metadata in fixtures; changing installed pins, activating streams, workflow dispatch, tag/release publication and human acceptance are explicitly separate operator acts.
5. Audit path-trigger/build-context coverage. If a CI workflow change is necessary, identify its owner and deliver it through the existing workflow-scoped maintainer gate, not a worker token workaround. Required unshipped infrastructure remains a recorded release blocker, never silently waived.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneBundleCompat$' ./cmd/deskinstall/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneBundleCompat ' "$milestone_verify_out")` | exit 0 and named PASS; actual temporary candidate/adopter paths verify supported pairs and zero writes on incompatible readers; missing sibling module/unsupported feature fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneTwoRootPilot$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneTwoRootPilot ' "$milestone_verify_out")` | exit 0 and named PASS; actual CLI fixtures cover consumer-preserving split, missing receipt, legacy regression, exact versions and incomplete coverage; planted alias duplication/blanket gate or false acceptance fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
