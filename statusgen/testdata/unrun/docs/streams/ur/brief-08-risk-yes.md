---
brief: ur/08
title: Done with a brief-wide risk yes and an untagged UNRUN row
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: []
---

# Brief 08 — done, brief-wide risk yes

Row 2 carries NO live/mutating tag of its own. It is risk-bearing because the
brief's own risk block answers `irreversible: yes` — an irreversible brief's
unrun check is risk-bearing by definition.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `./scripts/migrate.sh --apply` | migration applied |

## Evidence

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
| 2 | `./scripts/migrate.sh --apply` | UNRUN | no staging environment available | 2026-07-08 | sonnet-verifier |
