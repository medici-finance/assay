---
brief: desktools-go-git/06
title: migrate push + retire ambient-credential machinery + preflight transport probe
wave: 4
depends: ["desktools-go-git/02", "desktools-go-git/03"]
unblocks: ["desktools-go-git/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — thesis (pushes inherit ambient credential machinery; the preflight probes because auth is ambient)"
  - "docs/streams/desktools-go-git/inventory.md — op family 2 (push), 3 (push dry-run probe)"
  - "docs/streams/desktools-go-git/brief-02-gitcore-transport-auth.md — gitcore.Push + gitcore.List, in-process BasicAuth"
why: >-
  Pushes today inherit whatever credential machinery the environment carries — helpers,
  the :443 insteadOf dodge, keychain state — which is exactly why the preflight has to
  PROBE what its own transport will do. gitcore.Push takes an explicit in-memory token,
  so the ambient machinery and the probe both disappear: a caller mints a repo-scoped
  token and can send it nowhere but that op's URL, and reachability is proved by an
  authenticated List rather than a dry-run push.
---

# Brief 06 — migrate push + retire ambient-credential machinery + preflight probe

## Context

files:
- `tools/desk/cmd/deskpr/deskpr.go`, `tools/desk/cmd/deskpr/exec.go` — `push -u origin <branch>` (literal
  argv). Migrate to `gitcore.Push` with in-process `BasicAuth`.
- `tools/desk/cmd/deskreply/deskreply.go`, `tools/desk/cmd/deskreply/exec.go` — the push path. Migrate to
  `gitcore.Push`.
- `tools/desk/cmd/verifyloop/durable.go` — the `push HEAD:main` step of the durable-Evidence loop.
  Migrate the PUSH to `gitcore.Push`. (The `pull --rebase` race resolution is NOT
  migrated here — go-git supports neither rebase nor non-FF pull; that redesign is the
  named follow-on, see spec boundaries. Keep the existing race handling around the new
  push for now.)
- `tools/desk/internal/deskkit/preflight.go` — DELETE the `push --dry-run --porcelain` transport
  probe (and its `exec.LookPath("git")`, `GIT_TERMINAL_PROMPT=0`); replace its purpose
  (prove transport + auth reachability) with `gitcore.List` (ls-remote) using the
  in-memory token.
- `docs/streams/desktools-go-git/inventory.md` (planned) — tick op families 2 and 3.

facts:
- `gitcore.Push(&PushOptions{RefSpecs: "HEAD:refs/heads/<branch>", Auth})`: force is
  impossible unless `Force`/`+` is set — "no force possible" is now type-level. Fully-
  qualified refspecs are the Go values passed.
- The probe's PURPOSE (discover the ambient credential's verdict) disappears when the
  token is explicit; `gitcore.List(&ListOptions{Auth})` proves transport + auth
  reachability positively, and it is what preflight callers now consult.
- Eliminated outright by this brief: credential helpers (and the stale-keychain 401
  class), the `:443` `insteadOf` dodge, token-in-URL pushes, `-c credential.helper=`
  suppression, `GIT_TERMINAL_PROMPT=0`, and the dry-run probe itself.
- Behaviour to preserve and golden: a push of a valid branch still lands the ref; a push
  that would need force is REJECTED (not silently forced); preflight still returns a
  reachable/unreachable verdict for its callers via `List`.
- Out of scope: fetch (brief 05); deskmerge push (its push stays paired with the trial
  merge — brief 07); the verifyloop rebase redesign (follow-on); linked worktrees.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only. (Tests exercise gitcore.Push against a LOCAL fixture remote,
  never a real one.)
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- The token stays in-memory: no helper, no env var, no token-in-URL, no log line.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Migrate deskpr, deskreply, and the verifyloop durable-push step to `gitcore.Push`
   with in-process `BasicAuth`.
2. Delete the preflight `push --dry-run` probe and its `LookPath`/`GIT_TERMINAL_PROMPT=0`
   machinery; replace its reachability check with `gitcore.List`, keeping the caller
   contract (reachable / unreachable verdict) intact.
3. Golden-verify against a LOCAL fixture remote: a valid push lands the ref; a force-
   requiring push is rejected; the preflight `List` returns the same verdict shape.
4. Add a NEIGHBOUR row exercising a preflight caller (the adjacent consumer of the shared
   reachability verdict), not just the probe itself.
5. Tick op families 2 and 3 in `inventory.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskpr/ ./cmd/deskreply/ ./cmd/verifyloop/ ./internal/deskkit/ && go vet ./cmd/deskpr/ ./cmd/deskreply/ ./cmd/verifyloop/ ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskpr/ ./cmd/deskreply/ ./cmd/verifyloop/ ./internal/deskkit/` | exit 0; push + List goldens pass against the local fixture remote |
| 3 | `cd tools/desk && go test ./cmd/deskpr/ -run ForcePushRejected` | exit 0; a force-requiring push is REJECTED, not silently forced (type-level no-force holds) |
| 4 | `cd tools/desk && grep -crE -e 'dry-run' -e 'GIT_TERMINAL_PROMPT' internal/deskkit/preflight.go` | exit 0; count 0 (the ambient-credential probe is gone) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run PreflightReachability` | exit 0; the neighbour row — a preflight CALLER still gets a correct reachable/unreachable verdict via List |
| 6 | `sh tools/desk/scripts/count-git-exec.sh` | prints `git-exec sites: <N>`; N below the count recorded before this brief |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — the sensitive credential path was reviewed in
brief 02; this brief routes callers onto it and deletes ambient machinery, golden-
verified with a force-push-rejection row and a preflight-caller neighbour row).
Reviewer records verdict + date in the stream README table.
