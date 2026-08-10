# Brief 08 — Unbacked done: implementer-only Evidence

The README row is `done` with a dated Verified cell and an attributed Reviewed
cell, and the `## Evidence` section is filled — but its only runner is the
implementer, so there is no independent (non-implementer) row. Per brief-16 an
`attributed` verification needs an independent runner, so this row is NOT backed
even though it looks like a real done. Because this file carries no
`schema: brief-v1` frontmatter, attributionProblems is exempt — this is exactly
the blind spot point quality closes: it must render `done*`.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | implementer (sonnet) |
