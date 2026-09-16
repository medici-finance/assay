---
brief: assay:assay:statusgen:03
title: 'Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched'
wave: 2
depends: ["statusgen/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-20 (authored clean for the statusgen board)
sources:
  - "A maintainer directive (2026-07-16): capture the self-improvement metric — where the loops filed an issue about something they were seeing/experiencing and it got resolved vs one a human touched"
  - "statusgen/02 (the --issues infra + author-class this extends)"
  - "The `raised-by:<desk>` label (identifies which desk noticed the problem)"
  - "A real backlog-close sweep whose data showed most closed issues were agent-raised + agent-fixed (a verify desk filed the bug, a worker filed a bug it hit)"
why: >-
  The whole thesis is a system that improves itself. We should be able to see it: how many issues did
  the loops NOTICE from their own experience, file, and RESOLVE on their own — versus the ones a human
  had to raise, direct, decide, or fix. That ratio (the autonomy / self-healing rate) is the single
  clearest measure of whether the machine is getting better at fixing itself, and whether that rate is
  rising over time.
version: 1
id: 8b86c40d-4687-4c5c-b80a-69d4fc4000e4
---

# Brief 03 — Self-improvement metric (self-healed vs human-touched)

## Context
files: extends `statusgen/issues.go` (the statusgen/02 deliverable) with a self-improvement
classifier + segment; `statusgen/issues_test.go` (planned).

This brief builds on statusgen/02's `--issues` infrastructure (author classification + the
`raised-by:<desk>` label). Where statusgen/02 leaves the by-desk cut `unattributed` until the desks
stamp the label, this metric degrades the same way — a resolved issue with no raised-by stamp is
classified from the author signal alone.

facts:
- **The classification, per RESOLVED (closed) issue:**
  - **SELF-HEALED (closed-loop autonomy)** — ALL of: (a) **agent-raised** — author is an agent
    (the automation account / a `*[bot]`) OR the issue carries a `raised-by:<desk>` label; (b) **fixed
    by an agent-authored PR** (the merged fixing PR's commits are agent-authored); (c) **no human
    intervention in the lifecycle** (see the touch definition below).
  - **HUMAN-TOUCHED** — any of: human-*raised* (a non-team human author), a human
    *comment* on the issue, a `needs-decision` the human answered, a human *reopen* or *manual close*,
    or a *human-authored* fixing PR.
- **THE LOAD-BEARING CAVEAT — the standing merge gate is NOT a "human touch".** A human maintainer
  merges every PR; if "merged by a human" counted as a touch, nothing would ever be self-healed and
  the metric would be useless. A "touch" is human **diagnosis / direction / decision / intervention**
  — raising it, commenting to steer, deciding a fork, reopening, closing by hand, or authoring the
  fix. The merge approval is the system's *standing* gate on all writes, orthogonal to who *found and
  fixed* the problem. State this in the output so the number is read correctly.
- **The metric:** `self_improvement_rate = self-healed ÷ (self-healed + human-touched-among-resolved)`,
  plus the raw counts and a **`--series`** trend (is autonomy rising?). Also break the human-touched
  bucket by touch TYPE (raised / steered-by-comment / decided / reopened / fixed-by-human) so it's
  clear WHERE the human was needed.
- Signals reused: author class (statusgen/02), the `raised-by:<desk>` label, the issue timeline
  (`gh api repos/<r>/issues/N/timeline` — comments/reopens/closes + actor), the fixing PR's author.
- Diagnostic banner rides the output (same as `--dora`/`--issues`): never a target or a scoreboard.

## Ground rules
- NEVER git push / trigger workflows beyond the standing branch+draft-PR flow.
- Stop at `implemented`. NEEDS_CONTEXT over guessing (esp. the "agent vs human" account allowlist —
  reuse statusgen/02's `--team-logins`; a human on the team is still a human touch for THIS metric,
  distinct from statusgen/02's internal/external axis).

## Task
1. Extend the issue tool (statusgen/02) with the self-improvement classifier: per resolved issue,
   derive agent-raised, agent-fixed, and human-touch (with touch-type) from author + `raised-by` +
   timeline + fixing-PR author.
2. Emit `self_improvement_rate` + counts + the human-touched-by-type breakdown; `--json` + `--series`.
3. Carry the merge-gate-is-not-a-touch caveat in the output verbatim.
4. Tests: the classifier (a fully self-healed issue; one steered by a human comment; a human-raised
   one; a human-authored fix), the rate math, and the merge-not-a-touch rule (an agent-raised +
   agent-fixed issue that a human merged counts as self-healed).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --issues --self-improvement` | exit 0; prints self-healed vs human-touched counts + rate |
| 2 | `statusgen --root . --issues --self-improvement \| grep -iE -e 'self-healed' -e 'human-touched' -e 'merge.*not.*touch'` | ≥1 — segments + the merge-gate caveat render |
| 3 | `statusgen --root . --issues --self-improvement --json \| jq -e '.selfHealed,.humanTouched,.selfImprovementRate,.humanTouchedByType'` | exit 0 — JSON carries the cut |
| 4 | `cd statusgen && GOWORK=off go test . -run SelfImprovement -count=1` | exit 0 — the classifier tests (self-healed, human-touched-by-type, and the merge-is-not-a-touch rule) run and pass |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- filled by a non-implementer at verify time -->
Verified against merged main at SHA `4fdfde1fb3f1843ce58f021fb709e282b236a718`.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `statusgen --root . --issues --self-improvement` | exit 0; self-healed vs human-touched counts + rate | exit 0. Printed: Resolved=1013, self-healed=44, human-touched=835, rate=0.05, unclassified=134, by-type breakdown (raised 581 / steered-by-comment 359 / decided 134 / reopened-or-manual-close 360 / fixed-by-human 56), plus the merge-gate caveat banner. Zero could-not-check entries. | 2026-09-15 | sonnet-5-verifier |
| 2 | `... \| grep -iE -e 'self-healed' -e 'human-touched' -e 'merge.*not.*touch'` | ≥1 match | exit 0, 5 matching lines — both segment headers and "THE STANDING HUMAN MERGE GATE IS NOT A TOUCH" caveat rendered | 2026-09-15 | sonnet-5-verifier |
| 3 | `... --json \| jq -e '.selfHealed,.humanTouched,.selfImprovementRate,.humanTouchedByType'` | exit 0, JSON carries the cut | exit 0. Returned 44, 835, "0.05", and humanTouchedByType (raised 581, steeredByComment 359, decided 134, reopened 360, fixedByHuman 56) | 2026-09-15 | sonnet-5-verifier |
| 4 | `cd statusgen && GOWORK=off go test . -run SelfImprovement -count=1` | exit 0, classifier tests pass | exit 0. All 16 TestSelfImprovement* tests PASS in 0.261s, including TestSelfImprovementMergeIsNotATouch | 2026-09-15 | sonnet-5-verifier |
| 5 | `statusgen --root . --lint` | exit 0 | exit 0, LINT: PASS, 0 PROBLEM-level findings | 2026-09-15 | sonnet-5-verifier |

**Risk-bearing-value analysis.** Classification rule literals are at `statusgen/selfimprovement.go`
lines 106, 135-144, 146-151, 154, 382-388, 437-443. The merge-gate exclusion is structural: the
fetch layer never reads a "merged-by" field at all, so a human merge cannot be counted as a touch
by construction. Of the two easy gaming routes for this metric — a bare self-close, and a
merge-gate false-positive — both are structurally closed by the classifier design. The residual
gaming surface (an empty-but-linked fix PR counted as agent-fixed) is an inherent, disclosed,
out-of-brief-scope limitation, not a defect.

VERIFY: PASS

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
