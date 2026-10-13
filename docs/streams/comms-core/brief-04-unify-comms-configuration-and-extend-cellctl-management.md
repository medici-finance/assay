---
brief: assay:assay:comms-core:04
title: Unify comms configuration and extend cellctl management
why: "Cellctl already manages a comms manifest, while gateway and drain parse their own inputs. Shared validation and explicit settings operations prevent those views drifting as service hosts multiply."
wave: 3
depends: ["comms-core/02", "desktools-v2/16"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Cellctl already manages a comms manifest, while gateway and drain parse their own inputs. Shared validation and explicit settings operations prevent those views drifting as service hosts multiply."
issues: []
schema: brief-v2
authored: 2026-10-13 by statusgen newbrief
sources:
  - "docs/proposals/non-ci-architecture/20-communications.md"
  - "freshness-checked 2026-10-13 at a435badbd1d245402407acd2a82a6abe5f37f637 after origin/main fetch"
version: 1
id: 239bce22-fd9c-4978-afb6-66361945f001
decision-trigger: spec
exec-tier: strong
exec-tier-why: "Cross-package authority and compatibility must agree through real consumer flows."
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl: fixed-here"
  - "tools/desk/cmd/commsgw: fixed-here"
  - "tools/desk/cmd/commsloop: fixed-here"
  - "docs/cellctl-comms.md: fixed-here"
---

# Brief 04 — Unify comms configuration and extend cellctl management

## Context

files:
- `comms/config (new)`
- `tools/desk/cmd/cellctl/comms.go`
- `tools/desk/cmd/cellctl/comms_init.go`
- `tools/desk/cmd/cellctl/comms_startup.go`
- `tools/desk/cmd/cellctl comms/config tests and CLI command registration`
- `tools/desk/cmd/commsgw/config.go`
- `tools/desk/cmd/commsloop/main.go`
- `comms/server`
- `docs/cellctl-comms.md`
- `changelog/comms-config.md` (planned)

facts:
- At a435badbd, cellctl owns init/check/run, CELL_COMMS_CONFIG and the private manifest. init already provisions local material. The documented profile is interim; credentials are path references with platform-specific custody checks.
- Source snapshot: 2026-10-13, a435badbd1d245402407acd2a82a6abe5f37f637. Refresh code, claims and PRs before dispatch; no implementation is claimed here.

layering: comms/config for pure schema and resolved values; cellctl for operator effects.
single-point-of-failure: service admission remains authoritative; client preflight is advisory, and existing transport/custody and effect-boundary controls remain independent checks.
design-fit:
  owner: comms/config for pure schema and resolved values; cellctl for operator effects
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

1. Extract a versioned shared schema and pure validation/resolution result recording provenance. Preserve current manifest/environment precedence and explicit-empty behavior via a fixture matrix; distinguish missing, invalid, disabled and unsupported modes.
2. Extend the existing cellctl comms command group with config show, validate and set. Default show redacts sensitive values and exposes references/provenance only. validate is offline; diagnostics contact an endpoint only with explicit opt-in naming it before contact.
3. Set validates the entire candidate and uses a same-directory atomic replacement under existing custody/ownership checks. Unknown keys, unsupported modes and key material inline or on argv refuse. Preserve POSIX and Windows custody behavior and current init platform restrictions.
4. Make gateway/drain composition consume the same resolved values through an explicit adapter; no implicit repository config search. Persisting settings does not start/reload a service, mint new capabilities or enable full mode. Report the pending-versus-running revision and require an explicit operator restart/apply using existing lifecycle ownership.
5. Keep native session activation and any revised ceiling under the existing mode-enforcement owner and human decision. This brief supplies reusable configuration, not that decision. Update help and host-setup examples with actual commands and unchanged defaults.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestCommsConfigFlow$' > /tmp/comms-core-04-flow.log 2>&1 && grep -F -- '--- PASS: TestCommsConfigFlow ' /tmp/comms-core-04-flow.log` | exit 0; named test runs and compares the specified end-to-end and legacy fixtures |
| 2 | check +flow | `cd comms && GOWORK=off go test -count=1 -v ./... -run '^TestCommsConfigCustodyAndNoActivation$' > /tmp/comms-core-04-boundary.log 2>&1 && grep -F -- '--- PASS: TestCommsConfigCustodyAndNoActivation ' /tmp/comms-core-04-boundary.log` | exit 0; named test runs; forbidden authority/import/custody paths refuse, including lower-layer bypass cases |
| 3 | check | `(cd comms && GOWORK=off go test -count=1 ./... && cd ../tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl/... ./cmd/commsgw/... ./cmd/commsloop/...)` | exit 0; existing and moved tests pass |
| 4 | check | `(cd comms && GOWORK=off GOOS=windows go vet ./... && cd ../tools/desk && GOWORK=off GOOS=windows go vet ./cmd/deskcomms ./cmd/commsgw ./cmd/commsloop)` | exit 0; Windows compile/vet only, not a native-runtime witness |

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
