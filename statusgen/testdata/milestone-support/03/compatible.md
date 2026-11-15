---
schema: milestone-v1
milestone: assay:assay:delivery/m-01
contract-version: 1
title: "Runner capability is available"
owner: platform maintainers
sources: ["docs/spec.md#runner"]
contributes: []
requires-milestones: []
---

## Outcome

Builds run on the shared runner that already exists.

## Supported scope

Default runner only.

## Acceptance

| ID | Condition | Method | Verifier |
|---|---|---|---|
| C01 | A build runs on the shared runner | existing run record | independent verifier |

## Coverage

| Criterion | Contributors | Evidence |
|---|---|---|
| C01 | — | `docs/evidence/runner.md` |

## Prerequisites

None.

## Acceptance authority

The stream's existing decision record names the accepting maintainer.

## Change and open decisions

Existing capability; the empty contribution list is justified by the run record above.
