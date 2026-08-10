---
brief: assay-dogfood/04
title: Dogfood cutover — this repo installs the plugin, retires loose skill copies, CLAUDE.md shrinks
wave: 2
depends: ["assay-dogfood/02", "assay-dogfood/03"]
unblocks: ["assay-dogfood/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [221]
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md))
sources: ["INTAKE [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)", "INTAKE [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)", "issue #221 (the problem class this retires for skills)", "PR #206 comments (shadowing — retirement of same-name loose copies is mandatory, not hygiene)", "methodology/14 (CLAUDE.md diet — coordinate, don't duplicate)", "freshness-checked 2026-07-10 @ fb9223ce"]
gate-why: >-
  This flips what EVERY session in the repo loads as its operating rules: skills switch from
  local files to the pinned plugin, resident rules move from CLAUDE.md text to a
  SessionStart hook, and the loose ~/.claude copies are deleted (mandatory — personal
  copies SHADOW the plugin's names' project-level equivalents and a stale one silently wins).
  human:<name> is confirming the swap of rule-distribution mechanism itself, with rollback rehearsed.
why: >-
  Briefs 02-03 make the artifacts exist; nothing is dogfooded until this repo actually
  consumes them and the old distribution surfaces are retired. Leaving both alive is the
  worst state — two sources of truth for the rules every session runs on, with shadowing
  deciding the winner silently.
---

# Brief 04 — Dogfood cutover

## Context
files: `.claude/settings.json` (enabledPlugins), `CLAUDE.md` (methodology sections shrink to
pointers — the hook now carries residency), this repo's thin wrappers (updated to reference
`assay:` names)
out-of-repo files: `~/.claude/skills/{the-desk,pr-review-desk,verify-desk,batch-fanout,author-brief}/`
(DELETED at cutover — see rule 7 / issue #221 protocol; the ~/.claude git repo records the
deletion as a commit, which is also the rollback vehicle)
facts:
- Order inside the cutover (rollback-safe): install+pin plugin → verify `assay:*` skills +
  hook live in a fresh session → update wrappers/CLAUDE.md pointers → LAST, delete the loose
  personal copies (commit in the ~/.claude repo). Rollback = revert that ~/.claude commit +
  disable the plugin — two steps, pre-written in the PR body.
- Shadow retirement is MANDATORY and verified, not assumed: post-deletion, the skill list
  must show the assay-namespaced skills and NO same-name personal survivors (the #206
  shadowing comment is the incident-grade rationale).
- CLAUDE.md changes coordinate with methodology/14: this brief moves METHODOLOGY-residency
  content (now hook-injected) to pointers; 14 diets what remains. If 14 has landed, this is
  a further shrink; if not, note the interaction in both PRs. Project-specific rules
  (Canton/k8s/debugging) are untouched — they are the project's, not the methodology's.
- The desk's standing sessions (this coordinator, review/verify windows) must be restarted
  onto the plugin after cutover — a live session keeps its old loaded skills; record the
  restart in Evidence (the #221 live-before-review hazard, inverted: here the OLD rules
  linger, not new ones leaking).
- agent-runtime / medici-examples repos: NOT in scope here — they onboard in brief 05's
  drill (their sessions load user-level + plugin, which is already cross-repo).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo deletion follows issue #221's protocol: declared above; staged as the
  documented plan in the PR; executed as the LAST step, WITH human:<name> at the gate; committed in
  the ~/.claude repo.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Consumer wiring: enabledPlugins pin in `.claude/settings.json` (exact version tag),
   wrapper updates to `assay:` references, CLAUDE.md methodology sections → one pointer
   paragraph each (moved content inventory in the PR body, methodology/14-style zero-rule-
   loss mapping).
2. Pre-write the rollback (two steps per facts) and the cutover-order runbook in the PR body.
3. Execute the cutover WITH human:<name>: install → verify → repoint → delete loose copies → restart
   standing sessions. Record each step in Evidence.
4. Post-cutover drill: one full desk cycle (board read, a review dispatch, a verify pass)
   running entirely on `assay:*` skills — the methodology works when consumed, not just
   when installed.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `jq -e '.enabledPlugins \| keys[] \| select(startswith("assay@"))' .claude/settings.json` | exit 0 (pinned enablement present) |
| 2 | `ls ~/.claude/skills/ \| grep -cE -e "the-desk" -e "pr-review-desk" -e "verify-desk" -e "batch-fanout" -e "author-brief" \|\| true` | 0 (loose copies retired). No existence guard on the directory: `~/.claude/skills/` being absent *is* the retired state, so an empty listing is a true pass, not a masked error. **RED at `5` on 2026-08-03** and left red deliberately. This brief is `todo` and Task 3 (the cutover, executed WITH human:<name>) has not run, so all five loose copies are still installed — `author-brief`, `batch-fanout`, `pr-review-desk`, `the-desk`, `verify-desk`. The `5` is the true state, not a broken row: before this sweep the row carried a literal-pipe alternation under `-E`, matched ~0, and printed `0` — a green cell over an unretired tree. Fixing the underlying state is out of scope here on two counts: the cutover is human:<name>'s to execute (verify-gate), and `~/.claude/**` is a serialized out-of-repo surface (#221) this PR does not touch. The count is read from stdout, not the exit code (`\|\| true` is the sanctioned rule-2 remedy), so a verifier must read the printed number — an exit 0 alone proves nothing here |
| 3 | fresh session skill list | shows `assay:the-desk` etc.; SessionStart rules injection observed |
| 4 | `git -C ~/.claude log --oneline -1` | the deletion commit (rollback vehicle exists) |
| 5 | post-cutover desk cycle (Task 4) | completes on plugin skills; noted per step |
| 6 | `statusgen --root . --lint; echo $?` (via the brief-03 pinned binary) | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item;
     human:<name>'s cutover sign-off recorded here (the verify-gate issue is the surface). -->

## Review
Gate: human (see gate-why). `/security-review` NOT required (no auth/funds surface) but the
reviewer walks the rollback before the flip: both steps must actually restore the pre-cutover
state in a scratch check, not just read plausibly.
