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
| 15 | `DeleteRef(repo, ref)` | delete one git ref | `DELETE /repos/{o}/{r}/git/refs/{ref}` (git-data refs) — replaces `fanoutloop`'s `gh api -X DELETE repos/…/git/refs/dispatch/…` | `DELETE /projects/:id/repository/branches/:branch` (Branches API, **Tier: Free**). GitLab CE exposes NO general ref-delete endpoint, so only the `heads/<branch>` namespace maps; every other namespace is a could-not-check REFUSAL naming the gap, never a silent success | implemented |

| 16 | `ListLabelEvents(repo, number)` | read label APPLICATIONS with their applier | `deskpost` `listLabelEvents` / `deskflip` `readLabelEvents` (`GET /issues/{n}/timeline`, paginated, `labeled` events only) | `GET /merge_requests/:iid/resource_label_events`, `action:add` only; an event whose label was since deleted comes back unnamed and is dropped (a stamp nobody can name attests to nothing) | implemented |
| 17 | `ListComments(repo, number)` | read comments (identity + marker + hidden state) | `deskreply` `--workpad` comment list (GraphQL `comments(first:100)` — REST carries no `isMinimized`) | `GET /merge_requests/:iid/notes` ordered `created_at asc`; SYSTEM notes dropped (GitLab lists its own activity beside human comments); `Minimized` is false because GitLab has no minimise feature — exact, not defaulted; `URL` is empty because GitLab publishes no per-note permalink | implemented |
| 18 | `EditComment(repo, commentID, body)` | edit ONE existing comment | `deskreply` `--workpad` edit (GraphQL `updateIssueComment`, node id) | `PUT /merge_requests/:iid/notes/:id`; the opaque id is the backend-minted `gitlab:<owner>/<name>!<iid>#note<id>`, and a foreign or project-mismatched id is REFUSED rather than resolved | implemented |
| 19 | `ApplyLabels(repo, number, change)` | reconcile a change's labels in one operation | `deskpost` `ensureLabel`+`listLabels`+`addLabels`+`removeLabel` (4 hand-built requests) / `deskflip` `ensureLabelSwap` (`gh pr edit --add-label/--remove-label`) | `POST /projects/:id/labels` to ensure (409/400 "already taken" = the ensure's post-condition already holds), then ONE `PUT /merge_requests/:iid` carrying `add_labels`+`remove_labels` — atomic, where the GitHub backend issues one DELETE per removal. Colors travel as bare hex and each backend renders its own form (GitHub forbids a leading `#`, GitLab requires one) | implemented |
| 20 | `RequiredStatusChecks(repo, branch)` | read the status checks branch protection REQUIRES on a branch | `deskflip` `readRequiredChecks` (`GET /repos/{o}/{r}/branches/{branch}/protection/required_status_checks`; 404 = no protection / none required = empty set, every other non-2xx = could-not-check) — union of the legacy `contexts` and the newer `checks[].context` | `GET /projects/:id` `only_allow_merge_if_pipeline_succeeds` — the all-tier pipeline-gating setting; ON → a synthetic `pipeline` context (a check IS required), OFF → empty. External status checks stay in the Ultimate/MR-scoped lane (op 5) | implemented |

| 21 | `WriteFile(repo, in)` | write a file's whole content on a branch (Evidence landing) | `deskevidence` `commitFile` (`PUT /repos/{o}/{r}/contents/{path}`, base64, branch) | `POST`/`PUT /projects/:id/repository/files/:path` (Repository Files API); `start_branch` creates the side branch inline; the default branch is protected (pilot D-8) so a direct write to it returns the `DefaultBranchNotWritable` sentinel — nothing written, no write call | implemented |
| 22 | `ReadFile(repo, in)` | read a file's content at a ref (Evidence merge) | `deskevidence` `fetchRemoteFile` (`GET /repos/{o}/{r}/contents/{path}?ref=`) | `GET /projects/:id/repository/files/:path?ref=`; `last_commit_id` → the opaque `FileContent.SHA` an update cites; a 404 propagates as a could-not-check the caller tests with `IsForgeNotFound` | implemented |

**Ops 21–22 were added by brief `forge-neutral/04` under the same freeze rule**, with `deskevidence` as
the consuming call site in the same change: it routes its Evidence write through `WriteFile` and its
`--brief-path` Evidence-section merge (read → transform → write) through `ReadFile`, and drops its
hand-rolled JWT/installation exchange + `apiBaseURL` — the mint moves to the resolver's custody binding
(`ForgeFor(fr, "verifier")`). `WriteFile` is deliberately FAT: it folds the idempotency read (returning a
`Changed` flag), the append-only shrink guard (`AppendOnly`/`AllowShrink` passed in, the backend refuses
post-fetch), and the default-branch writability probe + inline branch-creation fallback, so the Evidence
lane needs no separate `CreateRef` op on the frozen interface. The ratchet ceiling is UNCHANGED at 16:
the forge-method count is not the ratchet, and no `deskevidence` permit row exists to remove (it reaches
the forge over `net/http`, never a forge CLI). The `deskpr`/`deskfile`/`deskclose` gh-migration those
tools' permit rows still gate is the FOLLOW-ON brief (`forge-neutral/04b`), which first adds the
enumerated ops each still needs (branch→change lookup, PR body/title fields, an issue-search op, a
label-list op) — see #509's ruling for why a code-aware rescope, not a ratchet-number correction, is
what that work needs.

| # | Operation | Purpose | GitHub source | GitLab mapping | Status |
|---|-----------|---------|---------------|----------------|--------|
| 23 | `ListOpenChanges(repo)` | bulk open-PR read + CI rollup (board) | `deskboard` `fetchOpenPRs` (one `gh api graphql`, `pullRequests(states:OPEN,first:100)` with the rollup CONTEXTS but not the `actions:read`-gated `checkSuite/workflowRun` sub-field) | **degraded (issue #686).** Serves each open MR (`GET /projects/:id/merge_requests?state=opened`, capped at 100 like the GitHub read) with the metadata the board's NEEDS-REVIEW/RE-REVIEW trigger needs — number, title (draft prefix stripped), body, draft, author, labels, head/base refs, created-at — while marking the two fields with no 1:1 GitLab mapping **could-not-check PER CHANGE**: `mergeStateStatus` left EMPTY (the board's own `mergeVerdictUnknown` → MERGE-NOW withheld) and the CI rollup a single `GitLabRollupUnmapped` entry (`ciState` reads it as `ciUnknown` → CI-green, and thus MERGE-NOW/FLIP, withheld). `LastEditedAt` is EMPTY too (GitLab exposes no title/body-edit timestamp; `updated_at` moves on unrelated events). The full rollup+`mergeStateStatus` mapping (CI is pipelines-and-jobs — a DIFFERENT shape, see op 5; `mergeStateStatus` is GitHub-only) remains deferred to the forge-gitlab board-read brief — nothing is approximated, the unmappable fields are named unreadable | degraded |
| 24 | `ListOpenIssues(repo)` | bulk open-issue read (issue board) | `issueboard` `fetchOpenIssues` (`gh issue list --state open`, issues only) | **could-not-check.** GitLab lists open issues, but this summary FEEDS the trust gate + escalation clock (rendered bot-suffixed login, numeric author id, created-at) paired per issue with `IssueTrustEvents` (op 26, could-not-check on GitLab), so the whole issue lane is deferred to the forge-gitlab trust-events brief rather than shipping a list its gate cannot admit | github-only |
| 25 | `PRTrustEvents(repo, number)` | PR trust-gate content events | `deskboard` `prBlessed` (`gh api graphql`, `deskkit.PRTrustQuery`) | **could-not-check.** The gate reads GitHub GraphQL `lastEditedAt` content-edit tracking + numeric `databaseId` with Bot/User actor discrimination; GitLab's note/system-note model and id space do not map 1:1. Deferred to the forge-gitlab trust-events brief, never approximated (a guessed blessing is fail-open) | github-only |
| 26 | `IssueTrustEvents(repo, number)` | issue trust-gate content events | `deskboard` `issueBlessed` / `issueboard` trust gate + escalation clock / `scanloop` queueing gate (`gh api graphql`, `deskkit.IssueTrustQuery`) | **could-not-check.** The issue twin of op 25 — same GraphQL content-edit / numeric-actor-id gap, same deferral | github-only |

**Ops 23–26 were added by brief `forge-neutral/06`** under the freeze rule, each with its consuming call
site migrated in the same change: `deskboard`'s two hand-authored GraphQL reads (open-PR + trust) route
through ops 23/25/26; `issueboard` migrates FULLY onto ops 24/26 (+ `GetIssue` for a RETIRE-row title,
which gains an `Issue.Title` field); `scanloop` migrates FULLY (its trust probe onto `GetIssue`+op 26, its
title/body refresh onto the sanctioned `deskpr edit` verb, and its `RealExec` seam onto literal-argv
dispatch so it leaves the unresolved-argv ledger). The three retired permit rows (issueboard's `ghRun`,
scanloop's `lane.go` and `trust.go` gh sites) drop the forge-CLI ceiling 16 → 13; `deskboard`'s row is
NARROWED (its five peripheral read categories stay on `ghRun`) with `forge-neutral/12` as its exit. On
GitLab all four ops are could-not-check-with-gap per the ruling: the GraphQL trust reads and the
CI-rollup-bearing bulk read are not 1:1, and the issue lane defers with its trust gate.

| # | Operation | Purpose | GitHub source | GitLab mapping | Status |
|---|-----------|---------|---------------|----------------|--------|
| 27 | `ListRecentCommits(repo, limit)` | commit-history listing (branch health) | `deskboard` `fetchRecentCommits` (`GET /repos/{o}/{r}/commits?per_page=N`, default branch) | `GET /projects/:id/repository/commits` — 1:1: the sha (`id`) and `committed_date` are the fields the probe reads (only the sha). An EMPTY project answers 404 (GitHub 409), surfaced as the `IsForgeEmptyRepo` known-state | implemented |
| 28 | `GetCommit(repo, sha)` | single-commit read (stall clock) | `deskboard` `fetchHeadCommit` (`GET /repos/{o}/{r}/commits/{sha}`) | `GET /projects/:id/repository/commits/:sha` — `committed_date` is 1:1; the resolved-account login fields are a per-field could-not-check (left EMPTY) because a GitLab commit carries raw git author/committer name+email, not a resolved instance account (the ReviewsAtHead-CommitID posture) | implemented |
| 29 | `CompareRefs(repo, base, head)` | ref comparison (benign-merge + behind-by) | `deskboard` `changedFilesBetween` + `fetchBehindMain` (`GET /repos/{o}/{r}/compare/{base}...{head}` — files + `behind_by` + `status`) | **could-not-check.** GitLab's compare reports neither the divergence STATUS word (identical/ahead/behind/diverged) nor a `behind_by` count — the two facts the consumers key on; behind_by needs a separate inverted compare and the status vocabulary has no analog. Deferred to the forge-gitlab compare brief, never approximated | github-only |
| 30 | `SearchOpenChanges(owner)` | owner-wide open-change search (scope reconcile) | `deskboard` `searchOpenPRs` (`gh search prs --owner <o> --state open` → `GET /search/issues?q=is:pr+is:open+user:<o>`) | **could-not-check.** GitHub's owner-wide PR search has no 1:1 GitLab analog — GitLab search is group/project-scoped and paginated differently. Deferred to the forge-gitlab search brief; an under/over-reported scope reconciliation is worse than a stated could-not-check | github-only |
| 31 | `ListWorkflowFiles(repo, ref)` | workflow-directory listing (zero-CI probe) | `deskboard` `listWorkflowFiles` (`GET /repos/{o}/{r}/contents/.github/workflows?ref=`) | **could-not-check.** GitHub-Actions-specific: GitLab CI config is a single `.gitlab-ci.yml`, not a per-workflow-file directory, so there is no 1:1 listing. Deferred to the forge-gitlab CI brief | github-only |
| 32 | `ChangeDiff(repo, number)` | raw unified-diff document (human display) | `deskboard` `cmdDiff` (`gh pr diff` → `GET /repos/{o}/{r}/pulls/{n}` with the `.v3.diff` media type) | **could-not-check.** GitLab serves a change's diff as a STRUCTURED per-file list (op 4, `ListChangedFiles`), not a single raw unified-diff document; the raw-text read has no 1:1 form. Deferred to the forge-gitlab diff brief, never assembled here | github-only |

| # | Method | Frozen op (spec §6) | GitHub impl | GitLab mapping | gitlab impl |
|---|--------|--------------------|-------------|----------------|-------------|
| 33 | `OpenChangeForBranch(repo, branch)` | resolve the single OPEN change for a SOURCE-branch name | `deskpr` existing-PR check + `warnIfConflicting` (`GET /repos/{o}/{r}/pulls?head={owner}:{branch}&state=open`) | `GET /projects/:id/merge_requests?source_branch=…&state=opened`; more than one open change on one source branch is a could-not-check REFUSAL (ambiguous), never a silent first-match; NONE → (nil, nil) | implemented |
| 34 | `EditChange(repo, number, in)` | replace a change's own title/body text | `deskpr edit` body replace (`PATCH /repos/{o}/{r}/pulls/{n}`) | `PUT /projects/:id/merge_requests/:iid`; an empty field is not sent (a body-only edit leaves the `Draft:` title prefix untouched); changing neither is a could-not-check refusal | implemented |
| 35 | `SearchIssues(repo, in)` | free-text dedupe search over a repo's ISSUES | `deskfile` dedupe (`GET /search/issues?q=repo:o/r is:issue …`) — number, title, state, labels, URL | `GET /projects/:id/issues?search=…`; issues and MRs are separate sequences so a project issue search returns issues only. `GetIssue` (op 2) gains the shared `URL` field here too (#691) | implemented |
| 36 | `ListLabels(repo)` | the repo's labels by name, READS ONLY (never creates) | `deskfile` label-existence probe (`GET /repos/{o}/{r}/labels`) | `GET /projects/:id/labels`; the deliberate opposite of `ApplyLabels`'s ensure step — a missing label makes `deskfile` file UNSTAMPED rather than mint one | implemented |

**Ops 33–36 were added by brief `forge-neutral/13`** (the `04b` follow-on #509 named) under the same
freeze rule, each with its consuming call site re-seated onto the resolver in the SAME change: 33 by
`deskpr`'s existing-PR-for-branch check and `warnIfConflicting`; 34 by `deskpr edit`'s body replace; 35
and 36 by `deskfile`'s file-time dedupe and its label-existence probe. The two search/list reads are the
enumerated ops `deskpr`/`deskfile`/`deskclose` still lacked, which is why #509 ruled their gh-migration a
code-aware rescope rather than a ratchet-number correction. With the four ops added and the three verbs
routed through `ForgeFor`, their three permit rows (`cmd/deskclose/exec.go::runGH::gh`,
`cmd/deskfile/exec.go::gh::gh`, `cmd/deskpr/exec.go::gh::gh`) are removed and the forge-CLI ceiling falls
12 → 9. `deskclose` needed no new op/method (its reads/writes all mapped to existing ops; its `viewer{login}`
whoami is replaced by the minted role's known login, an identity-layer change, not a forge op). It
did extend three existing result shapes, each with `deskclose`/`deskfile`/`deskpr` as the in-change
consumer under the freeze rule (which binds methods, not fields): `Issue` gained `URL` (#691),
`Labels` and `Body` (its decision-label gate and PR-ref extraction read one `GetIssue`);
`PullRequest` gained `Title` (`deskpr edit`'s idempotency); and `ListComments`' comment author now
carries its numeric id (the blessing-authority strict id-pin `deskclose`'s authority read compares —
the GitHub GraphQL query gained the `databaseId` inline-fragment selection, GitLab already carried
it). The App/Bot exclusion in `deskclose`'s authority gate moves from the REST `type` field (absent
from the seam) to the seam's canonical `<slug>[bot]`/`app/<slug>` rendered-login discriminator; the
strict id-pin is unchanged, so the two-layer defense is preserved.
`SearchIssues` returns ISSUES only on both backends (GitHub filters PRs out with `is:issue`; GitLab's
project issue search is issue-only by the endpoint's own shape). On GitLab all four map 1:1 — none is a
could-not-check-with-gap — because each is a concrete project-scoped REST read/write with a direct analog.

**Ops 27–32 were added by brief `forge-neutral/12`** under the freeze rule, each with its consuming
`deskboard` call site migrated in the same change — the five PERIPHERAL read categories `forge-neutral/06`
left on the NARROWED `ghRun` permit row, plus the reads that already had an enumerated op and only stayed
on `ghRun` for historical reasons (`fetchReviews`→`ReviewsAtHead`, `fetchChangedFiles`→`GetPullRequest`+
`ListChangedFiles`, `fetchCheckRuns`+`fetchCombinedStatusTotal`→`ChecksAtHead`, `fetchWorkflowContent`+
`cmdFiles`→`ReadFile`, `fetchLabelEvents`→`ListLabelEvents`, `fetchLastAuthorComment`→`ListComments`,
`fetchPRState`→`GetPullRequest` gaining a `MergedAt` field, `cmdQueue`→`ListOpenIssues` gaining a `URL`
field, the policy-drift metadata reads→`RepoVisibility`). The combined-status TOTAL folded into
`ChecksAtHead`'s existing `StatusTotalCount` rather than growing a redundant op. With `deskboard`'s last
`ghRun` caller gone, its `cmd/deskboard/board.go::ghRun::gh` permit row is removed and the forge-CLI
ceiling falls 13 → 12. Ops 27–28 map 1:1 on GitLab (commit reads); ops 29–32 are could-not-check-with-gap
(a genuine non-1:1 each, named above) per the same ruling `forge-neutral/06` landed under.

**Ops 16–19 were added by brief `forge-neutral/03` under the same freeze rule**, each with its consuming
call sites converted in the same change: 16 by `deskflip`'s model-capability-floor read and `deskpost`'s
sibling; 17 and 18 by `deskreply`'s `--workpad` upsert; 19 by `deskflip`'s queue-label swap and
`deskpost`'s mechanical verdict labels. `ApplyLabels` is declarative rather than a set of primitives
(create / list / add / remove) precisely so the seam grows ONE method where the tools consume one
intent — and so a caller never needs a label-LISTING operation of its own: it names the label-name
FAMILIES it owns this run and the backend drops their stale members. A family the caller has no
definite value for is simply not named, so nothing in it is touched.

**Op 20 (`RequiredStatusChecks`) was added under the same freeze rule** with its one consuming call
site converted in the same change: `deskflip`'s checks-green condition. It exists because an ABSENT
CI rollup has two very different meanings — "nothing is required to merge" (green) and "the required
checks have not reported yet" (could-not-verify) — and the coarse roster ci-tag cannot tell them
apart, so a repo that runs CI without REQUIRING any check on App-authored PRs was unflippable
forever. The read keys the gate on what the forge actually enforces: an empty required set makes an
absent rollup green, a non-empty one keeps it could-not-verify, and a required-set that cannot be
read stays could-not-check (fail closed). It is a READ only; nothing about the gate's non-empty-rollup
behaviour changed.

`PostComment` (op 9) also gained a return value in that brief — a `CommentRef` carrying the created
comment's opaque id, numeric id and (where the forge publishes one) URL — so a write is answerable
without a follow-up read, the same way `CreateDraftChange` returns a `PullRef`.

**Op 15 was added by brief 08 under the spec §6 freeze rule** — with its consuming callsite converted in
the same change (`fanoutloop`'s dispatch-claim sink), not speculatively. It is the one op whose argument
is path-shaped, which is exactly the shape an arbitrary-endpoint escape hatch takes, so it carries a
validator: `deskkit.ValidateRefPath` (`forge_refpath.go`) refuses an un-namespaced ref, a traversal
(`..`), a URL-significant character (`? # %`), and git's own forbidden ref characters BEFORE a request
exists. The golden `delete_ref_refuses_namespace_escape` pins that a ref aimed at
`…/branches/main/protection` emits **zero** requests.

| # | Method | Frozen op (spec §6) | GitHub impl | GitLab mapping | gitlab impl |
|---|--------|--------------------|-------------|----------------|-------------|
| 37 | `RefExists(repo, ref)` | ref-existence read (model-capability-floor stamp age-out) | `GET /repos/{o}/{r}/git/ref/{ref}` (SINGULAR single-reference read, distinct from the plural `git/refs/` `DeleteRef` targets) — the logic extracted from `deskpost`'s hand-rolled `refExists`; a 404 is the ANSWER "absent" (false, nil), every other non-2xx is could-not-check | `GET /projects/:id/repository/branches/:branch` (Branches API, **Tier: Free**) — `DeleteRef`'s read twin, reaching exactly as far: GitLab CE exposes NO general ref-existence endpoint, so only the `heads/<branch>` namespace maps and every other namespace is a could-not-check REFUSAL naming the gap. The dispatch claim ref (`refs/heads/dispatch/<key>`) is INSIDE that namespace, so the live read round-trips here. A 404 → absent (false, nil); a 403 → could-not-check, never a guessed release | implemented |

**Op 37 (`RefExists`) was added by brief `forge-gitlab/09` under the same freeze rule**, with its one
consuming call site converted in the same change: `deskpost`'s `claimLiveness` — the model-capability
floor's stamp age-out — which read the claim ref through a hand-rolled REST call and now reads it through
this typed op (the GitHub backend, constructed with the token that path already minted; `deskpost`'s
verdict/comment/flip preconditions stay GitHub-only, `newGHClient` having already refused a
GitLab-resolved repo before any verb reaches `claimLiveness`). It is the second path-shaped-argument op
after `DeleteRef` and carries the same `deskkit.ValidateRefPath` bound, so it cannot address an arbitrary
endpoint; the goldens `ref_exists_present`/`ref_exists_absent`/`ref_exists_refuses_namespace_escape`
(GitHub) and `ref_exists_present`/`ref_exists_absent`/`ref_exists_non_branch_namespace_refused` (GitLab)
pin the present/absent/refused shapes, the last two emitting **zero** requests. Only a positive ABSENT
ages a stamp out; every uncertain path is could-not-check, which changes nothing. The
`GitLabRepoInfoFetcher` adapter (below, delta note) landed in the same brief so the public-repo gate can
run on a GitLab-resolved repo; it is not a `Forge` method (the gate takes the string-signature
`RepoInfoFetcher`), so it is not a row here.

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

## Residual forge-CLI call sites (brief 08 task 1)

Brief 08 makes the closed surface an enforced fact. The machine-readable half of this section is
`tools/desk/internal/forgeban/allowlist.go`, which the gate (`TestNoForgeCLIShellout`) reads; this
table is the human-readable map, and the two are reconciled by the gate's stale-row detection — a
permit that no longer matches a call site fails CI, so neither can rot silently.

**Measured at landing: 25 forge-CLI call sites across 23 declarations, plus 14 exec sites whose
argv[0] is not a compile-time constant.** One call site was RETIRED by this brief:

| Retired | Was | Now |
|---------|-----|-----|
| `fanoutloop` dispatch-claim sink | `gh api -X DELETE repos/<o>/<r>/git/refs/dispatch/<key>` — the stream's live passthrough: an argv carrying a whole REST path, so the reach was every endpoint by every method | `DeleteRef(repo, "dispatch/<key>")` (op 15), ref-validated inside the repo's namespace before a request exists |

**Follow-up (issue #834): two more reads retired.** The verify FAIL on Verify row 3 (issue #834)
noted the ban half shipped as a ratchet rather than closure-to-zero. Two of the three residual
`exec.Command("gh", …)` naive-grep hits were display-only reads whose stated blockers had gone
stale — the ops they needed had landed since brief 08:

| Retired | Was | Now |
|---------|-----|-----|
| `deskroster` `ghViewPR` | `gh pr view --json state,isDraft,title` (listed under **identity**/no-op — blocked on a `title` field `GetPullRequest` lacked) | `GetPullRequest` (op 1), which gained `Title` with its `deskpr edit` consumer; the merged/closed display state is derived from `State`+`Merged` |
| `deskroster` `ghListOpenPRs` | `gh pr list --state open --json number,title,isDraft` (listed under **no enumerated op**) | `ListOpenChanges` (op 23), which landed with `deskboard`'s `fetchOpenPRs` |

Both are READ-only display annotations under the session's own minted App token (not a write), so
no token-custody ruling gated them. The forge-CLI ceiling came down 9 → 7. The one remaining
naive-grep hit — `repohardenguard`'s `ghRun` — is **open work, not a ruled exception**: the driver
ratified closure-to-zero (option B on #834), so every residual `gh` shell-out must reach the seam.
That site is owned by the guard-read-custody brief (`forge-gitlab/11`) and its design record; its
`forgeban` permit row is permitted only until that brief lands, and row 3 closes to 0 with it.

The rest are classified, not migrated. Every one is blocked on a decision this brief does not own:

| Class | Call sites | What blocks the migration |
|-------|-----------|---------------------------|
| **identity** | `deskclose`, `deskdigest`, `deskfile`, `deskflip` (7 sites), `deskreply`, `deskpr`, `deskboard`, `deskmerge`, `deskdisposition`, `issueboard`, `scanloop` | Each reaches the forge under the caller's AMBIENT CLI credential BY DOCUMENTED DESIGN ("gates WHETHER and WHAT, never WHO … mints no App token on any path"). Both backends REFUSE to build a client without an explicitly minted token — deliberately (brief 07's posture, mirroring #562/#563). Routing these through the seam therefore changes WHO performs each write. That is a **token-custody ruling**, not a transport change, and it is the single decision gating ~20 of the 25 sites. (`deskroster` was here — its two reads migrated under #834; see the follow-up table above.) |
| **no enumerated op** | labels (`deskdispatch`, `deskflip`, `deskdisposition`, `scanloop`), `pr list` (`deskdisposition`), branch→PR resolution (`deskpushguard`), issue listing + GraphQL counts (`issueboard`, `deskboard`), merge-authority read (`deskmerge`), trust-association read (`scanloop`) | Spec §6's freeze rule forbids adding a method without converting its consuming call site in the same change. Each of these is a real op set with a real GitLab mapping question (project-scoped labels, MR source-branch lookup, issue IID sequences) and needs its own brief rather than a speculative method. (`deskroster`'s `pr list` was here — migrated onto the now-landed `ListOpenChanges` under #834.) |
| **not a forge op at all** | `deskadvisory` (`gh auth token`), `repohardenguard` (`gh api` reads of rulesets / branch protection / App permissions) | The first is the identity layer (delta D2); the second is repo HARDENING, the same class delta D3 keeps out of the frozen set. Neither has a Forge method it could move to *today* — under the ratified closure-to-zero both are OPEN WORK, `repohardenguard` under the guard-read-custody brief (`forge-gitlab/11`), not permanent carve-outs. |

The 14 unresolved-argv sites are a **could-not-check ledger, not a permit**: each runs a resolved
`statusgen`/`desktoken`/callout binary or a caller-supplied argv, none launches a forge CLI on any
path the checker can reach, and none can be PROVEN not to — so they are recorded as what they are.
`askassay`'s entry carries a correction to the brief's own premise: the brief describes its `gh`
entry as a vestigial binary-present probe that can simply be dropped, and the tree does not agree —
`readOnlyBinaries["gh"]` backs two live registry questions through a default-deny read-only guard,
so dropping it removes two answers rather than a dead check. Re-sourcing those answers through the
interface is a brief of its own.

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

`forge_github_golden_test.go` (`TestForgeGithubGolden`) pins one case per operation (plus the
`read_file` / `write_file_*` cases added with the file ops) — request
method/path/query, write bodies, pagination (`per_page=100`, multi-page walk, short-page stop),
result mapping, and error classification (404 → `IsForgeNotFound`; 403 → `ForgeAPIError`).
`TestForgeGithubGoldenCount` guards the floor (≥ 10). Regenerate on an INTENTIONAL change with
`-update`; an unintentional wire change shows as a golden diff — the single-point-of-failure
control the brief names, independent of the per-tool tests.
