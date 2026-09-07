---
schema: brief-v1
brief: rerun/01
title: Stale Verified cell after a shepherd re-ran the rows
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-07-25 by Fable session (test)
sources: ["s"]
---

# Brief 01

The original verifier (k3-verifier) ran the table on 2026-07-27. A shepherd then
fixed review findings, which changed the artifact two rows measure, and RE-RAN
those rows on 2026-07-29 — stamping them with its own identity. The Verified cell
still names the original verifier, so the cell and the table now disagree.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `manual source check` | 0 | ok | 2026-07-27 | k3-verifier |
| 2 | `grep -c foo article.md` | 0 | 3 | 2026-07-29 | opus-verifier |
| 3 | `grep -ci bar article.md` | 0 | 1 | 2026-07-29 | opus-verifier |
