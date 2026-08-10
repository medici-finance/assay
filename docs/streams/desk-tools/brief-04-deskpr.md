---
brief: desk-tools/04
title: deskpr — push feature branch + open draft PR, draft-only by construction
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md (C-2, C-7, C-10)", "CLAUDE.md 'Parallel sessions & worktrees' (the worker recipe being encoded)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  Every worker ends its brief with the same two prompting steps: push the feature branch and
  open a draft PR — both standing-authorized (CLAUDE.md worker recipe, human:<name> 2026-07-09). One
  tool that can ONLY do the safe form (feature branch, draft, no force) removes the prompts
  without widening what a worker can do.
---

# Brief 04 — deskpr — push feature branch + open draft PR

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskpr/` (new); uses `../assay-toolkit/tools/desk/internal/deskkit` (brief 01)
facts:
- The policy being encoded (CLAUDE.md worker recipe): workers push their own feature branches
  and open DRAFT PRs themselves; push-to-main and merging remain human-gated; force-push is
  denied repo-wide (layer-c deny rules — this tool must be incapable of it, not merely avoid it).
- Two subcommands. `deskpr create --title T (--body-file F | --body-min B) [--base main]` and
  `deskpr update` (follow-up push of the current branch to its EXISTING open draft PR — the
  fix→re-review cycle's hot path; refuses if no open PR exists for the branch, or if that PR is
  not a draft authored on this branch). `create`:
  run from inside a git worktree. Behavior: verify preconditions → `git push -u origin <branch>`
  (plain push, never `--force`/`--force-with-lease`; enforce by constructing argv, no
  passthrough of caller flags to git) → `gh pr create --draft` → print the PR URL.
- **C-2 preconditions (verified in-tool):** (a) cwd is inside a git worktree whose checked-out
  branch is NOT the default branch (resolve default via `origin/HEAD`; refuse `main`/`master`
  outright even if origin/HEAD is unreadable — C-10: unreadable → exit 6); (b) the branch has
  ≥1 commit not on the default branch; (c) `origin` remote maps to a repo in the deskkit set
  (C-4); (d) working tree has no staged-but-uncommitted changes (refuse: the worker forgot to
  commit — exit 5 with that message).
- **C-7 (weakest verb):** draft is hardcoded — there is no flag to create ready; no verbs for
  edit/close/merge/ready exist in this tool (ready lives in deskpost with its preconditions).
- Idempotency (C-5): if an open PR already exists for this head branch, print its URL,
  result=noop, exit 0 — do NOT create a duplicate (the #140/#148 duplicate-PR incident class).
- Body via `--body-file` (16 KiB cap + deskkit bodycheck — it lives in deskkit precisely so
  this tool uses it) or `--body-min` one-liner. No stdin path. The same scan runs best-effort
  over title, branch name, AND `git diff <default>...HEAD` before any push (create AND update)
  — refuse exit 5 on hits (TM-2 seatbelt; committed-code residual recorded in scoping).
- Idempotency (C-5 v2): key `(repo, branch, headSHA, "pr-create")`; push-ok/create-failed rerun
  proceeds to create; existing open PR for the branch → noop printing its URL, exit 0.
- Repo scope: ALL THREE deskkit repos (workers author PRs in agent-runtime/examples too — the
  v1 scoping out-of-scope bullet was corrected in v2).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl — EXCEPT the tool under test
  performing its own single-purpose push in a live Verify row, which is the deliverable itself.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `deskpr create` exactly per facts; `deskkit.Guard()` first; audit every path.
2. Git interactions via constructed argv (`exec.Command("git", "push", "-u", "origin", branch)`)
   — never a shell string, never caller-supplied git flags.
3. Tests against a local bare-repo fixture (git init --bare + worktree): on-default-branch →
   exit 5; zero-commits-ahead → exit 5; staged-uncommitted → exit 5; origin outside repo set →
   exit 5; unreadable origin/HEAD → exit 6; kill switch → exit 3. gh interactions behind a
   PATH-shim fake recording argv: assert `--draft` is ALWAYS present and `--force` NEVER
   appears in any git argv.
4. Idempotency test: fake gh reports an existing open PR for the branch → noop exit 0, no
   create call recorded.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskpr/... -count=1` | exit 0; includes every negative test in Task 3 + the always-draft argv assertion |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | fixture run: in a scratch worktree on a feature branch with 1 commit, PATH-shimmed gh | push argv recorded without --force; gh argv contains `pr create --draft` |
| 4 | `go build -o /tmp/deskpr ./tools/desk/cmd/deskpr && DESK_TOOLS_DISABLED=1 /tmp/deskpr create --title x --body-min y; echo $?` | 3 |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

> Verify-4 amended at implementation (mid-flight-tweak rule, same commit): go1.26's
> `go run` masks a child's non-zero exit code to 1, so the disabled-path exit 3 must be
> observed on the COMPILED binary, not via `go run` (deskpost hit the same go1.26 issue).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `444e95a4`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskpr/... -count=1` | 0 | `ok .../deskpr 3.169s` — all Task-3 negatives (default-branch/zero-ahead/staged-uncommitted/origin-outside → exit 5; unreadable-origin-head → 6; kill-switch → 3; idempotent-noop) + `TestCreateSuccessAlwaysDraftNeverForce` (no `--force` in any git argv; `pr create --draft` always present) | 2026-07-11 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | clean | 2026-07-11 | opus-verifier |
| 3 | fixture run: scratch worktree + feature branch + PATH-shimmed `gh` (offline bare repo) | 0 | push argv `git push -u origin feature/test-branch` (no `--force`); gh argv `pr create --draft`; fake gh returned `.../pull/101` — no real GitHub mutation | 2026-07-11 | opus-verifier |
| 4 | compiled binary `DESK_TOOLS_DISABLED=1 deskpr create …; echo $?` | 3 | `refused: desk tools disabled (result=disabled)` exit 3 (compiled binary — go1.26 go-run exit-masking avoided per the Verify-4 amendment) | 2026-07-11 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-11 | opus-verifier |

**VERIFY: PASS** — deskpr is draft-only by construction (argv tests assert no `--force`, always `--draft`), refuses default-branch/zero-commit/dirty-tree/outside-origin, and honors `DESK_TOOLS_DISABLED`. All mutation surfaces exercised offline; no rows UNRUN.

## Review
Gate: model. Reviewer checks: no code path can emit `--force` or omit `--draft` (grep is not
enough — the argv-recording tests must assert it), and the default-branch refusal can't be
bypassed by a detached HEAD (detached → exit 6, unverifiable).
