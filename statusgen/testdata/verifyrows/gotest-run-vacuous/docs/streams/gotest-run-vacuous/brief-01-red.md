---
brief: gotest-run-vacuous/01
title: go test -run selectors with no --- PASS assertion
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v1
authored: 2026-09-23 by fixture
sources: ["fixture: statusgen/14 — go test -run exits 0 on `no tests to run`"]
---

# Red — selectors nothing asserts

Every row below runs `go test -run` and exits 0 whether or not the named test
(or anything under the selector) exists, is built, or was ever renamed away,
because none of them chains a `--- PASS` assertion the rule can trust.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/... -run TestFixtureBareRed` | PASS |
| 2 | `go test ./statusgen/... -run=TestFixtureEqualsRed` | PASS |
| 3 | `go test ./statusgen/... -run '^TestFixtureAnchorRed$' -v > "${TMPDIR:-/tmp}/gtrv-r3.out" 2>&1 && grep -F -e '--- PASS: TestFixtureOtherName' "${TMPDIR:-/tmp}/gtrv-r3.out"` | exit 0 |
| 4 | `go test ./statusgen/... -run '^TestFixtureNegatedRed$' -v > "${TMPDIR:-/tmp}/gtrv-r4.out" 2>&1 && ! grep -F -e '--- PASS: TestFixtureNegatedRed' "${TMPDIR:-/tmp}/gtrv-r4.out"` | exit 0 |
| 5 | `go test ./statusgen/... -run '^TestFixtureNeutralRed$' -v > "${TMPDIR:-/tmp}/gtrv-r5.out" 2>&1 && grep -F -e '--- PASS: TestFixtureNeutralRed' "${TMPDIR:-/tmp}/gtrv-r5.out" \|\| true` | exit 0 |
| 6 | `go test ./statusgen/... -run '^TestFixtureChainARed$' -v > "${TMPDIR:-/tmp}/gtrv-r6a.out" 2>&1 && go test ./statusgen/... -run '^TestFixtureChainBRed$' -v > "${TMPDIR:-/tmp}/gtrv-r6b.out" 2>&1 && grep -F -e '--- PASS: TestFixtureChainARed' "${TMPDIR:-/tmp}/gtrv-r6b.out"` | exit 0 |

## Review
Gate: model.
