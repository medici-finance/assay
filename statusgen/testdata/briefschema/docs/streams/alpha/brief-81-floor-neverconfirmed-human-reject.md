---
brief: alpha/81
title: Risk-flagged brief whose Verified runner is a never-confirmed human token
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified, never-confirmed human verifier"]
---

# Brief 81 — never-confirmed human runner (well-formed shape only) (#280)

The Verified cell names `human:carol`. `carol` is a well-formed GitHub login SHAPE
but was NEVER a confirmed human: not in the current ASSAY_HUMAN_LOGIN_MAP, and not in
ASSAY_FORMER_HUMAN_LOGIN_MAP either. Shape alone is not proof a human acted, so the
floor must FAIL this loud — this is the forgery rejection the shape-only form dropped.
The exemption clears only for a human confirmed now OR historically.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | human:alex |

## Review
Gate: human (customer). The Verified cell reads `human:carol` — a login-shaped name
that was never a confirmed human (neither current nor former map). It must FAIL the
floor: a plausible login shape is not confirmation that a human acted.
