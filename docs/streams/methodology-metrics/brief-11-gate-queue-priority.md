---
brief: methodology-metrics/11
title: Gate-queue prioritization — review and verify loops drain by brief priority, not arrival order
wave: 1
depends: ["methodology-metrics/09"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: the review/verification loops should prioritize which they do, similar to the worker-queue prioritization", "human:<name> 2026-07-09: daml-hardening/01 on the critical path of many briefs, stuck awaiting verify — review+verify gates also need to look at brief priority", "tools/statusgen/nextup.go (the score being mirrored)", "docs/streams/methodology-metrics/research-prioritization-systems-2026-07-09.md", "methodology-metrics/10 (the depth alarm this adds ordering to)", "issue #221 (out-of-repo skill edit protocol)", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  Prioritization currently stops at `implemented`: Next-up carefully scores what workers pick
  up, but both gate loops drain their queues in arrival/list order. With verification the
  system's standing constraint (32 awaiting vs 17 done at authoring), an unordered drain means
  a brief blocking half a stream waits behind trivia — daml-hardening/01 sat exactly there.
  Throughput at the constraint should follow the same value signal as intake to it.
---

# Brief 11 — Gate-queue prioritization (review + verify loops)

## Context
files: `../assay-toolkit/statusgen/` (gate-score computation + ordered rendering of the
Awaiting-verification/review queue), `../assay-toolkit/tools/desk/cmd/deskboard/` coordination note (brief
desk-tools/02 — PR ordering consumes the same score)
out-of-repo files: `~/.claude/skills/verify-desk/SKILL.md`, `~/.claude/skills/pr-review-desk/SKILL.md`
(one line each: drain top-of-queue first — see rule 7 / issue #221 protocol)
facts:
- **One score, both gates.** gate-score(brief) = priorityWeight(stream priority) +
  unblocksWeight × blockedCount(brief) + days-in-queue × stalenessPerDay, where
  blockedCount = the number of not-done briefs TRANSITIVELY gated on this brief reaching
  `done` (walk the typed `depends:` graph in reverse — methodology-metrics/09 already
  builds the typed-deps machinery; reuse it, do not reimplement). priorityWeight and
  stalenessPerDay reuse nextup.go's constants; unblocksWeight is new — start it at
  2× priorityWeight(P1) so a brief blocking 3+ others outranks a P0 blocking none, and
  document the constant as a tunable, not a truth.
- **Verify queue (the constraint):** statusgen renders the Awaiting-verification/review
  queue ORDERED by gate-score descending, with two new columns: score and blockedCount.
  The board IS the order — verify-desk drains top-first; no separate tool.
- **Review queue:** deskboard (desk-tools/02) orders actionable PRs (NEEDS-REVIEW /
  RE-REVIEW) by the owning brief's gate-score — PR→brief mapping via the branch's brief row
  (branch-as-claim). PRs with no owning brief (register PRs, docs) take default weight,
  tie-break oldest-first. This lands as a coordination note amendment in desk-tools/02's
  Context if 02 is still todo (one-line, changes no Verify row); as follow-up work there if
  02 already shipped. statusgen exposes the score (e.g. `--gate-scores` JSON) so deskboard
  never re-derives the graph.
- **Scope guard ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) discipline):** this brief does NOT change the Next-up worker-queue
  score — that's scoring-v2 ([I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md)) territory. Same honest limitation applies to gate-score
  as to Next-up scoring: staleness rewards age, blockedCount has no value/effort term; the
  README paragraph documenting the formula says "evolving heuristic," verbatim discipline
  from the [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) note. When [I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md) lands a better worker score, the gate score follows it —
  note the coupling in the formula doc.
- Ordering is GUIDANCE the loops follow, not a hard gate: an urgent human request ("review
  #N now") legitimately jumps the queue; the loops record queue-jumps by simply doing them —
  no ceremony, the ordered board just makes the default drain order visible and right.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo skill files follow issue #221's protocol: declared above; apply live edits as
  the LAST step before `implemented`; paste before/after diffs into the PR body.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. statusgen: compute blockedCount by reverse-walking typed `depends:` from each
   awaiting-queue brief over all not-done briefs (cycle-safe; a malformed/unknown typed ID
   contributes 0 and emits a NOTICE, never panics). Unit tests: chain, diamond, cross-stream
   edge, cycle, unknown-ID.
2. statusgen: order the Awaiting-verification/review queue by gate-score descending; render
   score + blockedCount columns; add the formula paragraph (with the evolving-heuristic
   limitation) to the section header or the methodology-metrics README.
3. statusgen: `--gate-scores` emitting `{brief, score, blockedCount, stream, status}` JSON
   for deskboard consumption.
4. desk-tools/02 coordination: if still `todo`, add the one-line Context note (PR ordering
   consumes `--gate-scores`); if implemented, file the follow-up as a one-line INTAKE entry
   instead — do not implement deskboard changes under this brief.
5. Out-of-repo (per #221): one line in verify-desk and pr-review-desk SKILL.md each — drain
   the board's rendered order top-first; human requests may jump the queue.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes chain/diamond/cycle/unknown-ID blockedCount tests |
| 2 | `statusgen --root . --gate-scores \| head -c 400` | valid JSON rows with brief/score/blockedCount |
| 3 | regenerate STATUS.md on a scratch copy: the Awaiting queue section is score-ordered with score + blockedCount columns | observed; a brief with blockedCount ≥3 sits above same-stream blockedCount 0 |
| 4 | PR body contains before/after diffs of both skill one-liners (#221 protocol) | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `a6286beb`, PR #705, 2026-07-19)

| # | Run | Result |
|---|-----|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; 7 `TestGateScores*` cases (chain/diamond/cross-stream/cycle-safe/unknown-dep/ordering/ignores-non-awaiting) |
| 2 | `--root . --gate-scores \| head -c 400` | valid JSON rows: brief/score/blockedCount/stream/status |
| 3 | STATUS.md Awaiting queue | committed (main-CI) Awaiting table has Score + Blocked cols, score-descending, blockedCount≥3 ranks above same-stream blockedCount 0 (read committed file per no-full-regen guard) |
| 4 | PR body out-of-repo diffs (#221) | PR #705 body carries before/after diffs for verify-desk + pr-review-desk `SKILL.md` one-liners; both live |
| 5 | `--root . --lint; echo $?` | exit 0 |

VERIFY: PASS — all 5 rows. gate-score reuses `buildRevDeps`/`blockedCount` from `nextup.go` (shared with brief 14, no re-derivation); desk-tools/02 follow-up routed to INTAKE `I-deskboard-gateorder`. `gate: model`, all risks `no` → flip.

## Review
Gate: model. Reviewer confirms (a) blockedCount walks TYPED depends transitively and is
cycle-safe, (b) the worker-queue Next-up score is untouched ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)/[I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md) scope guard), (c) the
formula doc carries the evolving-heuristic limitation verbatim discipline, (d) the #221
declaration matches the files actually touched.
