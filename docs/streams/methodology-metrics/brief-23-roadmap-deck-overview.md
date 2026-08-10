---
brief: methodology-metrics/23
title: 'Roadmap deck page 1 — statusgen --roadmap: the all-streams portfolio grid (computed health, fixed order, delta badges)'
wave: 1
depends: []
unblocks: ["methodology-metrics/24"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction; design basis researched same day)
sources: ["human:<name> 2026-07-12: 'we also need some product roadmap decks to be generated daily. these should a) show a overall highlevel picture of all the streams on one page, b) for each stream show the briefs in them. do some research on best practices…'", "docs/streams/methodology-metrics/roadmap-deck-research.md (the researched design basis — REQUIRED READING, all layout/health/delta decisions derive from it)", "methodology-metrics/01 (status historian — the delta source)", "methodology-metrics/10 (verification-debt alarm — an exceptions-strip input)", "methodology-metrics/22 (daily harvest — the scheduled vehicle this output joins)", "docs/brand-guide.md (Medici system: the deck is a product surface — dark canvas, blue #3366FF leads)", "freshness-checked 2026-07-12 @ post-#365 main"]
why: >-
  The board (STATUS.md) is a working surface for the desks; nothing today gives human:<name> or an
  exec the one-page portfolio picture — and hand-made roadmap decks rot in a day at our
  merge velocity. The research is unambiguous that a DAILY multi-stream view should be a
  computed status grid with watermelon-proof health (rule printed beside every pill) and
  WBR-stable layout; all inputs (lifecycle counts, transitions, Next-up, alarms) already
  exist in statusgen — this renders them, asserting nothing.
---

# Brief 23 — Roadmap deck: portfolio overview page

## Context

files: `../assay-toolkit/statusgen/` (new `--roadmap` mode + roadmap.go), output
`docs/reports/roadmap/index.html` (planned); research basis
`docs/streams/methodology-metrics/roadmap-deck-research.md`.

facts:
- Inputs all exist: stream READMEs (rows/lifecycle), typed `depends:` graph + waves,
  Next-up scoring (span-capped), the historian's transition log (mm/01), FINDINGS
  affects-links, verification-debt (mm/10).
- Health is COMPUTED, never asserted (research finding 2). v1 rules (render the firing rule
  beside the pill, and keep rules in one Go table so the legend generates from the same
  source): red = P0 stream with an unresolved affecting FINDING, or any brief stalled
  in-stage > 7d (implemented or blocked; critical-path filter deferred — historian too young); amber = ≥2 briefs implemented-unverified > 3d, or
  Next-up entry from this stream unpicked > 3d; else green. Tune at retro, in code.
- Layout invariants (research findings 3/4/6): stream order FIXED (priority then name —
  never health-sorted), exceptions strip computed with an explicit empty-state, Next-up is
  the only forward list, todo long-tail renders as counts in the stage mini-bar.
- Delta badges derive from the historian (last-24h transitions), not a snapshot diff.
- Brand: Medici dark product surface (NOT the white paper surface — this is a tool/deck,
  not a report); blue leads; stage palette + legend identical on every page; single
  self-contained HTML (no external fetches), print-CSS so the same file exports to PDF.
- Every rendered item hyperlinks its artifact: brief file, PR, finding (research finding 7).
- Generated-at + source commit in the header (freshness anchor).
- **Strategic-mix consciousness (human:<name> 2026-07-12; research finding 11)**: every stream README
  frontmatter gains a `serves:` key — one of `lending-app | reconciler | assay | platform` —
  and the stated priority order is FIXED data in the same Go table as the health rules:
  **lending-app first (the main aim is to make money off it), reconciler second,
  assay/methodology supporting**. Page 1 renders a goal-mix strip: active-WIP, Next-up, and
  merged-last-24h composition BY GOAL, with a computed INVERSION callout when the activity
  mix contradicts the priority order (e.g. "merges today: 80% assay, 5% lending — inverted").
  A stream with no `serves:` renders "untagged" — visibly, the unmapped-is-the-signal rule.
  Mix is computed from tags, never asserted (the watermelon defense applied to strategy).
  Beside the goal-mix strip: a per-goal DORA rollup line (lead time + deploy freq per goal,
  from mm/26's `--dora --by goal --json`; render "pending mm/26" until it lands rather than
  blocking).
- consumers (rule 6): mm/22's daily-harvest workflow (adds `--roadmap` output when both
  land — one-line wire, either brief's follow-up), mm/24 (per-stream pages extend the same
  roadmap.go), the-desk boot read, exec surfaces (assay-toolkit exec brief cites the deck).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `statusgen --roadmap` renders `docs/reports/roadmap/index.html` (planned) — page 1 only
   (mm/24 adds per-stream pages): header (generated-at, commit, portfolio stage-stacked bar
   with Δ counts) → Next-up block (verbatim rows, linked) → exceptions strip (computed;
   explicit "no exceptions" empty state) → the stream grid: one fixed-order row per stream
   with health pill + printed firing rule, stage mini-bar (todo count compressed), Δ badges
   (blank = unchanged), next wave gate, top blocker (from depends/findings), owner cell
   (stream README's maintenance owner line, "—" when absent).
2. Health rules + exceptions rules live in ONE Go table; the page legend is generated from
   it (a hand-written legend drifts).
3. Deterministic output (stable ordering, no timestamps beyond the header) so a same-state
   re-run is byte-identical — diffable like every generated artifact here.
4. Unit tests: health-rule table (each rule fires on a synthetic fixture), fixed-order
   invariant, empty-exceptions state, delta-badge derivation from a fixture historian log.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --roadmap && test -f docs/reports/roadmap/index.html` | exit 0 |
| 2 | `grep -c "3366FF" docs/reports/roadmap/index.html` | ≥ 1 (Medici blue present) |
| 3 | `grep -ciE "amber:|red:" docs/reports/roadmap/index.html \|\| grep -c "no exceptions" docs/reports/roadmap/index.html` | ≥ 1 (pills carry printed rules, or explicit empty state) |
| 4 | `statusgen --root . --roadmap && shasum docs/reports/roadmap/index.html && statusgen --root . --roadmap && shasum docs/reports/roadmap/index.html` | identical hashes (deterministic) |
| 5 | `go test ./tools/statusgen/ -run Roadmap -count=1` | exit 0 |
| 6 | `statusgen --root . --lint` | exit 0 |
| 7 | `grep -ciE -e "goal-mix" -e "lending-app" docs/reports/roadmap/index.html` | ≥ 1 (goal-mix strip present with the lending-first ordering) |

## Evidence
<!-- appended at implementation time -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `a6286beb`, 2026-07-19)

| Row | Expect | Observed |
|-----|--------|----------|
| 1 | `--roadmap` writes deck; `index.html` exists | exit 0; deck written |
| 2 | Medici blue (`3366FF`) present | 10 |
| 3 | amber/red pills carry rules (or empty state) | 31 |
| 4 | deterministic (identical hashes) | byte-identical `26423c64…` |
| 5 | `go test -run Roadmap -count=1` | exit 0 |
| 6 | `--lint` | exit 0 (NOTICEs only) |
| 7 | goal-mix strip, lending-first ordering | 11 |

VERIFY: PASS. Footprint clean (deck created/removed; `--roadmap`/`--lint` exit pre-STATUS.md, so the main-CI-only writer is untouched). `gate: model`, all risks `no` → flip.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
