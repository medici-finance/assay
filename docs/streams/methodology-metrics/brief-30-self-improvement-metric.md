---
brief: methodology-metrics/30
title: 'Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched'
why: >-
  The whole thesis is a system that improves itself. We should be able to see it: how many issues did
  the loops NOTICE from their own experience, file, and RESOLVE on their own — versus the ones a human
  had to raise, direct, decide, or fix. That ratio (the autonomy / self-healing rate) is the single
  clearest measure of whether the machine is getting better at fixing itself, and whether that rate is
  rising over time. human:<name> 2026-07-16 wants it captured.
wave: 5
depends: ["methodology-metrics/28", "methodology-metrics/29"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources: ["human:<name> 2026-07-16: 'capture the self-improvement metrics — where the loops filed an issue about something they were seeing/experiencing and it got resolved vs one a human touched'", "methodology-metrics/28 (the --issues infra + author-class this extends)", "methodology-metrics/29 (the raised-by:<desk> label — identifies which desk noticed the problem)", "the 2026-07-16 backlog-close sweep (real data: most of the ~28 closed were agent-raised + agent-fixed — e.g. verify-desk filed #541/#575, workers filed bugs they hit)", "freshness-checked 2026-07-16 @ e00e719d (brief-28/29 provide author-class + raised-by; no self-improvement/human-touch cut exists yet)"]
---

# Brief 30 — Self-improvement metric (self-healed vs human-touched)

## Context
files: extends `../assay-toolkit/statusgen/issues.go` (planned) (brief-28 deliverable) with a self-improvement
classifier + segment; `../assay-toolkit/statusgen/issues_test.go` (planned).
facts:
- **The classification, per RESOLVED (closed) issue:**
  - **SELF-HEALED (closed-loop autonomy)** — ALL of: (a) **agent-raised** — author is an agent
    (`the-org` / a `*[bot]`) OR the issue carries a `raised-by:<desk>` label (brief-29); (b) **fixed
    by an agent-authored PR** (the merged fixing PR's commits are agent-authored); (c) **no human
    intervention in the lifecycle** (see the touch definition below).
  - **HUMAN-TOUCHED** — any of: human-*raised* (author `human:<name>` / a non-team human), a human
    *comment* on the issue, a `needs-decision` the human answered, a human *reopen* or *manual close*,
    or a *human-authored* fixing PR.
- **THE LOAD-BEARING CAVEAT — the standing merge gate is NOT a "human touch".** human:<name> merges every PR;
  if "merged by human:<name>" counted as a touch, nothing would ever be self-healed and the metric would be
  useless. A "touch" is human **diagnosis / direction / decision / intervention** — raising it,
  commenting to steer, deciding a fork, reopening, closing by hand, or authoring the fix. The merge
  approval is the system's *standing* gate on all writes, orthogonal to who *found and fixed* the
  problem. State this in the output so the number is read correctly.
- **The metric:** `self_improvement_rate = self-healed ÷ (self-healed + human-touched-among-resolved)`,
  plus the raw counts and a **`--series`** trend (is autonomy rising?). Also break the human-touched
  bucket by touch TYPE (raised / steered-by-comment / decided / reopened / fixed-by-human) so it's
  clear WHERE the human was needed.
- Signals reused: author class (brief-28), `raised-by:<desk>` (brief-29), the issue timeline
  (`gh api repos/<r>/issues/N/timeline` — comments/reopens/closes + actor), the fixing PR's author.
- Diagnostic banner rides the output (same as --dora/--issues): never a target or a scoreboard.

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl beyond the standing branch+draft-PR flow.
- Stop at `implemented`. NEEDS_CONTEXT over guessing (esp. the "agent vs human" account allowlist —
  reuse brief-28's `--team-logins`; a human on the team (`human:<name>`) is still a human touch for THIS
  metric, distinct from brief-28's internal/external axis).

## Task
1. Extend the issue tool (brief-28) with the self-improvement classifier: per resolved issue, derive
   agent-raised, agent-fixed, and human-touch (with touch-type) from author + `raised-by` + timeline +
   fixing-PR author.
2. Emit `self_improvement_rate` + counts + the human-touched-by-type breakdown; `--json` + `--series`.
3. Carry the merge-gate-is-not-a-touch caveat in the output verbatim.
4. Tests: the classifier (a fully self-healed issue; one steered by a `human:<name>` comment; a human-raised
   one; a human-authored fix), the rate math, and the merge-not-a-touch rule (an agent-raised +
   agent-fixed issue that human:<name> merged counts as self-healed).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --issues --self-improvement` (literal flag per Evidence) | exit 0; prints self-healed vs human-touched counts + rate |
| 2 | `statusgen --root . --issues --self-improvement \| grep -iE -e 'self-healed' -e 'human-touched' -e 'merge.*not.*touch'` | ≥1 — segments + the merge-gate caveat render |
| 3 | `statusgen --root . --issues --self-improvement --json \| jq -e '.selfHealed,.humanTouched,.selfImprovementRate,.humanTouchedByType'` | exit 0 — JSON carries the cut |
| 4 | `go test ./tools/statusgen/ -run 'SelfImprove|Autonomy|Touch' -count=1` | exit 0 |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
