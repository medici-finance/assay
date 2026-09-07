---
schema: brief-v1
brief: dg/07
title: Risk-gated in-progress with an unparseable authored (bare YAML date)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
gate-why: touches a risk surface (fixture)
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
authored: 2026-09-10
sources: ["fixture"]
---

# Brief 07

A bare, unquoted `authored: 2026-09-10` (no trailing "by ..." text) decodes as
a YAML timestamp rather than a string, so `data["authored"].(string)` fails
its type assertion and `bf.Authored` stays `""` — the present-but-unparseable
case the design-approval gate must fail OPEN on, but VISIBLY (a NOTICE), never
silently.

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | rc 0 |
