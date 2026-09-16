---
brief: vf/04
title: Human-gated brief whose Verified cell clears the floor but whose Evidence does not
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: gate:human, verified, cell names a floor-tier runner, rows were only ever run below the floor"]
---

# Brief 04 — cell says one thing, the rows say another

The Verified cell names a floor-tier runner, but the Evidence records the rows
as run ONLY by a below-floor runner — the re-stamp landed without the re-run's
rows. The floor reads the complete signal: refuse.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
