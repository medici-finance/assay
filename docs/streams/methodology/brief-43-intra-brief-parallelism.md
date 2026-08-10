---
brief: methodology/43
title: 'Intra-brief file-scoped parallelism — split one large brief across concurrent workers on disjoint file sets'
why: >-
  Today the finest dispatch grain is one worker per brief. A large (Effort: L) brief that touches
  several independent file sets — e.g. a DAML change + its frontend wiring + its docs — runs
  serially in one worker even though the parts don't collide. CCPM splits a single unit into
  concurrent file-scoped agents coordinated by conflicts/parallel metadata, cutting wall-clock on
  exactly the biggest work items. This adds an OPTIONAL faster path for L briefs without changing how
  S/M briefs dispatch. Surfaced by the 2026-07-17 competitive scan (CCPM; human:<name> directive).
wave: 1
depends: ["methodology/42"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
sources:
  - "2026-07-17 competitive scan (human:<name> directive) — CCPM's conflicts_with/parallel/depends_on splits one feature into concurrent file-scoped agents in one worktree"
  - "depends methodology/42 — a second concurrent worker on the same brief needs the dispatch-claim model to be coherent first; the sub-worker claims must not collide"
  - "freshness-checked 2026-07-17 @ 4bcbcd0b: no existing brief covers intra-brief parallelism; batch-fanout dispatches exactly one worker per brief"

---

# Brief 43 — Intra-brief file-scoped parallelism

## Context
files: brief schema doc / `author-brief` guidance (the optional new frontmatter field);
`../oit/.claude/skills/batch-fanout/SKILL.md` (the split-and-dispatch logic); optionally
`../assay-toolkit/statusgen/` (only if the field needs validation — keep it OPTIONAL so existing briefs are
unaffected).

facts:
- **Today:** batch-fanout dispatches one worker per brief; the worker isolates in one worktree and
  opens one draft PR. Correct for S/M; leaves L briefs with independent parts serialized.
- **The idea (CCPM):** a brief declares that its work decomposes into N **parallel sub-streams** each
  scoped to a **disjoint file set**; the dispatcher runs one worker per sub-stream concurrently, and
  the parts recombine into ONE brief's deliverable.
- **The hard constraints that make this safe (bind the design):**
  1. **Disjointness must be provable, not asserted** — if two sub-streams' file globs overlap, they
     must NOT run in parallel (fall back to serial). The split is an optimization, never a correctness
     risk.
  2. **One brief = one PR still holds** (CLAUDE.md). N concurrent sub-workers must converge on a
     SINGLE brief branch + PR — either N workers committing to disjoint paths on one shared branch
     (coordinated), or N worktrees whose disjoint diffs are merged into one branch before the PR opens.
     Decide and specify one model; do not leave it ambiguous.
  3. **Claim coherence** (why this depends on methodology/42): sub-worker claims must not fight the
     brief-level claim; the brief is claimed once, sub-streams are claimed under it.
  4. **Only when it pays:** gate the split on Effort: L (or an explicit brief opt-in). S/M briefs
     dispatch exactly as today — no behavior change.

## Task
1. **Add an OPTIONAL brief frontmatter field** declaring parallel sub-streams with file scopes, e.g.
   `parallel-streams: [{name: daml, files: ["daml/**"]}, {name: fe, files: ["frontend/**"]}]`
   (final shape the implementer's call; must carry a name + a file glob per sub-stream). Absent field =
   today's single-worker behavior. Document it in `author-brief`.
2. **Disjointness check** — before parallel dispatch, verify the sub-streams' globs do not overlap
   (and, ideally, that they cover the brief's actual touched files). Overlap → refuse the split, log
   why, fall back to one worker. This check is a hard precondition.
3. **Dispatch + recombine in batch-fanout** — for an L brief with a valid split, dispatch one worker
   per sub-stream (each its own worktree, per the isolation rule), each restricted to its file set,
   then recombine into ONE brief branch + ONE draft PR (per constraint 2's chosen model). Preserve the
   per-brief claim (methodology/42) and the draft-PR-per-brief contract.
4. **Fail safe:** any sub-worker failure degrades to a clearly-reported partial (the brief stays
   in-progress with a note), never a silently half-applied brief.

## Ground rules
- NEVER push to main / trigger workflows / merge. Branch + draft PR; stop at `implemented`.
- Shared value: the new frontmatter field is read by batch-fanout AND author-brief AND (maybe)
  statusgen — enumerate consumers, keep the field OPTIONAL so no existing brief breaks. Add a
  flow-level Verify (an L brief with two disjoint sub-streams dispatches two workers and yields ONE PR).
- Isolation: each sub-worker gets its OWN owned worktree (the shared-checkout rule applies per worker).
- NEEDS_CONTEXT over guessing the recombine model if the harness makes one-branch-many-workers hard.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -niE 'parallel-streams' .claude/skills/author-brief/SKILL.md docs/streams/methodology/*.md 2>/dev/null \| head` | ≥1 — the optional field is documented |
| 2 | `grep -niE -e disjoint -e overlap -e 'fall.?back' .claude/skills/batch-fanout/SKILL.md` | ≥1 — the disjointness precondition + serial fallback are specified |
| 3 | `grep -niE -e 'one brief' -e 'one PR' -e 'single.*branch' -e 'single.*PR' -e recombine .claude/skills/batch-fanout/SKILL.md` | ≥1 — N sub-workers converge on one brief PR |
| 4 | `grep -niE -e 'Effort: L' -e 'opt-in' -e 'S/M.*unchanged' -e 'unchanged.*S/M' .claude/skills/batch-fanout/SKILL.md` | ≥1 — split gated to L/opt-in; S/M unchanged |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
