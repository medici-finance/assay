---
brief: loop-engine/04
title: Guardrail module — one home for the six rule blocks duplicated across all five loop skills
wave: 1
depends: ["loop-engine/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-19 by Fable design session (human:<name>'s fix-the-verify-loop direction)
sources: ["docs/loop-engine-architecture.md (§5 seam analysis + drift table — freshness-checked 2026-07-19)", "the five skill bodies (.claude/skills/{verify-desk,pr-review-desk,issue-loop,batch-fanout,the-desk}/SKILL.md)", "the-desk SKILL.md § Rule-ownership principle (methodology/22: every rule has exactly one home)", "tools/desk/internal/deskkit (Guard, audit, BodyCheck, writeguard — the existing implementations)", "assay-toolkit#13 (insight-routing)"]
exec-tier: any
exec-tier-why: extraction + repointing against an enumerated drift table; the judgment (which blocks, which home) is already in the architecture doc.
why: >-
  Six rule blocks — stop-flag/Guard(), git-push policy, insight-routing, escalation labels,
  worktree isolation, no-attribution — are near-verbatim in all five loop skills and already
  drifting (issue-loop's stop-flag exists only in a user-level delta; verify-desk and
  pr-review-desk word the push policy differently). the-desk's own rule-ownership principle
  says every rule has exactly one home; the skills violate it six times over. Most blocks
  already have code homes in deskkit — this brief makes those the single home, the engine
  their single caller for drain loops, and the skills pointers + irreducibles. Independent
  seam: pays for itself even where the conductor never runs (archetypes B and C).
---

# Brief 04 — Guardrail module extraction

## Context
files: `../assay-toolkit/tools/desk/internal/deskkit/` (gap-fill only — most blocks exist), `../assay-toolkit/tools/desk/`
README (the canonical operator text for each block), all five
`.claude/skills/*/SKILL.md` (restatements → pointers; per-loop irreducibles STAY),
`../oit/docs/loop-engine-architecture.md` §5 (the binding block inventory)
facts:
- The six blocks and their target homes:
  1. stop-flag/Guard() boot + iteration check → deskkit (exists); canonical operator text in
     tools/desk README (exists per desk-tools/08); skills carry the two-line boot pointer only.
  2. git-push policy → single canonical text (tools/desk README or collaboration-protocol —
     implementer picks ONE and records why); per-loop DIFFERENCES (verify lands straight to
     main; workers branch-push only) are irreducibles and stay in their skills, phrased as
     deltas from the canonical text.
  3. insight-routing (assay-toolkit#13) → one canonical paragraph, skills point.
  4. escalation labels (question/help-wanted discipline) → one canonical paragraph, skills point.
  5. worktree isolation (F-34/F-35) → writeguard + deskwt are the enforcement (exist); one
     canonical text; skills keep only loop-specific incident notes.
  6. no-attribution → CLAUDE.md already owns it (user+project rule); skills drop restatements.
- The engine (01) already calls Guard() every iteration for drain loops — after this brief
  it is the single caller pattern: a drain loop cannot skip a guardrail because the loop
  never sees them. Archetype B/C skills keep pointers (their loops still run prose).
- Placement rule (CLAUDE.md) binds: rules stay RESIDENT where every session needs them —
  this brief compresses wording and de-duplicates homes; it must NOT replace a load-bearing
  resident rule with a bare pointer where the placement rule requires residence.
- Skills are in-repo single-home since brief-22 (the ~/.claude forks were removed); the
  issue-loop user-delta stop-flag block is the one known exception — brief-03 or this brief
  repairs it in-repo (whichever lands second points, not restates).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Pure extraction/repointing: NO behavioral change to any guard, NO new policy. A wording
  conflict between two skills' versions of a block is resolved by finding the authoritative
  source (incident/issue/human:<name> direction cited in the block) — if genuinely ambiguous, file a
  `question`-labeled issue and leave that block un-migrated rather than adjudicating.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Audit pass: for each of the six blocks, table the per-skill wording deltas (PR body);
   flag any delta that is a real policy difference (irreducible — stays) vs drift (migrates).
2. Land the canonical texts in their target homes per facts; gap-fill deskkit only where a
   block's enforcement half is missing.
3. Repoint all five skills: restatement → pointer + irreducible delta. Line counts should
   DROP in every skill.
4. Verify the engine's call sites (01) cover blocks 1/5 for drain loops; note in the PR body
   which blocks remain prose-enforced for archetypes B/C (that's expected, not a gap).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0 |
| 2 | `for f in .claude/skills/{verify-desk,pr-review-desk,issue-loop,batch-fanout,the-desk}/SKILL.md; do grep -c 'assay-toolkit#13' $f; done` | ≤1 per file (the pointer; the near-verbatim paragraph is gone from ≥3 of them) |
| 3 | `wc -l .claude/skills/{verify-desk,pr-review-desk,issue-loop,batch-fanout,the-desk}/SKILL.md` | every count ≤ its pre-brief baseline (276/307/248/223/214); record the deltas |
| 4 | PR body contains the six-block delta table with drift-vs-irreducible ruling per delta | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model. Reviewer confirms (a) zero behavioral change to any guard (extraction only),
(b) no load-bearing resident rule became a bare pointer where the CLAUDE.md placement rule
requires residence, (c) real policy differences were kept as per-loop irreducibles, not
"harmonized" away, (d) ambiguous wording conflicts were filed as questions, not adjudicated
by the implementer, (e) every skill shrank.
