---
brief: assay:assay:comms-core:01
title: Extract the comms contract and isolate Assay policy
why: "External tools cannot import the current internal comms package, and its generic envelope is coupled to Assay roles. Separate the contract and policy without changing accepted messages or refusals."
wave: 0
depends: []
unblocks: ["comms-core/02"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Moving the contract changes which messages a public module accepts and which policy still refuses them. A human confirms the compatibility report before any trust-adjacent code moves."
design: DR-comms-core-extract
decision-issue: 4397
ruling: "https://github.com/medici-finance/assay/issues/4397#issuecomment-6399352645"
issues: []
schema: brief-v2
outcome: none
authored: 2026-10-13 by statusgen newbrief
sources:
  - "docs/proposals/non-ci-architecture/24-communications.md"
  - "freshness-checked 2026-10-13 at a435badbd1d245402407acd2a82a6abe5f37f637 after origin/main fetch"
version: 1
id: 8e8fc4a3-2af0-4b72-9adb-6765f0a18de6
decision-trigger: spec
exec-tier: strong
exec-tier-why: "Cross-package authority and compatibility must agree through real consumer flows."
domain: complicated
consumers:
  - "tools/desk/cmd/commsgw: fixed-here"
  - "tools/desk/cmd/commsloop: fixed-here"
  - "tools/desk/cmd/deskcomms: fixed-here"
  - "tools/desk/internal/commsqueue: fixed-here"
  - "comms/server: follow-up comms-core/02"
  - "comms/client: follow-up comms-core/03"
---

# Brief 01 — Extract the comms contract and isolate Assay policy

## Context

files:
- `comms/go.mod` (planned), `comms/go.sum` (planned)
- `tools/desk/go.mod`, `tools/desk/go.sum` (consumer dependency)
- `comms/contract (new)`
- `tools/desk/internal/comms`
- `tools/desk/internal/deskkit (existing policy owner; no public export)`
- `tools/desk/internal/commspolicy (optional new composition adapter; the Assay adapter may instead stay in internal/comms, and either way it lives under tools/desk/internal/)`
- `tools/desk/cmd/{commsgw,commsloop,deskcomms} (import adaptation only)`
- `tools/desk/internal/commsqueue (import adaptation only; its quarantine filer stays in tools/desk, see facts)`
- `docs/contracts.md`
- `docs/library-first.md`
- `changelog/comms-contract.md` (planned)

facts:
- At a435badbd, internal/comms/laneacl.go imports internal/deskkit. Envelope, identity and ACL tests already exist; public Go consumers cannot import the internal package. `tools/desk/internal/commsqueue/quarantine.go` also reaches the forge (`DeskfileIssueFiler` shells out to `deskfile`; `FileDeskIssue` imports `internal/deskkit`), so commsqueue is a second deskkit coupling to cut in comms-core/02, not a plain import move. The cell-wide key profile and cellmsg-v1 wire remain unchanged.
- Source snapshot: 2026-10-13, a435badbd1d245402407acd2a82a6abe5f37f637. Refresh code, claims and PRs before dispatch; no implementation is claimed here.

layering: comms/contract for structural types; existing Assay policy via internal composition.
single-point-of-failure: service admission remains authoritative; client preflight is advisory, and existing transport/custody and effect-boundary controls remain independent checks.
design-fit:
  owner: comms/contract for structural types; existing Assay policy via internal composition
  contract: docs/proposals/non-ci-architecture/24-communications.md
  retires: command-local duplicated or unimportable implementation only after callers move
  weight: no additional daemon or broker; narrow public packages and compatibility fixtures
  why-add: reuse the existing implementation across clients/hosts without exporting authority

## Read first

- [Communications target](../../proposals/non-ci-architecture/24-communications.md).
- [Library boundary](../../library-first.md) and [host comms](../../cellctl-comms.md).
- Existing tests beside the source paths above.

## Human decision

At pickup, produce a compatibility and authority report before approval. It must identify
every changed credential source, accepted message, refusal, persisted record and public API.
The intended scope preserves the existing trust and activation profile.

Options:
1. **Approve the bounded extraction (recommended)** — accept the reviewed report with the existing authority and disclosure controls preserved.
2. **Narrow or revise** — remove incompatible changes and resubmit the report.

Default if no answer: none — blocks until answered.

Ruled 2026-10-25: option 1, on decision issue #4397 (the `ruling:` frontmatter link). The
design record is [DR-comms-core-extract](../decisions/DR-comms-core-extract.md).

## Ground rules

Feature branch and draft PR only; no live infrastructure, activation, release or merge.
Do not alter completed Evidence or claim verification as the implementer. Keep supported-path
fixes independently shippable and preserve their tests when moving code. Proposed paths and
test names below are deliverables, not claims they already exist.

## Task

1. Create the root `comms/` module with module path `github.com/medici-finance/assay/comms`; it must not import desk-tools, including transitively. Wire desk-tools as a versioned consumer and qualify both modules with GOWORK=off. Inventory the exported contract and all importers. Move structural types and parsing primitives into comms/contract. Keep Assay verb/class/role validation and compiled ACL in the Assay adapter, which lives under `tools/desk/internal/` (in `internal/comms` or a new `internal/commspolicy`); do not widen acceptance when separating structural parsing from policy.
2. Publish typed message/address/error/receipt interfaces and the legacy wire mapping from the specification. Mark future delivery-token and per-client identity semantics as unimplemented versioned capabilities, not guarantees of cellmsg-v1.
3. Keep one implementation: migrate importers or retain thin aliases/delegators with named consumers. Do not move deskkit, token minting, forge access, role execution or process launch into the public package dependency closure.
4. Add characterization fixtures for valid and refused envelopes, assertion bytes, unknown/reserved verbs, class validation and lane rules. Exercise the same fixtures through old-compatible and extracted entrypoints. Place the policy-call mutation (`TestPolicyAdapterRefusesForbiddenMessage`) in `tools/desk`, beside the Assay policy adapter that Task 1 keeps there; the `comms` module cannot import it. Removing the policy call must make an otherwise structurally valid forbidden message fail that test.
5. Update the existing semantic-owner index and library-first map with actual exported names and consumers. No separate ownership registry or new release is required.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractCompatibility$' > ${TMPDIR:-/tmp}/comms-core-01-flow.log 2>&1 && grep -F -- '--- PASS: TestContractCompatibility ' ${TMPDIR:-/tmp}/comms-core-01-flow.log` | exit 0; `TestContractCompatibility` runs the legacy valid/refused envelope and assertion-byte fixtures through the `comms/contract` parser and shows byte-identical structural results; it makes no policy claim |
| 2 | check +flow | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractDependencyBoundary$' > ${TMPDIR:-/tmp}/comms-core-01-boundary.log 2>&1 && grep -F -- '--- PASS: TestContractDependencyBoundary ' ${TMPDIR:-/tmp}/comms-core-01-boundary.log` | exit 0; `TestContractDependencyBoundary` walks the `comms` module's transitive imports and fails on tools/desk, deskkit, forge, credential or process-launch packages |
| 3 | check | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./internal/comms/... ./internal/commsqueue/... ./cmd/commsgw/... ./cmd/commsloop/... ./cmd/deskcomms/...)` | exit 0; existing and moved tests pass |
| 4 | check | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | exit 0; Windows compile/vet only, not a native-runtime witness |
| 5 | check +flow | `cd tools/desk && GOWORK=off go test -count=1 -v ./internal/... -run '^TestPolicyAdapterRefusesForbiddenMessage$' > ${TMPDIR:-/tmp}/comms-core-01-policy.log 2>&1 && grep -F -- '--- PASS: TestPolicyAdapterRefusesForbiddenMessage ' ${TMPDIR:-/tmp}/comms-core-01-policy.log` | exit 0; the test runs in the module that owns the Assay policy adapter, in whichever `tools/desk/internal` package holds it: a structurally valid but policy-forbidden message is refused through the extracted contract, and the same test fails when the policy call is removed (mutation) |

## Pre-mortem

A moved parser silently widens acceptance: flow fixtures include structurally valid but
policy-forbidden messages. A client gains server authority: boundary tests walk transitive
imports and exercise bypass/refusal paths. A named test disappears: Verify requires its
PASS line, not only go test exit 0. An old persisted message or CLI stops working: the
legacy fixture and command suites remain required. Review checks semantic-owner duplication
and whether the declared dependency closure is narrow enough; those are not claimed by a
line-count assertion.

## Evidence

<!-- Independent verifier records commands, exits, output, revision, date and identity. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractCompatibility$' > ${TMPDIR:-/tmp}/comms-core-01-flow.log 2>&1 && grep -F -- '--- PASS: TestContractCompatibility ' ${TMPDIR:-/tmp}/comms-core-01-flow.log` | pass exit=0 | sha256:a148007abc0c | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractDependencyBoundary$' > ${TMPDIR:-/tmp}/comms-core-01-boundary.log 2>&1 && grep -F -- '--- PASS: TestContractDependencyBoundary ' ${TMPDIR:-/tmp}/comms-core-01-boundary.log` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./internal/comms/... ./internal/commsqueue/... ./cmd/commsgw/... ./cmd/commsloop/... ./cmd/deskcomms/...)` | fail exit=1 | sha256:92d05c4fab0d | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test -count=1 -v ./internal/... -run '^TestPolicyAdapterRefusesForbiddenMessage$' > ${TMPDIR:-/tmp}/comms-core-01-policy.log 2>&1 && grep -F -- '--- PASS: TestPolicyAdapterRefusesForbiddenMessage ' ${TMPDIR:-/tmp}/comms-core-01-policy.log` | pass exit=0 | sha256:3c0007441c51 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4985 run=c70c4258dd71eb01876a3d7d1669af95086fb01845532d4c37ff04b465351025 source=5fdae0d07b58aba9042b112258727ac33862bd93 brief=docs/streams/comms-core/brief-01-extract-the-comms-contract-and-isolate-assay-policy.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — rows 2 and 3 exit 1; rows 1, 4 and 5 pass. Source 5fdae0d07b58, Go 1.27.2 darwin/arm64, witness written by statusgen verifyrun v1.0.67 with TMPDIR set to a scratch directory outside any Go module. Evidence only: gate human, sensitive-data risk, lifecycle left at its current state for the human gate.

Failure detail. Row 2 fails on the last negative control of the dependency-boundary test (comms/contract/deps_test.go, line 108): it runs go list -e -deps ./... in an empty temporary directory and expects an error or an empty listing. Under Go 1.27.2, outside any module, that command exits 0 and lists the pseudo-package ./... carrying a load error, so the test reports "a directory with no module listed cleanly — a missing module must fail the row, not count zero". All seven planted-import subtests pass; only the missing-module control fails. The test passes only when the temporary directory happens to sit below an unrelated go.mod, where ./... matches no packages and the listing is empty. Row 3 fails on the same test inside its comms half. Class: implementation defect in the delivered test's negative control (a listed package carrying a load error is not treated as a failed walk); the contract's real closure check and every planted-import check pass.

Supplemental observations (no acceptance credit; 2026-10-30; runner assay-verifier-app[bot], model verifier):

- Row 2 test with TMPDIR unset (default /tmp): exit 1, same line-108 failure. With TMPDIR below a directory holding an unrelated go.mod: exit 0. Direct probe: go list -e -deps ./... in an empty directory outside any module exits 0 and prints ./... under Go 1.27.2.
- Row 3 split: the comms half (cd comms and GOWORK=off go test -count=1 ./...) exits 0 only with TMPDIR below an enclosing go.mod; the tools/desk half with row 3's exact package list exits 0, all five packages ok. Row 3's failure is the row 2 test alone; the tools/desk half is short-circuited by the and-chain in the witness run.
- Row 5 mutation: in a scratch clone at 5fdae0d07b58, the ACL check inside Admit (tools/desk/internal/comms/envelope.go) disabled, the policy-adapter refusal test exits 1 with a FAIL line and no PASS line. The mutation obligation discriminates.
- Ambient identity: no row calls gh, so the forge-identity Runner stamp carries no ambient-login exposure here.

Risk-bearing values (risk sensitive-data yes, irreversible no). Enumeration covered the non-test Go files under comms/contract and the alias layer in tools/desk/internal/comms changed by the implementation merge 2cb2b41f02e6; every literal is byte-identical to its pre-extraction value in tools/desk/internal/comms at d71c48d61886. Ranked: wire and signature literals first (a change strands every signed or persisted cellmsg-v1 message), then the assertion validity window, then size caps and capability labels (reversible operational bounds, ranked last, no derivation required: MaxEnvelopeBytes = 64 * 1024 @ comms/contract/envelope.go:23, MaxPayloadBytes = 16 * 1024 @ :25, MaxFieldBytes = 256 @ :27, MaxRefs = 32 @ :29, MaxRefBytes = 256 @ :31, CapDeliveryToken = "delivery-token/v1" @ comms/contract/types.go:211, CapClientIdentity = "client-identity/v1" @ comms/contract/types.go:216).

- RISK-VALUE: DERIVED — assertion domain tag = "comms/assertion/cellmsg-v1" plus a newline byte @ comms/contract/identity.go:258 — identical to the pre-extraction literal (same file line in tools/desk/internal/comms at d71c48d61886); the brief requires unchanged assertion bytes, and row 1's assertion-byte fixtures pass.
- RISK-VALUE: DERIVED — Schema = "cellmsg-v1" @ comms/contract/envelope.go:14 — identical to the pre-extraction literal (tools/desk/internal/comms envelope.go line 15 at d71c48d61886); the brief keeps the cellmsg-v1 wire unchanged.
- RISK-VALUE: NAMED, NOT DERIVED — DefaultTTL = 2 * time.Minute @ comms/contract/identity.go:88 — unchanged by the extraction, as the brief requires; whether two minutes is the right assertion lifetime belongs to the brief that introduced it and is not derived here.
- RISK-VALUE: NAMED, NOT DERIVED — DefaultSkew = 1 * time.Minute @ comms/contract/identity.go:93 — unchanged by the extraction; the right clock-skew tolerance is not derived here for the same reason.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractCompatibility$' > ${TMPDIR:-/tmp}/comms-core-01-flow.log 2>&1 && grep -F -- '--- PASS: TestContractCompatibility ' ${TMPDIR:-/tmp}/comms-core-01-flow.log` | pass exit=0 | sha256:e879805da36d | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestContractDependencyBoundary$' > ${TMPDIR:-/tmp}/comms-core-01-boundary.log 2>&1 && grep -F -- '--- PASS: TestContractDependencyBoundary ' ${TMPDIR:-/tmp}/comms-core-01-boundary.log` | fail exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./internal/comms/... ./internal/commsqueue/... ./cmd/commsgw/... ./cmd/commsloop/... ./cmd/deskcomms/...)` | fail exit=1 | sha256:40c42cb64298 | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test -count=1 -v ./internal/... -run '^TestPolicyAdapterRefusesForbiddenMessage$' > ${TMPDIR:-/tmp}/comms-core-01-policy.log 2>&1 && grep -F -- '--- PASS: TestPolicyAdapterRefusesForbiddenMessage ' ${TMPDIR:-/tmp}/comms-core-01-policy.log` | pass exit=0 | sha256:3c0007441c51 | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5197 run=6dd744216345ae7ad5adda2d2f4aee68ee1afc55fe312eea00ce45d7b5277466 source=1bfcf9a4dcb5689ed5c9172e35abf95fc2241a88 brief=docs/streams/comms-core/brief-01-extract-the-comms-contract-and-isolate-assay-policy.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — rows 2 and 3 exit 1; rows 1, 4 and 5 exit 0. Fresh independent run at source 1bfcf9a4dcb5 (unchanged under comms/ and tools/desk/internal/comms since 5fdae0d07b58), Go 1.27.2 darwin/arm64, witness written by statusgen verifyrun v1.0.67 on the host (every row is class check, none check:ci, so no container bracket applies), TMPDIR set to a scratch directory outside the home and outside any Go module. Evidence only: gate human, sensitive-data risk; lifecycle left at its current state.

Failure detail (observed this run). Row 2: TestContractDependencyBoundary fails at comms/contract/deps_test.go line 108, "a directory with no module listed cleanly — a missing module must fail the row, not count zero"; all seven planted-import subtests (net, net/http, os/exec, tools/desk, deskkit, forgeread, third party) PASS. The failing assertion is the missing-module negative control: it runs go list -e -deps ./... in an empty temporary directory and expects an error, but under Go 1.27.2 that command exits 0 and prints the pseudo-package ./... (probe below). Row 3 fails on the same test in its comms half, and the and-chain then short-circuits the tools/desk half. Class: implementation defect in the delivered test's negative control, not an environment limit; it fails with any TMPDIR that is not below an unrelated go.mod, including the default /tmp.

Supplemental observations (no acceptance credit; 2026-11-01; runner assay-verifier-app[bot], model claude-opus-5-5; run in a git-archive copy of 1bfcf9a4dcb5 outside the home):

- Row 3 comms half alone (cd comms, GOWORK=off go test -count=1 ./...): exit 1, same line-108 failure.
- Row 3 tools/desk half alone with row 3's exact package list: exit 0, all five packages ok, with a short TMPDIR. With the long scratch TMPDIR the commsgw and deskcomms packages fail on unix-socket bind (invalid argument, the darwin socket-path length limit); that is a probe environment limit, not a finding.
- go list -e -deps ./... in an empty directory outside any module: exit 0, prints ./...
- Row 5 mutation: policy check in Admit (tools/desk/internal/comms/envelope.go line 130) disabled with a false guard; the row 5 test exits 1, four policy-forbidden fixtures report "want ErrLaneForbidden", zero PASS lines. The mutation obligation discriminates. Mutation reverted in the copy.

Risk-bearing values (risk sensitive-data yes, irreversible no). Enumeration covered the non-test Go files under comms/contract and the changed alias layer in tools/desk/internal/comms from the delivering merge 2cb2b41f02e6, compared against its first parent d71c48d61886; laneacl.go is unchanged by that merge. Literals: Schema = "cellmsg-v1" @ comms/contract/envelope.go:14; assertion domain tag "comms/assertion/cellmsg-v1\n" @ comms/contract/identity.go:258; DefaultTTL = 2 * time.Minute @ comms/contract/identity.go:88; DefaultSkew = 1 * time.Minute @ comms/contract/identity.go:93; MaxEnvelopeBytes = 64 * 1024 @ comms/contract/envelope.go:23; MaxPayloadBytes = 16 * 1024 @ :25; MaxFieldBytes = 256 @ :27; MaxRefs = 32 @ :29; MaxRefBytes = 256 @ :31; CapDeliveryToken = "delivery-token/v1" @ comms/contract/types.go:211; CapClientIdentity = "client-identity/v1" @ comms/contract/types.go:216; ErrorCode strings oversize, malformed, bad_schema, absent_triple, unknown_verb, unknown_class, unknown_cell, bad_signature, expired, not_yet_valid, replay, identity_mismatch, mint_input, unclassified @ comms/contract/types.go:81-97; Stage strings accepted, refused, held, delivered, acked @ comms/contract/types.go:164-168. The first nine are byte-identical to the pre-extraction literals in tools/desk/internal/comms at d71c48d61886. Ranked: wire schema and assertion domain tag first (a change strands every signed or persisted cellmsg-v1 message), then the assertion validity window, then the new public error/stage/capability names (a rename breaks importers but is reversible by a versioned edit), then the size caps (reversible operational bounds, no derivation required).

- RISK-VALUE: DERIVED — Schema = "cellmsg-v1" @ comms/contract/envelope.go:14 — identical to the pre-extraction literal at d71c48d61886 (tools/desk/internal/comms envelope.go line 15); the brief keeps the cellmsg-v1 wire unchanged.
- RISK-VALUE: DERIVED — assertion domain tag = "comms/assertion/cellmsg-v1\n" @ comms/contract/identity.go:258 — identical to the pre-extraction literal at the same line at d71c48d61886; the brief requires unchanged assertion bytes, and row 1's assertion-byte fixtures pass this run.
- RISK-VALUE: NAMED, NOT DERIVED — DefaultTTL = 2 * time.Minute @ comms/contract/identity.go:88 — unchanged by the extraction as the brief requires; whether two minutes is the right assertion lifetime belongs to the brief that introduced it.
- RISK-VALUE: NAMED, NOT DERIVED — DefaultSkew = 1 * time.Minute @ comms/contract/identity.go:93 — unchanged by the extraction; the right clock-skew tolerance is not derived here for the same reason.

## Review

Gate: human. Confirm the compatibility report, actual served/direct bypass cases, and
that no new identity grant, full-mode activation or broker switch was hidden in extraction.
