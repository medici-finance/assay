---
brief: methodology-metrics/40
title: 'opmetrics — local operator-layer collector: relay ratio, intervention rate, decision latency, session hygiene → aggregates-only day-file'
wave: 1
depends: []
unblocks: ["methodology-metrics/41", "methodology-metrics/42"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
value: high
authored: 2026-07-22 by Opus 4.8 session (human:<name> direction, from #766)
sources: ["#766 (adoption-ladder metrics, section (a) operator metrics — this brief implements it)", "#766 2026-07-18 addendum comment (behavior defines the step; markers)", "docs/streams/methodology/scada-ooda-lineage.md (operator-as-instrument lens)", "#759–#765 (the 07-15→18 usage-retro remedies these metrics measure)", "freshness-checked 2026-07-22 @ ef1de62a (no existing brief/tool covers operator metrics; grep opmetrics/relay across streams+tools = empty)"]
gate-why: >-
  The collector parses PRIVATE session transcripts (~/.claude/projects/**.jsonl — prompts,
  pasted content, potentially secrets) and commits a derived artifact to the repo. The human
  gate confirms, before the collection loop goes live, that the emitted day-file is
  aggregates-only (counts/ratios/percentiles — no transcript text, prompts, or paths) and
  that the fixture-based leak test (Verify #3) genuinely covers the emit path.
exec-tier: strong
exec-tier-why: (a) message-classification heuristics (relay vs substantive, zombie detection) are judgment calls the facts only bound, not fully pre-specify.
consumers:
  - "docs/reports/daily/<date>/opmetrics.json (NEW artifact schema): follow-up methodology-metrics/41 (autonomy/token metrics read it), follow-up methodology-metrics/42 (ladder indicator reads it)"
why: >-
  The 07-15→18 usage retro found the operator (human:<name>) acting as a message bus — ~20–25% of his
  messages were pure state relays, ~2.2 messages per merged PR — and the #759–#765 remedies were
  filed to fix it. Nothing measures whether they worked: the adoption ladder (#766) says step-3
  progress is operator decision-throughput, and today that layer has zero instrumentation. This
  collector turns "does the desk still need human:<name> as plumbing?" into a daily number.
---

# Brief 40 — opmetrics: local operator-layer collector

## Context
files:
- `../assay-toolkit/tools/desk/cmd/opmetrics/` — NEW Go command (module `../assay-toolkit/tools/desk/go.mod`, reuse `internal/deskkit`)
- `../assay-toolkit/tools/desk/README.md` — one section documenting the command + the day-file schema
- `docs/streams/methodology-metrics/README.md` — convention line (authored in this brief set)

facts:
- **Where the data lives (verified 2026-07-22 — this is the head of the critical path):** session
  transcripts are `~/.claude/projects/<project-slug>/*.jsonl`; dispatch claims are
  `~/.claude/desk-tools/claims/*.claim`; roster beacons `~/.claude/desk-tools/roster/<session>.json`
  (types in `../assay-toolkit/tools/desk/cmd/deskroster/roster.go`). **None of these exist on the CI runners**
  (`medici-builder`, where `daily-harvest.yml` runs) — so #766's "run by the daily harvest" is
  WRONG as written; the collector runs on the OPERATOR machine and its day-file rides into
  `docs/reports/daily/<date>/` alongside (not via) the CI harvest. The transcript source is
  proven sufficient: the 07-18 retro computed relay ratio ≈20–25% by hand from exactly these files.
- **Metrics (from #766 (a), with the retro baselines):** relay ratio (% operator msgs that are
  state relays/pokes — "sync to main", "X is merged", "where are…", re-sent near-identical
  prompts; baseline ~20–25%); intervention rate (operator msgs per merged PR; baseline ~2.2);
  decision latency (ready-flip or needs-decision label → merge/decision, P50/P90, via `gh`);
  session hygiene (sessions >24h; zombie agents: roster beacons `Updated` >60 min stale with
  open work); correction recurrence (same behavioral rule corrected >1× in 7 days — heuristic:
  repeated near-identical short corrective messages; report candidates, don't over-claim).
  Prompt blockage (permission prompts) is OUT OF SCOPE — transcripts don't reliably record
  prompt-wait spans; note it as unmeasured rather than faking it.
- **Emit**: `opmetrics --root <repo> --date YYYY-MM-DD` writes
  `docs/reports/daily/<date>/opmetrics.json` — AGGREGATES ONLY: counts, ratios, percentiles,
  and dispatch-claim COUNTS (per-day claims filed, sessions active). Never message text, never
  prompt excerpts, never file paths from transcripts, never session IDs beyond a stable short
  hash. The schema (documented in tools/desk/README.md) is the interface mm/41 and mm/42 consume.
- **Classification is versioned**: the relay/zombie/correction heuristics live in one file with a
  `classifierVersion` field emitted in the JSON, so trend breaks caused by heuristic changes are
  attributable (anti-gaming: these are diagnostics, never scorecards — stream rule).
- **Landing path**: the day-file is committed by the operator's local routine (operator machine is
  a sanctioned main-writer per F-13 backstop; small doc commits). Wiring the local schedule
  itself (`~/.claude/scheduled-tasks/…`) is OUT OF SCOPE for this brief — deliver the tool +
  a documented one-line invocation; human:<name> wires the routine (out-of-repo surface, rule-7 avoided).
- 7-day trend: the tool also accepts `--trend 7` reading the prior day-files it finds under
  `docs/reports/daily/` and appending a `trend` block (deltas) to today's emit.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- NEVER commit real transcript-derived output — tests use FIXTURES only; the real day-file is
  produced by the operator's routine post-merge.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first, against a fixture transcript dir (synthetic .jsonl in testdata/):
   relay classification (relay vs substantive), intervention rate joins msgs to merged-PR count,
   decision latency percentiles from fixture gh JSON, zombie detection from fixture beacons,
   aggregates-only emit (see Verify #3 leak test).
2. Implement `../assay-toolkit/tools/desk/cmd/opmetrics` per facts. `--transcripts`, `--desk-tools`, `--gh-fixture`
   flags so tests never touch the real home dir; defaults resolve to `~/.claude/...`.
3. Document command + JSON schema (with `classifierVersion`) in `../assay-toolkit/tools/desk/README.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./cmd/opmetrics/ -v` | exit 0; tests cover relay ratio, intervention rate, decision latency, zombie detection, trend block |
| 2 | `cd tools/desk && go vet ./... && go build ./...` | exit 0 |
| 3 | `cd tools/desk && go test ./cmd/opmetrics/ -run Leak -v` | exit 0; `TestLeak*` plants a sentinel secret string in a fixture transcript, runs the full emit, asserts the sentinel (and any ≥12-char fixture message substring) is ABSENT from the emitted JSON |
| 4 | `cd tools/desk && go run ./cmd/opmetrics --transcripts cmd/opmetrics/testdata/transcripts --desk-tools cmd/opmetrics/testdata/desk-tools --gh-fixture cmd/opmetrics/testdata/gh.json --root /tmp/opm-verify --date 2026-07-22 && jq -e '.relay_ratio and .classifierVersion' /tmp/opm-verify/docs/reports/daily/2026-07-22/opmetrics.json` | exit 0 |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data yes — see gate-why). The reviewer additionally confirms the emit path
has no code path that copies message text into the artifact, and that defaults never point tests
at the real `~/.claude`. Verdict + date in the stream README table; `done` requires `human:<name>`.
