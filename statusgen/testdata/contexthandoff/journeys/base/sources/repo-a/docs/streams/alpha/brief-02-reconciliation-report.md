---
brief: example:repo-a:alpha:02
title: Reconciliation report
depends: []
consumers:
  - "ledger-export: reads the nightly file"
---

# Brief 02 — Reconciliation report

## Task

Build the reconciliation report.

## Verify

| # | Command | Expect |
|---|---|---|
| 1 | `make report` | exit 0 |

## Source account — 2026-10-02

The finance team asked for a daily cut.

## Evidence

| # | Result | Date |
|---|---|---|
| 1 | partial run recorded | 2026-10-03 |
