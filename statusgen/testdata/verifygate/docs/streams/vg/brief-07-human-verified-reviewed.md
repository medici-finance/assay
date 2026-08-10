---
brief: vg/07
title: Human-gated verified brief that already carries a human reviewer
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: human-gated, verified, already human-reviewed (#159)"]
---

# Brief 07 — human-gated, verified, already human-reviewed

An irreversible brief already carries a `human:<name>` sign-off in Reviewed at
the `verified` stage (the #159 rule). Under the two-touch model a verify-gate
issue STILL fires (the done-close is a distinct acceptance touch), and
close-verify must preserve this original provenance while appending the
acceptance sign-off.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | glm-verifier |

## Review
Gate: human (irreversible); human-reviewed before verified.
