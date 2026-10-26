---
brief: assay:assay:milestone-support:09
title: "Preserve workflow subjects and human rulings across aliases"
why: "Reference compatibility must preserve exact work/evidence subjects and must never transfer a human ruling to a different revision or brief."
wave: 4
depends: ["milestone-support/05", "milestone-support/08"]
unblocks: ["milestone-support/10", "milestone-support/12"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: a25c8c4e-7b7b-4c59-8259-92b8f3f65469
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes how human rulings and verification evidence bind aliases and revisions; the bridge must never transfer a ruling or accepted evidence to another subject."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/contexthandoff/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/domain/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/decisionruling.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/decisiongatehold.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/rulinggate.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/briefschemagate.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskevidence/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "spec/context-handoff-v1.md: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "spec/verification-domain-v1.md: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md: follow-up milestone-support/12 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 09 — Preserve workflow subjects and human rulings across aliases

## Context

files: `statusgen/contexthandoff/`, `statusgen/domain/`, `statusgen/decisionruling.go`, `statusgen/decisiongatehold.go`, `tools/desk/internal/deskkit/rulinggate.go`, `tools/desk/internal/deskkit/briefschemagate.go`, `tools/desk/cmd/deskevidence/`, `spec/context-handoff-v1.md`, `spec/verification-domain-v1.md`, `changelog/milestone-support-09.md` (planned), `statusgen/testdata/milestone-support/09-mutations.json` (planned). New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: existing handoff/domain and ruling adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "existing handoff/domain and ruling adapters"
  contract: S-verification-domain
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

## Human decision

Before changing the governed boundary, present the concrete implementation/design through the existing decision mechanism. Confirm the preserved actor, subject, revocation and effect-admission rules. Options: accept the bounded implementation; request corrections; hold. No answer supplies no approval. Do not infer this ruling from the planning request.

Default if no answer: none — blocks until answered

## Task

1. Introduce version-aware edge adapters around existing context-handoff, verification-domain and human-gate readers. Resolve the new typed reference with workref, then pass existing numeric fullId plus an independently retained subject/version binding to legacy APIs. Do not globally broaden their existing strict grammar or reinterpret old digests.
2. Bind ruling-link board IDs and aliases to the same exact logical brief and approved subject using existing human identity, issue/link and scope checks. Refuse missing/changed/mismatched subjects even if an upper parser or capability check is bypassed. Agent-authored metadata cannot create a human ruling.
3. Preserve archived evidence, withdrawal context, source revisions, workflow/verification identities and immutable acceptance digests. Current modules contain no workflow store/controller: cover existing records/adapters now and document that future workflow owners must use the same bridge, without depending on or inventing that absent implementation.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedHandoffSubject$' ./contexthandoff/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedHandoffSubject ' "$milestone_verify_out")` | exit 0 and named PASS; old records retain their digests/meaning, new adapter binds exact revision and wrong-subject inputs fail even with upper checks bypassed. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRulingAliasAuthority$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestRulingAliasAuthority ' "$milestone_verify_out")` | exit 0 and named PASS; real ruling gate accepts only the same human/issue/work subject across aliases; copied ruling, wrong revision/root and bot actor fail. |
| 3 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedVerifySubject$' ./domain/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedVerifySubject ' "$milestone_verify_out")` | exit 0 and named PASS; normalized legacy identity alone cannot erase the requested revision; stale or substituted candidate evidence fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
