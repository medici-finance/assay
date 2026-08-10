# Roadmap-deck research — design basis for mm/23 + mm/24

Synthesis of external best practices for a DAILY, auto-generated, multi-initiative roadmap
deck (human:<name>'s ask 2026-07-12: one high-level all-streams page + per-stream pages showing the
briefs). Full sources at the end; the briefs derive their design from this doc.

## Findings that drive the design

1. **Daily cadence ⇒ status grid, not timeline.** Now/Next/Later and swimlane timelines are
   planning artifacts that barely change day-over-day; a portfolio status grid (one row per
   stream) is the monitoring artifact that meaningfully diffs daily. Swimlane practice caps
   at 4-10 lanes for an exec view — ~15 streams is over the line anyway (Aha!, Tempo,
   ideaplan, projectmanagercopilot).
2. **Compute health; print the rule next to the pill.** RAG is "easy to game, easy to shade
   optimistic"; the documented fix is anchoring status to observable data that cannot be
   manually adjusted (Cultivated Management, Intellect Design, Mastt). Our derived-status
   machinery is exactly this — the deck must NEVER carry a hand-asserted color. Render
   "amber: 2 briefs implemented-unverified 6d", not "amber".
3. **Stable visual anchors (Amazon WBR).** Identical layout, palette, stream ORDER, and page
   positions every day — fingertip-feel comes from sameness; anomalies pop against a familiar
   frame. Never re-sort streams by health; surface urgency via an exceptions strip and badges
   (Commoncog WBR deep-dive, Holistics).
4. **Exception-based narration.** Show everything; annotate only exceptions. Green gets no
   text ("blood in the water" rule). An EMPTY exceptions strip renders explicitly as
   "no exceptions" — absence must be visible, not ambiguous.
5. **Deltas as badges, from yesterday.** Card anatomy Label→Value→Δ→Timeframe; per-row change
   pills (`+2 done`, `1→in-progress`, `NEW`), blank when unchanged (Setproduct, Geckoboard).
   Our status-transition historian (mm/01) provides deltas without needing a snapshot diff.
6. **Scarcity of forward surface (ShapeUp betting table).** The only forward-looking LIST on
   the overview is the computed Next-up queue; the todo long-tail compresses to counts.
7. **Every claim links to its artifact (GitLab direction-page pattern).** Brief ids, PRs,
   findings — traceable, never badge-asserted.
8. **Waves are ordinal, not temporal.** Render the dependency waves as a stacked ladder
   (Wave 1 done → Wave 2 active → Wave 3 blocked), never on a time axis — a Gantt fakes
   precision the data doesn't have (Appcues, ProductPlan).
9. **Outcome line against the feature-factory smell**: each stream page carries its one-line
   WHY from the stream README, distinct from the item list (ProdPad, Product School).
10. **Self-evident freshness**: generated-at timestamp + source commit on every page.
11. **Strategic-mix consciousness (human:<name>, 2026-07-12).** The portfolio view must weigh streams by
    the BUSINESS GOAL they serve, not render all streams as equal citizens: the stated priority
    order is **(1) the lending app — the main aim is to make money off it, (2) the reconciler,
    (3) assay/methodology — important, but supporting**. Process-improvement work is fast and
    merge-dense, so an undifferentiated activity view systematically over-represents it — the
    deck must show the WIP/Next-up/merged mix BY GOAL and CALL OUT inversions ("this week's
    merges: 80% methodology, 5% lending — inverted vs stated priorities"). Same watermelon
    defense as health: the mix is computed from per-stream goal tags, never asserted.

## The two-page-tier structure (what mm/23 + mm/24 build)

- **Page 1 (mm/23)**: generated-at + commit · portfolio stage-stacked bar with Δ counts ·
  Next-up verbatim (already priority/staleness/capped) · computed exceptions strip (rules in
  legend) · one fixed-order row per stream: health pill + printed reason, stage mini-bar,
  Δ badges, next milestone (wave gate), top blocker, owner.
- **Pages 2..N (mm/24)**: identical skeleton per stream — header band (name · owner · health
  + reason · outcome line · x done / y total) → "since yesterday" delta panel first →
  blockers & asks (issue → effect → action, computed from depends/findings; hand-authored
  asks visually distinct from computed) → next wave gate → brief table grouped by wave with
  days-in-stage staleness ages, collapsed fully-done waves → fixed legend footer.

## Sources

Layouts: prodpad.com/blog/invented-now-next-later-roadmap · aha.io/roadmapping/guide/product-roadmap/what-is-a-product-portfolio-roadmap · productplan.com/templates/executive-facing-portfolio-roadmap · basecamp.com/shapeup/2.2-chapter-08 · about.gitlab.com/direction/plan. Exec reading order: projectmanagercopilot.eu/project-status-report-for-executives · clearpointstrategy.com/blog/performance-reporting-for-executives. Anti-patterns: appcues.com/blog/a-gantt-chart-is-not-a-product-roadmap · cultivatedmanagement.com/watermelon-reporting · instituteprojectmanagement.com/blog/rag-status-in-project-management. Auto-generation: commoncog.com/the-amazon-weekly-business-review · holistics.io/blog/how-amazon-measures · setproduct.com/blog/dashboard-ui-design · geckoboard.com/resources/dashboard-design · mastt.com/resources/project-health-dashboard.
