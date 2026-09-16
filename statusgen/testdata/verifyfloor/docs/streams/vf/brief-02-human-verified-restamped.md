---
brief: vf/02
title: Human-gated brief re-verified at a floor-tier runner (second stamp landed)
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: gate:human, verified, floor-tier re-verify stamp leads the Verified cell"]
---

# Brief 02 — the two stamps

The routine drain ran at a local tier (first stamp); a floor-tier runner then
re-ran the table, appended its Evidence rows, and re-stamped the Verified cell
with its own pass LEADING the cell (second stamp). A human done close flips.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

Routine drain (local tier):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |

Floor-tier re-verify (before the human close):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | opus-verifier |

**VERIFY: PASS** (model) — all rows green at both tiers.

## Review
Gate: human (regulatory).
