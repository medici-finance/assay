---
id: RUNBOOK-fixture-undrilled
kind: runbook
date: "2026-09-17"
title: "Fixture runbook, one undrilled row"
trigger: "fixture trigger"
cadence: "quarterly"
---

Negative path (Verify row 5, pinned permanently here): row 2's Evidence cell is empty.

## Drill rows

| # | Action | Expect | Evidence |
|---|--------|--------|----------|
| 1 | do the thing | thing happens | 2026-09-17 fixture-runner: ran it, passed |
| 2 | do the other thing | other thing happens | |
