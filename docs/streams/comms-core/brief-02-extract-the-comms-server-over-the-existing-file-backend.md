---
brief: assay:assay:comms-core:02
title: Extract the comms server over the existing file backend
why: "A second service host cannot reuse package-main gateway logic. Extract the server and its existing file backend once so standalone and later composed hosts enforce the same rules."
wave: 1
depends: ["comms-core/01"]
unblocks: ["comms-core/03", "comms-core/04", "desktools-v2/42"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The extraction moves admission, mailbox custody and drain behavior behind one authoritative path. A human confirms that no persisted-state, identity or activation change is hidden in the move."
issues: []
schema: brief-v2
outcome: none
authored: 2026-10-13 by statusgen newbrief
sources:
  - "docs/proposals/non-ci-architecture/21-communications.md"
  - "freshness-checked 2026-10-13 at a435badbd1d245402407acd2a82a6abe5f37f637 after origin/main fetch"
version: 1
id: ca4dd298-1734-40bd-88cd-63ba5c34f5cf
decision-trigger: spec
exec-tier: strong
exec-tier-why: "Cross-package authority and compatibility must agree through real consumer flows."
domain: complicated
consumers:
  - "tools/desk/cmd/commsgw: fixed-here"
  - "tools/desk/cmd/commsloop: fixed-here"
  - "tools/desk/cmd/deskcomms: follow-up comms-core/03"
  - "comms/config: follow-up comms-core/04"
  - "deskd service host: out-of-scope (later integration after host and custody qualification)"
---

# Brief 02 — Extract the comms server over the existing file backend

## Context

files:
- `comms/server (new)`
- `comms/backend/file (new)`
- `tools/desk/internal/commsqueue`
- `tools/desk/internal/commstransport`
- `tools/desk/internal/commspolicy`
- `tools/desk/cmd/commsgw`
- `tools/desk/cmd/commsloop`
- `comms/server/testdata (new legacy-state and host fixtures)`
- `docs/contracts.md`
- `docs/cellctl-comms.md`
- `changelog/comms-server.md` (planned)

facts:
- At a435badbd, commsgw owns admission/listeners and commsloop owns drain/routing; commsqueue defines shared file state and also holds a forge-reaching quarantine filer (`tools/desk/internal/commsqueue/quarantine.go`: `DeskfileIssueFiler`, and after #2731 `internal/deskkit`'s `FileDeskIssue`), which is a second coupling beside laneacl→deskkit. Native dispatch and role-bound effects are coupled to the command composition. Refactoring must not enable them.
- Source snapshot: 2026-10-13, a435badbd1d245402407acd2a82a6abe5f37f637. Refresh code, claims and PRs before dispatch; no implementation is claimed here.

layering: comms/server for message use cases, file backend for transition state; existing runtime owner retains native dispatch.
single-point-of-failure: service admission remains authoritative; client preflight is advisory, and existing transport/custody and effect-boundary controls remain independent checks.
design-fit:
  owner: comms/server for message use cases, file backend for transition state; existing runtime owner retains native dispatch
  contract: docs/proposals/non-ci-architecture/21-communications.md
  retires: command-local duplicated or unimportable implementation only after callers move
  weight: no additional daemon or broker; narrow public packages and compatibility fixtures
  why-add: reuse the existing implementation across clients/hosts without exporting authority

The L scope is intentional: producer and drain must move against one file-state contract
in the same change to avoid incompatible queue writers/readers. No broker integration is included.

## Read first

- [Communications target](../../proposals/non-ci-architecture/21-communications.md).
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

1. Extract Send/Poll/Ack use cases and explicit authentication, authorization, filtering and backend ports. Require policy at construction; never default missing policy to allow. Trusted transport supplies caller identity.
2. Move the generic file-queue code (accepted/held records, mailbox, acknowledgments) behind comms/backend/file without changing persisted state or advertising broker-grade durability. `tools/desk/internal/commsqueue/quarantine.go` is not generic: `DeskfileIssueFiler` shells out to `deskfile`, and `FileDeskIssue` imports `internal/deskkit`. Keep both in `tools/desk` behind a quarantine-filing port that the server calls and the Assay composition satisfies; the public backend must not import them. Move messaging filter/drain orchestration into reusable service code; invoke the existing Assay policy adapter. Keep native dispatch and forge effects outside the public library.
3. Make commsgw a composition/listener host and commsloop a supervised adapter; preserve local socket/pipe custody, A2A mTLS admission, dedupe, rate limits, stop checks, held records and mailbox ownership. Both listeners must call the same authoritative service path.
4. Run old persisted-state fixtures through the extracted backend. An independently constructed host fixture must send, filter, poll and acknowledge. Bypass the CLI and mounted listener in negative cases; missing authorizer, forged identity, unauthorized mailbox and filter failure must refuse or hold.
5. Preserve all native-dispatch safety tests at the Assay boundary. No broker, network forwarding, new identity grants, full-mode enablement or real deskd integration belongs in this extraction. Document public host construction and remaining deployment limits.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestServerCompatibilityFlow$' > ${TMPDIR:-/tmp}/comms-core-02-flow.log 2>&1 && grep -F -- '--- PASS: TestServerCompatibilityFlow ' ${TMPDIR:-/tmp}/comms-core-02-flow.log` | exit 0; `TestServerCompatibilityFlow` replays legacy accepted/held/acked file-state fixtures through `comms/server` over `comms/backend/file`, then an independently constructed host fixture sends, filters, polls and acknowledges with results equal to the legacy ones |
| 2 | check +flow | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestServerBypassRefused$' > ${TMPDIR:-/tmp}/comms-core-02-boundary.log 2>&1 && grep -F -- '--- PASS: TestServerBypassRefused ' ${TMPDIR:-/tmp}/comms-core-02-boundary.log` | exit 0; `TestServerBypassRefused` calls the service directly (no CLI, no listener): a missing authorizer, forged identity, unauthorized mailbox and a failing filter each refuse or hold |
| 3 | check | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./cmd/commsgw/... ./cmd/commsloop/... ./internal/commsqueue/... ./internal/commstransport/...)` | exit 0; existing and moved tests pass |
| 4 | check | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | exit 0; Windows compile/vet only, not a native-runtime witness |
| 5 | check +flow | `cd tools/desk && GOWORK=off go test -count=1 -v ./cmd/commsgw -run '^TestGatewayListenersShareServicePath$' > ${TMPDIR:-/tmp}/comms-core-02-mounted.log 2>&1 && grep -F -- '--- PASS: TestGatewayListenersShareServicePath ' ${TMPDIR:-/tmp}/comms-core-02-mounted.log` | exit 0; the test runs in the module that owns the gateway host: the local socket/pipe and A2A listeners both reach the same service path, and the quarantine-filing port is satisfied by the Assay adapter, not by the public module |

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
