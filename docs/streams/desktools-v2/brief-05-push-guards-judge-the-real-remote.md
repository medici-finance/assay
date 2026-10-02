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
version: 2
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
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGitLabPushRewriteRefusesSSH$' -v` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); top-level TestGitLabPushRewriteRefusesSSH PASS; both rewrite forms, self-managed host, non-22 port, nested subgroup and oauth2 user (desktools-v2/12 GitLab row) |
| 10 | check | `cd tools/desk && go test ./cmd/deskpushguard/ -run '^TestPrePushCmdForwardsArgs$' -v` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); top-level TestPrePushCmdForwardsArgs PASS; generated Windows wrapper forwards remote arguments (desktools-v2/12 Windows row) |

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

### Non-implementer verifier run — 2026-10-02 claude-opus-5-5-verifier

SHA cross-check: the verification worktree HEAD is cca9028244d9b3bae6501f11df6ea6ebf043ed66 and the forge reports main at cca9028244d9b3bae6501f11df6ea6ebf043ed66. They are equal. The worktree was clean before the run and clean after it (the two mutation rows mutate in place and restore; the status listing after each long run was empty). Toolchain: go1.27.1 darwin/arm64, module proxy off, tests run under a throwaway home directory.

What moved since the last run: the Verify table is now version 2. Rows 9 and 10 are new (added by #1997, which also added the row 10 test file); rows 1–8 are unchanged in wording. The 2026-10-01 block covered rows 1–8 at 4b1f8fc8bfaf; this run covers all ten at cca9028244d9. Rows 1–8 gave the same verdict as that block (all pass, same mutation totals 7/0 and 30/0); only timings differ. Rows 9 and 10 have no earlier result to compare against.

Grounded expectation, written from the brief text before running anything: the pre-push guard takes the remote from the hook's first argument, so a two-remote repository compares against the pushed remote's main (row 3) and a remote with no main refuses instead of using origin (row 4); the transport gate refuses a remote whose effective push URL is rewritten to SSH by either rewrite form (row 5), including on a self-managed GitLab host, a non-22 port, a nested subgroup and an oauth2 user (row 9); no quoted origin literal remains in the two named guard files (row 6); putting origin back on any of seven paths, or any of thirty transport-gate regressions, reddens a test (rows 7, 8); the existing suites still pass (rows 1, 2); and the generated Windows hook wrapper passes its arguments through to the guard (row 10).

| # | Command | Expect | Result (exit + real output line) | Date | Runner |
|---|---------|--------|----------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | exit 0 | Verify row 1. Exit 0. Build and vet printed nothing. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 2 | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | exit 0; the existing guard suites still pass | Verify row 2. Exit 0. Three package lines, all ok: deskpushguard 40.953s, deskpr 34.412s, deskwt 63.883s. No FAIL line. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | the literal PASS line for the selected test | Verify row 3 (the #1201 dereference). Exit 0. Output contains "--- PASS: TestForeignCommitCheckUsesThePushedRemotesMain (1.60s)"; no SKIP, no FAIL; package ok 2.045s. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | the literal PASS line for the selected test | Verify row 4 (negative path, no fall-back to origin). Exit 0. Output contains "--- PASS: TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin (0.85s)"; no SKIP, no FAIL; package ok 1.132s. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | BOTH literal PASS lines | Verify row 5 (the #884 dereference, both rewrite forms). Exit 0. Output contains "--- PASS: TestPushTransportRefusesInsteadOfRewrittenToSSH (0.09s)" and "--- PASS: TestPushTransportRefusesPushInsteadOfRewrittenToSSH (0.07s)"; no SKIP, no FAIL; package ok 0.560s. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 6 | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | exit 0 and no line printed | Verify row 6 (removal check). Run at the repo root. Exit 0; zero bytes of output. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 7 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskpushguard/pushedremote-mutations.json` | exit 0; baseline GREEN, positive control CAUGHT, 7 caught, 0 NOT CAUGHT | Verify row 7 (mutation). Exit 0. "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate." The seven CAUGHT mutants are the run() default, the foreign-commit base, its empty-name branch, the register-id base, its empty-name branch, the sibling candidates and the liveness probe — the seven paths the row names. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 8 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/pushtransport-mutations.json` | exit 0; baseline GREEN, positive control CAUGHT, 30 caught, 0 NOT CAUGHT | Verify row 8 (mutation). Exit 0. "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 30 caught, 0 NOT CAUGHT, 0 could-not-mutate." | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGitLabPushRewriteRefusesSSH$' -v` | the named top-level PASS line; both rewrite forms, self-managed host, non-22 port, nested subgroup, oauth2 user | Verify row 9 (new in version 2). Exit 0. Output contains "--- PASS: TestGitLabPushRewriteRefusesSSH (0.47s)" and eight subtest PASS lines: insteadOf and pushInsteadOf, each with self_managed, non22_port, nested_subgroup and oauth2_user. No SKIP, no FAIL; package ok 0.664s. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |
| 10 | `cd tools/desk && go test ./cmd/deskpushguard/ -run '^TestPrePushCmdForwardsArgs$' -v` | the named top-level PASS line; generated Windows wrapper forwards remote arguments | Verify row 10 (new in version 2). Exit 0. Output contains "--- PASS: TestPrePushCmdForwardsArgs (0.00s)"; no SKIP, no FAIL; package ok 0.260s. | 2026-10-02 | claude-opus-5-5-verifier @ cca9028244d9 |

Execution witness (`statusgen verifyrun --brief <this brief> --dry-run`, statusgen v1.0.30, host darwin/arm64): exit 0, ten rows, ten pass, zero fail. Every row reported "pass exit=0". For rows 3, 4, 5, 9 and 10 the witness added the note "expect: exit-status only (nothing else in the Expect cell is machine-decidable — the output hash is the record a reviewer weighs)": the witness decided those five on exit status alone, and the required PASS lines were confirmed by hand in the table above. No row in this table is classed check:ci, so nothing needed the network-off Linux sandbox. The dry run wrote nothing into the brief. An audit of the brief as it stands on main (`statusgen verifyrun --check`) reports 0 pass, 0 fail, 10 missing of 10 and exits 2: main carries no witness table for this brief yet, so the witness has to be written by a non-dry run before the brief is witness-complete.

Risk-bearing value. The risk metadata is present and all four fields are no, so the fail-safe trigger does not fire on metadata; the enumeration was done anyway because both changes are security guards. Scope enumerated: the non-test Go files the brief's Deliverables name, read at cca9028244d9 — the three deskpushguard files, the deskkit push-transport file, and the deskpr and deskwt exec files.

- `Remote = "origin"` @ tools/desk/cmd/deskpr/exec.go:106 and tools/desk/cmd/deskwt/exec.go:39 — authority binding: which remote the transport gate judges.
- resolver argv `"remote", "get-url", "--push", "--all", "origin"` @ tools/desk/cmd/deskpr/exec.go:111 and tools/desk/cmd/deskwt/exec.go:44 — how the effective push URL is read.
- base ref `"refs/remotes/" + remoteName + "/main"` @ tools/desk/cmd/deskpushguard/foreigncommit.go:225.
- `remoteName := ""` (no default) @ tools/desk/cmd/deskpushguard/main.go:109.
- `strings.HasPrefix(remoteName, "-")` @ tools/desk/cmd/deskpushguard/registerid.go:221 — option-injection refusal on the liveness probe.
- `len(pushurls) < 2` @ tools/desk/internal/deskkit/pushtransport.go:371 — when the remedy line uses the multi-valued form.
- `suffix = ".insteadof"` / `".pushinsteadof"` @ tools/desk/internal/deskkit/pushtransport.go:392 and :394 — rule-key suffixes.

Ranking: all seven sit in client-side advisory guards; a wrong value is undone by an edit and a reinstall, and server-side branch protection and the App's permissions bind regardless. None is irreversible. The first three rank highest because a wrong value silently reinstates #884 or #1201; the last four are remedy-text or defensive details and need no derivation.

RISK-VALUE: DERIVED — Remote = "origin" @ tools/desk/cmd/deskpr/exec.go:106 (and tools/desk/cmd/deskwt/exec.go:39) — the gate must judge the remote the caller itself pushes to; deskpr's own pushes are `git push -u origin <branch>` (deskpr.go:362 and :641) and deskwt's transport fix writes remote.origin.pushurl (transport.go:226), so origin is the real target for these two callers, unlike the pre-push hook, whose target git supplies as its first argument.
RISK-VALUE: DERIVED — resolver argv "remote get-url --push --all origin" @ tools/desk/cmd/deskpr/exec.go:111 (and tools/desk/cmd/deskwt/exec.go:44) — git-remote(1): get-url expands insteadOf and pushInsteadOf, --push selects push URLs, --all lists every URL a push fans out to; it reads local config and contacts no remote. That is the effective-URL question the brief's facts require. Row 9's test independently checks this against git's own answer before asserting the refusal.
RISK-VALUE: DERIVED — base = "refs/remotes/" + remoteName + "/main" @ tools/desk/cmd/deskpushguard/foreigncommit.go:225 — gitrevisions(7) resolves refs/heads/ ahead of refs/remotes/ for a bare `<name>/main`, so only the fully qualified spelling is immune to a stray local branch of that name; the brief's facts require keeping that spelling and substituting the remote name only, which is what the literal does.

Findings.
- Check-definition note, rows 3, 4, 5, 9, 10: the Expect cells require specific PASS lines, but the witness can only decide these rows on exit status. A selector that matches nothing would exit 0 and the witness would record pass. Today all five selectors match (PASS lines observed above), so this is a weakness of the check form, not a failure. A grep for the PASS line piped into the row's command would make the row machine-decidable.
- Check-strength note, row 10: the test asserts that the generated wrapper text contains the argument pass-through token; it does not execute the wrapper. That matches the row's wording ("generated Windows wrapper forwards remote arguments") as a static check; behaviour on a real Windows host is outside what this row proves.
- No invented scope found on the guard paths. No flaky row: every row was run twice (by hand and by the witness) with the same verdict.
- Not examined in this pass: the brief's Task 4 (a Fail-first section in the implementing PR's body); no Verify row checks it.

rows_passed=10 rows_total=10

VERIFY: PASS — all ten Verify rows passed by hand and in the dry-run witness at cca9028244d9, with every required PASS line and both mutation totals observed.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | pass exit=0 | sha256:2b7a6b9d9a80 | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | pass exit=0 | sha256:74faf798ab34 | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | pass exit=0 | sha256:170158f9943a | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | pass exit=0 | sha256:58a024bf94b1 | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskpushguard/pushedremote-mutations.json` | pass exit=0 | sha256:f931a67481b2 | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/pushtransport-mutations.json` | pass exit=0 | sha256:94ffe5f9fa4b | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGitLabPushRewriteRefusesSSH$' -v` | pass exit=0 | sha256:5ff36abadb41 | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskpushguard/ -run '^TestPrePushCmdForwardsArgs$' -v` | pass exit=0 | sha256:44863cc451fd | 2026-10-02 | assay-verifier-app[bot] @ beb5461ca66e (on-behalf-of human:ian) (forge-identity) |

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/ ./internal/deskkit/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -timeout 10m ./cmd/deskpushguard/ ./cmd/deskpr/ ./cmd/deskwt/` | pass exit=0 | sha256:17f1ccf8c362 | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestForeignCommitCheckUsesThePushedRemotesMain -v` | pass exit=0 | sha256:694d3bdea73d | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestNoMainOnPushedRemoteIsCouldNotCheckNotOrigin -v` | pass exit=0 | sha256:1f76ededa3c0 | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestPushTransportRefuses.*RewrittenToSSH' -v` | pass exit=0 | sha256:8dc022741782 | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -nE '"origin"' tools/desk/cmd/deskpushguard/main.go tools/desk/cmd/deskpushguard/registerid.go; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskpushguard/pushedremote-mutations.json` | pass exit=0 | sha256:2bea5e0b0198 | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/pushtransport-mutations.json` | pass exit=0 | sha256:60f8006717ed | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGitLabPushRewriteRefusesSSH$' -v` | pass exit=0 | sha256:1cf421507384 | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskpushguard/ -run '^TestPrePushCmdForwardsArgs$' -v` | pass exit=0 | sha256:c24982ce3aed | 2026-10-02 | assay-verifier-app[bot] @ 443a1ca91fc1 (on-behalf-of human:ian) (forge-identity) |

## Review
Gate: model (all four risk answers no — both changes make an existing guard evaluate the real
remote; no capability is removed and no credential is touched). The security-relevant nature is
handled by the no-weakening carve-out in Ground rules plus the negative-path rows. Row 3
dereferences #1201, row 4 is its negative path, row 5 dereferences #884 in both rewrite forms,
row 6 is the removal check. Reviewer records verdict + date in the stream README table.
