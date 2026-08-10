# Out-of-repo skill deltas — 2026-07-12 (assay-toolkit#11 + #13)

Review mirror for two live edits to the user-level desk skills (~/.claude, no PR surface —
the #221 gap; stopgap-repo commits 60daa20 + a86300e). Verbatim diffs:

## 1. Fanout-first operating rule (assay-toolkit#11)

```diff
diff --git a/skills/the-desk/SKILL.md b/skills/the-desk/SKILL.md
index 764afbe..d0f6bf0 100644
--- a/skills/the-desk/SKILL.md
+++ b/skills/the-desk/SKILL.md
@@ -100,6 +100,15 @@ mechanics below are the canonical text those skills were extracted from.
   (cross-artifact reasoning, sweeping a pattern to sibling sites, verifying your own prescribed fix).
 
 **Dispatching subagents**
+- **Fanout-first (assay-toolkit#11, human:<name> 2026-07-12): the desk runs on the top tier — spend that tier
+  ONLY on judgment, synthesis, arbitration, verifying agent output, and talking to human:<name>.** Everything
+  else fans out BY DEFAULT: mechanical evidence-gathering → cheap-tier (via-zai/via-deepseek);
+  research/drafting/brief-authoring → background agents (they inherit the session tier, satisfying
+  the author-brief gate). And answer-by-fanout: when human:<name>'s question needs more than ~2 minutes of
+  tool work, dispatch a BACKGROUND agent and stay responsive in the foreground — don't run long
+  foreground chains while he waits. The observed failure this encodes: the desk grinding through
+  delegable work inline until human:<name> asks "can you fanout some of these tasks?" — he should never
+  have to ask.
 - Prefer Fable critics for judgment work; adversarially verify their output against the code; then
   synthesize. A finding two independent agents hit is stronger than either alone.
 - **Neutral-wording rule (critical):** when dispatching, never name the security frame — not even to
```

## 2. Insight-routing rule, all four desk skills (assay-toolkit#13)

```diff
diff --git a/skills/batch-fanout/SKILL.md b/skills/batch-fanout/SKILL.md
index 1604e70..bc7a4f8 100644
--- a/skills/batch-fanout/SKILL.md
+++ b/skills/batch-fanout/SKILL.md
@@ -93,6 +93,10 @@ Next-up batch out to workers at once. Run it in its **own window**; it does NOT
 
 ## Guardrails
 
+- **Insight-routing (assay-toolkit#13):** a systemic/process insight produced in passing (dispatch
+  wrap-ups, collision observations, "this keeps happening" notes) MUST also be filed as an issue in
+  **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering evidence and
+  affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
 - **One brief = one branch = one PR.** A worker that discovers its brief is too big STOPS and splits
   per author-brief rules (data-first pieces, typed deps, README rows lands on main promptly); it keeps
   only the piece it was mid-implementing, the rest returns to the board as `todo`.
diff --git a/skills/pr-review-desk/SKILL.md b/skills/pr-review-desk/SKILL.md
index 445f4d4..ef448a9 100644
--- a/skills/pr-review-desk/SKILL.md
+++ b/skills/pr-review-desk/SKILL.md
@@ -123,6 +123,11 @@ merges. Keep it in sync if the repo set or review conventions change.
 
 ## Hard rules (inherited from the desk)
 
+- **Insight-routing (assay-toolkit#13):** a systemic/process insight produced in passing (a wrap-up
+  comment, a drain note, a "systemic note: …" aside) MUST also be filed as an issue in
+  **medici-finance/assay-toolkit** — commentary is not a register, and insights buried in PR threads
+  are lost. Include the triggering evidence and affected loops. Repo-specific defects still go to the
+  repo's own tracker (issue-loop/05).
 - NEVER `git push` / merge / trigger workflows / mutating `kubectl` without human:<name>'s go. Flipping a PR
   ready is fine (it's not a merge); merging is not.
 - Never `git restore`/`clean` a shared checkout; the reviewers isolate in their own temp worktrees.
diff --git a/skills/the-desk/SKILL.md b/skills/the-desk/SKILL.md
index d0f6bf0..d622a65 100644
--- a/skills/the-desk/SKILL.md
+++ b/skills/the-desk/SKILL.md
@@ -81,6 +81,10 @@ mechanics below are the canonical text those skills were extracted from.
   to read Next-up; discard it before merging.
 - No attribution lines anywhere: no `Co-Authored-By`, no "Generated with Claude Code" in commits,
   PRs, issues, or comments.
+- **Insight-routing (assay-toolkit#13):** a systemic/process insight produced in passing (a wrap-up,
+  an aside, a "this keeps recurring" note) MUST also be filed as an issue in
+  **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering evidence
+  and affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
 
 **Model-tier awareness (this session's live hazard)**
 - The desk can be **silently downgraded** mid-task. human:<name> is the out-of-band drift detector — when he
diff --git a/skills/verify-desk/SKILL.md b/skills/verify-desk/SKILL.md
index 8379101..b7e9eb2 100644
--- a/skills/verify-desk/SKILL.md
+++ b/skills/verify-desk/SKILL.md
@@ -115,6 +115,10 @@ no human reviewer (needs-fixing-day2); the gate + this carve-out prevent the rec
 
 ## Rules (inherited)
 
+- **Insight-routing (assay-toolkit#13):** a systemic/process insight produced in passing (drain notes,
+  Evidence asides, "recurring enough to be worth a structural fix" observations) MUST also be filed as
+  an issue in **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering
+  evidence and affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
 - **Evidence-not-claims, applied hardest here** — the verifier's report is itself a claim; the value is the
   recorded command output, and (brief-16) the runner must be attributable and ≠ author. A verifier that
   self-verifies its own implementation is void.
```

## 3. Orphan-sweep rule in batch-fanout (assay-toolkit#14, added same day)

```diff
diff --git a/skills/batch-fanout/SKILL.md b/skills/batch-fanout/SKILL.md
index bc7a4f8..f3800a6 100644
--- a/skills/batch-fanout/SKILL.md
+++ b/skills/batch-fanout/SKILL.md
@@ -17,6 +17,19 @@ Next-up batch out to workers at once. Run it in its **own window**; it does NOT
 
 ## Procedure
 
+0. **Orphan sweep FIRST (assay-toolkit#14, human:<name> 2026-07-12).** Before dispatching any fresh brief,
+   scan the open PRs across ALL watched repos (oit, agent-runtime, medici-examples,
+   assay-toolkit, reconciler, decks, proposals):
+   `gh pr list --repo <r> --state open --json number,isDraft,updatedAt,reviewDecision,statusCheckRollup`.
+   A PR is **ORPHANED** when the worker owes it action (reviewDecision `CHANGES_REQUESTED` at the
+   current head, CI red, or a draft whose findings were never answered) AND it has had no
+   commit/comment for **> 4h** AND no live claim exists in `~/.claude/desk-tools/claims/`.
+   **Resuming an orphan takes PRIORITY over starting a fresh brief** — finishing started work beats
+   starting new (mm/10's drain-before-dispatch, applied to PRs; the cost of ignoring this: a PR sat
+   14h with unaddressed findings while workers took fresh briefs). Dispatch the resume-worker WITH
+   the PR's open findings as its task; it claims the PR like a brief. PRs that are approved-awaiting-
+   merge or ready-flipped are NOT orphans (they wait on the human, not a worker).
+
 1. **Sync to fresh `origin/main`, then regenerate + read Next-up.** `git fetch origin` and read from a
    checkout at current `origin/main` (Next-up is generated from main — a stale checkout yields a stale
    board, so workers collide on already-taken briefs or miss new ones):
```

## 4. Escalation labels — question / help wanted, all four desk skills (same day)

```diff
diff --git a/skills/batch-fanout/SKILL.md b/skills/batch-fanout/SKILL.md
index f3800a6..2b1d7b9 100644
--- a/skills/batch-fanout/SKILL.md
+++ b/skills/batch-fanout/SKILL.md
@@ -110,6 +110,16 @@ Next-up batch out to workers at once. Run it in its **own window**; it does NOT
   wrap-ups, collision observations, "this keeps happening" notes) MUST also be filed as an issue in
   **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering evidence and
   affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
+- **Escalation labels (human:<name> 2026-07-12): any desk/loop may label a PR or issue `question` (needs an
+  answer from human:<name> or a stronger-tier model to proceed — the item PARKS awaiting input) or
+  `help wanted` (the desk hit its capability/authority edge — a human or higher-tier model should
+  weigh in on the work itself).** Both are GitHub default labels — they exist in every repo, no
+  setup. Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from
+  whom when labeling; whoever answers removes the label with their response. A `question` that
+  matures into a formal decision fork promotes to `needs-decision` (issue-loop/06) with the
+  pros/cons template — these two stay lightweight. Labeled items are WAITING-ON-INPUT: they join
+  the human/escalation queue (I-28 panel, mm/11 ordering) and are NOT orphans for the
+  batch-fanout sweep.
 - **One brief = one branch = one PR.** A worker that discovers its brief is too big STOPS and splits
   per author-brief rules (data-first pieces, typed deps, README rows lands on main promptly); it keeps
   only the piece it was mid-implementing, the rest returns to the board as `todo`.
diff --git a/skills/pr-review-desk/SKILL.md b/skills/pr-review-desk/SKILL.md
index ef448a9..f975d5a 100644
--- a/skills/pr-review-desk/SKILL.md
+++ b/skills/pr-review-desk/SKILL.md
@@ -128,6 +128,16 @@ merges. Keep it in sync if the repo set or review conventions change.
   **medici-finance/assay-toolkit** — commentary is not a register, and insights buried in PR threads
   are lost. Include the triggering evidence and affected loops. Repo-specific defects still go to the
   repo's own tracker (issue-loop/05).
+- **Escalation labels (human:<name> 2026-07-12): any desk/loop may label a PR or issue `question` (needs an
+  answer from human:<name> or a stronger-tier model to proceed — the item PARKS awaiting input) or
+  `help wanted` (the desk hit its capability/authority edge — a human or higher-tier model should
+  weigh in on the work itself).** Both are GitHub default labels — they exist in every repo, no
+  setup. Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from
+  whom when labeling; whoever answers removes the label with their response. A `question` that
+  matures into a formal decision fork promotes to `needs-decision` (issue-loop/06) with the
+  pros/cons template — these two stay lightweight. Labeled items are WAITING-ON-INPUT: they join
+  the human/escalation queue (I-28 panel, mm/11 ordering) and are NOT orphans for the
+  batch-fanout sweep.
 - NEVER `git push` / merge / trigger workflows / mutating `kubectl` without human:<name>'s go. Flipping a PR
   ready is fine (it's not a merge); merging is not.
 - Never `git restore`/`clean` a shared checkout; the reviewers isolate in their own temp worktrees.
diff --git a/skills/the-desk/SKILL.md b/skills/the-desk/SKILL.md
index d622a65..1754c0d 100644
--- a/skills/the-desk/SKILL.md
+++ b/skills/the-desk/SKILL.md
@@ -85,6 +85,16 @@ mechanics below are the canonical text those skills were extracted from.
   an aside, a "this keeps recurring" note) MUST also be filed as an issue in
   **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering evidence
   and affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
+- **Escalation labels (human:<name> 2026-07-12): any desk/loop may label a PR or issue `question` (needs an
+  answer from human:<name> or a stronger-tier model to proceed — the item PARKS awaiting input) or
+  `help wanted` (the desk hit its capability/authority edge — a human or higher-tier model should
+  weigh in on the work itself).** Both are GitHub default labels — they exist in every repo, no
+  setup. Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from
+  whom when labeling; whoever answers removes the label with their response. A `question` that
+  matures into a formal decision fork promotes to `needs-decision` (issue-loop/06) with the
+  pros/cons template — these two stay lightweight. Labeled items are WAITING-ON-INPUT: they join
+  the human/escalation queue (I-28 panel, mm/11 ordering) and are NOT orphans for the
+  batch-fanout sweep.
 
 **Model-tier awareness (this session's live hazard)**
 - The desk can be **silently downgraded** mid-task. human:<name> is the out-of-band drift detector — when he
diff --git a/skills/verify-desk/SKILL.md b/skills/verify-desk/SKILL.md
index b7e9eb2..1625108 100644
--- a/skills/verify-desk/SKILL.md
+++ b/skills/verify-desk/SKILL.md
@@ -119,6 +119,16 @@ no human reviewer (needs-fixing-day2); the gate + this carve-out prevent the rec
   Evidence asides, "recurring enough to be worth a structural fix" observations) MUST also be filed as
   an issue in **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering
   evidence and affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
+- **Escalation labels (human:<name> 2026-07-12): any desk/loop may label a PR or issue `question` (needs an
+  answer from human:<name> or a stronger-tier model to proceed — the item PARKS awaiting input) or
+  `help wanted` (the desk hit its capability/authority edge — a human or higher-tier model should
+  weigh in on the work itself).** Both are GitHub default labels — they exist in every repo, no
+  setup. Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from
+  whom when labeling; whoever answers removes the label with their response. A `question` that
+  matures into a formal decision fork promotes to `needs-decision` (issue-loop/06) with the
+  pros/cons template — these two stay lightweight. Labeled items are WAITING-ON-INPUT: they join
+  the human/escalation queue (I-28 panel, mm/11 ordering) and are NOT orphans for the
+  batch-fanout sweep.
 - **Evidence-not-claims, applied hardest here** — the verifier's report is itself a claim; the value is the
   recorded command output, and (brief-16) the runner must be attributable and ≠ author. A verifier that
   self-verifies its own implementation is void.
```
