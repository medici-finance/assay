---
brief: vgc/01
title: Human-gated verified brief whose Verify and Evidence overrun the issue-body limit
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-21 by fixture
sources: ["fixture: human-gated, verified, oversize Verify and Evidence"]
gate-why: >-
  Changes an integrity-check surface; the human confirms the recorded results.
---

# Brief 01 — human-gated, verified, oversize card body

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
<!-- oversize: verify -->

## Evidence
PR: [#42](https://github.com/example-org/tracker/pull/42) (merged 2026-10-21)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
<!-- oversize: evidence -->

## Review
Gate: human (regulatory + irreversible).
