---
brief: assay:assay:comms-core:03
title: Extract the client SDK and make deskcomms an adapter
why: "Clients currently depend on command-local wire and transport code. A narrow SDK lets ordinary tools use the service without importing server authority or queue internals."
wave: 2
depends: ["comms-core/02"]
unblocks: ["desktools-v2/42"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Clients currently depend on command-local wire and transport code. A narrow SDK lets ordinary tools use the service without importing server authority or queue internals."
issues: []
schema: brief-v2
authored: 2026-10-13 by statusgen newbrief
sources:
  - "docs/proposals/non-ci-architecture/20-communications.md"
  - "freshness-checked 2026-10-13 at a435badbd1d245402407acd2a82a6abe5f37f637 after origin/main fetch"
version: 1
id: b5393e24-6e64-4114-971f-3d881d7241e2
decision-trigger: spec
exec-tier: strong
exec-tier-why: "Cross-package authority and compatibility must agree through real consumer flows."
domain: complicated
consumers:
  - "tools/desk/cmd/deskcomms: fixed-here"
  - "comms/client/testdata: fixed-here"
  - "tools/desk/cmd/cellctl: out-of-scope (existing invocation remains compatible)"
  - "desktools-v2/42 CLI adapters: follow-up desktools-v2/42"
---

# Brief 03 — Extract the client SDK and make deskcomms an adapter

## Context

files:
- `comms/client (new)`
- `comms/client/testdata (new external consumer fixture)`
- `tools/desk/cmd/deskcomms`
- `tools/desk/internal/commstransport`
- `tools/desk/go.mod`
- `tools/desk/go.sum`
- `tools/desk/scripts/test-comms-consumer.sh (new)`
- `docs/cellctl-comms.md`
- `docs/library-first.md`
- `changelog/comms-client.md` (planned)

facts:
- At a435badbd, deskcomms/gateway.go contains request/response and transport implementation in package main. tools/desk/go.mod requires a placeholder forgeread version and a local replace; external packaging must be demonstrated, not presumed.
- Source snapshot: 2026-10-13, a435badbd1d245402407acd2a82a6abe5f37f637. Refresh code, claims and PRs before dispatch; no implementation is claimed here.

layering: comms/client; CLI retains argument rendering and Assay preflight only.
single-point-of-failure: service admission remains authoritative; client preflight is advisory, and existing transport/custody and effect-boundary controls remain independent checks.
design-fit:
  owner: comms/client; CLI retains argument rendering and Assay preflight only
  contract: docs/proposals/non-ci-architecture/20-communications.md
  retires: command-local duplicated or unimportable implementation only after callers move
  weight: no additional daemon or broker; narrow public packages and compatibility fixtures
  why-add: reuse the existing implementation across clients/hosts without exporting authority

## Read first

- [Communications target](../../proposals/non-ci-architecture/20-communications.md).
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

1. Extract typed Send/Poll/Ack and local transport into comms/client over the contract from 01 and service from 02. Expose cancellation, typed refusal/unknown outcomes and explicit configuration. Preserve existing signing and peer-custody checks.
2. Make deskcomms a thin client with the existing command syntax, exit codes and audit behavior. Mandatory authority remains on the service even if client preflight is bypassed. Do not perform the Cobra migration owned by desktools-v2/42.
3. Keep the client dependency closure free of server/backend packages, deskkit credentials, environment discovery and executors. Client credentials are explicit bounded caller inputs; never inherit gateway or operator credentials.
4. Add an external consumer fixture building with GOWORK=off, no relative replace and no repository-local import shortcuts. Use an offline file module proxy for the comms module and any required dependencies. The client-only consumer must resolve comms without tools/desk or its forgeread dependency. Prove imports compile and the consumer sends/polls/acks against a disposable service; a disallowed API/import negative fixture must fail. This is packaging qualification, not a claim that release pins exist.
5. Compare actual CLI and direct SDK flows for allowed, refused, peer-impersonation, timeout and cancellation cases. Document supported wire/API versions and release prerequisites; no Rust SDK or new protocol is required.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestClientCLIParity$' > /tmp/comms-core-03-flow.log 2>&1 && grep -F -- '--- PASS: TestClientCLIParity ' /tmp/comms-core-03-flow.log` | exit 0; named test runs and compares the specified end-to-end and legacy fixtures |
| 2 | check +flow | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestClientDependencyBoundary$' > /tmp/comms-core-03-boundary.log 2>&1 && grep -F -- '--- PASS: TestClientDependencyBoundary ' /tmp/comms-core-03-boundary.log` | exit 0; named test runs; forbidden authority/import/custody paths refuse, including lower-layer bypass cases |
| 3 | check | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./cmd/deskcomms/... ./internal/commstransport/...)` | exit 0; existing and moved tests pass |
| 4 | check | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | exit 0; Windows compile/vet only, not a native-runtime witness |
| 5 | check +flow | `bash tools/desk/scripts/test-comms-consumer.sh` | exit 0; independent module positive build and flow pass, prohibited import/API fixture fails, no external network or workspace dependency |

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
