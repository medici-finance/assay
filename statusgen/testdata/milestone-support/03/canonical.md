---
schema: milestone-v1
milestone: assay:assay:delivery:m-03
contract-version: 1
title: "Build changes are less brittle"
owner: build maintainers
sources: ["docs/spec.md"]
contributes: ["assay:assay:delivery:b-11", "assay:assay:delivery:b-12@2"]
requires-milestones: ["delivery:m-01@1"]
---

# Build changes are less brittle

## Outcome

A maintainer can change a build rule without editing unrelated pipelines.

## Supported scope

The repository's own build on the default runner. Excludes release packaging.

## Acceptance

| ID | Condition | Method | Verifier |
|---|---|---|---|
| C01 | A rule change touches one file | Verify row 1 of brief 11 | independent verifier |
| C02 | Failing rules name their owner | Verify row 2 of brief 12 | independent verifier |

## Coverage

| Criterion | Contributors | Evidence |
|---|---|---|
| C01 | `assay:assay:delivery:b-11` | `docs/evidence/c01.md` |
| C02 | `assay:assay:delivery:b-12` | — |

## Prerequisites

`delivery:m-01@1` supplies the runner capability; needed for final acceptance only.

## Acceptance authority

The stream's existing decision record names the accepting maintainer.

## Change and open decisions

Version 1. No open decisions.
