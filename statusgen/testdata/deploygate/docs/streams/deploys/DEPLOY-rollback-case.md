---
id: DEPLOY-rollback-case
kind: deploy
date: "2026-09-17"
title: "Fixture deploy whose no-rollback sentinel is mis-cased and has no approver"
environment: staging
brief: dp/01
authority: "human:ada"
rollback: None-Accepted
---

Register-shape path: `rollback: None-Accepted` is the no-reverse-path sentinel in a
non-canonical case. It MUST be recognized as the sentinel (case-insensitive) and, with no
named `rollback-approver`, be flagged exactly like the lowercase form — otherwise the gate
fails OPEN and a mis-cased sentinel silently skips the human-approver requirement.
