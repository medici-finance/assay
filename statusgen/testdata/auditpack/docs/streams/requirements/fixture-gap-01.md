---
id: REQ-fixture-gap-01
date: "2026-01-01"
title: "A requirement whose satisfied-by chain does not resolve"
impact: minor
asked-by: "test fixture"
acceptance:
  - "the pack reports this requirement as could-not-check, never satisfied"
status: accepted
satisfied-by: ["fx/99"]
---
Test fixture requirement with a dangling satisfied-by (fx/99 does not exist).
