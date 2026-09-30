# desktools-v2/01 — reach-around inventory

File:line-accurate inventory of every site that reaches past `tools/desk/internal/deskkit/forge.go`'s
`Forge` interface with a GitHub-specific fact, or with an ambient credential, across `statusgen/**`,
`tools/desk/**`, `tools/cellctl/**`, `plugins/assay/`/`.claude/` skill bodies, and `.github/workflows/**`.
Produced by re-sweeping the tree at the commit below with the commands listed under **How this inventory is
derived** — **not** copied from `spec.md`'s freshness note or this brief's own frontmatter, both of which are
already stale in places the sweep below calls out explicitly (clause: verify before applying a correction).
Shapes (a) and (c) each have a declared sweep command, and every line those commands print is either a row or is
accounted for in the text. Shapes (b) and (d) have no exhaustive command: their rows are the sites found by
reading, and the known sites that are not rows are listed as could-not-check, not cleared.

- Tree state: `origin/main` @ `fabe4926e` (2026-09-30). The first sweep ran at `951ca784d` (2026-09-18), i.e.
  **after** the `spec.md` §1 freshness base (`57509073`, 2026-09-17) and after the `desktools-v2` frontmatter's
  own freshness base (`e9fa19d3`, 2026-09-16). This is a **re-derivation** of that sweep: every row's file:line
  was re-anchored by matching its cited line's content at `951ca784d` against the current tree, and every
  surface was re-swept with the commands under **How this inventory is derived** below. Rows 1–69 keep their
  numbers (the text cross-references them); rows added by the re-derivation are numbered 70 onward and
  sit in the group they belong to. The previous anchor of this re-derivation was `b89b39572`; between it and
  `fabe4926e` every sweep below prints the same lines except one (`deskfleet/main.go:121`, see row 77), and the
  only cited lines that moved are `forge.go:1510`, `forge_github.go:1808` and `deskboard/board.go:630`, updated
  in place. Several rows record a site the older documents call "open" that the tree has
  since closed, and vice versa; each such case is called out under the row rather than silently reconciled away.
- Shapes: **(a)** a `gh` subprocess (`exec.Command("gh"`, a shim, or a script that shells `gh`); **(b)** a
  hardcoded remote name (`"origin"`); **(c)** a hardcoded query shape (a `pullRequest`/`mergeRequest` GraphQL
  block or a REST path fragment) built outside `forge_github.go`/`forge_gitlab.go`; **(d)** a token/identity
  assumption (an inherited `GH_TOKEN`, a `HOME` override, a token attached only to a child named `gh`).
- Counts are the run's own, not asserted from memory: **78 numbered reach-around rows** — **29 in
  `statusgen/**`** (16 files; forge-neutral/18's territory) and **49 in `tools/desk/**` + `plugins/assay/**` +
  `.github/workflows/**`** (desk's own territory, this stream's). Three desk-side rows (34, 35, 53) are
  **RESOLVED** and kept only as the record of where a §1 issue lived, so **75 sites are live**. Row 33b is a
  disputed annotation row and is not counted, as at the first sweep.
- Per shape (a row with two shapes counts under both): statusgen — (a) 29, (d) 1. Desk side — (a) 25,
  (b) 3, (c) 21 (17 in group E, 2 in group E2, and the two (c)-adjacent rows in group D), (d) 10.

## How this inventory is derived

Each group's row set is the output of a command, so the next drift is a diff, not an audit. Run from the
repository root.

- **Group A (statusgen, shape a):**
  `git grep -n 'exec.Command("gh"' -- 'statusgen/*.go' ':!*_test.go'` — one row per output line (29 lines,
  16 files at `fabe4926e`). The file-level set Verify row 5 checks is
  `grep -rl 'exec.Command("gh"' statusgen --include='*.go' | grep -v '_test.go$'`.
- **Groups B and C (tools/desk Go, shape a):** the `forgeban` scan. `TestNoForgeCLIShellout`
  (`tools/desk/internal/deskkit/forge_surface_test.go`) reconciles every Go exec site under `tools/desk` against
  `tools/desk/internal/forgeban/allowlist.go`; it passes at `fabe4926e` with 6 permitted forge-CLI call sites
  and 32 unresolved-argv exec sites (29 register rows). Each permit is one group B row, except that the
  `ambientLoginProbe` permit covers two `gh` launches in one function (rows 73 and 78). The literal-`gh` subset
  is also visible to a plain grep:
  `git grep -nE '"gh"' -- 'tools/desk/*.go' ':!*_test.go' ':!tools/desk/internal/forgeban/*'`.
- **Groups E and E2, and the could-not-check list (tools/desk Go, shape c):**
  `git grep -nE '/repos/|"repos/|api\.github\.com|"/graphql"|vnd\.github' -- 'tools/desk/*.go' ':!*_test.go' ':!tools/desk/internal/deskkit/forge_github.go' ':!tools/desk/internal/deskkit/forge_gitlab.go' ':!tools/desk/cmd/deskpost/github.go' | grep -vE ':[0-9]+:[[:space:]]*//'`
  — the two backend files and group E's file are excluded, and so are comment-only lines. The pattern matches
  `/repos/` anywhere in a literal, so a path built as `%s/repos/…` on a base URL is caught as well as one that
  starts with `/repos/`, and `"repos/` catches a path handed to `gh api` without a leading slash. Every line it prints at `fabe4926e` is a row in group E2, falls inside an existing row, or
  is named under **Not classified by this re-derivation** below.
- **Shell and generated-shell surfaces (groups C, G):**
  `git grep -nE '(^|[^A-Za-z0-9_./-])gh (api|issue|pr|repo|auth|run|workflow|label|release|search)( |$)' -- plugins/assay tools/cellctl tools/desk .claude ':!*.go' ':!*.md' ':!*.json'`
  plus `git grep -n 'exec gh\|gh auth token' -- tools/desk/cmd/cellctl` for the shell the Go cell launcher
  generates. Comment lines, `command -v gh` presence probes, test stubs (`*.test.sh`, `tests/`) and
  `testdata/` oracles are excluded.
- **Group H (workflows):** the same `gh <subcommand>` pattern over `.github/workflows`, comment lines excluded.
- **Re-anchoring a cited line:** for a citation `F:N` made at base `B`, `git show B:F | sed -n Np` gives the
  cited content, and `grep -nxF -- "<that content>" F` gives its line(s) now.

## Reach-around sites

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|

### A. `statusgen/**` — owned by `forge-neutral/18`, not by this stream (29 sites, 16 files)

Every non-test `statusgen/**` file shelling `gh` (`grep -rl 'exec.Command("gh"' statusgen --include='*.go'`,
tests excluded), one row per `exec.Command("gh", …)` call. statusgen is a separate Go module that does not
import `deskkit` **by design** (`statusgen/forgeread.go` header) — it reaches the seam by *running* the
`deskread` verb and parsing its JSON envelope, never by linking `deskkit`. So the "seam op" column here names
what `deskread`'s envelope would need to carry, not a `Forge` method statusgen would call directly; none of
these rows proposes a client, a library, or a port for statusgen (out of scope, per this brief's own facts).
`defaultScanIssueLister` (row 20) has **already** migrated onto `deskreadIssueLister` since #1223 — it is listed
here only because `ghIssueLister`, the pre-migration implementation it superseded, is still live code reachable
from three *other* statusgen modes (row 20's own function is dead on the `--scan-issues` path but still called by
`--transcribe-scan`, `--transcribe-scan-delta` and `--transcribe-verdict`; see its row). Since the first sweep,
#1255 moved the `--scan-issues` comment read and bless read onto the native forge as well
(`defaultScanCommentLister`, `defaultScanBlessChecker`, wired at `statusgen/main.go:1863`), so rows 11 and 21
are **no longer on the `scanloop` path**; they stay rows because `--transcribe-scan` still wires both.

This group's scope is shape (a) only, as this brief's facts define it. statusgen's other GitHub facts are not
rows here and are recorded for forge-neutral/18: the in-process REST client in `statusgen/ghfetch.go` (its
`https://api.github.com` base at `statusgen/ghfetch.go:47` and its inherited `GITHUB_TOKEN` read at
`statusgen/ghfetch.go:71`), and the `git remote get-url origin` reads at `statusgen/forge.go:113`,
`statusgen/corroborate.go:1545` and `statusgen/doratiming.go:626`.

| # | file:line | tool/skill | shape | issue | seam op / envelope field (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 1 | statusgen/transcribescan.go:77 | statusgen (`--transcribe-scan`) | (a) | #1223 | GAP — author-identity resolution (`ghAuthorResolver`) | forge-neutral/18 |
| 2 | statusgen/transcribescan.go:109 | statusgen (`--transcribe-scan`) | (a) | #1223 | GAP — comment-permalink resolution (`ghCommentResolver`) | forge-neutral/18 |
| 3 | statusgen/doratiming.go:631 | statusgen (`--dora`) | (a) | #1223 | GAP — ambient-repo fallback #3 of 3 (`doraTargetRepo`; #1/#2 are `$GITHUB_REPOSITORY` and `git remote get-url origin`, neither of which shells `gh`) | forge-neutral/18 |
| 4 | statusgen/issues.go:577 | statusgen (`--issues`/`--dora`) | (a) | #1223 | GAP — all-state issue metrics list (`ghIssueMetricLister`; `ListOpenIssues` covers only `state=open`) | forge-neutral/18 |
| 5 | statusgen/decisiongateanchor.go:229 | statusgen (decision-gate anchor) | (a) | #1223 | `GetIssueTyped`-shaped (`fetchDecisionGateIssue`) | forge-neutral/18 |
| 6 | statusgen/claimdecay.go:63 | statusgen (claim-decay reader) | (a) | #1223 | GAP — all-state PR list with `headRefName`/`isCrossRepository` fields (`ghPRListJSON`; `ListOpenChanges` is open-only) | forge-neutral/18 |
| 7 | statusgen/briefflowreview.go:72 | statusgen (brief-flow review) | (a) | #1223 | GAP — merged-PRs-since query (`ghBFPRSource.MergedPRs`) | forge-neutral/18 |
| 8 | statusgen/briefflowreview.go:103 | statusgen (brief-flow review) | (a) | #1223 | `ReviewsAtHead`-shaped (`ghBFPRSource.Reviews`) | forge-neutral/18 |
| 9 | statusgen/transcribeverdict.go:506 | statusgen (`--transcribe-verdict`) | (a) | #1223 | `GetIssueTyped`-shaped (`ghVerdictIssueResolver`) | forge-neutral/18 |
| 10 | statusgen/transcribeverdict.go:579 | statusgen (`--transcribe-verdict`) | (a) | #1223 | `ChecksAtHead`-shaped (`ghVerdictMainHealth`) | forge-neutral/18 |
| 11 | statusgen/trustgate.go:208 | statusgen (trust gate, `--transcribe-scan`) | (a) | #1223, #628 (**off the `scanloop` path since #1255** — `--scan-issues` now reads through `deskreadIssueBlessChecker`) | `IssueTrustEvents`-shaped (`ghIssueBlessChecker`) | forge-neutral/18 |
| 12 | statusgen/corroborate.go:850 | statusgen (corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`ghPRBaseRef`) | forge-neutral/18 |
| 13 | statusgen/corroborate.go:1118 | statusgen (corroboration) | (a) | #1223 | `ChangeDiff`-shaped (`fetchPRDiff`) | forge-neutral/18 |
| 14 | statusgen/corroborate.go:1146 | statusgen (corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`fetchPRData`) | forge-neutral/18 |
| 15 | statusgen/citationcorroborate.go:435 | statusgen (citation corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`fetchCitedArtifact`, PR form) | forge-neutral/18 |
| 16 | statusgen/citationcorroborate.go:461 | statusgen (citation corroboration) | (a) | #1223 | `GetIssueTyped`-shaped (`fetchCitedArtifact`, issue form) | forge-neutral/18 |
| 17 | statusgen/briefdecision.go:41 | statusgen (decision-queue source) | (a) | #1223 | `SearchIssues`-shaped (`ghDecisionQueueSource.Issues`) | forge-neutral/18 |
| 18 | statusgen/autonomy.go:451 | statusgen (`--autonomy`) | (a) | #1223 | GAP — merged-PR authors, all-state (`autonomyMergedAuthors`) | forge-neutral/18 |
| 19 | statusgen/autonomy.go:479 | statusgen (`--autonomy`) | (a) | #1223 | GAP — merged-PR gate rollups, all-state (`autonomyGates`) | forge-neutral/18 |
| 20 | statusgen/scanissues.go:116 | statusgen (`ghIssueLister`, legacy) | (a) | #1223 | superseded by `deskreadIssueLister` **on the `--scan-issues` path only**; still the live implementation for `--transcribe-scan`, `--transcribe-scan-delta` and `--transcribe-verdict` (wired at `statusgen/main.go:1870`, `:1877`, `:1884`) — GAP until those modes migrate too | forge-neutral/18 |
| 21 | statusgen/scanissues.go:953 | statusgen (`issueCommentLister`, `--transcribe-scan`) | (a) | #1223, #628 (**off the `scanloop` path since #1255** — `--scan-issues` now reads through `defaultScanCommentLister`) | GAP — per-issue comment list, `--paginate` (deskread's `OpenIssues` envelope carries no comments) | forge-neutral/18 |
| 22 | statusgen/autoflip.go:793 | statusgen (`--auto-flip-model`) | (a) | #1223 | GAP — merged-PR-for-commit lookup (`ghModelFlipSource.MergedPRForCommit`) | forge-neutral/18 |
| 23 | statusgen/autoflip.go:873 | statusgen (`--auto-flip-model`) | (a) | #1223 | GAP — PR branch commit list (`prBranchCommits`) | forge-neutral/18 |
| 24 | statusgen/autoflip.go:969 | statusgen (`--auto-flip-model`) | (a) | #1223 | `GetPullRequest`-shaped, head/state/mergedAt (`ReviewState`, call 1 of 2) | forge-neutral/18 |
| 25 | statusgen/autoflip.go:979 | statusgen (`--auto-flip-model`) | (a) | #1223 | `ReviewsAtHead`-shaped (`ReviewState`, call 2 of 2) | forge-neutral/18 |
| 70 | statusgen/autoflip.go:908 | statusgen (`--auto-flip-model`) | (a) | #1223 | `GetPullRequest`-shaped, body + `changed_files` (`ghModelFlipSource.PRShape`, call 1 of 2; added after the first sweep, #1691) | forge-neutral/18 |
| 71 | statusgen/autoflip.go:918 | statusgen (`--auto-flip-model`) | (a) | #1223 | `ListChangedFiles`-shaped, paginated (`ghModelFlipSource.PRShape`, call 2 of 2; added after the first sweep, #1691) | forge-neutral/18 |
| 26 | statusgen/selfimprovement.go:400 | statusgen (self-improvement digest) | (a) | #1223 | GAP — item detail fetch (`ghSelfImprovementDetailFetcher`) | forge-neutral/18 |
| 72 | statusgen/decisionruling.go:643 | statusgen (`--corroborate`, `rulingForgeClient`) | (a), (d) | #1223 | GAP — `gh auth token` is an identity read, not a forge operation (same shape as row 27): the third fallback after `GH_TOKEN` and `GITHUB_TOKEN`, and its result becomes the bearer token of statusgen's own in-process REST client (`newGHClient`, `statusgen/ghfetch.go`). Added after the first sweep, #1571 | forge-neutral/18 |

### B. `tools/desk/**` — the six sanctioned desk-verb `gh` exceptions (identity, not transport)

Already permitted in `tools/desk/internal/forgeban/allowlist.go`'s `AllowedInvocations` (ceiling 6). Per this
brief's own facts: these are a **separable, token-custody-gated follow-wave, not transport gaps** — routed here
as such, not to a v2 transport migration. The brief's facts name five; the sixth (row 73, `ambientLoginProbe`)
landed after the first sweep (#1528) and raised the ceiling from 5 to 6. That sixth permit now covers two `gh`
launches in one function, so it has two rows (73 and 78). `tools/desk/cmd/deskpushguard/main.go`'s
line keeps drifting from the allowlist's own (line-less) key: the allowlist keys carry no line number.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 27 | tools/desk/cmd/deskadvisory/advisory.go:183 | deskadvisory | (a), (d) | — | GAP — `gh auth token` reads the ambient CLI credential, i.e. IS the identity layer (D2 keeps identity deliberately outside `Forge`); no `Forge` method could replace it | token-custody follow-wave (unrouted in v2) |
| 28 | tools/desk/cmd/deskdigest/exec.go:47 | deskdigest | (a), (d) | — | mixed; also runs `issue list`, which has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 29 | tools/desk/cmd/deskdisposition/exec.go:30 | deskdisposition | (a), (d) | — | write-only now (`ListOpenChanges` already absorbed the read half, #1123); `label list` has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 30 | tools/desk/cmd/deskmerge/exec.go:116 | deskmerge | (a), (d) | — | `pr view` half maps to `GetPullRequest`; the merge-authority `gh api` read has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 31 | tools/desk/cmd/deskpushguard/main.go:441 | deskpushguard | (a) | — | GAP — branch→PR lookup by name; no enumerated op is keyed by branch (every read on the interface is keyed by number) | token-custody follow-wave (unrouted in v2) |
| 73 | tools/desk/internal/deskkit/preflight.go:1713 (`gh` resolved by `exec.LookPath` at :1707) | deskkit (`ambientLoginProbe`, the preflight ambient-identity check) | (a), (d) | — | GAP — runs `gh api user` under the AMBIENT environment on purpose, to learn which login a tool fall-through would silently act as. Both backends refuse to build a client without a minted token, so the seam can never observe the credential this check exists to catch; the allowlist's own reason says retiring it means deleting the check, not migrating it. Added after the first sweep, #1528 | token-custody follow-wave (unrouted in v2) |
| 78 | tools/desk/internal/deskkit/preflight.go:1733 (same `gh` path as row 73) | deskkit (`ambientLoginProbe`, stored-credential look) | (a), (d) | — | GAP — the identity-read exception again, same shape as rows 27 and 72: after `gh api user` answers "not logged in", the same function runs `gh auth token` (a local read, no network call) and only tests whether the answer is empty. It is the second `gh` launch under the one `ambientLoginProbe` permit, so the allowlist's ceiling stays 6 while `forgeban`'s unresolved-argv count goes from 31 to 32. Added after the first re-derivation (the allowlist reason for this permit now names both calls) | token-custody follow-wave (unrouted in v2) |

### C. `tools/desk/**` — other `gh`-subprocess / ambient-identity sites (not in the allowlist)

The bash cell launcher the first sweep cited (`tools/cellctl/cellctl`) is gone: `cellctl` is now the Go binary
`tools/desk/cmd/cellctl`, and the bash script survives only as a test oracle under `tools/cellctl/testdata/`
(excluded — test fixture, not shipped). The Go launcher still **generates** shell that shells `gh`: row 33 is
re-anchored onto that generated shim, and row 74 is the cell's `gh` wrapper the same file writes (#1631). Neither
is a Go `exec` of `gh`, so `forgeban` does not see either — the `gh` lives inside a Go string constant.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 32 | tools/desk/askassay/probe.go:360 (via `readOnlyBinaries["gh"]=true` at :46, dispatched at :138, exec at :361) | askassay (ask-pane) | (a) | — | GAP — two registry questions read issue counts through this probe; already recorded in `forgeban`'s `UnresolvedArgv` ledger as a could-not-check (argv[0] is a variable), not a permit; the allowlist header states plainly it "CAN launch `gh`" and that retiring it needs the answers re-sourced through the interface first | unrouted — "a brief of its own" per the allowlist's own header |
| 33 | tools/desk/cmd/cellctl/shims.go:34 (`gh_token="$(gh auth token …)"`, in the generated shim `shimTemplate`) | cellctl (generated shell) | (a), (d) | #1145 | GAP — `gh auth token` is the identity-read exception, same shape as row 27; the shim resolves the ambient credential before swapping `HOME` and hands it on as `CELLCTL_GH_AMBIENT` (not `GH_TOKEN`, since #1631), which is the #1145 fix itself, not the bug — but it is still a live `gh` subprocess outside the two backends and belongs in the count | desktools-v2/06 |
| 74 | tools/desk/cmd/cellctl/shims.go:67 (`exec gh "$@"`, in the generated `gh` wrapper `ghWrapTemplate`) | cellctl (generated shell) | (a), (d) | #1145 | GAP — the cell's `gh` wrapper, first on a shimmed verb's PATH, turns `CELLCTL_GH_AMBIENT` back into `GH_TOKEN` for a `gh` child with no token of its own: a token attached only to a child named `gh`, by design. Added after the first sweep (#1631) | desktools-v2/06 |

### C2. `tools/desk/cmd/deskdispatch` — #1146, status disputed with `desktools-v2/06`

`desktools-v2/06` (authored 2026-09-16, freshness-checked `e9fa19d3`) states #1146 is open and specifies an
unlanded test, `TestDispatchHandsTokenToScriptChild` (`git grep -l TestDispatchHandsTokenToScriptChild -- tools/desk`
finds nothing at `fabe4926e` — confirmed absent). But `resolveClaimAuth`
(tools/desk/cmd/deskdispatch/dispatch.go:1182, credential hand-off at :1225; its GitLab twin
`resolveClaimAuthGitLab` hands off at :1335) already threads the minted role token into the **full child
environment** (`append(os.Environ(), "GH_TOKEN="+tok)`) for the legacy claim script, not onto a child literally
named `gh` — which is the shape #1146 names. Its own doc comments attribute this fix to a *different* issue, #1151
("the claim tools read only the AMBIENT credential — bare `gh api` in the script, GH_TOKEN/--token-file in the
binary"). Reported as a discrepancy, not resolved either way here: it is possible #1151's fix subsumed #1146's
shape for the claim step specifically, or #1146 names a distinct exec site this sweep did not find (a dispatched
worker/reviewer session's own git-credential path was not audited to the same depth as the claim step).
`desktools-v2/06`'s own Verify row 5 is the authoritative test either way.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 33b | tools/desk/cmd/deskdispatch/dispatch.go:1225 | deskdispatch (`resolveClaimAuth`, legacy claim script) | (d) | #1146 (disputed — see note above; comment attributes this line to #1151) | N/A — token already threaded via full child env, not argv[0]-gated; `desktools-v2/06`'s own test decides whether this closes #1146 | desktools-v2/06 |

### D. `tools/desk/**` — hardcoded/misrouted query shape (shape c) — RESOLVED since the first sweep

`tools/desk/cmd/deskclose/authority.go` does **not** hardcode a `pullRequest`/`mergeRequest` GraphQL literal
anywhere (an earlier draft of `desktools-v2/04` said it did; that was wrong — the word `pullRequest` appears in
this file only in an explanatory comment, `authority.go:123`). The defect the first sweep recorded was narrower:
`fetchComment` (the untyped path) called `Forge.ListComments`, which on the GitHub backend selects a PR's comment
thread, so an authorizing comment on an **issue** silently came back empty. `desktools-v2/04` fixed it (#1320):
`fetchComment` (`authority.go:143`) now derives the kind from the permalink and reads through
`ListCommentsTyped` — "deskclose has no kind-less read left to fall back to". The two call sites below are kept
as RESOLVED rows only because Verify row 3 requires `#1019` present and this is where it lived.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 34 | tools/desk/cmd/deskclose/authority.go:261 | deskclose (`authorize`, the ruling gate) | (c)-adjacent — wrong-typed call, not a literal query outside the backend | #1019 | **RESOLVED** — `ListCommentsTyped`, reached through `fetchComment`'s kind derivation (#1320) | desktools-v2/04 (verified) |
| 35 | tools/desk/cmd/deskclose/authority.go:289 | deskclose (`authorizeManifest`) | (c)-adjacent — wrong-typed call, not a literal query outside the backend | #1019 | **RESOLVED** — as row 34 | desktools-v2/04 (verified) |

### E. `tools/desk/cmd/deskpost/github.go` — a second, hand-rolled GitHub REST+GraphQL client (largest undocumented finding)

`deskpost`'s verdict/comment/flip read+write path is **entirely GitHub-only**, through its own App-authenticated
`ghClient` in this one 1109-line file — never `deskkit.Forge`. This is self-documented in the tree
(`github.go:614-615`: "this binary's GitHub path runs its own hand-rolled REST client (this file), not
deskkit.Forge") and deliberate as a GitLab **fail-closed** (`requireGitHubForge`, #772) — but per this brief's own
definition, a GitHub fact outside the two backend files is a reach-around row regardless of intent, and **no
brief in `desktools-v2/02..11` currently names this file**. It mints its own token in-process
(`mintInstallationToken`, refuses-if-unminted, no ambient fallback — so it does **not** violate Principle 1's
custody rule) but every read/write below is a hand-built REST path or GraphQL string built outside
`forge_github.go`, largely duplicating operations the `Forge` interface already enumerates. Listed as its own
group because it is a structural duplicate of the seam, not a scattered leak. The set of request sites is
unchanged since the first sweep (17); every line moved by +15 or more.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 36 | tools/desk/cmd/deskpost/github.go:595 | deskpost (`ghClient.getPR`) | (c) | — | `GetPullRequest` | unrouted — new finding, no v2 brief names `tools/desk/cmd/deskpost/github.go` |
| 37 | tools/desk/cmd/deskpost/github.go:607 | deskpost (`ghClient.getIssue`/`getIssueTyped`) | (c) | — | `GetIssueTyped` | unrouted |
| 38 | tools/desk/cmd/deskpost/github.go:671 | deskpost (`ghClient.headCommitAuthor`) | (c) | — | `GetCommit` | unrouted |
| 39 | tools/desk/cmd/deskpost/github.go:683 | deskpost (`ghClient.listReviews`, paginated) | (c) | — | `ReviewsAtHead` | unrouted |
| 40 | tools/desk/cmd/deskpost/github.go:730 | deskpost (`ghClient.listLabelEvents`, paginated) | (c) | — | `ListLabelEvents` (already enumerated — exact duplicate) | unrouted |
| 41 | tools/desk/cmd/deskpost/github.go:762 | deskpost (`ghClient.listLabels`, paginated) | (c) | — | GAP — no enumerated op reads the present-labels set on one item (`ListLabels(repo)` is repo-wide label *definitions*, not per-item applied labels) | unrouted |
| 42 | tools/desk/cmd/deskpost/github.go:809 | deskpost (`ghClient.listFiles`, paginated) | (c) | — | `ListChangedFiles` | unrouted |
| 43 | tools/desk/cmd/deskpost/github.go:840 | deskpost (`ghClient.combinedStatusAt`, paginated) | (c) | — | `ChecksAtHead` | unrouted |
| 44 | tools/desk/cmd/deskpost/github.go:861 | deskpost (`ghClient.checkRunsAt`, paginated) | (c) | — | `ChecksAtHead` | unrouted |
| 45 | tools/desk/cmd/deskpost/github.go:880 | deskpost (`ghClient.postReview`) | (c) | — | `PostReview` | unrouted |
| 46 | tools/desk/cmd/deskpost/github.go:887 | deskpost (`ghClient.postComment`) | (c) | — | `PostComment`/`PostCommentTyped` | unrouted |
| 47 | tools/desk/cmd/deskpost/github.go:911 | deskpost (`ghClient.getRepoFile`) | (c) | — | `ReadFile` | unrouted |
| 48 | tools/desk/cmd/deskpost/github.go:941 | deskpost (`ghClient.listCommentBodies`) | (c) | — | `ListComments`/`ListCommentsTyped` | unrouted |
| 49 | tools/desk/cmd/deskpost/github.go:962 | deskpost (`ghClient.fetchPRTrustPayload`, raw POST `/graphql` using `deskkit.PRTrustQuery`) | (c) | — | `PRTrustEvents` (the query constant is already shared/centralized in `tools/desk/internal/deskkit/trustfetch.go` — only the transport differs) | unrouted |
| 50 | tools/desk/cmd/deskpost/github.go:979 | deskpost (`ghClient.fetchIssueTrustPayload`, raw POST `/graphql` using `deskkit.IssueTrustQuery`) | (c) | — | `IssueTrustEvents` | unrouted |
| 51 | tools/desk/cmd/deskpost/github.go:1071 | deskpost (`ghClient.markReadyForReview`, raw GraphQL mutation) | (c) | — | `MarkReadyForReview` (already enumerated — exact duplicate) | unrouted |
| 52 | tools/desk/cmd/deskpost/github.go:1097 | deskpost (`ghClient.RepoVisibility`) | (c) | — | `RepoVisibility` (already enumerated, same name, different transport) | unrouted |

`tools/desk/cmd/deskpost/github.go:1083,1087` (`readMergeHold`/`setMergeHold`) are **not** rows: both are unconditional
GitHub-typed-not-applicable stubs that issue no request. `tools/desk/cmd/deskboard/board.go` and `tools/desk/cmd/issueboard/board.go`'s own
`PRTrustQuery`/`IssueTrustQuery` consumers (rows their code comments still describe as `gh api graphql`) have
**already migrated** onto `f.PRTrustEvents`/`f.IssueTrustEvents` — `deskboard/board.go:630` (`prBlessed`) and
`tools/desk/cmd/issueboard/board.go`'s `fetchIssueBlessed` (its stale comment now at `:264`) call the typed `Forge` methods
directly; the comments are stale, the code is not. Not rows; not reach-arounds.

### E2. `tools/desk/cmd/**` — other hand-built GitHub REST calls added after the first sweep (shape c)

Two verbs that landed after the first sweep each carry a small GitHub-only REST client of their own, outside
the two backend files. Neither reads an ambient credential: `deskinbox` mints a role token
(`tools/desk/cmd/deskinbox/detail.go:67`), and `deskfleet` reads the token from an operator-named `--token-file`
(`tools/desk/cmd/deskfleet/labels.go:157`). But each
request is a GitHub path built outside `forge_github.go`, which is a reach-around under this brief's definition.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 76 | tools/desk/cmd/deskinbox/detail.go:84 (`listIssueComments`: the `%s/repos/%s/%s/issues/%d/comments` path on a GitHub base URL, `Accept: application/vnd.github+json` at :91) | deskinbox (`--walk`/`--html` detail fetch) | (c) | — | `ListCommentsTyped` with the issue kind. The file header says no typed op returns comment bodies, but `deskkit.Comment` carries `Body` (`tools/desk/internal/deskkit/forge.go:538`) and `ListCommentsTyped` (`forge.go:1510`) reads an issue's own thread. A non-GitHub repo gets could-not-check here today. Added after the first sweep, #1507 | unrouted — no v2 brief names `tools/desk/cmd/deskinbox` |
| 77 | tools/desk/cmd/deskfleet/labels.go:64 (`githubLabels.create`: `POST /repos/{owner}/{repo}/labels` on deskfleet's own client, GitHub base `deskkit.GitHubAPIBase` at `tools/desk/cmd/deskfleet/main.go:121` (a literal `https://api.github.com` until #1864), `Accept` header at `tools/desk/cmd/deskfleet/client.go:43`) | deskfleet (fleet label definitions) | (c) | — | GAP — creating a label definition on its own. The GitHub backend creates missing definitions only as step 1 of `ApplyLabels` (`forge_github.go:1808`), which also applies them to an item; no op creates a definition without an item. The same file's `gitlabLabels.create` (:33) is the GitLab arm. Added after the first sweep | unrouted — no v2 brief names `tools/desk/cmd/deskfleet` |

### F. `tools/desk/**` — hardcoded remote name (shape b)

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 53 | ~~tools/desk/cmd/deskpushguard/main.go~~ (was ~line 409/432) | deskpushguard | (b) | #1201 | **RESOLVED** — fixed by commit `539a3735c` (PR #1271, "Fix deskpushguard hardcoding remote name \"origin\"", already on `origin/main`), *before* the first sweep ran. `checkForeignCommits`/`resolveRemoteMain` now take the actual push-target remote name from the pre-push hook's own `args[0]`, falling back to `"origin"` only when that argument is absent (`main.go:102,377` are the now-legitimate fallback lines, not the bug). Kept as a row only because Verify row 3 requires `#1201` present and this is where it lived. | desktools-v2/05 (closed already; brief narrows to #884) |
| 54 | tools/desk/cmd/deskpushguard/main.go (no `insteadOf`/`pushInsteadOf` expansion anywhere in this package — `grep -n insteadOf tools/desk/cmd/deskpushguard/*.go` returns nothing) | deskpushguard | (b), (d) | #884 | GAP — the push-transport guard parses the raw remote URL git hands it and never expands `url.<base>.insteadOf`/`pushInsteadOf`, so a rewritten URL evades the remote/repo derivation this file does (contrast `tools/desk/cmd/deskgit/deskgit.go`, which explicitly expands `insteadOf` for the same class of gap, and `tools/desk/internal/gitcore/gitcore.go`, which has none *because* it accepts no remote alias at all) | desktools-v2/05 |
| 55 | tools/desk/internal/deskkit/forgeresolve.go:87 (`originRemoteHost`, consulted at :152) | deskkit (`resolveForgeKind`) | (b) | — | GAP-adjacent — a documented, tested **fallback of last resort**: the roster (`ASSAY_REPO_FORGES`) is always consulted first (`TestForgeForResolvesFromRepoConfig` panics if `originRemoteHost` is reached while the roster can answer), so this is lower-severity than #1201's class (no unconditional misattribution), but it is still a hardcoded `"origin"` read used to decide which *forge* answers a caller with no roster entry, for a repo assumed to be the CWD's own checkout | unrouted — no v2 brief targets general forge-kind resolution |

Excluded from this table by design, not by oversight: `tools/desk/**` has ~30 further `"origin"` literals
(`tools/desk/cmd/deskflip/flip.go`, `deskmerge/{merge,currency}.go`, `tools/desk/cmd/deskdispatch/dispatch.go`, `tools/desk/cmd/deskclaim-ref/gogit.go`,
`deskpr/{exec,deskpr}.go`, `deskwt/{deskwt,exec,roleinit}.go`, `verifyloop/{durable,preflight}.go`,
`tools/desk/cmd/deskboot/boot.go`, `scanloop/{plan,lane}.go`, `tools/desk/cmd/deskreply/deskreply.go`, `tools/desk/cmd/deskroster/preflight.go`,
`deskkit/{pushtransport,preflight}.go`). Every one of those is ordinary git-transport plumbing — "push/fetch this
worktree's own configured `origin` remote" — not a forge-identity assumption: none of them decides *which forge
or which repo* a Forge op targets, which is what shape (b) is about. That class is git-transport, not a
Forge-seam bypass, and sits with `desktools-go-git` (spec.md §4), not this stream; folding all ~30 in here would
dilute the count the ban-lint (`desktools-v2/02`) needs to be meaningful.

### G. `plugins/assay/scripts/*.sh` — human-facing read-only monitors (shape a)

Read-only shell tools intended to run under an **operator's own ambient `gh` login** on their desktop, not desk
automation — each shells `gh` for one read, documented as such in its own header comment. Still a
GitHub-specific subprocess outside the two backends per this brief's definition, and named explicitly by facts'
search-surface list. Since the first sweep `inbound-monitor.sh` gained a per-owner token: when one is named for
the repo it runs the read with that token attached to the `gh` child only (row 75), else it falls back to the
ambient login (row 58).

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 56 | plugins/assay/scripts/assay-inbox.sh:511 | assay-inbox (skill script, `gh issue list`) | (a) | — | GAP — a human-run desktop tool, not a desk-tool binary; no Forge equivalent is reachable from shell without a Go/CLI client | unrouted |
| 57 | plugins/assay/scripts/assay-inbox.sh:860 | assay-inbox (skill script, `gh issue view`, `--walk`/`--html` only) | (a) | — | GAP (same as above) | unrouted |
| 58 | plugins/assay/scripts/inbound-monitor.sh:354 | inbound-monitor (skill script, `gh issue list`, ambient login) | (a) | — | GAP (same as above) | unrouted |
| 75 | plugins/assay/scripts/inbound-monitor.sh:348 | inbound-monitor (skill script, `GH_TOKEN="$_rtok" gh issue list`, per-owner token) | (a), (d) | — | GAP (same as above); the token is attached to the one `gh` child, never exported. Added after the first sweep | unrouted |
| 59 | plugins/assay/scripts/pr-monitor.sh:252 | pr-monitor (skill script, `gh pr list`) | (a) | — | GAP (same as above) | unrouted |

No executable `gh`-shelling shell code was found under `.claude/`; its skill-body `.md` files that mention `gh`
do so in prose (rules, worked examples, "never shell `gh` directly"), not as an executed script this checker can
count as a site.

### H. `.github/workflows/**` — CI's own `gh` calls (shape a, different execution boundary)

GitHub Actions steps, not desk-tool processes: each runs under the workflow's own `GITHUB_TOKEN`/App token in
CI, with no Go process and nothing that could import `deskkit` even in principle — the `Forge` seam is a Go
abstraction inside `tools/desk`, and these are standalone `bash:` step bodies in a separate execution context
entirely. Included because facts names `.github/workflows/**` as a search surface; each is still a
GitHub-specific fact (a `gh` invocation) that no desk verb mediates. Kept as its own group with a structural
note rather than routed into a Go migration, since there is nothing in `tools/desk/**` for these to migrate
*onto* — a CI step cannot shell out to a Go binary's internal interface. Unchanged since the first sweep.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 60 | .github/workflows/changelog-check.yml:144 | CI (`changelog-check.yml`) | (a) | — | N/A — CI step, no Go seam to reach | unrouted (out of the Go seam's reach by construction) |
| 61 | .github/workflows/evidence-automerge.yml:156 | CI (`evidence-automerge.yml`) | (a) | — | N/A | unrouted |
| 62 | .github/workflows/evidence-automerge.yml:265 | CI (`evidence-automerge.yml`, `gh api graphql`) | (a) | — | N/A | unrouted |
| 63 | .github/workflows/evidence-automerge.yml:278 | CI (`evidence-automerge.yml`, `gh pr view`) | (a) | — | N/A | unrouted |
| 64 | .github/workflows/verify-gate-open.yml:105 | CI (`verify-gate-open.yml`, `gh issue list`) | (a) | — | N/A | unrouted |
| 65 | .github/workflows/verify-gate-open.yml:135 | CI (`verify-gate-open.yml`, `gh issue create`) | (a) | — | N/A | unrouted |
| 66 | .github/workflows/verify-gate-close.yml:132 | CI (`verify-gate-close.yml`, `gh issue reopen`) | (a) | — | N/A | unrouted |
| 67 | .github/workflows/verify-gate-close.yml:133 | CI (`verify-gate-close.yml`, `gh issue comment`) | (a) | — | N/A | unrouted |
| 68 | .github/workflows/verify-gate-close.yml:212 | CI (`verify-gate-close.yml`, `gh issue comment`) | (a) | — | N/A | unrouted |
| 69 | .github/workflows/verify-gate-close.yml:236 | CI (`verify-gate-close.yml`, `gh issue comment`) | (a) | — | N/A | unrouted |

(`verify-gate-close.yml:298,299,306` are three more `gh issue reopen`/`gh issue comment` calls in the same
retry-branch shape as :132-133 above; grouped rather than triple-counted as separate rows since they are the same
step's retry path, not a distinct call site. `release.yml`'s two comments about `gh pr view` are historical —
no live `gh` call remains in that file; every workflow's `echo "gh present: $(gh --version …)"` preflight line
is a presence probe, not a forge operation, and is excluded the same way `forgeban`'s `UnresolvedArgv` excludes
presence probes.)

### Not classified by this re-derivation — shape (c) candidates (could-not-check)

The first sweep recorded no command for its shape (c) search. This re-derivation declares one (under **How this
inventory is derived**, groups E and E2), and every line it prints at `fabe4926e` is accounted for here or in a
row. The sites added after the first sweep (`deskinbox`, `deskfleet`) were read and are rows 76 and 77. The sites
below that were present at the first sweep but not listed then are **could-not-check**, not rows and not
cleared: whether each is a reach-around under this brief's definition needs the per-site reading groups E and E2
got. They are follow-ups for a later pass, not part of this re-derivation's drift fix.

- `tools/desk/cmd/deskadvisory/advisory.go:196` — `ghAPI`, a REST client on `githubAPIBase` (`Accept` header at
  :201) that reads `repos/…` paths at :234, :389, :423 and :435. It takes its credential from an inherited
  `GH_TOKEN` or `GITHUB_TOKEN` (:177, :180) before the `gh auth token` fallback that row 27 covers, so it also
  carries shape (d).
- `tools/desk/cmd/deskrelease/github.go:214` and `:246` — `getRef` / `createTagRef`, hand-built `git/ref` reads
  and `git/refs` writes on `deskrelease`'s own client (`Accept` header at :166).
- `tools/desk/internal/deskkit/compositionsource.go:320` — `LatestReleaseTag`, a `releases/latest` read of the
  release home.
- `tools/desk/internal/deskkit/repovis.go:163` (`HTTPRepoInfoFetcher.RepoVisibility`, `Accept` header at :169)
  and `tools/desk/internal/deskkit/trustliveness.go:110` (`HTTPAccountFetcher.GetAccount`, `Accept` header at
  :115) — deskkit-local HTTP fetchers outside `forge_github.go`, the same situation as `tokenidentity.go:57`
  below.

The remaining lines the sweep prints are not candidates, for the reason given with each:

- `tools/desk/cmd/deskdigest/collect.go:212,213,272,273` and `tools/desk/cmd/deskmerge/authority.go:69,70,86` —
  REST paths and headers handed to `gh api`, so already inside rows 28 and 30 as shape (a). (`authority.go:86`
  builds the expected item path to compare against a comment's own URL; it sends nothing.)
- `tools/desk/cmd/deskinbox/detail.go:84,91`, `tools/desk/cmd/deskfleet/labels.go:64` and
  `tools/desk/cmd/deskfleet/client.go:43` — rows 76 and 77. (`deskfleet/main.go:121` printed here too until
  #1864 replaced its literal base with `deskkit.GitHubAPIBase`; it is still cited in row 77 as the base.)
- `tools/desk/cmd/desktoken/coverage.go:263,301` and `tools/desk/cmd/desktoken/desktoken.go:208,381` — the
  `Accept` headers of `desktoken`'s App-JWT installation lookups and token exchange. That is the token minter,
  the identity layer D2 keeps outside `Forge` on purpose, not a forge operation a verb could route through it.
- `tools/desk/internal/deskkit/forge.go:37` (the `GitHubAPIBase` constant) and
  `tools/desk/internal/deskkit/forgegit.go:125` (host mapping for the git transport) — seam-side definitions,
  not requests.
- `tools/desk/internal/deskkit/tokenidentity.go:57` — a `viewer` GraphQL read, but it is a `GitHubForge` method
  in a backend-side file outside `forge_github.go`; recorded here, not as a reach-around.

### Not swept exhaustively — shapes (b) and (d) (could-not-check)

Shapes (b) and (d) have no declared sweep command. Their rows (53–55, and the (d) cells in groups B, C and G) are
the sites found by reading. These further (d) sites were found by a reviewer's spot check and are present at
the first sweep; they are could-not-check here, not cleared:

- `tools/desk/cmd/deskclaim-ref/gogit.go:595` — reads an inherited `GH_TOKEN`, `GITHUB_TOKEN` or `GITLAB_TOKEN`
  when no `--token-file` is passed.
- `tools/desk/cmd/cellctl/cell.go:498` — `ghConfigRelPath`, the operator's `gh` config directory that the cell
  launcher links into the cell's `HOME`.
- `tools/desk/cmd/deskadvisory/advisory.go:177,180` — the inherited-token reads named with `ghAPI` above.

## Reconciled:

Cross-check against `tools/desk/internal/forgeban/allowlist.go` (`const allowedInvocationCeiling = 6` at
`fabe4926e`; it was 5 at the first sweep), so no Go call site is double-counted or dropped between the two
registers. `TestNoForgeCLIShellout` passes at `fabe4926e`: 6 permitted call sites, 32 unresolved-argv exec sites.

- **`AllowedInvocations` (6 rows) → inventory rows 27–31 and 73 (with 78).** All six map cleanly by
  `<file>::<enclosing decl>::gh` key. The sixth, `internal/deskkit/preflight.go::ambientLoginProbe::gh`, landed
  after the first sweep (#1528) and is inventory row 73. Its function now launches `gh` twice (`gh api user`,
  then `gh auth token`), and the key has no line, so the one permit covers both: rows 73 and 78. Line numbers drift freely (allowlist keys carry no line
  number, only `file::func::bin`, so the drift is invisible to the allowlist itself — it only shows up here
  because this inventory cites concrete lines).
- **`UnresolvedArgv` (29 rows, 32 exec sites) → inventory rows 32, 73 and 78, the rest excluded with reasons
  already stated in the register itself.** `askassay/probe.go::execRead::<unresolved>` is the one row that the
  register's own text says "CAN launch `gh`" — inventory row 32. `internal/deskkit/preflight.go::ambientLoginProbe::<unresolved>`
  covers the two `exec.CommandContext` calls on the `gh` path `ambientLoginProbe` resolved by `exec.LookPath` —
  the same two sites as rows 73 and 78, not further ones. That register row now counts two exec sites, which is
  why the total went from 31 to 32 with no new register row. The other 27 `UnresolvedArgv` entries are excluded per the register's own
  stated reasons: each resolves to a non-forge binary that the checker cannot statically prove but that carries
  no forge path on any reachable branch. At the first sweep they were `tools/desk/cmd/clusterguard/shim.go`,
  `deskadvisory/advisory.go::runChecks`, five `deskboard/*.go` rows, `deskpreflight`,
  `tools/desk/cmd/verifyloop/durable.go`, `tools/desk/internal/acp/client.go`, `tools/desk/internal/deskkit/callout.go`,
  `internal/deskkit/preflight.go::coldMintProbe`, `tools/desk/internal/deskkit/riskcallout.go`,
  `internal/deskkit/untrustscan.go::runSemgrep`, `cmd/desksupervise/live.go::showClaim`, and the two
  `tools/desk/internal/deskkit/migrate.go` statusgen-binary rows (a cluster CLI, `desktoken`, `statusgen`,
  Semgrep, an agent-protocol server, an operator callout, or a consumer repo's own `dispatch-claim.sh`). Added
  since: the eight `cmd/cellctl/*.go` rows of the Go cell launcher (the operator's harness, a terminal
  multiplexer, the cockpit CLIs it probes by name, the cell's own binaries), `cmd/deskinbox/flow.go::runReaderFn`
  and `cmd/deskrelease/github.go::resolveDeskTokenPath` (`desktoken`). None is an inventory row.
- **Inventory rows with no allowlist entry at all:** every row in groups A (statusgen — the allowlist counts
  `tools/desk/**` Go call sites only, never `statusgen/**`), C (rows 33 and 74, shell generated from a Go string
  constant, which `forgeban`'s AST checker does not read as an exec), D (rows 34–35, not a
  `gh`-subprocess shape at all), E (rows 36–52, `tools/desk/cmd/deskpost/github.go` — an HTTP client, not a `gh` subprocess, so
  `forgeban`'s AST checker — which greps for `exec.Command`/`gh` argv — cannot and does not see it), F (rows
  53–55, hardcoded-remote shape, not a `gh`-subprocess shape), E2 (rows 76–77, HTTP clients, for the same reason as E), G (rows 56–59 and 75, shell scripts outside
  `tools/desk/**`), and H (rows 60–69, YAML, not Go). `forgeban` is scoped to `tools/desk/**` Go `exec.Command`
  call sites (`const allowedInvocationCeiling`'s own package, `tools/desk/internal/forgeban`); the other
  groups are exactly the "shell, skills, and **statusgen** sites it does not cover" this brief's facts describe.

## Outward writes

One row per text-carrying `Forge` write call site and per push-path text surface, with the checks it runs today.
Seeded from `docs/streams/desktools-v2/spec.md` §8.2 and re-verified line by line against `origin/main` @
`fabe4926e` for this inventory (`desktools-v2/10` ticks against this table). Every citation below was read
directly, not copied — `deskpr edit`'s row corrects `spec.md`'s bare `edit.go:NNN` citations to their real
package path, `tools/desk/cmd/deskpr/edit.go` (there is no deskpost/edit.go; `EditChange`'s only definition is
in `deskpr`). The write-site set was re-derived with
`git grep -nE '\.(FileIssue|PostComment|PostCommentTyped|EditComment|CreateDraftChange|EditChange|PostReview|WriteFile|ApplyLabels)\(' -- 'tools/desk/cmd/*.go' ':!*_test.go' | grep -v 'os\.WriteFile'`;
the text-carrying writes it found that the first table lacked (`deskpr edit`'s review notice,
`deskautolane`, `deskrestamp`, and `deskevidence --outcome-record`'s record file and draft change) are added
below, and the label-write row lists every `ApplyLabels` caller.

| Verb (write) | Surface | Secret scan | Self-contained + withheld (public targets) | Override offered |
|---|---|---|---|---|
| `deskfile new` (`FileIssue`, tools/desk/cmd/deskfile/deskfile.go:860) | issue title + body | yes (deskfile.go:724,727) | **no** | no (no scan override; `--force-new` bypasses the dedupe search and the blocker-evidence gate (flag at deskfile.go:593, gates at :738 and :754), and `--force-file` raises the rate gate; neither is a text scan) |
| `deskfile attach` (`PostCommentTyped`, tools/desk/cmd/deskfile/deskfile.go:1011) | comment body | yes (:979) | **no** | no |
| `deskpr create` (`CreateDraftChange`, tools/desk/cmd/deskpr/deskpr.go:378) | PR title + body | yes (deskpr.go:188,792) | yes (deskpr.go:274) | yes |
| `deskpr create`/`update` (the push) | branch name | yes (deskpr.go:796) | **no** | yes |
| `deskpr create`/`update` (the push) | branch diff | secret arms + ruling-claim guard only (deskpr.go:834,838) | **no** | yes |
| `deskpr create`/`update` (the push) | commit messages | **no** | **no** | — |
| `deskpr edit` (`EditChange`, tools/desk/cmd/deskpr/edit.go:302) | PR title + body | yes (edit.go:107,114) | yes (edit.go:230) | yes |
| `deskpr edit` (`PostComment`, tools/desk/cmd/deskpr/edit.go:312) | tool-composed re-review notice | **no** | **no** | — |
| `deskreply` reply and `--workpad` (tools/desk/cmd/deskreply/deskreply.go:331, workpad.go:100,243) | reply body | yes (deskreply.go:162) | yes (deskreply.go:172) | yes |
| `deskpost comment` (tools/desk/cmd/deskpost/comment.go:186,189) | comment body | yes (comment.go:54) | yes (comment.go:64) | no |
| `deskpost review` (tools/desk/cmd/deskpost/review.go, forgeclient.go:293) | review body | yes (review.go:128) | yes (review.go:144) | no |
| `deskevidence` (`WriteFile`, tools/desk/cmd/deskevidence/deskevidence.go:561,630) | file content (added lines) | yes (deskevidence.go:420) | **no** | no |
| `deskevidence` (`CreateDraftChange`, :651) | tool-composed PR title + body | **no** | **no** | — |
| `deskevidence --outcome-record` (`WriteFile`, tools/desk/cmd/deskevidence/outcomerecord.go:173,208) | verify-outcome record file (whole file, new path) | yes (outcomerecord.go:112) | **no** | no |
| `deskevidence --outcome-record` (`CreateDraftChange`, outcomerecord.go:224) | tool-composed PR title + body | **no** | **no** | — |
| `deskclose` (`PostCommentTyped`, tools/desk/cmd/deskclose/github.go:189) | tool-composed pre-close comment (can name a cross-repository canonical target) | **no** | **no** | — |
| `deskprovenance` (tools/desk/cmd/deskprovenance/main.go:238,241) | tool-composed comment | **no** | **no** | — |
| `deskautolane` (`PostComment`, tools/desk/cmd/deskautolane/lane.go:562) | tool-composed eject comment | **no** | **no** | — |
| `deskrestamp` (`PostCommentTyped`, tools/desk/cmd/deskrestamp/restamp.go:329) | tool-composed re-stamp record (names the original and re-stamping actors) | **no** | **no** | — |
| `desklabel`, `deskpost label`, `deskflip`, `deskdispatch`, `deskfile new`, `deskclose superseded`, `deskpathguard`, `deskautolane`, `deskpr` (decided), `deskrestamp` (`ApplyLabels`) | label names | **no** | **no** | — |
| `deskpushguard` (pre-push hook) | commit messages, ref names, diff | **no** — guards lineage and merged-branch pushes; scans no text | **no** | — |
| **`tools/desk/cmd/deskpost/github.go`'s own `ghClient` writes** (rows 45–46, 51 above: `postReview`, `postComment`, `markReadyForReview`) | review body, comment body, ready-flip mutation | **outside the check inventory entirely** — these never construct a `Forge`, so `ResolveForge`'s wrap (§8.3) cannot reach them even after `desktools-v2/10` lands, until rows 36–52 are migrated onto the seam | **no** | no |

Access-pattern (N+1) candidates for `desktools-v2/09`, seeded here rather than re-derived there: a board sweep
that calls `PRTrustEvents`/`ReviewsAtHead`/`ChecksAtHead` once per PR (`tools/desk/cmd/deskboard/board.go`'s `prBlessed` and its
CI-rollup read) and once per issue (`tools/desk/cmd/issueboard/board.go`'s `fetchIssueBlessed`) is exactly the N+1 shape
Principle 3 targets — each already routes through a typed `Forge` op (not a reach-around; the two migrated
consumers noted in group E), so `desktools-v2/09`'s work is adding a *snapshot* operation (one review-queue /
board-sweep GraphQL read) these can fold onto, not fixing a bypass. That operation has since landed (#1851):
`deskboard`'s open-PR read now calls `ReviewQueueSnapshot` (`tools/desk/cmd/deskboard/board.go:282`), which
carries each open change's reviews in the same round-trip. `prBlessed` and `fetchIssueBlessed` still read trust
events once per untrusted item.

## GAP summary (interface additions a future brief may need)

Recorded, not resolved here, per this brief's own scope: a per-item "present labels" read (row 41; `ApplyLabels`
writes but nothing reads the current set through the seam), a standalone label-definition create (row 77;
today definitions are created only as a side effect of `ApplyLabels`), a branch→change lookup (row 31; every read today is
keyed by PR/issue number, never a branch name), and the statusgen-side envelope gaps in group A's "GAP" cells
(author/comment-permalink resolution, all-state PR/issue listings, merged-PRs-since queries, and the ambient
token read feeding statusgen's own REST client, row 72) — all of these are `deskread`-envelope questions for
`forge-neutral/18`, not `Forge`-interface questions for this stream, since statusgen never calls `Forge`
directly (Principle 2).
