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
- `tools/desk/internal/commspolicy (new composition adapter if needed)`
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

Default if no answer: none — implementation approval remains pending.

## Ground rules

Feature branch and draft PR only; no live infrastructure, activation, release or merge.
Do not alter completed Evidence or claim verification as the implementer. Keep supported-path
fixes independently shippable and preserve their tests when moving code. Proposed paths and
test names below are deliverables, not claims they already exist.

## Task

1. Create the root `comms/` module with module path `github.com/medici-finance/assay/comms`; it must not import desk-tools, including transitively. Wire desk-tools as a versioned consumer and qualify both modules with GOWORK=off. Inventory the exported contract and all importers. Move structural types and parsing primitives into comms/contract. Keep Assay verb/class/role validation and compiled ACL in the Assay adapter; do not widen acceptance when separating structural parsing from policy.
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
| 5 | check +flow | `cd tools/desk && GOWORK=off go test -count=1 -v ./internal/comms/... ./internal/commspolicy/... -run '^TestPolicyAdapterRefusesForbiddenMessage$' > ${TMPDIR:-/tmp}/comms-core-01-policy.log 2>&1 && grep -F -- '--- PASS: TestPolicyAdapterRefusesForbiddenMessage ' ${TMPDIR:-/tmp}/comms-core-01-policy.log` | exit 0; the test runs in the module that owns the Assay policy adapter: a structurally valid but policy-forbidden message is refused through the extracted contract, and the same test fails when the policy call is removed (mutation) |

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

## Review

Gate: human. Confirm the compatibility report, actual served/direct bypass cases, and
that no new identity grant, full-mode activation or broker switch was hidden in extraction.
