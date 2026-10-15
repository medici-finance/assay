---
brief: assay:assay:ci-tooling:04
title: "Pinned installation and check actions"
why: "Repeated workflow shell steps recreate installation and result handling. Fixed-purpose pinned actions let consumers adopt one tested implementation without copying logic."
wave: 4
depends: ["ci-tooling/03"]
unblocks: ["ci-tooling/05"]
effort: "M"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "09f263c9-f08d-4e8b-99f9-c2557676fb9d"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/24-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/actions/install-tools/: follow-up ci-tooling/04", "ci/actions/claims-check/: follow-up ci-tooling/04", "ci/actions/closure-check/: follow-up ci-tooling/04", "ci/tests/actions_contract.py: follow-up ci-tooling/04", "ci/README.md: follow-up ci-tooling/04", "ci/staged-workflows/ci.yml: follow-up ci-tooling/04"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 04 — Pinned installation and check actions

## Context

files: `ci/actions/install-tools/` (planned), `ci/actions/claims-check/` (planned), `ci/actions/closure-check/` (planned), `ci/tests/actions_contract.py` (planned), `ci/README.md` (planned), `ci/staged-workflows/ci.yml` (planned), `changelog/ci-tooling-04.md` (planned)

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

## Human decision

The proposed migration changes a boundary used to run checks or keep private material local.
Approve the reviewed scope only after the implementation identifies actual versions,
consumer coverage and negative-control results. Approval does not expand credentials.

Options:
1. **Approve the qualified cutover** — retain current authority and use the tested rollback combination.
2. **Hold the cutover** — keep the supported implementation until the missing proof is supplied.

Default if no answer: none — activation blocks until answered.

## Ground rules

- Author staged workflows only; no workflow activation, credential expansion or live infrastructure probes.
- Preserve historical Evidence and independent controls. Stop at implementation; do not assert verified/done.
- Missing publication, release or consumer evidence is could-not-check, never permission to delete source.

## Task

1. Implement the three actions specified in spec.md; keep source under ci/actions and invocation relative to github.action_path. External consumer roots are explicit inputs. Document exact action inputs, outputs, supported platforms and compatible binary/library versions.
2. Require explicit version and platform checksum, verify before execution, and refuse absent/mismatched pins without fallback. Use the existing verified installer if compatible; factor it instead of maintaining a competing implementation.
3. Keep credentials job-owned and narrowly scoped. No arbitrary command input, token minting, private PATH fallback or privileged execution of untrusted candidate code. Preserve check mode/outcome separation and escape annotation text.
4. Use synthetic local assets and a throwaway external consumer to prove the real action step flow; test checksum failure before invocation, missing files, malicious annotation content, unsupported platform and non-root checkout. Stage the Assay self-test workflow; activation remains separate.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `python3 ci/tests/actions_contract.py` | exit 0; ACTIONS PASS; actual steps reject every specified negative case | check +flow +dereference |
| 2 | `(cd ci && GOWORK=off go test ./report/... ./invocation/...)` | exit 0; output and exit contracts pass | check |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
