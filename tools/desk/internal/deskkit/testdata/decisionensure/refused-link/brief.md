---
brief: assay:ex:sample:07
title: "Pick the retention window"
gate: human  # the driver rules
decision-trigger: start
gate-why: >-
  The window is a policy call
  with a cost either way.
why: Logs grow without bound.
effort: S
risk: data-loss=no
---

# Sample

## Human decision

Context: logs grow.

1. Keep 30 days, see https://example.invalid/policy.
2. Keep 90 days.

Default if no answer: option 1 after 2026-12-01

## Deliverables

none
