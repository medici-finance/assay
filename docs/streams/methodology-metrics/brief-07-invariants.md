---
brief: methodology-metrics/07
title: Two written invariants — "Observe ∝ Act" and "Orient integrity is paramount"
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA/OODA — the two written invariants)", "docs/streams/methodology/scada-ooda-lineage.md", "docs/streams/methodology/red-team-2026-07-09.md ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) orient integrity)"]
---

# Brief 07 — Two written invariants

## Context
files: a new invariants doc under `docs/streams/methodology/` (filename invariants.md, or a section in
the methodology README), cross-linked from `scada-ooda-lineage.md`.
facts:
- [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md)'s SCADA/OODA analysis distilled two load-bearing invariants the methodology has been following by
  convergent evolution, but never stated:
  - **Observe ∝ Act** — observation effort must scale with action volume. A system that acts fast
    (35 merges/day) but observes little (5 done, no stability metrics) is out of balance; this whole
    stream exists to restore it.
  - **Orient integrity is paramount** — the OODA "orient" step (the registers: STATUS, FINDINGS, Next-up)
    must be trustworthy above all, because a corrupted orient poisons every downstream decide/act.
    This is why the registers are append-only, attributable (brief-16), and now point-quality-flagged
    (brief-04); [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)'s red-team is the standing proof that orient integrity is the system's soft spot.
- A doc-only brief: name the invariants so they are checkable design principles, not tacit habits.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the invariants doc at docs/streams/methodology/invariants.md (or a methodology-README section) stating the two
   invariants, each with: the principle, why it holds, how the methodology currently honors it (cite the
   mechanisms — verify-desk, the historian, brief-16 attribution, point-quality), and what a violation
   looks like (the diagnostic).
2. Cross-link from `scada-ooda-lineage.md` and the methodology README so they're discoverable.
3. Keep it terse and design-principle-shaped — a reference the desk can cite when a decision trades off
   observation for speed, or when orient integrity is at stake.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/methodology/invariants.md && grep -ci "observe\|orient" docs/streams/methodology/invariants.md` | file exists; ≥2 (both invariants named) |
| 2 | `grep -rl "invariants.md" docs/streams/methodology/scada-ooda-lineage.md docs/streams/methodology/README.md` | both cross-link it |
| 3 | `statusgen --lint` | exit 0 (doc change doesn't break lint) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f invariants.md && grep -ci "observe\|orient"` | 0 | file exists; count 25 (≥2) | 2026-07-10 | opus-verifier |
| 2 | `grep -rl invariants.md` (scada-ooda-lineage, README) | 0 | both cross-link it | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --lint` | 0 | doc change doesn't break lint | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — two written invariants exist + cross-linked; lint unaffected.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
