---
stream: ur
status: active
priority: P1
track: platform
---

# UNRUN fixture stream

Fixture stream for methodology-metrics brief-37 (UNRUN as a first-class board
state) and F-impl-claims-unproven. Every brief here is legacy (no frontmatter)
EXCEPT brief-08, which is `schema: brief-v1` so the brief-wide `risk.*: yes`
path has something to read. The point of the mix is that the derivation is
schema-agnostic: a legacy brief is exempt from the hard brief-v1 checks but
must still be covered by a board state.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Done over an unrouted UNRUN live row](./brief-01-unrun-unrouted.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 02 | [Done over a routed UNRUN live row](./brief-02-unrun-routed.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 03 | [Done over an UNRUN non-risk-bearing row](./brief-03-unrun-not-risky.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 04 | [Done, fully run](./brief-04-fully-run.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 05 | [Done, live row silently absent from Evidence](./brief-05-silent-omission.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 06 | [Implemented, zero coverage](./brief-06-implemented-uncovered.md) | 0 | S | implemented | — | — |
| 07 | [Implemented, partially covered](./brief-07-implemented-covered.md) | 0 | S | implemented | — | — |
| 08 | [Done, brief-wide risk yes](./brief-08-risk-yes.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 09 | [Done, Evidence row with no dated runner](./brief-09-undated-row.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
