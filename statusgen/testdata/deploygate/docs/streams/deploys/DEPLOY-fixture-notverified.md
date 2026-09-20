---
id: DEPLOY-fixture-notverified
kind: deploy
date: "2026-09-17"
title: "Fixture deploy against an implemented-only brief"
environment: staging
brief: dp/02
authority: "human:ada"
rollback: "re-point to the prior artifact"
---

Negative path (Verify row 4's mutation, pinned permanently here): dp/02 is only
`implemented`, so this record's precondition MUST be refused.
