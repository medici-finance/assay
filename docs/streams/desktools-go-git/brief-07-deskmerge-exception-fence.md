---
brief: desktools-go-git/07
title: deskmerge exception — fence the trial merge as the sole git-binary caller, migrate the rest
wave: 3
depends: ["desktools-go-git/01", "desktools-go-git/02"]
unblocks: ["desktools-go-git/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — decision 5 (deskmerge trial merge stays on the git binary; the one sanctioned caller)"
  - "docs/streams/desktools-go-git/inventory.md — op family 9 (merge --no-ff --no-commit + conflict enumeration) vs the migratable read/commit verbs"
  - "docs/streams/desktools-go-git/brief-01-inventory-and-seam-contract.md — the internal/gitexec allowlist this fence lives in"
why: >-
  go-git's merge is fast-forward-only with no three-way merge and no conflict-stage
  enumeration, so it cannot express deskmerge's `merge --no-ff --no-commit` trial merge,
  conflicted-file enumeration, parent verification, and "rolled back; nothing pushed"
  posture — the desk's most security-reasoned git write. Per the spec's decided exception,
  deskmerge's trial merge stays on the git binary as the ONE sanctioned caller, fenced and
  allowlisted; its other verbs still migrate to gitcore so the exception is exactly the
  trial merge and nothing more.
---

# Brief 07 — deskmerge exception: fence the trial merge, migrate the rest

## Context

files:
- `tools/desk/cmd/deskmerge/assess.go` — the trial merge: `merge --no-ff --no-commit <sha>` +
  `diff --name-only --diff-filter=U` conflict enumeration + `merge --abort`. This STAYS
  on the git binary, routed through `internal/gitexec` with an allowlist entry naming
  exactly these verbs for exactly this tool.
- `tools/desk/cmd/deskmerge/merge.go`, `tools/desk/cmd/deskmerge/currency.go`, `tools/desk/cmd/deskmerge/exec.go` — the
  MIGRATABLE verbs: `rev-parse`, `rev-list --parents`, `merge-base`, `diff` reads, `add`,
  `commit` (with explicit `Author`/`Parents`), `update-ref -d`. Migrate these to
  `gitcore`. (deskmerge's base/PR FETCH migrated in brief 05; its scratch-worktree
  add/remove stays on the binary — worktree-family, follow-on.)
- NEW/EXTENDED doc in `internal/gitexec` (and a line in the stream README/inventory)
  stating the exception plainly: the trial merge is the sole sanctioned git-binary
  caller, why (FF-only go-git), and its usage envelope (human-gated `deskmerge merge`,
  run only on the desk machine, never by agents).
- `docs/streams/desktools-go-git/inventory.md` (planned) — mark op family 9 as EXCEPTION (fenced),
  tick the migrated deskmerge verbs.

facts:
- Merge-commit construction on the migrated side: pass both `Parents` explicitly to
  `gitcore` commit — deskmerge's parent VERIFICATION becomes construction, not post-hoc
  `rev-list --parents` parsing.
- The `internal/gitexec` allowlist, seeded broad in brief 01, is narrowed here so
  deskmerge's trial-merge verbs are its remaining justified entries; brief 08 flips the
  CI gate to FAIL on any git-exec site outside this allowlist.
- Behaviour to preserve and golden: a clean trial merge still succeeds and rolls back
  with nothing pushed; a CONFLICTING trial merge still enumerates the conflicted files
  and aborts. The trial-merge posture ("rolled back; nothing pushed") is unchanged — this
  brief does not move it to any server-side merge API (that decision, if ever taken, is a
  separate ruling and is NOT in scope here).
- Out of scope: replacing the trial merge with go-git or the update-branch API; scratch/
  linked worktree ops; the base/PR fetch (brief 05).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Do NOT change the trial-merge posture or move it off the binary — fence it, do not
  redesign it. If the exception seems to need redesign, STOP and report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Route deskmerge's trial merge (`merge --no-ff --no-commit` / conflict enumeration /
   `merge --abort`) through `internal/gitexec` with a narrow allowlist entry naming those
   verbs for deskmerge only.
2. Migrate deskmerge's read/commit verbs (`rev-parse`, `rev-list`, `merge-base`, `diff`
   reads, `add`, `commit` with explicit parents, `update-ref -d`) to `gitcore`.
3. Document the exception in `internal/gitexec` and in the stream README/inventory: the
   trial merge is the sole sanctioned git-binary caller; state the FF-only reason and the
   human-gated desk-only usage envelope.
4. Narrow the `internal/gitexec` allowlist to the deskmerge trial-merge verbs.
5. Golden-verify: clean trial merge succeeds + rolls back with nothing pushed; a
   conflicting trial merge enumerates conflicts and aborts.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskmerge/ ./internal/gitexec/ && go vet ./cmd/deskmerge/ ./internal/gitexec/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskmerge/ ./internal/gitexec/` | exit 0; trial-merge (clean + conflict) and allowlist goldens pass |
| 3 | `cd tools/desk && go test ./cmd/deskmerge/ -run ConflictEnumeratedAndAborted` | exit 0; a conflicting trial merge still enumerates conflicted files and aborts (posture preserved) |
| 4 | `cd tools/desk && grep -cE 'no-ff' cmd/deskmerge/assess.go` | exit 0; count >= 1 (the trial merge is still the `--no-ff --no-commit` form, not a server-side merge) |
| 5 | `grep -cE -e 'sole sanctioned' -e 'deskmerge' tools/desk/internal/gitexec/gitexec.go` | exit 0; count >= 1 (the exception is documented at the allowlist) |
| 6 | `sh tools/desk/scripts/count-git-exec.sh` | prints `git-exec sites: <N>`; N reduced to the deskmerge trial-merge allowlist entries only (below the pre-brief count) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — implements a decided exception; the trial-merge
posture is preserved unchanged and merely fenced, and the other verbs are a behaviour-
preserving migration). Reviewer confirms row 3 preserves conflict enumeration + abort and
that the allowlist is narrowed to exactly the trial-merge verbs. Reviewer records verdict
+ date in the stream README table.
