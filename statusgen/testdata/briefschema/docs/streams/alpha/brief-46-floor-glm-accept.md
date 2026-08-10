---
brief: alpha/46
title: Risk-flagged brief verified by glm-5.2 (cheap on price, strong on capability — floor accepts)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified by a glm-5.2 runner the floor must accept"]
---

# Brief 46 — risk-flagged, verified by glm-5.2

The floor keys on CAPABILITY, not price. `glm-5.2` is inexpensive to run and
strong enough to re-run a Verify table, so a `glm-5.2-verifier` runner must
clear the floor. An earlier substring list matched the bare string `glm` and
rejected exactly this cell.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | glm-5.2-verifier |

## Review
Gate: human (customer). A strong-capability runner verified it — floor cleared.
