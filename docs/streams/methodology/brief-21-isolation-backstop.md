---
brief: methodology/21
title: Mechanical isolation backstop — main-commit guard hook + dispatch-isolation protocol ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md))
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["[F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) (unresolved — subagent dispatch landed 3 commits on the shared checkout's main despite prose instructions)", "RETRO.md 'Worktree isolation: instruction vs hook' (operator/15 began in the shared checkout hours after the rule landed; PreToolUse-hook banked as an R-01 one-change candidate)", "docs/assay-review-1/README.md (B-04)", "methodology/06 (layer-c deny list — covers force-push/workflows/kubectl, deliberately not git commit)"]
---

# Brief 21 — Mechanical isolation backstop ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md))

## Context
files: .githooks/pre-commit (new); CLAUDE.md (2-line install/override note); docs/streams/methodology/README.md
(dispatch-protocol convention); recorded deltas for the user-level batch-fanout skill and the
superpowers:subagent-driven-development usage note (via methodology/22 or the human)
facts:
- Worktree isolation has failed as prose twice in two days: operator/15 (a session edited code in
  the shared checkout hours after the rule landed) and [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) (a dispatched subagent, prose-told to
  stay on its branch, committed to the shared checkout's `main`; its self-report claimed success and
  only a `git log --all` cross-check caught it). [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) is still `Resolved: no`.
- The layer-c deny list (methodology/06) blocks force-push, workflow triggers, and mutating kubectl.
  `git commit` is deliberately exempt (the desk and the sanctioned branch-push loop use it) — so
  there is no mechanical guard at [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)'s exact landing zone: a commit to `main` in the shared
  checkout by a session that shouldn't be there.
- git supports versioned hooks: commit `.githooks/` and activate per-clone with
  `git config core.hooksPath .githooks`. Hooks are local — CI checkouts and GitHub-side merges are
  unaffected. Sanctioned main-writers (the coordinator desk, verify-desk Evidence commits, human:<name> at
  the keyboard) pass by exporting an env override once per shell.
- The RETRO already banks "instruction vs hook" as an R-01 one-change candidate. This brief BUILDS
  the mechanism and documents it; switching it on in the shared checkout is the retro/desk
  enactment decision, not the implementer's.

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done. Do NOT run the core.hooksPath activation in
  the shared checkout — enactment is the desk's/retro's call (see Task 3).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **`.githooks/pre-commit`** (POSIX sh, no dependencies): if the current branch is `main` and
   `ASSAY_MAIN_COMMIT_OK` is unset/empty, refuse the commit with a 3-line message naming the rule
   (worktree isolation, [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)), the override (`export ASSAY_MAIN_COMMIT_OK=1` — desk/verify-desk
   sessions and human:<name>'s own shell), and the right path for workers (a worktree + feature branch).
   Detached HEAD and non-main branches pass untouched. Keep it under ~25 lines.
2. **Dispatch-isolation protocol** as a methodology README convention (and a recorded delta for the
   user-level batch-fanout skill): any subagent dispatch that will run shell/git in this repo
   passes `isolation: "worktree"` — prose instructions are not sufficient ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)); after the
   subagent reports, the dispatcher verifies landing with `git log --all --oneline | grep
   <expected-msg-substring>` rather than trusting the self-report.
3. **Install + enactment note** in CLAUDE.md (2 lines max, per the brief-14 diet direction): the
   activation command, the override variable, and that activation in the shared checkout is
   desk-enacted (R-01 candidate). On enactment, [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) flips `Resolved: yes` citing this brief.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -x .githooks/pre-commit && sh -n .githooks/pre-commit` | exit 0 (exists, executable, parses) |
| 2 | scratch clone: `git init /tmp/hooktest && cd /tmp/hooktest && git config core.hooksPath <repo>/.githooks && git commit --allow-empty -m x` on branch main, env unset | commit REFUSED, message names ASSAY_MAIN_COMMIT_OK |
| 3 | same, with `ASSAY_MAIN_COMMIT_OK=1` | commit succeeds |
| 4 | same clone, on branch `feature/x`, env unset | commit succeeds (non-main unaffected) |
| 5 | `grep -c "ASSAY_MAIN_COMMIT_OK" CLAUDE.md` | ≥1 (install/override documented) |
| 6 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -x .githooks/pre-commit && sh -n .githooks/pre-commit` | 0 | hook exists, executable, POSIX-parses clean | 2026-07-10 | opus-verifier |
| 2 | scratch clone, branch main, env unset, `git commit --allow-empty` | 1 | commit REFUSED; stderr names the rule + `ASSAY_MAIN_COMMIT_OK` override + worker-isolate path | 2026-07-10 | opus-verifier |
| 3 | same, `ASSAY_MAIN_COMMIT_OK=1` | 0 | commit succeeds | 2026-07-10 | opus-verifier |
| 4 | same clone, branch `feature/x`, env unset | 0 | commit succeeds (non-main unaffected) | 2026-07-10 | opus-verifier |
| 5 | `grep -c "ASSAY_MAIN_COMMIT_OK" CLAUDE.md` | 0 | 1 (install/override documented) | 2026-07-10 | opus-verifier |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — main-commit backstop hook refuses main without the override, honors it, leaves feature branches unaffected; documented in CLAUDE.md.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer should specifically check the hook cannot brick sanctioned flows: desk main-commits with
the override, worker branch commits, CI (hooks are local-only), and human:<name>'s own pushes.
