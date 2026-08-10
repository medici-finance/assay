---
brief: methodology/42
title: 'GitHub-durable dispatch claim — replace the machine-local batch-fanout claim lock with a cross-machine GitHub signal'
why: >-
  batch-fanout already claims a brief before dispatching it, but the claim is a file under
  ~/.claude/desk-tools/claims/ — machine-local. Two desk windows on the SAME machine are serialized;
  two desks on DIFFERENT machines (the stated multi-cell direction) are not — both would dispatch the
  same Next-up brief and open duplicate PRs. This directly contradicts the desks-coordinate-via-GitHub
  principle (durable GitHub state, never machine-local files). As the house moves to per-product cells
  on separate machines, the local lock silently stops working. Surfaced by the 2026-07-17 competitive
  scan (Paperclip's atomic task-checkout; human:<name> directive).
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
sources:
  - "2026-07-17 competitive scan (human:<name> directive; CCPM/Paperclip/orchestration/Kimi) — Paperclip's atomic task-checkout; freshness-checked: batch-fanout's local claim already exists"
  - "principle: desks-coordinate-via-github-not-local — inter-desk hand-offs ride durable GitHub state, never machine-local files/~/.claude"
  - "freshness-checked 2026-07-17 @ 4bcbcd0b: .claude/skills/batch-fanout/SKILL.md lines ~46-55 implement the machine-local `~/.claude/desk-tools/claims/<stream>--<NN>.claim` noclobber lock with a 120-min TTL — this brief REPLACES that mechanism, does not add a new one"

---

# Brief 42 — GitHub-durable dispatch claim

## Context
files: `../oit/.claude/skills/batch-fanout/SKILL.md` (the CLAIM-before-dispatch section, ~lines 46-55). May
add a tiny helper (Go tool or documented `gh` snippet) but the skill is the primary change.

facts:
- **What exists today (do NOT re-add it):** batch-fanout claims each brief before dispatch by
  atomically creating `~/.claude/desk-tools/claims/<stream>--<NN>.claim` with `set -C` (noclobber);
  creation-succeeds = the brief is yours; a claim under 120 min old (or an existing branch) = SKIP.
  The worker deletes the claim once its branch is pushed (branch-as-claim takes over).
- **The defect:** `~/.claude/desk-tools/claims/` is on the local filesystem. It serializes dispatchers
  sharing one checkout/machine, but NOT dispatchers on different machines — which is exactly the
  per-cell / multi-machine direction (I-three-cell-split, desks on separate machines). There, two
  desks both find the brief "free" and double-dispatch → duplicate PRs (the #524/#637 duplicate class,
  but caused by dispatch rather than authoring).
- **The principle it violates:** desks-coordinate-via-github-not-local — coordination MUST ride durable
  GitHub state (labels/comments/issues/branch existence), never machine-local files or `~/.claude`.
- **Existing partial GitHub signal:** "a branch for that brief now exists → SKIP" already uses a
  GitHub-durable signal. The claim WINDOW (between deciding to dispatch and the worker pushing a
  branch) is the only part still riding a local file — that window is what must move to GitHub.

## Task
1. **Replace the local-file claim with a GitHub-durable claim** for the pre-branch window. Recommended
   mechanism (implementer may substitute an equivalent GitHub-durable one and justify it):
   - A claim is a GitHub artifact keyed by `<stream>/<NN>` that any desk on any machine can see:
     e.g. a label `dispatched:<stream>-<NN>` on a per-brief tracking issue, or a claim **comment** on a
     standing dispatch-tracking issue, or a zero-diff `dispatch/<stream>-<NN>` ref/branch created
     atomically. Whatever the carrier, claiming MUST be atomic (create-if-absent) so two desks racing
     resolve to exactly one winner (GitHub's ref-create / `gh` label semantics give this).
   - Keep the **120-min TTL** semantics and the **branch-existence takeover** (once a real
     `fix/…`/`docs/…` branch exists, that is the claim; the pre-branch claim can be released).
   - Release/clear the claim when the worker's branch is pushed OR on TTL expiry OR when the brief
     leaves Next-up.
2. **Idempotent + observable:** a second dispatch attempt of an already-claimed brief no-ops and logs
   WHAT it deduplicated against (which desk/when). No silent double-dispatch.
3. **Remove the `~/.claude/desk-tools/claims/` file mechanism** from the skill (do not leave both — two
   locks with different visibility is worse than one). If a local fast-path is kept as an optimization,
   the GitHub claim MUST be the authority.
4. Update any sibling skill/doc that references the local claims dir (grep `desk-tools/claims`).

## Ground rules
- NEVER push to main / trigger workflows / merge. Branch + draft PR; stop at `implemented`.
- Edits `../oit/.claude/skills/batch-fanout/SKILL.md` — IN-REPO, no out-of-repo declaration. Do NOT edit any
  `~/.claude` copy; the repo copy is canonical.
- Shared value: the claim carrier is read by every desk window/machine — enumerate consumers (any skill
  that dispatches or checks "is this brief taken": batch-fanout, and any desk doc referencing the claim)
  and keep them coherent. Add a flow-level Verify (two dispatchers, one winner), not only a site check.
- NEEDS_CONTEXT over guessing the exact GitHub carrier if repo permissions/labels constrain it.

## Task-level design note
Prefer the **weakest sufficient** carrier: if a `dispatch/<id>` ref-create gives atomic
create-if-absent with no extra issue clutter, that beats a label needing a tracking issue. Whatever is
chosen, it must be greppable/inspectable by a human (desks-coordinate-via-GitHub is also about human
visibility).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -niE -e 'github' -e 'label' -e 'claim' .claude/skills/batch-fanout/SKILL.md \| grep -iE -e durable -e 'across machine' -e 'across desk' -e 'ref/' -e 'gh '` | ≥1 — the claim is described as a GitHub-durable, cross-machine signal |
| 2 | `test -f .claude/skills/batch-fanout/SKILL.md && ! grep -q 'desk-tools/claims' .claude/skills/batch-fanout/SKILL.md` | exit 0 — the machine-local file lock is removed (or demoted to a non-authoritative fast-path, in which case the row is 0 for it being the AUTHORITY — reviewer checks). Guarded by `test -f .claude/skills/batch-fanout/SKILL.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 3 | `grep -niE -e '120' -e 'ttl' -e 'branch.*exist' .claude/skills/batch-fanout/SKILL.md` | ≥1 — TTL + branch-existence takeover preserved |
| 4 | `grep -niE -e 'idempotent' -e 'no-?op' -e 'deduplicat' .claude/skills/batch-fanout/SKILL.md` | ≥1 — second-dispatch no-op + logs what it dedup'd against |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
