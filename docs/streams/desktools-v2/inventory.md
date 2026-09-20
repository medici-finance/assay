# desktools-v2/01 — reach-around inventory

Frozen, file:line-accurate inventory of every site that reaches past `tools/desk/internal/deskkit/forge.go`'s
`Forge` interface with a GitHub-specific fact, or with an ambient credential, across `statusgen/**`,
`tools/desk/**`, `tools/cellctl/**`, `plugins/assay/`/`.claude/` skill bodies, and `.github/workflows/**`.
Produced by a full re-sweep of the tree at the commit below — **not** copied from `spec.md`'s freshness note or
this brief's own frontmatter, both of which are already stale in places the sweep below calls out explicitly
(clause: verify before applying a correction).

- Tree state: `origin/main` @ `951ca784d` (2026-09-18), i.e. **after** the `spec.md` §1 freshness base (`57509073`,
  2026-09-17) and after the `desktools-v2` frontmatter's own freshness base (`e9fa19d3`, 2026-09-16). Several rows
  below record a site the older documents call "open" that the tree has since closed, and vice versa; each such
  case is called out under the row rather than silently reconciled away.
- Shapes: **(a)** a `gh` subprocess (`exec.Command("gh"`, a shim, or a script that shells `gh`); **(b)** a
  hardcoded remote name (`"origin"`); **(c)** a hardcoded query shape (a `pullRequest`/`mergeRequest` GraphQL
  block or a REST path fragment) built outside `forge_github.go`/`forge_gitlab.go`; **(d)** a token/identity
  assumption (an inherited `GH_TOKEN`, a `HOME` override, a token attached only to a child named `gh`).
- Counts are the run's own, not asserted from memory: **69 reach-around sites** total — **26 in `statusgen/**`**
  (forge-neutral/18's territory) and **43 in `tools/desk/**` + `tools/cellctl/**` + `plugins/assay/**` +
  `.github/workflows/**`** (desk's own territory, this stream's).

## Reach-around sites

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|

### A. `statusgen/**` — owned by `forge-neutral/18`, not by this stream (26 sites, 15 files)

Every non-test `statusgen/**` file shelling `gh` (`grep -rl 'exec.Command("gh"' statusgen --include='*.go'`,
tests excluded), one row per `exec.Command("gh", …)` call. statusgen is a separate Go module that does not
import `deskkit` **by design** (`statusgen/forgeread.go` header) — it reaches the seam by *running* the
`deskread` verb and parsing its JSON envelope, never by linking `deskkit`. So the "seam op" column here names
what `deskread`'s envelope would need to carry, not a `Forge` method statusgen would call directly; none of
these rows proposes a client, a library, or a port for statusgen (out of scope, per this brief's own facts).
`defaultScanIssueLister` (row 20) has **already** migrated onto `deskreadIssueLister` since #1223 — it is listed
here only because `ghIssueLister`, the pre-migration implementation it superseded, is still live code reachable
from two *other* statusgen modes (rows 20's own function is dead on the `--scan-issues` path but still called by
`--transcribe-scan`/`--transcribe-verdict`; see its row).

| # | file:line | tool/skill | shape | issue | seam op / envelope field (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 1 | statusgen/transcribescan.go:72 | statusgen (`--transcribe-scan`) | (a) | #1223 | GAP — author-identity resolution (`ghAuthorResolver`) | forge-neutral/18 |
| 2 | statusgen/transcribescan.go:104 | statusgen (`--transcribe-scan`) | (a) | #1223 | GAP — comment-permalink resolution (`ghCommentResolver`) | forge-neutral/18 |
| 3 | statusgen/doratiming.go:631 | statusgen (`--dora`) | (a) | #1223 | GAP — ambient-repo fallback #3 of 3 (`doraTargetRepo`; #1/#2 are `$GITHUB_REPOSITORY` and `git remote get-url origin`, neither of which shells `gh`) | forge-neutral/18 |
| 4 | statusgen/issues.go:577 | statusgen (`--issues`/`--dora`) | (a) | #1223 | GAP — all-state issue metrics list (`ghIssueMetricLister`; `ListOpenIssues` covers only `state=open`) | forge-neutral/18 |
| 5 | statusgen/decisiongateanchor.go:229 | statusgen (decision-gate anchor) | (a) | #1223 | `GetIssueTyped`-shaped (`fetchDecisionGateIssue`) | forge-neutral/18 |
| 6 | statusgen/claimdecay.go:63 | statusgen (claim-decay reader) | (a) | #1223 | GAP — all-state PR list with `headRefName`/`isCrossRepository` fields (`ghPRListJSON`; `ListOpenChanges` is open-only) | forge-neutral/18 |
| 7 | statusgen/briefflowreview.go:72 | statusgen (brief-flow review) | (a) | #1223 | GAP — merged-PRs-since query (`ghBFPRSource.MergedPRs`) | forge-neutral/18 |
| 8 | statusgen/briefflowreview.go:103 | statusgen (brief-flow review) | (a) | #1223 | `ReviewsAtHead`-shaped (`ghBFPRSource.Reviews`) | forge-neutral/18 |
| 9 | statusgen/transcribeverdict.go:500 | statusgen (`--transcribe-verdict`) | (a) | #1223 | `GetIssueTyped`-shaped (`ghVerdictIssueResolver`) | forge-neutral/18 |
| 10 | statusgen/transcribeverdict.go:573 | statusgen (`--transcribe-verdict`) | (a) | #1223 | `ChecksAtHead`-shaped (`ghVerdictMainHealth`) | forge-neutral/18 |
| 11 | statusgen/trustgate.go:204 | statusgen (trust gate) | (a) | #1223, #628 (**on the `scanloop` path**) | `IssueTrustEvents`-shaped (`ghIssueBlessChecker`) | forge-neutral/18 |
| 12 | statusgen/corroborate.go:712 | statusgen (corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`ghPRBaseRef`) | forge-neutral/18 |
| 13 | statusgen/corroborate.go:981 | statusgen (corroboration) | (a) | #1223 | `ChangeDiff`-shaped (`fetchPRDiff`) | forge-neutral/18 |
| 14 | statusgen/corroborate.go:1009 | statusgen (corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`fetchPRData`) | forge-neutral/18 |
| 15 | statusgen/citationcorroborate.go:437 | statusgen (citation corroboration) | (a) | #1223 | `GetPullRequest`-shaped (`fetchCitedArtifact`, PR form) | forge-neutral/18 |
| 16 | statusgen/citationcorroborate.go:463 | statusgen (citation corroboration) | (a) | #1223 | `GetIssueTyped`-shaped (`fetchCitedArtifact`, issue form) | forge-neutral/18 |
| 17 | statusgen/briefdecision.go:41 | statusgen (decision-queue source) | (a) | #1223 | `SearchIssues`-shaped (`ghDecisionQueueSource.Issues`) | forge-neutral/18 |
| 18 | statusgen/autonomy.go:451 | statusgen (`--autonomy`) | (a) | #1223 | GAP — merged-PR authors, all-state (`autonomyMergedAuthors`) | forge-neutral/18 |
| 19 | statusgen/autonomy.go:479 | statusgen (`--autonomy`) | (a) | #1223 | GAP — merged-PR gate rollups, all-state (`autonomyGates`) | forge-neutral/18 |
| 20 | statusgen/scanissues.go:114 | statusgen (`ghIssueLister`, legacy) | (a) | #1223 | superseded by `deskreadIssueLister` **on the `--scan-issues` path only**; still the live implementation for `--transcribe-scan`/`--transcribe-verdict` (rows 1–2, 9–10 call sites reuse it via `ghIssueLister` at `main.go:1777,1784`) — GAP until those two modes migrate too | forge-neutral/18 |
| 21 | statusgen/scanissues.go:930 | statusgen (`issueCommentLister`) | (a) | #1223, #628 (**on the `scanloop` path**) | GAP — per-issue comment list, `--paginate` (deskread's `OpenIssues` envelope carries no comments) | forge-neutral/18 |
| 22 | statusgen/autoflip.go:535 | statusgen (`--auto-flip-model`) | (a) | #1223 | GAP — merged-PR-for-commit lookup (`ghModelFlipSource.MergedPRForCommit`) | forge-neutral/18 |
| 23 | statusgen/autoflip.go:615 | statusgen (`--auto-flip-model`) | (a) | #1223 | GAP — PR branch commit list (`prBranchCommits`) | forge-neutral/18 |
| 24 | statusgen/autoflip.go:656 | statusgen (`--auto-flip-model`) | (a) | #1223 | `GetPullRequest`-shaped, head/state/mergedAt (`ReviewState`, call 1 of 2) | forge-neutral/18 |
| 25 | statusgen/autoflip.go:666 | statusgen (`--auto-flip-model`) | (a) | #1223 | `ReviewsAtHead`-shaped (`ReviewState`, call 2 of 2) | forge-neutral/18 |
| 26 | statusgen/selfimprovement.go:400 | statusgen (self-improvement digest) | (a) | #1223 | GAP — item detail fetch (`ghSelfImprovementDetailFetcher`) | forge-neutral/18 |

### B. `tools/desk/**` — the five sanctioned desk-verb `gh` exceptions (identity, not transport)

Already permitted in `tools/desk/internal/forgeban/allowlist.go`'s `AllowedInvocations` (ceiling 5). Per this
brief's own facts: these are a **separable, token-custody-gated follow-wave, not transport gaps** — routed here
as such, not to a v2 transport migration. `tools/desk/cmd/deskpushguard/main.go`'s line has **drifted** from the allowlist's own
(line-less) key since the allowlist was last touched: the fix for #1201 (row 41 below) added 23 lines above it.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 27 | tools/desk/cmd/deskadvisory/advisory.go:183 | deskadvisory | (a), (d) | — | GAP — `gh auth token` reads the ambient CLI credential, i.e. IS the identity layer (D2 keeps identity deliberately outside `Forge`); no `Forge` method could replace it | token-custody follow-wave (unrouted in v2) |
| 28 | tools/desk/cmd/deskdigest/exec.go:47 | deskdigest | (a), (d) | — | mixed; also runs `issue list`, which has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 29 | tools/desk/cmd/deskdisposition/exec.go:30 | deskdisposition | (a), (d) | — | write-only now (`ListOpenChanges` already absorbed the read half, #1123); `label list` has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 30 | tools/desk/cmd/deskmerge/exec.go:114 | deskmerge | (a), (d) | — | `pr view` half maps to `GetPullRequest`; the merge-authority `gh api` read has no enumerated op | token-custody follow-wave (unrouted in v2) |
| 31 | tools/desk/cmd/deskpushguard/main.go:432 | deskpushguard | (a) | — | GAP — branch→PR lookup by name; no enumerated op is keyed by branch (every read on the interface is keyed by number) | token-custody follow-wave (unrouted in v2) |

### C. `tools/desk/**` — other `gh`-subprocess / ambient-identity sites (not in the allowlist)

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 32 | tools/desk/askassay/probe.go:360 (via `readOnlyBinaries["gh"]=true` at :46, dispatched at :138) | askassay (ask-pane) | (a) | — | GAP — two registry questions read issue counts through this probe; already recorded in `forgeban`'s `UnresolvedArgv` ledger as a could-not-check (argv[0] is a variable), not a permit; the allowlist header states plainly it "CAN launch `gh`" and that retiring it needs the answers re-sourced through the interface first | unrouted — "a brief of its own" per the allowlist's own header |
| 33 | tools/cellctl/cellctl:758 (`gh_token="$(gh auth token …)"`, in `gen_shims`) | cellctl (shell) | (a), (d) | #1145 | GAP — `gh auth token` is the identity-read exception, same shape as row 27; the shim threads the result through as `GH_TOKEN` for a shimmed verb's own `gh` child, which is the #1145 fix itself, not the bug — but it is still a live `gh` subprocess outside the two backends and belongs in the count | desktools-v2/06 |

### C2. `tools/desk/cmd/deskdispatch` — #1146, status disputed with `desktools-v2/06`

`desktools-v2/06` (authored 2026-09-16, freshness-checked `e9fa19d3`) states #1146 is open and specifies an
unlanded test, `TestDispatchHandsTokenToScriptChild` (`grep -rl TestDispatchHandsTokenToScriptChild tools/desk`
finds nothing at this commit — confirmed absent). But `resolveClaimAuth`
(tools/desk/cmd/deskdispatch/dispatch.go:872, credential hand-off at :904) already threads the minted role token
into the **full child environment** (`append(os.Environ(), "GH_TOKEN="+tok)`) for the legacy claim script, not
onto a child literally named `gh` — which is the shape #1146 names. Its own doc comment attributes this fix to a
*different* issue, #1151 ("the claim tools read only the AMBIENT credential — bare `gh api` in the script,
GH_TOKEN/--token-file in the binary"). Reported as a discrepancy, not resolved either way here: it is possible
#1151's fix subsumed #1146's shape for the claim step specifically, or #1146 names a distinct exec site this
sweep did not find (a dispatched worker/reviewer session's own git-credential path was not audited to the same
depth as the claim step). `desktools-v2/06`'s own Verify row 5 is the authoritative test either way.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 33b | tools/desk/cmd/deskdispatch/dispatch.go:904 | deskdispatch (`resolveClaimAuth`, legacy claim script) | (d) | #1146 (disputed — see note above; comment attributes this line to #1151) | N/A — token already threaded via full child env, not argv[0]-gated; `desktools-v2/06`'s own test decides whether this closes #1146 | desktools-v2/06 |

### D. `tools/desk/**` — hardcoded/misrouted query shape (shape c)

`tools/desk/cmd/deskclose/authority.go` does **not** hardcode a `pullRequest`/`mergeRequest` GraphQL literal
anywhere (an earlier draft of `desktools-v2/04` said it did; that was wrong — the word `pullRequest` appears in
this file only in an explanatory comment, `authority.go:135`). The real defect is narrower: `fetchComment`
(the untyped path) calls `Forge.ListComments`, which on the GitHub backend selects a PR's comment thread
(`forge_github.go`'s own `pullRequest` GraphQL block, correctly *inside* the backend) — so an authorizing
comment on an **issue** silently comes back empty. `fetchCommentTyped`/`ListCommentsTyped` already exist and
already fix this (the triage lane uses them, `triage.go:347`); the two rows below are the callers still on the
untyped path.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 34 | tools/desk/cmd/deskclose/authority.go:248 | deskclose (`authorize`, the ruling gate) | (c)-adjacent — wrong-typed call, not a literal query outside the backend | #1019 | `ListCommentsTyped` (already enumerated) | desktools-v2/04 |
| 35 | tools/desk/cmd/deskclose/authority.go:276 | deskclose (`authorizeManifest`) | (c)-adjacent — wrong-typed call, not a literal query outside the backend | #1019 | `ListCommentsTyped` (already enumerated) | desktools-v2/04 |

### E. `tools/desk/cmd/deskpost/github.go` — a second, hand-rolled GitHub REST+GraphQL client (largest undocumented finding)

`deskpost`'s verdict/comment/flip read+write path is **entirely GitHub-only**, through its own App-authenticated
`ghClient` in this one 1094-line file — never `deskkit.Forge`. This is self-documented in the tree
(`github.go:589-590`: "this binary's GitHub path runs its own hand-rolled REST client (this file), not
deskkit.Forge") and deliberate as a GitLab **fail-closed** (`requireGitHubForge`, #772) — but per this brief's own
definition, a GitHub fact outside the two backend files is a reach-around row regardless of intent, and **no
brief in `desktools-v2/02..11` currently names this file**. It mints its own token in-process
(`mintInstallationToken`, refuses-if-unminted, no ambient fallback — so it does **not** violate Principle 1's
custody rule) but every read/write below is a hand-built REST path or GraphQL string built outside
`forge_github.go`, largely duplicating operations the `Forge` interface already enumerates. Listed as its own
group because it is a structural duplicate of the seam, not a scattered leak.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 36 | tools/desk/cmd/deskpost/github.go:570 | deskpost (`ghClient.getPR`) | (c) | — | `GetPullRequest` | unrouted — new finding, no v2 brief names `tools/desk/cmd/deskpost/github.go` |
| 37 | tools/desk/cmd/deskpost/github.go:582 | deskpost (`ghClient.getIssue`/`getIssueTyped`) | (c) | — | `GetIssueTyped` | unrouted |
| 38 | tools/desk/cmd/deskpost/github.go:656 | deskpost (`ghClient.headCommitAuthor`) | (c) | — | `GetCommit` | unrouted |
| 39 | tools/desk/cmd/deskpost/github.go:668 | deskpost (`ghClient.listReviews`, paginated) | (c) | — | `ReviewsAtHead` | unrouted |
| 40 | tools/desk/cmd/deskpost/github.go:715 | deskpost (`ghClient.listLabelEvents`, paginated) | (c) | — | `ListLabelEvents` (already enumerated — exact duplicate) | unrouted |
| 41 | tools/desk/cmd/deskpost/github.go:747 | deskpost (`ghClient.listLabels`, paginated) | (c) | — | GAP — no enumerated op reads the present-labels set on one item (`ListLabels(repo)` is repo-wide label *definitions*, not per-item applied labels) | unrouted |
| 42 | tools/desk/cmd/deskpost/github.go:794 | deskpost (`ghClient.listFiles`, paginated) | (c) | — | `ListChangedFiles` | unrouted |
| 43 | tools/desk/cmd/deskpost/github.go:825 | deskpost (`ghClient.combinedStatusAt`, paginated) | (c) | — | `ChecksAtHead` | unrouted |
| 44 | tools/desk/cmd/deskpost/github.go:846 | deskpost (`ghClient.checkRunsAt`, paginated) | (c) | — | `ChecksAtHead` | unrouted |
| 45 | tools/desk/cmd/deskpost/github.go:865 | deskpost (`ghClient.postReview`) | (c) | — | `PostReview` | unrouted |
| 46 | tools/desk/cmd/deskpost/github.go:872 | deskpost (`ghClient.postComment`) | (c) | — | `PostComment`/`PostCommentTyped` | unrouted |
| 47 | tools/desk/cmd/deskpost/github.go:896 | deskpost (`ghClient.getRepoFile`) | (c) | — | `ReadFile` | unrouted |
| 48 | tools/desk/cmd/deskpost/github.go:926 | deskpost (`ghClient.listCommentBodies`) | (c) | — | `ListComments`/`ListCommentsTyped` | unrouted |
| 49 | tools/desk/cmd/deskpost/github.go:947 | deskpost (`ghClient.fetchPRTrustPayload`, raw POST `/graphql` using `deskkit.PRTrustQuery`) | (c) | — | `PRTrustEvents` (the query constant is already shared/centralized in `tools/desk/internal/deskkit/trustfetch.go` — only the transport differs) | unrouted |
| 50 | tools/desk/cmd/deskpost/github.go:964 | deskpost (`ghClient.fetchIssueTrustPayload`, raw POST `/graphql` using `deskkit.IssueTrustQuery`) | (c) | — | `IssueTrustEvents` | unrouted |
| 51 | tools/desk/cmd/deskpost/github.go:1056 | deskpost (`ghClient.markReadyForReview`, raw GraphQL mutation) | (c) | — | `MarkReadyForReview` (already enumerated — exact duplicate) | unrouted |
| 52 | tools/desk/cmd/deskpost/github.go:1082 | deskpost (`ghClient.RepoVisibility`) | (c) | — | `RepoVisibility` (already enumerated, same name, different transport) | unrouted |

`tools/desk/cmd/deskpost/github.go:1068,1072` (`readMergeHold`/`setMergeHold`) are **not** rows: both are unconditional
GitHub-typed-not-applicable stubs that issue no request. `tools/desk/cmd/deskboard/board.go` and `tools/desk/cmd/issueboard/board.go`'s own
`PRTrustQuery`/`IssueTrustQuery` consumers (rows their code comments still describe as `gh api graphql`) have
**already migrated** onto `f.PRTrustEvents`/`f.IssueTrustEvents` — `deskboard/board.go:581` and
`issueboard/board.go:265` call the typed `Forge` methods directly; the comments are stale, the code is not. Not
rows; not reach-arounds.

### F. `tools/desk/**` — hardcoded remote name (shape b)

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 53 | ~~tools/desk/cmd/deskpushguard/main.go~~ (was ~line 409/432) | deskpushguard | (b) | #1201 | **RESOLVED** — fixed by commit `539a3735c` (PR #1271, "Fix deskpushguard hardcoding remote name \"origin\"", already on `origin/main`), *before* this audit ran. `checkForeignCommits`/`resolveRemoteMain` now take the actual push-target remote name from the pre-push hook's own `args[0]`, falling back to `"origin"` only when that argument is absent (`main.go:93,368` are the now-legitimate fallback lines, not the bug). Kept as a row only because Verify row 3 requires `#1201` present and this is where it lived. | desktools-v2/05 (closed already; brief narrows to #884) |
| 54 | tools/desk/cmd/deskpushguard/main.go (no `insteadOf`/`pushInsteadOf` expansion anywhere in this package — `grep -n insteadOf tools/desk/cmd/deskpushguard/*.go` returns nothing) | deskpushguard | (b), (d) | #884 | GAP — the push-transport guard parses the raw remote URL git hands it and never expands `url.<base>.insteadOf`/`pushInsteadOf`, so a rewritten URL evades the remote/repo derivation this file does (contrast `tools/desk/cmd/deskgit/deskgit.go`, which explicitly expands `insteadOf` for the same class of gap, and `tools/desk/internal/gitcore/gitcore.go`, which has none *because* it accepts no remote alias at all) | desktools-v2/05 |
| 55 | tools/desk/internal/deskkit/forgeresolve.go:87 (`originRemoteHost`, consulted at :145) | deskkit (`resolveForgeKind`) | (b) | — | GAP-adjacent — a documented, tested **fallback of last resort**: the roster (`ASSAY_REPO_FORGES`) is always consulted first (`TestForgeForResolvesFromRepoConfig` panics if `originRemoteHost` is reached while the roster can answer), so this is lower-severity than #1201's class (no unconditional misattribution), but it is still a hardcoded `"origin"` read used to decide which *forge* answers a caller with no roster entry, for a repo assumed to be the CWD's own checkout | unrouted — no v2 brief targets general forge-kind resolution |

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
automation — each shells exactly one `gh` subcommand, documented as such in its own header comment. Still a
GitHub-specific subprocess outside the two backends per this brief's definition, and named explicitly by facts'
search-surface list.

| # | file:line | tool/skill | shape | issue | seam op it should use (or GAP) | migrating brief |
|---|---|---|---|---|---|---|
| 56 | plugins/assay/scripts/assay-inbox.sh:339 | assay-inbox (skill script, `gh issue list`) | (a) | — | GAP — a human-run desktop tool, not a desk-tool binary; no Forge equivalent is reachable from shell without a Go/CLI client | unrouted |
| 57 | plugins/assay/scripts/assay-inbox.sh:552 | assay-inbox (skill script, `gh issue view`, `--walk`/`--html` only) | (a) | — | GAP (same as above) | unrouted |
| 58 | plugins/assay/scripts/inbound-monitor.sh:231 | inbound-monitor (skill script, `gh issue list`) | (a) | — | GAP (same as above) | unrouted |
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
*onto* — a CI step cannot shell out to a Go binary's internal interface.

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

## Reconciled:

Cross-check against `tools/desk/internal/forgeban/allowlist.go` (`const allowedInvocationCeiling = 5` at this
commit), so no Go call site is double-counted or dropped between the two registers.

- **`AllowedInvocations` (5 rows) → inventory rows 27–31, one-to-one.** All five map cleanly by
  `<file>::<enclosing decl>::gh` key; only `tools/desk/cmd/deskpushguard/main.go`'s line number has drifted (allowlist keys
  carry no line number, only `file::func::bin`, so the drift is invisible to the allowlist itself — it only
  shows up here because this inventory cites concrete lines).
- **`UnresolvedArgv` (16 rows) → one inventory row (32), the rest excluded with reasons already stated in the
  register itself.** `askassay/probe.go::execRead::<unresolved>` is the one row that the register's own text
  says "CAN launch `gh`" — inventory row 32. The other 15 `UnresolvedArgv` entries
  (`tools/desk/cmd/clusterguard/shim.go`, `deskadvisory/advisory.go::runChecks`, four `deskboard/*.go` rows, `deskpreflight`,
  `tools/desk/cmd/verifyloop/durable.go`, `tools/desk/internal/acp/client.go`, `tools/desk/internal/deskkit/callout.go`,
  `internal/deskkit/preflight.go::coldMintProbe`, `tools/desk/internal/deskkit/riskcallout.go`,
  `internal/deskkit/untrustscan.go::runSemgrep`, `cmd/desksupervise/live.go::showClaim`, and the two
  `tools/desk/internal/deskkit/migrate.go` statusgen-binary rows) are excluded per the register's own stated reasons:
  each resolves to a non-forge binary (a cluster CLI, `desktoken`, `statusgen`, Semgrep, an agent-protocol
  server, an operator callout, or a consumer repo's own `dispatch-claim.sh`) that the checker cannot statically
  prove but that carries no forge path on any reachable branch. None is an inventory row.
- **Inventory rows with no allowlist entry at all:** every row in groups A (statusgen — the allowlist counts
  `tools/desk/**` Go call sites only, never `statusgen/**`), C (row 33, cellctl shell), D (rows 34–35, not a
  `gh`-subprocess shape at all), E (rows 36–52, `tools/desk/cmd/deskpost/github.go` — an HTTP client, not a `gh` subprocess, so
  `forgeban`'s AST checker — which greps for `exec.Command`/`gh` argv — cannot and does not see it), F (rows
  53–55, hardcoded-remote shape, not a `gh`-subprocess shape), G (rows 56–59, shell scripts outside
  `tools/desk/**`), and H (rows 60–69, YAML, not Go). `forgeban` is scoped to `tools/desk/**` Go `exec.Command`
  call sites (`const allowedInvocationCeiling`'s own package, `tools/desk/internal/forgeban`); the other seven
  groups are exactly the "shell, skills, and **statusgen** sites it does not cover" this brief's facts describe.

## Outward writes

One row per text-carrying `Forge` write call site and per push-path text surface, with the checks it runs today.
Seeded from `docs/streams/desktools-v2/spec.md` §8.2 and re-verified line by line against `origin/main` @
`951ca784d` for this inventory (`desktools-v2/10` ticks against this table). Every citation below was read
directly, not copied — `deskpr edit`'s row corrects `spec.md`'s bare `edit.go:NNN` citations to their real
package path, `tools/desk/cmd/deskpr/edit.go` (there is no deskpost/edit.go; `EditChange`'s only definition is
in `deskpr`). All other citations from `spec.md` §8.2 checked out unchanged.

| Verb (write) | Surface | Secret scan | Self-contained + withheld (public targets) | Override offered |
|---|---|---|---|---|
| `deskfile new` (`FileIssue`, tools/desk/cmd/deskfile/deskfile.go:661) | issue title + body | yes (deskfile.go:545,548) | **no** | no ("No override flag exists", deskfile.go:540) |
| `deskfile attach` (`PostCommentTyped`, tools/desk/cmd/deskfile/deskfile.go:812) | comment body | yes (:780) | **no** | no |
| `deskpr create` (`CreateDraftChange`, tools/desk/cmd/deskpr/deskpr.go:335) | PR title + body | yes (deskpr.go:178,720) | yes (deskpr.go:232) | yes |
| `deskpr create`/`update` (the push) | branch name | yes (deskpr.go:724) | **no** | yes |
| `deskpr create`/`update` (the push) | branch diff | secret arms + ruling-claim guard only (deskpr.go:762,766) | **no** | yes |
| `deskpr create`/`update` (the push) | commit messages | **no** | **no** | — |
| `deskpr edit` (`EditChange`, tools/desk/cmd/deskpr/edit.go:273) | PR title + body | yes (edit.go:98,105) | yes (edit.go:221) | yes |
| `deskreply` reply and `--workpad` (tools/desk/cmd/deskreply/deskreply.go:321, workpad.go:100,243) | reply body | yes (deskreply.go:162) | yes (deskreply.go:172) | yes |
| `deskpost comment` (tools/desk/cmd/deskpost/comment.go:186,189) | comment body | yes (comment.go:54) | yes (comment.go:64) | no |
| `deskpost review` (tools/desk/cmd/deskpost/review.go, forgeclient.go:303) | review body | yes (review.go:128) | yes (review.go:136) | no |
| `deskevidence` (`WriteFile`, tools/desk/cmd/deskevidence/deskevidence.go:370,422) | file content (added lines) | yes (deskevidence.go:281) | **no** | no |
| `deskevidence` (`CreateDraftChange`, :442) | tool-composed PR title + body | **no** | **no** | — |
| `deskclose` (`PostCommentTyped`, tools/desk/cmd/deskclose/github.go:189) | tool-composed pre-close comment (can name a cross-repository canonical target) | **no** | **no** | — |
| `deskprovenance` (tools/desk/cmd/deskprovenance/main.go:238,241) | tool-composed comment | **no** | **no** | — |
| `desklabel`, `deskpost label`, `deskflip`, `deskdispatch`, `deskfile new`, `deskclose superseded` (`ApplyLabels`) | label names | **no** | **no** | — |
| `deskpushguard` (pre-push hook) | commit messages, ref names, diff | **no** — guards lineage and merged-branch pushes; scans no text | **no** | — |
| **`tools/desk/cmd/deskpost/github.go`'s own `ghClient` writes** (rows 45–46, 51 above: `postReview`, `postComment`, `markReadyForReview`) | review body, comment body, ready-flip mutation | **outside the check inventory entirely** — these never construct a `Forge`, so `ResolveForge`'s wrap (§8.3) cannot reach them even after `desktools-v2/10` lands, until rows 36–52 are migrated onto the seam | **no** | no |

Access-pattern (N+1) candidates for `desktools-v2/09`, seeded here rather than re-derived there: a board sweep
that calls `PRTrustEvents`/`ReviewsAtHead`/`ChecksAtHead` once per PR (`tools/desk/cmd/deskboard/board.go`'s `prBlessed` and its
CI-rollup read) and once per issue (`tools/desk/cmd/issueboard/board.go`'s `fetchIssueBlessed`) is exactly the N+1 shape
Principle 3 targets — each already routes through a typed `Forge` op (not a reach-around; the two migrated
consumers noted in group E), so `desktools-v2/09`'s work is adding a *snapshot* operation (one review-queue /
board-sweep GraphQL read) these can fold onto, not fixing a bypass.

## GAP summary (interface additions a future brief may need)

Recorded, not resolved here, per this brief's own scope: a per-item "present labels" read (row 41; `ApplyLabels`
writes but nothing reads the current set through the seam), a branch→change lookup (row 31; every read today is
keyed by PR/issue number, never a branch name), and the statusgen-side envelope gaps in group A's "GAP" cells
(author/comment-permalink resolution, all-state PR/issue listings, merged-PR-since queries) — all of these are
`deskread`-envelope questions for `forge-neutral/18`, not `Forge`-interface questions for this stream, since
statusgen never calls `Forge` directly (Principle 2).
