---
brief: methodology/36
title: Register tombstone check scopes to origin/main history — branch-only cleanup allowed, on-main deletion stays fatal
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [269]
schema: brief-v1
authored: 2026-07-10 by Fable session (issue #269 triage)
sources: ["issue #269", "PR #255 (the correct implementation, closed 2026-07-10 by human:<name> as a GATING decision, not a correctness rejection — commit c864e722 on fix/statusgen-integrity-scope-to-main)", "PR #242 (the blocked-cleanup evidence)", "methodology/23 (the check's origin)", "[F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md)/[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) (the anti-falsification lineage)", "freshness-checked 2026-07-10 @ 85b850f1"]
gate-why: >-
  This brief modifies the register anti-falsification guard itself (deletedRegisterFiles,
  the [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) tombstone-not-delete wire). human:<name> closed PR #255 — the same fix — precisely
  because loosening a tamper-guard on agent-only review + agent flip is a self-referential
  loop the App-verdict gate isn't meant to cover. All four risk answers are no (revertible
  Go lint code), but the gate is human BY DIRECTION: human:<name> confirms the scoping rule (main
  history = fatal, branch-only = allowed), the fail-closed fallback, and that post-merge
  strictness is not weakened.
why: >-
  A duplicate register entry committed only on a feature branch can never be git rm'd on
  that branch: deletedRegisterFiles walks the CURRENT checkout's history, so the branch's
  own add satisfies "once existed" (its error text says "on main" — the code does not
  match its message). Reviewers instruct pre-merge cleanups that can never lint green
  (PR #242). Branch-only files have no tamper-evidence value; forcing tombstones onto
  main for entries that were never live is permanent register noise.
---

# Brief 36 — Register tombstone check scopes to origin/main history

## Context
files: tools/statusgen/registers.go (`deletedRegisterFiles`, ~line 87) + registers tests;
docs/streams/methodology/README.md (row)
facts:
- Current mechanism (confirmed at 85b850f1): `git log --diff-filter=A --name-only -- <dir>/`
  with NO ref argument — enumerates adds reachable from HEAD, so branch-local adds count.
  The emitted PROBLEM text claims "once existed on main".
- The fix already exists and its correctness was not disputed: PR #255 / commit `c864e722`
  scoped enumeration to the HEAD↔origin/main merge-base and failed closed (strict, current
  behavior) when origin/main is unresolvable. Resurrect it (cherry-pick or re-derive) —
  do NOT invent a third semantics.
- Required semantics: deletion of a file present in `origin/main`'s history (merge-base
  form: `git log origin/main --diff-filter=A`, or merge-base..-scoped equivalent) = PROBLEM
  exactly as today; deletion of a branch-only file = allowed; `origin/main` missing/
  unreachable (shallow clone, offline) = fall back to today's strict HEAD-history behavior
  (fail closed — never fail open).
- Squash-merge caveat (from #269): after a squash-merge, branch history vanishes — the
  check on MAIN afterwards sees main's own history, so post-merge protection is unaffected.
  State this in the check's doc comment.
- Error-text honesty: whatever ships, the message must describe what the code checks.
- Tests: (a) branch-only add then delete → clean; (b) file in main history deleted on a
  branch → PROBLEM; (c) origin/main absent → strict fallback (branch-only delete → PROBLEM);
  existing register-integrity tests unweakened.
- Interim guidance until this lands (also posted on #269): reviewers do NOT instruct
  pre-merge register deletions; a branch-only duplicate is withdrawn IN-PLACE
  (`disposition: rejected` tombstone), which lints green today.
- CI note: the PR-lint runner has origin/main (actions/checkout with default depth may be
  shallow — verify fetch-depth in `.github/workflows/` and record it in Evidence; if
  shallow, the fallback path is what CI exercises and the merge-base path must be proven
  in a full clone).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD: table-driven tests for the three cases in facts (temp git fixtures with and
   without an `origin/main` ref), plus no-weakening runs of the existing register tests.
2. Implement per the required semantics — starting from PR #255's `c864e722`; keep or
   improve its fail-closed fallback; fix the PROBLEM text to match the actual scope.
3. Update the check's doc comment: the scoping rule, the fail-closed fallback, and the
   squash-merge rationale for why post-merge strictness is preserved.
4. Update the stream-README row; lint green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Register' -v` | exit 0; includes branch-only-delete-clean, main-history-delete-PROBLEM, and no-origin-fallback cases |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint; echo $?` | 0 |
| 4 | `grep -c "origin/main" tools/statusgen/registers.go` | ≥1 (scoping implemented, doc comment updated) |
| 5 | live drill: on a scratch branch, add a throwaway `docs/streams/intake/` file, commit, `git rm` + commit, run `--lint` | exit 0 (branch-only cleanup allowed); then delete a file that exists on main → PROBLEM |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). CI fetch-depth
     finding recorded here. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `444e95a4`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Register' -v` | 0 | passes, BUT the `-run 'Register'` filter does NOT match the 3 new scoping tests (named `TestDeletedFileIntegrity*`); confirmed via `-run 'DeletedFile' -v`: `TestDeletedFileIntegrityScopedToMain` (branch-only-delete-clean + main-delete-PROBLEM), `_BranchBehindMain`, `_NoOriginFallsClosed` all PASS. Substance present; brief `-run` regex mis-scoped (brief-text bug) | 2026-07-11 | opus-verifier |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | tests ok, vet clean | 2026-07-11 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-11 | opus-verifier |
| 4 | `grep -c "origin/main" tools/statusgen/registers.go` | 0 | 7 (≥1) — merge-base scoping + doc comment reference origin/main | 2026-07-11 | opus-verifier |
| 5 | live drill (scratch branch, isolated verifier worktree) | — | Part A: branch-only add→`git rm`→commit → `--lint` exit 0 (branch-only cleanup allowed, no tombstone PROBLEM). Part B: `git rm` a real main-history intake file → commit → `--lint` exit 1 `PROBLEM: register entry removed (tombstone-not-delete): … landed as of the merge-base with origin/main`. merge-base path exercised (origin/main resolved) | 2026-07-11 | opus-verifier |

Implementation: `registers.go` `deletedRegisterFiles` scopes to `git merge-base HEAD origin/main`, falls back to HEAD (fail-closed) when origin/main is unresolvable; doc comment documents the scoping rule + fail-closed fallback + squash-merge rationale.

**VERIFY: PASS** — branch-only register cleanup is allowed, on-main deletion stays a fatal tombstone PROBLEM, fail-closed when origin/main is unresolvable. Row 1's brief-text `-run` regex is mis-scoped (tests are `TestDeletedFileIntegrity*`) — substance confirmed under the correct filter. **This modifies the register-tombstone integrity/tamper-guard and re-lands closed #255 → human-gate: the `verified` flip + `human:ian` Reviewed stamp are proposed in THIS checkpoint PR and land only on human:<name>'s approve+merge, not on a model verify.**

## Review
Gate: human (gate-why above — human:<name> confirms the scoping rule, the fail-closed fallback,
and that post-merge strictness is unweakened; this re-lands #255 under the gate it was
closed to obtain).
