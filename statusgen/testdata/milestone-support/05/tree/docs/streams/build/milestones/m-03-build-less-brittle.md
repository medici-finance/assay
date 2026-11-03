---
schema: milestone-v1
milestone: assay:assay:build:m-03
contract-version: 1
title: "Build changes are less brittle"
owner: fixture maintainers
sources: ["docs/spec.md"]
contributes: ["assay:assay:build:b-11", "assay:assay:build:b-12@2"]
requires-milestones: ["stream-y:m-01@1", "stream-z:m-02@1"]
---

# Build changes are less brittle

## Outcome

The fixture outcome for build m-03.

## Supported scope

The fixture tree only.

## Acceptance

| ID | Condition | Method | Verifier |
|---|---|---|---|
| C01 | The contributing work is done | Verify row 1 of each contributor | independent verifier |

## Coverage

| Criterion | Contributors | Evidence |
|---|---|---|
| C01 | `assay:assay:build:b-11`, `assay:assay:build:b-12` | `docs/evidence/c01.md` |

## Prerequisites

`stream-y:m-01@1` is a start requirement of brief 12 only; `stream-z:m-02@1` is needed for final acceptance only.

## Acceptance authority

The stream's existing decision record names the accepting maintainer.

## Change and open decisions

Version 1. No open decisions.
