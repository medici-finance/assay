---
id: DEPLOY-fixture-rollback-ok
kind: deploy
date: "2026-09-17"
title: "Fixture deploy that accepts no rollback WITH a named approver"
environment: staging
brief: dp/01
authority: "human:ada"
rollback: none-accepted
rollback-approver: "human:ada"
---

Register-shape path: `rollback: none-accepted` WITH a named `rollback-approver` is the
legitimate accepted-consequence form and must NOT be flagged.
