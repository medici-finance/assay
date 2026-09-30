---
brief: grep-pipe-zero-count/01
title: Absence checks that fail exactly when the thing is absent
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1699]
schema: brief-v1
authored: 2026-09-30 by fixture
sources: ["fixture: #1699 — the witness recorded fail exit=1 on a row whose output was the expected 0"]
---

# Red — the success path exits 1

Every row counts the matches of a grep that feeds a later stage and expects
zero. Under `bash -o pipefail` the grep's no-match exit 1 becomes the row's
exit, so each row fails on the tree where the property holds. Row 1 is the
#1699 row verbatim.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -n '"/bin/bash"' HEAD -- tools/desk/cmd/scanloop/ \| wc -l` | `0` |
| 2 | `grep -rn 'api.example.test' tools/cellctl \| wc -l` | prints `0` — the host comes from configuration |
| 3 | `grep -rn -e '"gh"' tools/desk/cmd --include='*.go' \| grep -v _test.go \| wc -l` | `0` — no forge CLI literal |
| 4 | `cd tools/desk && grep -rn 'LegacyAdapter' cmd/ \| wc -l \| tr -d ' '` | `0` |
| 5 | `git -C tools grep -n 'LegacyAdapter' HEAD \| wc -l` | output is `0` |
| 6 | `egrep -rn 'LegacyAdapter' tools \| sort \| wc -l` | 0 (adapter retired) |

## Review
Gate: model.
