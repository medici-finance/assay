# Brief 03 — a legacy brief with no frontmatter

Legacy (no `schema: brief-v1` frontmatter) → exempt from the #509 row rules, the
same opt-in every other brief-file check uses. Its Verify row carries the
mis-escaped alternation on purpose: the lint must stay silent here.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ciE "arm64\|amd64" S1-report.md` | ≥1 |
