---
brief: assay:assay:forge-neutral:33
title: Forge reads for statusgen's remaining sites — four operations and their result fields, each consumed by a deskread kind
why: >-
  statusgen still runs `gh` at 32 places, and forge-neutral/18 cannot finish because some of
  them ask the forge something the seam has no way to ask: a closed issue's closer, a change's
  own commits, its merge commit, its author, whether it came from a fork. Until those reads exist
  on the seam, with a GitLab mapping and a refusal where GitLab cannot answer, those statusgen
  checks only work on GitHub, and only through an ambient CLI login. Adding exactly these reads,
  and nothing more general, is what lets 18 move the remaining sites without narrowing its own
  completion test.
wave: 1
depends: []
unblocks: ["forge-neutral/18", "forge-neutral/35"]
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
  - "#2253 and its comment 6013956713: the CI read-identity question (which credential the CI-only statusgen modes read under). It is a SEPARATE decision, ruled (option a, comment 6014356117) after this brief was drafted; the CI-transport brief in #2314 carries that ruling's work. This brief neither decides it nor depends on it; see Context, 'Why this brief is independent of #2253'"
  - "#2312 (draft): forge-neutral/18's in-flight PR. It deletes the `doratiming.go` repo-view fallback and leaves every site this brief serves untouched"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: its ground rule 'Add no operation to `Forge` … a later call site that genuinely needs one … is a separate brief under the freeze rule' (this is that brief), its Verify row 3 (zero non-comment `\"gh\"` literals in non-test statusgen) and its row 15 (`TestForgeSurfaceUnchangedByDeskread`)"
  - "docs/streams/forge-neutral/README.md, 'Shared conventions the briefs inherit': refusal not fallback; the surface stays closed and a new operation joins the frozen inventory with its consuming verb; negative-path rows are mandatory; no hand-built API call is evidence"
  - "docs/streams/forge-gitlab/inventory.md: the frozen method-set table (55 method rows numbered 1–54 at the freshness base, because number 27 is used twice: `IssueContentEvents` at `:78` and `ListRecentCommits` at `:96`; the new rows are 55–58) that `TestForgeGitlabCoverage` reconciles the seam against (`tools/desk/internal/deskkit/forge_gitlab_test.go:2605-2672`, discovery of `docs/streams/*/inventory.md`)"
  - "tools/desk/internal/deskkit/forge.go: the `Forge` interface (`:1370`), `Account` (`:65`), `PullRequest` (`:86`), `Issue` (`:286`), `ChangedFile` (`:330`), `ChangeRef` (`:853`), `IssueSummary` (`:890`)"
  - "tools/desk/cmd/deskread/main.go: the closed `readKinds` set (`:74`), `perIssueKinds` (`:81`), envelope schema 1, and the minted-App-role identity (`ciEligible=false`)"
  - "freshness-checked 2026-10-06 @ 11228951d (origin/main): 32 forge-CLI launch sites in non-test statusgen: 31 `exec.Command(\"gh\"` matches across 16 files plus one `exec.CommandContext(ctx, \"gh\", …)` at `autonomy.go:539`; forge-neutral/18 row 3's widened grep counts all 32. `allowedInvocationCeiling = 6` (`tools/desk/internal/forgeban/allowlist.go:94`). The Forge interface has 55 methods (`forge.go:1370`, `PushTransportHint` included), matching the inventory's 55 rows. `forgeMaxIssuePages = 100` × `forgeIssuePerPage = 100` (`forge_github.go:667-672`); `forgeListChangesMaxPages = 5` × 100 (`:680-681`); `gitlabMaxIssuePage = 25` (`forge_gitlab.go:450`)"
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
  - "tools/desk/cmd/deskread: follow-up forge-neutral/33 (this brief's implementation: the new read kinds that consume each addition, and the existing `comments` kind's new fields and its change target, `--change owner/name#N`, which reads `ListCommentsTyped(…, TargetChange)`)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/33 (this brief's implementation: inventory rows 55–58 and an 'added by' note)"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md: fixed-here (its `depends:` gains this brief, so the edge the #2025 routing promised is in the graph; its `forge.go` consumers entry and its 'Add no operation' ground rule name this brief as the source of the new reads; Verify row 3 becomes a count of non-comment `\"gh\"` literals, which also counts the `exec.CommandContext` launch, and its Expect is re-measured to 31 (32 less the `decisionruling.go` site the path filter below excludes); row 15's Expect moves from 7 to 6 to match `allowedInvocationCeiling`; §6 gains a re-measure note pointing at this census; row 3 excludes `statusgen/decisionruling.go` by path, because the ruling resolver and its `gh auth token` fallback at `:643` move in forge-neutral/35, and its completion Expect is `2`, the lines in `transcribescan.go`'s `ghCommentResolver` and `transcribeverdict.go`'s `ghVerdictIssueResolver` that new rows 19 and 20 pin and 35 also moves; 18's risk note, ground rules, Task 6 and Review name both resolvers and the verdict-issue read as 35's, and Task 6 gains a table of the control-feeding reads 18 moves with each one's old and new signal source; 18 adds `issue` to `deskread`'s CI-transport kind set)"
  - "docs/streams/forge-neutral/brief-35-ruling-resolver-onto-the-read-verb.md: fixed-here (new brief, split from 18 on review: it moves both human-ruling resolvers onto the read verb. The ruling resolver's two reads move onto the `comments` and `issue` kinds this brief adds, and `rulingForgeClient` goes with its `gh auth token` fallback at `:643`, with no replacement credential. The transcribe lanes' sign-off resolver, `ghCommentResolver` in `transcribescan.go`, moves onto the `comments` kind's new fields and its change target, so a sign-off posted on a pull request keeps arming. The verdict-issue read, `ghVerdictIssueResolver` in `transcribeverdict.go`, moves onto the `issue` and `trust` kinds; its old source never set the edit flag, so that move turns the edited-issue refusals on. It is human-gated because each move rewrites the inputs of a control)"
  - "statusgen/autoflip.go, statusgen/autonomy.go, statusgen/briefdecision.go, statusgen/briefflowreview.go, statusgen/claimdecay.go, statusgen/corroborate.go, statusgen/decisiongateanchor.go, statusgen/issues.go, statusgen/selfimprovement.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go: follow-up forge-neutral/18 (moving each site onto `deskread` is 18's Task and Verify row 3; this brief adds the reads and touches no statusgen file. `transcribescan.go`'s `ghCommentResolver`, `transcribeverdict.go`'s `ghVerdictIssueResolver` and the call at `corroborate.go:1481` are forge-neutral/35's)"
  - "statusgen/decisionruling.go: follow-up forge-neutral/35 (the ruling resolver reads one issue comment by id at `:262` and one issue by number at `:354` through statusgen's own HTTP client, whose token comes from the environment or, failing that, from `gh auth token` at `:643`. 35 moves both reads onto `deskread`'s `comments` and `issue` kinds and deletes `rulingForgeClient` and its fallback with no replacement, so `:643` disappears with the move. This brief adds the comment fields those reads consume, Task 2.8 and Task 3, and touches no statusgen file)"
  - "statusgen/transcribescan.go `ghCommentResolver`: follow-up forge-neutral/35 (the transcribe lanes' sign-off resolver at `:109` reads one comment by URL; 35 moves it onto `deskread comments`, selecting by `databaseId`, with the `databaseId` and `authorType` fields this brief adds to the kind in Task 3, and with the issue or change target the URL's path names, through the change target this brief also adds in Task 3. This brief touches no statusgen file)"
  - "statusgen/transcribeverdict.go `ghVerdictIssueResolver`: follow-up forge-neutral/35 (the verdict-issue read at `:506` feeds the verdict and scan-delta lanes' author pins and edited-issue refusals; 35 moves it onto `deskread issue`, with the author type this brief adds, and `deskread trust` for the edit time, and owns every mapping of those values to an outcome. This brief touches no statusgen file)"
  - "statusgen/ghfetch.go: out-of-scope (statusgen's own native HTTP client, which row 3's grep does not see; it is not a forge-CLI site and is not in this brief's census)"
  - ".github/workflows/assay-statusgen.yml: out-of-scope (the CI read identity, which is #2253's decision)"
---

# Brief 33 — Forge reads for statusgen's remaining sites

## Context

forge-neutral/18 moves statusgen off `gh` and onto `deskread`, the seam's read half packaged
as a process. Its ground rule is that **18 adds no operation to `Forge`**: every read it moves
must already exist on the seam with both backends. 18's precheck on #2025 found that this is
false for part of the remaining work. Some sites ask for a read, or a result field, that the
frozen surface does not carry, so 18's Verify row 3 (zero forge-CLI launches, `exec.Command` or
`exec.CommandContext`, in non-test statusgen) cannot reach 0 under 18's own rules. The routing on #2025 kept 18's goal, did not
narrow row 3, and made the missing reads a separate brief. This is that brief.

That routing is a **reversible default, not a ruling**: widening the surface was chosen over
narrowing 18's row 3 to the sites the existing surface serves. It is declared in this PR's
`## Desk-decided` block, with the alternative and the cost of reversing it, so it can be
declined at merge.

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
- The seam today has 55 methods. Among the reads statusgen's sites need, these already exist:
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
    `Account{Login, ID}` and leaves `Type` empty);
  - a comment's last-update time. `Comment` (`forge.go:552`) carries `CreatedAt` but no
    update time, and the GitHub comment query does not select `updatedAt`. The ruling
    resolver (`decisionruling.go:296-307`) refuses an edited ruling comment by comparing the
    two;
  - on `deskread comments`, the comment's `databaseId`, author type and URL. `CommentJSON`
    (`deskread/main.go:432-437`) emits only author login and id, `createdAt` and body, so the
    sites that select one comment by id or read the author type (`transcribescan.go:109`,
    `scanissues.go:953`, `decisionruling.go:262`) cannot be served by the kind as it stands;
  - on `deskread comments`, a change's conversation thread. The kind reads the issue target
    only (`deskread/main.go:15`, `:526`), so a comment posted on a pull request cannot be read
    through it, although `ListCommentsTyped` already takes `TargetChange` on both backends
    (`forge.go:989-990`).
- No `Forge` method reads one comment by id. The house pattern is `ListCommentsTyped` on the
  comment's issue, selecting by `DatabaseID`: that read returns the whole thread or an error,
  so an id that is not in a complete thread is an observed absence, not a could-not-check.
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

That is the same answer as the stream's other read briefs (06, 12, and 18, which moves
statusgen's other sites onto these reads, each keeping the signal its control reads, and
leaves both human-ruling resolvers to forge-neutral/35). Several fields feed
controls once a consumer reads them: actor type in trust and transcription, merge commit and
branch commits in the auto-flip, cross-repo and head repository in claim decay (the fork-spoof
guard, `claimdecay.go:126-170`, which treats a missing signal as unattributed), the issue closer
in the decision-gate anchor (`decisiongateanchor.go:157`, `:226-245`), the closing change's merge
state in self-improvement's human-touch count (`selfimprovement.go:492-501`), and a comment's
update time and author type in the ruling resolver (`decisionruling.go:296-313`). That is why
their **absent** values are specified, and why Verify rows 5–10 and 21–29 test the negative
path for each.

Three consumers' moves rewrite a control's inputs, and forge-neutral/35 makes all three; it
is human-gated (`sensitive-data: yes`) for them. In the ruling resolver, the deleted comment, the
comment-to-issue binding, the edit check and the bot check all change source. The transcribe
lanes' sign-off resolver (`transcribescan.go:109`), which both enactment gates read their
sign-off check from, goes from one comment read by id to a thread read selected by
`databaseId`, and its author type comes from the kind's new field; the URL's path names the
issue or change target it reads, through the change target Task 3 adds. The verdict-issue read
(`transcribeverdict.go:506`), behind the verdict and scan-delta lanes' author pins and
edited-issue refusals, never set its edit flag from its old source, so its move turns those
refusals on. This brief stays
model-gated because it changes no consumer: it adds reads and specifies what each absent value
means, and the decision about how a control uses them sits with the brief that wires it. This brief adds
transport only: kinds, targets and envelope fields. Every mapping of an author type or an edit
flag to an accept-or-refuse outcome stays in forge-neutral/35. If a
reviewer reads any of these fields as a control change in itself rather than a read, the right
move is to flip `sensitive-data` and gate this brief on a human, not to drop the rows.

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
| `transcribescan.go:109` | one comment by URL: author + type + body | owned by forge-neutral/35 (it is the transcribe lanes' sign-off resolver, `ghCommentResolver`, split from 18 with the ruling resolver because both enactment gates read their sign-off check from it): exists: `ListCommentsTyped` on the URL's number, on the issue or change target the URL's path names, select by `DatabaseID`, with the `comments` kind's **new** id and type fields and its **new** change target (Task 3) | transcribe |
| `issues.go:577` | **all-state** issues: number, state, createdAt, **closedAt**, author, labels, title | **new** `ListIssues` (op 55), which lists issues only (Task 1, op 55) so its ceiling is spent on issues. Today's `--limit 1000` truncates silently; the new read serves up to 10,000 and says `Incomplete` beyond that | — |
| `claimdecay.go:63` | all-state changes: head ref, state, **cross-repo, head repository** | `ListChanges` + **new** `ChangeRef.CrossRepo`/`HeadRepo`. `ListChanges` walks at most 500 changes (`forgeListChangesMaxPages` × 100) and reports `Incomplete` beyond; this repository is past that, so 18 must treat the overflow as could-not-check (a follow-up for 18, not this brief) | — (plain `--lint`) |
| `decisiongateanchor.go:229` | issue state, body, **closed_by** | `GetIssueTyped` + **new** `Issue.ClosedBy` | `--corroborate` |
| `corroborate.go:863` | base ref | exists: `GetPullRequest.BaseRef` | `--corroborate` |
| `corroborate.go:1149` | file list **with patches** | `ListChangedFiles` + **new** `ChangedFile.Patch` | `--corroborate` |
| `corroborate.go:1173` | changed-file count | exists: `GetPullRequest.ChangedFiles` | `--corroborate` |
| `corroborate.go:1195` | reviews + comments | exists: `ReviewsAtHead` + `ListCommentsTyped` | `--corroborate` |
| `briefdecision.go:41` | label-filtered issues by state, with createdAt, **closedAt** | **new** `ListIssues` (op 55) | — |
| `autonomy.go:491` | merged changes with **author + bot flag** | `ListChanges` + **new** `ChangeRef.Author` | — |
| `autonomy.go:539` (an `exec.CommandContext` launch, counted by row 3's widened grep) | merged changes in a date window, then each one's check rollup | exists: `ListChanges(Merged)` filtered on `MergedAt` (`Incomplete` is could-not-check), then `ChecksAtHead(HeadSHA)` | — |
| `trustgate.go:208` | issue trust events | exists: `IssueTrustEvents` (`deskread trust`) | transcribe |
| `briefflowreview.go:72` | closed changes **into main** with body, merged_at | `ListChanges(Merged)` + **new** `ChangeRef.BaseRef` | — |
| `briefflowreview.go:103` | reviews | exists: `ReviewsAtHead` | — |
| `citationcorroborate.go:435` | issue comments + existence | exists: `GetIssue` (existence and kind) + `ListCommentsTyped` (the full thread) | — |
| `citationcorroborate.go:461` | change reviews | exists: `ReviewsAtHead` | — |
| `scanissues.go:116` | open issues | exists: `ListOpenIssues` (`deskread issues`) | transcribe |
| `scanissues.go:953` | issue comments with author id/type | exists: `ListCommentsTyped` (`deskread comments`) | transcribe |
| `transcribeverdict.go:506` | issue author login/id/**type**, body, edited | owned by forge-neutral/35 (the verdict-issue read, `ghVerdictIssueResolver`: its old source never set the edit flag, so the move turns the edited-issue refusals on): `GetIssueTyped` (`deskread issue`) + **new** author type, then edited from `IssueTrustEvents.BodyEdited` (`deskread trust`), read after the issue | transcribe |
| `transcribeverdict.go:579` | open issues with createdAt | exists: `ListOpenIssues` | transcribe |
| `autoflip.go:1197` | changes behind a commit with **merge-commit SHA** and head | `ListCommitChanges` + `GetPullRequest` + **new** `PullRequest.MergeCommitSHA` | `--auto-flip-model` |
| `autoflip.go:1277` | a change's **own commits** | **new** `ListChangeCommits` (op 57) | `--auto-flip-model` |
| `autoflip.go:1330` | body, changed files, base ref, **repo default branch** | `GetPullRequest` + **new** `RepoDefaultBranch` (op 58) | `--auto-flip-model` |
| `autoflip.go:1341` | file names + renames | exists: `ListChangedFiles` | `--auto-flip-model` |
| `autoflip.go:1389` | merged changes whose body names a brief | exists: `ListChanges(Merged)` + body filter (`Incomplete` is could-not-check). Same 500-change window as `claimdecay.go:63`, so on this repository it reads `Incomplete` (a follow-up for 18) | `--auto-flip-model` |
| `autoflip.go:1415` | body last-edited time | exists: `PRTrustEvents.BodyEdited` | `--auto-flip-model` |
| `autoflip.go:1488` | head, state, merged time | exists: `GetPullRequest` | `--auto-flip-model` |
| `autoflip.go:1498` | reviews with commit id | exists: `ReviewsAtHead` | `--auto-flip-model` |
| `selfimprovement.go:400` | comment authors + types; **close/reopen actors; closing changes (merged, author)** | `ListCommentsTyped` + **new** `IssueStateEvents` (op 56) | — |
| `decisionruling.go:643` | `gh auth token`, the token fallback for the ruling resolver's two reads (one comment by id at `:262`, one issue by number at `:354`) | owned by forge-neutral/35 (split from 18 because the move rewrites the ruling control's inputs): both reads move onto `deskread comments` (with **new** `Comment.UpdatedAt` and the kind's id, type and URL fields, Task 2.8 and Task 3) and `deskread issue`, and the fallback is deleted with no replacement, so this line disappears with the move | `--corroborate` |

**This corrects the list in 18's progress note:** "check rollup" is already served by
`ChecksAtHead` and is not missing.

## Ground rules

- NEVER git push, trigger workflows or run mutating commands against any forge. Commit only
  per the task instructions.
- Stop at `implemented`. You do not set verified or done; a different, non-implementing
  identity does.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Don't guess.
- **Touch no file under `statusgen/` and no workflow file.** Moving the sites is
  forge-neutral/18's Task, and forge-neutral/35's for `decisionruling.go`,
  `transcribescan.go`'s `ghCommentResolver` and `transcribeverdict.go`'s
  `ghVerdictIssueResolver`. The CI read identity
  is #2253's decision.
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
- **Transport only; no outcome mapping.** This brief adds kinds, targets and envelope fields.
  It maps no author type and no edit flag into an accept or refuse outcome, and no kind or
  field it adds carries a verdict. Every such mapping, such as the Bot pin, the edited-issue
  refusal and the sign-off author check, stays in forge-neutral/35.

## Task

1. **Add four operations to `Forge`** (inventory rows 55–58). Each lands with its `deskread`
   kind (Task 3) as its consuming call site, and its doc comment names that consumer and
   forge-neutral/18 as the statusgen consumer to come.
   - **55 `ListIssues(repo, in IssueListQuery) (*IssueList, error)`**
     - Query: `IssueListQuery{State: open|closed|all, Label: string (optional, one label)}`.
       An unknown state is refused.
     - A `Label` containing a comma is refused: both backends read a comma as a list of
       labels, so it would silently widen the filter. The label is sent as one encoded query
       value (GraphQL variable on GitHub, URL-encoded `labels` parameter on GitLab), never
       spliced into a path or query string by hand.
     - Result: `IssueList{Issues []IssueSummary, Incomplete bool, PageCap int}`.
       `Incomplete=true` when the page ceiling is hit with the forge still paginating.
     - **The ceiling is spent on issues only.** The REST `GET /repos/{o}/{r}/issues` endpoint
       is NOT used: it returns changes as well, so on a repository with more changes than
       issues a page ceiling is consumed by entries the read then drops, and `issues.go:577`
       would become a permanent could-not-check. Each backend below excludes changes on the
       server side.
     - `IssueSummary` gains `State` (`open`/`closed`) and `ClosedAt` (RFC3339, empty while
       open), both `omitempty` so existing goldens stay byte-identical.
     - GitHub: the GraphQL `repository.issues(states:[OPEN|CLOSED], labels:[…], first:100,
       after:…, orderBy:{field:CREATED_AT, direction:DESC})` connection, which lists issues
       and never pull requests. Fields: `number title state createdAt closedAt url
       labels(first:100){nodes{name} pageInfo{hasNextPage}}` and `author{login __typename ... on User{databaseId}
       ... on Bot{databaseId}}`. Ceiling: `forgeMaxIssuePages` (100) pages of 100, so 10,000
       issues, the same ceiling the open-issue walk already uses. A `labels` connection with
       `hasNextPage` on any issue also sets `Incomplete`.
     - GitLab: `GET /projects/:id/issues?state=opened|closed|all&labels=…&per_page=100`,
       which lists issues only (merge requests have their own endpoint), with `opened`
       normalised to `open`. Ceiling: `gitlabMaxIssuePage` (25) pages of 100, so 2,500 issues,
       the same ceiling the GitLab open-issue walk uses. The two ceilings differ and each is
       reported in `PageCap`.
     - Author `ID` is the forge's numeric account id (GraphQL `databaseId`, GitLab
       `author.id`). An author the forge does not report (a deleted account, GitHub's
       `ghost`) keeps `ID == 0` and `Type == ""`, which is could-not-check, never a match.
   - **56 `IssueStateEvents(repo, number) (*IssueStateHistory, error)`**
     - Result: `IssueStateHistory{Events []IssueStateEvent{Kind closed|reopened, Actor Account,
       CreatedAt}, ClosingChanges []ClosingChange{Repo, Number, Merged bool, Author Account},
       Complete bool}`. `Repo` is the closing change's own `owner/name`: a change in another
       repository can close the issue, and a bare number would then name the wrong change.
     - GitHub: one GraphQL read of the issue, with every connection sized:
       - `timelineItems(first:100, itemTypes:[CLOSED_EVENT,REOPENED_EVENT]){pageInfo{hasNextPage}
         nodes{__typename ... on ClosedEvent{createdAt actor{login __typename ... on
         User{databaseId} ... on Bot{databaseId}}} ... on ReopenedEvent{createdAt actor{login
         __typename ... on User{databaseId} ... on Bot{databaseId}}}}}`. The `Actor` interface
         carries no `databaseId`, so the id is read through the concrete-type fragments.
       - `closedByPullRequestsReferences(first:100, includeClosedPrs:true){pageInfo{hasNextPage}
         nodes{number state merged repository{nameWithOwner} author{login __typename ... on
         User{databaseId} ... on Bot{databaseId}}}}`. Each closing change carries its own
         `number`, `repository{nameWithOwner}` for `Repo`, `merged`, `state` and `author`.
     - **What today's query reads.** `selfimprovement.go:382-388` already sends both
       connections and already reads `merged` and `author{login __typename}` on each closing
       change and `actor{login __typename}` on each event, with `closedByPullRequestsReferences`
       sized `first:10` and `timelineItems` `first:100`, neither with `pageInfo`. New here: the
       id fragments, the closing change's `number`, `state` and `repository{nameWithOwner}`,
       the event `createdAt`, `pageInfo` on both connections, and `first:100` on the closing
       references.
     - **`merged` is read explicitly and fails closed.** `selfimprovement.go:492-501` skips
       every closer whose `merged` is false and counts the close as manual when no merged
       closer remains, so a merged closer misread as unmerged inflates the human-touch count.
       `Merged` is therefore never derived by default. It is `true` only when the node's
       `merged` is `true` and its `state` is `MERGED`, and `false` only when `merged` is
       `false` and `state` is `OPEN` or `CLOSED`. A `null` or missing `merged`, a missing
       `state`, or a `merged` that disagrees with `state` sets `Complete=false` for the whole
       result, so the consumer reads the issue as could-not-check, never as "closed by no
       merged change".
     - `Complete=false` when either connection reports `hasNextPage`.
     - GitLab: `GET /projects/:id/issues/:iid/resource_state_events` (state `closed`/`reopened`,
       `user`, `created_at`) and `GET /projects/:id/issues/:iid/closed_by` (the merge requests
       that close it, with `iid` for `Number`, `state`, `author` and `project_id`, resolved to
       the project path for `Repo`; an unresolvable project leaves `Repo` empty). `Merged` is
       `true` only for `state == merged` and `false` only for `opened`, `closed` or `locked`;
       a missing or unknown `state` sets `Complete=false`, as on GitHub.
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
      - GitLab: filled **only when `state == merged`**: `merge_commit_sha`, else
        `squash_commit_sha` when the merge request was squashed. Empty otherwise, for example
        on a fast-forward merge or on any merge request not yet merged.
      - Empty means could-not-check.
   2. **`ChangeRef.Author`** (`Account`, with `Type` per 2.6):
      - GitHub: GraphQL `author{login __typename ... on User{databaseId} ... on
        Bot{databaseId}}` in `ListChanges`' existing query, so `ID` is the numeric account
        id. An author the forge does not report keeps `ID == 0`, which is could-not-check.
      - GitLab: the merge request's `author`.
   3. **`ChangeRef.BaseRef`**:
      - GitHub: `baseRefName`.
      - GitLab: `target_branch`.
   4. **`ChangeRef.CrossRepo` and `ChangeRef.HeadRepo`**:
      - `CrossRepo` uses the same `CrossRepoSame`/`CrossRepoFork` vocabulary as
        `PullRequest.CrossRepo` (`forge.go:175-183`), empty meaning could-not-check.
      - `HeadRepo` is `owner/name`, empty when unreadable, for example a deleted fork.
      - GitHub: `isCrossRepository` + `headRepository{nameWithOwner}`.
      - GitHub null handling: a `null` `isCrossRepository` leaves `CrossRepo` empty, and a
        `null` `headRepository` leaves `HeadRepo` empty. Neither is ever filled as
        `CrossRepoSame` or as the base repository. `isCrossRepository` is non-null in
        GitHub's schema, so the realistic deleted fork is `isCrossRepository: true` with
        `headRepository: null`: `CrossRepo` = `CrossRepoFork`, `HeadRepo` empty. The null
        `isCrossRepository` branch stays as the defensive case.
      - GitLab: `source_project_id` vs `target_project_id`. Equal gives `CrossRepoSame` and
        `HeadRepo` = the target path. Different gives `CrossRepoFork` and `HeadRepo` empty
        (the source project's path is not read). Either id missing leaves both empty.
      - On GitLab the two fields come from **one** signal, the project-id comparison, while on
        GitHub they are two independent fields. `claimdecay.go:126-170` treats them as two
        signals; the doc comment on `ChangeRef.HeadRepo` states that on GitLab they are one,
        so 18 does not count agreement between them as corroboration.
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
   7. **`Issue.ClosedBy`** (`Account`), filled **only when the issue's state is closed**.
      The decision-gate anchor (`decisiongateanchor.go:157`) reads the closer as the anchor's
      authority, so a closer reported on an issue that is open again (closed, then reopened)
      must not survive into the result:
      - GitHub: `closed_by`, ignored unless `state == closed`.
      - GitLab: `closed_by`, ignored unless `state == closed`.
      - An unreadable or `null` closer on a closed issue leaves `ClosedBy` zero-valued, which
        is could-not-check, never the issue's author.
   8. **`Comment.UpdatedAt`** (RFC3339). The ruling resolver refuses a ruling comment edited
      after it was posted by comparing creation and update times (`decisionruling.go:296-307`),
      so forge-neutral/35 cannot move that read without it:
      - GitHub: `updatedAt` in every comment query `ListCommentsTyped` walks.
      - GitLab: the note's `updated_at`.
      - An unreadable update time leaves `UpdatedAt` empty, which is could-not-check. It is
        never filled from `CreatedAt`: that would present every comment as unedited.
3. **Add the consuming `deskread` kinds** to `readKinds` (`main.go:74`), each with its own
   addressing flag. Each kind refuses the other addressing flags, exactly as the existing kinds
   do. The envelope stays schema 1, and partial-is-a-result semantics apply unchanged.
   - `issue-list --repo … [--state open|closed|all] [--label <name>]` → op 55.
   - `issue --issue owner/name#N` → `GetIssueTyped` (issue kind), carrying the body, state,
     `ClosedBy` and the author `Type`. A number that names a change is a kind mismatch and
     lands in `partial`, never an item.
   - The existing `comments` kind gains `databaseId`, `authorType`, `url` and `updatedAt`
     (2.8) on each comment, all `omitempty`, so existing consumers and goldens are unchanged
     and the envelope stays schema 1. An empty `authorType` or `updatedAt` is
     could-not-check for the consumer, never "User" or "unedited". These are the fields the
     `transcribescan.go:109`, `scanissues.go:953` and `decisionruling.go:262` sites read.
   - The existing `comments` kind gains a change target. `comments --issue owner/name#N`
     keeps reading `ListCommentsTyped(…, TargetIssue)`, and `comments --change owner/name#N`
     reads `ListCommentsTyped(…, TargetChange)`, a pull or merge request's conversation
     thread. The caller names the target. The kind never guesses it, never infers it from the
     number, and never retries under the other target. Both flags, or neither, exit 5 with zero
     forge calls. The envelope, the item shape and the fields are the same as for the issue
     target. A number of the other kind gets the seam's own answer, unchanged: on GitHub a
     `TargetChange` read of an issue number is `deskkit.Unverifiable` and lands in `partial`,
     never an empty thread; on GitLab, where issue and merge-request numbers are separate
     sequences, the same number names a different thread under each target, which is why the
     caller must name it. `trust` stays issue-only. No `Forge` operation is added:
     `TargetChange` is already on the seam.
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
   - Replace "`comments` is deskkit.Forge.ListCommentsTyped on an ISSUE, and nothing else"
     with: `comments` is `ListCommentsTyped` on the issue or change target its address flag
     names. Amend "TWO ADDRESSING SHAPES" so `comments` takes exactly one of `--issue` and
     `--change`.
   - Name forge-neutral/18 as the statusgen consumer that each new kind is waiting for, and
     forge-neutral/35 as the consumer of `issue` and the new `comments` fields in the ruling
     resolver, of the new `comments` fields and change target in the transcribe lanes' sign-off
     resolver, and of `issue` and `trust` in the verdict-issue read.
4. **Register the surface change.**
   - Add inventory rows 55–58 to `docs/streams/forge-gitlab/inventory.md`, in its existing
     column shape, all `implemented`, plus one "added by forge-neutral brief 33" note naming
     the `deskread` kinds as consumers.
   - Add the four names to `TestForgeSurfaceUnchangedByDeskread`'s `want` list, with a
     one-line comment citing this brief.
   - Do not change `allowedInvocationCeiling`.
5. **Goldens on both backends** for every operation and every field, on recorded fixtures,
   including the negative cases Verify rows 5–10 and 21–29 name.

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
- `TestListIssuesServesIssuePopulation` (planned)
- `TestChangeRefCrossRepoUnreadableStaysEmpty` (planned)
- `TestClosedByEmptyUnlessClosed` (planned)
- `TestCommentUpdatedAtUnreadableStaysEmpty` (planned)
- `TestDeskreadCommentsCarryIdentityFields` (planned)
- `TestIssueStateEventsMergedUnreadableIsIncomplete` (planned)
- `TestDeskreadCommentsChangeTarget` (planned)
- `TestCommentFieldsOnChangeTarget` (planned)

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeGithubGolden$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2a.out" 2>&1 && grep -F -e '--- PASS: TestForgeGithubGolden' "${TMPDIR:-/tmp}/b33-r2a.out" && go test ./internal/deskkit/ -run '^TestForgeGitlabGolden$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2b.out" 2>&1 && grep -F -e '--- PASS: TestForgeGitlabGolden' "${TMPDIR:-/tmp}/b33-r2b.out" && go test ./internal/deskkit/ -run '^TestForgeGitlabCoverage$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r2c.out" 2>&1 && grep -F -e '--- PASS: TestForgeGitlabCoverage' "${TMPDIR:-/tmp}/b33-r2c.out"` | exit 0, each `--- PASS:` line printed. Both backends' wire is pinned for ops 55–58 and every Task 2 field, and coverage reconciles the seam against inventory rows 55–58 |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeNoPassthrough$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3a.out" 2>&1 && grep -F -e '--- PASS: TestForgeNoPassthrough' "${TMPDIR:-/tmp}/b33-r3a.out" && go test ./internal/deskkit/ -run '^TestNoForgeCLIShellout$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3b.out" 2>&1 && grep -F -e '--- PASS: TestNoForgeCLIShellout' "${TMPDIR:-/tmp}/b33-r3b.out" && go test ./internal/deskkit/ -run '^TestForgeSingleConstructionSite$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r3c.out" 2>&1 && grep -F -e '--- PASS: TestForgeSingleConstructionSite' "${TMPDIR:-/tmp}/b33-r3c.out"` | exit 0, each `--- PASS:` line printed. The seam grows four typed reads and stays closed |
| 4 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeSurfaceUnchangedByDeskread$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r4.out" 2>&1 && grep -F -e '--- PASS: TestForgeSurfaceUnchangedByDeskread' "${TMPDIR:-/tmp}/b33-r4.out"` | exit 0, `--- PASS:` printed. The pinned method set is the base's 55 plus exactly `ListIssues`, `IssueStateEvents`, `ListChangeCommits` and `RepoDefaultBranch`: 59 names |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestListIssuesIncompleteIsNotAbsence$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r5.out" 2>&1 && grep -F -e '--- PASS: TestListIssuesIncompleteIsNotAbsence' "${TMPDIR:-/tmp}/b33-r5.out"` | **negative path**: a population beyond the page ceiling (GitHub 100 pages of 100, GitLab 25 pages of 100) returns `Incomplete=true` with `PageCap` set on both backends, never a short list presented as the whole. A label containing a comma is refused with zero forge calls |
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
| 17 | check | `git diff --name-only "$(git merge-base refs/remotes/origin/main HEAD)" HEAD -- statusgen/ .github/workflows/`, run on the brief's implementation branch before merge | empty output. No statusgen site and no workflow moved here; both are 18's and #2253's |
| 18 | check | `statusgen --root . --lint` | `LINT: PASS`, rc 0 |
| 19 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/33 --base "$(git merge-base refs/remotes/origin/main HEAD)"`, on the implementation branch | exit 0. Every `consumers:` claim is corroborated against the diff |
| 20 | check | GitLab live row | **could-not-check by design**: this repository has no CI-reachable GitLab project. Rows 2, 5–10 and 21–29 pin the GitLab mapping against recorded fixtures; a live read is the GitLab conformance work's job, not this brief's |
| 21 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestListIssuesServesIssuePopulation$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r21.out" 2>&1 && grep -F -e '--- PASS: TestListIssuesServesIssuePopulation' "${TMPDIR:-/tmp}/b33-r21.out"` | exit 0, `--- PASS:` printed. A GitHub fixture of 1,200 issues across 12 pages, on a repository whose fixture also holds 1,500 changes, returns all 1,200 issues with `Incomplete=false` and no change in the list, and the recorded request is the GraphQL `issues` connection, never REST `/issues`. A GitLab fixture of 1,200 issues returns all of them with `Incomplete=false`. Each issue carries `State`, `CreatedAt`, `ClosedAt` (empty while open), author login and numeric `ID`, labels and title: every field `issues.go:577` reads |
| 22 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestChangeRefCrossRepoUnreadableStaysEmpty$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r22.out" 2>&1 && grep -F -e '--- PASS: TestChangeRefCrossRepoUnreadableStaysEmpty' "${TMPDIR:-/tmp}/b33-r22.out"` | **negative path**: a GitHub change whose fixture carries `isCrossRepository: null` and `headRepository: null` (a deleted fork) reads back `CrossRepo == ""` and `HeadRepo == ""`, never `CrossRepoSame` or the base repository. A GitHub change whose fixture carries `isCrossRepository: true` and `headRepository: null` (the realistic deleted fork) reads back `CrossRepo == CrossRepoFork` and `HeadRepo == ""`. A GitLab merge request whose `source_project_id` differs from `target_project_id` reads back `CrossRepo == CrossRepoFork` and `HeadRepo == ""`, and one with either id missing reads back both empty |
| 23 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestClosedByEmptyUnlessClosed$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r23.out" 2>&1 && grep -F -e '--- PASS: TestClosedByEmptyUnlessClosed' "${TMPDIR:-/tmp}/b33-r23.out"` | **negative path**: an OPEN issue (closed, then reopened) whose fixture still carries a `closed_by` account reads back a zero `ClosedBy` on both backends. A closed issue whose `closed_by` is `null` reads back a zero `ClosedBy`, never the issue's author |
| 24 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestClosedByEmptyUnlessClosed$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r24.out" 2>&1 && grep -F -e '--- PASS: TestClosedByEmptyUnlessClosed' "${TMPDIR:-/tmp}/b33-r24.out"`. **Mutation:** in the GitHub `GetIssue` mapping, and then separately in the GitLab one, fill `ClosedBy` from `closed_by` regardless of state; run the command after each, then restore the file and re-run | exit 0 unmutated. Exit **1** on each mutant (no `--- PASS:` line), with the test naming the reopened-issue case |
| 25 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestChangeRefCrossRepoUnreadableStaysEmpty$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r25.out" 2>&1 && grep -F -e '--- PASS: TestChangeRefCrossRepoUnreadableStaysEmpty' "${TMPDIR:-/tmp}/b33-r25.out"`. **Mutation:** in the GitHub `ListChanges` mapping, read a `null` `isCrossRepository` as `false` (so it maps to `CrossRepoSame`); separately, in the GitLab mapping, fill `HeadRepo` with the target path when the project ids differ; separately, in the GitHub mapping, fill a `null` `headRepository` with the base repository; run the command after each, then restore the file and re-run | exit 0 unmutated. Exit **1** on each mutant (no `--- PASS:` line), with the test naming the null-fork case |
| 26 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCommentUpdatedAtUnreadableStaysEmpty$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r26a.out" 2>&1 && grep -F -e '--- PASS: TestCommentUpdatedAtUnreadableStaysEmpty' "${TMPDIR:-/tmp}/b33-r26a.out" && go test ./cmd/deskread/ -run '^TestDeskreadCommentsCarryIdentityFields$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r26b.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadCommentsCarryIdentityFields' "${TMPDIR:-/tmp}/b33-r26b.out"` | **negative path**: a GitHub comment whose fixture carries no `updatedAt`, and a GitLab note with no `updated_at`, read back `UpdatedAt == ""` with `CreatedAt` set, never `UpdatedAt == CreatedAt`. An edited comment reads back the two times as they differ. `deskread comments` emits `databaseId`, `authorType`, `url` and `updatedAt` when the seam has them and omits each when empty, so a comment with none of the four reads byte-identical to the pre-change output |
| 27 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestIssueStateEventsMergedUnreadableIsIncomplete$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r27.out" 2>&1 && grep -F -e '--- PASS: TestIssueStateEventsMergedUnreadableIsIncomplete' "${TMPDIR:-/tmp}/b33-r27.out"` | **negative path**: a GitHub closing change whose fixture carries `merged: null`, one with `merged: false` and `state: MERGED`, and one with no `state`, each return `Complete=false`, never `Merged=false` with `Complete=true`. A GitLab closing merge request with a missing or unknown `state` does the same. A merged closer reads back with its `Number`, `Repo` and author |
| 28 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCommentUpdatedAtUnreadableStaysEmpty$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r28a.out" 2>&1 && grep -F -e '--- PASS: TestCommentUpdatedAtUnreadableStaysEmpty' "${TMPDIR:-/tmp}/b33-r28a.out" && go test ./internal/deskkit/ -run '^TestIssueStateEventsMergedUnreadableIsIncomplete$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r28b.out" 2>&1 && grep -F -e '--- PASS: TestIssueStateEventsMergedUnreadableIsIncomplete' "${TMPDIR:-/tmp}/b33-r28b.out"`. **Mutation:** in the GitHub comment mapping, fill an absent `updatedAt` from `createdAt`; separately, in the GitHub `IssueStateEvents` mapping, read a `null` `merged` as `false` without touching `Complete`; run the command after each, then restore the file and re-run | exit 0 unmutated. Exit **1** on each mutant (a `--- PASS:` line missing), with the test naming the absent-update-time or null-merged case |
| 29 | check:ci +flow +dereference | `cd tools/desk && go test ./cmd/deskread/ -run '^TestDeskreadCommentsChangeTarget$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r29a.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadCommentsChangeTarget' "${TMPDIR:-/tmp}/b33-r29a.out" && go test ./internal/deskkit/ -run '^TestCommentFieldsOnChangeTarget$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r29b.out" 2>&1 && grep -F -e '--- PASS: TestCommentFieldsOnChangeTarget' "${TMPDIR:-/tmp}/b33-r29b.out"` | **positive and negative path.** The `deskread` leg, on the recording fake `Forge`: `comments --change o/n#7` records exactly one `ListCommentsTyped` call at 7 with `TargetChange`, and `comments --issue o/n#7` exactly one with `TargetIssue`. Both emit schema 1 with the same item shape and the four Task 3 fields. Both flags, neither flag, or `--change` on `trust` exits 5 with zero forge calls. A refusal or not-found from the seam lands in `partial` and is never retried under the other target. The exit is 0 when another item in the set was read, and 6 for a lone target, since `deskread` exits 6 only when nothing in the set could be read (`tools/desk/cmd/deskread/main.go`, the `partial` contract in its usage text). The deskkit leg, on recorded fixtures for BOTH real backends: a GitHub pull-request conversation thread and a GitLab merge-request note thread each read back under `TargetChange`, every comment carrying `DatabaseID`, `URL`, `UpdatedAt` and the author `Type` (empty where unresolved), and GitHub `TargetChange` at an issue number returns `deskkit.Unverifiable`, never an empty thread. The fake leg shows only that the kind passes the stated target. The deskkit leg is what shows the real backends serve a change thread with these fields, and forge-neutral/35's sign-off move depends on it |
| 30 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskread/ -run '^TestDeskreadCommentsChangeTarget$' -count=1 -v > "${TMPDIR:-/tmp}/b33-r30.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadCommentsChangeTarget' "${TMPDIR:-/tmp}/b33-r30.out"`. **Mutation:** in `deskread`'s `comments` case, pass `TargetIssue` whatever the flag; separately, retry a seam refusal under `--change` as a `TargetIssue` read; run the command after each, then restore the file and re-run | exit 0 unmutated. Exit **1** on each mutant (no `--- PASS:` line), with the test naming the change-target case |

## Named mutations

- **M1**, row 13: `ListChangeCommits` always reports complete. This is the 250-commit cap read
  as the whole branch, which would let the auto-flip owner check credit a change for a commit
  that lies beyond the page.
- **M2**, row 14: `MergeCommitSHA` filled on an open change. A test-merge SHA would then match
  a commit no merge produced.
- **M3**, row 24: `ClosedBy` filled regardless of state. A reopened issue would then still
  present its former closer to the decision-gate anchor as the authority that closed it.
- **M4**, row 25: an unreadable fork read as same-repository, or a fork's head credited to the
  base repository. Row 25 mutates it three ways: the GitHub null `isCrossRepository`, the
  GitLab differing project ids, and the GitHub null `headRepository` filled as the base. Claim decay would then attribute a fork's branch to this repository.
- **M5**, row 28: an absent comment update time filled from the creation time, or an
  unreadable closing-change merge state read as unmerged. The first would present an edited
  ruling comment as unedited to the ruling resolver; the second would count a fixed issue's
  close as a manual human touch in self-improvement.
- **M6**, row 30: the change target read as the issue target, or a refused change read retried
  as an issue read. On GitLab the issue with the same number is a different thread, so a
  sign-off posted on a merge request would be looked up on an unrelated issue. On GitHub a
  sign-off posted on a pull request would refuse where it should arm.

## Pre-mortem → detection map

*"This shipped and was wrong. What went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| A GitLab account the users API cannot resolve reads as `"User"`, so a bot's closing action or comment counts as a human's | row 8 |
| GitHub's test-merge SHA on an open change is returned as its merge commit | rows 7, 14 |
| The 250-commit cap or a page-capped list reads as the whole population | rows 5, 6, 13 |
| A binary or oversized file's missing patch reads as "no change in this file" | row 10 |
| An unknown default branch defaults to `main`, so a change into a non-default branch is credited | row 9 |
| The `ListIssues` GitHub mapping keeps pull requests in the issue list, inflating issue metrics | rows 2, 21 (the read is the issues-only connection, and the fixture's changes must not appear) |
| `ListIssues` spends its page ceiling on changes it then drops, so `issues.go:577` is a permanent could-not-check on a repository with more changes than issues | row 21 (1,200 issues beside 1,500 changes come back whole) |
| A deleted fork's `null` cross-repo or head-repository field reads as same-repository, so claim decay attributes a fork's branch to this repository | rows 22, 25 |
| A reopened issue keeps its former closer in `ClosedBy`, so the decision-gate anchor credits a close that no longer stands | rows 23, 24 |
| A label with a comma silently widens `ListIssues`' filter to several labels | row 5 |
| A comment's unreadable update time is filled from its creation time, so an edited ruling comment passes the ruling resolver's unedited check | rows 26, 28 |
| `deskread comments` still lacks the id, author type or URL, so 18 cannot select a comment by id or tell a bot's comment from a human's | row 26 |
| A closing change whose `merged` is `null` or disagrees with its `state` reads as unmerged, so self-improvement counts a fixed issue's close as manual | rows 27, 28 |
| `comments` reads only the issue target, so a sign-off posted on a pull request cannot be read and forge-neutral/35's move either refuses it or falls back to another read | rows 29, 30 |
| The change target is shown only against the recording fake, so no test shows the real backends serve a change thread with the four fields | row 29's deskkit leg, on both backends' recorded fixtures |
| This brief maps an author type or an edit flag into an outcome (a bot or edited verdict on the envelope), moving a control decision under a model gate | ground rule "Transport only; no outcome mapping"; Review reads the envelope diff |
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
and date in the stream README table. The reviewer also confirms three things from the diff:
- no file under `statusgen/` or `.github/workflows/` changed;
- `deskread`'s identity resolution is untouched;
- no kind or field maps an author type or an edit flag into an accept or refuse outcome, and
  the change target only selects which thread is read.

The first two keep this brief independent of #2253. The third keeps every control decision in
forge-neutral/35, under its human gate.
