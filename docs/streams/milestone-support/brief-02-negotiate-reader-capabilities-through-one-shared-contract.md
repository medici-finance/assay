---
brief: assay:assay:milestone-support:02
title: "Negotiate reader capabilities through one shared contract"
why: "A parser accepting extra YAML is not proof that dispatch understands its required semantics. A shared capability assessment must stop incompatible operations before they write or execute."
wave: 1
depends: ["milestone-support/01"]
unblocks: ["milestone-support/03", "milestone-support/04", "milestone-support/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes required-semantics gates used before authoring and worker admission; wrong capability interpretation must not authorize unsupported work or weaken existing gates."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: e24c986c-1c67-4d3a-9a75-6d5409433b02
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
  - "statusgen/verifyoutcomes_singlereader_test.go: fixed-here (module-relative exemptions, collision control and existing positive control retained)"
  - "tools/desk/cmd/deskevidence/deskevidence.go and tools/desk/cmd/deskevidence/capabilities.go: fixed-here (receipt-root named reads, remote and final-byte admission, bounded refusals and visible audit-append errors)"
  - "docs/witness-readers.md: fixed-here (existing reader census follows the moved register-name declaration; census check unchanged)"
  - "statusgen/load.go, statusgen/parse.go and statusgen/briefdoc/: fixed-here (one register roster, marker and stream YAML type; existing discovery behavior retained)"
  - "tools/desk/cmd/deskevidence/main.go and tools/desk/cmd/deskevidence/writeflow.go: fixed-here (preflight inside the existing commit audit window; one refusal row, zero forge effects)"
  - "statusgen/briefdoc/document.go and statusgen/brieffile.go: fixed-here (one shared wired schema registry for parser and advertisements)"
  - "statusgen/readercaps/: fixed-here (pure assessment/decoders, existing-codec record adapter and explicit filesystem adapter)"
  - "statusgen/briefschemaversions.go: fixed-here (truthful additive JSON discovery)"
  - "statusgen/main.go: fixed-here (existing version/discovery entrypoint and ordinary/differential admission updated and tested)"
  - "tools/desk/internal/deskkit/briefschemagate.go: fixed-here (shared assessment with preserved legacy release-boundary adaptation)"
  - "tools/desk/cmd/deskboard/: fixed-here (offline JSON discovery and independent consuming-boundary test)"
  - "tools/desk/go.mod: fixed-here (existing statusgen require/replace reused, unchanged; source consumers tested GOWORK=off)"
  - "spec/reader-capabilities-v1.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "schemas/reader-capabilities-v1.json: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "spec/milestone-v1.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "spec/stream-v1.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "spec/brief-v1.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "docs/contracts.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "docs/library-first.md: fixed-here (shared core, discovery/preflight or canonical contract; existing module edge reused)"
  - "docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md: follow-up milestone-support/03 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md: follow-up milestone-support/04 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md: follow-up milestone-support/08 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 02 — Negotiate reader capabilities through one shared contract

## Context

files: `tools/desk/cmd/deskevidence/mutations.json` (existing root-binding mutation re-anchored to the actual read, test/expectation retained), `tools/desk/cmd/deskevidence/capabilities.go`, `tools/desk/cmd/deskevidence/deskevidence.go`, `statusgen/readercaps/diagnostic.go`, `statusgen/readercaps/tree/record.go`, `docs/witness-readers.md`, `tools/desk/cmd/deskclaim/capabilities_test.go`, `tools/desk/cmd/deskpr/capabilities_test.go`, `statusgen/briefdoc/registers.go`, `statusgen/briefdoc/streamfields.go`, `statusgen/load.go`, `statusgen/parse.go`, `statusgen/verifyoutcomes_singlereader_test.go` (qualified exemptions and explicit collision control), `tools/desk/cmd/deskevidence/main.go`, `tools/desk/cmd/deskevidence/writeflow.go`, `statusgen/briefdoc/mapping_test.go`, `tools/desk/cmd/deskevidence/capabilities_test.go`, `tools/desk/cmd/deskevidence/principalstamp_test.go`, `statusgen/briefdoc/document.go`, `statusgen/brieffile.go`, `statusgen/readercaps/spelling_test.go`, `statusgen/readercaps/tree/tree_test.go`, `statusgen/readercaps/` (planned), `statusgen/briefschemaversions.go`, `statusgen/main.go`, `statusgen/versiongate.go`, `statusgen/capabilities_test.go` (planned), `tools/desk/internal/deskkit/briefschemagate.go`, `tools/desk/internal/deskkit/capabilities_test.go` (planned), `tools/desk/cmd/deskboard/`, `tools/desk/go.mod`, `spec/reader-capabilities-v1.md` (planned), `schemas/reader-capabilities-v1.json` (planned), `spec/milestone-v1.md`, `spec/stream-v1.md`, `spec/brief-v1.md`, `docs/contracts.md`, `docs/library-first.md`, `changelog/milestone-support-02.md` (planned), `statusgen/testdata/milestone-support/02-mutations.json` (planned), `statusgen/testdata/milestone-support/02-mutate.py` (planned), `statusgen/testdata/milestone-support/01-mutate.py` (existing bounded runner reused) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen/readercaps owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen/readercaps"
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

## Human decision

Review the concrete identity/capability boundary and preserved existing authority checks through the existing decision process before implementation. Options: approve the bounded design, request correction, or hold. No answer supplies no approval. The authoring request does not grant implementation sign-off.

Default if no answer: none — blocks until answered

## Task

1. Define and implement the shared pure reader-capabilities contract in public statusgen/readercaps, consumed by both binaries. Settle the §10 open envelope/stream opt-in decision here: record provider/version, supported schema/semantic versions and operation scope, and explicit requirements carried by opted-in records. Publish spec/schema and conformance fixtures in this delivery before dependent briefs author those fields.
2. Extend existing machine-readable version/discovery entrypoints without changing bare version output. Advertisements derive from implemented operation support, not an aspirational version number or a hardcoded future release. Scope syntax parsing, milestone assessment and dispatch support separately so early parser completion cannot advertise whole-system readiness.
3. Route the desk-tools schema gate through the same assessment function, with host-specific transport/exit adaptation only. Known unsupported required semantics refuse; missing/unreadable/malformed/stale peer advertisements remain could-not-check. Unknown required features never disappear as optional data. Legacy operations with no new requirements preserve existing behavior.
4. Prove an old-reader/new-record pair cannot reach file mutation, claim, dispatch or acceptance; also bypass an upper capability check to exercise the independent consuming boundary. No user pin, installed cache, stream status or release is changed by implementing the contract.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v ./readercaps/... ./briefdoc > "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsCompat ' "$milestone_verify_out" && grep -q -- '--- PASS: TestRecordRequirements ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsSpelling ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsTree ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsDiagnostic ' "$milestone_verify_out" && grep -q -- '--- PASS: TestParseMapping ' "$milestone_verify_out")` | exit 0 and named PASS; old peer plus new requirement refuses, no-requirement legacy operation succeeds; ignoring a required capability makes the case fail. |
| 2 | check:ci +flow +mutation +dereference | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^(TestStatusgenCapsTruth\|TestVerifyOutcomesSingleReader\|TestVerifyOutcomesSingleReaderPaths\|TestWitnessReaderCensus)$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestStatusgenCapsTruth ' "$milestone_verify_out" && grep -q -- '--- PASS: TestVerifyOutcomesSingleReader ' "$milestone_verify_out" && grep -q -- '--- PASS: TestVerifyOutcomesSingleReaderPaths ' "$milestone_verify_out" && grep -q -- '--- PASS: TestWitnessReaderCensus ' "$milestone_verify_out")` | exit 0 and named PASS; actual version/discovery reports only wired support, unknown/stale data cannot look supported; planted unsupported advertisement fails. |
| 3 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskCapsPreflight$' ./internal/deskkit/... ./cmd/deskboard/... ./cmd/deskevidence/... ./cmd/deskclaim/... ./cmd/deskpr/... > "$milestone_verify_out" && test "$(grep -c -- '--- PASS: TestDeskCapsPreflight ' "$milestone_verify_out")" -eq 5 && GOWORK=off go test -count=1 -v -run '^TestPrincipalStampReadsBodyNotFrontmatter$' ./cmd/deskevidence > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrincipalStampReadsBodyNotFrontmatter ' "$milestone_verify_out")` | exit 0 and named PASS; production gate invokes shared assessment with zero landing/dispatch effects on incompatibility even with the outer check bypassed; Evidence named reads use the admitted root, final bytes are checked after a late mutation, and bounded refusal rows preserve a following ordinary write. Audit append failures are visible. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

Verifier pass, 2026-10-31, by a non-implementer (model claude-opus-5-5, tier strong) at merged main dbed7e67aab8923b480937f5266855f33d8f336e. Delivering change: #4947, merge commit 74d0a8901ccf3a9167878070674b5f8edb35bc80, read from the forge; its first parent is ca3ce92a594777502548802b1d9236906c8714a2. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run 057eb1c9d890, attestation #5104). Installed statusgen v1.0.67. The Verify table re-derived with 3 rows and 0 author-added (deskpathguard rederive, base dbed7e67aab8).

Rows 1-3 (check:ci) ran in one closed container bracket: the pinned verify-runner v1.0.67 image (linux/amd64 under emulation, go 1.27.3, digest below), --network none, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null, a non-root uid, the source tree, its git dir and the Go module cache mounted read-only, and a fresh GOCACHE. No forge token entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result.

Fail-first is owed and has no witness. The brief's risk block answers sensitive-data: yes, so every row is risk-bearing (statusgen failFirstRiskyIDs), and the fail-first-closure lint needs a current Base-column witness for each. Rows of class check:ci cannot run on this darwin host, and the container run cannot cut the base worktree from a read-only git dir. Discarded attempt, disclosed: the first bracket (opened 2026-10-31T23:14:41Z) passed --fail-first --base ca3ce92a594777502548802b1d9236906c8714a2, ran no row, wrote no witness and did not close. Its refusal: `fatal: could not create directory of '<git dir>/worktrees/tree': Read-only file system`. The git dir was not made writable, because it lies outside this run's home. The --history-out and --history-pack transport for this case first ships in v1.0.68; v1.0.67, installed here and in the image, has neither flag.

Rows S1-S3 are supplemental and carry no acceptance credit. S2 and S3 ran in the same image and network-off settings against tracked-file snapshots outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | statusgen verifyrun --sandbox=container-netns --fail-first --base ca3ce92a594777502548802b1d9236906c8714a2 (bracketed, as above) | Base-column witness for rows 1-3 | could-not-run: exit 2 before any row, base worktree refused on the read-only git dir; v1.0.67 has no history transport | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 | rows 1-3 as written, against a snapshot of ca3ce92a5947 | each row red before the change | exit 1, exit 1, exit 1; row 1 reports no readercaps directory. Probe only, not a fail-first witness | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 | python3 statusgen/testdata/milestone-support/02-mutate.py, against a snapshot of dbed7e67aab8 | every manifest plant turns its named test red | exit 0; 47 of 47 cases printed CAUGHT, all 12 distinct named baseline tests passed, and no manifest case was missed. Includes required-feature, unsupported-discovery, audit-append-failure-visible and final-record-admission | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Risk-bearing values. Enumerated over every non-test Go file in the diff of ca3ce92a5947..74d0a8901ccf, plus the deliverables' schema and spec. No value is irreversible; each is undone by an edit and a release. A wrong one mis-sizes a refusal or misroutes an exit code. Ranked top entries:

- RISK-VALUE: DERIVED — statusgenExitUnsupportedCaps = 5 @ statusgen/versiongate.go:52 — equals deskkit ExitRefused = 5 (tools/desk/internal/deskkit/exitcodes.go:51), so both binaries report a known unsupported requirement the same way, and it differs from could-not-check.
- RISK-VALUE: DERIVED — statusgenExitTreeTooNew = 6 @ statusgen/versiongate.go:49 — unchanged value now also carrying could-not-check; equals deskkit ExitUnverifiable = 6 (tools/desk/internal/deskkit/exitcodes.go:55).
- RISK-VALUE: DERIVED — DiagnosticLimit = 2048 @ statusgen/readercaps/diagnostic.go:9 — spec/reader-capabilities-v1.md:122 pins it, and it sits far below the audit reader's 4 MiB line cap (tailMaxLine, tools/desk/internal/deskkit/audittail.go:45), which is the bound the spec says a refusal must not exceed. The truncation branch reserves the marker's length, so output stays within the limit.
- RISK-VALUE: DERIVED — AdvertisementSchema = "reader-capabilities-v1" @ statusgen/readercaps/caps.go:13 and RequirementsSchema = "reader-requirements-v1" @ statusgen/readercaps/caps.go:14 — both equal the const values in schemas/reader-capabilities-v1.json.
- RISK-VALUE: NAMED, NOT DERIVED — semantic id "required-semantics-v1" @ statusgen/briefschemaversions.go:58 and tools/desk/internal/deskkit/briefschemagate.go:112 — the two advertisements agree, and the design approved on #4531 calls for operation-scoped semantic versions without naming this id. It is a name choice with no first-principles value to derive.

The design decision on #4531 records a human approval of the bounded design and the issue is still open. This brief is gate: human: these rows do not sign it off and no lifecycle cell was changed. The stream board row still reads todo although #4947 merged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v ./readercaps/... ./briefdoc > "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsCompat ' "$milestone_verify_out" && grep -q -- '--- PASS: TestRecordRequirements ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsSpelling ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsTree ' "$milestone_verify_out" && grep -q -- '--- PASS: TestReaderCapsDiagnostic ' "$milestone_verify_out" && grep -q -- '--- PASS: TestParseMapping ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ dbed7e67aab8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^(TestStatusgenCapsTruth\|TestVerifyOutcomesSingleReader\|TestVerifyOutcomesSingleReaderPaths\|TestWitnessReaderCensus)$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestStatusgenCapsTruth ' "$milestone_verify_out" && grep -q -- '--- PASS: TestVerifyOutcomesSingleReader ' "$milestone_verify_out" && grep -q -- '--- PASS: TestVerifyOutcomesSingleReaderPaths ' "$milestone_verify_out" && grep -q -- '--- PASS: TestWitnessReaderCensus ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ dbed7e67aab8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskCapsPreflight$' ./internal/deskkit/... ./cmd/deskboard/... ./cmd/deskevidence/... ./cmd/deskclaim/... ./cmd/deskpr/... > "$milestone_verify_out" && test "$(grep -c -- '--- PASS: TestDeskCapsPreflight ' "$milestone_verify_out")" -eq 5 && GOWORK=off go test -count=1 -v -run '^TestPrincipalStampReadsBodyNotFrontmatter$' ./cmd/deskevidence > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrincipalStampReadsBodyNotFrontmatter ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ dbed7e67aab8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9c661ba489e0bb8abb206638a3949ce5a057964157e32699a3646ace9d510aed
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5104 run=057eb1c9d8906933bb50ab9f286b30e4f5b72a1f7f8221ae899ae4c6e88529d8 source=dbed7e67aab8923b480937f5266855f33d8f336e brief=docs/streams/milestone-support/brief-02-negotiate-reader-capabilities-through-one-shared-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=057eb1c9d8906933bb50ab9f286b30e4f5b72a1f7f8221ae899ae4c6e88529d8 source=dbed7e67aab8923b480937f5266855f33d8f336e tree=sha256:9c661ba489e0bb8abb206638a3949ce5a057964157e32699a3646ace9d510aed witness=sha256:71f1b9890a3e3557b3a861abaf9feb266c861cd535f08eb89e382febfbee6b0e opened=2026-10-31T23:15:22Z closed=2026-10-31T23:19:52Z before=receipt after=check-verifier

**VERIFY: BLOCKED** — rows 1-3 pass in the closed bracket (verdict=match) and 47 of 47 mutations were caught, with no failed obligation observed. The fail-first witness owed for these risk-bearing rows could not be produced under v1.0.67; it needs a re-dispatch on v1.0.68 or later, host and image, using its history transport.

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
