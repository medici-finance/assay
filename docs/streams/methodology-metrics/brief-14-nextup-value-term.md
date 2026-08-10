---
brief: methodology-metrics/14
title: 'Next-up value term — explicit value field + held-up-by count in the score ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md), R-01 decision)'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (R-01 follow-on)
sources: ["RETRO R-01 (human:<name>'s knob decision, 2026-07-10: value component = explicit value field + held-up-by term)", "[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) (the scoring weakness this closes)", "docs/streams/methodology-metrics/research-prioritization-systems-2026-07-09.md (aging-from-item-entry finding)", "methodology-metrics/11 (blockedCount machinery to reuse)", "freshness-checked 2026-07-10 @ post-mm12 main"]
why: >-
  Next-up scores priority + staleness only — [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)'s recorded weakness: it rewards neglect
  regardless of why a stream aged and has no notion of what a brief is worth or what it
  holds up. human:<name> decided the shape at R-01; this brief is the deferred statusgen change.
---

# Brief 14 — Next-up value term

## Context
files: `../assay-toolkit/statusgen/nextup.go` (+ tests), `../oit/.claude/skills/author-brief/SKILL.md`
(in-repo wrapper — one line documenting the optional field)
facts:
- New OPTIONAL brief-v1 field `value: low | med | high` (absent = med); lint validates the
  value (invalid → PROBLEM), never requires the field.
- Score becomes: `priorityWeight + staleness + valueWeight(value) + unblocksWeight ×
  blockedCount`, where blockedCount is the reverse transitive typed-`depends:` walk —
  REUSE methodology-metrics/11's gate-score machinery, do not reimplement. Constants
  documented as tunables in the formula paragraph (evolving-heuristic wording preserved,
  [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) discipline).
- ALSO fix [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)'s clock defect (research-doc finding): staleness counts from the BRIEF's
  own last transition (historian log), not the stream's LastTouch — a rival stream's git
  activity must no longer reset an item's aging. Fall back to stream LastTouch only when
  the historian has no row for the brief.
- Never let value/blockedCount suppress starvation protection: aging still eventually
  floats anything (the OS-scheduler rationale in the research doc).
- exec-tier (methodology/29) and this field are orthogonal; no interaction.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the field + score change + clock fix per facts; formula paragraph updated
   where it lives (STATUS header / stream README — find the current home, single-source it).
2. Tests: value ordering at equal priority; blockedCount pulls a blocker above a same-
   priority non-blocker; staleness unaffected by rival-stream touches (fixture historian);
   invalid value → lint PROBLEM; absent field = med.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes every Task-2 case |
| 2 | `statusgen --root . --lint; echo $?` | 0 |
| 3 | scratch STATUS regen: a brief with blockedCount ≥3 outranks a same-priority blockedCount-0 sibling | observed |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok github.com/medici/statusgen 1.162s` — value-ordering, blocker-promotes, staleness-clock-fix + rival-stream-touch-immunity, invalid-value→PROBLEM cases all pass | 2026-07-10 | opus-verifier |
| 2 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | NOTICE lines only, no PROBLEM; exit 0 | 2026-07-10 | opus-verifier |
| 3 | scratch STATUS regen: blockedCount ≥3 outranks same-priority blockedCount-0 sibling | 0 | `TestNextUpBlockedCountOutranksInRender` observed ordering `[ablock/00 zfree/00]` — blocked sorts above free sibling | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — Next-up value term reuses mm/11 shared blockedCount machinery, staleness clock reads the brief's own historian row with stream-touch fallback, starvation-aging preserved.

## Review
Gate: model. Reviewer confirms blockedCount is reused from mm/11 (not duplicated), the
staleness clock reads the brief's own history, and starvation protection survives.
