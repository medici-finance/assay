# Brief 09 — Backed done: bare human reviewer

Filled Evidence with an independent (non-implementer) runner, a dated Verified
cell, and a Reviewed cell that is a bare `human:ian` token (no leading date).
checkBriefFiles' human-gate accepts exactly this shape (hasHumanReviewer does
not require a date), so point quality must too — this row is backed and must
render plain `done`, never `done*`.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
