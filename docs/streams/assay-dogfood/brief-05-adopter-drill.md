---
brief: assay-dogfood/05
title: Adopter drill — a clean consumer onboards from the marketplace alone; every gap becomes an issue
wave: 3
depends: ["assay-dogfood/04"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md))
sources: ["INTAKE [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)", "INTAKE [I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md) (productization — this drill is its evidence)", "INTAKE [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) ('dogfooding the exact consumption model an external Assay adopter would have')", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  The stream's claim is "an adopter can pick this up" — that claim is only real if a
  consumer with NONE of this machine's accumulated state onboards from the published
  artifacts alone. The drill converts every stumble into an assay-toolkit issue, which is
  both the fix pipeline and the proof the governance loop (outsiders propose via issues)
  works end to end.
---

# Brief 05 — Adopter drill

## Context
files: drill report lands as docs/streams/assay-dogfood/evidence/adopter-drill.md — a new
file in this repo; gaps land as issues on medici-finance/assay-toolkit
facts:
- The clean consumer: `../agent-runtime` (a real sibling repo that has never carried the
  desk skills) driven by a FRESH session — plus, if cheap to arrange, a scratch OS user so
  no `~/.claude` state leaks in. The drill runner follows ONLY: assay-toolkit README +
  marketplace install + this repo's public stream docs. No transcript archaeology, no desk
  memory.
- Drill script (each step timed, pass/fail, gap notes): ① marketplace add + plugin install
  from the tag; ② statusgen pinned-binary install + hash verify; ③ author one toy brief in
  agent-runtime using `assay:author-brief`; ④ run a mini review loop on its PR; ⑤ file one
  methodology-change proposal as an assay-toolkit issue (the governance loop exercised for
  real, expecting the 403-boundary experience brief 01 built).
- Every gap = one assay-toolkit issue with the drill step, expected/actual, and severity
  (blocker = an adopter is stuck; paper-cut = friction). The drill report indexes them.
- Success bar is honest: the drill may FAIL steps — a failed step with a filed issue is a
  successful drill outcome; an undocumented workaround is not. Do not polish gaps away in
  the report ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) discipline).
- This drill's numbers (steps passed, time-to-onboard, issue count) are Article-1-grade
  material — note them for methodology/09's post-R-01 pass, subject to its FORBIDDEN
  NUMBERS sourcing rules (every figure traceable to the drill report).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Filing assay-toolkit issues is
  in-scope (that repo's permission model invites it). Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Prepare the drill script as a checklist with expected outcomes per step (before running —
   the expectations are the test).
2. Run the drill per facts with a fresh session on ../agent-runtime; capture the report.
3. File the gap issues on assay-toolkit; index them in the report.
4. One-paragraph verdict in the report: could a motivated external adopter onboard today —
   yes/no/with-which-blockers.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/assay-dogfood/evidence/adopter-drill.md && grep -c "①\|②\|③\|④\|⑤\|Step" docs/streams/assay-dogfood/evidence/adopter-drill.md` | ≥5 (all steps recorded) |
| 2 | `gh issue list --repo medici-finance/assay-toolkit --json number \| jq length` | ≥1 (the governance loop was exercised — at minimum the step-⑤ proposal) |
| 3 | `grep -ci "verdict" docs/streams/assay-dogfood/evidence/adopter-drill.md` | ≥1 |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item;
     the drill report is itself the primary evidence artifact. -->

## Review
Gate: model. Reviewer checks the report for polished-away gaps (workarounds used but not
filed), and that step ⑤ was performed by the CONSUMER identity — the boundary experience is
the thing under test, not a formality.
