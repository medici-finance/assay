---
brief: assay:assay:graph-execution:28
title: Verifier domain contracts with qualified identities and candidate-bound acceptance
why: "The planned verifier runtime, its database and its cockpit all need one answer to 'which work item, which candidate, which acceptance, which facts'. Without one pure contract they each invent their own, and a verdict can attach to the wrong candidate or pass on missing facts."
wave: 2
depends: ["graph-execution/09"]
unblocks: ["graph-execution/23", "graph-execution/29", "graph-execution/04", "graph-execution/16", "graph-execution/19", "graph-execution/26", "forge-neutral/37", "forge-neutral/38", "graph-execution/30", "graph-execution/31"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — package R1 (domain contracts and regression floor)
  - docs/proposals/non-ci-architecture/12-future-state-model.md
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — R1 owner row
  - "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
  - freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Cross-component contract (identity, acceptance binding, fact completeness) that the runtime, store and projection briefs all consume; a wrong meaning compounds.
domain: complicated
outcome: none
consumers:
  - "statusgen/domain, spec/verification-domain-v1.md, schemas/verification-domain-v1.json, docs/contracts.md: follow-up graph-execution/28"
  - "workflow candidate record (spec/workflow-candidate-v1.md as planned by graph-execution/23): follow-up graph-execution/23"
  - "regression-floor adapters: follow-up graph-execution/29"
  - "statusgen/evaluation: out-of-scope (consumed unchanged through its exported API; statusgen/15 owns it)"
  - "durable coordination store (workflow/store): follow-up graph-execution/19 (non-CI R2 amendment, 2026-10-13; keys its records on this contract)"
  - "runtime controller: out-of-scope (the non-CI R5 follow-up consumes this contract; see the amendment manifest)"
id: fefddc8e-4cd2-4438-8c26-965b24ffd5dc
---

# Brief 28 — Verifier domain contracts with qualified identities and candidate-bound acceptance

## Context

files: `spec/verification-domain-v1.md` (planned), `schemas/verification-domain-v1.json` (planned),
`statusgen/domain/` (planned) (new pure package and its tests), `statusgen/testdata/domain/` (planned),
`docs/contracts.md` (one new semantic-owner row), `docs/lifecycle.md`,
`changelog/graph-execution-28-verifier-domain-contracts.md` (planned).

facts (a435badbd, 2026-10-13):
- `statusgen/evaluation` exists on main (statusgen/15): `EvaluateEligibility` and
  `EvaluateCoverage` are pure functions over supplied observations; a value the caller cannot
  supply is could-not-check, never pass. `Identity` reports evaluator version plus policy digest.
- graph-execution/09 owns instance/node/attempt identity and pinned acceptance references
  (`spec/workflow-instance-v1.md` (planned)). This brief keys its records on those IDs and does
  not mint a second identity scheme.
- graph-execution/23 planned its own candidate record (`spec/workflow-candidate-v1.md` (planned)). The
  candidate/acceptance binding is defined once, here; graph-execution/23 consumes it (its
  2026-10-13 amendment).
- `docs/contracts.md` already names S-eligibility, S-evidence-coverage, S-identity,
  S-decision-acceptance and S-review-verdict. The new owner covers only the record shapes
  below and refers to those owners for their meanings.
- No `statusgen/domain` (planned) package, `verification-domain` spec or schema exists at a435badbd.

layering: the contract is data plus pure validation. Loading files, git observation, forge reads,
database rows and process effects stay in adapters outside `statusgen/domain` (planned). Verify 7 enforces
the import boundary; Verify 6 proves the seam with the existing evaluator.

single-point-of-failure: the candidate/acceptance binding check in `statusgen/domain` (planned) — behind it,
the existing evaluator's revision-bound coverage (statusgen/evaluation) independently refuses a
witness recorded at a different revision, and graph-execution/23's reviewer snapshot check refuses
a verdict whose subject hash differs.

design-fit:
  owner: statusgen/domain under a new S-verification-domain row in docs/contracts.md
  contract: verification-domain-v1 (spec + schema + fixtures, the three-part contract pattern)
  retires: [graph-execution/23 standalone candidate record definition]
  weight: "verbs 0, flags 0, refusals 0 at the CLI; one importable package"
  why-add: "R1 needs one shared record for the runtime, store and projection; the evaluator answers eligibility and coverage but has no candidate, fact-completeness or reevaluation-generation record."

## Read first

- `docs/proposals/non-ci-architecture/16-delivery-sequence.md` (R1 row and the operating
  obligations table: resource/retirement identities, reevaluation generations, guard events and
  distinct execution/acceptance/retirement states).
- `docs/proposals/non-ci-architecture/12-future-state-model.md`.
- `docs/contracts.md` (three-part contract pattern; existing owners above).
- `docs/streams/graph-execution/brief-09-instance-contract.md`,
  `docs/streams/graph-execution/brief-23-candidate-workspaces.md`.

## Ground rules

- Work in an isolated branch; draft PR only. No merge, release, deployment or live provider call.
- Offline synthetic fixtures only (example-org names). No adopter data.
- Stop at implemented; independent verification owns verified/done.
- Do not change `statusgen/evaluation` semantics. If the seam needs a new exported field there,
  report NEEDS_CONTEXT instead of editing it.
- This brief activates nothing: no runtime, store or publisher is wired to the contract.

## Verifier migration amendment — 2026-10-19

Consume [the migration contract](verifier-migration.md). Within the existing pure record
surface, represent component progress (bind/admit/execute/assess/decide/publish), execution
outcome, acceptance disposition, publication receipt state, typed hold and observation time
separately. These are projections of the existing semantic owners, not new authority or a
second queue classifier. Unsupported mandatory state is could-not-check. Existing /30
owns complete applicability context; this brief must not approximate it with file hashes.
Add `TestDomainStageSeparation` to the declared `statusgen/domain/` tests. It must distinguish
execution-success/acceptance-held, accepted/publication-pending and complete/retirement-held.


## Task

1. Write `spec/verification-domain-v1.md` (planned) and `schemas/verification-domain-v1.json` defining
   these records: qualified work identity (the four-segment `<cell>:<repo>:<stream>:<NN>` brief
   ID, with elided references expanded against the declaring brief, plus the
   graph-execution/09 instance/attempt IDs; a file path is never an identity); candidate
   (repository, base and candidate commit and tree, artifact manifest digest); acceptance
   binding (acceptance version and digest bound to exactly one candidate); fact set with an
   explicit completeness state per required fact (present, missing, could-not-check);
   reevaluation generation; and distinct execution, acceptance and retirement states.
2. Implement `statusgen/domain` (planned) as a pure package: constructors and validators for the records,
   canonical serialization, and an `Assess` function that maps a candidate, acceptance binding and
   fact set onto the existing `evaluation.EvaluateCoverage` / `evaluation.EvaluateEligibility`
   inputs and returns a typed result naming the evaluator `Identity` used. A missing or
   could-not-check required fact holds; it is never rounded to pass.
3. Add positive and negative fixtures under `statusgen/testdata/domain/` (planned): renamed path keeps
   identity, missing fact holds, acceptance bound to candidate A applied to candidate B is
   refused, execution success without acceptance stays unaccepted, stale reevaluation generation
   is refused. Validate every fixture against the JSON schema in a test.
4. Add the S-verification-domain row to `docs/contracts.md`, the lifecycle note to
   `docs/lifecycle.md` and the changelog fragment.

## Interface contract

Callers hand `statusgen/domain` (planned) normalized records; it returns validated records or a typed
refusal, and `Assess` returns a result carrying the evaluator identity. graph-execution/23 imports
the candidate and acceptance-binding types instead of defining its own. graph-execution/29 runs
the same fixtures through old and new adapters. Unsupported mandatory fields and stale generations
are refused, never ignored.

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd. Each must run
production code with independent expected results; each +mutation row names a mutation that must
turn its test red.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | exit 0; named PASS; a renamed brief file keeps its qualified identity and two files claiming one identity are refused; mutation: derive identity from the file path — the test must fail |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | exit 0; named PASS; missing and could-not-check required facts both hold; mutation: treat could-not-check as present — the test must fail |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | exit 0; named PASS; acceptance bound to candidate A is refused for candidate B with the same branch name; mutation: compare branch names instead of commit and tree — the test must fail |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | exit 0; named PASS; a succeeded execution with no acceptance record stays unaccepted, and retirement never implies acceptance; mutation: derive acceptance from execution success — the test must fail |
| 5 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | exit 0; named PASS; a result recorded at an older reevaluation generation is refused as current; mutation: skip the generation comparison — the test must fail |
| 6 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | exit 0; named PASS; `Assess` drives the real `statusgen/evaluation` functions on frozen fixtures, the result names `evaluation.CurrentIdentity()`, and a witness at a different revision holds; mutation: map the candidate commit into the evaluator in place of the tree revision — the test must fail |
| 7 | check:ci +mutation | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | exit 0; the package exists and imports no filesystem, process or network package; mutation: add an `os` import to the package — the row must fail |
| 8 | check:ci +dereference | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | exit 0; named PASS; every fixture under `statusgen/testdata/domain/` (planned) validates (positive) or is refused (negative) against `schemas/verification-domain-v1.json` (planned), and the spec names each schema record |
| 9 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | exit 0; named PASS; the three stage combinations remain distinct; mutation: map execution success directly to accepted — the test fails |

## Pre-mortem

Likely wrong builds: identity from paths (row 1), unknown facts rounded to pass (row 2), a
branch-name binding (row 3), acceptance inferred from execution (row 4), stale results reused
(row 5), a second evaluator hidden in the package (row 6), effects creeping in (row 7) and a
schema that drifts from the code (row 8). Adequacy of the record shapes stays a review judgment.

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->

## Review

Gate: model (all four risk answers no). The reviewer confirms that no second identity, evaluator
or candidate record was introduced, and that every refusal above has a failing control.
