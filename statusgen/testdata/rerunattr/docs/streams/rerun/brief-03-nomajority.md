---
schema: brief-v1
brief: rerun/03
title: No single runner ran a majority of the rows
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

# Brief 03

An even split: one row each by two runners. No runner ran a strict majority, so
there is no single actor for the cell to disagree with — the check stays silent
rather than making a judgement call.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `manual source check` | 0 | ok | 2026-07-27 | k3-verifier |
| 2 | `grep -c foo article.md` | 0 | 3 | 2026-07-29 | opus-verifier |
