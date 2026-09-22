---
id: REQ-apfixture-unresolved
date: "2026-01-01"
title: "Worked example: a requirement whose satisfied-by chain does not resolve"
impact: minor
asked-by: "sdlc/08 (audit-pack export) — a self-contained fixture, not a real ask"
acceptance:
  - "A release-keyed audit pack that is in scope for the audit-pack-example fixture stream reports this requirement as could-not-check, with a reason, and never as satisfied."
status: accepted
satisfied-by: ["audit-pack-example/99"]
---

Fixture entry for `statusgen --export-audit-pack`'s worked example (sdlc/08). Its
`satisfied-by` names `audit-pack-example/99`, a brief that does not exist — deliberately, so
the pack's THREE-STATE handling has a real could-not-check chain to report on a tree that
never ran a real release. Existence of a `satisfied-by` brief is not checked at register-
validation time (registers-v1 §6.5); it is checked here, at rollup/audit-pack time, which is
exactly the gap this entry exercises.

Not a real ask, for the same reason `REQ-apfixture-ok` is not.
