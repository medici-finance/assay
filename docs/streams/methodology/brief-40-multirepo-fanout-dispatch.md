---
brief: methodology/40
title: 'Multi-repo dispatch — batch-fanout reads every product repo''s Next-up board, not just oit''s'
why: >-
  Streams now live in more than one repo — desk-console and desk-solo are in assay-toolkit,
  reconciler has its own board — but the dispatcher only ever reads oit's
  STATUS.md, so those streams sit forever un-worked while looking "active". The review desk is
  already multi-repo (it watches ~11 house repos); dispatch silently is not, so the house looks
  fully staffed when half of it is never picked up. This closes that asymmetry: every repo that
  generates a board gets dispatched from, using the single existing fanout window.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources:
  - "human:<name> 2026-07-16: 'what i'd like is for each repo with streams to regenerate its status (ideally the statusgen should come from assay-toolkit repo). then when this is done, I want the current batch-fanout to read them. once we are further along, we can then change the desks to only read repos of their product'"
  - "diagnosis 2026-07-16: desk-console/desk-solo (in assay-toolkit) never dispatched — batch-fanout boots from oit's board only"
  - "[I-three-cell-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-14-three-cell-split-per-cell-desks-master-aggregator.md) — the phase-3 direction (per-cell desks) this bridges toward; explicitly NOT this brief"
  - "assay-toolkit#82/#83 — the phase-1 board-bootstrap fix that makes assay-toolkit's board exist (this brief's precondition, in a sibling repo)"
  - "freshness-checked 2026-07-16 @ b514528d: `../oit/.claude/skills/batch-fanout/SKILL.md` line 65 dispatches from `go run ./tools/statusgen --root . && … STATUS.md` (oit only); line 50 already scans PRs across oit/agent-runtime/medici-examples — the asymmetry is real and current"
---

# Brief 40 — Multi-repo dispatch for batch-fanout

## Context
files: `../oit/.claude/skills/batch-fanout/SKILL.md` (the only file this brief changes).

facts:
- **The bug, precisely.** `SKILL.md` boots dispatch from ONE board:
  - line ~19: *"Boot by `cd`-ing into the oit checkout so it resolves."*
  - line ~65: `git fetch origin && go run ./tools/statusgen --root . && sed -n '/Next up/,/^## /p' STATUS.md`
  Both are oit-only. Meanwhile line ~50 ALREADY scans open PRs across
  `oit, agent-runtime, medici-examples` — so review/PR-scan is multi-repo but
  **dispatch is single-repo**. That gap is why streams in other repos are never worked.
- **The board-bearing repo set (repos with streams), with readiness verified on `main` (2026-07-16):**
  | repo | checkout base | statusgen on `main` | board on `main` | board regen command | dispatchable now? |
  |------|---------------|---------------------|-----------------|---------------------|-------------------|
  | `oit` | `~/work/oit` | ✅ `tools/statusgen` | ✅ generated | `go run ./tools/statusgen --root .` | **yes** |
  | `assay-toolkit` | `~/work/assay-toolkit` (or owned clone) | ✅ `statusgen/` | ❌ (bootstrap bug — fixed by #83) | `cd statusgen && go run . --root ..` | **after #83 merges** |
  | `reconciler` | `~/work/reconciler` (or owned clone) | ❌ on `main` (tool + streams live only on unmerged branch `docs/reconciler-agentic-spec`; `main` STATUS.md is a hand-maintained phases doc, NOT generated) | ❌ | `cd statusgen && go run . --root ..` (once merged) | **no — provisioning in-flight** |
  | `platform-repo` | `~/work/platform-repo` | ❌ none (streams scaffolding on `main`; `platform-build` stream on branch) | ❌ | (none until provisioned) | **no — needs tool+workflow** |
  Note the statusgen invocation DIFFERS by repo: oit's tool is rooted at `../assay-toolkit/statusgen/` (`--root .`);
  assay-toolkit's / reconciler's are a standalone Go module rooted at `statusgen/` (run from inside with
  `--root ..`). The skill must carry both forms, keyed by repo — do not assume one command. The skill
  reads whatever repos are **dispatchable** (a working statusgen it can run against `main` streams) and
  **skips the rest with a logged note**, so it degrades gracefully as repos come online. Today that means
  oit now, assay-toolkit once #83 merges; reconciler + platform-repo join once provisioned.
- **Phase-1 provisioning is a precondition for the not-yet-ready repos (tracked in methodology/41, not
  this brief):** reconciler must land its agentic-spec branch + a workflow + switch STATUS.md
  manual→generated; platform-repo needs both the statusgen tool and the workflow. Per human:<name> (2026-07-16)
  the tool should be sourced FROM assay-toolkit rather than copied a fourth time — that decision lives in
  methodology/41. This brief only requires a repo be *dispatchable* to include it, and enumerates the set
  in one place so a newly-provisioned repo is a one-line add.
- **This is a shared-value change (author-brief rule 6): the dispatch repo-set is consumed by
  workers.** A pick is no longer implicitly "in oit"; every dispatched brief now carries its
  **repo**, and the worker must isolate + open its PR **in that repo**. Consumers to keep coherent:
  (a) the worker-dispatch prompt/template (must name the repo + its checkout + its statusgen
  command); (b) worktree isolation (each repo needs its own owned worktree — the shared-checkout
  isolation rule is per-repo); (c) the PR-scan set on line ~50 (align it with the board-bearing set,
  or state explicitly why the two sets differ — today PR-scan lists agent-runtime/medici-examples,
  which have no streams, and omits assay-toolkit/reconciler, which do).
- **Precondition (cross-repo, tracked separately).** assay-toolkit's board must actually exist for
  its streams to be dispatchable — that is assay-toolkit#83 (board-bootstrap fix). Until it merges,
  assay-toolkit's `go run . --root ..` still produces a board locally, so the skill can read it, but
  the committed `STATUS.md` won't be on main. The skill reads a **freshly regenerated** board (it
  already regens to local scratch — "never commit STATUS.md on a branch"), so it does not depend on
  the committed copy; still, cross-reference #83 so the dependency is visible.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Branch + draft-PR only; stop at
  `implemented`.
- This edits `../oit/.claude/skills/batch-fanout/SKILL.md`, which is IN-REPO (not `~/.claude`) — no
  out-of-repo declaration needed. Do NOT also edit any `~/.claude` copy; the repo copy is canonical
  (methodology/22).
- Phase 3 (per-cell desks — each cell reads only its own repo, [I-three-cell-split]) is EXPLICITLY
  out of scope. This brief keeps the SINGLE existing fanout window and widens what it reads.
- NEEDS_CONTEXT over guessing — especially the exact per-repo checkout paths (they may be owned
  clones rather than the shared worktrees above).

## Task
1. **Replace the single-board boot with a repo-set loop.** Where the skill says "Boot by cd-ing into
   the oit checkout" and reads one `STATUS.md`, make it iterate the board-bearing
   repo set (table above): for each repo, regenerate its board to local scratch with THAT repo's
   statusgen command and extract its `Next up` section.
2. **Merge into one prioritized batch, tagged by repo.** Combine the per-repo Next-up rows into a
   single dispatch batch; each pick carries its `repo`. Preserve the existing per-stream cap and
   priority/staleness ordering **within each repo's board** (statusgen already applies those per
   board); the cross-repo merge just concatenates the capped per-repo batches — do not invent a new
   global scoring pass. State the interleave rule plainly (e.g. round-robin across repos, or all of
   oit then others) so it is deterministic.
3. **Make the worker-dispatch carry the repo.** The per-worker instructions must name: the target
   repo, its checkout base, "isolate in an owned worktree OF THAT REPO", and open the draft PR in
   that repo. A worker must never assume oit.
4. **Reconcile the PR-scan set (line ~50) with the board-bearing set** — either widen it to include
   assay-toolkit + reconciler, or add one sentence stating why the PR-scan set and the dispatch set
   differ. No silent mismatch.
5. **Define the repo-set in ONE place in the skill** (a short table or list) that both the dispatch
   loop and the PR-scan reference, so adding a future product repo is a one-line edit. Note in a
   comment that phase-3 will replace this shared set with a per-cell single repo.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c -iE -e assay-toolkit -e reconciler .claude/skills/batch-fanout/SKILL.md` | ≥ 2 — the board-bearing repos are named in the skill |
| 2 | `grep -iF 'cd statusgen && go run . --root ..' .claude/skills/batch-fanout/SKILL.md` | ≥1 — the standalone-module statusgen form is carried (not just oit's `--root .`) |
| 3 | `grep -iE -e 'board-bearing' -e 'per-repo' -e 'each repo' -e 'each board' -e 'repo set' .claude/skills/batch-fanout/SKILL.md` | ≥1 — the loop reads a repo SET, not one board |
| 4 | `grep -iE -e worktree -e isolate -e checkout .claude/skills/batch-fanout/SKILL.md` | ≥1 — worker-dispatch carries the target repo + its per-repo isolation |
| 5 | `bash -c 'cd ~/work/assay-toolkit/statusgen && go run . --root .. >/dev/null && sed -n "/Next up/,/^## /p" ../STATUS.md | grep -iE -e desk-console -e desk-solo'` | ≥1 — assay-toolkit's freshly-regenerated board actually surfaces the previously-invisible streams (proves the multi-repo read yields real picks) |
| 6 | `grep -iE -e 'per-cell' -e 'three-cell' -e 'phase.?3' .claude/skills/batch-fanout/SKILL.md` | ≥1 — the phase-3 boundary is noted as out of scope / future |

## Evidence
<!-- filled by a non-implementer at verify time: one row per Verify item -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
