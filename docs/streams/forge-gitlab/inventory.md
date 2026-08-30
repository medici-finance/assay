# forge-gitlab/01 — forge-operation inventory

The op → tool → call-site inventory the `Forge` interface (`tools/desk/internal/deskkit/forge.go`)
is defined FROM (brief-01 task 1). Two facts drive the shape of the seam:

1. **Two transports reach the forge today.** The App/service-identity tools
   (`deskpost`, `deskevidence`, `deskrelease`, `desktoken`, `deskadvisory`) construct GitHub
   REST/GraphQL requests directly over `net/http`. The coordination tools (`deskpr`,
   `deskfile`, `deskclose`, `deskboard`, `deskdigest`, …) shell out to the `gh` CLI. The
   brief's Verify item 2 (`grep api.github.com tools/desk/cmd == 0`) targets the FIRST group —
   the direct host-literal construction — so the extraction lifts the host literal into the
   forge module (`deskkit.GitHubAPIBase`) and the operation behavior into the `github`
   implementation (`forge_github.go`), pinned by goldens.
2. **Token minting and the wrappers stay OUT of the interface.** App-JWT / PAT minting is the
   identity layer (spec §2, §5) — a `Forge` receives an already-minted token. Budgets, rate
   limiting, breakers, and body/secret checks WRAP the interface (spec §6) and never move into
   an implementation.

## The `Forge` interface — method set (frozen)

One row per method of `deskkit.Forge`. This table is the dereference target for Verify item 4
(`go doc ./tools/desk/internal/deskkit Forge` must match these rows). Concept mapping to
GitLab (spec §6) is recorded per row for brief 02.

The `gitlab impl` column is the brief-02 tick: `implemented` means `forge_gitlab.go` covers the
operation AND `forge_gitlab_test.go` carries at least one golden-pinned contract case for it —
a pairing `TestForgeGitlabCoverage` reconciles against this very table, so a row cannot be
ticked without a case behind it.

| # | Method | Frozen op (spec §6) | GitHub impl (extracted from) | GitLab mapping (brief 02) | gitlab impl |
|---|--------|--------------------|------------------------------|---------------------------|-------------|
| 1 | `GetPullRequest(repo, number)` | read change (supports flip + checks) | `deskpost` `getPR` (`GET /repos/{o}/{r}/pulls/{n}`) | `GET /projects/:id/merge_requests/:iid`; `changes_count` → `ChangedFiles` (a truncated `N+` reports `N+1`, so it can only fail closed) | implemented |
| 2 | `GetIssue(repo, number)` | resolve number kind | `deskpost` `getIssue` / `deskclose` `fetchItem` (`GET /issues/{n}`) | `GET /projects/:id/issues/:iid` AND `…/merge_requests/:iid` — separate IID sequences make a bare number ambiguous, so both are probed and a both-resolve is refused | implemented |
| 3 | `ReviewsAtHead(repo, number)` | read reviews at head | `deskpost` `listReviews` (`GET /pulls/{n}/reviews`, paginated) | MR approvals + notes; the head-pin comes from `reset_approvals_on_push` (approvals) and the diff-version timestamp (notes), and is left unset where neither establishes it | implemented |
| 4 | `ListChangedFiles(repo, number)` | read change files (risk gate) | `deskpost` `listFiles` (`GET /pulls/{n}/files`, paginated, rename-aware) | `GET /merge_requests/:iid/diffs` (paginated; `changes` is the deprecated single-shot form) | implemented |
| 5 | `ChecksAtHead(repo, sha)` | read checks at head | `deskpost` `combinedStatusAt` + `checkRunsAt` (both rollups, paginated) | commit `status` → combined state; `/statuses` → statuses; last pipeline's JOBS → check-runs. External status checks are MR-scoped and stay for the Ultimate lane | implemented |
| 6 | `IssueReactions(repo, number)` | read reactions/awards | `deskpost` `IssueReactions` / `deskkit.HTTPRepoInfoFetcher` (squirrel-girl, single page) | award emoji, with names mapped to GitHub's reaction vocabulary and the human/bot discriminator resolved from the users API (never defaulted) | implemented |
| 7 | `RepoVisibility(repo)` | repo visibility gate | `deskpost` `RepoVisibility` / `deskkit.HTTPRepoInfoFetcher` (`GET /repos/{o}/{r}`) | `GET /projects/:id` `.visibility` (`internal` passes through) | implemented |
| 8 | `CreateDraftChange(repo, in)` | create draft change | `deskpr` `gh pr create --draft` → REST `POST /pulls` `draft:true` | `Draft:` MR (`POST /merge_requests`), refused if it does not come back marked draft | implemented |
| 9 | `PostComment(repo, number, body)` | comment | `deskpost` `postComment` (`POST /issues/{n}/comments`) | MR/issue note, routed by the same kind resolution as op 2 | implemented |
| 10 | `PostReview(repo, number, in)` | approve/review | `deskpost` `postReview` (`POST /pulls/{n}/reviews`, head-pinned) | note first, then `POST /approve` with `sha` (server-validated); REQUEST_CHANGES = note + unapprove | implemented |
| 11 | `MarkReadyForReview(nodeID)` | flip draft | `deskpost` `markReadyForReview` (GraphQL mutation) | clear the `Draft:` prefix via `PUT`; the node id is the backend-minted `gitlab:<owner>/<name>!<iid>` | implemented |
| 12 | `FileIssue(repo, in)` | file issue | `deskfile` `gh issue create` → REST `POST /issues` | `POST /projects/:id/issues` | implemented |
| 13 | `CloseIssue(repo, number, reason)` | close issue | `deskclose`/`deskfile` `gh issue close` → REST `PATCH /issues/{n}` `state:closed` | `PUT /issues/:iid` `state_event:close`; the reason has no GitLab field and is recorded as a note | implemented |
| 14 | `PushTransportHint(repo)` | push-transport hints | `x-access-token` https + inline credential.helper (no token-in-URL) | `oauth2` username + inline credential.helper, host from the configured instance | implemented |

## Per-tool call-site inventory (current state)

### Direct-HTTP tools (the `api.github.com` construction Verify item 2 targets)

| Tool | File | Operation(s) | Now → after brief 01 |
|------|------|--------------|----------------------|
| `deskpost` | `tools/desk/cmd/deskpost/github.go` | getPR, getIssue, listReviews, listFiles, combinedStatusAt, checkRunsAt, postReview, postComment, markReadyForReview, RepoVisibility, IssueReactions, trust GraphQL | host literal → `deskkit.GitHubAPIBase`; behavior extracted verbatim into `forge_github.go` (goldens) |
| `deskevidence` | `tools/desk/cmd/deskevidence/github.go` | fetchRemoteFile, commitFile (Contents API) | host literal → `deskkit.GitHubAPIBase`. Contents-API commit is Evidence-landing, NOT a frozen forge op (see delta D3) |
| `deskrelease` | `tools/desk/cmd/deskrelease/github.go` | getRef, createTagRef (git-data refs) | host literal → `deskkit.GitHubAPIBase`. Tag/ref ops are release-integrity, NOT a frozen forge op (delta D3) |
| `desktoken` | `tools/desk/cmd/desktoken/desktoken.go` | list installations, exchange JWT | host literal → `deskkit.GitHubAPIBase`. Token mint is the identity layer, OUT of the interface (delta D2) |
| `deskadvisory` | `tools/desk/cmd/deskadvisory/advisory.go` | ghAPI GET (security advisories) | host literal → `deskkit.GitHubAPIBase`. Advisory read is not a frozen forge op (delta D3) |
| `deskkit` | `tools/desk/internal/deskkit/repovis.go` | HTTPRepoInfoFetcher: RepoVisibility, IssueReactions | already deskkit-level; default base aligned to `deskkit.GitHubAPIBase` |

### `gh`-CLI tools (reach the forge without a host literal)

| Tool | File | Operation(s) | Interface method |
|------|------|--------------|------------------|
| `deskpr` | `tools/desk/cmd/deskpr/deskpr.go` | `gh pr create --draft`, `gh pr view/list` | `CreateDraftChange` (+ reads via `GetPullRequest`) |
| `deskfile` | `tools/desk/cmd/deskfile/deskfile.go` | `gh issue create`, `gh issue comment`, `gh issue view` | `FileIssue`, `PostComment`, `CloseIssue` |
| `deskclose` | `tools/desk/cmd/deskclose/exec.go` | `gh issue/pr view`, `gh issue/pr comment/close` | `GetIssue`, `PostComment`, `CloseIssue` |
| `deskreply` | `tools/desk/cmd/deskreply/deskreply.go` | `gh pr/issue comment` | `PostComment` |
| `deskflip` | `tools/desk/cmd/deskflip/flip.go` | `gh pr ready` | `MarkReadyForReview` |

These are NOT rewired in brief 01 — see delta D1.

## Reconciliation deltas vs spec §6

- **D1 — call-site migration is staged, not wholesale (task 4 scope).** Brief 01 delivers the
  seam and its golden safety-net, and neutralises the direct host-literal construction so
  Verify item 2 is 0. The `gh`-CLI call sites and the bespoke per-tool REST clients are NOT
  ripped through the interface in this change: each carries tool-specific error mapping (e.g.
  `deskrelease`'s 422→Refused tag guard, `deskpost`'s 403 App-permission diagnosis and 401
  re-mint) whose wholesale relocation is exactly the "subtle behavior change survives per-tool
  tests" hazard the brief's exec-tier note names. Zero behavior change is the deliverable, so
  each tool migrates behind the goldens in its own follow-on change (freeze rule: a consuming
  tool per addition). The `github` implementation captures the behavior verbatim so a migrating
  tool has a pinned contract to stay equal to.
- **D2 — token minting is excluded from the interface.** Spec §6 lists the operation set; App-
  JWT / PAT minting is the identity layer (spec §2, §5) and stays in each tool. A `Forge`
  receives an already-minted token (the `GitHubForge.Token` field). `PushTransportHint` (op 14)
  carries the transport SHAPE only — no secret.
- **D3 — three direct-HTTP operations are NOT frozen forge ops.** `deskevidence`'s Contents-API
  Evidence commit, `deskrelease`'s git-data ref read/tag create, and `deskadvisory`'s advisory
  read each construct `api.github.com` but are not in the spec §6 frozen list (Evidence landing,
  release integrity, and advisory intake are distinct concerns). Brief 01 relocates their host
  literal into `deskkit.GitHubAPIBase` (so Verify item 2 is 0) but does NOT add them to the
  `Forge` method set — that would violate the freeze rule (no consuming reframe in this change).
  If a second forge needs them, each is added with its consuming tool.
- **D4 — reads added beyond the bare spec list.** `GetPullRequest`, `GetIssue`,
  `ListChangedFiles`, and `RepoVisibility` are consumed by shipping tools (`deskpost`'s
  flip/review/risk/public-repo gates) and are therefore in the frozen set even though spec §6's
  prose enumerates the mutations and the two "read … at head" rollups. They are not additions
  beyond what a shipping tool consumes — they are the reconciliation the brief's task 2 asks for.

## Golden corpus

### gitlab (brief 02)

`forge_gitlab_test.go` (`TestForgeGitlabGolden`) pins the GitLab backend the same way — request
method, ESCAPED path, query, write bodies, `X-Next-Page` pagination, result mapping and error
classification, one golden per scenario. `TestForgeGitlabCoverage` is the reconciliation: it
measures the corpus against the frozen `Forge` interface (by reflection) AND against this
document's method-set table, and fails naming any operation with no case. `TestForgeGitlabTierErrors`
holds the three-state surface — 401/403/404 each arrive as a `could-not-check` refusal carrying a
`ForgeAPIError`, never as an empty result. `forge-gitlab-mutations.json` is the re-runnable
`muhar` spec for the mappings that would otherwise fail silently (`go run ./cmd/muhar -j 0 -spec
internal/deskkit/forge-gitlab-mutations.json` from `tools/desk`).

### github (brief 01)

`forge_github_golden_test.go` (`TestForgeGithubGolden`) pins 17 operations — request
method/path/query, write bodies, pagination (`per_page=100`, multi-page walk, short-page stop),
result mapping, and error classification (404 → `IsForgeNotFound`; 403 → `ForgeAPIError`).
`TestForgeGithubGoldenCount` guards the floor (≥ 10). Regenerate on an INTENTIONAL change with
`-update`; an unintentional wire change shows as a golden diff — the single-point-of-failure
control the brief names, independent of the per-tool tests.
