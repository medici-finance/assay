---
brief: assay:assay:ci-tooling:02
title: "Assay-wide result and repository input libraries"
why: "Repeated coverage and file-reading machinery can disagree about unexamined input. Narrow shared packages let independent consumers retain the same uncertainty and provenance semantics."
wave: 2
depends: ["ci-tooling/01"]
unblocks: ["ci-tooling/03"]
effort: "M"
gate: "model"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "0c8692b6-bc95-4eeb-b957-efebf6c4ef44"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/25-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["core/: follow-up ci-tooling/02", "ci/tests/external_core.py: follow-up ci-tooling/02", "docs/contracts.md: follow-up ci-tooling/02", "docs/library-first.md: follow-up ci-tooling/02", "ci/staged-workflows/release.yml: follow-up ci-tooling/02"]
---

# Brief 02 — Assay-wide result and repository input libraries

## Context

files: `core/` (planned), `ci/tests/external_core.py` (planned), `docs/contracts.md`, `docs/library-first.md`, `ci/staged-workflows/release.yml`, `changelog/ci-tooling-02.md` (planned)

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

1. First record, in the inventory, what `deskcore/facts` (versioned bundle, unknown-schema refusal, policy digest, completeness) already serves. Create core/go.mod and core/checkresult only for the versioned typed results, input/policy digests, findings and explicit coverage that CI consumers cannot take from `deskcore` without importing desk-domain types; reuse rather than duplicate. The result package is pure and does not map advisory policy to a clean verdict. Keep existing statusgen/evaluation and forgeread owners unchanged.
2. Create core/repofiles only for common mechanics demonstrated by at least two consumers in the inventoried check cohort. Callers choose tracked/worktree/artifact selection, globs, exclusions, symlinks and bounds. Preserve different selection contracts; record unavailable files instead of silently dropping them.
3. Add transitive import checks, extending `deskcore/archtest` where it applies rather than writing a second closure checker: result types cannot reach filesystem, environment, clock, subprocess, network, credentials, CI or services. Repofiles cannot import custody or forge effects. Prove deterministic and adversarial path/empty/unreadable input cases.
4. Qualify external consumers with GOWORK=off and a local module proxy without replace. Stage release-matrix wiring that emits a `core/vX.Y.Z` module tag (the release process owns tagging, per the spec); publish only after the release owner's separate authorization. In-tree consumers use a relative `replace` until then. Record the real resolvable module version before any adopter cutover.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd core && GOWORK=off go test ./...)` | exit 0; purity and repository-input suites pass | check |
| 2 | `python3 ci/tests/external_core.py` | exit 0; EXTERNAL CORE PASS; no workspace/replace/private source | check +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: model. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 2 is the negative-control row (external resolution with no workspace, `replace`
or private source); row 1 runs the purity and adversarial-input suites; row 3 covers
consumer routing. Publication adequacy and retained authority also require review.
