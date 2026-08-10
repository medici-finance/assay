---
brief: methodology-metrics/33
title: Trim the always-unknown DORA metrics from the retro-facing emit
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-simplify-retro-dora](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-simplify-retro-dora-section.md)", "assay-product/08 critical-thinking review", "tools/statusgen/dora.go renderDoraText/renderDoraLine (the retro-facing text render)", "#885 (tracking)"]
why: >-
  `statusgen --dora` renders five metrics, but two of the five (failed-deploy recovery time,
  rework rate) are structurally un-automatable and ALWAYS render `unknown [needs:
  verify-desk|manual]`, and change-failure only ever renders its automated bug-issue slice.
  A retro dashboard that is half honest-unknowns is mechanical noise — the reader learns
  nothing from three permanently-blank rows. Report what is computed; footnote the rest.
---

# Brief 33 — Trim the always-unknown DORA metrics from the retro-facing emit

## Context
files:
- `../assay-toolkit/statusgen/dora.go` — `renderDoraText` / `renderDoraLine` (the human/retro-facing
  text render); `doraThroughputKeys` / `doraInstabilityKeys` (render order); `computeDora`
  (leaves recovery/rework as `Computed:false, Needs:"verify-desk|manual"`)
- `../assay-toolkit/statusgen/dora_test.go` — render tests
- `docs/streams/methodology-metrics/README.md` — one convention line

facts:
- The metric structs already carry `Computed bool`. Recovery (`doraRecovery`) and rework
  (`doraRework`) are `Computed:false` with a `Needs` marker and no number — permanently, by
  construction (`computeDora`, dora.go:234-265). Change-failure (`doraChangeFail`) IS
  `Computed:true` when gh inputs exist (the bug-issue partial slice, dora.go:243-256) — it
  STAYS. Deployment frequency and change lead time are computed — they STAY.
- Design: the retro-facing **text** render (`renderDoraText`) omits any metric that is
  `!Computed && Needs != ""` from its family tables, and instead prints ONE compact footnote
  line naming what is not-yet-automated (e.g. `not yet automated: failed-deploy recovery time,
  rework rate — needs verify-desk|manual`). Never silently drop: the reader must still see the
  gap exists, just not as full blank rows. `renderDoraLine` is unchanged.
- **JSON export stays complete** (`--dora --json`): the trim is text-render-only, so no
  downstream consumer (mm/16 series, mm/26 breakdowns) loses a field. This is a presentation
  change, not a data change.
- The `--dora` grouped/series renders (`renderDoraGroupedText`, `renderDoraSeriesText`) are
  OUT of scope — this brief trims only the primary `renderDoraText` retro emit; leave the
  grouped views' handling of unknowns as-is (a follow-up if the same noise shows there).
- Anti-gaming (stream rule): trimming honest-unknowns is NOT hiding a bad number — there is no
  number; a permanently-blank placeholder is the noise. This preserves the "honest unknown,
  never a fabricated zero" discipline (the fabricated value is what dora.go already refuses).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing test first (`superpowers:test-driven-development`): a `renderDoraText` case
   asserting (a) recovery + rework rows are ABSENT from the family tables, (b) a single
   not-yet-automated footnote naming both is present, (c) deployment-frequency, lead-time, and
   the change-failure partial row are all still rendered.
2. Implement the `!Computed && Needs != ""` filter + footnote in `renderDoraText`. Keep
   `--dora --json` output byte-for-byte identical (add a JSON test asserting recovery/rework
   keys still present in the JSON export).
3. README: one line under conventions — the retro-facing `--dora` text trims permanently-unknown
   metrics to a footnote; JSON stays complete.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Dora' -v` | exit 0; the new render-trim + json-complete subtests PASS |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --dora \| grep -ci 'recovery\|rework'` | prints `1` (the single footnote line only — no per-family rows) |
| 4 | `statusgen --root . --dora --json \| grep -c '"failed_deploy_recovery_time"\|"rework_rate"'` | `2` (JSON export still complete) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; a presentation-only trim of
the retro emit, JSON export unchanged). Reviewer records verdict + date in the stream README table.
