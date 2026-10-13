---
stream: comms-core
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: [2740]
board: generated
---

# Reusable communications foundation

Design and extraction work; no implementation or activation claimed. The
[future-state specification](../../proposals/non-ci-architecture/20-communications.md)
owns the target and separates initial compatibility refactors from broker migration.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Extract the comms contract and isolate Assay policy](brief-01-extract-the-comms-contract-and-isolate-assay-policy.md) | 0 | M | todo | — | — |
| 02 | [Extract the comms server over the existing file backend](brief-02-extract-the-comms-server-over-the-existing-file-backend.md) | 1 | L | todo | — | — |
| 03 | [Extract the client SDK and make deskcomms an adapter](brief-03-extract-the-client-sdk-and-make-deskcomms-an-adapter.md) | 2 | M | todo | — | — |
| 04 | [Unify comms configuration and extend cellctl management](brief-04-unify-comms-configuration-and-extend-cellctl-management.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

The library is the standalone Go module `github.com/medici-finance/assay/comms`
at repository-root `comms/`. Desk-tools consumes it; generic comms never imports desk-tools.

## Critical path and waves

Contract/policy extraction (01) → server extraction (02) → client extraction (03).
Configuration (04) follows 02 and the existing cellctl CLI owner, desktools-v2/16.
The CLI migration desktools-v2/42 follows 02 and 03 plus its existing dependency.
These are code dependencies, not a requirement to await a broker decision or deskd rewrite.
Wave numbers are derived from the existing dependency graph.

The actual first blocker is Go's internal-package boundary and policy coupling:
internal/comms imports deskkit, and gateway/client implementations live in package main.
Extract those seams with the old behavior qualified first. Do not publish internal/deskkit.

## Later delivery

Broker qualification, one selected broker adapter, deskd composition, and state migration
are subsequent bounded briefs to author against the specification's acceptance cases.
They are not implied by completing these four extraction briefs. No broker,
new trust model, native session activation, or supported release is selected here.

Before dispatch refresh source, claims and open PRs; the inventory is a 2026-10-13 snapshot.
An already-running change needs an explicit checkpoint/reconciliation, not silent changes
of its scope. Keep urgent supported-path fixes independent and port their regressions.
