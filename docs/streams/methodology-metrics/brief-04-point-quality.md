---
brief: methodology-metrics/04
title: Point-quality rendering — an unverified done* visibly distinct from evidence-backed done
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA — point quality)", "docs/streams/methodology/scada-ooda-lineage.md", "methodology/brief-16 (Evidence/attribution)"]
---

# Brief 04 — Point-quality rendering

## Context
files: `../assay-toolkit/statusgen/` (STATUS.md render + `--lint` NOTICE), reads the Verified/Reviewed cells +
Evidence sections (already parsed for brief-02/16 checks).
facts:
- SCADA "point quality" = a sensor reading carries a quality flag (good / bad / UNCERTAIN); a value with
  no quality is a lie. Applied here: a `done` row whose Evidence is empty or whose Verified/Reviewed cell
  is bare/unattributed is **not the same** as an evidence-backed `done`, but STATUS.md renders them
  identically today — a false-confidence signal.
- The methodology's own red-team ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)) is precisely this: status derived from agent-authored artifacts
  can be asserted without backing. Point-quality rendering makes the difference *visible*.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In statusgen, compute a **quality flag** per `verified`/`done` row: `backed` (Evidence section filled +
   Verified/Reviewed cells dated+attributed per brief-16) vs `unbacked` (missing/bare/unattributed).
2. Render the distinction in STATUS.md — e.g. `done` vs `done*` (star = unbacked), with a one-line legend;
   and emit a `--lint` NOTICE (not a hard error) listing unbacked `done`/`verified` rows so the desk sees
   the false-confidence points.
3. Do not change the lifecycle gates themselves (brief-02/03/16 own those) — this is a *rendering/visibility*
   change only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run PointQuality` | exit 0 (backed vs unbacked classification tests) |
| 2 | testdata: a `done` row with empty Evidence → regenerate STATUS | renders `done*`; a `done` with filled Evidence + attributed cells renders `done` |
| 3 | `statusgen --lint` on that testdata | NOTICE names the unbacked `done*` row (exit code unchanged by the notice) |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` (real tree) | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0c9e0bce`; tests re-confirmed at `927d8107`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run PointQuality` | 0 | `ok` — 14 PointQuality subtests PASS (re-ran green at 927d8107) | 2026-07-09 | opus-verifier |
| 2 | testdata: unbacked `done` → `done*`, backed `done` → `done` | 0 | `TestPointQualityRendering` asserts unbacked-empty-evidence renders `done*`, backed renders plain `done`, done/done* legend present | 2026-07-09 | opus-verifier |
| 3 | `--lint` NOTICE names unbacked `done*`, exit unchanged | 0 | live: `NOTICE: frontend/brief-00: done row is unbacked … renders as done*`; LINT_EXIT 0 (`TestPointQualityLintNoticeDoesNotChangeExitCode`) | 2026-07-09 | opus-verifier |
| 4 | `go vet ./tools/statusgen/ && --lint` | 0 | VET_EXIT 0, LINT_EXIT 0 | 2026-07-09 | opus-verifier |

**VERIFY: PASS** — unbacked verified/done rows render `*`-suffixed with a non-blocking lint NOTICE; exit code unchanged. Re-confirmed green after statusgen changes landed at 927d8107.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
