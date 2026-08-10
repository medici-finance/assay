# Brief-22 user-level deltas

> **2026-08-08 update:** the stubs below were applied at some point after this file's
> "NOT APPLIED" box, but they went stale on a second move — `assay-selfcontain/08` relocated
> the canonical desk-skill bodies from `oit` to
> `medici-finance/assay-toolkit`'s own `.claude/skills/`, and the four user-level pointer
> stubs still named the old (`oit`) home in both frontmatter `description:` and body text.
> Fixed on the live `~/.claude/skills/*` files directly (local, no PR); the templates in this
> file are corrected in place below so re-deriving a stub from here doesn't reintroduce the
> stale home.

> ## STATUS: **NOT APPLIED** as of 2026-07-15 (#541). Read this box before using any stub below.
>
> Brief-22 is `done`, but that covers only its **in-repo** half. The stubs in this file were authored
> and never applied — `~/.claude/skills/verify-desk/SKILL.md` is still a full ~146-line fork that
> contradicts the in-repo copy on whether the desk may land its own work. That contradiction cost an
> afternoon of verification (#541). The in-repo skills meanwhile asserted the post-application state
> in the present tense ("the user-level copy *is* a thin pointer"), so nothing surfaced the gap.
>
> **Structural cause, worth fixing at the root:** an out-of-repo application step has **no tracking
> surface** (#221). It cannot be a Verify row, so a brief reaches `done` with the step outstanding and
> no one is wrong. Until #221 lands, an unapplied stub is only as visible as this box.
>
> ### PRECONDITION — port before you stub. Do not skip this.
> On **2026-07-12** four rules were added **only** to the user-level copies (stopgap commits
> `60daa20` + `a86300e`, mirrored in
> `skill-deltas-2026-07-12-fanout-first-and-insight-routing.md`): fanout-first, insight-routing,
> orphan-sweep, escalation labels. **The fork is bidirectional** — the user-level copies are stale on
> tier/push policy but **ahead** on these. Replacing a copy with a stub while a rule lives only there
> **deletes that rule**; the mirror file is a diff doc, loaded by no session.
>
> | Skill | insight-routing + escalation labels ported in-repo? | Stub safe to apply? |
> |---|---|---|
> | `verify-desk` | **yes** — done in #541 | **yes** |
> | `batch-fanout` | **yes** — + orphan-sweep ported (`fix/desk-skill-repo-reconcile`) | **verify first** — see 2026-07-16 note |
> | `pr-review-desk` | **yes** — repo copy carries main's reconciled sections | **yes** — reviewer-identity conflict resolved (2026-07-16 merge); see note |
> | `the-desk` | **yes** — + fanout-first ported (`fix/desk-skill-repo-reconcile`) | **verify first** — see 2026-07-16 note |
>
> For each skill: diff the two copies, port anything user-level-only into the repo copy **on a PR**,
> and only then apply that skill's stub. `diff ~/.claude/skills/<s>/SKILL.md .claude/skills/<s>/SKILL.md`
>
> #### 2026-07-16 port (`fix/desk-skill-repo-reconcile`)
> The four named 2026-07-12 rules are now carried in the repo copies — additive only, no repo-only
> content removed:
> - **fanout-first** → `the-desk` (Dispatching subagents).
> - **insight-routing** → `pr-review-desk`, `the-desk`, `batch-fanout`.
> - **orphan-sweep** → `batch-fanout` (Procedure step 0).
> - **escalation-labels** → `pr-review-desk`, `the-desk`, `batch-fanout`.
>
> **The `pr-review-desk` reviewer-identity conflict is now RESOLVED (2026-07-16, merge of `origin/main`).**
> When this PR was authored, the repo `pr-review-desk` still said the App verdict was **"tamper-evident"**
> and this PR left that wording as-is, flagged for a human. Main has since independently landed the
> reconciliation, so the merge takes **main's version of `../oit/.claude/skills/pr-review-desk/SKILL.md`** —
> this PR now makes **no change to that file**. What main's copy already carries (so no separate port
> is needed here):
> - **Reviewer identity — a distinct, auditable actor (attribution, not authorization)** — the
>   corrected verdict (assay-toolkit#37/#38): a worker CAN forge in principle; real enforcement pends
>   the desk-apps stream; the merge stays human:<name>'s.
> - **Out-of-scope discoveries → file an issue** (oit#474, DESK-FLAG retirement) — which also carries
>   the **insight-routing** rule (systemic insight → an assay-toolkit issue) and the **question**
>   escalation label, so the two `pr-review-desk` port targets on the list above are satisfied by
>   main's own sections rather than by this PR's bullets.
> - **Post as the App, always** (assay-toolkit#38).
>
> So: the **four named precondition rules are done** (the table's tracked criterion), and the
> `pr-review-desk` copy is now current and lossless. The one item still user-level-only for
> `pr-review-desk` is the **`--scan-issues` issue-loop scanner step**; it is being reconciled in the
> issue-loop desk work (2026-07-16), not here.

The desk skills moved into this repo (`.claude/skills/`) per brief 22. This file records
the thin-pointer stubs that replace the `~/.claude/skills/` copies. Applying these is the
human's (or a user-scope session's) step — the in-repo work does NOT edit `~/.claude/**`
(issue #221).

## Files to replace/update

### 1. `~/.claude/skills/batch-fanout/` — replace entirely

Replace the existing `~/.claude/skills/batch-fanout/SKILL.md` with a thin pointer:

```markdown
---
name: batch-fanout
description: Run the work-dispatch role of the process desk — fan out the Next-up batch of briefs to parallel worker agents that each implement one brief in its own worktree and open a draft PR. Use when human:<name> says "fan out the next batch / work the next N briefs in parallel / do what's next in parallel / fan out", i.e. the plural of "work on what's next". Reads STATUS.md Next-up (already priority + staleness + 2-per-stream-capped), dispatches one worker per brief, and hands the resulting draft PRs to the pr-review-desk window. Persona Bob; driver human:<name>; the human merges.
---

# Batch Fan-out

**Moved into the repo.** The canonical copy lives at .claude/skills/batch-fanout/SKILL.md
in `medici-finance/assay-toolkit` (relocated here from `oit`
by `assay-selfcontain/08`). This user-level stub exists so the skill name resolves; always
consult the in-repo version for the current operating manual.
```

### 2. `~/.claude/skills/pr-review-desk/` — replace entirely

Replace the existing `~/.claude/skills/pr-review-desk/SKILL.md` with a thin pointer:

```markdown
---
name: pr-review-desk
description: Run the PR-review-loop role of the process desk — the standing review window that watches the open-PR queue across this project's repos (oit, agent-runtime, medici-examples, plus the medici-finance report repos assay-toolkit/reconciler/decks/proposals), dispatches reviewers to every new/updated PR, drives the fix-to-re-review-to-ready cycle, and flips PRs ready-for-human. Use when starting or resuming the dedicated review window, when asked to "run the review loop / watch the PR queue / review the PRs", or when the coordinator desk delegates the review half. Persona still Bob; driver human:<name>; the human merges.
---

# PR-Review Desk

**Moved into the repo.** The canonical copy lives at .claude/skills/pr-review-desk/SKILL.md
in `medici-finance/assay-toolkit` (relocated here from `oit`
by `assay-selfcontain/08`). The bundled tools (`deskboard.go`, `mint-reviewer-token.go`) live
there too. This user-level stub exists so the skill name resolves; always consult the
in-repo version for the current operating manual.
```

Delete `~/.claude/skills/pr-review-desk/deskboard.go` and
`~/.claude/skills/pr-review-desk/mint-reviewer-token.go` — they moved to the repo.

### 3. `~/.claude/skills/verify-desk/` — replace entirely

> **READY TO APPLY (#541).** The precondition is met for this skill: `insight-routing` and
> `escalation labels` — which existed only in the user-level copy — are now carried in
> .claude/skills/verify-desk/SKILL.md (originally `../oit/.claude/skills/verify-desk/SKILL.md`
> before `assay-selfcontain/08` relocated it here). Applying this stub now loses nothing.
>
> **Why an agent cannot apply it:** `~/.claude/**` is outside every worktree and has no branch, no PR,
> and no isolation — an edit goes live in **every session on this machine instantly**, including desks
> running right now. That makes it human:<name>'s call, not an agent's (#221).
>
> **Applying this stub is the fix for #541's root cause** — it retires the fork that told the desk
> "pushing is gated" while the in-repo copy told it to commit straight to main. Until it is applied,
> a verify-desk session may still load the wrong instruction and stall. Verify after applying:
> `wc -l ~/.claude/skills/verify-desk/SKILL.md` → ~12 lines, not 146.
>
> Commit in the `~/.claude` stopgap repo so the edit is diffable and revertable.

Replace the existing `~/.claude/skills/verify-desk/SKILL.md` with a thin pointer:

```markdown
---
name: verify-desk
description: Run the post-merge VERIFY role of the process desk — the standing window that drains the "Awaiting verification / review" queue. Merging a brief-PR does NOT complete it; a NON-implementer must run the brief's Verify table on merged main, fill Evidence, and advance implemented-to-verified-to-done. Use when starting/resuming the verify window, when asked to "run the verifier loop / drain the awaiting queue / verify merged briefs / turn merged into done", or when the coordinator delegates the verify half. Mirrors pr-review-desk but POST-merge. Persona Bob; driver human:<name>.
---

# Verify Desk

**Moved into the repo.** The canonical copy lives at .claude/skills/verify-desk/SKILL.md
in `medici-finance/assay-toolkit` (relocated here from `oit`
by `assay-selfcontain/08`). This user-level stub exists so the skill name resolves; always
consult the in-repo version for the current operating manual.
```

### 4. `~/.claude/skills/the-desk/` — replace entirely

Replace the existing `~/.claude/skills/the-desk/SKILL.md` with a thin pointer:

```markdown
---
name: the-desk
description: Boot or resume ONLY the single standing COORDINATOR / process-desk session (persona "Bob", driver human:<name>) for the oit streams methodology — the one arbiter-across-streams window. Load ONLY on an explicit desk-boot request: the user types `/the-desk`, or says "boot/resume the desk", "you are the desk", "resume Bob", "coordinate the streams". Do NOT load this for a WORKER/IMPLEMENTER session, a fanout worker, a plain "what's next" pick, or the review/verify windows — those implement one brief or run their own loop (`batch-fanout`, `pr-review-desk`, `verify-desk`) and must NOT adopt the coordinator persona. Not a general session-start or "methodology work" trigger.
---

# TheDesk

**Moved into the repo.** The canonical copy lives at .claude/skills/the-desk/SKILL.md
in `medici-finance/assay-toolkit` (relocated here from `oit`
by `assay-selfcontain/08`). This user-level stub exists so the skill name resolves; always
consult the in-repo version for the current operating manual.
```

### 5. `~/.claude/skills/author-brief/` — update push wording

The user-level `author-brief` skill's brief template still says "NEVER git push" — the
corrected wording (in this repo's briefs) is: "NEVER push to **main**, merge, or trigger
workflows. Workers pushing their own feature branch + opening a draft PR is
standing-authorized (the I-12 loop)." **This is a user-level fix — apply it to
`~/.claude/skills/author-brief/SKILL.md`** by finding the "NEVER git push" line in the
template ground-rules section and replacing it with the branch-push-sanctioned version above.

## Reconciliations recorded (old → new)

These are the practice-vs-writer drift fixes applied in the in-repo skills (the user-level
copies had the stale text):

| What | Old (user-level skill) | New (in-repo skill) |
|------|------------------------|---------------------|
| Reviewer bot name | `medici-stuff[bot]` | `reviewer-app[bot]` (App <app-id>, renamed 2026-07-09) |
| Verifier dispatch tier | blanket "opus+" for all reviews & verifications | risk-keyed per methodology/19: cheap-tier for risk-clear, strong-tier/human for risk-flagged |
| Git push policy | "NEVER git push" / "never git push without human:<name>'s go" | Branch push + draft PR is standing-authorized (I-12 loop); desk Evidence commits straight to main (human:<name>-directed 2026-07-09); main push and merge remain human:<name>-gated |
| Verifier practice vs text | Skill said dispatch opus+ verifiers; desk operated with glm-5.2 verifiers | Policy is risk-keyed floor (methodology/19), not a blanket tier — both the old blanket opus+ and the ad-hoc blanket glm are replaced by a rule |

## Application note

Apply the stubs above to `~/.claude/skills/`, then delete `deskboard.go` and
`mint-reviewer-token.go` from the user-level `pr-review-desk/` dir. The `author-brief` push
wording fix is a targeted edit in the user-level skill file — not a full replacement.
Once applied, the user-level skill descriptions (which load every session) are unchanged
and trigger correctly; the bodies are thin pointers that redirect to this repo's versions.

Commit the `~/.claude` changes in the `~/.claude` stopgap git repo so the edits are
diffable and revertable.
