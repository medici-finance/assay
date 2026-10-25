---
stream: milestone-support
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/milestone-support/README.md
---

# Milestone support across statusgen and desk-tools

**Version:** v1.0
**Status:** approved — approval of this implementation scope and stream activation take effect on the maintainer's merge of [PR #4182](https://github.com/medici-finance/assay/pull/4182), the ruling record under lifecycle §8.4.

**Routes-to:** `docs/streams/milestone-support/`

Implement the merged delivery-milestone and prefixed-reference contracts through shared
libraries and existing readers. With v1.0.57 published, this change proposes approval
of the implementation scope and activation together on the maintainer's merge of PR #4182.
That merge supplies neither reader capability nor brief completion evidence.
Existing independent human, review, verification and adoption gates remain in force.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Extract one shared typed work-reference resolver](brief-01-extract-one-shared-typed-work-reference-resolver.md) | 0 | M | implemented | — | — |
| 02 | [Negotiate reader capabilities through one shared contract](brief-02-negotiate-reader-capabilities-through-one-shared-contract.md) | 1 | M | todo | — | — |
| 03 | [Load and validate versioned milestone records](brief-03-load-and-validate-versioned-milestone-records.md) | 2 | M | todo | — | — |
| 04 | [Add negotiated brief schema and prefixed authoring](brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md) | 2 | M | todo | — | — |
| 05 | [Build the combined milestone and brief dependency graph](brief-05-build-the-combined-milestone-and-brief-dependency-graph.md) | 3 | M | todo | — | — |
| 06 | [Bind milestone acceptance to existing evidence and decisions](brief-06-bind-milestone-acceptance-to-existing-evidence-and-decisions.md) | 4 | M | todo | — | — |
| 07 | [Evaluate milestone readiness through the canonical eligibility API](brief-07-evaluate-milestone-readiness-through-the-canonical-eligibility-api.md) | 5 | M | todo | — | — |
| 08 | [Normalize PR trailers and claims through the shared resolver](brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md) | 3 | M | todo | — | — |
| 09 | [Preserve workflow subjects and human rulings across aliases](brief-09-preserve-workflow-subjects-and-human-rulings-across-aliases.md) | 4 | M | todo | — | — |
| 10 | [Enforce milestone capabilities at dispatch and pickup](brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md) | 6 | M | todo | — | — |
| 11 | [Project milestone state into stream views and deskboard](brief-11-project-milestone-state-into-stream-views-and-deskboard.md) | 6 | M | todo | — | — |
| 12 | [Preserve milestone scope and identity in history and exports](brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md) | 6 | M | todo | — | — |
| 13 | [Trace and report a selected milestone through canonical readers](brief-13-trace-and-report-a-selected-milestone-through-canonical-readers.md) | 7 | M | todo | — | — |
| 14 | [Qualify the two-binary rollout and prepare release adoption](brief-14-qualify-the-two-binary-rollout-and-prepare-release-adoption.md) | 8 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Scope and source admission

The [controlling implementation scope](#implementation-scope) and stream activation
are ratified together by the maintainer's merge of PR #4182. The existing root contracts
remain authoritative inputs; this change does not approve or alter them. Scope approval
is not implementation or acceptance. New IDs are not used operationally: these briefs use
supported brief-v2 fullIds and existing board/dependency forms until the readers ship.

## Critical path and verified head

Source baseline: main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, 22 October 2026.
The real unblocking head is /01: the existing QID and trailer readers are present,
the public shared package is absent, and the tools/desk module does not require the
statusgen module. /02 follows to settle and implement negotiated support before any
new record/generator/dispatch semantics. Import visibility and source builds from
both actual modules are /01's acceptance, not assumed from a package-main import.
No external workflow-controller, new service or cluster is a prerequisite for this head.

Dependency joins (`+` means all named prerequisites; equal M sizing is not a duration forecast):

```text
01 -> 02 -> 03 + 04 -> 05 -> 06
05 + 06 -> 07
01 + 02 + 04 -> 08
05 + 08 -> 09
07 + 08 + 09 -> 10
07 + 08 -> 11
07 + 09 -> 12
11 + 12 -> 13
10 + 13 -> 14
```

The /03 and /04 fork rejoins at /05. The /08 branch rejoins the combined graph at
/09 through /05, feeds /11 directly, and feeds /12 through /09. /10 joins eligibility,
claim linkage and subject/ruling adapters. The longest chains pass through /05, /06,
/07, /11 or /12, /13 and /14. Review, verification, source admission and actual release
publication can dominate elapsed lead time even where they are not new implementation
edges. No outcome is already accepted.

## Dependency waves

| Wave | Briefs | Delivery |
|---|---|---|
| 0 | [01](brief-01-extract-one-shared-typed-work-reference-resolver.md) | Extract one shared typed work-reference resolver |
| 1 | [02](brief-02-negotiate-reader-capabilities-through-one-shared-contract.md) | Negotiate reader capabilities through one shared contract |
| 2 | [03](brief-03-load-and-validate-versioned-milestone-records.md), [04](brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md) | Load and validate versioned milestone records; Add negotiated brief schema and prefixed authoring |
| 3 | [05](brief-05-build-the-combined-milestone-and-brief-dependency-graph.md), [08](brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md) | Build the combined milestone and brief dependency graph; Normalize PR trailers and claims through the shared resolver |
| 4 | [06](brief-06-bind-milestone-acceptance-to-existing-evidence-and-decisions.md), [09](brief-09-preserve-workflow-subjects-and-human-rulings-across-aliases.md) | Bind milestone acceptance to existing evidence and decisions; Preserve workflow subjects and human rulings across aliases |
| 5 | [07](brief-07-evaluate-milestone-readiness-through-the-canonical-eligibility-api.md) | Evaluate milestone readiness through the canonical eligibility API |
| 6 | [10](brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md), [11](brief-11-project-milestone-state-into-stream-views-and-deskboard.md), [12](brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md) | Enforce milestone capabilities at dispatch and pickup; Project milestone state into stream views and deskboard; Preserve milestone scope and identity in history and exports |
| 7 | [13](brief-13-trace-and-report-a-selected-milestone-through-canonical-readers.md) | Trace and report a selected milestone through canonical readers |
| 8 | [14](brief-14-qualify-the-two-binary-rollout-and-prepare-release-adoption.md) | Qualify the two-binary rollout and prepare release adoption |

## Proposed useful outcomes — authored scope, not live milestone records

These proposals live here until /02–/07 provide the declared support. Membership is
specified once in [the coverage section](#outcome-and-criterion-coverage); reverse views will
be generated by /11. Numbering implies no automatic prerequisite chain. A wave is
execution order; outcomes can overlap and span waves. No `milestones/` YAML or
milestone-start field is planted in the existing operational corpus.

## Existing owners and overlap dispositions

- `statusgen/15` owns the existing evaluation API: extend delivered code, preserve
  its independent verification/review history; do not author a second evaluator.
- `desk-tools/45` owns verification-queue classification; `desk-tools/49` owns repair
  filing deduplication. Those behaviors are out of scope here. /08–/11 adapt reference
  inputs/outputs at their existing seams, not replace their scheduling semantics.
- `graph-execution` owns the future workflow store/controller and existing runtime
  contracts. /09 covers current handoff/domain adapters; it does not implement or
  depend on a nonexistent workflow store. Future consumers inherit the public resolver.
- `docs/library-first.md` preserves the authorized online forge read boundary.
  The new libraries are offline/pure. No deskkit import, private credential transfer,
  daemon or wrapper binary is introduced.

## Authoring validation and remaining gates

All 14 briefs are M-sized, strong-tier work. Their named tests and new paths are
implementation deliverables, not reported PASS evidence. Authoring checks passed:
source-built statusgen read-only lint, whitespace checks, and a structural check of
all 14 identities, dependency/inverse edges, derived waves, source links and initial
statuses. The 32 named future implementation tests were not run. A title-based scan of all 43 open PRs at authoring
found no milestone/resolver/brief-v4 implementation proposal; this is bounded overlap
coverage, not proof no other worker owns adjacent code. Recheck active claims and
file-level PR overlap at pickup.

Scope approval and stream admission take effect on the maintainer's merge of PR #4182.
Remaining release/adoption acts: risk-gated design decisions,
independent verification, complete operation-specific reader qualification, actual
module/tool publication and adopter pin/activation decisions. No planning or fixture
run supplies these acts. Brief completion states and ready labels remain untouched.

## Implementation scope

This authored scope is part of the stream plan; it is not an implemented contract
or a deliverable of one of the new briefs. The merged root specs remain authoritative.

### Sources and observed baseline

Controlling contracts: [milestone-v1](../../../spec/milestone-v1.md),
[stream-v1](../../../spec/stream-v1.md), [brief-v1 §3.7–3.8](../../../spec/brief-v1.md),
[context handoff](../../../spec/context-handoff-v1.md), and the
[library-first direction](../../library-first.md), at main
`a631d47ea71b3270ab33ba08af47d3dde8c9e7d5` inspected 22 October 2026. #4164 merged the contract and skill guidance;
it did not ship runtime support. The graph alias registry identifies this cell/repo
as `assay:assay`. Public consumers preserve its aliases; missing private roots remain
unknown rather than leaking their locators or inventing mappings.

Code inspected: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go`,
`statusgen/briefschemaversions.go`, `statusgen/evaluation`, `statusgen/streamview`,
`statusgen/newbrief.go`, `tools/desk/internal/deskkit/trailer.go`, schema/admission,
claim/ruling entrypoints and both module manifests. Existing statusgen/schema support
is v1–v3. Separate parsers are an observed coupling mechanism; no complexity, runtime
performance or maintenance-cost reduction has been measured. This bounded refactor
consolidates precisely the behavior new syntax would otherwise duplicate.

## Architecture: direct public library calls

Place `workref` and `readercaps` as public, pure packages under the EXISTING statusgen
Go module (`github.com/medici-finance/assay/statusgen/workref` and `/readercaps`).
Both statusgen's adapters and desk-tools import and call their functions in-process.
Do not import the statusgen package main or tools/desk/internal across modules.
No separate resolver executable, subprocess call, daemon, top-level module, service,
credential broker, task queue or acceptance authority is proposed.

The library owns grammar, typed identity/revision resolution and deterministic
capability assessment. Explicit declaring context, supplied registry and source
observations enter; typed results leave. Host adapters own registry/source I/O,
existing strict-API compatibility and currentness/authority checks. Existing online
forge reads retain their authorized deskread boundary. Shared meaning does not remove
independent enforcement at a trust boundary.

This placement reuses the existing identity and graph semantic owner and keeps its
stdlib-only core importable by tools/desk. There is currently no reverse module
require from statusgen to tools/desk. /01 must prove both production imports under
GOWORK=off, including a temporary external consumer and dependency closure; code merely
existing in another module or a local replace is not proof of released compatibility.
/14 prepares candidate bundle/pin evidence; actual publication remains an operator act.

## Supported behavior and exclusions

- Canonical full references use `cell:repo:stream:b-NN` (optional exact authored
  `@brief-version` on references) and `cell:repo:stream:m-NN@contract-version`.
  Repo/stream and stream-only colon elisions bind actual declaring context.
- Read all contract-listed legacy forms as aliases of the SAME UUID/logical work.
  Determine type from the final segment only; malformed, wrong-kind, ambiguous and
  unsupported-required semantics cannot become ready. Exact unavailable revisions
  remain unknown; unversioned refs bind a selected source snapshot.
- Milestone contribution/completion edges and final-acceptance requirements are distinct
  from explicit brief start requirements. Waves describe brief dependency order only.
- Acceptance binds exact contract/source/subject, criteria and authorized existing
  evidence/decision mechanisms; owner metadata does not grant approval. Done work,
  accepted outcome and current availability remain distinct.
- No cellctl/Kubernetes implementation or deployment, workflow store/controller,
  broad verifier scheduling rewrite, corpus renaming, live claim mutation, tag cut,
  installed pin change or stream activation is included in these briefs.

## Outcome and criterion coverage

The following are PROPOSED milestone promises, kept as authored scope because the
runtime cannot read operational milestone declarations yet. IDs do not imply order.
Acceptance authority remains the existing design/review/independent verification and
release/adoption mechanisms; no acceptance is recorded here.

| Proposed outcome | Direct contributing briefs | Acceptance criteria |
|---|---|---|
| `assay:assay:milestone-support:m-01@1` — shared reference and reader contract | 01, 02 | C01: both actual modules call the same resolver, legacy profiles remain strict, all aliases/versions/kinds are deterministic. C02: shared operation-scoped capability assessment refuses unsupported required semantics before effects, without breaking unrequired legacy behavior. |
| `assay:assay:milestone-support:m-02@1` — readable, assessable milestone plans | 03, 04, 05, 06, 07 | C03: versioned loader/schema/generator validate owning roots and records. C04: completion/start graph edges, cycles and missing revision/root coverage are correct. C05: work complete without exact authorized acceptance remains outstanding; revoked/wrong-subject evidence cannot establish readiness. |
| `assay:assay:milestone-support:m-03@1` — safe desk consumption | 08, 09, 10, 11 | C06: aliases preserve one claim/work/evidence identity and exact human ruling subject. C07: actual dispatcher/pickup preflights shared capability and eligibility results with stale-input and lower-boundary negative controls; board views expose the same states. |
| `assay:assay:milestone-support:m-04@1` — trustworthy outcome reporting | 12, 13 | C08: aliases and membership changes preserve historical scope/evidence. C09: report uses canonical readers, keeps acceptance separate, shows six count lines with 7 daily + 3 preceding weekly period-end observations and unfinished prerequisites only. Missing observations remain unknown. |
| `assay:assay:milestone-support:m-05@1` — qualified candidate for release/adoption | 14 | C10: offline two-root pilot and actual candidate entrypoints prove compatible behavior and old/new refusal cases; module/artifact gaps and actual operator publication/adoption gates remain visible. |

Prerequisites are exactly the typed brief dependencies in the [README](README.md),
not extra implicit outcome chains. /02 and /06 settle the open capability envelope
and existing acceptance-receipt binding before their dependent implementation work.
No milestone record is treated as accepted merely because these briefs are authored
or later marked done. Human acceptance/release acts remain outside fixture evidence.

## Verification and handoff

Each brief has concrete named production-path tests, a named PASS guard and a planted
wrong-but-plausible mutation. Those tests are future deliverables. Independent Evidence
must record source/delivery revisions, actual commands and results, coverage, relevant
consumer/library versions and unavailable checks. A module import or green fixture
alone cannot prove the live supported outcome. Recheck current source, claims and
consumer overlap before pickup and preserve completed/in-flight work history.

The maintainer's merge of PR #4182 ratifies this scope and activates the stream.
It does not advance any brief lifecycle state or grant a PR-ready label, tag, installed
pin, release publication, root-contract approval or live runtime activation.
