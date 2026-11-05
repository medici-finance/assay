---
brief: vc/01
title: v1 corpus brief 01
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-25 by statusgen/36 frozen corpus
sources: ["fixture: statusgen/36 v1 corpus"]
---

# Brief 01 — v1 corpus

## Context
files: `src/one.txt`

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `true` | pass exit=0 | sha256:abc123def456 | 2026-09-25 | fixture-verifier @ {{BASE}} |

## Review
Gate: model.
