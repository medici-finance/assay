---
id: REQ-apfixture-ok
date: "2026-01-01"
title: "Worked example: a requirement with one resolvable backing brief"
impact: minor
asked-by: "sdlc/08 (audit-pack export) — a self-contained fixture, not a real ask"
acceptance:
  - "A release-keyed audit pack that is in scope for the audit-pack-example fixture stream names this requirement and its one backing brief."
status: accepted
---

Fixture entry for `statusgen --export-audit-pack`'s worked example (sdlc/08). It exists so
the release-keyed pack has at least one requirement whose backing-brief chain resolves
cleanly — the counterpart to `REQ-apfixture-unresolved`, whose chain does not.

Not a real ask: nothing in this repo depends on this requirement's `status:` ever advancing
past `accepted`, and no brief outside `docs/streams/audit-pack-example/` may cite it.
