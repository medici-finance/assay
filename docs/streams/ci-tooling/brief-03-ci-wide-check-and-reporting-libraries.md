---
brief: assay:assay:ci-tooling:03
title: "CI-wide check and reporting libraries"
why: "Commands currently bind check logic to presentation and process exits. Reusable checks and CI adapters allow the same rules to serve existing binaries, actions and release composition."
wave: 3
depends: ["ci-tooling/02"]
unblocks: ["ci-tooling/04", "ci-tooling/05"]
effort: "L"
gate: "model"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "93d388ee-2c34-49ce-8f36-2bf165188b33"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/24-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/go.mod: follow-up ci-tooling/03", "ci/checks/: follow-up ci-tooling/03", "ci/report/: follow-up ci-tooling/03", "ci/invocation/: follow-up ci-tooling/03", "ci/tests/library_contract.py: follow-up ci-tooling/03", "docs/contracts.md: follow-up ci-tooling/03", "docs/library-first.md: follow-up ci-tooling/03", "ci/staged-workflows/release.yml: follow-up ci-tooling/03"]
---

# Brief 03 — CI-wide check and reporting libraries

## Context

files: `ci/go.mod` (planned), `ci/checks/` (planned), `ci/report/` (planned), `ci/invocation/` (planned), `ci/tests/library_contract.py` (planned), `docs/contracts.md`, `docs/library-first.md`, `ci/staged-workflows/release.yml`, `changelog/ci-tooling-03.md` (planned)

facts:
- Source inspection: 2026-10-13; public Assay base 4696d598e. Refresh inventory and consumer pins at pickup.
- The stream spec defines the two library layers, private namespace and staged/released/adopted distinction.
- This is an authored migration obligation; the named new tests and paths are deliverables, not existing capabilities.

layering: Keep domain checks explicit-input and shared mechanisms narrow; process/provider and CI presentation adapters stay outside pure evaluators. Task and Verify below exercise the complete consumer flow.

design-fit:
  owner: CI tool family and the existing domain owners named in the spec
  contract: none — this draft proposes the packaging/check-result seam; existing S-eligibility and S-evidence-coverage remain unchanged
  retires: [duplicated adapters or obsolete paths named in this brief after consumer proof]
  weight: verbs +0, flags +0, refusals +0 intended externally; rule-text lines added for the bounded migration contract
  why-add: a source move alone cannot preserve pin, publication and workflow authority; this contract assigns the retirement proof to its owner

## Read first

- [Stream spec](spec.md) and [dependency map](README.md).
- Public Assay `docs/contracts.md` and `docs/library-first.md`; do not introduce a rival semantic owner.

## Ground rules

- Author staged workflows only; no workflow activation, credential expansion or live infrastructure probes.
- Preserve historical Evidence and independent controls. Stop at implementation; do not assert verified/done.
- Missing publication, release or consumer evidence is could-not-check, never permission to delete source.

## Task

1. Create the ci module with checks/claims and checks/closure, importing core result types. Obtain only independently publication-cleared source exports and synthetic fixtures; never require a private checkout to build the public module.
2. Expose explicit policy/snapshot inputs and a resolver interface for closure observations. Distinguish advisory exit zero from could-not-check; preserve existing flags through adapters and record deliberate output changes as versioned compatibility deltas.
3. Create report/invocation packages for structured output, text/Actions presentation and policy-to-exit mapping. Escape untrusted annotation text. Share no arbitrary shell command runner or credential resolver. Import-closure tests prevent effects in pure checks.
4. Qualify release-check composition through the typed claims API and a synthetic external consumer. Preserve process-isolated callers when independently justified. Require library/CLI fixture parity including missing data and approved diagnostic changes. Stage release wiring and record published module versions before external adoption.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd ci && GOWORK=off go test ./checks/... ./report/... ./invocation/...)` | exit 0; positive, incomplete-input and escaping cases pass | check |
| 2 | `python3 ci/tests/library_contract.py` | exit 0; CI LIBRARIES PASS; external composition and rejected imports demonstrated | check +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: model. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
