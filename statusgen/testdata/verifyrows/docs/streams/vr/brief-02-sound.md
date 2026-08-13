---
brief: vr/02
title: A brief-v1 file whose Verify rows can actually fail
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [509]
schema: brief-v1
authored: 2026-07-15 by fixture
sources: ["fixture: sound Verify rows — the false-positive guard"]
---

# Brief 02 — rows that can fail

Every row here is sound and MUST stay silent: the alternation is written as
separate `-e` patterns, the literal pipe uses a bracket class, the counts are
read as output rather than gated on zero, and the absence checks gate on the
exit status the way `! grep -q` is meant to be used — row 8 additionally carries
the `test -f` existence guard, because a bare `! grep -q` over a target that is
absent exits 0 having examined nothing (methodology/44 fact 6). None of that is
decidable by the rules above; the guard is here so the fixture named *sound*
models the idiom the sweep prescribes, not merely the shapes the lint can see.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ciE -e arm64 -e amd64 S1-report.md` | ≥1 (platform verdict recorded) |
| 2 | `grep -cE "a[\|]b" table.md` | ≥1 (literal pipe intended) |
| 3 | `grep -c "docs/" README.md` | ≥4 (doc index present) |
| 4 | `! grep -qE "30:1" deck.md` | exit 0 (forbidden number absent) |
| 5 | `grep -cE "30:1" deck.md \|\| true` | 0 (F-12) |
| 6 | `ls docs/streams/intake/ \| grep -c 2026-07` | ≥1 (an intake entry landed) |
| 7 | `go test` | exit 0 |
| 8 | `test -f web/site/index.html && ! grep -Eiq -e "<script[^>]*src=" -e "<link[^>]*stylesheet" web/site/index.html` | exit 0 (self-contained) |
| 9 | `go test ./tools/example/cmd/exampleboard/... -count=1` | exit 0 |
| 10 | `test -d docs/reports/daily/$(date +%F)` | exit 0 (today's harvest landed) |

## Review
Gate: model.
