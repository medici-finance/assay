---
brief: gorun-exit/02
title: The same assertions written so they can actually fail
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [493]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the #493 fix — build once, then assert the binary's exit"]
---

# Green — the negative control

Every row here MUST stay silent. Rows 1-3 build the binary first, so the exit
code the row reads is the program's own. Row 4 keeps `go run` because it asserts
exit **0**, which `go run` does propagate faithfully.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build -o /tmp/rhg ./cmd/repohardenguard && /tmp/rhg --repo x; echo $?` | rc=5 (refused) |
| 2 | `go build -o /tmp/dp ./cmd/deskpost && /tmp/dp ready --pr 1; echo $?` | exit status 6 |
| 3 | `go build -o /tmp/db ./cmd/deskboard && /tmp/db --repo unlisted; echo $?` | exit code 3 |
| 4 | `go run ./cmd/statusgen --root . --lint` | exit 0 |

## Review
Gate: model.
