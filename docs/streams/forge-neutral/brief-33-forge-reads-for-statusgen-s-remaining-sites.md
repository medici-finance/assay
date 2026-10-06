---
brief: assay:assay:forge-neutral:33
title: Forge reads for statusgen's remaining sites — four operations and their result fields, each consumed by a deskread kind
why: >-
  statusgen still runs `gh` at 31 places, and forge-neutral/18 cannot finish because some of
  them ask the forge something the seam has no way to ask: a closed issue's closer, a change's
  own commits, its merge commit, its author, whether it came from a fork. Until those reads exist
  on the seam, with a GitLab mapping and a refusal where GitLab cannot answer, those statusgen
  checks only work on GitHub, and only through an ambient CLI login. Adding exactly these reads,
  and nothing more general, is what lets 18 move the remaining sites without narrowing its own
  completion test.
wave: 1
depends: []
unblocks: ["forge-neutral/18"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
outcome: none
id: 51454ffd-9b06-442d-ae92-42909529649d
authored: 2026-10-06 by forge-neutral authoring session (the #2025 routing)
sources:
  - "#2025 comment 5945992401: the precheck of forge-neutral/18's remaining slices. It stopped before building because at least five of the 31 sites need a read the frozen surface lacks (change commits, change author, cross-repo head, all-state issues with timestamps), and it named the `gh auth token` site as a credential and not a read"
  - "#2025 comment 5946009835: the routing this brief implements. The default was that 18's goal stands and a separate brief adds the missing reads, with 18's row 3 depending on it and not narrowed. That brief is authoring work that arrives as its own draft PR, where the choice can be declined at merge"
  - "#2025 comment 6013927144: 18's progress note (#2312 meets row 13 and removes one site, 31 → 30). It lists the reads still missing: closed-state lists with closedAt, merge-commit SHA, PR commits, file patches, timeline events, check rollup and closed_by. This brief re-checks that list against code; check rollup is already served (see Context)"
  - "#2253 and its comment 6013956713: the CI read-identity question (which credential the CI-only statusgen modes read under). It is a SEPARATE pending decision. This brief neither decides it nor depends on it; see Context, 'Why this brief is independent of #2253'"
  - "#2312 (draft): forge-neutral/18's in-flight PR. It deletes the `doratiming.go` repo-view fallback and leaves every site this brief serves untouched"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: its ground rule 'Add no operation to `Forge` … a later call site that genuinely needs one … is a separate brief under the freeze rule' (this is that brief), its Verify row 3 (zero `exec.Command(\"gh\"` in non-test statusgen) and its row 15 (`TestForgeSurfaceUnchangedByDeskread`)"
  - "docs/streams/forge-neutral/README.md, 'Shared conventions the briefs inherit': refusal not fallback; the surface stays closed and a new operation joins the frozen inventory with its consuming verb; negative-path rows are mandatory; no hand-built API call is evidence"
  - "docs/streams/forge-gitlab/inventory.md: the frozen method-set table (rows 1–54 at the freshness base) that `TestForgeGitlabCoverage` reconciles the seam against (`tools/desk/internal/deskkit/forge_gitlab_test.go:2605-2672`, discovery of `docs/streams/*/inventory.md`)"
  - "tools/desk/internal/deskkit/forge.go: the `Forge` interface (`:1370`), `Account` (`:65`), `PullRequest` (`:86`), `Issue` (`:286`), `ChangedFile` (`:330`), `ChangeRef` (`:853`), `IssueSummary` (`:890`)"
  - "tools/desk/cmd/deskread/main.go: the closed `readKinds` set (`:74`), `perIssueKinds` (`:81`), envelope schema 1, and the minted-App-role identity (`ciEligible=false`)"
  - "freshness-checked 2026-10-06 @ 11228951d (origin/main): 31 matches of row 3's grep across 16 non-test statusgen files, plus one `exec.CommandContext(ctx, \"gh\", …)` at `autonomy.go:539` that the grep does not match. `allowedInvocationCeiling = 6` (`tools/desk/internal/forgeban/allowlist.go:94`). The Forge interface has 54 methods"
exec-tier: strong
exec-tier-why: >-
  Question (c), silent failure toward a pass. Almost every field added here has an "absent"
  value that a careless mapping turns into a confident wrong answer: an empty actor type that
  reads as a human, a test-merge SHA on an open change that reads as its merge commit, a 250-commit
  page that reads as the whole branch, a missing patch that reads as an empty change, a page-capped
  issue list that reads as the population. Each one passes a happy-path golden.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/forge.go: follow-up forge-neutral/33 (this brief's implementation: the four operations and the result fields of Task 1–2)"
  - "tools/desk/internal/deskkit/forge_github.go, tools/desk/internal/deskkit/forge_gitlab.go: follow-up forge-neutral/33 (this brief's implementation: both backends of every addition)"
  - "tools/desk/internal/deskkit/forge_github_golden_test.go, tools/desk/internal/deskkit/forge_gitlab_test.go, tools/desk/internal/deskkit/testdata/: follow-up forge-neutral/33 (this brief's implementation: golden cases on both backends)"
  - "tools/desk/internal/deskkit/forge_surface_deskread_test.go: follow-up forge-neutral/33 (this brief's implementation: its `want` list gains exactly the four new names, in this change, as the test's own failure message directs)"
  - "tools/desk/cmd/deskread: follow-up forge-neutral/33 (this brief's implementation: the new read kinds that consume each addition)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/33 (this brief's implementation: inventory rows 55–58 and an 'added by' note)"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: fixed-here (its `depends:` gains this brief, so the edge the #2025 routing promised is in the graph)"
  - "statusgen/autoflip.go, statusgen/autonomy.go, statusgen/briefdecision.go, statusgen/briefflowreview.go, statusgen/claimdecay.go, statusgen/corroborate.go, statusgen/decisiongateanchor.go, statusgen/issues.go, statusgen/selfimprovement.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go: follow-up forge-neutral/18 (moving each site onto `deskread` is 18's Task and Verify row 3; this brief adds the reads and touches no statusgen file)"
  - "statusgen/decisionruling.go: out-of-scope (`:643` runs `gh auth token`. That is credential acquisition, not a read, and where a CI or local statusgen run gets its credential is the question #2253 holds open)"
  - "statusgen/ghfetch.go: out-of-scope (statusgen's own native HTTP client, which row 3's grep does not see; it is not a forge-CLI site and is not in this brief's census)"
  - ".github/workflows/assay-statusgen.yml: out-of-scope (the CI read identity, which is #2253's decision)"
---

# Brief 33 — Forge reads for statusgen's remaining sites

## Context

forge-neutral/18 moves statusgen off `gh` and onto `deskread`, the seam's read half packaged
as a process. Its ground rule is that **18 adds no operation to `Forge`**: every read it moves
must already exist on the seam with both backends. 18's precheck on #2025 found that this is
false for part of the remaining work. Some sites ask for a read, or a result field, that the
frozen surface does not carry, so 18's Verify row 3 (zero `exec.Command("gh"` in non-test
statusgen) cannot reach 0 under 18's own rules. The routing on #2025 kept 18's goal, did not
narrow row 3, and made the missing reads a separate brief. This is that brief.

The brief **adds the reads and stops**. It adds the operations and fields to `deskkit`, gives
each a GitLab mapping (or a stated could-not-check), pins both backends with goldens, records
the operations in the frozen inventory, and exposes each addition through a new `deskread`
kind, which is the consuming call site the freeze rule requires. It moves no statusgen site;
that stays 18's job.

files:
- `tools/desk/internal/deskkit/forge.go`: the `Forge` interface (`:1370`) and the result types
  named in Task 1–2.
- `tools/desk/internal/deskkit/forge_github.go`, `tools/desk/internal/deskkit/forge_gitlab.go`:
  both backends of every addition.
- `tools/desk/internal/deskkit/forge_github_golden_test.go`,
  `tools/desk/internal/deskkit/forge_gitlab_test.go`,
  `tools/desk/internal/deskkit/testdata/forge_golden/`,
  `tools/desk/internal/deskkit/testdata/forge_gitlab_golden/`: golden cases on recorded
  fixtures.
- `tools/desk/internal/deskkit/forge_surface_deskread_test.go`: the method-set `want` list.
- `tools/desk/cmd/deskread/main.go`, `tools/desk/cmd/deskread/forge.go`, and the deskread
  tests: the new kinds.
- `docs/streams/forge-gitlab/inventory.md`: rows 55–58.

facts:
- The seam today has 54 methods. Among the reads statusgen's sites need, these already exist:
  - `GetPullRequest` (base ref, changed-file count, head, state, merged time);
  - `ListChanges(states)` (bounded, `Incomplete` on overflow);
  - `ListOpenIssues` (open only);
  - `ReviewsAtHead`, `ChecksAtHead(sha)` and `ListChangedFiles` (names and renames, no patch);
  - `ListCommentsTyped` (with the author type from GraphQL `__typename`);
  - `IssueTrustEvents` and `PRTrustEvents` (with `BodyEdited`);
  - `ListCommitChanges(sha)` (change numbers only) and `ChangeDiff` (GitHub `pr diff`).
- Missing:
  - any read of a closed or all-state issue list, of an issue's closer, or of its close/reopen
    timeline;
  - a change's own commit list;
  - a repository's default branch;
  - a change's merge-commit SHA;
  - `ChangeRef`'s author, base and fork facts;
  - per-file patches;
  - the issue author's type on GitHub `GetIssue` (`forge_github.go:549` builds
    `Account{Login, ID}` and leaves `Type` empty).
- `ChangeDiff` is NOT a substitute for per-file patches. `corroborate.go:1144-1148` records
  that GitHub's `pr diff` fails with 406 above 300 files, while the paginated file list
  reaches 3000. Serving `corroborate.go:1149` from `ChangeDiff` would turn a 301-file change
  into a could-not-check that is green today.
- `deskread` takes only the kinds in its closed `readKinds` map (`main.go:74`). It
  authenticates only as a minted App role (`ciEligible=false`). Its envelope is schema 1, and
  a pinned consumer refuses an unrecognised schema. Adding a kind is not a schema change.
- `TestForgeSurfaceUnchangedByDeskread` (`forge_surface_deskread_test.go:35`) pins the method
  set by name. Its own failure message says a legitimate surface change updates `want` "in
  THAT change". This brief is that change; 18's own diff still must not move it.

### Why the risk answers are all "no" although the paths are under `deskkit`

`tools/desk/internal/deskkit/` is a security-path trigger, so the four answers were checked
against the paths and not just copied from 18:
- Every addition is a **read** of forge metadata that the reading identity can already see.
- No addition mints, stores, forwards or widens a credential, and none writes to a forge.
- No addition touches a payment, customer record or irreversible action.

That is the same answer as the stream's other read briefs (06, 12, 18). Several fields feed
controls once 18 consumes them: actor type in trust and transcription, merge commit and branch
commits in the auto-flip, cross-repo in claim decay. That is why their **absent** values are
specified, and why Verify rows 5–10 test the negative path for each. If a reviewer reads any of
these as a control change rather than a read, the right move is to flip `sensitive-data` and gate
this brief on a human, not to drop the rows.

### Why this brief is independent of #2253

#2253 asks which credential statusgen's CI-only modes should read under: the workflow token
or an App identity. Those modes are `--corroborate`, `--auto-flip-model` and the transcribe
lanes. This brief **does not decide that and does not depend on it**:

- It changes no workflow file.
- It does not change `deskread`'s identity resolution or its `ciEligible` stance.
- It mints nothing.

Every read it adds is the same forge-neutral read under any answer:

- **(a)** an opt-in workflow-token transport for `deskread`, or **(b)** an App token in CI:
  the new kinds are exactly what those CI modes then call.
- **(c)** row 3 narrowed to exclude CI-only modes: the reads still serve every non-CI site in
  the table below (claim decay, the issue metrics, decision latency, autonomy authorship,
  brief-flow review, self-improvement). They also remain the only route for the CI modes to
  run on a GitLab forge at all.

The table's last column says which sites #2253 names, so a reviewer who wants to decline
part of the brief at merge can see where the line falls.

### Site census at the freshness base (`11228951d`)

There are 32 forge-CLI launch sites in non-test statusgen. Each row says what the site reads
and which seam read serves it after this brief. **New** marks this brief's additions;
**exists** means 18 can move the site today with a kind of its own.

| Site | What it reads | Served by | CI mode per #2253 |
|---|---|---|---|
| `doratiming.go:631` | repo name fallback | none (deleted by #2312) | — |
| `transcribescan.go:77` | issue author login/id/**type** | `GetIssueTyped` + **new** author type on GitHub (Task 2.6) | transcribe |
| `transcribescan.go:109` | one comment by URL: author + type + body | exists: `ListCommentsTyped` on the URL's number, select by `DatabaseID` | transcribe |
| `issues.go:577` | **all-state** issues: number, state, createdAt, **closedAt**, author, labels, title | **new** `ListIssues` (op 55) | — |
| `claimdecay.go:63` | all-state changes: head ref, state, **cross-repo, head repository** | `ListChanges` + **new** `ChangeRef.CrossRepo`/`HeadRepo` | — (plain `--lint`) |
| `decisiongateanchor.go:229` | issue state, body, **closed_by** | `GetIssueTyped` + **new** `Issue.ClosedBy` | `--corroborate` |
| `corroborate.go:863` | base ref | exists: `GetPullRequest.BaseRef` | `--corroborate` |
| `corroborate.go:1149` | file list **with patches** | `ListChangedFiles` + **new** `ChangedFile.Patch` | `--corroborate` |
| `corroborate.go:1173` | changed-file count | exists: `GetPullRequest.ChangedFiles` | `--corroborate` |
| `corroborate.go:1195` | reviews + comments | exists: `ReviewsAtHead` + `ListCommentsTyped` | `--corroborate` |
| `briefdecision.go:41` | label-filtered issues by state, with createdAt, **closedAt** | **new** `ListIssues` (op 55) | — |
| `autonomy.go:491` | merged changes with **author + bot flag** | `ListChanges` + **new** `ChangeRef.Author` | — |
| `autonomy.go:539` (not matched by row 3's grep) | merged changes in a date window, then each one's check rollup | exists: `ListChanges(Merged)` filtered on `MergedAt` (`Incomplete` is could-not-check), then `ChecksAtHead(HeadSHA)` | — |
| `trustgate.go:208` | issue trust events | exists: `IssueTrustEvents` (`deskread trust`) | transcribe |
| `briefflowreview.go:72` | closed changes **into main** with body, merged_at | `ListChanges(Merged)` + **new** `ChangeRef.BaseRef` | — |
| `briefflowreview.go:103` | reviews | exists: `ReviewsAtHead` | — |
| `citationcorroborate.go:435` | issue comments + existence | exists: `ListComments` | — |
| `citationcorroborate.go:461` | change reviews | exists: `ReviewsAtHead` | — |
| `scanissues.go:116` | open issues | exists: `ListOpenIssues` (`deskread issues`) | transcribe |
| `scanissues.go:953` | issue comments with author id/type | exists: `ListCommentsTyped` (`deskread comments`) | transcribe |
| `transcribeverdict.go:506` | issue author login/id/**type**, body, edited | `GetIssueTyped` + **new** author type; edited from `IssueTrustEvents.BodyEdited` | transcribe |
| `transcribeverdict.go:579` | open issues with createdAt | exists: `ListOpenIssues` | transcribe |
| `autoflip.go:1197` | changes behind a commit with **merge-commit SHA** and head | `ListCommitChanges` + `GetPullRequest` + **new** `PullRequest.MergeCommitSHA` | `--auto-flip-model` |
| `autoflip.go:1277` | a change's **own commits** | **new** `ListChangeCommits` (op 57) | `--auto-flip-model` |
| `autoflip.go:1330` | body, changed files, base ref, **repo default branch** | `GetPullRequest` + **new** `RepoDefaultBranch` (op 58) | `--auto-flip-model` |
| `autoflip.go:1341` | file names + renames | exists: `ListChangedFiles` | `--auto-flip-model` |
| `autoflip.go:1389` | merged changes whose body names a brief | exists: `ListChanges(Merged)` + body filter (`Incomplete` is could-not-check) | `--auto-flip-model` |
| `autoflip.go:1415` | body last-edited time | exists: `PRTrustEvents.BodyEdited` | `--auto-flip-model` |
| `autoflip.go:1488` | head, state, merged time | exists: `GetPullRequest` | `--auto-flip-model` |
| `autoflip.go:1498` | reviews with commit id | exists: `ReviewsAtHead` | `--auto-flip-model` |
| `selfimprovement.go:400` | comment authors + types; **close/reopen actors; closing changes (merged, author)** | `ListCommentsTyped` + **new** `IssueStateEvents` (op 56) | — |
| `decisionruling.go:643` | `gh auth token` (a credential, not a read) | out of scope | (#2253) |

**This corrects the list in 18's progress note:** "check rollup" is already served by
`ChecksAtHead` and is not missing.

## Ground rules

- NEVER git push, trigger workflows or run mutating commands against any forge. Commit only
  per the task instructions.
- Stop at `implemented`. You do not set verified or done; a different, non-implementing
  identity does.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Don't guess.
- **Touch no file under `statusgen/` and no workflow file.** Moving the sites is
  forge-neutral/18's Task. The CI read identity is #2253's decision.
- **Do not change `deskread`'s identity.** `ciEligible` stays `false`, there is no
  environment-token path, and there is no ambient-CLI fallback.
- **The surface stays closed.** Each addition is a typed read with a fixed endpoint per
  backend. There is no `--query`/`--path`/`--endpoint` flag, no raw-document return, and no
  generic method. `TestForgeNoPassthrough`, `TestNoForgeCLIShellout` and
  `TestForgeSingleConstructionSite` stay green, and `allowedInvocationCeiling` does not rise.
- **Absent is never a value.** Each field below names its empty meaning. A backend leaves the
  field empty, or returns could-not-check, rather than filling a plausible default. That means
  no "User" for an unknown actor, no "main" for an unknown branch, and no test-merge SHA for an
  unmerged change.
- **Recorded fixtures only.** No live GitHub or GitLab call in the suite, and no `curl`
  evidence.

## Task

1. **Add four operations to `Forge`** (inventory rows 55–58). Each lands with its `deskread`
   kind (Task 3) as its consuming call site, and its doc comment names that consumer and
   forge-neutral/18 as the statusgen consumer to come.
   - **55 `ListIssues(repo, in IssueListQuery) (*IssueList, error)`**
     - Query: `IssueListQuery{State: open|closed|all, Label: string (optional, one label)}`.
       An unknown state is refused.
     - Result: `IssueList{Issues []IssueSummary, Incomplete bool}`, bounded by the same
       page-count ceiling `ListChanges` uses. `Incomplete=true` when the ceiling is hit with
       the forge still paginating.
     - `IssueSummary` gains `State` (`open`/`closed`) and `ClosedAt` (RFC3339, empty while
       open), both `omitempty` so existing goldens stay byte-identical.
     - GitHub: `GET /repos/{o}/{r}/issues?state=…&labels=…&per_page=100`. Drop entries
       carrying `pull_request`, because the issues endpoint lists changes too.
     - GitLab: `GET /projects/:id/issues?state=opened|closed|all&labels=…&per_page=100`, with
       `opened` normalised to `open`.
   - **56 `IssueStateEvents(repo, number) (*IssueStateHistory, error)`**
     - Result: `IssueStateHistory{Events []IssueStateEvent{Kind closed|reopened, Actor Account,
       CreatedAt}, ClosingChanges []ClosingChange{Number, Merged bool, Author Account},
       Complete bool}`.
     - GitHub: one GraphQL read of `timelineItems(itemTypes:[CLOSED_EVENT,REOPENED_EVENT])`
       with actor `login`/`__typename`, plus `closedByPullRequestsReferences(includeClosedPrs:
       true)`. This is the same shape `selfimprovement.go:381-388` sends today. `Complete=false`
       when either connection reports `hasNextPage`.
     - GitLab: `GET /projects/:id/issues/:iid/resource_state_events` (state `closed`/`reopened`,
       `user`, `created_at`) and `GET /projects/:id/issues/:iid/closed_by` (the merge requests
       that close it, with `state` and `author`).
     - Actor and author `Type` follow Task 2.6.
   - **57 `ListChangeCommits(repo, number) (*ChangeCommits, error)`**
     - Result: `ChangeCommits{SHAs []string, Complete bool}`. These are the commits the change
       introduced, in the forge's order. Consumers use membership, not order.
     - GitHub: `GET /repos/{o}/{r}/pulls/{n}/commits?per_page=100`, paginated. The endpoint
       stops at 250 commits, so `Complete` is true only when the listed count equals the change's
       own `commits` count from `GET /pulls/{n}`.
     - GitLab: `GET /projects/:id/merge_requests/:iid/commits`, paginated. `Complete=false` on
       hitting the page ceiling.
   - **58 `RepoDefaultBranch(repo) (string, error)`**
     - GitHub: `GET /repos/{o}/{r}` `.default_branch`.
     - GitLab: `GET /projects/:id` `.default_branch`.
     - An empty value is a could-not-check error, never `"main"`.
     - This is its own operation rather than a `PullRequest` field so that GitLab's
       `GetPullRequest` does not gain a project read on every call.
2. **Add the result fields.** Fields are not methods, so each lands with its `deskread` kind as
   call site. Each is `omitempty`.
   1. **`PullRequest.MergeCommitSHA`**
      - GitHub: `merge_commit_sha`, filled **only when `merged` is true**. GitHub also reports
        a test-merge SHA on open changes, and that must not leak through.
      - GitLab: `merge_commit_sha`, else `squash_commit_sha` when the merge request was
        squashed. Empty otherwise, for example on a fast-forward merge.
      - Empty means could-not-check.
   2. **`ChangeRef.Author`** (`Account`, with `Type` per 2.6):
      - GitHub: GraphQL `author{login __typename}` in `ListChanges`' existing query.
      - GitLab: the merge request's `author`.
   3. **`ChangeRef.BaseRef`**:
      - GitHub: `baseRefName`.
      - GitLab: `target_branch`.
   4. **`ChangeRef.CrossRepo` and `ChangeRef.HeadRepo`**:
      - `CrossRepo` uses the same `CrossRepoSame`/`CrossRepoFork` vocabulary as
        `PullRequest.CrossRepo` (`forge.go:175-183`), empty meaning could-not-check.
      - `HeadRepo` is `owner/name`, empty when unreadable, for example a deleted fork.
      - GitHub: `isCrossRepository` + `headRepository{nameWithOwner}`.
      - GitLab: `source_project_id` vs `target_project_id`. `HeadRepo` is the target path when
        they are equal and empty otherwise.
   5. **`ChangedFile.Patch` and `ChangedFile.PatchAbsent`**:
      - GitHub: the file entry's `patch`. `PatchAbsent=true` when the entry has no `patch` key
        (binary or oversized).
      - GitLab: the diff entry's `diff`. `PatchAbsent=true` when `too_large` or `collapsed`
        is set.
      - An absent patch is stated, never rendered as an empty change.
   6. **The author or actor `Type` where this brief reads one**: `GetIssue`/`GetIssueTyped`'s
      `Issue.Author`, the new `Issue.ClosedBy`, `ChangeRef.Author`, and op 56's actors and
      authors.
      - GitHub: REST `user.type` or GraphQL `__typename`, with the vocabulary unchanged
        ("User", "Bot", …).
      - GitLab: resolve human/bot the way op 6 (`IssueReactions`) already does, from the
        users API, once per distinct account id per call.
      - An account that cannot be resolved keeps `Type` empty, which the `Account` doc
        already defines as could-not-check (`forge.go:68-73`).
   7. **`Issue.ClosedBy`** (`Account`), filled only when the issue is closed:
      - GitHub: `closed_by`.
      - GitLab: `closed_by`.
3. **Add the consuming `deskread` kinds** to `readKinds` (`main.go:74`), each with its own
   addressing flag. Each kind refuses the other addressing flags, exactly as the existing kinds
   do. The envelope stays schema 1, and partial-is-a-result semantics apply unchanged.
   - `issue-list --repo … [--state open|closed|all] [--label <name>]` → op 55.
   - `issue --issue owner/name#N` → `GetIssueTyped` (issue kind), carrying `ClosedBy` and the
     author `Type`.
   - `issue-states --issue owner/name#N` → op 56.
   - `changes --repo … --state merged|closed|all` → `ListChanges`, carrying 2.2–2.4.
   - `change --change owner/name#N` → `GetPullRequest`, carrying `MergeCommitSHA`.
   - `change-commits --change owner/name#N` → op 57.
   - `change-files --change owner/name#N` → `ListChangedFiles`, carrying 2.5.
   - `default-branch --repo …` → op 58.

   Also amend `main.go`'s header and `usage` to fit:
   - Replace "IT ADDS NO OPERATION TO Forge" with the accurate statement: the verb's
     migrations add none, and ops 55–58 were added under the freeze rule by forge-neutral/33
     with these kinds as their consumers.
   - Name forge-neutral/18 as the statusgen consumer that each new kind is waiting for.
4. **Register the surface change.**
   - Add inventory rows 55–58 to `docs/streams/forge-gitlab/inventory.md`, in its existing
     column shape, all `implemented`, plus one "added by forge-neutral brief 33" note naming
     the `deskread` kinds as consumers.
   - Add the four names to `TestForgeSurfaceUnchangedByDeskread`'s `want` list, with a
     one-line comment citing this brief.
   - Do not change `allowedInvocationCeiling`.
5. **Goldens on both backends** for every operation and every field, on recorded fixtures,
   including the negative cases Verify rows 5–10 name.

## Verify (executable — no prose-only DoD items)

The Class column uses the stream's convention:
- `check:ci` runs in CI.
- `check` is a local or grep-level assertion.
- `+dereference` resolves a claim instead of counting presence.
- `+flow` exercises verb → `ForgeFor` → backend.
- `+mutation` is a mutation demonstration.

Test names are this brief's planned deliverables:
- `TestListIssuesIncompleteIsNotAbsence` (planned)
- `TestListChangeCommitsOverCapIsIncomplete` (planned)
- `TestMergeCommitSHAEmptyUnlessMerged` (planned)
- `TestAccountTypeUnresolvedStaysEmpty` (planned)
- `TestRepoDefaultBranchEmptyRefuses` (planned)
- `TestChangedFilePatchAbsentIsStated` (planned)
- `TestIssueStateEventsOverflowIsIncomplete` (planned)
- `TestDeskreadNewKindsRoundTrip` (planned)
- `TestDeskreadNewKindsAddressingRefusals` (planned)

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeGithubGolden$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2a.out" 2>&1 && grep -F -e '--- PASS: TestForgeGithubGolden' "${TMPDIR:-/tmp}/b33-r2a.out" && go test ./internal/deskkit/ -run '^TestForgeGitlabGolden$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2b.out" 2>&1 && grep -F -e '--- PASS: TestForgeGitlabGolden' "${TMPDIR:-/tmp}/b33-r2b.out" && go test ./internal/deskkit/ -run '^TestForgeGitlabCoverage$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2c.out" 2>&1 && grep -F -e '--- PASS: TestForgeGitlabCoverage' "${TMPDIR:-/tmp}/b33-r2c.out"` | exit 0, each `--- PASS:` line printed. Both backends' wire is pinned for ops 55–58 and every Task 2 field, and coverage reconciles the seam against inventory rows 55–58 |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeNoPassthrough$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3a.out" 2>&1 && grep -F -e '--- PASS: TestForgeNoPassthrough' "${TMPDIR:-/tmp}/b33-r3a.out" && go test ./internal/deskkit/ -run '^TestNoForgeCLIShellout$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3b.out" 2>&1 && grep -F -e '--- PASS: TestNoForgeCLIShellout' "${TMPDIR:-/tmp}/b33-r3b.out" && go test ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3c.out" 2>&1 && grep -F -e '--- PASS: TestForgeSingleConstructionSite' "${TMPDIR:-/tmp}/b33-r3c.out"` | exit 0, each `--- PASS:` line printed. The seam grows four typed reads and stays closed |
| 4 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeSurfaceUnchangedByDeskread$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r4.out" 2>&1 && grep -F -e '--- PASS: TestForgeSurfaceUnchangedByDeskread' "${TMPDIR:-/tmp}/b33-r4.out"` | exit 0, `--- PASS:` printed. The pinned method set is the base's 54 plus exactly `ListIssues`, `IssueStateEvents`, `ListChangeCommits` and `RepoDefaultBranch` |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestListIssuesIncompleteIsNotAbsence$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r5.out" 2>&1 && grep -F -e '--- PASS: TestListIssuesIncompleteIsNotAbsence' "${TMPDIR:-/tmp}/b33-r5.out"` | **negative path**: a population beyond the page ceiling returns `Incomplete=true` on both backends, never a short list presented as the whole |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestListChangeCommitsOverCapIsIncomplete$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r6.out" 2>&1 && grep -F -e '--- PASS: TestListChangeCommitsOverCapIsIncomplete' "${TMPDIR:-/tmp}/b33-r6.out"` | **negative path**: a GitHub change whose `commits` count (300) exceeds the listed 250 returns `Complete=false`. GitLab at the page ceiling does the same |
| 7 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestMergeCommitSHAEmptyUnlessMerged$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r7.out" 2>&1 && grep -F -e '--- PASS: TestMergeCommitSHAEmptyUnlessMerged' "${TMPDIR:-/tmp}/b33-r7.out"` | **negative path**: an OPEN GitHub change whose fixture carries a test-merge `merge_commit_sha` reads back `MergeCommitSHA == ""`. A GitLab fast-forward merge with no merge or squash SHA reads back empty |
| 8 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestAccountTypeUnresolvedStaysEmpty$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r8.out" 2>&1 && grep -F -e '--- PASS: TestAccountTypeUnresolvedStaysEmpty' "${TMPDIR:-/tmp}/b33-r8.out"` | **negative path**: a GitLab author whose users-API read fails keeps `Type == ""` on `GetIssue`, `ChangeRef.Author` and op 56, never `"User"` |
| 9 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRepoDefaultBranchEmptyRefuses$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r9a.out" 2>&1 && grep -F -e '--- PASS: TestRepoDefaultBranchEmptyRefuses' "${TMPDIR:-/tmp}/b33-r9a.out" && go test ./internal/deskkit/ -run '^TestIssueStateEventsOverflowIsIncomplete$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r9b.out" 2>&1 && grep -F -e '--- PASS: TestIssueStateEventsOverflowIsIncomplete' "${TMPDIR:-/tmp}/b33-r9b.out"` | **negative path**: an empty `default_branch` is a could-not-check error (`deskkit.Unverifiable`), never `"main"`. A timeline or closing-reference connection with `hasNextPage` returns `Complete=false` |
| 10 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestChangedFilePatchAbsentIsStated$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r10.out" 2>&1 && grep -F -e '--- PASS: TestChangedFilePatchAbsentIsStated' "${TMPDIR:-/tmp}/b33-r10.out"` | **negative path**: a GitHub entry with no `patch` key, and GitLab entries with `too_large`/`collapsed`, read back `PatchAbsent=true`, never an empty `Patch` with `PatchAbsent=false` |
| 11 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/ -run '^TestDeskreadNewKindsRoundTrip$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r11.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadNewKindsRoundTrip' "${TMPDIR:-/tmp}/b33-r11.out"` | exit 0, `--- PASS:` printed. Each of the eight new kinds runs through the recording fake `Forge`, emits schema 1, carries the new fields, and reports an unreadable item in `partial` with exit 0 |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestDeskreadNewKindsAddressingRefusals$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r12a.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadNewKindsAddressingRefusals' "${TMPDIR:-/tmp}/b33-r12a.out" && go test ./cmd/deskread/ -run '^TestDeskreadRefusesUnknownKindAndFlags$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r12b.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadRefusesUnknownKindAndFlags' "${TMPDIR:-/tmp}/b33-r12b.out"` | **negative path**: each new kind given another kind's address flag, an unknown `--state`, or any address-shaped flag outside the closed set exits 5 with zero forge calls |
| 13 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestListChangeCommitsOverCapIsIncomplete$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r13.out" 2>&1 && grep -F -e '--- PASS: TestListChangeCommitsOverCapIsIncomplete' "${TMPDIR:-/tmp}/b33-r13.out"`. **Mutation:** in the GitHub `ListChangeCommits`, drop the count reconciliation so `Complete` is always true, run the command, then restore the file and re-run | exit 0 unmutated. Exit **1** on the mutant (no `--- PASS:` line), with the test naming the 250-of-300 case |
| 14 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestMergeCommitSHAEmptyUnlessMerged$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r14.out" 2>&1 && grep -F -e '--- PASS: TestMergeCommitSHAEmptyUnlessMerged' "${TMPDIR:-/tmp}/b33-r14.out"`. **Mutation:** in the GitHub `GetPullRequest` mapping, fill `MergeCommitSHA` from `merge_commit_sha` unconditionally, run the command, then restore the file and re-run | exit 0 unmutated. Exit **1** on the mutant (no `--- PASS:` line) |
| 15 | check +dereference | `grep -c -E '^[\|] 5[5-8] [\|]' docs/streams/forge-gitlab/inventory.md` | `4`. Rows 55–58 exist in the inventory the coverage test discovers |
| 16 | check | `grep -o 'allowedInvocationCeiling = [0-9]*' tools/desk/internal/forgeban/allowlist.go` | a value ≤ `6`. No forge-CLI permit was added |
| 17 | check | `git diff --name-only "$(git merge-base origin/main HEAD)" HEAD -- statusgen/ .github/workflows/`, run on the brief's implementation branch before merge | empty output. No statusgen site and no workflow moved here; both are 18's and #2253's |
| 18 | check | `statusgen --root . --lint` | `LINT: PASS`, rc 0 |
| 19 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/33 --base "$(git merge-base origin/main HEAD)"`, on the implementation branch | exit 0. Every `consumers:` claim is corroborated against the diff |
| 20 | check | GitLab live row | **could-not-check by design**: this repository has no CI-reachable GitLab project. Rows 2 and 5–10 pin the GitLab mapping against recorded fixtures; a live read is the GitLab conformance work's job, not this brief's |

## Named mutations

- **M1**, row 13: `ListChangeCommits` always reports complete. This is the 250-commit cap read
  as the whole branch, which would let the auto-flip owner check credit a change for a commit
  that lies beyond the page.
- **M2**, row 14: `MergeCommitSHA` filled on an open change. A test-merge SHA would then match
  a commit no merge produced.

## Pre-mortem → detection map

*"This shipped and was wrong. What went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| A GitLab account the users API cannot resolve reads as `"User"`, so a bot's closing action or comment counts as a human's | row 8 |
| GitHub's test-merge SHA on an open change is returned as its merge commit | rows 7, 14 |
| The 250-commit cap or a page-capped list reads as the whole population | rows 5, 6, 13 |
| A binary or oversized file's missing patch reads as "no change in this file" | row 10 |
| An unknown default branch defaults to `main`, so a change into a non-default branch is credited | row 9 |
| The `ListIssues` GitHub mapping keeps pull requests in the issue list, inflating issue metrics | row 2 (the golden fixture carries a `pull_request` entry that must not appear) |
| A new kind grows a generic `--query`/`--path` flag, reopening the passthrough | rows 3, 12 |
| This brief quietly starts moving statusgen sites or edits a workflow, pre-deciding #2253 | row 17; Review reads the diff |
| `TestForgeSurfaceUnchangedByDeskread` is weakened (`want` loosened, or the test skipped) instead of extended by four names | row 4; Review reads the test diff |
| The inventory is not updated, so coverage reconciles against the interface only | rows 2, 15 |
| A live forge call is made during implementation or verification | **no row**. That is a ground-rules violation, caught only by review of the commands actually run |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter; all four risk answers no). The reviewer records the verdict
and date in the stream README table. The reviewer also confirms two things from the diff:
- no file under `statusgen/` or `.github/workflows/` changed;
- `deskread`'s identity resolution is untouched.

Together these keep this brief independent of #2253.
