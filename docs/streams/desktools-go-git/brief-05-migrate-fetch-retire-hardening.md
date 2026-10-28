---
brief: assay:assay:desktools-go-git:05
title: migrate fetch + retire bespoke hardening (deskgit / deskadvisory / deskmerge)
wave: 4
depends: ["desktools-go-git/02", "desktools-go-git/03"]
unblocks: ["desktools-go-git/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
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
version: 3
id: 7d8455f5-acc2-477a-9d89-3facc6c7de7e
---

# Brief 05 — migrate fetch + retire bespoke hardening

## Context

files:
- `tools/desk/cmd/deskgit/deskgit.go`, `tools/desk/cmd/deskgit/transportexec.go`, `tools/desk/cmd/deskgit/exec.go` — the hardened `fetch` (pinned `--upload-pack=git-upload-pack`, `--refmap=`, `--no-recurse-submodules`, scrubbed env, effective-origin-URL gate). Migrate to `gitcore.Fetch`; keep ONLY the roster / allowed-repo gate (now: read `remote.origin.url`, parse, check allowed, use verbatim — no insteadOf layer to smuggle through). Delete the argv-hardening code made moot.
- `tools/desk/cmd/deskadvisory/advisory.go` — the third-party-fork fetch with its own controlled `GIT_ASKPASS` + `-c credential.helper=` + `init`/`checkout`. Replace with an in-memory-storer clone/fetch via `gitcore` so the token never touches disk or the child env; drop the askpass file entirely.
- `tools/desk/cmd/deskmerge/currency.go` — the base + `refs/pull/N/head` fetch. Migrate to `gitcore.Fetch`. (deskmerge's TRIAL MERGE stays on the binary — brief 07.)
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
| 4 | `grep -cE -e 'GIT_ASKPASS' -e 'credential.helper' tools/desk/cmd/deskadvisory/advisory.go` | exit 1 (grep exits 1 when nothing matches); output is `0` (the askpass file + helper-suppression path is gone). Command and Expect re-written 2026-10-03 (#1862): the path is named from the repository root with no `cd`, because a failed `cd` exits 1, the status this Expect requires; a missing file now exits 2 and fails the row. |
| 5 | `test -d tools/desk/cmd/deskgit && { grep -rnE -e 'upload-pack' -e 'refmap' tools/desk/cmd/deskgit/ \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` (the moot argv-hardening flags are deleted). Command and Expect re-written 2026-10-03 (#1862): the earlier form began with `cd tools/desk &&` and was decided by exit status alone, so a failed `cd` (status 1, the one a no-match grep also returns) passed it; the directory is now named from the repository root behind a `test -d` leg (an absent directory leaves no output line and exits 1, which fails the row), and the matching lines are counted. |
| 6 | `bash docs/streams/desktools-go-git/reproduce-05-counts.bash` | exit 0; pinned parent `44054ca37` reads 199 (52 direct, 147 seam), delivering commit `c93cd7dd8` reads 192 (52 direct, 140 seam), using each tree's own counter; exactly seven seam sites removed and no direct-spawn increase |

Row 6 was re-authored under [#3035](https://github.com/medici-finance/assay/issues/3035)
to measure this brief's **delivery delta**, rather than comparing a later main with
an older branch count. The [inventory](inventory.md#baseline-counter) records the
reproduced revisions and concurrent growth. The command archives only tracked
counter inputs, runs each revision's own counter, and fails on missing history,
unexpected output or a changed total/direct/seam result. It makes no remote reads.
The current counter remains advisory and is not re-baselined by this amendment.

The historical Evidence below is unchanged: its row-6 FAIL at `bbf8e3516245`
judged the old Expect and remains that result. This amendment is authored check
repair, **not a new independent Verify pass**. Lifecycle remains `implemented`;
the human security-posture review remains owed.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/ && go vet ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | pass exit=0 | sha256:bd60b2a83e96 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./cmd/deskgit/ -run DisallowedOriginRefused` | pass exit=0 | sha256:0efc7b15c130 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
| 4 | `grep -cE -e 'GIT_ASKPASS' -e 'credential.helper' tools/desk/cmd/deskadvisory/advisory.go` | pass exit=1 | sha256:9a271f2a916b | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
| 5 | `test -d tools/desk/cmd/deskgit && { grep -rnE -e 'upload-pack' -e 'refmap' tools/desk/cmd/deskgit/ \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
| 6 | `sh tools/desk/scripts/count-git-exec.sh` | pass exit=0 | sha256:479c94e0ee82 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (git-config) |
Verification-Attestation: medici-finance/assay#3029 run=7d76f2777784188682fe3419c92c75ccf11e57eac09712b1b7c59657f6595140 source=bbf8e351624538c2a03159cd5a58116fb97fc293 brief=docs/streams/desktools-go-git/brief-05-migrate-fetch-retire-hardening.md model=claude-opus-5-5 tier=any

Observed results. Each row was run separately at merged main bbf8e3516245 and again through the witness above. Go 1.27.2 darwin/arm64 was used with the real build and module caches, and the tests ran under a throwaway HOME.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1 (go build + go vet of deskgit, deskadvisory, deskmerge) | exit 0 | exit 0. Build and vet printed nothing. | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 (go test of the three packages) | exit 0; fetch + advisory goldens pass | exit 0: ok deskgit 90.3s, ok deskadvisory 1.2s, ok deskmerge 40.5s. Under -v, these advisory goldens PASS: Fetch Advisory Tree Token In Memory Only, Fetch Advisory Tree Failure Cleans Up, and Positive Control Shipped Credential Guard Catches Planted Token. The fixture-fork test (Check Advisory Fixture Fork Real In Process Fetch) is part of the same package run. | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 (go test deskgit -run DisallowedOriginRefused) | exit 0; disallowed origin still REFUSED | exit 0. Under -v, two tests PASS: Fetch Disallowed Origin Refused and Fetch HTTPS Disallowed Origin Refused. Each requires exit 5 and that no fetch runs. Mutation check on a scratch copy outside the home: with the allowed-repo check at deskgit.go line 481 short-circuited to false, Fetch HTTPS Disallowed Origin Refused FAILS ("exit = 6, want 5 (refused)"), and it passes again once the check is restored. The row discriminates. | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 (grep -c GIT_ASKPASS / credential.helper in the deskadvisory source) | exit 1; output 0 | exit 1; output 0 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 (count of upload-pack / refmap lines under cmd/deskgit) | output 0 | exit 0; output 0 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 (count-git-exec.sh) | git-exec sites N; N below the count recorded before this brief | exit 0; git-exec sites: 192 (52 direct, 140 seam). The count recorded before this brief, in the inventory's Brief 05 paragraph, is 179, read on 3c8326f95. That figure reproduces, as does the recorded 172 on d6c91cadc. 192 is not below 179, so this row FAILS as written. Like-for-like, each tree measured with its own copy of the script: 199 on 44054ca37 (the parent of the delivering squash commit c93cd7dd8), then 192 on c93cd7dd8 and on main. The brief removed 7 seam call sites. Concurrent desk-tool growth raised the count by 20 between 3c8326f95 and 44054ca37. This is the same stale-baseline class as the brief 06 row 6 ruling. Filed as medici-finance/assay#3035. | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Risk-bearing values. The trigger fired because the frontmatter has sensitive-data: yes. The enumeration covered the delivering commit c93cd7dd8, limited to its production files in deskgit, deskadvisory, deskmerge currency, and gitcore auth, fetchstage, fetchtree and gitcore. It found:
- appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13
- credentialHostOK host == "github.com" @ tools/desk/cmd/deskgit/deskgit.go:117, with port "" or "443" @ deskgit.go:121
- advisory fetch URL "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450
- advisory refspec "+refs/heads/*:refs/remotes/tpf/*" @ advisory.go:451
- trackingRefspec = "+refs/heads/*:refs/remotes/origin/*" @ deskgit.go:286
- deskmerge FetchOpts Force: true @ tools/desk/cmd/deskmerge/currency.go:306
- deskmerge PR refspec "+refs/pull/N/head:refs/deskmerge/pr-N" @ currency.go:334 and currency.go:259
- stageNamespace = "refs/gitcore-fetch/" @ tools/desk/internal/gitcore/fetchstage.go:46
- Tags: git.NoTags @ gitcore.go:575 and fetchtree.go:77
- the deniedTransportExec list losing its "upload-pack" entry @ tools/desk/cmd/deskgit/transportexec.go:57-63
- the temp dir pattern "deskadvisory-*" @ advisory.go:444

Ranked by what a wrong value would expose: the three token-destination bindings first, then the transport deny-list change. The refspecs, staging namespace, tag mode and temp pattern rank last because they are local and fixable by an edit.

RISK-VALUE: DERIVED — credentialHostOK host = "github.com", port "" or "443" @ tools/desk/cmd/deskgit/deskgit.go:117-121 — the token is a GitHub App installation token, so only GitHub's https origin may receive it. The deleted path scoped its credential helper as credential.https://github.com.helper, which answers only https://github.com on the default port, and 443 is that default. url.Parse puts the real host in Hostname(), so userinfo and lookalike hosts are rejected. Any other origin is fetched with no credential.
RISK-VALUE: DERIVED — advisory URL = "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450 — this is byte-identical to the deleted fetchURL literal, and the slug is roster-checked earlier in checkAdvisory. The token now rides only as an in-memory BasicAuth value on this one URL, sent to an in-memory storer (gitcore.FetchTree, memory.NewStorage). No askpass file and no environment variable carry it, and row 4 confirms the askpass and helper code is gone.
RISK-VALUE: DERIVED — appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13 — GitHub requires this username when an App installation token is presented over HTTPS basic auth. It is the same literal the deleted askpass script echoed for the Username prompt.
RISK-VALUE: DERIVED — deniedTransportExec without "upload-pack" @ tools/desk/cmd/deskgit/transportexec.go:57-63 — fetch builds no git argv any more, so no flag can reach a program. The probe used a scratch build under a throwaway HOME. It ran fetch with --upload-pack=, --upload-p=, --exec= and -c, and push with --upload-pack. Every case exited 5, with the FlagSet's "flag provided but not defined" refusal for the first. Push keeps its pinned receive-pack.

**VERIFY: FAIL** — row 6: git-exec sites 192 is not below the recorded pre-brief count 179 (stale-baseline class; like-for-like 199 to 192; medici-finance/assay#3035). Rows 1-5 pass. This brief is gate: human, and the human security-posture review stays owed regardless of this verdict.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/ && go vet ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | fail exit=1 sandbox=container-netns | sha256:5112d82a132a | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskgit/ -run DisallowedOriginRefused` | pass exit=0 sandbox=container-netns | sha256:3d522de9a677 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE -e 'GIT_ASKPASS' -e 'credential.helper' tools/desk/cmd/deskadvisory/advisory.go` | pass exit=1 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -d tools/desk/cmd/deskgit && { grep -rnE -e 'upload-pack' -e 'refmap' tools/desk/cmd/deskgit/ \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash docs/streams/desktools-go-git/reproduce-05-counts.bash` | pass exit=0 sandbox=container-netns | sha256:f64281ad113a | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |

Verification-Attestation: medici-finance/assay#3664 run=8ccd70a8454a1dd32398c0a34fc1d8fec10e0733f6f4326b9507ccf27b61614a source=331aba8f343c828763a1fae23c4cba26abff7560 brief=docs/streams/desktools-go-git/brief-05-migrate-fetch-retire-hardening.md model=claude-opus-5-5 tier=strong

Re-wake Evidence at merged main 331aba8f343c, attested by the dispatcher binding above. The witness table above was produced by statusgen verifyrun with sandbox=container-netns in a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47.3), started with --network none and the engine's default seccomp profile, GOPROXY=off. Every row was also run separately on darwin/arm64 (Go 1.27.2, git 2.56.0, throwaway HOME). The runner choice follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker).

Prior Evidence: VERIFY FAIL on 2026-10-15 at bbf8e3516245, on row 6 only (a stale pre-brief baseline). Since then row 6 was re-authored under medici-finance/assay#3035 to pin the delivery delta, and the deskmerge fetch gained private temporary refs and offline ref cleanup (medici-finance/assay#2513, medici-finance/assay#2525). The row-6 blocker is fixed: it passes in the witness and by hand.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1 (go build + go vet of deskgit, deskadvisory, deskmerge) | exit 0 | darwin exit 0, no output; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 (go test of the three packages) | exit 0; fetch + advisory goldens pass | darwin exit 0: ok deskgit 69.5s, ok deskadvisory 0.7s, ok deskmerge 73.8s; under -v the advisory goldens Fetch Advisory Tree Token In Memory Only, Fetch Advisory Tree Failure Cleans Up and Positive Control Shipped Credential Guard Catches Planted Token PASS; one SKIP (Shipped Checkdef Passes On Real Base, which needs an external base clone). Witness exit 1: deskgit and deskadvisory ok; deskmerge FAILS Fetch Refs Are Private and Fetch Failure Drains Refs, both with "operation refs leaked" after the cleanup warning "native git hook list required — git said: error: unknown subcommand: list". Classified environment-only, see below. | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 (go test deskgit -run DisallowedOriginRefused) | exit 0; disallowed origin still REFUSED | darwin exit 0; under -v Fetch Disallowed Origin Refused and Fetch HTTPS Disallowed Origin Refused PASS; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 (grep -c GIT_ASKPASS / credential.helper in the deskadvisory source) | exit 1; output 0 | darwin exit 1, output 0; witness exit 1 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 (count of upload-pack / refmap lines under cmd/deskgit) | output 0 | darwin exit 0, output 0; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 (reproduce-05-counts.bash) | exit 0; 199 (52, 147) on 44054ca37, 192 (52, 140) on c93cd7dd8 | darwin exit 0: "fetch delivery count: 199 -> 192; seven seam sites removed; direct spawns unchanged at 52"; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row 2 witness failure, classification: environment-only (the witness image's git version), with a test-harness gap. The two failing tests were added by medici-finance/assay#2513 and medici-finance/assay#2525, after this brief's delivery. They assert that deskmerge removes its temporary fetch refs, and that removal goes through the shared maintenance hook check, which shells out to the native git hook list subcommand. Git 2.47.3 in the witness image has no such subcommand ("error: unknown subcommand: list"), so cleanup is refused fail-closed, the refs are retained with a receipt, and the currency verdict is kept (the sibling test Fetch No Hook List Verdict PASSES in the same witness run). On darwin with git 2.56.0 the same two tests PASS, and the full row exits 0. The row's assertion is about the fetch and advisory goldens, not the environment, and every fetch and advisory golden passes in both runs. The gap is that these two tests carry no git-version floor guard (the repo's gitversion helper exists to turn exactly this into a named SKIP); it is filed separately and does not touch this brief's deliverable.

Risk-bearing values. The trigger fired on sensitive-data: yes. Enumeration covered the delivering commit c93cd7dd8 plus the later changes at main to the same production files (deskgit, deskadvisory, deskmerge currency, gitcore auth, fetchstage, fetchtree, gitcore). Current literals: appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13; credentialHostOK host == "github.com" @ tools/desk/cmd/deskgit/deskgit.go:117 with port "" or "443" @ deskgit.go:121; advisory URL "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450; advisory refspec "+refs/heads/*:refs/remotes/tpf/*" @ advisory.go:451; temp dir pattern "deskadvisory-*" @ advisory.go:444; trackingRefspec = "+refs/heads/*:refs/remotes/origin/*" @ deskgit.go:286; deskmerge FetchOpts Force: true @ tools/desk/cmd/deskmerge/currency.go:307; deskmerge PR refspec "+refs/pull/N/head:" + a private temporary ref under refs/deskmerge/tmp/ @ currency.go:356 (changed since the prior pass, from a fixed refs/deskmerge/pr-N); stageNamespace = "refs/gitcore-fetch/" @ tools/desk/internal/gitcore/fetchstage.go:46; Tags: git.NoTags @ gitcore.go:575 and fetchtree.go:77; deniedTransportExec without "upload-pack" @ tools/desk/cmd/deskgit/transportexec.go:57-63; the allowed-repo gate deskkit.IsAllowedRepo(repo) @ deskgit.go:483, whose admitted set is roster configuration rather than a code literal. Ranked: the three token-destination bindings first, then the transport deny-list, then refspecs, namespaces, tag mode and temp patterns (local, fixable by an edit).

RISK-VALUE: DERIVED — credentialHostOK host = "github.com", port "" or "443" @ tools/desk/cmd/deskgit/deskgit.go:117-121 — the token is a GitHub App installation token, so only GitHub's https origin on its default port may receive it; Hostname() excludes userinfo and lookalike hosts, and any other origin is fetched with no credential. Unchanged since the prior pass.
RISK-VALUE: DERIVED — advisory URL = "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450 — byte-identical to the deleted fetch URL literal, slug roster-checked earlier in checkAdvisory; the token rides only as an in-memory BasicAuth value to an in-memory storer, and row 4 confirms the askpass and helper code is gone.
RISK-VALUE: DERIVED — appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13 — the username GitHub requires with an App installation token over HTTPS basic auth, the same literal the deleted askpass script echoed.
RISK-VALUE: DERIVED — deniedTransportExec without "upload-pack" @ tools/desk/cmd/deskgit/transportexec.go:57-63 — fetch builds no git argv, so no fetch flag can reach a program; push keeps receive-pack, upload-archive, exec, exec-path and config-env denied.

**VERIFY: PASS** — all six rows pass on darwin; in the docker witness rows 1 and 3-6 pass and row 2's two deskmerge temp-ref tests fail only because git 2.47.3 lacks the git hook list subcommand (environment-only). This is a first stamp; the gate: human security-posture sign-off stays owed and is not given here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/ && go vet ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskgit/ ./cmd/deskadvisory/ ./cmd/deskmerge/` | fail exit=1 sandbox=container-netns | sha256:fab40325b6cd | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskgit/ -run DisallowedOriginRefused` | pass exit=0 sandbox=container-netns | sha256:5e9afb86a690 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE -e 'GIT_ASKPASS' -e 'credential.helper' tools/desk/cmd/deskadvisory/advisory.go` | pass exit=1 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -d tools/desk/cmd/deskgit && { grep -rnE -e 'upload-pack' -e 'refmap' tools/desk/cmd/deskgit/ \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash docs/streams/desktools-go-git/reproduce-05-counts.bash` | pass exit=0 sandbox=container-netns | sha256:f64281ad113a | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |

Verification-Attestation: medici-finance/assay#3713 run=5269a7d19d6a1559132576cbed4f6d4cb19126bec4557e4dfb4c4559d73f9abb source=b1cf3d7907080ca6ff8dc3b023a0e7558af1123e brief=docs/streams/desktools-go-git/brief-05-migrate-fetch-retire-hardening.md model=claude-opus-5-5 tier=strong

Receipt re-pass Evidence at merged main b1cf3d790708, attested by the dispatcher binding above. The witness table above was produced by statusgen verifyrun with sandbox=container-netns in a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47.3), started with --network none and the engine's default seccomp profile, GOPROXY=off. Every row was also run separately on darwin/arm64 (Go 1.27.2, git 2.56.0, throwaway HOME). The runner choice follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker).

Prior Evidence: VERIFY PASS on 2026-10-20 at 331aba8f343c (attestation medici-finance/assay#3664). It landed without a verify-outcome record because the outcome-record writer refused the brief's wrapped Context files: block at that source; medici-finance/assay#3685 has since put each files: entry on one line. Between 331aba8f343c and b1cf3d790708 no file under deskgit, deskadvisory, deskmerge or gitcore changed; the only change to this brief is that Context repair and the earlier Evidence landings.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1 (go build + go vet of deskgit, deskadvisory, deskmerge) | exit 0 | darwin exit 0, no output; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 (go test of the three packages) | exit 0; fetch + advisory goldens pass | darwin exit 0: ok deskgit 53.9s, ok deskadvisory 0.5s, ok deskmerge 58.0s; under -v the advisory goldens Check Advisory Fixture Fork Real In Process Fetch, Fetch Advisory Tree Token In Memory Only, Fetch Advisory Tree Failure Cleans Up and Positive Control Shipped Credential Guard Catches Planted Token PASS; one SKIP (Shipped Checkdef Passes On Real Base, which needs an external base clone). Witness exit 1: deskgit and deskadvisory ok; deskmerge FAILS Fetch Refs Are Private and Fetch Failure Drains Refs, both with "operation refs leaked" after the cleanup warning "native git hook list required — git said: error: unknown subcommand: list". Classified environment-only, see below. | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 (go test deskgit -run DisallowedOriginRefused) | exit 0; disallowed origin still REFUSED | darwin exit 0; under -v Fetch Disallowed Origin Refused and Fetch HTTPS Disallowed Origin Refused PASS; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 (grep -c GIT_ASKPASS / credential.helper in the deskadvisory source) | exit 1; output 0 | darwin exit 1, output 0; witness exit 1 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 (count of upload-pack / refmap lines under cmd/deskgit) | output 0 | darwin exit 0, output 0; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 (reproduce-05-counts.bash) | exit 0; 199 (52, 147) on 44054ca37, 192 (52, 140) on c93cd7dd8 | darwin exit 0: "44054ca37: git-exec sites: 199 (direct spawns: 52, seam call sites: 147)", "c93cd7dd8: git-exec sites: 192 (direct spawns: 52, seam call sites: 140)", "fetch delivery count: 199 -> 192; seven seam sites removed; direct spawns unchanged at 52"; witness exit 0 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row 2 witness failure, classification: environment-only (the witness image's git version), with a test-harness gap. It is the same failure the prior pass recorded, and it reproduces in a separate supplemental run of the row under -v in the same image with --network none. The two failing tests check that deskmerge removes its temporary fetch refs. That removal goes through the shared maintenance hook check, which shells out to the native git hook list subcommand. Git 2.47.3 in the witness image has no such subcommand, so cleanup is refused fail-closed, the refs are retained with a receipt, and the currency verdict is kept; the sibling tests Fetch No Hook List Verdict and Fetch Hook Keeps Verdict PASS in the same run. On darwin with git 2.56.0 all four tests PASS under -v, and the full row exits 0. The row asserts the fetch and advisory goldens, not anything about the environment, and every fetch and advisory golden passes in both runs. The gap is that these two tests have no git-version floor guard, which would turn this into a named SKIP. It is routed as a separate test-harness finding and does not touch this brief's deliverable.

Risk-bearing values. The trigger fired on sensitive-data: yes. Enumeration covered the delivering commit c93cd7dd8 plus the later changes at main to the same production files (deskgit, deskadvisory, deskmerge currency, gitcore auth, fetchstage, fetchtree, gitcore); none of those files changed since the prior pass, and every line below was re-read at b1cf3d790708. Current literals: appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13; credentialHostOK host == "github.com" @ tools/desk/cmd/deskgit/deskgit.go:117 with port "" or "443" @ deskgit.go:121; advisory URL "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450; advisory refspec "+refs/heads/*:refs/remotes/tpf/*" @ advisory.go:451; temp dir pattern "deskadvisory-*" @ advisory.go:444; trackingRefspec = "+refs/heads/*:refs/remotes/origin/*" @ deskgit.go:286; deskmerge FetchOpts Force: true @ tools/desk/cmd/deskmerge/currency.go:307; deskmerge PR refspec "+refs/pull/N/head:" + a private temporary ref under refs/deskmerge/tmp/ @ currency.go:356; stageNamespace = "refs/gitcore-fetch/" @ tools/desk/internal/gitcore/fetchstage.go:46; Tags: git.NoTags @ gitcore.go:575 and fetchtree.go:77; deniedTransportExec = receive-pack, upload-archive, exec, exec-path, config-env (no "upload-pack") @ tools/desk/cmd/deskgit/transportexec.go:57-63; the allowed-repo gate deskkit.IsAllowedRepo(repo) @ deskgit.go:483, whose admitted set is roster configuration rather than a code literal. Ranked: the three token-destination bindings first, then the transport deny-list, then refspecs, namespaces, tag mode and temp patterns (local, fixable by an edit).

RISK-VALUE: DERIVED — credentialHostOK host = "github.com", port "" or "443" @ tools/desk/cmd/deskgit/deskgit.go:117-121 — the token is a GitHub App installation token, so only GitHub's https origin on its default port may receive it; Hostname() excludes userinfo and lookalike hosts, and any other origin is fetched with no credential. Unchanged since the prior pass.
RISK-VALUE: DERIVED — advisory URL = "https://github.com/" + tpfSlug + ".git" @ tools/desk/cmd/deskadvisory/advisory.go:450 — byte-identical to the deleted fetch URL literal, slug roster-checked earlier in checkAdvisory; the token rides only as an in-memory BasicAuth value to an in-memory storer, and row 4 confirms the askpass and helper code is gone.
RISK-VALUE: DERIVED — appUsername = "x-access-token" @ tools/desk/internal/gitcore/auth.go:13 — the username GitHub requires with an App installation token over HTTPS basic auth, the same literal the deleted askpass script echoed.
RISK-VALUE: DERIVED — deniedTransportExec without "upload-pack" @ tools/desk/cmd/deskgit/transportexec.go:57-63 — fetch builds no git argv, so no fetch flag can reach a program; push keeps receive-pack, upload-archive, exec, exec-path and config-env denied.

**VERIFY: PASS** — all six rows pass on darwin; in the docker witness rows 1 and 3-6 pass and row 2's two deskmerge temp-ref tests fail only because git 2.47.3 lacks the git hook list subcommand (environment-only). This is a first stamp; the gate: human security-posture sign-off (medici-finance/assay#2371) stays owed and is not given here.

## Review
Gate: human (sensitive-data yes — retires two bespoke hardening layers that fetch
attacker-influenceable third-party forks with a token in play). The human reviewer
confirms the roster/allowed-repo gate is preserved (row 3 REFUSES a disallowed origin),
the token never reaches disk/env/URL/log, and the in-memory-storer path is sound before
the old fences are deleted. Reviewer records verdict + human name + date in the stream
README table.
