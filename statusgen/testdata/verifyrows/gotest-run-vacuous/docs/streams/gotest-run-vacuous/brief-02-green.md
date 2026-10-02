---
brief: gotest-run-vacuous/02
title: go test -run selectors written so the row can actually fail
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v1
authored: 2026-09-23 by fixture
sources: ["fixture: statusgen/14 — the shape the rule recommends"]
---

# Green — the negative control

Row 1 anchors a named selector and asserts its own `--- PASS:` line. Row 2 is a
GROUP token (it matches every test whose name contains it, e.g. `TestCadence…`)
asserted with a generic `--- PASS` line, the `brief-13` row-4 form. Row 3 runs
`go test` with no `-run` at all. All rows MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test -count=1 -run '^TestFixtureGreenNamed$' -v ./... > "${TMPDIR:-/tmp}/gtrv-g1.out" 2>&1 && grep -F -e '--- PASS: TestFixtureGreenNamed' "${TMPDIR:-/tmp}/gtrv-g1.out"` | exit 0 |
| 2 | `go test . -count=1 -run Cadence -v > "${TMPDIR:-/tmp}/gtrv-g2.out" 2>&1 && grep -q -- '--- PASS' "${TMPDIR:-/tmp}/gtrv-g2.out"` | exit 0 |
| 3 | `go test ./statusgen/... -count=1` | exit 0 |

## Review
Gate: model.
