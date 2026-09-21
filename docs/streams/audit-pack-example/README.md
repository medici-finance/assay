---
stream: audit-pack-example
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
board: generated
issues: []
---

# audit-pack-example Stream — worked example for the release-keyed audit pack

This stream is a **fixture, not real work**. It exists so `statusgen --export-audit-pack`
(sdlc/08) has a small, self-contained, always-on-disk tree to demonstrate the release-keyed
audit-pack format against — see [`docs/evidence-bundle.md`](../../evidence-bundle.md)'s
release-keyed section and [`docs/release-notes/v0.0.0-fixture.md`](../../release-notes/v0.0.0-fixture.md).

It ships `status: parked` so it is never scored into Next-up and never dispatched: nothing
here is meant to advance past `todo`, and nobody should pick up brief 01 as real work. The
brief below is cited by two fixture entries in the shared requirements register
(`docs/streams/requirements/apfixture-ok.md`, `apfixture-unresolved.md`) so the pack has one
resolvable and one deliberately-unresolvable backing chain to report on.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Fixture brief for the sdlc/08 audit-pack worked example](brief-01-fixture-example.md) | 0 | S | todo | — | — |
<!-- statusgen:briefs:end -->
