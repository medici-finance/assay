---
brief: assay:assay:desktools-v2:05
title: "the push guards judge the remote actually being pushed to — deskpushguard base ref (#1201) and insteadOf in the push-transport gate (#884)"
why: >-
  Two push-time guards each judge a remote they assumed rather than the one in use. deskpushguard
  compares a branch against refs/remotes/origin/main whatever remote git is pushing to, so in a
  worktree whose origin is a different repository than the push target it MISFIRES: every real
  commit is reported as a foreign commit and a correct push is refused (#1201). The
  push-transport gate reads the configured URL and never applies git's insteadOf rewrites, so an
  https remote that git will rewrite to SSH passes a gate whose whole job is to refuse SSH pushes
  under a bot identity (#884). One is a false refusal and one is a false pass; both come from
  reading a name or a string instead of asking git what it will really do.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1201, 884]
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session; re-derived from the two issues' own text 2026-09-17
sources:
  - "#1201 — title: 'hardcodes remote name origin — MISFIRES when a worktree's origin points at a different repo than the branch's own target'; its fix shape: resolve the remote by the PUSH TARGET, which the pre-push hook's own first argument already names"
  - "#884 — 'push-transport guard doesn't expand insteadOf/pushInsteadOf — https→SSH rewrite bypasses it'"
  - "tools/desk/cmd/deskpushguard/main.go:79-81 — run() reads the hook's SECOND argument (the URL) and never its FIRST (the remote name); :351 falls back to RemoteURL(\"origin\")"
  - "tools/desk/cmd/deskpushguard/foreigncommit.go:210-220 — resolveOriginMain resolves the literal refs/remotes/origin/main; :189-200 explains why the FULL refs/remotes/ spelling is load-bearing, which this brief keeps"
  - "tools/desk/cmd/deskpushguard/registerid.go:216 — `ls-remote --heads origin`"
  - "tools/desk/internal/deskkit/pushtransport.go:72 CheckPushTransport, :54 (Remote: empty means origin); tools/desk/cmd/deskpr/exec.go:77-83 passes Remote: \"origin\" and a `git config --list -z` reader"
  - "tools/desk/cmd/deskgit/deskgit.go:475 — the in-tree precedent: `ls-remote --get-url` expands url.<base>.insteadOf locally and contacts no remote"
  - "freshness-checked 2026-09-17 @ 57509073 — all of the above read at that commit. An earlier draft of this brief described #1201 as an EVASION and placed #884 in deskpushguard; both were wrong — #1201 is a misfire, and #884's gate is deskkit.CheckPushTransport"
consumers:
  - "tools/desk/cmd/deskpushguard: follow-up desktools-v2/05 (this brief; the base ref, URL fallback and liveness probe use the pushed-to remote — flips to fixed-here when the implementation lands)"
  - "tools/desk/internal/deskkit/pushtransport.go: follow-up desktools-v2/05 (this brief; the effective push URL is resolved through git's rewrite rules)"
  - "tools/desk/cmd/deskpr/exec.go, tools/desk/cmd/deskwt (the two CheckPushTransport callers): follow-up desktools-v2/05 (this brief; they pass the reader the new resolution needs)"
  - "git transport / push mechanics: out-of-scope (owned by desktools-go-git; this brief reads the effective URL and changes no transport)"
exec-tier: strong
exec-tier-why: >-
  question (c) — two security guards where a subtle error (an insteadOf rule resolved for fetch
  but not for push, a remote-derived base that silently falls back to origin) survives
  happy-path tests.
domain: complicated
version: 1
id: a8da9afb-a695-4ee1-bbb0-87064d9eae86
---

# Brief 05 — the push guards judge the remote actually being pushed to

## Context

files:
- `tools/desk/cmd/deskpushguard/main.go`, `foreigncommit.go`, `registerid.go` — take the remote
  from the hook's first argument.
- `tools/desk/internal/deskkit/pushtransport.go` (+ the two callers) — resolve the effective URL.
- NEW/extended `_test.go` beside each.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

single-point-of-failure: each guard IS a single evaluation, and this brief does not add a second
guard; it makes each evaluation true. The independent layer behind both is server-side: branch
protection and the App's own permissions bind whatever a client-side hook decides. For #884
there is also a second local signal — the credential actually used for the push — which the
transport gate exists to predict, not to replace.

facts:
- git invokes a pre-push hook as `<hook> <remote-name> <remote-url>`. deskpushguard reads only
  the URL. Its base ref, its URL fallback and its liveness probe all spell `origin`.
- #1201 is a FALSE REFUSAL. In a worktree whose `origin` is repository A while the push goes to a
  remote for repository B, the guard compares B's branch against A's `main`, finds every commit
  "foreign", and refuses. The fix is to compare against `refs/remotes/<pushed-remote>/main`.
  The same substitution also removes the mirror-image false PASS (a foreign commit that happens
  to be on A's main).
- KEEP the full `refs/remotes/<remote>/main` spelling. `foreigncommit.go:189-200` records why: a
  bare `<remote>/main` resolves a stray local branch of that name first. Substitute the remote
  NAME only.
- When the named remote has no `main` tracking ref, the answer is could-not-check, said as such
  (the guard's existing fail-open contract) — never a silent fall-back to `origin`.
- #884 is a FALSE PASS. `CheckPushTransport` decides from `remote.<name>.pushurl` / `.url` as
  configured. git applies `url.<base>.pushInsteadOf` (push only) and `url.<base>.insteadOf`
  before connecting, so the URL that leaves can be SSH when the configured one is https.
  Resolve it the way git does — `git remote get-url --push <remote>` applies both — through the
  existing single config/exec seam, contacting no remote.
- Out of scope: any credential change; moving these reads to go-git (desktools-go-git);
  weakening any existing assertion in either guard.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires removing/weakening either guard or its CI assertion: STOP and escalate
  (needs-decision, gate:human) — a guard's coverage is never traded for a green check.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. deskpushguard: read the hook's first argument as the remote name and use it for the base
   ref, the URL fallback and the liveness probe. No literal `origin` remains on those three paths.
2. `CheckPushTransport`: decide from the EFFECTIVE push URL (rewrites applied), and say in the
   refusal which rewrite rule turned the configured URL into an SSH one.
3. Tests, named so the Verify rows target them:
   `TestForeignCommitCheckUsesThePushedRemotesMain` (two remotes whose `main` differ; pushing to
   the second compares against the second — the #1201 reproduction, RED on the unfixed code with
   the "foreign commit" refusal),
   `TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin`,
   `TestPushTransportRefusesInsteadOfRewrittenToSSH` and
   `TestPushTransportRefusesPushInsteadOfRewrittenToSSH` (both RED on the unfixed code: the gate
   passes).
4. Quote the three red runs in the PR body under `## Fail-first`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | exit 0; the existing guard suites still pass — no assertion weakened |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | output contains the literal line `--- PASS: TestForeignCommitCheckUsesThePushedRemotesMain` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) — the #1201 misfire no longer reproduces |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | output contains the literal line `--- PASS: TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin` — the negative path: no silent fall-back to `origin` |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | output contains BOTH literal lines `--- PASS: TestPushTransportRefusesInsteadOfRewrittenToSSH` and `--- PASS: TestPushTransportRefusesPushInsteadOfRewrittenToSSH` — #884 in both rewrite forms; one line alone is a fail |
| 6 | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | exit 0 and no line printed — the literal is gone from the URL fallback and the liveness probe (comments spelling `refs/remotes/origin/main` in prose are not matched: the pattern is the quoted Go string) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — both changes make an existing guard evaluate the real
remote; no capability is removed and no credential is touched). The security-relevant nature is
handled by the no-weakening carve-out in Ground rules plus the negative-path rows. Row 3
dereferences #1201, row 4 is its negative path, row 5 dereferences #884 in both rewrite forms,
row 6 is the removal check. Reviewer records verdict + date in the stream README table.
