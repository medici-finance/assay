---
brief: desktools-go-git/05
title: migrate fetch + retire bespoke hardening (deskgit / deskadvisory / deskmerge)
wave: 4
depends: ["desktools-go-git/02", "desktools-go-git/03"]
unblocks: ["desktools-go-git/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — thesis (the fetch-hardening class collapses structurally); boundaries"
  - "docs/streams/desktools-go-git/inventory.md — op family 1 (fetch) and its three seam sites"
  - "docs/streams/desktools-go-git/brief-02-gitcore-transport-auth.md — gitcore.Fetch + in-process BasicAuth"
gate-why: >-
  This brief retires two bespoke security-hardening layers — the fetch-hardening essay
  that pins upload-pack / scrubs env / gates the effective origin URL, and the separate
  third-party-fork askpass + credential-helper suppression — and replaces them with
  in-process gitcore.Fetch. The replacement is structurally stronger (no exec, no
  helper, no insteadOf, no askpass file, token in-memory only), but retiring a hardened
  fetch that pulls attacker-influenceable third-party forks with a token in play is a
  security-posture change a model must not self-certify. A human confirms the new path
  preserves the roster/allowed-repo gate and the token-containment guarantee before the
  old fences are removed.
why: >-
  Fetch is where the git-binary threat model concentrated: config-named programs, env
  injection, upload-pack override, remote helpers, insteadOf, PATH trust. gitcore.Fetch
  executes none of them. Migrating the three fetch sites is what lets the old hardening
  code be deleted and the deskadvisory askpass-to-disk pattern disappear.
---

# Brief 05 — migrate fetch + retire bespoke hardening

## Context

files:
- `tools/desk/cmd/deskgit/deskgit.go`, `tools/desk/cmd/deskgit/transportexec.go`, `tools/desk/cmd/deskgit/exec.go` —
  the hardened `fetch` (pinned `--upload-pack=git-upload-pack`, `--refmap=`,
  `--no-recurse-submodules`, scrubbed env, effective-origin-URL gate). Migrate to
  `gitcore.Fetch`; keep ONLY the roster / allowed-repo gate (now: read
  `remote.origin.url`, parse, check allowed, use verbatim — no insteadOf layer to
  smuggle through). Delete the argv-hardening code made moot.
- `tools/desk/cmd/deskadvisory/advisory.go` — the third-party-fork fetch with its own controlled
  `GIT_ASKPASS` + `-c credential.helper=` + `init`/`checkout`. Replace with an
  in-memory-storer clone/fetch via `gitcore` so the token never touches disk or the
  child env; drop the askpass file entirely.
- `tools/desk/cmd/deskmerge/currency.go` — the base + `refs/pull/N/head` fetch. Migrate to
  `gitcore.Fetch`. (deskmerge's TRIAL MERGE stays on the binary — brief 07.)
- `docs/streams/desktools-go-git/inventory.md` (planned) — tick op family 1.

facts:
- go-git makes the hardening flags MOOT: no upload-pack override to pin, no remote
  helpers, no submodule recursion by default, no hooks; `--refmap=` pinning is inherent
  (refspecs are exactly the Go values passed). The effective-origin-URL gate reduces to
  reading `remote.origin.url` + allowed-repo check.
- Auth is in-process `BasicAuth{Username: "x-access-token"}` from `gitcore` (brief 02),
  token in-memory only. deskadvisory uses an in-memory storer so a third-party fork's
  contents are fetched without a working tree on disk and without a token on disk/env.
- Behaviour to preserve and golden: the allowed-repo gate still REFUSES a disallowed
  origin; a fetch of a valid base + PR ref still lands the expected refs; deskadvisory
  still returns the advisory it did before on the same fixture fork.
- Out of scope: push (brief 06); the preflight transport probe (brief 06); the deskmerge
  trial merge (brief 07); linked worktrees.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- The token stays in-memory: no askpass file, no env var, no token-in-URL, no log line.
- Do NOT weaken the roster / allowed-repo gate — carry it forward exactly. If migrating
  it would change what it admits, STOP and report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Migrate deskgit `fetch` to `gitcore.Fetch`; carry the allowed-repo gate forward as a
   `remote.origin.url` read + allowed check; delete the now-moot argv-hardening code.
2. Migrate deskadvisory to an in-memory-storer `gitcore` clone/fetch; delete the
   `GIT_ASKPASS` file, the `-c credential.helper=` suppression, and the on-disk
   init/checkout for the advisory path.
3. Migrate deskmerge's base + `refs/pull/N/head` fetch to `gitcore.Fetch`.
4. Golden-verify: allowed-repo REFUSAL still fires; a valid fetch lands the expected
   refs; deskadvisory returns the same advisory on a fixture fork; assert no askpass file
   is written and the token appears in no fetch URL/log.
5. Tick op family 1 in `inventory.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/ && go vet ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | exit 0; fetch + advisory goldens pass |
| 3 | `cd tools/desk && go test ./cmd/deskgit/ -run DisallowedOriginRefused` | exit 0; a disallowed origin is still REFUSED after migration (the allowed-repo gate survived — mutation-style row) |
| 4 | `cd tools/desk && grep -crE -e 'GIT_ASKPASS' -e 'credential.helper' cmd/deskadvisory/advisory.go` | exit 0; count 0 (the askpass file + helper-suppression path is gone) |
| 5 | `cd tools/desk && grep -crE -e 'upload-pack' -e 'refmap' cmd/deskgit/` | exit 0; count 0 (the moot argv-hardening flags are deleted) |
| 6 | `sh tools/desk/scripts/count-git-exec.sh` | prints `git-exec sites: <N>`; N below the count recorded before this brief |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data yes — retires two bespoke hardening layers that fetch
attacker-influenceable third-party forks with a token in play). The human reviewer
confirms the roster/allowed-repo gate is preserved (row 3 REFUSES a disallowed origin),
the token never reaches disk/env/URL/log, and the in-memory-storer path is sound before
the old fences are deleted. Reviewer records verdict + human name + date in the stream
README table.
