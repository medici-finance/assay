---
brief: vgc/02
title: Human-gated brief at implemented with a pass whose Evidence overruns the issue-body limit
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-21 by fixture
sources: ["fixture: human-gated, implemented, VERIFY: PASS, oversize Evidence"]
---

# Brief 02 — human-gated, implemented with pass, oversize card body

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Trade-offs
**Why we want it:** the gate card stays fileable. **What it limits:** a long
Evidence section is cut on the card; prior state needs no remediation.

## Evidence
PR: [#43](https://github.com/example-org/tracker/pull/43) (merged 2026-10-21)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
<!-- oversize: evidence -->

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
