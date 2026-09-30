---
brief: grep-pipe-zero-count/02
title: The same absence checks with the no-match status handled
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1699]
schema: brief-v1
authored: 2026-09-30 by fixture
sources: ["fixture: the #1699 fix — neutralise only the no-match status, or gate on the exit"]
---

# Green — the negative control

Each row either takes the grep's status out of the final pipeline, negates it,
or does not expect zero. All rows MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -n "$(git ls-tree --name-only HEAD tools/desk/cmd/scanloop/)" && { git grep -n '"/bin/bash"' HEAD -- tools/desk/cmd/scanloop/ \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` |
| 2 | `{ grep -rn 'api.example.test' tools/cellctl \|\| true; } \| wc -l` | `0` |
| 3 | `grep -rn -e '"gh"' tools/desk/cmd --include='*.go' \| grep -v _test.go \| wc -l \|\| true` | `0` |
| 4 | `! git grep -q 'LegacyAdapter' HEAD -- tools/` | exit 0 |
| 5 | `grep -rn 'LegacyAdapter' tools \| wc -l` | `≥ 1` — the adapter is still wired |
| 6 | `! grep -rn 'LegacyAdapter' tools \| grep -v _test.go` | exit 0 |

## Review
Gate: model.
