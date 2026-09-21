---
id: DEPLOY-fixture-rollback-bad
kind: deploy
date: "2026-09-17"
title: "Fixture deploy that accepts no rollback with no named approver"
environment: staging
brief: dp/01
authority: "human:ada"
rollback: none-accepted
---

Register-shape path: `rollback: none-accepted` with no `rollback-approver` must be flagged —
the whole point of the field is that "we can't roll this back" is a decision someone is
named as having accepted, never a silent omission.
