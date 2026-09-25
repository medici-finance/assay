---
brief: literal-pipe/02
title: Test selectors written so a table cell cannot change their meaning
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [374]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the #374 fix — single tokens and &&-chained runs carry no pipe"]
---

# Green — the negative control

There is no spelling of an RE2 alternation that survives a GFM table cell
unambiguously, so these rows do not try: each selector is a single token, and a
set is expressed by chaining runs. Rows 1-2 also carry the `--- PASS`
assertion statusgen/14 requires of a `-run` selector, so this fixture stays
silent under that rule too. All rows MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run Forged -v > "${TMPDIR:-/tmp}/lp-g1.out" 2>&1 && grep -F -e '--- PASS' "${TMPDIR:-/tmp}/lp-g1.out"` | PASS |
| 2 | `go test ./statusgen/ -run Dora -v > "${TMPDIR:-/tmp}/lp-g2a.out" 2>&1 && grep -F -e '--- PASS' "${TMPDIR:-/tmp}/lp-g2a.out" && go test ./statusgen/ -run Weekly -v > "${TMPDIR:-/tmp}/lp-g2b.out" 2>&1 && grep -F -e '--- PASS' "${TMPDIR:-/tmp}/lp-g2b.out"` | PASS |
| 3 | `go test -bench Board ./... -benchtime=1x` | PASS |
| 4 | `go test ./statusgen/ -count=1` | PASS |

## Review
Gate: model.
