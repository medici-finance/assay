---
brief: vg/11
title: Human-gated verified brief whose existing reviewer is undated
wave: 0
depends: []
unblocks: []
effort: M
gate: human
gate-why: "fixture: reproduces the undated-existing-Reviewed-cell close-verify/lint disagreement"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: reproduces the undated-Reviewed-cell case — closer 'ada' at 2026-08-07, close-verify onto a bare undated Reviewed cell"]
---

# Brief 11 — human-gated, verified, existing reviewer has no date

A gate:human brief already carries a bare `human:<name>` sign-off in Reviewed
at the `verified` stage (methodology/03's hasHumanReviewer does not require a
date, so this is a sanctioned value — matches the daml-hardening/14 row).
close-verify must still produce a Reviewed cell that starts with a dated
stamp at `done` (methodology/19), even though the prior sign-off it is
preserving has no leading date of its own to anchor on.

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
Gate: human; human-reviewed (undated) before verified.
