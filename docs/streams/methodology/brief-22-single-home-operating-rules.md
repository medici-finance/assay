---
brief: methodology/22
title: Single-home the operating rules — desk skills into the repo, reconcile doc-vs-practice drift
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["docs/assay-review-1/README.md (B-07, U-06)", "verify-desk SKILL vs practice: skill says dispatch opus+ verifiers and pushes are gated; practice (human:<name>-directed 2026-07-09) is glm-5.2 verifiers committing Evidence straight to main", "batch-fanout SKILL still names the reviewer bot medici-stuff[bot]; the App was renamed reviewer-app[bot] and repo docs reconciled in 6433e178 — the user-level skill was not", "user-level author-brief brief template ground rule 'NEVER git push' vs the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) sanctioned worker branch-push + draft-PR loop", "statusgen source inventory 2026-07-09 (docs overclaim: Verified-by trailer checks, Reviewed-cell runner-attribution, Verify-table pre-flight)"]
---

# Brief 22 — Single-home the operating rules

## Context
files: `.claude/skills/` (new in-repo homes: batch-fanout, pr-review-desk + deskboard.go +
mint-reviewer-token.go, verify-desk, the-desk); `~/.claude/skills/*` (become thin pointers — a
recorded delta the human applies, since user scope is outside this repo); CLAUDE.md (pointer
lines); docs/streams/methodology/README.md (rule-ownership note + the overclaim fixes)
facts:
- The methodology's four operating manuals (the desk skills) and both gate tools (deskboard.go,
  mint-reviewer-token.go) live in `~/.claude/skills/` — unversioned, un-PR-reviewable, absent from
  every fresh clone, and invisible to statusgen's link checker. Assay's own toolkit repo exists
  (medici-finance/assay-toolkit) yet the roles that RUN Assay here aren't in any repo.
- Practice diverged from the skill texts within a day, invisibly, because no PR touches them:
  (a) verify-desk says "dispatch a NON-IMPLEMENTER verifier (opus+)" and "never git push without
  human:<name>'s go" — practice is glm-5.2 verifiers and desk Evidence commits straight to main (both
  human:<name>-directed 2026-07-09, recorded only in one user's private session memory); (b) batch-fanout
  still names `medici-stuff[bot]` — the App is `reviewer-app[bot]` since the 6433e178 doc
  reconcile; (c) the user-level author-brief brief TEMPLATE says "NEVER git push", contradicting
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop every worker now runs (this repo's new briefs already use the corrected wording).
- Docs overclaim statusgen enforcement in three places (verified against source 2026-07-09):
  methodology README:143-149 says runner-attribution covers "Verified/Reviewed cell text" (it
  covers Verified + Evidence only) and implies `Verified-by:` trailers are best-effort-checked
  (statusgen never reads trailers); the author-brief closing-step pre-flight implies Verify tables
  are validated (no Verify parsing exists). methodology/19 makes two of these TRUE with lints —
  coordinate: whichever lands second reconciles the remainder in docs.
- human:<name>'s standing preference (skill-org): broadly-applicable core stays user-level; project-specific
  content lives in the project. The desk skills are entirely project-specific.
- Secrets stay out: the App private key (`~/.config/adopter/reviewer-app.pem`) and minted token
  paths remain config outside the repo; only the tooling that READS them moves in.

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Move the four desk skills + deskboard.go + mint-reviewer-token.go into this repo's
   `.claude/skills/` (project-scope skills), updating internal paths (`~/.claude/skills/...` →
   repo-relative) and CLAUDE.md references. Verify no secret material lands (key/token PATHS only).
2. While moving, reconcile each recorded contradiction to CURRENT practice: verifier tier → cite
   methodology/19's risk-keyed floor as the policy (not blanket opus+, not blanket glm); push
   policy → [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) + the verify-desk direct-to-main Evidence flow as human:<name> directed; reviewer bot name
   → `reviewer-app[bot]` everywhere; author-brief template ground rule → the "never push to
   MAIN / branch-push sanctioned" wording. Every reconciliation lists old→new in the PR body.
3. Write the user-level delta file (`docs/streams/methodology/evidence/brief-22-user-level-deltas.md` (planned)):
   for each `~/.claude/skills/` file, the thin-pointer stub content that replaces it. Applying it is
   the human's (or a user-scope session's) step — record that explicitly.
4. Add a **rule-ownership note** to the methodology README conventions: every operating rule has
   exactly ONE home (this repo's skills/README/CLAUDE.md per the brief-14 placement rule); other
   surfaces point, never restate. Session-memory is a cache, never the sole home of a load-bearing
   rule. Fix the three statusgen-overclaim passages to match verified tool behavior (or 19's lints,
   if landed).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `ls .claude/skills/batch-fanout/SKILL.md .claude/skills/pr-review-desk/SKILL.md .claude/skills/pr-review-desk/deskboard.go .claude/skills/pr-review-desk/mint-reviewer-token.go .claude/skills/verify-desk/SKILL.md .claude/skills/the-desk/SKILL.md` | all exist (exit 0) |
| 2 | `test -d .claude/skills/ && ! grep -rq "medici-stuff" .claude/skills/` | exit 0 (stale bot name gone from repo skills). Guarded by `test -d .claude/skills/ &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 3 | `grep -rl "BEGIN RSA\|BEGIN PRIVATE\|ghs_" .claude/skills/ \| wc -l` | 0 (no secret material committed) |
| 4 | `test -f docs/streams/methodology/evidence/brief-22-user-level-deltas.md` | exit 0 (delta file recorded) |
| 5 | `grep -ci "one home\|single home" docs/streams/methodology/README.md` | ≥1 (ownership note added) |
| 6 | `statusgen --root . --lint` | exit 0 (link checker passes over moved paths) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0174b912`; skills landed via PR #368 / `83a4a96e`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `ls .claude/skills/{batch-fanout,pr-review-desk,verify-desk,the-desk}/SKILL.md + pr-review-desk/{deskboard.go,mint-reviewer-token.go}` | 0 | all 6 files exist (6.5k–13k) — desk skills single-homed in-repo | 2026-07-12 | opus-verifier |
| 2 | `grep -rc "medici-stuff" .claude/skills/` | 1 | every file count 0 — stale bot name absent | 2026-07-12 | opus-verifier |
| 3 | `grep -rl "BEGIN RSA\|BEGIN PRIVATE\|ghs_" .claude/skills/ \| wc -l` | 0 | 0 — no secret material committed | 2026-07-12 | opus-verifier |
| 4 | `test -f docs/streams/methodology/evidence/brief-22-user-level-deltas.md` | 0 | delta file present | 2026-07-12 | opus-verifier |
| 5 | `grep -ci "one home\|single home" docs/streams/methodology/README.md` | 0 | 2 (≥1) — ownership note added | 2026-07-12 | opus-verifier |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0; link checker passes over moved paths | 2026-07-12 | opus-verifier |

Verifier confirmed: merge `83a4a96e` touched **no** `tools/statusgen` source — the Task-4 "statusgen-overclaim" fixes are prose corrections in the README describing behavior; the anti-falsification/register-integrity/tombstone logic is unchanged, so the standard model gate applies (no tamper-guard escalation).

**VERIFY: PASS** — desk skills single-homed into the repo (6 files, no stale bot name, no secrets), user-level deltas recorded, ownership note added; statusgen integrity logic untouched.

## Review
Gate: model (from frontmatter). Reviewer must check Task 2's old→new reconciliation list against
[F-16](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verifier-tier-practice-contradicts-the-tiering-default-and-t.md) and [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) — a "reconciliation" that silently picks the stale side re-ships the drift.
