---
brief: statusgen/13
title: 'Cadenced roadmap artifacts — `--cadence weekly|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-27 (authored clean for the statusgen board)
why: >-
  The `--roadmap` renderer answers one clock — "what is the portfolio's state right now."
  Reviewers on a weekly or monthly cadence ask a different question: "how did the closed
  week trend" and "what shipped over the month, and is every objective still covered."
  Stretching one point-in-time page to answer all three bloats it; the durable fix is a
  cadence flag that computes a closed time window (prior ISO week, prior calendar month)
  and re-renders the SAME roadmap skeleton over that window. The scheduling of these runs,
  the priority order they rank against, and the brand they wear are all adopter CONFIG —
  the tool bakes in none of them, so any repo that consumes statusgen gets cadenced
  artifacts by configuring, not forking.
sources:
  - "statusgen/roadmap.go + statusgen/roadmapdora.go — the `--roadmap` deck-overview renderer and its ONE Go health-rule table (`roadmapHealthRules`); this brief reuses that renderer and rule table over a computed window, it does not add a second renderer"
  - "statusgen/main.go — the `-roadmap` flag registration (~L1107) and the `--roadmap` output path `docs/reports/roadmap/index.html`; the cadence flag is added beside it and switches the output directory to `docs/reports/<cadence>/<window>/`"
  - "statusgen/main.go `-scope` — the existing product-tag (`serves:`) vocabulary the priority order is expressed in; the ordered list is read from config, never hard-coded in source"
  - "the roadmap renderer's stream frontmatter reader — where the optional `theme:` key is parsed and the unmapped-renders-visibly rule lives"
---

# Brief 13 — Cadenced roadmap artifacts (`--cadence weekly|monthly`)

An **implementation brief** on statusgen's own source. It generalizes the existing
point-in-time `--roadmap` renderer to two additional cadences by computing a closed time
window and re-rendering the same skeleton over it. Everything that differs between adopters —
the priority order the monthly ranks against, the brand the deck wears, which sections a
cadence emits — is CONFIG the tool reads, never a value in source. This brief adds no new
renderer and no new health rules; it reuses `roadmap.go`'s renderer and its one health-rule
table.

## Context

files: statusgen/main.go, statusgen/roadmap.go, statusgen/cadence.go (planned),
statusgen/cadence_test.go (planned), the stream-frontmatter reader in the roadmap renderer.

files (created/extended):
- `statusgen/main.go` — register a `-cadence` string flag (`weekly` | `monthly`; empty =
  today's point-in-time `--roadmap`, unchanged). When set with `-roadmap`, it selects the
  window computation below and switches the output directory from
  `docs/reports/roadmap/` to `docs/reports/<cadence>/<window>/`.
- `statusgen/cadence.go` (planned) — pure window computation: given `now` and a cadence, return
  the closed window `[start, end)`. `weekly` = the prior complete ISO week (Mon 00:00 UTC →
  next Mon 00:00 UTC), labelled `%G-W%V`. `monthly` = the prior complete calendar month,
  labelled `%Y-%m`. No I/O — a pure function so the boundary math is unit-testable.
- `statusgen/roadmap.go` — thread the computed window into the existing renderer so
  transition/exception accounting is scoped to `[start, end)` instead of point-in-time.
  Exceptions that BOTH opened and closed inside the window count (a within-window churn
  signal), not only those still open at `end`. Renderer and health-rule table are otherwise
  unchanged.
- the stream-frontmatter reader — parse an optional `theme:` key per stream README and apply
  the unmapped-renders-visibly rule (below).

facts:
- **The renderer already exists.** `--roadmap` renders the deck-overview page from
  `roadmap.go` + its `roadmapHealthRules` table to `docs/reports/roadmap/index.html`. This
  brief does NOT rebuild it — `--cadence` reuses it verbatim over a computed window.
- **Priority order is config, not source.** The monthly's effort-mix section ranks work by an
  ORDERED list of product (`serves:`) tags read from adopter config — the same `serves:`
  vocabulary `-scope` already validates. The tool ships with NO baked-in order; a repo that
  sets none gets the sections rendered in an unranked/declaration order. No product name and
  no ranking is compiled into statusgen.
- **Brand is config, not source.** The deck's visual surface (palette, wordmark, dark/light)
  is read from the running repo's brand config (a `docs/brand/` convention) when present, and
  falls back to a neutral built-in default otherwise. statusgen hard-codes no brand.
- **The revenue/supporting callout is a generic reporting concept, kept.** The monthly emits
  an explicit line when supporting-tier work outweighed revenue-tier work for the window,
  where "revenue tier" vs "supporting tier" is derived from the position of each `serves:` tag
  in the configured priority order — no project identity attached, purely the ordered tiers
  the adopter configured.
- **`theme:` is optional and fails visibly, not silently.** A stream README may carry an
  optional `theme:` frontmatter key. A `theme:` value with no mapped render style renders as a
  VISIBLE "unmapped theme: <value>" marker on the deck rather than being dropped — an unmapped
  theme is itself the signal, the same philosophy as the roadmap's untagged-renders-visibly
  handling. Absent `theme:` is fine (no marker).
- **Graceful degradation.** A cadence section whose upstream data is unavailable emits a
  "pending" placeholder for that section rather than failing the whole run.
- Output discipline: each cadence writes its own `docs/reports/<cadence>/<window>/` tree and
  is STATUS.md-free, matching the existing `--roadmap`/`--dora`/`--trend` discipline (no
  STATUS.md regen as a side effect).

## Ground rules
- Stop at `implemented` — you do not set verified/done.
- No product names, no repo-specific priority order, and no brand values in source, tests, or
  fixtures — the priority order and brand are read from config at runtime and are provided by
  the adopter. Test fixtures use neutral placeholder tags only (e.g. `example-app`,
  `example-service`), matching the `-scope` vocabulary already in `main.go`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Add the `-cadence weekly|monthly` flag (empty = unchanged point-in-time `--roadmap`) and
   the pure window computation in `statusgen/cadence.go` (planned) (ISO-week and
   calendar-month boundaries).
   Thread the computed window into `roadmap.go`'s existing renderer; switch the output to
   `docs/reports/<cadence>/<window>/`. Weekly WBR deck and monthly exec-summary content are
   the same skeleton scoped to the window; exception accounting counts opened-AND-closed
   within the window.
2. Read the priority order (an ordered list of `serves:` tags) and the brand from adopter
   config; ship NO baked-in order or brand. The monthly's effort-mix section and the generic
   revenue-vs-supporting-tier callout derive their tiers from the configured order. Deck brand
   reads from the repo's `docs/brand/` convention, neutral built-in fallback when absent.
3. Add the optional `theme:` stream-frontmatter key and the unmapped-renders-visibly rule.
4. Tests (`statusgen/cadence_test.go` (planned), plus renderer-level cases): cadence window computation
   (ISO-week boundary incl. year-boundary weeks, and month boundary incl. December→January
   and leap-February); opened-AND-closed exception accounting; unmapped-`theme:` renders a
   visible marker; brand/priority fallbacks when config is absent (default order, default
   brand, no panic).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --roadmap --cadence weekly && command ls docs/reports/weekly/` | a week-labelled (`%G-W%V`) directory exists (literal window label per implementer's naming, recorded in Evidence) |
| 2 | `statusgen --root . --roadmap --cadence monthly && command ls docs/reports/monthly/` | a month-labelled (`%Y-%m`) artifact directory exists |
| 3 | `statusgen --root . --roadmap` (no `-cadence`) still writes `docs/reports/roadmap/index.html` | exit 0 — point-in-time mode unchanged |
| 4 | `go test ./statusgen/ -count=1 -run Cadence -v > /tmp/cad.log 2>&1 && grep -q -- '--- PASS' /tmp/cad.log && go test ./statusgen/ -count=1 -run Theme -v > /tmp/thm.log 2>&1 && grep -q -- '--- PASS' /tmp/thm.log` | exit 0 — both the cadence-window and unmapped-theme test groups EXIST (a `--- PASS` line) and pass. No raw pipe: redirect to a file + `grep FILE`, chained with `&&`, so a group that runs nothing (no `--- PASS`) goes red |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time -->

Implemented on feat/statusgen-13. Verify table run locally (offline, KUBECONFIG=/dev/null)
against this root:

| # | Result |
|---|--------|
| 1 | `--roadmap --cadence weekly` → rc 0; `docs/reports/weekly/` holds the ISO-week-labelled directory `2026-W34/` (the prior complete ISO week for the run date). |
| 2 | `--roadmap --cadence monthly` → rc 0; `docs/reports/monthly/` holds the month-labelled directory `2026-07/` (the prior complete calendar month). |
| 3 | `--roadmap` (no `-cadence`) → rc 0; still writes `docs/reports/roadmap/index.html` — point-in-time mode unchanged. |
| 4 | `go test ./statusgen/ -run Cadence` and `-run Theme` → both groups exist (`--- PASS` lines) and pass. |
| 5 | `statusgen --root . --lint` → rc 0 (`LINT: PASS`; only pre-existing NOTICEs). |

Generated `docs/reports/<cadence>/<window>/` trees are STATUS.md-free build artifacts and
are not committed, matching the existing `--roadmap`/`--dora`/`--trend` discipline.
### Non-implementer verifier run — VERIFY: PASS — 2026-09-04 opus-4.8[1m]-verifier (verify-desk dispatch), merged main 4e500df

Runner != implementer. Offline (KUBECONFIG=/dev/null). gate: model, risk {all no}, irreversible: no.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | statusgen --root . --roadmap --cadence weekly && ls docs/reports/weekly/ | 0 | wrote docs/reports/weekly/2026-W35/index.html + 14 stream pages; prior complete ISO week | 2026-09-04 | opus-4.8[1m]-verifier |
| 2 | statusgen --root . --roadmap --cadence monthly && ls docs/reports/monthly/ | 0 | wrote docs/reports/monthly/2026-08/index.html; prior complete month | 2026-09-04 | opus-4.8[1m]-verifier |
| 3 | statusgen --root . --roadmap (no cadence) | 0 | docs/reports/roadmap/index.html present; point-in-time mode unchanged | 2026-09-04 | opus-4.8[1m]-verifier |
| 4 | go test ./statusgen/ -run Cadence and -run Theme | 0 | 18 Cadence PASS (year-boundary, Dec->Jan, leap-Feb, half-open) + 3 Theme PASS | 2026-09-04 | opus-4.8[1m]-verifier |
| 5 | statusgen --root . --lint | 0 | LINT: PASS (pre-existing NOTICEs only) | 2026-09-04 | opus-4.8[1m]-verifier |

**VERIFY: PASS** — all 5 rows ran (row 4 via the module-dir equivalent; literal-from-root fails only on go.work layout, not implementation).

**RISK-VALUE: DERIVED** — weekly window = curMon.AddDate(0,0,-7) with daysSinceMon=(Weekday()+6)%7 @ statusgen/cadence.go:41,43 — Go Weekday Sun=0..Sat=6; (w+6)%7 maps Mon->0..Sun->6; -7 lands prior Monday, giving half-open [prevMon,thisMon); ISOWeek supplies the ISO year for Dec/Jan-straddling weeks. Matches %G-W%V; year-boundary test passes.
**RISK-VALUE: DERIVED** — monthly window = firstThis.AddDate(0,-1,0) -> end=firstThis @ statusgen/cadence.go:51-54 — first-of-this minus one month = first-of-prior; AddDate normalization rolls Dec->Jan, yielding [firstPrior,firstThis); label 2006-01 == %Y-%m. Dec->Jan and leap-Feb tests pass.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
