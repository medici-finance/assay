---
brief: assay:assay:milestone-support:04
title: "Add negotiated brief schema and prefixed authoring"
why: "New brief references and milestone start requirements need a real schema and safe generator rather than tolerated unknown keys."
wave: 2
depends: ["milestone-support/01", "milestone-support/02"]
unblocks: ["milestone-support/05", "milestone-support/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: add5e7f2-1524-4cdb-b4c2-f035eeb32f26
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
  - "statusgen/briefv2.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/brieffile.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/briefschemaversions.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/newbrief.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/newbrief_test.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/conform_test.go: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/testdata/milestone-support/04/: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "schemas/brief-v4.json: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "spec/brief-v1.md: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "docs/contracts.md: follow-up milestone-support/04 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-05-build-the-combined-milestone-and-brief-dependency-graph.md: follow-up milestone-support/05 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md: follow-up milestone-support/08 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 04 — Add negotiated brief schema and prefixed authoring

## Context

files: `statusgen/briefv2.go`, `statusgen/brieffile.go`, `statusgen/briefschemaversions.go`, `statusgen/newbrief.go`, `statusgen/newbrief_test.go`, `statusgen/conform_test.go`, `statusgen/testdata/milestone-support/04/` (planned), `schemas/brief-v4.json` (planned), `spec/brief-v1.md`, `docs/contracts.md`, `changelog/milestone-support-04.md` (planned), `statusgen/testdata/milestone-support/04-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen brief parsing and newbrief owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen brief parsing and newbrief"
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

1. Allocate the next brief schema only after rechecking the supported-schema table and parallel work; brief-v4 is the current proposal, not an already shipped version. Extend validator, descriptive JSON schema and capability/version coverage together. Preserve v1-v3 validation and existing fullIds.
2. Implement full colon-only b- definition IDs, separate authored version, explicit requires-milestones, and optional exact brief revision selectors on references using workref. Bind an unversioned ref to the selected source snapshot; unavailable requested versions remain unknown. Do not mint a UUID or task for an alias/version selector.
3. Extend newbrief and stream scaffolding through the existing front door. Select the supporting identifier/schema pair only after /02 validates every required consumer. Unsupported output must refuse or use an explicitly selected supported legacy mode; never silently drop a requested gate. Keep filenames, source provenance, risk derivation, dependency inverses and existing generated cells intact.
4. Provide cross-schema and old-reader fixtures. No corpus migration, operational declaration, spec-state flip or supporting-release number is fabricated in this implementation.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedBriefSchema$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedBriefSchema ' "$milestone_verify_out")` | exit 0 and named PASS; actual parser validates new IDs/requirements, legacy forms still read once, duplicate UUID aliases and unavailable versions refuse/are unknown; a parser accepting a mismatched schema fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewbriefCaps$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNewbriefCaps ' "$milestone_verify_out")` | exit 0 and named PASS; actual generator writes a valid ID/schema pair and inverses; unsupported required consumers cause zero writes, even with a tolerant YAML decoder. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
