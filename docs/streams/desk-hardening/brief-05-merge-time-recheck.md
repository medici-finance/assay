---
brief: desk-hardening/05
title: Merge-time re-check + cite-by-expression + body/Verify re-derive
why: >
  The review gate asks "is this PR correct against main as it was when I reviewed it?" — but main
  moves between review and merge, and nothing re-asks the question against the tree the PR actually
  lands in. Five times in one evening the natural merge would have silently reverted landed work
  or shipped a defect the review could not see; four were caught only because someone diffed
  against MERGED main instead of trusting the PR. Compounding it, PR bodies and Verify tables go
  stale after a main-sync/version-bump and end up contradicting the code the human signed against.
  This occupies the review→merge gap that no instrument holds.
wave: 1
depends: ["desk-hardening/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [54, 74]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#54 (RETRO: nothing checks correctness at merge-time — 5 near-misses in one evening)"
  - "assay-toolkit#74 (PR bodies/Verify tables go stale after main-sync+version bumps and contradict the code — undermines the human sign-off gate)"
  - "inherits desk-hardening/01: verify against the tree the artifact lands in, not a convenient proxy"
exec-tier: strong
exec-tier-why: "a fail-open merge gate is safety plumbing — a subtle error (a stale-base merge slipping through) survives the brief's own tests; correctness spans review-time vs merge-time trees."
consumers:
  - "[oit] .claude/skills/pr-review-desk/SKILL.md: fixed-here (delta/conflict review discipline; body-vs-diff re-check)"
  - "[oit] a CI check / .github/workflows/: follow-up (head-not-current-with-main gate for shared-file PRs; no-version-bump assertion matches reality)"
---

# Brief 05 — Merge-time re-check + cite-by-expression + body/Verify re-derive

## Context
files:
- `[oit]` `../oit/.claude/skills/pr-review-desk/SKILL.md` (merge-time discipline; body/Verify re-derive rule)
- `[oit]` a CI check or `.github/workflows/` gate (head-current-with-main; assertion-matches-reality)
- `[toolkit]` `docs/brief-rules.md` (cite-by-expression convention; material-diff-change re-derive rule)
out-of-repo files: none
facts:
- the gap (#54): review's `main` and merge's `main` are different worlds; a stale branch produces a *clean* merge through the one file that doesn't conflict, and a clean merge reads as "nothing to see"
- the emergent discipline (codify it): every delta/conflict review diffs against **merged main** (3-dot / `origin/main`), never the prior head; verifies any artifact against the SOURCE, never a previous render; names the safe merge order + exact resolution when a PR shares a file with another open/recently-merged PR
- **CONFLICTING resolution touching the PR's own files = a new change ⇒ mandatory RE-REVIEW** (already house rule; enforce it — none of the five would have merged blind if this always held)
- **cite by EXPRESSION, not line number** — a citation naming what it points at cannot go stale when the file moves (the #18/oit#515 convergence); make it the house convention
- #74: nothing forces the PR body/Verify table to be re-derived when the diff materially changes (version bump, DAR part-count, a reverted design decision); on `gate: human` briefs the human signs the BODY, so a stale body means the signature attests to fiction (oit#524 asserted a funds-protection property the code had reverted)

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Merge-time re-check (the core ask). Candidate approaches, cheapest first:**
   (a) *Discipline* — codify in the reviewer dispatch template: CONFLICTING resolution on the
   PR's own files ⇒ RE-REVIEW against merged main; diff 3-dot vs `origin/main`, never prior head.
   (b) *CI gate* — fail a PR whose `head` is not current with `main` when it touches shared files
   (any register, any generated artifact, shared config), forcing a fresh merge + re-review.
   (c) *Re-run assertions* against current `origin/main`, not the review-time SHA.
   Recommend (a)+(b) together — (a) covers all five instances, (b) makes it mechanical.
2. **Cite-by-expression convention** (`docs/brief-rules.md`): citations name the expression they
   point at, not a `file:line`; optionally a linter that resolves every reference at merge-time
   HEAD and fails on one that lands on the wrong content (e.g. a citation now pointing at a code
   fence, oit#515).
3. **Body/Verify re-derive rule (#74):** a materially-changed diff (version bump, DAR part-count
   change, a fix reverting a claimed design decision) REQUIRES updating the PR body + Verify table
   in the same push; mandatory on `gate: human` briefs. Reviewer discipline: every delta
   re-review checks the body + Verify table against the *current* diff and flags any claim the
   diff contradicts as a blocker.
4. **(Optional) cheap guard** for briefs whose body asserts "no version bump / no DAR change": a
   CI/pre-flip check that the assertion matches reality (daml.yaml / EXPECTED_DAR_VERSION /
   ConfigMap diff) — catches the self-contradiction mechanically.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'merged main\|3-dot\|CONFLICTING\|re-review' <pr-review-desk SKILL.md>` | exit 0; ≥ 1 (merge-time discipline codified) |
| 2 | `grep -ci 'cite by expression\|expression, not line\|EXPRESSION' docs/brief-rules.md` | exit 0; ≥ 1 |
| 3 | `grep -ci 'materially\|re-derive\|version bump\|signs the body' <pr-review-desk SKILL.md docs/brief-rules.md>` | exit 0; ≥ 1 (#74 rule present) |
| 4 | (if the head-current CI gate landed) open a fixture PR with a stale base touching a shared file | the gate FAILS (positive control — it sees the stale base) |
| 5 | `cd status* && go run . --root .. --lint; echo $?` | exit 0 |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm the head-current gate (if built)
actually fails on a stale-base fixture — an advisory gate that never blocks is exactly the
"advisory, not enforceable" hole (#47) one level down.
