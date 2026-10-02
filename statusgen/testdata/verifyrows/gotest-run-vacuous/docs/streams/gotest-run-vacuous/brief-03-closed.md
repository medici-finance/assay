---
brief: gotest-run-vacuous/03
title: a CLOSED brief whose Verify table still carries the vacuous-selector shape
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v1
authored: 2026-09-23 by fixture
sources: ["fixture: statusgen/14 — the closed-brief exemption + summary count"]
---

# Closed — the historical-record exemption

This brief's README row is `done`. Its two rows below carry the same defect as
`brief-01-red.md`'s rows 1 and 3, but because the brief is closed the rule must
NOT emit a per-row notice for either — the record is not rewritten. Both rows
still contribute to the single run-wide summary NOTICE.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/... -run TestFixtureClosedBare` | PASS |
| 2 | `go test ./statusgen/... -run '^TestFixtureClosedAnchor$' -v > "${TMPDIR:-/tmp}/gtrv-c2.out" 2>&1 && grep -F -e '--- PASS: TestFixtureClosedOther' "${TMPDIR:-/tmp}/gtrv-c2.out"` | exit 0 |

## Evidence
Recorded 2026-09-23 by fixture — both rows were run at the time and passed
against the code as it then stood; the fixture exists to prove the rule's
closed-brief exemption, not to model a real regression.

## Review
Gate: model. Reviewed 2026-09-23 human:fixture.
