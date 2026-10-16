---
brief: assay:assay:graph-execution:09
title: Versioned workflow instances and shared identity
why: An approved pattern is not yet an executable contract for a particular change. Binding its subject, Cell and acceptance prevents a retry from silently using new inputs or a different permission set.
wave: 1
depends:
- graph-execution/02
unblocks:
- graph-execution/14
- graph-execution/16
- graph-execution/19
- graph-execution/28
- performance-collection/09
effort: M
gate: model
risk:
  regulatory: no
  customer: no
  irreversible: no
  sensitive-data: no
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — non-CI architecture reconciliation 2026-10-13 (R1 owner row)
- freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'statusgen/experiment.go: follow-up graph-execution/05'
- 'drainloop: follow-up graph-execution/16'
- 'tools/desk: follow-up graph-execution/14'
- 'workflow/store and bindings: follow-up graph-execution/19'
- 'workflow/controller: follow-up graph-execution/21'
version: 3
id: eb0a9f7c-669d-42f6-9686-e30c661a2450
---

# Brief 09 — Versioned workflow instances and shared identity

## Context

files: `spec/workflow-instance-v1.md` (planned), `schemas/workflow-instance-v1.json` (planned), `statusgen/instance.go` (planned), `statusgen/instance_test.go` (planned), `statusgen/main.go`, `statusgen/testdata/instances/` (planned), `docs/lifecycle.md`, `changelog/graph-execution-09-instance-contract.md` (planned)

facts: Pattern v1 explicitly excludes instantiation; keep its implemented schema intact until a compatible extension is reviewed. Canonical work IDs already exist; instance/node/attempt IDs are additional identities.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Implement WI-1/WI-2 in the instance schema and validator already owned here. The packet is
an exported role view of the canonical instance, never a new run record. Bind intent,
acceptance scope, source/dependency fingerprints, coverage/omissions, unresolved inputs,
artifact references, delta and next safe action. Keep runner/control/reservation references
without inventing authority. A complete diff may be large; reject silent truncation.

Add positive and negative fixtures in the declared instance testdata: same-input round trip,
changed policy outside the touched source file, omitted mandatory evidence and unknown
coverage. Raw artifacts remain accessible. Serialize result/handoff provenance using the
same IDs. Actual prompt assembly/dispatch belongs to 14; usage settlement belongs to 16.
No semantic dependency inference or provider-specific model client is in scope.

## Task workflow amendment — 2026-10-02

Extend the existing instance contract with backend-assignment/generation and runner-profile references, plus a versioned execution-state projection. IDs and acceptance remain canonical. 19 owns persisted instances and the real coverage-binding adapter; a declared instance with a missing binding cannot degrade to no-pattern behavior. Keep packet schemas provider-neutral.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Non-CI architecture reconciliation — 2026-10-13

The maintainer ratified the non-CI verifier target (medici-finance/assay#2500, revision
23f655f5d); approval activates nothing. Package R1 (domain contracts and regression floor) is
owned by graph-execution/28 and graph-execution/29, which key their qualified-identity,
candidate and acceptance-binding records on the instance, node and attempt IDs this brief
defines. This brief stays the single owner of those IDs; graph-execution/28 adds no second
identity scheme. The Task and Verify sections below are unchanged by this amendment. See
`docs/proposals/non-ci-architecture/20-amendment-manifest.md`.

## Task

1. Define and validate the GEA-01 instance fields, pinned input/acceptance/policy references and supersession. Include export/import and old-reader capability refusal; no in-place rewrite of prior decisions.
2. Add statusgen instance validate --file PATH --json. Reuse pattern lint and eligibility. Instance effects must be a subset of both pattern declarations and resolved actor capabilities; an instance never mints credentials.
3. Publish fixtures for immutable identity, stale acceptance, unknown mandatory field and cross-Cell substitution. Document that human decisions are external authenticated acts, not a new human token-bearing model role.

## Interface contract

Instantiator → validator → serialized instance preserves exact references; tampering is rejected.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstance" ./...` | exit 0; output includes PASS for TestInstance, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceCrossCellDenied" ./...` | exit 0; output includes PASS for TestInstanceCrossCellDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceRoundTripIdentity" ./...` | exit 0; output includes PASS for TestInstanceRoundTripIdentity, with no [no tests to run] for its owning package |
| 4 | check:ci +flow | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketViewProvenance.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketViewProvenance$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketViewProvenance " "$wi_out")` | exit 0; named PASS; canonical instance → role view → handoff preserves revision, scope and omissions |
| 5 | check:ci +mutation | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketChangedDependency.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketChangedDependency$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketChangedDependency " "$wi_out")` | exit 0; named PASS; changed dependency or omitted required evidence cannot masquerade as the previous current packet; mutation: accept a stale dependency fingerprint — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd statusgen && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstanceBackendRoundTrip$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInstanceBackendRoundTrip " "$routing_out")` | exit 0; named PASS; export/import preserves identity, assignment and acceptance; substitution is rejected; mutation: drop the assignment generation on export — the named test must fail |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstance" ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceCrossCellDenied" ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceRoundTripIdentity" ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketViewProvenance.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketViewProvenance$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketViewProvenance " "$wi_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketChangedDependency.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketChangedDependency$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketChangedDependency " "$wi_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstanceBackendRoundTrip$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInstanceBackendRoundTrip " "$routing_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ 7d52be61b66c (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3060 run=271faea33f2908ed7a041095db0da25487b48d49d8dca39f6fc8b71c66ea5ba2 source=7d52be61b66cb81f5ad2ab7e1d250747a6f125de brief=docs/streams/graph-execution/brief-09-instance-contract.md model=claude-opus-5-5 tier=any

### Independent verification — 2026-10-15, merged main 7d52be61b66cb81f5ad2ab7e1d250747a6f125de

The execution witness above was produced first, on the clean tree; every check:ci row in it is could-not-run because this host is darwin and has no `unshare --net`. Each Verify row was then executed by hand at the same commit, with outbound network denied by macOS sandbox-exec using the profile `(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))`. The deny was proven inside the sandbox before the runs: curl to the Go module proxy exited 6 (host could not be resolved), a raw TCP connect to 1.1.1.1:443 failed with Operation not permitted, a loopback connect succeeded, and the same curl outside the sandbox returned HTTP 200. Environment: go1.27.2 darwin/arm64, GOWORK=off, GOPROXY=off, GOTOOLCHAIN=local, throwaway HOME, the real Go build and module caches, KUBECONFIG=/dev/null, scratch TMPDIR outside the worktree.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstance" ./...` | exit 0; PASS for TestInstance; owning package not [no tests to run] | exit 0; 17 tests PASS in the owning package (TestInstanceFixtures, CrossCellDenied, RoundTripIdentity, PacketViewProvenance, PacketChangedDependency, BackendRoundTrip, Supersession, UnknownCapability, CredentialRejected, SecretNotEchoed, ParseErrNotEcho, ParseLineOwn, TrustNotPromoted, EffectsSubset, MissingPattern, CLIJSON, SchemaParity); "ok github.com/medici-finance/assay/statusgen 0.424s" with no [no tests to run]; only the sibling packages contexthandoff, evaluation and streamview report [no tests to run] | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceCrossCellDenied" ./...` | exit 0; named PASS; owning package not [no tests to run]; mutation goes red | exit 0; "--- PASS: TestInstanceCrossCellDenied (0.01s)"; "ok github.com/medici-finance/assay/statusgen 0.205s". Mutations (scratch copy, see below): import Cell check disabled gives exit 1, "import into example-cell-b: err nil, want a instance-cross-cell refusal"; node actor Cell check disabled gives exit 1, "cross-cell actor: state checked-clean ... want checked-failed [instance-cross-cell]". Two independent boundaries (validator and import) each caught by the test | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceRoundTripIdentity" ./...` | exit 0; named PASS; owning package not [no tests to run] | exit 0; "--- PASS: TestInstanceRoundTripIdentity (0.02s)"; "ok github.com/medici-finance/assay/statusgen 0.297s". Flow seam: the test drives production instantiate, exportInstance, validateInstanceFile (the CLI pipeline), importInstance and re-export (byte-identical), plus tamper-after-export refused with instance-seal-mismatch | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |
| 4 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketViewProvenance.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketViewProvenance$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketViewProvenance " "$wi_out")` | exit 0; named PASS; instance to role view to handoff keeps revision, scope and omissions | exit 0 (grep matched). Same test re-run with visible output: "--- PASS: TestInstancePacketViewProvenance (0.04s)". Flow seam: production projectPacket, buildHandoff and acceptHandoff; asserts acceptance version/ref, exclusions, omissions and unresolved reporter input carried instance to packet to handoff | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |
| 5 | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketChangedDependency.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketChangedDependency$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketChangedDependency " "$wi_out")` | exit 0; named PASS; mutation accepting a stale dependency fingerprint makes the test fail | exit 0 (grep matched). Visible re-run: "--- PASS: TestInstancePacketChangedDependency (0.02s)". Mutation (only a missing observation counts as stale) gives exit 1, "changed-policy.json: acceptance err nil, want a refusal containing dependencies moved"; restored gives exit 0 PASS | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |
| 6 | `(cd statusgen && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstanceBackendRoundTrip$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInstanceBackendRoundTrip " "$routing_out")` | exit 0; named PASS; export/import keeps identity, assignment, acceptance; substitution rejected; mutation dropping the assignment generation on export makes the test fail | exit 0 (grep matched). Visible re-run: "--- PASS: TestInstanceBackendRoundTrip (0.01s)". Mutation (export writes generation 0) gives exit 1, "import rejected: [instance-schema-violation] assignment.generation: 0 is below the minimum 1"; second mutation (same-generation backend change not treated as substitution) gives exit 1, "same-generation backend change: [], want [instance-assignment-substituted]"; both restored give exit 0 PASS | 2026-10-15 assay-verifier-app[bot], model claude-opus-5-5 |

Mutation procedure: a bounded tracked snapshot of the attested commit (git archive) was extracted to a scratch directory outside the worktree; its baseline run of the TestInstance tests exited 0. Each mutation was one literal replacement in statusgen/instance.go taken from statusgen/instance-mutations.json (row 2: lines 728 and 811; row 5: line 1654; row 6: lines 647 and 1043), run sandboxed with `go test -count=1 -v -run '^<named test>$' .`, then the file was rewritten from git show of the attested commit, a diff against the attested file came back empty, and the named test re-ran green. The worktree itself was never mutated.

Task 2 CLI cross-check (supplementary, same sandbox): a scratch build of statusgen at this commit ran `instance validate --file <fixture> --root testdata/instances/root --json` from the statusgen directory. valid.yaml exit 0 checked-clean, eligibility verdict eligible; cross-cell.yaml exit 1 instance-cross-cell; capability-minted.yaml exit 1 instance-capability-minted; stale-acceptance.yaml exit 1 instance-acceptance-stale; unknown-mandatory-field.yaml exit 2 could-not-check instance-unsupported-capability (requires settlement@1, refused rather than ignored).

Review-section items: credential exclusion is covered by TestInstanceCredentialRejected and TestInstanceSecretNotEchoed, and reporter-label trust provenance by TestInstanceTrustNotPromoted; all three PASS inside row 1. The schema copy embedded in statusgen is byte-identical to the top-level schemas copy (cmp exit 0).

Risk metadata: gate model; regulatory, customer, irreversible and sensitive-data all no. Enumerated over the literals in statusgen/instance.go and the workflow-instance-v1 schema introduced by the delivering change; ranked by what a wrong value would let through.

- RISK-VALUE: DERIVED — assignment.generation minimum = 1 @ statusgen/schemas/workflow-instance-v1.json:109 (execution-state generation minimum = 1 @ :120) — a Go int zero value is 0, so a floor of 1 makes a dropped or unset generation a schema refusal instead of a silently valid assignment; row 6's drop-generation mutation is refused at exactly this bound.
- RISK-VALUE: DERIVED — seal and contract digest form = "sha256:" + 64 lowercase hex @ statusgen/instance.go:583 and statusgen/schemas/workflow-instance-v1.json:17 — SHA-256 hex output is 32 bytes, 64 hex characters; collision resistance is what makes tamper-after-export detectable, and the pattern rejects a truncated digest.
- RISK-VALUE: DERIVED — instanceReaderCapabilities = {packet-view@1, assignment@1, execution-state@1} @ statusgen/instance.go:120-124 — identical to instanceWriterRequires @ statusgen/instance.go:129, so this reader accepts exactly what this writer declares and refuses any other required capability (old-reader refusal, Task 1).
- RISK-VALUE: DERIVED — instanceProjectionVersions = {1: true} @ statusgen/instance.go:133 — v1 of the contract defines one execution-state projection; any other version is refused, never read as optional.
- RISK-VALUE: DERIVED — acceptance.ref commit = 40 hex @ statusgen/schemas/workflow-instance-v1.json:80 (policy.revision @ :90) — a full SHA-1 git object id; an abbreviated or branch-name ref is refused, which is what pins acceptance immutably. A SHA-256 object-format repository (64 hex) would be refused, which fails closed.
- RISK-VALUE: DERIVED — AWS access key shape AKIA[0-9A-Z]{16} @ statusgen/instance.go:450 — AWS access key IDs are 20 characters, AKIA plus 16.
- RISK-VALUE: NAMED, NOT DERIVED — credential minimum lengths {20,} for gh*_, github_pat_, glpat-, sk- @ statusgen/instance.go:447-449,451, {10,} for xox* @ :452, {8,} for bearer/basic @ :454 — heuristic floors not tied to a published format specification in this repository; too high a floor would let a short token through the scan. Defense-in-depth only (an instance carries references, never material), so ranked below the identity bounds.
- RISK-VALUE: NAMED, NOT DERIVED — instance id length {2,62} after "inst-" plus one leading character @ statusgen/schemas/workflow-instance-v1.json:36 — no stated derivation; a reversible naming bound.
- Exit codes 0/1/2 @ statusgen/instance.go:81-84 follow the three-state checked-clean / checked-failed / could-not-check convention; reversible, no derivation needed.

Verification-Attestation: medici-finance/assay#3060 run=271faea33f2908ed7a041095db0da25487b48d49d8dca39f6fc8b71c66ea5ba2 source=7d52be61b66cb81f5ad2ab7e1d250747a6f125de brief=docs/streams/graph-execution/brief-09-instance-contract.md model=claude-opus-5-5 tier=any

**VERIFY: PASS** — 6/6 rows exit 0 by hand under a proven network-off sandbox at 7d52be61b66c; mutations for rows 2, 5 and 6 each turn the named test red and restore green; the statusgen verifyrun witness rows are could-not-run on this darwin host (no `unshare --net`), so a Linux re-run would be needed for a green witness.


## Review

Review the packet fixtures for credential exclusion and trust provenance: a synthetic secret
must be rejected or replaced with an authorized reference, and a reporter-supplied trust
label must not become an authoritative disposition through export/import.

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
