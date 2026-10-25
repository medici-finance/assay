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
version: 6
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
  - "statusgen/domain: fixed-here (shared pure records, assessment seam and offline failure controls)"
  - "spec/verification-domain-v1.md: fixed-here (shared record and adapter contract)"
  - "schemas/verification-domain-v1.json: fixed-here (wire shape consumed by the domain fixture tests)"
  - "docs/contracts.md: fixed-here (S-verification-domain owner and consumer boundary)"
  - "statusgen/contexthandoff/jsonschema_test.go: fixed-here (uses the shared statusgen/internal/testschema helper)"
  - "docs/streams/desktools-v2/cli-migration.json: fixed-here (exact source-test route for statusgen/domain/mutate.py; read by the unchanged tools/desk/internal/clicontract inventory, research-replay and routing-mutation checks)"
  - "workflow candidate record (spec/workflow-candidate-v1.md as planned by graph-execution/23): follow-up graph-execution/23"
  - "regression-floor adapters: follow-up graph-execution/29"
  - "statusgen/evaluation: out-of-scope (consumed unchanged through its exported API; statusgen/15 owns it)"
  - "durable coordination store (workflow/store): follow-up graph-execution/19 (non-CI R2 amendment, 2026-10-13; keys its records on this contract)"
  - "runtime controller: out-of-scope (the non-CI R5 follow-up consumes this contract; see the amendment manifest)"
id: fefddc8e-4cd2-4438-8c26-965b24ffd5dc
---

# Brief 28 — Verifier domain contracts with qualified identities and candidate-bound acceptance

## Context

files: `spec/verification-domain-v1.md`, `schemas/verification-domain-v1.json`,
`statusgen/domain/` (new pure package, tests and mutation runner), `statusgen/testdata/domain/`,
`statusgen/internal/testschema/schema.go`, `statusgen/contexthandoff/jsonschema_test.go`,
`docs/contracts.md` (one new semantic-owner row), `docs/lifecycle.md`,
`docs/streams/desktools-v2/cli-migration.json`, `changelog/k8s-verifier-domain-contracts.md`,
`tools/desk/cmd/deskwt/maintenancecustody_unix_test.go` (test-only CI repair).

facts (implementation notes):

CI shepherd repair (2026-10-20, medici-finance/assay#3736): the legacy crash fixture
publishes its complete child PID through a same-directory rename. A deterministic
test observes the create-before-write boundary; the existing crash reader, surviving-child
witness, custody marker and refusal assertions remain unchanged. This repair changes
no maintenance production code or verifier-domain semantics.

Implementation routing (2026-10-20): `statusgen/domain/mutate.py` is this brief's
source-only offline failure-control runner. `docs/streams/desktools-v2/cli-migration.json`
routes that exact path as a test with this brief as owner in the reason. The
`tools/desk/internal/clicontract` inventory, research-replay and routing-mutation checks
consume the registry; no broader exclusion or operator CLI migration is introduced.

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
Add `TestDomainStageSeparation` (planned) to the declared `statusgen/domain/` tests. It must distinguish
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

### Consumer routing and acceptance boundary

The immutable Verify section below retains the delivered acceptance definition.
The following clarification retains its normative force for independent review;
consumer ownership corroboration does not discharge required contract use.

The named tests are deliverables of this brief and do not exist at a435badbd. Each must run
production code with independent expected results; each +mutation row names a mutation that must
turn its test red.

For row 10, the independent verifier obtains the actual implementation delivery from the
merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or a branch tip. Both commits must be available in the verification
checkout; absent or unknown provenance fails closed. Record exact consumer result counts
and distinguish follow-up ownership corroboration from actual contract consumption.
Required candidate-workspace (/23), regression-adapter (/29) and durable-store (/19)
consumption stays held until separately proved, even when their owning briefs are
corroborated; zero disproofs alone is not complete flow coverage. A pre-merge PR-range
check can leave follow-up entries unchecked; delivery mode can corroborate their owning
briefs without proving consumption. PR-range results are local sanity only and are not
row-10 delivery evidence. The writer of this brief does not assert post-merge verification
here.

Row 10 consumer expectation:

exit 0 only for the named landed delivery and its actual first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; delivery-mode corroboration of follow-up graph-execution/23, /29 and /19 establishes owning-brief existence and routing only, not contract use; their required candidate-workspace, regression-adapter and durable-store consumption stays held until separately proved, regardless of corroborated ownership; the unchanged evaluator and deferred runtime-controller exclusions remain reviewer judgments, and omitted consumers remain a review obligation

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd. Each must run
production code with independent expected results; each +mutation row names a mutation that must
turn its test red.

For row 10, the independent verifier obtains the actual implementation delivery from the
merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or a branch tip. Both commits must be available in the verification
checkout; absent or unknown provenance fails closed. Record exact consumer result counts
and resolve unchecked required consumers separately; zero disproofs alone is not complete
flow coverage. A pre-merge PR-range check is local sanity only and is not row-10 delivery
evidence. The writer of this brief does not assert post-merge verification here.

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
| 10 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its actual first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required candidate-workspace, regression-adapter and durable-store consumers stay held; the unchanged evaluator and deferred runtime-controller exclusions remain reviewer judgments, and omitted consumers remain a review obligation |

## Pre-mortem

Likely wrong builds: identity from paths (row 1), unknown facts rounded to pass (row 2), a
branch-name binding (row 3), acceptance inferred from execution (row 4), stale results reused
(row 5), a second evaluator hidden in the package (row 6), effects creeping in (row 7), a
schema that drifts from the code (row 8), collapsed execution/acceptance/publication states
(row 9), and delivery provenance or consumer-routing failures (row 10). Row 10 corroborates
follow-up ownership, not actual contract use; downstream consumption and omitted consumers
remain independent proof and review obligations. Adequacy of the record shapes stays a
review judgment.

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->
### Independent verification — 2026-10-21

Runner: assay-verifier-app[bot], model gpt-6.1-sol, admitted tier strong; independent non-implementer. Observed source a62c2a851c52f19d3b7cfb3fa89687b4f354ce24, detached and unchanged except this Evidence section. Environment: Darwin arm64; go1.27.2; git 2.56.0; installed statusgen v1.0.53 and deskdispatch releaseTag v1.0.53/sourceSHA 74f79b7a; installed verifier skill 1.0.52. No runner image was invoked or qualified. KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local; temporary output remained outside the admitted source.

Verification-Attestation: medici-finance/assay#3827 run=1d1880e3dce9473f55557a256dfc3a069d2582ad7b3db5e5068c45592caeb0a7 source=a62c2a851c52f19d3b7cfb3fa89687b4f354ce24 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=gpt-6.1-sol tier=strong

The exact native pre-work admission check succeeded before any Verify result. The first attempt could not resolve api.github.com; the unchanged read-only command succeeded with network permission, preserving the same record, identity, source and home. Independent expectations were written from the brief and base-branch requirements before packet, implementation diff, PR discussion or implementation-test reads. The packet matched the admitted source and was read whole; its omitted Evidence/Review sections and three older brief commits were subsequently read at source. No packet result was counted as evidence.

Fresh native deskread change for medici-finance/assay#3728 returned merged=true, mergedAt 2026-10-20T17:16:08Z and mergeCommitSha 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf. Local immutable objects confirm actual first parent 0580e0a02df8dce18bd2d65359225e49d525bcd6 and the two parents' unique merge-base is that same commit. This is the implementation delivery, independently resolved from the forge. Both immutable values were exported as ASSAY_DELIVERY_SHA and ASSAY_DELIVERY_BASE_SHA in every row 10/witness shell.

Verification-Table: delivery=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf base=0580e0a02df8dce18bd2d65359225e49d525bcd6 original=0580e0a02df8dce18bd2d65359225e49d525bcd6

Native deskpathguard rederive against that immutable baseline exited 0: original rows 1–9 retain exact class/Command/Expect; row 10 is author-added. No original was replaced, filtered, renumbered or omitted. Row10 receives no original acceptance credit. There are nine original obligations and ten current rows.

Command preflight found rows 1–9 require check:ci network-off execution through Linux unshare. This host is Darwin, so those exact rows and their required mutation controls cannot run in the installed envelope. No alternate sandbox, container, selector or direct go-test run was substituted. The required native witness was executed as `statusgen verifyrun --root . --brief docs/streams/graph-execution/brief-28-verifier-domain-contracts.md --fail-first --base 0580e0a02df8dce18bd2d65359225e49d525bcd6 --dry-run` and exited 2. It printed the following witness; --dry-run did not append it. The captured table is preserved here with row 10's original native pass classification explicitly corrected to its independently rederived supplemental disposition. Native fail-first reports no risk-bearing row selected by its tag predicate; all base cells are not-selected. That statement does not establish any mutation kill or red-at-base evidence.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | author-added; execution exit=0 sandbox=none; no acceptance credit | sha256:dc7c78017bbe | 2026-10-21 | assay-verifier-app[bot] @ a62c2a851c52 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=a62c2a851c52 |

Row10's exact command also ran directly at the observed source, exited 0, and printed: `summary: 10 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. It checked the immutable delivered snapshot 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf. All 12 declared consumers are accounted for: seven fixed-here paths corroborated; /23,/29,/19 follow-up ownership corroborated; unchanged evaluation and deferred runtime-controller exclusions remain two unchecked reviewer judgments. Ownership corroboration does not show the three required downstream implementations import/use this contract. Those actual consumption obligations remain held. Workflow candidate spec and workflow/store are absent at this source; /29 remains planned in its owning brief. Zero disproofs is insufficient for complete flow coverage. Fresh issue read confirms medici-finance/assay#3731 remains the row 10 wording advisory; its older synthetic counts are not this run's counts.

Risk-value enumeration covered the actual 32-file delivered diff: 3029 literal occurrences in 31 literal-bearing files; the changelog contains prose only. The retained inventory includes every added quoted/raw/numeric/boolean/null literal as a superset, including fixtures and source-only test constants. Inventory SHA256 e6ccbc2979e3c6d9f01663d87ef2a2b9e59a28895d33c0fc2e6d37a2e3b21b57; diff SHA256 1fc489b0ac9ff9d076038d4642cb1374eb5239d84b6d058b35f4ce6f778f3a9a. Highest consequence is wrong subject/completeness/currentness support reaching a future acceptance; next are inherited identity/encoding/capability constraints; reversible fixture PID 4242, mode 0600, staging suffix and mutation timeouts 30s/60s rank last. No irreversible external act occurs in this pure contract delivery.

RISK-VALUE: DERIVED — Present = "present" @ statusgen/domain/records.go:171 — Task1/2 names present as the sole complete required-fact state; missing and could-not-check cannot release assessment.
RISK-VALUE: DERIVED — Missing = "missing" @ statusgen/domain/records.go:172 — Task1/2 requires explicit absence to hold, including an omitted required fact.
RISK-VALUE: DERIVED — CouldNotCheck = "could-not-check" @ statusgen/domain/records.go:173 — unavailable mandatory observations must remain uncertainty, never pass; the implementation returns before either evaluator while holds remain.
RISK-VALUE: DERIVED — eligibility-subject-count = 1 @ statusgen/domain/assess.go:101 — one qualified work item needs exactly one matching eligibility subject; zero is missing and more than one is ambiguous.
RISK-VALUE: DERIVED — coverage-subject-count = 1 @ statusgen/domain/assess.go:104 — assessment is bound to exactly one candidate/work acceptance, so an empty or multi-brief coverage envelope cannot identify the sole subject.
RISK-VALUE: DERIVED — completion-acceptance = "accepted" @ statusgen/domain/records.go:288 — migration contract allows Complete only after acceptance; execution success is a separate observation.
RISK-VALUE: DERIVED — completion-publication = "confirmed" @ statusgen/domain/records.go:288 — accepted/publication-pending is explicitly distinct from Complete; retirement may remain held separately.
RISK-VALUE: DERIVED — instanceRE = `^inst-[a-z0-9][a-z0-9-]{2,62}$` @ statusgen/domain/records.go:57 — this copies the existing normative workflow-instance-v1 schema instance.id grammar (schemas/workflow-instance-v1.json:36), preserving /09 ownership rather than selecting a new bound.
RISK-VALUE: DERIVED — ordinalRE = `^[1-9][0-9]*$` @ statusgen/domain/records.go:59 — workflow-instance-v1 section2 explicitly starts attempt ordinals at 1; zero and leading-zero aliases cannot be additional attempt identities.
RISK-VALUE: DERIVED — work-brief-pattern = `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[0-9]{2,4}[a-z]?$` @ schemas/verification-domain-v1.json:60 — existing context-handoff-v1 Identity section specifies four canonical segments and two-to-four digit optional-letter NN; contexthandoff/ids.go:22 is the same owner grammar.
RISK-VALUE: DERIVED — digestRE = `^sha256:[0-9a-f]{64}$` @ statusgen/domain/records.go:61 — SHA256 is 256 bits and each hexadecimal digit represents 4 bits, requiring exactly 64 digits; the tag identifies that algorithm without conveying authenticity.
RISK-VALUE: DERIVED — supported-capability = "domain@1" @ statusgen/domain/records.go:325 — the delivered contract is v1, and unsupported mandatory capabilities cannot be silently interpreted using another version's semantics.

Three fresh independently admitted contexts reviewed related row clusters 1–3,4/5/9 and 6–8 serially; each received only the brief and its own rows, never sibling results. Each returned could-not-run for every assigned row and separately inspected its production/test controls. Static inspection found the intended identity, fact, binding, stage, generation, evaluator and schema paths; it supplies no executable acceptance credit. Current fixture inventory is 18: six valid, six invalid, five semantic-invalid and one historical. The spec explicitly explains schema-shape checks plus semantic cross-field checks. The row 3 runner mutation compares repository only rather than the brief's conceptual branch-only comparison; no mutation kill is asserted.

RISK-VALUE: DERIVED — schema-additionalProperties = false @ schemas/verification-domain-v1.json:7 — unknown mandatory wire fields must not be silently accepted; the declared closed schema and normalized-record validator operate together.

Accounting: accepted PASS 0; observed failed obligations 0; original rows 1–9 could-not-run 9; current supplemental row 10 executed exit0 with no acceptance credit and incomplete downstream consumption proof. No mutation-control result or source-only test run is claimed. Required next capability is an admitted installed Linux runner supporting the exact network-off check:ci envelope, followed by fresh full original-command witnesses and mutation controls. Environment tracking remains medici-finance/assay#1800; downstream implementation holds remain with graph-execution/23,/29,/19. No lifecycle cell changed.

**VERIFY: BLOCKED** — nine original rows lack admissible execution evidence; supplemental row 10 does not clear required consumer holds.

## Review

Gate: model (all four risk answers no). The reviewer confirms that no second identity, evaluator
or candidate record was introduced, and that every refusal above has a failing control.
