---
brief: vr/01
title: A brief-v1 file whose Verify rows cannot fail
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [509]
schema: brief-v1
authored: 2026-07-15 by fixture
sources: ["fixture: unfailable Verify rows"]
---

# Brief 01 — rows that cannot fail

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ciE "arm64\|amd64\|version" S1-report.md` | ≥1 (platform verdict recorded) |
| 2 | `grep -c "medici-stuff" docs/brand/README.md` | 0 (stale bot name fixed) |
| 3 | `ls ~/.claude/skills/ \| grep -cE "the-desk\|verify-desk"` | 0 (loose copies retired) |
| 4 | `dpm test \| tee test.log` | exit 0 |
| 5 | `gh issue view <N> --json body` | the decision is recorded |
| 6 | `DESKPUSHGUARD_OFF=1 ...merged-fixture...; echo $?` | 0 with a stderr override warning |

## Review
Gate: model.
