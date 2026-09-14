---
brief: assay:assay:forge-gitlab:10
title: GitLab trust-events + commit author-login for the deskpost trust read
why: >-
  The reviewer write path (09) proved the GitLab Forge write ops exist and land the reviewer PAT
  auth, but the deskpost-verdict WIRING that routes review preconditions through the typed Forge
  surface is still blocked — and one named blocker is the trust read itself. `PRTrustEvents` is a
  could-not-check REFUSAL on the GitLab backend, and `GetCommit` leaves the author/committer login
  EMPTY on GitLab. deskpost's review precondition chain runs a TRUST read (`PRTrustEvents`) to
  decide whether a change's author and content-bearing events come from trusted identities — the
  gate that quarantines unvetted third-party work before any verdict lands — and deskboard's
  blessing + stall clock read the same trust payload and the head commit's author login. On GitLab
  both fail closed, so a GitLab review desk cannot form the trust verdict deskpost needs and cannot
  read a commit's attributed identity. This brief scopes the GitLab trust-events read and the
  commit author-login resolution to parity with GitHub, closing the could-not-check gap the stubs
  name. It unblocks the deskpost-verdict wiring alongside the claimLiveness and RepoInfoFetcher
  work (origin #798; the write ops and PAT auth landed in #800).
wave: 5
depends: ["forge-gitlab/02", "forge-gitlab/09"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [798]
schema: brief-v2
authored: 2026-09-10 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §1 (the two disclosed CE degradations, and nothing else — no third degradation without a recorded human ruling), §3 (governing security-parity requirement: at least as secure as the GitHub controls even where the mechanism differs), §6 (concept mapping + freeze rule: a read lands with its consuming call site)"
  - "docs/streams/forge-gitlab/edition-matrix.md — table A (every Forge operation is Free-tier; notes, commit and user reads carry no tier badge above Free)"
  - "docs/streams/forge-gitlab/brief-02-gitlab-forge-impl.md — the GitLab backend the trust read and the commit read land on (the note-body model, the numeric id space, the honest-empty posture ReviewsAtHead/GetCommit already take)"
  - "docs/streams/forge-gitlab/brief-09-gitlab-reviewer-write-path.md — the deskpost verdict/comment/ready write path whose precondition chain reads the trust gate through the Forge surface; this brief supplies the trust read that chain calls on GitLab"
  - "docs/streams/forge-gitlab/brief-08-close-the-forge-surface.md — the enumerated/closed Forge surface and the shell-exec ban this brief works within (no `glab` shell, no arbitrary-endpoint escape hatch to reach the notes/commit reads)"
  - "tools/desk/internal/deskkit/forge_gitlab.go — the `PRTrustEvents`/`IssueTrustEvents` could-not-check stubs (`trustEventsGap`) and `GetCommit` leaving AuthorLogin/CommitterLogin empty; the reads to implement"
  - "tools/desk/internal/deskkit/forge_github.go — the GitHub `PRTrustEvents` (GraphQL trust query) and `GetCommit` (email→account login) whose semantics the GitLab reads must equal"
  - "tools/desk/internal/deskkit/trust.go + trustfetch.go — the `TrustPayload`/`ContentEvent` shape, `deskkit.Blessed`, and the shared `trustFromEnvelope`/`collectEvents` reader whose verdict the GitLab payload must reproduce"
  - "freshness-checked 2026-09-10 @ b3c0fc08 — `PRTrustEvents`/`IssueTrustEvents` return `trustEventsGap` (could-not-check) on GitLab; `GetCommit` returns `RepoCommit{SHA, CommittedDate}` with AuthorLogin/CommitterLogin EMPTY; the GitHub backend implements both to the parity contract; deskboard's prBlessed/issueBlessed/fetchHeadCommit already consume these reads (freeze-rule call sites exist)"
exec-tier: strong
exec-tier-why: "the deliverable IS a security control — the trust gate that decides who may act on a change before a verdict lands. A plausible-but-wrong mapping fail-opens: a body content-edit signal that fails to re-quarantine after a bless-then-edit, an author id a recycled login can fake, or a guessed blessing on a partial thread all survive happy-path tests but admit unvetted content, which is exactly the failure the parity requirement forbids (spec §3; the stubs refuse rather than approximate for this reason). GetCommit's login is the identity the stall clock and the trust gate read — a wrong resolution mis-attributes a commit."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the GitLab `PRTrustEvents` implementation to GitHub parity — and its `IssueTrustEvents` twin, sharing `trustEventsGap` and the `trustFromEnvelope` reader — plus `GetCommit` resolving the author/committer login on the GitLab backend)"
  - "tools/desk/cmd/deskboard: consumes (prBlessed reads `PRTrustEvents`; issueBlessed reads `IssueTrustEvents`; fetchHeadCommit reads `GetCommit`'s author/committer login) — existing freeze-rule call sites that go from could-not-check to a real verdict on GitLab; no deskboard change is in scope"
  - "tools/desk/cmd/deskpost: follow-up (the blocked deskpost-verdict wiring routes its trust-gate precondition through the now-real Forge `PRTrustEvents`; that wiring is the downstream this unblocks, not delivered here)"
version: 1
id: 6d5026cf-4953-466e-a265-b72b84412c96
---

# Brief 10 — GitLab trust-events + commit author-login

## Context

The GitLab backend can read verdicts at head, checks, changed files and visibility (brief 02), and
the reviewer role can now authenticate and write on GitLab (brief 09, landed in #800). What it
still cannot do is read the two identity/trust facts the review and board paths gate on: it refuses
the trust-events read outright, and it reports a commit with no attributed account. Both are
could-not-check today — the honest fail-closed posture — which is safe but leaves a GitLab review
desk unable to form the trust verdict deskpost's precondition chain demands. Closing the gap to
GitHub parity is this brief.

**§1 — `PRTrustEvents` is a could-not-check stub on GitLab.** deskpost's review precondition chain
runs a TRUST read to decide whether a change's author and its content-bearing events (comments,
reviews, review comments) come from trusted identities, and — for an untrusted author — whether a
blessing authority's comment currently covers the content (`deskkit.Blessed`). On GitHub the read
is a GraphQL query returning the item body's `lastEditedAt` (a CONTENT-edit time, not a REST
`updated_at` that moves on unrelated events like labels), and, per event, the author's rendered
login (`example-bot[bot]` form for a bot), the numeric author id a recycled login cannot fake, the
creation time and the content edit time, plus a completeness flag for the single-page bound. On the
GitLab backend `PRTrustEvents` (and its `IssueTrustEvents` twin) returns `trustEventsGap` —
`could-not-check … the GitLab backend does not serve PRTrustEvents … deferred to the forge-gitlab
trust-events brief, never approximated (a guessed blessing is fail-open)`. This brief IS that
trust-events brief. The task is to produce the same `TrustPayload` (`BodyEdited`, `Events`,
`Complete`) from GitLab REST v4 so `deskkit.Blessed` draws the SAME verdict on equivalent content —
through the enumerated Forge surface, **not** by shelling `glab` and not via any arbitrary-endpoint
escape hatch (brief 08's `TestNoForgeCLIShellout`/`TestForgeNoPassthrough` stay green).

**§2 — `GetCommit` returns no author login on GitLab.** `GetCommit` fills `SHA` and `CommittedDate`
but leaves `AuthorLogin`/`CommitterLogin` EMPTY on GitLab, because a GitLab commit payload carries
the raw git author/committer name+email without resolving them to an instance account. GitHub
resolves the email to an account login — the identity comparable to a change author's login that
deskboard's `fetchHeadCommit` (the stall clock) and the trust path read. Parity means resolving the
commit's author/committer to a GitLab account login when one exists (Free-tier user lookup), and
keeping the field EMPTY only where no account resolves — a per-field could-not-check read as UNKNOWN
attribution, never fabricated and never read as "not the author".

files:
- `tools/desk/internal/deskkit/forge_gitlab.go` — replace the `PRTrustEvents`/`IssueTrustEvents`
  `trustEventsGap` refusals with a real read: GitLab MR/issue notes (`GET …/notes` or the
  discussions API, Free-tier), filtering out `system` notes, mapping each note's author `{id,
  username}` to `ContentEvent.Author` (rendered login, bot discrimination resolved from the users
  API, never defaulted) and `AuthorID` (the numeric id), `created_at` to `CreatedAt`, and the
  note-content edit time to `EditedAt`; the item body's content-edit time to `BodyEdited`; and
  pagination completeness to `Complete`. In the same edit, resolve `GetCommit`'s author/committer
  login by looking the commit's author/committer email up to an instance account (Free-tier user
  lookup), filling `AuthorLogin`/`CommitterLogin` when it resolves and leaving them EMPTY (per-field
  could-not-check) when it does not.
- `tools/desk/internal/deskkit/forge_github.go` — the reference semantics only; unchanged. The
  GitLab reads must equal what these already produce.
- `tools/desk/internal/deskkit/trust.go`, `trustfetch.go` — the `TrustPayload`/`ContentEvent` shape
  and `trustFromEnvelope`/`collectEvents`/`Blessed` the GitLab payload must feed identically; the
  GitLab backend routes its parsed events through the SAME reader so seam and CLI cannot draw
  different blessings from one payload. No shape change is in scope — the GitLab read fills the
  existing struct.

single-point-of-failure: the trust read is the one control the review/bless gate stands on — a
mapping that fail-opens (a `BodyEdited` signal that fails to move on a bless-then-edit so a rewritten
body is admitted, an `AuthorID` a recycled login can fake, or a `Complete=true` on a truncated
thread) admits unvetted content. It is backed by two INDEPENDENT layers, each a different signal in
a different component: (1) the pre-parity backend fails CLOSED (could-not-check → no admission), so
the fail-open is introduced ONLY by a wrong mapping — which makes the parity test that compares the
GitLab verdict against the GitHub verdict on the same content the enforcing control, not an
afterthought; and (2) the blessing is bounded by BOTH the numeric author id (identity the login
cannot recycle) AND the content-edit-time re-quarantine rule, so a single wrong field cannot on its
own turn a quarantine into a blessing.

facts:
- Spec §6 freeze rule governs both reads: each lands with an EXISTING consuming call site
  (`PRTrustEvents`/`IssueTrustEvents` ↔ deskboard's prBlessed/issueBlessed; `GetCommit` ↔ deskboard's
  fetchHeadCommit) — no speculative or generic method, no new interface shape.
- `IssueTrustEvents` is `PRTrustEvents`' twin and shares `trustEventsGap`; delivering the PR read to
  parity delivers the issue read at near-zero marginal cost (same notes read, item kind aside), so
  both close together — that is what the trust-events gap the stubs name resolves to.
- The content-edit signal is the crux GitHub solves with GraphQL `lastEditedAt` — a REST `updated_at`
  on the MR/issue body moves on unrelated events (labels, assignees) and would re-quarantine
  spuriously (`deskkit.Blessed`'s note is explicit). The GitLab mapping must find the content-edit
  signal that reproduces the same behaviour — a note edited after a blessing re-quarantines, a label
  event does not — analogous to how brief 02's `ReviewsAtHead` pinned the head in the note BODY
  rather than trusting a server field.
- Error mapping stays three-state (brief 02): a permission/tier/transport failure on either read is
  `could-not-check`, distinct from an empty read (a change with no notes; a commit whose email
  resolves to no account) and from a real payload. A read failure is never reported as a clean empty
  trust set — reading a green blessing off "we could not learn the events" is the fail-open the read
  exists to prevent.
- Brief 08's shell-exec ban and no-passthrough test are in force: the notes/user/commit reads go
  through the typed Forge backend over REST v4 — no `glab` shell, no arbitrary-endpoint method.
- OUT OF SCOPE: the deskpost-verdict wiring itself (routing deskpost's precondition chain through
  the Forge — the downstream this unblocks, alongside the claimLiveness and RepoInfoFetcher blockers,
  #798), and `ListOpenIssues` (a separate stub with its own issueboard consumer, unblockable once the
  issue trust read is real). This brief delivers only the two reads.

## Edition
Minimum GitLab tier: **free** (Community Edition). Every read is Free-tier: MR/issue notes and the
discussions API, the commit read (`GET …/repository/commits/:sha`), and the user lookup that
resolves an email/id to an account login — none carries a tier badge above Free (edition-matrix.md
table A). No Premium/Ultimate feature is touched, and the reviewer PAT (brief 03) is the Free
custody mechanism the read authenticates from.

**No new CE degradation is introduced.** The mechanism differs from GitHub's (REST notes + a
content-edit signal + a user lookup, versus one GraphQL query with `lastEditedAt` and `databaseId`),
but the CONTROL — the trust verdict `deskkit.Blessed` forms — must be reproduced to parity, which
spec §3 requires even where the mechanism differs. The two disclosed CE degradations remain exactly
the two spec §1 names (identity-granular protected branches; enforced approval rules); this read is
neither. If implementation finds a GitLab content-edit or identity fact that genuinely cannot reach
the GitHub verdict 1:1 — the reason the stub currently refuses rather than approximates — that is a
recorded human ruling under spec §3, NOT a silent weakening and NOT a fabricated blessing: the read
keeps its could-not-check posture until the ruling, exactly as it does today.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `PRTrustEvents` (and its `IssueTrustEvents` twin) on the GitLab backend: read the
   change's/issue's notes (Free-tier, `system` notes filtered), map each to a `ContentEvent`
   (rendered author login with bot discrimination from the users API, numeric `AuthorID`,
   `CreatedAt`, content `EditedAt`), set `BodyEdited` from the item body's content-edit time and
   `Complete` from pagination, and route the parsed events through the shared
   `trustFromEnvelope`/`collectEvents` reader so the verdict equals GitHub's on equivalent content.
   Remove the `trustEventsGap` refusals.
2. Resolve `GetCommit`'s author/committer login on GitLab by looking the commit's author/committer
   email (or id) up to an instance account (Free-tier user lookup); fill `AuthorLogin`/
   `CommitterLogin` when it resolves, leave EMPTY (per-field could-not-check) when it does not.
   Never fabricate a login and never read an empty login as "not the author".
3. Preserve the three-state error surface on both reads: a permission/tier/transport failure is
   `could-not-check`, distinct from an empty read and from a real payload.
4. Keep every affected tool's existing tests green unmodified; keep brief 08's ban/no-passthrough
   tests green (no `glab` shell, no arbitrary-endpoint method).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeGitlabTrustEvents -v` | exit 0; `PASS` — `PRTrustEvents` on a GitLab MR fixture returns a real `TrustPayload` (a non-nil result with `Events` carrying a rendered author login AND a non-zero numeric `AuthorID`, a `BodyEdited` time, and `Complete`), NOT a `could-not-check` stub; the `IssueTrustEvents` twin returns a real payload on an issue fixture |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeGitlabTrustParity -v` | exit 0; `PASS` — `deskkit.Blessed` on the GitLab payload draws the SAME verdict as GitHub on equivalent content: a note edited by an untrusted author AFTER the blessing re-quarantines, and a label/assignee event does NOT re-quarantine (no spurious re-quarantine from a body `updated_at`) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeGitlabGetCommitAuthorLogin -v` | exit 0; `PASS` — `GetCommit` on a GitLab commit fixture whose author/committer resolve to instance accounts returns NON-EMPTY `AuthorLogin`/`CommitterLogin`; a fixture whose email resolves to no account leaves them EMPTY (per-field could-not-check, never fabricated) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeGitlabTrustReadTierErrors -v` | exit 0; `PASS` — a permission/transport failure on the trust or commit read surfaces `could-not-check`, distinct from an empty read (no notes / unresolved email) and from a real payload; a read failure is never a clean empty trust set |
| 6 | `cd tools/desk && GOWORK=off go test ./... -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | exit 0 on both; `PASS` — the trust/commit reads add no `glab` shell-out and no arbitrary-endpoint passthrough method |
| 7 | `grep -c "does not serve PRTrustEvents\|does not serve IssueTrustEvents" tools/desk/internal/deskkit/forge_gitlab.go` | `0` — the `trustEventsGap` refusal is gone; the GitLab backend serves both trust reads |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). This brief's deliverable IS a security-parity control — the trust
gate that decides who may act on a change and how a commit's identity is attributed — so its PR
requires a SEPARATE `Security-Review:` review in addition to the correctness review (public-repo
discipline; matches briefs 07/08/09). Reviewer records the verdict + date in the stream README
table.

Reviewer answers the defense-in-depth questions: the parity test that compares the GitLab verdict
against the GitHub verdict is the upper layer — does a LOWER layer independently catch a fail-open
(does the numeric `AuthorID` guard hold when the login mapping is wrong, and does a Verify row prove
a bless-then-edit re-quarantines rather than admitting the rewritten content)? And is the
content-edit signal provably NOT a bare body `updated_at` — i.e. does a row show a label/assignee
event does not re-quarantine, so the GitLab mapping does not re-open the spurious-re-quarantine hole
`deskkit.Blessed` warns about?
