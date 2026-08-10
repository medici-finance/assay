---
brief: desk-hardening/08
title: Unreviewed desk authority — evidence-not-verdict dispatch + verify-before-apply
why: >
  Nothing in the pipeline reviews the desk. Workers are reviewed, PRs are reviewed, briefs are
  verified by a non-implementer — but a correction the coordinator hands down arrives already
  stamped as authoritative; it is the input to review, not a subject of it. Every gate points
  downstream. So when the desk inverted a heap figure (quoting a crashed probe's cap as the
  successful run's cost) and issued it as ground truth, five workers applied it faithfully into
  nine sites, two PRs, a grant draft, and two of the desk's own briefs. Worse, a dispatch that
  says "X is false — confirm" makes reviewers prosecutors for the desk's premise, so N agreements
  from one premise read as corroboration and are one observation. The remedy: dispatch evidence,
  never a verdict; verify the desk's ground truth against the artifact before applying it.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [48, 49]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#48 (desk-issued ground truth is unverified authority — applied by every worker, reviewed by no one)"
  - "assay-toolkit#49 (a desk that injects an unverified premise into dispatches manufactures its own corroboration)"
  - "in-flight: #49's concrete evidence-not-verdict fix is dispatched separately — this brief scopes the SYSTEMIC remainder (#48's verify-before-apply discipline)"
exec-tier: strong
exec-tier-why: "changes the dispatch-template contract across desks; a subtle wording error (verdict leaking back in as 'evidence') reinstates the manufactured-corroboration failure the brief exists to close."
consumers:
  - "[oit] .claude/skills/{the-desk,pr-review-desk,batch-fanout}/SKILL.md: fixed-here (dispatch template — evidence-not-verdict; worker verify-before-apply default)"
  - "[oit] #556 filing identity: cross-ref (disagreeing-with-the-desk output has somewhere durable to land)"
---

# Brief 08 — Unreviewed desk authority

## Context
files:
- `[oit]` `../oit/.claude/skills/the-desk/SKILL.md` (dispatch template: evidence-not-verdict; primary-source citation in the dispatch)
- `[oit]` `../oit/.claude/skills/pr-review-desk/SKILL.md`, `../oit/.claude/skills/batch-fanout/SKILL.md` (the un-framed-reviewer rule; the worker verify-before-apply default)
- `[toolkit]` `docs/brief-rules.md` or a dispatch-discipline doc (the "measured" rule; the upper-bound rule)
out-of-repo files: none
facts:
- the mechanism (#49): a dispatch saying "X is false — verify" is NOT the instruction "is X true?"; it makes the reviewer a prosecutor for a conclusion the desk already reached; five agents starting from one premise = ONE observation, not five; fanning out MORE reviewers increases confidence without evidence
- the only technique that worked (#49): open the primary artifact and compare the value — every other technique (hedging checks, caveat checks, adversarial passes) waved the same defect through three times
- #48: the desk's output has exactly the dangerous properties — confident, compressed, citation-free by the time a worker sees it, treated as settled; the methodology's stance is "evidence-not-claims applied to the desk most of all", and here the desk shipped a claim with no evidence, to five agents, as evidence
- the generalizable rules this incident produced: a claim that a figure is "measured" must name the artifact that measured it; **a cap a run survived bounds only the quantity it caps — it is not a measurement of consumption, and says nothing about quantities that live outside the cap** (e.g. a V8 old-space heap cap bounds the JS heap only; WASM linear memory and native buffers sit outside it, so the process working set is unbounded by it) (F-38)
- fidelity-to-source cannot catch a wrong source, and the desk IS the source (#37, related decision item) — so there is no reviewer-side fix; the fix is in the dispatch
- #49's concrete evidence-not-verdict change is in-flight as its own PR — scope the REMAINDER (the verify-before-apply discipline of #48, the "measured"/upper-bound rules, the un-framed-reviewer requirement) here, do not re-implement the in-flight piece

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Dispatch carries EVIDENCE, never a VERDICT.** Codify in the dispatch template: ban the shape
   "X is false — confirm"; write "the deck claims X; establish it from the primary source and
   report checked-clean / checked-failed / could-not-check." The desk may state what it *observed*
   ("I queried path P and got 404"), never what it *concluded* ("the issue does not exist"). A
   desk claim with no citable artifact+line is `could-not-check` and must be labelled so IN the prompt.
2. **A desk-issued correction ships its primary-source citation (file:line, or the expression per
   desk-hardening/05) in the dispatch itself** — not a paraphrase. A worker that receives
   `warmer/run.sh:17 → --max-old-space-size=12288` can check it; one that receives "the sanctioned
   wording is X" cannot. Cheapest fix; would have caught the inversion.
3. **Worker verify-before-apply default (#48).** Standing instruction: verify the desk's ground
   truth against the artifact before applying it, and report agreement/disagreement. Make
   disagreeing with the desk an expected output, not insubordination — several workers already do
   this; make it the rule, not the temperament.
4. **The "measured"/scoped-cap rules.** Any claim that a figure is "measured" names the artifact
   that measured it. A cap a run survived bounds only the quantity it caps — it is not a
   measurement of consumption, and it says nothing about quantities that live outside the cap.
   Add both to the dispatch-discipline doc.
5. **Un-framed reviewer (#49).** At least one reviewer per contested fact is dispatched WITHOUT
   the desk's framing — given the artifact and the question, not the conclusion. If its answer
   diverges from the framed reviewers, the desk's premise is the suspect. Never report
   "independently confirmed" for reviewers who received the same assertion — report "N reviewers
   agreed with the premise they were given."

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'evidence, never a verdict\|X is false — confirm\|could-not-check' <the-desk SKILL.md>` | exit 0; ≥ 1 (evidence-not-verdict codified) |
| 2 | `grep -ci 'file:line\|primary source\|in the dispatch' <the-desk SKILL.md>` | exit 0; ≥ 1 (citation-in-dispatch rule) |
| 3 | `grep -ci 'verify.*before.*apply\|disagree\|against the artifact' <batch-fanout SKILL.md>` | exit 0; ≥ 1 (worker default) |
| 4 | `grep -ci 'bounds only the quantity it caps\|a cap a run survived\|names the artifact' <dispatch-discipline doc>` | exit 0; ≥ 1 |
| 5 | `grep -ci 'un-framed\|without the desk.s framing\|one observation' <the-desk pr-review-desk SKILL.md>` | exit 0; ≥ 1 |

## Verify note
Skill/prose deliverables — PRESENCE gates; the review gate owns whether the wording actually
removes the verdict shape. Do NOT re-scope #49's in-flight concrete fix.

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST check the new dispatch template does not
smuggle a verdict back in under the word "evidence" (e.g. "the evidence shows X is false — verify")
— that reinstates the exact failure.
