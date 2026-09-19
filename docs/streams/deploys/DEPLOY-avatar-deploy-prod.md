---
id: DEPLOY-avatar-deploy-prod
kind: deploy
date: "2026-09-18"
title: "deskavatar generator promoted to the prod desk-tools release"
environment: prod
brief: apps-installer/01
authority: "human:release-manager"
rollback: "re-point the prod desk-tools pin to the prior desk-tools release tag (docs/distribution.md's pin/re-pin path); the generator writes no persistent state, so no data migration accompanies the rollback"
---

Worked example for [`../../deploy-model.md`](../../deploy-model.md) § "The deploy
transition" — a real record shape, not a live production deploy. `apps-installer/01`
(Role→App indirection) is `done` in this repo's own board
([`../apps-installer/README.md`](../apps-installer/README.md)), so this record's
precondition (the carried brief must be `verified` or `done`) is satisfied and
`statusgen --lint` reports it clean.
