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
version: 7
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

Acceptance-boundary amendment — 2026-10-23. This separates the existing pure contract
from required downstream integration; it grants no acceptance, activation or effect authority.

The boundary is approved through the acceptance-transition route, per the driver's ruling:
https://github.com/medici-finance/assay/issues/4452#issuecomment-6403449344

That ruling selected option A: local /28 acceptance is original rows 1–9, row 10 stays
present and uncredited, and the /19, /23 and /29 consumer-use holds attach to those briefs.

The existing independent Evidence PR #3921 owner recorded its checkpoint on the proposed
boundary and the preservation of historical Evidence (preserved: yes):
https://github.com/medici-finance/assay/pull/3921#issuecomment-6403147264

That checkpoint answers the request at
https://github.com/medici-finance/assay/pull/3921#issuecomment-6367743364 . It is a
readiness record, not acceptance of these semantics or merge permission; its reconciliation
conditions for the Evidence branch still apply. Independent review and the existing human
gates remain required.

The original acceptance definition is the nine rows at
0580e0a02df8dce18bd2d65359225e49d525bcd6. The implementation delivery is
3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf, whose actual first parent is that same
0580e0a02df8dce18bd2d65359225e49d525bcd6. PR #4174 restored the delivered table;
it is not a replacement implementation delivery. The entire Verify section and Evidence
below remain unchanged.

Local /28 acceptance requires every original row 1–9: qualified identity, required-fact
completeness, exact candidate/acceptance binding, separate execution/acceptance/retirement,
current reevaluation generation, the real evaluator seam, pure imports, schema fixtures
and separate component/publication states. Each behavioral, mutation, schema and flow
assertion remains owed with its exact immutable subject, qualified environment and
independent machine witness. Handwritten results or missing facts cannot pass.

Delivery-added row 10 remains a supplemental, uncredited consumer-routing disposition.
Run its unchanged command against the actual immutable delivery and first parent and
record exact corroborated/disproved/unchecked counts; do not mark it passed as an original
acceptance row. Owning-brief corroboration is not actual contract use. Its required
candidate-workspace, regression-adapter and durable-store consumption holds belong to
their consumer acceptance below, rather than being prerequisites of local /28 acceptance.

| Required actual use | Acceptance owner and existing checks |
|---|---|
| Old/new production adapters use the shared records and real assessment seam on identical frozen facts with independent expected results | /29 Task 3 and Verify 2; all six regression-floor rows and baseline/mutation controls remain mandatory |
| Durable qualified identity, candidate/acceptance/fact/generation records survive recovery and drive the real evaluator without accepting missing facts or a wrong subject | /19 Tasks 3–4 and Verify 1–2; all PostgreSQL, transaction/outbox, reconstruction and fencing rows remain mandatory |
| Frozen candidate and acceptance bindings reach independent reviewer input and survive valid restore while foreign, changed or incomplete inputs are refused | /23 Tasks 1–4 and Verify 1–3 |

These are required downstream integration holds until independently proved in their owning
deliveries; local /28 acceptance discharges none of them. /18 retains the connected
production journey and recovery proof. The unchanged evaluator and deferred runtime
controller exclusions remain reviewer judgments, and omitted consumers remain a review
obligation. No dependency edge, generated status, recorded result or existing activation
gate changes; /26–27 acquire no new /19 or /23 prerequisite from this amendment.

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

Row10's exact command also ran directly at the observed source, exited 0, and printed: `summary: 10 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. It checked the immutable delivered snapshot 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf. All 12 declared consumers are accounted for: seven fixed-here paths corroborated; /23,/29,/19 follow-up ownership corroborated; unchanged evaluation and deferred runtime-controller exclusions remain two unchecked reviewer judgments. Ownership corroboration does not show the three required downstream implementations import/use this contract. Those actual consumption obligations remain held. Workflow candidate spec and workflow/store are absent at this source; /29 remains planned in its owning brief. Zero disproofs is insufficient for complete flow coverage. Fresh issue read confirms medici-finance/assay#3731 remains the row 10 wording advisory; its older synthetic counts are not this run's counts. Disposition added 2026-10-26: these downstream actual-use obligations are deferred to graph-execution/23, graph-execution/29 and graph-execution/19 by the approved local-acceptance boundary (medici-finance/assay#4452); this does not discharge them.

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

### Independent verification — 2026-10-21 (run 2, source 1f0b54e7)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Host Darwin arm64, go1.27.2, git 2.56.0, installed statusgen v1.0.54. KUBECONFIG=/dev/null, GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local. All output and scratch copies stayed outside the admitted home; the home is detached at its source and unchanged except this Evidence section.

Verification-Attestation: medici-finance/assay#3910 run=13ba51e19f2e552ac9900b409e9127a4ba25b758e4a39c2c6bc1e6b3357cab87 source=1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong

The native pre-work admission check (deskdispatch --check-verifier) exited 0 before any Verify row ran. The packet recorded the same head commit; it was read whole as a reading aid only, and no packet line was used as an observation.

Delivery provenance (local immutable objects): 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf is "Merge pull request #3728" on the observed HEAD's first-parent chain, with first parent 0580e0a02df8dce18bd2d65359225e49d525bcd6 and second parent 0a2eced52a298ad074141d3af93c7f72d6f35843. The unique merge-base of the two parents is the first parent. No commit on the first-parent chain after the delivery touches statusgen/domain, the verification-domain schema or the spec.

Verification-Table: delivery=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf base=0580e0a02df8dce18bd2d65359225e49d525bcd6 original=0580e0a02df8dce18bd2d65359225e49d525bcd6

Original table: at the delivery base the Verify table has 9 rows. Rows 1–9 at the observed source are byte-identical to those rows (class, Command and Expect). Row 10 is author-added and receives no original acceptance credit. Note for landing: after delivery, 4025519bb reworded the Verify preamble and row 10's Expect, so the current Verify section is not byte-equal to the delivered section. Rows 1–9 are unchanged. The installed deskpathguard rederive (no base flag) reported 10 rows and 0 author-added against HEAD.

Execution envelope: the check:ci class requires network-off execution, and this Darwin host has no unshare. Rows 1–9 therefore ran in a local Linux container (golang 1.27.2 based image, linux/arm64) started with --network none; only loopback and a down tunnel interface were present. The worktree was mounted read-only and the host module cache read-only. The commands were extracted verbatim from the brief and each ran in a fresh bash at the repo root. Separately, statusgen verifyrun --sandbox container-netns --dry-run (built from the observed source, version dev) ran on a depth-1 copy of the same commit, which has the same tree as the home, inside the same --network none container. That witness reported rows 1–9 as "pass exit=0 sandbox=container-netns". Its row 10 exit 1 is an artifact of the copy: the delivery env vars were not set there and the depth-1 copy has no delivery objects. Row 10 is class check (not check:ci) and its result below comes from the host, which has the full object store.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | exit 0; named PASS; mutation (identity from path) turns it red | exit 0; printed "--- PASS: TestDomainPathIsNotIdentity (0.00s)"; mutation path-identity (key := item.Path + q.String()) KILLED with --- FAIL: TestDomainPathIsNotIdentity | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | exit 0; named PASS; mutation (could-not-check treated as present) turns it red | exit 0; printed "--- PASS: TestDomainMissingFactHolds (0.00s)"; mutation unknown-fact KILLED with --- FAIL: TestDomainMissingFactHolds | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | exit 0; named PASS; mutation (compare locator instead of commit and tree) turns it red | exit 0; printed "--- PASS: TestDomainAcceptanceBindsCandidate (0.00s)"; in-tree mutation candidate-binding (repository-only compare) KILLED; independent mutation comparing repository, base and manifest but not commit and tree: row exit 1, subtests commit and tree FAIL | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | exit 0; named PASS; mutation (acceptance from execution success) turns it red | exit 0; printed "--- PASS: TestDomainExecutionIsNotAcceptance (0.00s)"; mutation success-acceptance KILLED with --- FAIL: TestDomainExecutionIsNotAcceptance | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | exit 0; named PASS; mutation (skip generation comparison) turns it red | exit 0; printed "--- PASS: TestDomainReevaluationGeneration (0.00s)"; mutation generation (all three comparisons disabled) KILLED with --- FAIL: TestDomainReevaluationGeneration | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | exit 0; named PASS; real evaluator seam; mutation (commit in place of tree revision) turns it red | exit 0; printed "--- PASS: TestDomainEvaluationSeam (0.00s)"; mutation commit-for-tree KILLED with --- FAIL: TestDomainEvaluationSeam | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | exit 0; package exists with no effect imports; mutation (add os import) makes the row fail | exit 0, no output; independent mutation (planted package-domain file importing os, in a scratch copy) made the exact row exit 1; in-tree effect-import mutation also KILLED TestDomainPureImports | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | exit 0; named PASS; every fixture validates or is refused against the schema | exit 0; printed "--- PASS: TestDomainSchemaFixtures (0.01s)"; schema-open mutation (additionalProperties true) KILLED with --- FAIL: TestDomainSchemaFixtures | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | exit 0; named PASS; three stage combinations distinct; mutation (execution success to accepted) turns it red | exit 0 (grep -q, silent); verifyrun witness "pass exit=0 sandbox=container-netns"; mutation stage-conflation KILLED with --- FAIL: TestDomainStageSeparation | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 for the landed delivery and its first parent; exact counts; 0 disproved | author-added, no acceptance credit; host run with ASSAY_DELIVERY_SHA=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf and ASSAY_DELIVERY_BASE_SHA=0580e0a02df8dce18bd2d65359225e49d525bcd6: exit 0; "summary: 10 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing" | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Mutation controls: the in-tree offline runner (python3 -I statusgen/domain/mutate.py, network-off container) exited 0 and printed KILLED for all 11 mutations. It copies its inputs into its own temporary directory: path-identity, unknown-fact, candidate-binding, success-acceptance, generation, commit-for-tree, stage-conflation, effect-import, schema-open, schema-drift and canonical-order. Each failed its named test, and none was a build failure. Two further mutations were independent of the runner and were applied to a scratch copy in a fresh temporary dir. The first planted an os import, which made the exact row 7 command exit 1. The second dropped commit and tree from the candidate equality while keeping repository, base and manifest; the exact row 3 command exited 1, with commit and tree subtests FAIL. The brief's row 3 wording ("branch names") has no direct field in the record. The candidate carries no branch, and the test comments that the record cannot express that mutable locator. The independent mutation is the faithful translation: compare everything except the frozen commit and tree.

Row 10 consumer accounting: 12 declared consumers. Seven fixed-here entries are corroborated by the delivered diff. The /23, /29 and /19 follow-up entries are corroborated as owning-brief existence and routing only. That is not proof that those briefs consume the contract: their candidate-workspace, regression-adapter and durable-store consumption stays held with graph-execution/23, /29 and /19 until each is separately proved. The two out-of-scope entries are UNCHECKED, and remain reviewer judgments: unchanged statusgen/evaluation, and the runtime controller left to the R5 follow-up. Omitted consumers remain a review obligation. Disposition added 2026-10-26: these downstream actual-use obligations are deferred to graph-execution/23, graph-execution/29 and graph-execution/19 by the approved local-acceptance boundary (medici-finance/assay#4452); this does not discharge them.

Risk-bearing values. The trigger does not fire on the metadata: risk is present with all four answers no, and the item is not irreversible. Enumeration was still done over the delivered domain package, schema and mutation runner. Ranked by consequence, the top entries are completeness states, subject cardinality, completion conditions, identity grammars and digest shapes. Ranked last, as reversible knobs: mutation runner timeouts 30s and 60s @ statusgen/domain/mutate.py:71, and fixture constants.

RISK-VALUE: DERIVED — Present = "present" @ statusgen/domain/records.go:171 — the only complete required-fact state; Task 2 requires any other state to hold.
RISK-VALUE: DERIVED — Missing = "missing" @ statusgen/domain/records.go:172 — explicit absence must hold (Task 1/2); mutation unknown-fact shows the hold is load-bearing.
RISK-VALUE: DERIVED — CouldNotCheck = "could-not-check" @ statusgen/domain/records.go:173 — an observation that did not look is never a pass (evaluator contract, Task 2).
RISK-VALUE: DERIVED — eligibility subject count = 1 @ statusgen/domain/assess.go:101 — one qualified work item must match exactly one eligibility subject; 0 is missing, more than 1 is ambiguous.
RISK-VALUE: DERIVED — coverage briefs length = 1 @ statusgen/domain/assess.go:104 — the acceptance binding names exactly one candidate and work item, so coverage must name exactly that one subject.
RISK-VALUE: DERIVED — complete requires Acceptance "accepted" and Publication "confirmed" @ statusgen/domain/records.go:288 — migration amendment: accepted/publication-pending and complete/retirement-held must stay distinct from complete.
RISK-VALUE: DERIVED — instanceRE = `^inst-[a-z0-9][a-z0-9-]{2,62}$` @ statusgen/domain/records.go:57 — byte-identical to the graph-execution/09 owner grammar at schemas/workflow-instance-v1.json:36, so no second identity scheme exists.
RISK-VALUE: DERIVED — digestRE = `^sha256:[0-9a-f]{64}$` @ statusgen/domain/records.go:61 — SHA-256 is 256 bits = 64 hex digits; the tag names the algorithm.
RISK-VALUE: DERIVED — commit/tree pattern = `^([0-9a-f]{40}|[0-9a-f]{64})$` @ schemas/verification-domain-v1.json:94 — the full git object ID lengths for SHA-1 (160 bits = 40 hex digits) and SHA-256 (64 hex digits) object formats; no abbreviations, which could alias.
RISK-VALUE: DERIVED — supported capability = "domain@1" @ statusgen/domain/records.go:325 — the delivered contract is v1; an unsupported mandatory capability must refuse, not be interpreted.
RISK-VALUE: DERIVED — additionalProperties = false @ schemas/verification-domain-v1.json:7 — unknown mandatory fields must be refused, never ignored (Interface contract); mutation schema-open shows the control is load-bearing.

Accounting: original obligations 9, passed 9 with mutation controls killed; observed failed obligations 0. Supplemental row 10 executed exit 0 with 0 disproved and no acceptance credit; the downstream consumption holds above stay with their owning briefs. Executor envelope: a local, digest-unpinned golang image with --network none, plus a source-built verifyrun with container-netns. This is not the pinned harness image or a qualified installed Linux runner. If the gate requires that runner, this evidence routes back to the environment tracker, medici-finance/assay#1800.

**VERIFY: PASS** — 9/9 original rows pass network-off with mutation controls killed; supplemental row 10 exit 0, 10 corroborated / 0 disproved / 2 unchecked.
### Independent verification — 2026-10-25 (run 3, source c5d83e122, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main c5d83e12212768fbc9f92b2d66d1399ec5526ffb, detached home, unchanged except this Evidence section. This run is admitted by its own attestation (medici-finance/assay#4458), issued against brief version 7; the earlier runs' attestations predate that version and do not cover it. `deskdispatch --check-verifier` exited 0 before any row ran, and again at the bracket's open and close steps.

Acceptance boundary: per the driver ruling recorded in this brief's Context (https://github.com/medici-finance/assay/issues/4452#issuecomment-6403449344, option A), local acceptance is original rows 1–9. Row 10 is present, was run and witnessed, and receives no acceptance credit. Rows 1–9 at the observed source are byte-identical (class, Command, Expect) to rows 1–9 at the delivery base 0580e0a02df8dce18bd2d65359225e49d525bcd6 and at the delivery 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf. The whole Verify table at the observed source is byte-identical to the table at the delivery.

Runner class: local container, `--network none`. Rows 1–9 (check:ci) ran inside a closed admission bracket: `statusgen verifyrun --open-bracket` on the host, then `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9` inside the container, then `statusgen verifyrun --close-bracket` on the host, with no edit to the home between the steps. The image is the published verify-runner v1.0.60 image, run by digest (ghcr.io/medici-finance/verify-runner@sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3; linux/amd64, Debian 12, go1.27.2, its own installed statusgen v1.0.60, the same release as the host's), under the engine's default seccomp profile with no added privileges. `<home>` was bind-mounted read-only at its own path, the module cache (filled on the host from the observed source) read-only, the admission receipt read-only; the only writable mount was a new directory for the witness file. GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, KUBECONFIG=/dev/null. The container's admission step recomputed the mounted home's tree digest and matched the value the host recorded at the open step. Row 10 needs git history, so it ran on the host with `statusgen verifyrun --rows 10`, with ASSAY_DELIVERY_SHA=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf and ASSAY_DELIVERY_BASE_SHA=0580e0a02df8dce18bd2d65359225e49d525bcd6 (the actual first parent of the merge of #3728, an ancestor of the observed source). The two selections are disjoint and together cover the table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | pass exit=0 sandbox=container-netns | sha256:30640e44a49b | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | pass exit=0 sandbox=container-netns | sha256:9ea43021c397 | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | pass exit=0 sandbox=container-netns | sha256:762acb9363ef | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | pass exit=0 sandbox=container-netns | sha256:95d989fc9be8 | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | pass exit=0 sandbox=container-netns | sha256:17238e0eb24d | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | pass exit=0 sandbox=container-netns | sha256:2ba2a5fd8c3e | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:e1ecf4456a55 | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:e13c2aa24cd418b72b9939cd7ab753e7a3b111c16fb6515c16037a239c06ec60
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4458 run=237ac90d704d15f6b3d7d2f2a0b056b487587659b12ff7f3cd44e8d05b19fea1 source=c5d83e12212768fbc9f92b2d66d1399ec5526ffb brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=237ac90d704d15f6b3d7d2f2a0b056b487587659b12ff7f3cd44e8d05b19fea1 source=c5d83e12212768fbc9f92b2d66d1399ec5526ffb tree=sha256:e13c2aa24cd418b72b9939cd7ab753e7a3b111c16fb6515c16037a239c06ec60 witness=sha256:319fbed214b58b7a064d6ffbc8ab39b7d5a8a688178787f576cbf95249923037 opened=2026-10-25T22:06:50Z closed=2026-10-25T22:07:35Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | pass exit=0 | sha256:90eca4416afe | 2026-10-25 | assay-verifier-app[bot] @ c5d83e122127 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4458 run=237ac90d704d15f6b3d7d2f2a0b056b487587659b12ff7f3cd44e8d05b19fea1 source=c5d83e12212768fbc9f92b2d66d1399ec5526ffb brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong

Row 10 detail (supplemental, no acceptance credit): the same command, re-run once on the host at the same source, exited 0 with the same output digest and printed `consumers corroboration — base 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf (immutable delivered snapshot)` and `summary: 10 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. Seven fixed-here entries are corroborated by the delivered diff. The /23, /29 and /19 follow-up entries are corroborated as owning-brief routing only; that is not proof those briefs consume this contract, so the candidate-workspace, regression-adapter and durable-store consumption stays held with graph-execution/23, /29 and /19. The two out-of-scope entries (unchanged statusgen/evaluation; the deferred runtime controller) are UNCHECKED and remain reviewer judgments. Omitted consumers remain a review obligation. Disposition added 2026-10-26: these downstream actual-use obligations are deferred to graph-execution/23, graph-execution/29 and graph-execution/19 by the approved local-acceptance boundary (medici-finance/assay#4452); this does not discharge them.

Mutation controls (supplemental, no row of their own): the in-tree offline runner `python3 -I statusgen/domain/mutate.py`, run in a separate `--network none` container from the same image digest with `<home>` mounted read-only, printed `PASS unmutated snapshot` and then KILLED for all 11 mutations, exit 0: path-identity (row 1), unknown-fact (row 2), candidate-binding (row 3), success-acceptance (row 4), generation (row 5), commit-for-tree (row 6), stage-conflation (row 9), effect-import, schema-open (row 8), schema-drift and canonical-order. The runner counts a kill only when the named test reports FAIL and the build did not fail. Row 3's runner mutation compares the repository instead of the frozen candidate; the candidate record has no branch field, so that is the faithful form of "compare branch names instead of commit and tree". For row 7, an os import was planted in a scratch copy of the observed source, and the exact row 7 command run on that copy in the same container setup exited 1; on the unmutated source it exits 0 (witness above).

Post-delivery inputs: between the delivery and the observed source, statusgen/domain/mutate.py, statusgen/domain/mutate_test.py, statusgen/contexthandoff/ids.go and the new statusgen/workref package changed. The domain package's Go sources, the schema, the spec and the fixtures did not. The rows above ran against the observed source as it is.

Risk-bearing values. The trigger does not fire on the metadata: risk is present with all four answers no, and the item is not irreversible. Enumeration still covered the domain package, its schema and the mutation runner at the observed source. The top-ranked entries are the completeness states, subject cardinality, completion conditions, identity grammars and digest shapes. Ranked last, as reversible knobs: mutation-runner timeouts 30s and 60s @ statusgen/domain/mutate.py:96, and fixture constants.

RISK-VALUE: DERIVED — Present = "present" @ statusgen/domain/records.go:171 — the only complete required-fact state; Task 2 requires every other state to hold.
RISK-VALUE: DERIVED — Missing = "missing" @ statusgen/domain/records.go:172 — explicit absence must hold (Task 1/2); the unknown-fact mutation shows the hold is load-bearing.
RISK-VALUE: DERIVED — CouldNotCheck = "could-not-check" @ statusgen/domain/records.go:173 — an observation that did not look is never a pass (evaluator contract, Task 2).
RISK-VALUE: DERIVED — eligibility subject count = 1 @ statusgen/domain/assess.go:101 — one qualified work item must match exactly one eligibility subject; zero is missing and more than one is ambiguous.
RISK-VALUE: DERIVED — coverage briefs length = 1 @ statusgen/domain/assess.go:104 — the acceptance binding names exactly one candidate and work item, so coverage must name exactly that subject.
RISK-VALUE: DERIVED — complete requires Acceptance "accepted" and Publication "confirmed" @ statusgen/domain/records.go:288 — the migration amendment requires accepted/publication-pending and complete/retirement-held to stay distinct from complete.
RISK-VALUE: DERIVED — instanceRE = `^inst-[a-z0-9][a-z0-9-]{2,62}$` @ statusgen/domain/records.go:57 — byte-identical to the graph-execution/09 owner grammar at schemas/workflow-instance-v1.json:36, so no second identity scheme exists.
RISK-VALUE: DERIVED — ordinalRE = `^[1-9][0-9]*$` @ statusgen/domain/records.go:59 — attempt ordinals start at 1 in workflow-instance-v1; zero and leading-zero aliases cannot name additional attempts.
RISK-VALUE: DERIVED — digestRE = `^sha256:[0-9a-f]{64}$` @ statusgen/domain/records.go:61 — SHA-256 is 256 bits, which is 64 hex digits; the tag names the algorithm.
RISK-VALUE: DERIVED — commit/tree pattern = `^([0-9a-f]{40}|[0-9a-f]{64})$` @ schemas/verification-domain-v1.json:94 — the full git object ID lengths for the SHA-1 (40 hex digits) and SHA-256 (64 hex digits) object formats; abbreviations, which could alias, are refused.
RISK-VALUE: DERIVED — supported capability = "domain@1" @ statusgen/domain/records.go:325 — the delivered contract is v1; an unsupported mandatory capability must refuse, never be interpreted.
RISK-VALUE: DERIVED — additionalProperties = false @ schemas/verification-domain-v1.json:7 — unknown mandatory fields must be refused, never ignored (Interface contract); the schema-open mutation shows the control is load-bearing.

Superseded results: for rows 1–9 this bracketed witness supersedes the run-1 could-not-run rows and the hand-written run-2 table, which carried no machine witness. The earlier verdicts stay as history.

Accounting: original obligations 9; passed 9 (each with an execution witness inside the closed bracket and its mutation control killed); observed failed obligations 0; could-not-run 0. Supplemental row 10: executed exit 0, 10 corroborated / 0 disproved / 2 unchecked, no acceptance credit; the downstream consumption holds stay with graph-execution/23, /29 and /19.

**VERIFY: PASS** — original rows 1–9 pass with container-netns witnesses in a closed admission bracket, mutation controls killed; supplemental row 10 exit 0, uncredited.
### Run 4 — independent installed-v1.0.63 verification — 2026-10-26

Source: 8663d6ae05797bb259dd6af3535b15e8111c67b1, fetched merged main. The independently observed host and release-pinned network-off runner both report statusgen v1.0.63. Runner image: ghcr.io/medici-finance/verify-runner@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8. This run preserves every earlier Evidence block. Original rows 1–9 remain mandatory; supplemental row 10 receives no acceptance credit. The coordinate-only protected-record path is used without an acceptance transition. The prior installed-path qualification used a separate admission (#4532) and dry-run only; it supplies no acceptance credit to this run.

Verification-Table: delivery=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf base=0580e0a02df8dce18bd2d65359225e49d525bcd6 original=0580e0a02df8dce18bd2d65359225e49d525bcd6

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | author-added; execution exit=0; no acceptance credit | sha256:dc7c78017bbe | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4533 run=a4b43053394e5a840170b3dad515dfa7ee4262fd2dffa77a046531748d3055b7 source=8663d6ae05797bb259dd6af3535b15e8111c67b1 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=gpt-6.1-sol tier=strong



| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | pass exit=0 sandbox=container-netns | sha256:30640e44a49b | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | pass exit=0 sandbox=container-netns | sha256:9ea43021c397 | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | pass exit=0 sandbox=container-netns | sha256:762acb9363ef | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | pass exit=0 sandbox=container-netns | sha256:95d989fc9be8 | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | pass exit=0 sandbox=container-netns | sha256:17238e0eb24d | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | pass exit=0 sandbox=container-netns | sha256:2ba2a5fd8c3e | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:e1ecf4456a55 | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ 8663d6ae0579 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a5142d6f9fbf011bca58ebacf26a193eafe13b971683acd0975acf7696debbad
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4533 run=a4b43053394e5a840170b3dad515dfa7ee4262fd2dffa77a046531748d3055b7 source=8663d6ae05797bb259dd6af3535b15e8111c67b1 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=gpt-6.1-sol tier=strong
Admission-Bracket: run=a4b43053394e5a840170b3dad515dfa7ee4262fd2dffa77a046531748d3055b7 source=8663d6ae05797bb259dd6af3535b15e8111c67b1 tree=sha256:a5142d6f9fbf011bca58ebacf26a193eafe13b971683acd0975acf7696debbad witness=sha256:fafac26910a45343a27cddcbe34e58d6aab45f3dd284063f576091df3e227dce opened=2026-10-26T19:03:52Z closed=2026-10-26T19:04:47Z before=receipt after=check-verifier


Run accounting: 9 original obligations, 9 pass, 0 fail, 0 could-not-run. Supplemental row 10 executed on the host with exit 0 and the exact primary-output digest sha256:dc7c78017bbee5e9d684c4e1c67e2827a686c05ca7b3d7f279c6a636dc62314f. Its output is 10 corroborated, 0 disproved, 2 unchecked, 0 briefs claiming nothing; the two unchecked exclusions are reviewer judgments, not passing consumers. The candidate-workspace, regression-adapter and durable-store actual-use obligations remain deferred to graph-execution/23, graph-execution/29 and graph-execution/19, respectively. No downstream acceptance, deployment or runtime activation is claimed.

Independent failure controls: in the same release-pinned image, with network none, GOPROXY=off and GOTOOLCHAIN=local, python3 statusgen/domain/mutate.py exited 0: PASS unmutated snapshot; 11 of 11 mutations produced their named behavioral failure, with no build/setup failure accepted as a kill. The killed cases were path-identity, unknown-fact, candidate-binding, success-acceptance, generation, commit-for-tree, stage-conflation, effect-import, schema-open, schema-drift and canonical-order. These are the production test paths; no mutation touched the admitted home. The row-3 mutation changes the candidate comparison to repository-only, because the candidate record has no branch field; the wrong-subject assertion fails. No branch-only mutation is claimed.

Post-delivery input accounting (A4): compare immutable delivery 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf with this run's observed source, across every Context file/directory and the transitive contexthandoff, evaluation, workref, instance-schema and module inputs. Changed paths are:

- changelog/k8s-verifier-domain-contracts.md
- docs/contracts.md
- docs/streams/desktools-v2/cli-migration.json
- statusgen/contexthandoff/check.go
- statusgen/contexthandoff/decode.go
- statusgen/contexthandoff/discover.go
- statusgen/contexthandoff/doc.go
- statusgen/contexthandoff/fixtures_test.go
- statusgen/contexthandoff/ids.go
- statusgen/contexthandoff/ids_test.go
- statusgen/contexthandoff/patch.go
- statusgen/contexthandoff/plan.go
- statusgen/contexthandoff/product.go
- statusgen/contexthandoff/productbinding_test.go
- statusgen/contexthandoff/recheck.go
- statusgen/contexthandoff/records.go
- statusgen/contexthandoff/resolver_test.go
- statusgen/contexthandoff/rules.go
- statusgen/contexthandoff/snapshot.go
- statusgen/contexthandoff/withdrawn_test.go
- statusgen/domain/mutate.py
- statusgen/domain/mutate_test.py
- statusgen/evaluation/selectedclosure.go
- statusgen/evaluation/selectedclosure_test.go
- statusgen/go.mod
- statusgen/go.sum
- statusgen/workref/README.md
- statusgen/workref/boundary_test.go
- statusgen/workref/legacy.go
- statusgen/workref/trailers/trailers.go
- statusgen/workref/trailers/trailers_test.go
- statusgen/workref/workref.go
- statusgen/workref/workref_test.go

No other path in that declared/transitive comparison changed. In particular, the production domain Go files, domain JSON fixtures, verification-domain spec/schema, schema helper, lifecycle note, named maintenance fixture and changelog fragment are unchanged. The newly observed shared workref resolver, all changed contexthandoff files and evaluation selection code were present for this run; the real evaluator seam, schema and all original behavior rows ran afresh at the observed source. The current mutation runner copied the real shared-identity dependency into its temporary snapshot and passed its baseline before planting any mutation. This is not the earlier incomplete claim that only contexthandoff/ids.go changed.

Risk-value inventory: independently enumerated all 32 files of the actual delivered diff (base 0580e0a02df8dce18bd2d65359225e49d525bcd6 to delivery 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf), retaining every nonblank added source line as a conservative superset, including fixtures and source-only test constants. This complete line inventory has 3113 entries (including lines without literals); its serialized digest is sha256:4ce1db2e8b48827c594258810a7247622ce8998ffccea1c701b8ccf9a6e70663. The raw delivered diff digest is sha256:1fc489b0ac9ff9d076038d4642cb1374eb5239d84b6d058b35f4ce6f778f3a9a. This enumeration includes schema strings, state vocabularies, refusal labels, regex bounds, fixture identities and numeric/boolean/null literals; naming the top values below is not a claim the other literals were absent. The brief is all-risk-no and irreversible-no; every value is reversible by an edit and redeploy. Highest consequence is wrong subject or incomplete facts supporting acceptance; encoding/capability and state-separation constraints follow; fixture PID 4242, file mode 0600 and source-only mutation timeouts 30s/60s rank last as reversible test knobs.

RISK-VALUE: DERIVED — Present = "present", Missing = "missing", CouldNotCheck = "could-not-check" @ statusgen/domain/records.go:171-173 — Task 1 fixes these three required-fact states. Task 2 permits assessment only with complete supplied facts; assess.go:77 maps every non-Present state to an explicit FactHold before invoking either evaluator. Rows 2 and 6 and their mutations discriminate unknown facts and wrong revision.
RISK-VALUE: DERIVED — eligibility subject count = 1 and coverage brief count = 1 @ statusgen/domain/assess.go:101,104 — Task 2 binds one assessment to exactly one qualified subject and candidate; zero or multiple subjects cannot establish that claim, so both bounds refuse before evaluation.
RISK-VALUE: DERIVED — objectRE = ^([0-9a-f]{40}|[0-9a-f]{64})$ @ statusgen/domain/records.go:60 and schemas/verification-domain-v1.json:94 — a full Git SHA-1 object is 20 bytes/40 hex digits; SHA-256 is 32 bytes/64 hex digits. The record requires immutable objects, not abbreviations, symbolic refs or branches, and records.go:122 requires the candidate's four objects to use one format.
RISK-VALUE: DERIVED — digestRE = ^sha256:[0-9a-f]{64}$ @ statusgen/domain/records.go:61 — Task 1 pins a manifest and acceptance digest, which the contract names as SHA-256. The algorithm label plus 32-byte hex representation makes unsupported forms explicit rather than guessing.
RISK-VALUE: DERIVED — instanceRE = ^inst-[a-z0-9][a-z0-9-]{2,62}$ @ statusgen/domain/records.go:57 — Task 1 consumes graph-execution/09's existing instance grammar, matching schemas/workflow-instance-v1.json. It is compatibility with the owner, not a newly justified global size limit.
RISK-VALUE: DERIVED — schema additionalProperties = false @ schemas/verification-domain-v1.json:7 — Task 1 requires normalized supported record shapes; unsupported mandatory content must refuse. JSON-schema shape and domain cross-field validation are both needed, as the schema description explicitly states; row 8 and schema-open/schema-drift controls exercise that boundary.

The new witness supersedes the execution and closure limitations of earlier runs for local /28 acceptance. Historical execution rows and observation text are preserved; three consumer-accounting prose lines now carry explicit dated deferral annotations to their already-approved owners, so an old local hold is not mistaken for an unrecorded local acceptance obligation. This run supplies no acceptance credit for the supplemental row and closes none of the separately owned consumer obligations.

**VERIFY: PASS** — all 9 original obligations have fresh execution witnesses at 8663d6ae05797bb259dd6af3535b15e8111c67b1 in the pinned v1.0.63 network-off runner, 11/11 mutation controls killed, and supplemental row 10 is machine-recorded author-added with no acceptance credit. The admission bracket is closed with a matching digest. Protected-table closure and landing checks are performed against this exact assembled Evidence.
**Run 4 closure disposition — 2026-10-26.** The original execution accounting remains 9 pass, 0 fail, 0 could-not-run, plus supplemental row 10 with no acceptance credit; all 11 mutation controls were killed. The protected-table witness audit reports those exact counts and the run-4 bracket is credited. The independent installed-v1.0.63 execution therefore observed no failed original domain-contract obligation.

The canonical implemented-to-verified preflight nevertheless refuses the inherited RISK-VALUE definition below as a live unresolved hold. The exact transition was attempted at Evidence head 3f71c626d237e48a073a1605f7e8e83935f40f19:

```text
statusgen --version
v1.0.63
deskevidence flip --brief graph-execution/28 --sha 8663d6ae05797bb259dd6af3535b15e8111c67b1 --dry-run
exit=5
refusing: brief graph-execution/28 carries **VERIFY: PASS** but Evidence also reads "RISK-VALUE: DERIVED — Present = \"present\" @ statusgen/domain/records.go:171 — Task1/2 names present as the sole complete required-fact state; missing and could-not-check cannot release assessment." on a row not marked deferred — a PASS marker is not a flip signal while a non-deferred row still says HELD/could-not-check
```

The quoted line defines a completeness state; it records no unexecuted row. The verifier did not rewrite that historical semantic explanation to evade the guard. This new disposition supersedes run 4's earlier closing PASS marker, while preserving every observed row result, witness and mutation result. The board remains implemented. No verified outcome, done transition, ready flip or merge is claimed.

The closure-classification defect is filed as medici-finance/assay#4536 for intake and the statusgen owner. PR medici-finance/assay#4150 was checked and concerns attribution/NOTICE handling, not this scan. After the reviewed repair is released and adopted, fresh independent qualification and the ordinary closure gates remain required. The separately approved downstream /19, /23 and /29 integration deferrals remain unchanged.

**VERIFY: BLOCKED** — original rows 1–9 passed and supplemental row 10 is uncredited; native lifecycle closure is refused by the RISK-VALUE/hold-scan classification recorded on medici-finance/assay#4536. No acceptance obligation was observed to fail, and no lifecycle cell was advanced.
### Run 5 — independent installed-v1.0.63 verification — 2026-10-27

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main e25479a69d69ad678c15253309fcac6b7b0c86b0, detached home, unchanged except this Evidence section. Admission: `deskdispatch --check-verifier` exited 0 before any row ran (attestation #4589), and again at the bracket's open and close steps. Host: Darwin arm64, go1.27.2, git 2.56.0, installed statusgen v1.0.63. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8 (statusgen v1.0.63), linux/amd64, network none, GOPROXY=off, GOTOOLCHAIN=local, home mounted read-only. KUBECONFIG=/dev/null; scratch output stayed outside the home.

Delivery provenance, resolved from the forge this run: PR #3728 merged=true at 2026-10-20T17:16:08Z with merge commit 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf; its actual first parent is 0580e0a02df8dce18bd2d65359225e49d525bcd6 and the delivery is an ancestor of the observed source. Both values were exported as ASSAY_DELIVERY_SHA and ASSAY_DELIVERY_BASE_SHA for row 10. `deskpathguard rederive` exited 0 with 10 rows and 0 author-added against the current table; the protected record below pins the original table at the delivery base. Acceptance boundary per the driver ruling in this brief's Context (option A): original rows 1–9 are the local acceptance obligations; row 10 is supplemental with no acceptance credit.

Verification-Table: delivery=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf base=0580e0a02df8dce18bd2d65359225e49d525bcd6 original=0580e0a02df8dce18bd2d65359225e49d525bcd6

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | author-added; execution exit=0; no acceptance credit | sha256:dc7c78017bbe | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4589 run=56423fba222ddeebf76569a4505ae958212663343e742add547b5dd6a8a76343 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | pass exit=0 sandbox=container-netns | sha256:30640e44a49b | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | pass exit=0 sandbox=container-netns | sha256:9ea43021c397 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | pass exit=0 sandbox=container-netns | sha256:762acb9363ef | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | pass exit=0 sandbox=container-netns | sha256:95d989fc9be8 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | pass exit=0 sandbox=container-netns | sha256:17238e0eb24d | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | pass exit=0 sandbox=container-netns | sha256:2ba2a5fd8c3e | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:e1ecf4456a55 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4589 run=56423fba222ddeebf76569a4505ae958212663343e742add547b5dd6a8a76343 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=56423fba222ddeebf76569a4505ae958212663343e742add547b5dd6a8a76343 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 tree=sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d witness=sha256:1186f2db3736eb1e9800564be0a2882bd8e2228fc073bbee3ae9ebfbb34aa3df opened=2026-10-27T02:10:58Z closed=2026-10-27T02:11:53Z before=receipt after=check-verifier

Run accounting: 9 original obligations, 9 pass, 0 fail, 0 could-not-run. Rows 1–9 ran inside the closed admission bracket above; `statusgen verifyrun --bracket-digest` on the witness file printed verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. Supplemental row 10 ran on the host through the protected record (`--acceptance-record`, `--rows 10`), exit 0, witness output sha256:dc7c78017bbe; its primary output reads 10 corroborated, 0 disproved, 2 unchecked, 0 briefs claiming nothing. The two unchecked entries (statusgen/evaluation, runtime controller) are out-of-scope exclusions left to reviewer judgment, not passing consumers. The candidate-workspace, regression-adapter and durable-store actual-use obligations stay with graph-execution/23, graph-execution/29 and graph-execution/19 per the approved boundary; this run claims none of them.

Supplemental failure controls (no acceptance credit, no row of their own): in the same pinned image with network none, `python3 -I statusgen/domain/mutate.py` exited 0, printing PASS unmutated snapshot and then KILLED for all 11 mutations, each by its named failing test: path-identity (row 1), unknown-fact (row 2), candidate-binding (row 3), success-acceptance (row 4), generation (row 5), commit-for-tree (row 6), stage-conflation (row 9), effect-import, schema-open (row 8), schema-drift and canonical-order. Row 7's own mutation was also probed directly: in a scratch copy of statusgen inside the container, a file adding an os import to package domain made the unchanged row-7 command exit 1. The home was not modified by either probe.

Input accounting: between run 4's source 8663d6ae05797bb259dd6af3535b15e8111c67b1 and this source, the only changed path among the Context files and the transitive contexthandoff, evaluation, workref, instance-schema and module inputs is docs/streams/desktools-v2/cli-migration.json, in an unrelated deskpr entry (parser and state fields); the statusgen/domain/mutate.py test route this brief relies on is unchanged. The production domain package, fixtures, spec, schema and evaluator are byte-identical to run 4's source.

Risk-value inventory: the brief is all-risk-no and irreversible-no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row needs a fail-first base witness. Enumerated over the delivered diff 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf (32 files) and re-read at the observed source: completeness vocabulary, subject-count bounds, object and digest grammars, the instance and ordinal grammars, the capability token, the completion predicate and the closed schema. Every value is reversible by an edit and redeploy. Ranked by consequence: wrong-subject or incomplete-fact acceptance first, then encoding and capability, then stage separation; fixture constants and the mutation runner's timeouts rank last as test knobs.

RISK-VALUE: DERIVED — `Present = "present"` @ statusgen/domain/records.go:171 — the only complete required-fact state; assess.go:77 routes every other state into a FactHold before either evaluator is called, which is what Task 2 requires.
RISK-VALUE: DERIVED — `Missing = "missing"` and `CouldNotCheck = "could-not-check"` @ statusgen/domain/records.go:172-173 — Task 1 names exactly these two non-complete states; the unknown-fact mutation proves treating either as present is caught.
RISK-VALUE: DERIVED — `count != 1` and `len(o.Coverage.Briefs) != 1` @ statusgen/domain/assess.go:101,104 — one assessment binds one qualified subject and one candidate; zero is absent and more than one is ambiguous, so both refuse before evaluation.
RISK-VALUE: DERIVED — `objectRE = ^([0-9a-f]{40}|[0-9a-f]{64})$` @ statusgen/domain/records.go:60 and schemas/verification-domain-v1.json:94 — full SHA-1 (20 bytes, 40 hex) and SHA-256 (32 bytes, 64 hex) git object IDs; abbreviations and refs could alias, and records.go:122 requires all four candidate objects to share one format.
RISK-VALUE: DERIVED — `digestRE = ^sha256:[0-9a-f]{64}$` @ statusgen/domain/records.go:61 — SHA-256 is 32 bytes, 64 hex digits; the algorithm label makes any other form an explicit refusal.
RISK-VALUE: DERIVED — `instanceRE = ^inst-[a-z0-9][a-z0-9-]{2,62}$` @ statusgen/domain/records.go:57 — byte-identical to the graph-execution/09 owner grammar at schemas/workflow-instance-v1.json:36, so no second identity scheme exists.
RISK-VALUE: DERIVED — `ordinalRE = ^[1-9][0-9]*$` @ statusgen/domain/records.go:59 — attempt ordinals start at 1; zero and leading-zero aliases cannot name extra attempts.
RISK-VALUE: DERIVED — `v != "domain@1"` @ statusgen/domain/records.go:325 — the delivered contract is v1; an unsupported mandatory capability refuses as unknown rather than being interpreted (Interface contract).
RISK-VALUE: DERIVED — `Stage == "complete"` requires `Acceptance == "accepted"` and `Publication == "confirmed"` @ statusgen/domain/records.go:288 — the migration amendment requires accepted/publication-pending to stay distinct from complete; stage-conflation mutation is killed.
RISK-VALUE: DERIVED — `"additionalProperties": false` @ schemas/verification-domain-v1.json:7 — unknown mandatory fields must refuse, never be ignored (Interface contract); the schema-open mutation is killed.

**VERIFY: PASS** — original rows 1–9 pass at e25479a69d69ad678c15253309fcac6b7b0c86b0 inside a closed container-netns admission bracket (verdict=match), 11/11 mutation controls killed; supplemental row 10 exit 0 with 10 corroborated / 0 disproved / 2 unchecked and no acceptance credit.
**Run 5 closure disposition — 2026-10-27.** The run-5 execution accounting stands: original rows 1–9 pass inside the closed admission bracket above, 11/11 mutation controls killed, and supplemental row 10 carries no acceptance credit. No acceptance obligation was observed to fail.

The canonical implemented-to-verified gate still refuses this Evidence, for the same reason run 4 recorded. Re-run at this branch's run-5 Evidence commit db7fade0d07c3cfc5e96da77219b490509184150:

```text
$ statusgen verifyflip --root . --brief graph-execution/28 --sha e25479a69d69ad678c15253309fcac6b7b0c86b0 --runner 'assay-verifier-app[bot]' --dry-run
statusgen verifyflip: graph-execution/28: refusing: brief graph-execution/28 carries **VERIFY: PASS** but Evidence also reads "RISK-VALUE: DERIVED — Present = \"present\" @ statusgen/domain/records.go:171 — Task1/2 names present as the sole complete required-fact state; missing and could-not-check cannot release assessment." on a row not marked deferred — a PASS marker is not a flip signal while a non-deferred row still says HELD/could-not-check
exit=1
```

The run-5 lifecycle change on this branch was a row-scoped landing, not the flip verb, so it did not pass through this gate. It is withdrawn: the board row for /28 is restored to implemented, byte-identical to main. The verified outcome record committed earlier on this branch (`28-20261027T021153Z-e0a97380f826.json`) is superseded by a later blocked record. Outcome records are append-only, so the earlier file stays, and the later record governs. No verified, done, ready or merge transition is claimed.

The refused line is the inherited RISK-VALUE definition that names the completeness states. It records no unexecuted row, and this verifier does not rewrite it to get past the gate. The closure-classification defect is tracked on medici-finance/assay#4536, which is still open. After that repair is released and adopted, or a maintainer rules on how that definition line is handled, fresh independent qualification and the ordinary closure gates remain required.

**VERIFY: BLOCKED** — original rows 1–9 pass and supplemental row 10 is uncredited; native lifecycle closure is refused by the RISK-VALUE/hold-scan classification tracked on medici-finance/assay#4536. No acceptance obligation was observed to fail, and the board row stays implemented.
### Run 6 — independent installed-v1.0.64 re-verification — 2026-10-28

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1, detached home, unchanged except this Evidence section. Admission: `deskdispatch --check-verifier` exited 0 before any row ran (attestation #4773), and again at the bracket's open and close steps. Host: Darwin arm64, go1.27.2, git 2.56.0, installed statusgen v1.0.64. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner:v1.0.64@sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0, the value of the v1.0.64 release asset verify-runner-image.txt, whose sha256 matches that release's checksums.txt; linux/amd64, network none, GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, home mounted read-only. KUBECONFIG=/dev/null; scratch output stayed outside the home.

Why this run: run 5's closure was refused by the verified-flip hold scan reading an inherited RISK-VALUE definition line as an unresolved hold. That classification defect, medici-finance/assay#4536, is closed (completed 2026-10-27). This run re-executes every row at current main under the installed v1.0.64 tools and re-checks closure.

Delivery provenance, resolved from the forge this run: PR #3728 state MERGED at 2026-10-20T17:16:08Z with merge commit 3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf; local objects give first parent 0580e0a02df8dce18bd2d65359225e49d525bcd6 and second parent 0a2eced52a298ad074141d3af93c7f72d6f35843, and the delivery is an ancestor of the observed source. Both values were exported as ASSAY_DELIVERY_SHA and ASSAY_DELIVERY_BASE_SHA for row 10. `deskpathguard rederive` exited 0 with 10 rows and 0 author-added against the current table. Rows 1–9 are byte-identical at the delivery base, the delivery and the observed source; the delivery base table has exactly those 9 rows, and the whole 10-row table is byte-identical at the delivery and the observed source. Acceptance boundary per the driver ruling in this brief's Context (option A): original rows 1–9 are the local acceptance obligations; row 10 is supplemental with no acceptance credit.

Verification-Table: delivery=3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf base=0580e0a02df8dce18bd2d65359225e49d525bcd6 original=0580e0a02df8dce18bd2d65359225e49d525bcd6

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief graph-execution/28 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | author-added; execution exit=0; no acceptance credit | sha256:0bbf2befc10f | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4773 run=c0ad26b395e522e2fb67dfacbb866da4d527edb6f5e26bce3641bdb93b3498f9 source=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainPathIsNotIdentity$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainPathIsNotIdentity " "$out")` | pass exit=0 sandbox=container-netns | sha256:30640e44a49b | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainMissingFactHolds$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainMissingFactHolds " "$out")` | pass exit=0 sandbox=container-netns | sha256:9ea43021c397 | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainAcceptanceBindsCandidate$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainAcceptanceBindsCandidate " "$out")` | pass exit=0 sandbox=container-netns | sha256:59d96a074195 | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainExecutionIsNotAcceptance$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainExecutionIsNotAcceptance " "$out")` | pass exit=0 sandbox=container-netns | sha256:95d989fc9be8 | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainReevaluationGeneration$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainReevaluationGeneration " "$out")` | pass exit=0 sandbox=container-netns | sha256:17238e0eb24d | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainEvaluationSeam$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainEvaluationSeam " "$out")` | pass exit=0 sandbox=container-netns | sha256:2ba2a5fd8c3e | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./domain/) && test "$name" = domain && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./domain/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainSchemaFixtures$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDomainSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:d4b9a79dff2f | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDomainStageSeparation$" ./domain/ > "$out" && grep -Fq -- "--- PASS: TestDomainStageSeparation " "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 8db7d356334e (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:343e3f0afab4d279155c1a0154dfaaf4ca15b797bc40653fa0b88034f646bd16
Container-Image: sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0
Verification-Attestation: medici-finance/assay#4773 run=c0ad26b395e522e2fb67dfacbb866da4d527edb6f5e26bce3641bdb93b3498f9 source=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 brief=docs/streams/graph-execution/brief-28-verifier-domain-contracts.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c0ad26b395e522e2fb67dfacbb866da4d527edb6f5e26bce3641bdb93b3498f9 source=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 tree=sha256:343e3f0afab4d279155c1a0154dfaaf4ca15b797bc40653fa0b88034f646bd16 witness=sha256:e480c58eea27606c8447e09e3e8198cc4774f996282a7f7902770ae7b5ddc888 opened=2026-10-28T18:21:50Z closed=2026-10-28T18:23:28Z before=receipt after=check-verifier

Run accounting: 9 original obligations, 9 pass, 0 fail, 0 could-not-run. Rows 1–9 ran inside the closed admission bracket above; `statusgen verifyrun --bracket-digest` on the witness file printed verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1 and exited 0. A first container attempt in this run refused before any row executed, because the container engine parsed the short `-v <home>:<home>:ro` form into a different mount target, so the mounted root digested to the empty tree; the same step re-run with an explicit read-only `--mount` of the home at its own path, against the same unchanged receipt and home, is the witness above. No row result came from the refused attempt. Rows 3 and 8 carry different output digests from run 5 because their primary output line includes the test's elapsed time. Supplemental row 10 ran on the host through the protected record (`--acceptance-record`, `--rows 10`), exit 0, witness output sha256:0bbf2befc10f; the same command re-run once on the host at the same source exited 0 and printed `consumers corroboration — base 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf (immutable delivered snapshot), 1 brief(s)` and `summary: 10 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. The two unchecked entries (statusgen/evaluation, runtime controller) are out-of-scope exclusions left to reviewer judgment, not passing consumers. The /23, /29 and /19 follow-up entries are corroborated as owning-brief routing only; the candidate-workspace, regression-adapter and durable-store actual-use obligations stay with graph-execution/23, graph-execution/29 and graph-execution/19 per the approved boundary, and this run claims none of them. Omitted consumers remain a review obligation.

Supplemental failure controls (no acceptance credit, no row of their own): in a separate container from the same pinned image with network none and the home mounted read-only, `python3 -I statusgen/domain/mutate.py` exited 0, printing PASS unmutated snapshot and then KILLED for all 11 mutations, each by its named failing test: path-identity (row 1), unknown-fact (row 2), candidate-binding (row 3), success-acceptance (row 4), generation (row 5), commit-for-tree (row 6), stage-conflation (row 9), effect-import, schema-open (row 8), schema-drift and canonical-order. The row-3 mutation compares the repository instead of the frozen candidate commit and tree; the candidate record has no branch field, so that is the faithful form of the row's branch-name mutation. Row 7's own mutation was probed directly: in a scratch copy of statusgen inside the container, a file adding an os import to package domain made the unchanged row-7 command exit 1, and the same copy without that file exits 0. The home was not modified by either probe.

Input accounting: between run 5's source e25479a69d69ad678c15253309fcac6b7b0c86b0 and this source, the changed paths among the Context files and the transitive contexthandoff, evaluation, workref, testschema, instance-schema and module inputs are docs/contracts.md (one unrelated new row for cross-root delivery witnesses), docs/streams/desktools-v2/cli-migration.json (other entries; the statusgen/domain/mutate.py test route this brief relies on is unchanged) and statusgen/workref/trailers, which is outside the domain package's build and test dependency closure (`go list -deps -test ./domain/` names workref, contexthandoff, evaluation, domain and internal/testschema only). The production domain Go files, fixtures, spec and schema are byte-identical to the delivery.

Risk-value inventory: the brief is all-risk-no and irreversible-no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row needs a fail-first base witness. Enumerated over the delivered diff 0580e0a02df8dce18bd2d65359225e49d525bcd6..3ced448e5ff83c6fbbd2a7c08d6d4b9dc6e2fdaf (32 files) and re-read at the observed source: completeness vocabulary, subject-count bounds, object and digest grammars, the instance, ordinal and work-brief grammars, the capability token, the completion predicate and the closed schema; fixture constants and the mutation runner's 30s/60s timeouts (statusgen/domain/mutate.py:77-79 and :96) are test knobs. Every value is reversible by an edit and redeploy. Ranked by consequence: wrong-subject or incomplete-fact acceptance first, then encoding and capability, then stage separation; the test knobs rank last.

RISK-VALUE: DERIVED — `Present = "present"` @ statusgen/domain/records.go:171 — the only complete required-fact state; assess.go:77 routes every other state into a FactHold before either evaluator is called, which is what Task 2 requires.
RISK-VALUE: DERIVED — `Missing = "missing"` and `CouldNotCheck = "could-not-check"` @ statusgen/domain/records.go:172-173 — Task 1 names exactly these two non-complete states; the unknown-fact mutation proves treating either as present is caught.
RISK-VALUE: DERIVED — `count != 1` and `len(o.Coverage.Briefs) != 1` @ statusgen/domain/assess.go:101,104 — one assessment binds one qualified subject and one candidate; zero is absent and more than one is ambiguous, so both refuse before evaluation.
RISK-VALUE: DERIVED — `objectRE = ^([0-9a-f]{40}|[0-9a-f]{64})$` @ statusgen/domain/records.go:60 and schemas/verification-domain-v1.json:94 — full SHA-1 (20 bytes, 40 hex) and SHA-256 (32 bytes, 64 hex) git object IDs; abbreviations and refs could alias, and records.go:122 requires all four candidate objects to share one format.
RISK-VALUE: DERIVED — `digestRE = ^sha256:[0-9a-f]{64}$` @ statusgen/domain/records.go:61 — SHA-256 is 32 bytes, 64 hex digits; the algorithm label makes any other form an explicit refusal.
RISK-VALUE: DERIVED — `instanceRE = ^inst-[a-z0-9][a-z0-9-]{2,62}$` @ statusgen/domain/records.go:57 — byte-identical to the graph-execution/09 owner grammar at schemas/workflow-instance-v1.json:36, so no second identity scheme exists.
RISK-VALUE: DERIVED — `ordinalRE = ^[1-9][0-9]*$` @ statusgen/domain/records.go:59 — attempt ordinals start at 1; zero and leading-zero aliases cannot name extra attempts.
RISK-VALUE: DERIVED — work brief pattern `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[0-9]{2,4}[a-z]?$` @ schemas/verification-domain-v1.json:60 — the four-segment `<cell>:<repo>:<stream>:<NN>` identity Task 1 names, with the two-to-four digit, optional-letter NN of the existing context-handoff identity grammar; a file path cannot match it.
RISK-VALUE: DERIVED — `v != "domain@1"` @ statusgen/domain/records.go:325 — the delivered contract is v1; an unsupported mandatory capability refuses as unknown rather than being interpreted (Interface contract).
RISK-VALUE: DERIVED — `Stage == "complete"` requires `Acceptance == "accepted"` and `Publication == "confirmed"` @ statusgen/domain/records.go:288 — the migration amendment requires accepted/publication-pending to stay distinct from complete; the stage-conflation mutation is killed.
RISK-VALUE: DERIVED — `"additionalProperties": false` @ schemas/verification-domain-v1.json:7 — unknown mandatory fields must refuse, never be ignored (Interface contract); the schema-open mutation is killed.

**VERIFY: PASS** — original rows 1–9 pass at 8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 inside a closed container-netns admission bracket (verdict=match) in the release-pinned v1.0.64 runner, 11/11 mutation controls killed; supplemental row 10 exit 0 with 10 corroborated / 0 disproved / 2 unchecked and no acceptance credit.

## Review

Gate: model (all four risk answers no). The reviewer confirms that no second identity, evaluator
or candidate record was introduced, and that every refusal above has a failing control.
