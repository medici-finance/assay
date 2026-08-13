---
brief: portability/01
title: GNU-only constructs in rows a macOS desk also has to run
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [650]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: #650 — desk-hardening/06 rows 1,4,5,6,7 read /dev/fd/N as EMPTY under BSD grep"]
---

# Red — a different verdict per platform

Row 1 is #650's measured instance: under BSD `grep` the process substitution's
`/dev/fd/N` reads as empty, so the row returns 0 with the content plainly
present — a false FINDING, not a miss. The rest are the catalogue's other
GNU-isms.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -cE 'watchdog' <(sed -n '/3.2/,/3.3/p' contract.md)` | ≥ 18 |
| 2 | `grep -Pc '(?<=v)[0-9]+' versions.txt` | ≥ 1 |
| 3 | `sed -i 's/old/new/' docs/brief-rules.md && echo ok` | ok |
| 4 | `readlink -f ./statusgen` | an absolute path |
| 5 | `stat -c %s STATUS.md` | ≥ 1000 |

## Review
Gate: model.
