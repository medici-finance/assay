---
schema: brief-v1
brief: attr/09
title: Evidence row self-labels "(non-implementer)"
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-07-08 by Fable session (test)
sources: ["s"]
---

# Brief 09

The only Evidence Runner cell asserts its own independence with a
"(non-implementer)" self-label (security-hardening ID-2, hole-1). Before the
strip was removed this counted as an independent row and the brief passed; it
must now be flagged — a self-assertion does not establish independence. The
Verified cell names a distinct runner so ONLY the Evidence-independence problem
fires here (not a Verified-cell self-verification problem).

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet verifier (non-implementer) |
