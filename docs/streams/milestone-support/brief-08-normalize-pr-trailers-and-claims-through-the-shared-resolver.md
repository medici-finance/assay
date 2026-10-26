---
brief: assay:assay:milestone-support:08
title: "Normalize PR trailers and claims through the shared resolver"
why: "Different spellings must not acquire two claims, attach a PR to the wrong work, or make an authoring PR look delivered."
wave: 3
depends: ["milestone-support/01", "milestone-support/02", "milestone-support/04"]
unblocks: ["milestone-support/09", "milestone-support/10", "milestone-support/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: bef0746f-0619-4b99-bf3a-fd53ce31a1d2
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes PR/claim-to-work binding; an alias collision or lost repository scope could associate a foreign owner or authorization with the wrong work."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "tools/desk/internal/deskkit/trailer.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/briefauthoring.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/claimstore.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/gitcore/claimref.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskpr/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskfile/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskclaim-ref/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/loopengine/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-09-preserve-workflow-subjects-and-human-rulings-across-aliases.md: follow-up milestone-support/09 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-11-project-milestone-state-into-stream-views-and-deskboard.md: follow-up milestone-support/11 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 08 — Normalize PR trailers and claims through the shared resolver

## Context

files: `tools/desk/internal/deskkit/trailer.go`, `tools/desk/internal/deskkit/briefauthoring.go`, `tools/desk/internal/deskkit/claimstore.go`, `tools/desk/internal/gitcore/claimref.go`, `tools/desk/cmd/deskpr/`, `tools/desk/cmd/deskfile/`, `tools/desk/cmd/deskclaim-ref/`, `tools/desk/internal/loopengine/`, `changelog/milestone-support-08.md` (planned), `statusgen/testdata/milestone-support/08-mutations.json` (planned). New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: desk-tools identity adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "desk-tools identity adapters"
  contract: S-claim
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

1. Route all brief linkage and logical claim-key normalization through workref, preserving cell/repository ownership and UUID. Retire parallel grammar fragments in PR trailers, work-item/claim aliases and issue-bound brief lookups; inventory every caller with an explicit disposition.
2. Read all specified legacy forms and new b- references with optional revision selectors. Persist the selected subject/revision separately from the logical claim key. Mixed aliases must join the same existing claim; conflicting revision requirements hold, never create a second worker or overwrite a live claim.
3. Retain each role's existing custody boundary. Preserve Brief versus Authors semantics, ownership/generation checks and existing issue/PR alias behavior. Keep the no-ambient-auth rule. Milestones are not executable brief claims or delivery trailers. Migration must not close, steal, release or reassign real claims.
4. Do not reimplement desk-tools/49 repair-obligation deduplication or desk-tools/45 queue classification. Extend the currently available identity adapters only; reconcile simultaneous edits with those owners at pickup and add an explicit dependency if a not-yet-delivered API is truly required.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedTrailerIdentityJoin$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedTrailerIdentityJoin ' "$milestone_verify_out")` | exit 0 and named PASS; old/new forms join the same work, Authors remains authoring, wrong root/kind/version refuses; stripping the prefix before validating kind fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedClaimAliases$' ./cmd/deskclaim-ref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedClaimAliases ' "$milestone_verify_out")` | exit 0 and named PASS; real claim adapter with fake transport treats aliases as one owner/key, refuses conflicting pinned subjects and preserves generation; a duplicate logical claim fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
