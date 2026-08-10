---
brief: methodology-metrics/06
title: Next-up span-of-control + overflow-as-alarm (EEMUA-191)
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA — span-of-control per EEMUA-191)", "docs/streams/methodology/scada-ooda-lineage.md", "tools/statusgen (Next-up)"]
---

# Brief 06 — Next-up span-of-control + overflow-as-alarm

## Context
files: `../assay-toolkit/statusgen/` (Next-up computation + render).
facts:
- Next-up is the desk's HMI — the operator's active worklist. EEMUA-191 (control-room design) sets a
  **span of control**: an operator can only attend to so many active items; beyond that, quality drops.
- Today Next-up applies a 2-per-stream cap but has no overall span limit and no signal when the true
  backlog vastly exceeds what's shown — the desk can't tell "6 shown" means "6 to do" vs "60 waiting".
- SCADA discipline: an **overflow is itself an alarm** — silently truncating the list reads as "that's
  all there is," the opposite of the truth.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a configurable **span-of-control cap** to Next-up (total items shown, default e.g. 7 ± 2), on top
   of the existing per-stream cap.
2. When the eligible backlog exceeds the cap, render an explicit **overflow indicator** — e.g.
   `Next-up (7 of 23 eligible — 16 held back)` — never a silent truncation. Emit it as a `--lint` NOTICE
   too, so the overflow (WIP pressure) is a visible signal, not hidden.
3. Cap + threshold configurable and documented.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run NextUp` | exit 0 (span-cap + overflow tests) |
| 2 | testdata with 23 eligible briefs, cap 7 → regenerate STATUS | Next-up shows 7 with an explicit "7 of 23 eligible" overflow line |
| 3 | testdata with 5 eligible, cap 7 → STATUS | shows 5, no overflow indicator |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` (real tree) | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run NextUp` | 0 | `TestNextUpSpanCapOverflow` + `TestNextUpNoOverflow` PASS | 2026-07-10 | opus-verifier |
| 2 | 23 eligible, cap 7 → STATUS | 0 | asserts `"7 of 23 eligible"` + `"held back"` line | 2026-07-10 | opus-verifier |
| 3 | 5 eligible, cap 7 → STATUS | 0 | no overflow indicator | 2026-07-10 | opus-verifier |
| 4 | `go vet && --lint` | 0 | clean | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — Next-up span-of-control cap + overflow indicator work.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
