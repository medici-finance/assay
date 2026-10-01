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
  - "tools/desk/cmd/deskpushguard: fixed-here (the base ref, register-id candidates, URL fallback and liveness probe use the pushed-to remote; no remote name or no main on it is COULD-NOT-CHECK)"
  - "tools/desk/internal/deskkit/pushtransport.go: fixed-here (the effective push URL is resolved through git's rewrite rules, and a refusal names the rule)"
  - "tools/desk/cmd/deskpr/exec.go, tools/desk/cmd/deskwt (the two CheckPushTransport callers): fixed-here (both wire the push-url reader through their git argv seam)"
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
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | exit 0 |
| 2 | check +flow | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | exit 0; the existing guard suites still pass — no assertion weakened |
| 3 | check +dereference | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | output contains the literal line `--- PASS: TestForeignCommitCheckUsesThePushedRemotesMain` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) — the #1201 misfire no longer reproduces |
| 4 | check | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | output contains the literal line `--- PASS: TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin` — the negative path: no silent fall-back to `origin` |
| 5 | check +dereference | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | output contains BOTH literal lines `--- PASS: TestPushTransportRefusesInsteadOfRewrittenToSSH` and `--- PASS: TestPushTransportRefusesPushInsteadOfRewrittenToSSH` — #884 in both rewrite forms; one line alone is a fail |
| 6 | check | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | exit 0 and no line printed — the literal is gone from the URL fallback and the liveness probe (comments spelling `refs/remotes/origin/main` in prose are not matched: the pattern is the quoted Go string) |
| 7 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskpushguard/pushedremote-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, `Totals: 7 caught, 0 NOT CAUGHT`: putting `origin` back in place of the pushed remote on any one path (the run() default, the foreign-commit base or its empty-name branch, the register-id base, its empty-name branch, its sibling candidates, its liveness probe) reddens rows 3–4's tests |
| 8 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/pushtransport-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, `Totals: 30 caught, 0 NOT CAUGHT`: deciding from the configured url, falling back to it when no resolver is wired or it fails, passing a url-less remote git resolves to its bare name, dropping the insteadOf / pushInsteadOf attribution or its longest-prefix and alias rules, a remedy not decided by whether insteadOf rewrites its https target back to SSH, a two-step remedy collapsed to the rule alone, a set-url line that cannot run on a multi-valued pushurl, deskwt reading only the first push url, and either caller wiring no resolver each redden a test |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Verify — 2026-10-01 (post-merge, merged main 4b1f8fc8bfaf)

What moved since the last run: this is the first verify pass. There was no earlier Evidence block. The implementation is #1918 (merge commit f2005b00bb39), and it is an ancestor of the verified head 4b1f8fc8bfaf6520e26fe0659f944d22e43e3aa5.

Grounded expectation, written before running anything: rows 1–8 pass on merged main. The four named tests exist and pass with no SKIP. The two muhar specs report 7/0 and 30/0. The quoted origin literal is gone from main.go and registerid.go.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | exit 0 | Verify row 1. Exit 0 with no output from build or vet (go1.27.1 darwin/arm64). | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | exit 0; the existing guard suites still pass | Verify row 2. Exit 0: deskpushguard ok 50.077s, deskpr ok 44.483s, deskwt ok 80.560s. I also ran the same three packages with `-v` to make any skip visible: exit 0, 418 PASS lines, 0 SKIP lines, 0 FAIL lines. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | the literal `--- PASS:` line for the selected test | Verify row 3 (the #1201 dereference). Exit 0. The expected `--- PASS:` line for the selected test was printed (1.67s), with no SKIP and no FAIL anywhere in the output. Package ok 2.016s. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | the literal `--- PASS:` line for the selected test | Verify row 4 (negative path: no fall-back to origin). Exit 0. The expected `--- PASS:` line for the selected test was printed (0.74s), with no SKIP and no FAIL. Package ok 1.022s. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | BOTH `--- PASS:` lines (the insteadOf form and the pushInsteadOf form) | Verify row 5 (the #884 dereference, both rewrite forms). Exit 0. Two `--- PASS:` lines were printed, one for the insteadOf test (0.09s) and one for the pushInsteadOf test (0.07s), with no SKIP and no FAIL. Package ok 0.562s. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | exit 0 and no line printed | Verify row 6 (removal check). I ran it from the repo root. grep printed nothing and the final exit was 0. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskpushguard/pushedremote-mutations.json` | exit 0; baseline GREEN, positive control CAUGHT, 7 caught, 0 NOT CAUGHT | Verify row 7 (mutation). Exit 0. Output: "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate." All seven mutants were CAUGHT: the run() default, the foreign-commit base, the foreign-commit empty-name branch, the register-id base, the register-id empty-name branch, the sibling candidates, and the liveness probe. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/pushtransport-mutations.json` | exit 0; baseline GREEN, positive control CAUGHT, 30 caught, 0 NOT CAUGHT | Verify row 8 (mutation). Exit 0. Output: "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 30 caught, 0 NOT CAUGHT, 0 could-not-mutate." The caught mutants include: deciding from the configured url, falling back to it when no resolver is wired, the insteadOf and pushInsteadOf attribution, the longest-prefix and alias rules, the remedy decided by its https target, the multi-valued pushurl set-url form, deskwt reading only the first push url, and both callers wiring no resolver. | 2026-10-01 | assay-verifier-app[bot] @ 4b1f8fc8bfaf (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Scope traceability: each of the eight rows above discharges the Verify row with the same number. #1918 also changed some things that no Verify row checks: the tools/desk README section on push transport and the deskpr help text (both documentation), and a changelog fragment. The help-text change is covered indirectly, because row 1 builds that package. The brief's Task 4 asks for a "## Fail-first" section in the PR body quoting three red runs. No Verify row checks it, and I did not read the PR body in this pass.

Risk-bearing value. The brief's risk metadata is present and every field is no, including irreversible: no. So the fail-safe trigger does not fire on its metadata. I ran the enumeration anyway, because both changes are security guards. It covered the non-test Go changes in #1918: deskpushguard main.go, foreigncommit.go and registerid.go; deskkit pushtransport.go; and the deskpr and deskwt exec.go callers.

- `Remote = "origin"` and the resolver argv `remote get-url --push --all origin` @ tools/desk/cmd/deskpr/exec.go:106 and :111. This is the authority binding: it decides which remote the transport gate judges.
- `Remote = "origin"` and the same resolver argv @ tools/desk/cmd/deskwt/exec.go:39 and :44. Same binding.
- `base = "refs/remotes/" + remoteName + "/main"` @ tools/desk/cmd/deskpushguard/foreigncommit.go:225. This is the base-ref spelling.
- `remoteName = ""` (no default) @ tools/desk/cmd/deskpushguard/main.go:109.
- `len(pushurls) < 2` @ tools/desk/internal/deskkit/pushtransport.go:371. This is the threshold that decides when the remedy uses the multi-valued set-url form.
- `suffix = ".insteadof"` / `".pushinsteadof"` @ tools/desk/internal/deskkit/pushtransport.go:392. These are the rule-key match suffixes.
- The leading-dash refusal `strings.HasPrefix(remoteName, "-")` @ tools/desk/cmd/deskpushguard/registerid.go:221.

Ranking: every entry is a client-side advisory guard. A wrong value can be undone with an edit and a re-install, and the server-side branch protection and App permissions still bind regardless of what these guards decide. None is irreversible. The top-ranked entries are the two authority bindings and the base-ref spelling, because a wrong value there reinstates #884 or #1201 silently. I derived those three:

RISK-VALUE: DERIVED — Remote = "origin" @ tools/desk/cmd/deskpr/exec.go:106 (and deskwt/exec.go:39) — deskpr's own pushes are `git push -u origin <branch>` (tools/desk/cmd/deskpr/deskpr.go:362 and :641), and deskwt's transport fix writes remote.origin.pushurl (tools/desk/cmd/deskwt/transport.go:226). So origin is the remote these callers actually push to, which means the gate judges the real target. This is not the assumed-remote defect, which applies only to the pre-push hook. The hook now takes its remote from its first argument.
RISK-VALUE: DERIVED — resolver argv "remote get-url --push --all origin" @ tools/desk/cmd/deskpr/exec.go:111 — per git-remote(1), `get-url --push` returns the push URL with both pushInsteadOf and insteadOf expanded, and `--all` lists every push URL a push fans out to. It is a local config read that contacts no remote. That is exactly the effective-URL question #884 needs answered.
RISK-VALUE: DERIVED — base = "refs/remotes/" + remoteName + "/main" @ tools/desk/cmd/deskpushguard/foreigncommit.go:225 — per gitrevisions(7), a bare `<name>/main` resolves refs/heads/ before refs/remotes/, so only the fully qualified spelling is safe against a stray local branch (recorded at foreigncommit.go:186-200). Substituting the pushed remote's name compares the branch against the repository it is actually being pushed to (#1201).
The lower-ranked entries need no derivation. `len(pushurls) < 2` matches git's refusal of a plain `remote set-url --push` when pushurl has multiple values, and mutation row 8 covers it. The lowercase suffixes match `git config --list` lowercasing variable names.

rows_passed=8 rows_total=8

VERIFY: PASS

## Review
Gate: model (all four risk answers no — both changes make an existing guard evaluate the real
remote; no capability is removed and no credential is touched). The security-relevant nature is
handled by the no-weakening carve-out in Ground rules plus the negative-path rows. Row 3
dereferences #1201, row 4 is its negative path, row 5 dereferences #884 in both rewrite forms,
row 6 is the removal check. Reviewer records verdict + date in the stream README table.
