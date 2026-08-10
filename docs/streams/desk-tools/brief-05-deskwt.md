---
brief: desk-tools/05
title: deskwt — worktree add/remove under allowed prefixes only
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md (C-8, C-10, TM-5)", "CLAUDE.md 'Parallel sessions & worktrees' (the 2026-07-08 wiped-work incident this encodes)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  Worktree creation is the mandated first step of every worker session and prompts every time;
  cleanup is a raw `rm -rf`-class operation that already cost uncommitted work once (2026-07-08).
  A tool that can only touch sanctioned worktree paths makes the mandatory-isolation rule the
  path of least resistance instead of a prompt.
---

# Brief 05 — deskwt — worktree add/remove under allowed prefixes

> **Follow-on (feat/deskwt-prune):** a third verb `deskwt prune [--repo <path>] [--interval
> <dur>]` was added after this brief verified. It runs `git worktree prune` (bookkeeping)
> then removes ONLY worktrees proven safe — not the shared checkout, not the current
> worktree, tracked-clean, AND fully merged into `origin/main` (`git merge-base
> --is-ancestor HEAD origin/main`); an UNMERGED branch (open PR in flight), dirty, or
> unpushed worktree is always LEFT (active-worker guard). It reuses this brief's exact
> safe-remove helpers (no `--force`). `--interval` loops the sweep (k8s-pod / launchd
> supervisor). Motivation: worktree sprawl breaks the bash sandbox (E2BIG) and drives the
> #742 writeguard false-positives. Docs: `../assay-toolkit/tools/desk/README.md` (deskwt §),
> `../oit/scripts/launchd/README.md`.

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskwt/` (new); uses `../assay-toolkit/tools/desk/internal/deskkit` (brief 01)
facts:
- Subcommands: `deskwt add <name> [--branch B] [--base origin/main]` → creates
  `/private/tmp/oit-<name>` via `git worktree add`; `deskwt remove <path>` → `git worktree
  remove` + prune. Constructed argv only, no caller flag passthrough.
- **C-8 (path safety, both verbs):** the RESOLVED path (`filepath.EvalSymlinks`) must be under
  exactly `/private/tmp/oit-` or `<repo-root>/.claude/worktrees/` — prefix match on the
  resolved string, after resolving; a path that resolves elsewhere (symlink trick) → exit 5.
  The shared checkout (`git rev-parse --path-format=absolute --git-common-dir`'s parent) is
  refused by identity, not just prefix.
- **remove refusals:** dirty tree — staged/unstaged TRACKED changes only (untracked build
  artifacts like node_modules/.daml/dist do NOT block; a refusal that fires on every real
  worktree kills the verb) → exit 5 listing the dirty files (the 2026-07-08 incident guard: NEVER remove uncommitted TRACKED work); unpushed commits on the checked-out branch (`git log @{u}..` non-empty, or NO
  upstream at all) → exit 5; worktree not registered in `git worktree list` → exit 5. There is
  NO --force flag (C-7-style weakest verb).
- **add refusals:** target already exists → exit 5 (never reuse/clobber); `--base` ref
  unresolvable → exit 6 (C-10); repo not in deskkit set → exit 5.
- `git worktree add/remove` is mutating; all of C-5 (audit) applies; these are NOT
  outward-writes, so the outward-write rate limit does not apply (local-only verb class — record
  this distinction in deskkit if brief 01 didn't already; if ambiguous, NEEDS_CONTEXT).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement both verbs per facts; `deskkit.Guard()` first; audit every invocation.
2. Tests on a scratch repo fixture: symlinked path resolving outside prefix → exit 5;
   dirty-tree remove → exit 5 (create a file, don't commit); unpushed-commits remove → exit 5;
   no-upstream remove → exit 5; shared-checkout path → exit 5 by identity; existing target
   add → exit 5; bad --base → exit 6; kill switch → exit 3; happy-path add+clean-remove →
   exit 0 and the worktree is genuinely gone from `git worktree list`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskwt/... -count=1` | exit 0; includes every refusal test in Task 2 |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `go run ./tools/desk/cmd/deskwt add scratch-vt && go run ./tools/desk/cmd/deskwt remove /private/tmp/oit-scratch-vt; echo $?` | 0; worktree created then cleanly removed |
| 4 | `go run ./tools/desk/cmd/deskwt remove ~/work/oit; echo $?` | 5 (shared checkout refused by identity) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `cdf623c5`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskwt/... -count=1` | 0 | `ok .../deskwt 2.440s` — all Task-2 refusals: symlink-outside-prefix, shared-checkout-by-identity, unregistered, dirty-staged/modified-tracked, unpushed-commits, no-upstream, no-force-flag, kill-switch, existing-target, bad-base; untracked-artifact does NOT block; happy-path add + clean remove | 2026-07-12 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | clean | 2026-07-12 | opus-verifier |
| 3 | `deskwt add scratch-vt && deskwt remove /private/tmp/oit-scratch-vt` | UNRUN | real mutating worktree op outside a fixture (would register against the shared checkout's common git dir — read-only isolation forbids). Backed by fixture `TestAddThenCleanRemoveSucceeds` (add→exists→clean remove→pruned, no force) passing in row 1 | 2026-07-12 | opus-verifier |
| 4 | `deskwt remove <shared-checkout>; echo $?` | 5 | compiled binary refuses `… is the shared checkout … refused by identity, never removed` exit 5 (go1.26 go-run masks child 5→1; binary confirms 5) | 2026-07-12 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-12 | opus-verifier |

**VERIFY: PASS** — deskwt adds/removes worktrees only under allowed prefixes, matches prefix on the RESOLVED (symlink-followed) path, refuses the shared checkout by identity, refuses dirty/unpushed/no-upstream removes, has no force/override path, honors the kill switch (exit 5). Rows 1,2,4,5 real-pass; row 3 UNRUN (real mutating worktree op) backed by the passing fixture test.

## Review
Gate: model. Reviewer checks the prefix match happens on the RESOLVED path (symlink test is
present and meaningful) and that no force/override path exists in remove.
