---
brief: vf/03
title: Human-gated brief at implemented whose only recorded pass is below the floor
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by fixture
sources: ["fixture: gate:human, implemented, VERIFY: PASS recorded by a below-floor runner"]
---

# Brief 03 — implemented, pass recorded below the floor

The one-step implemented→done path stamps the Verified cell FROM this
Evidence. The runner it would stamp is below the floor, so the close must be
refused on the cell it is about to write.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-18 | sonnet-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
