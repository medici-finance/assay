---
brief: assay:assay:forge-gitlab:09
title: GitLab reviewer write path — deskpost verdict/comment/ready + deskfile/desktoken PAT auth
why: >-
  The Forge seam (01/02), the go-gh GitHub re-seat (07) and the closed surface (08) all landed,
  yet a LIVE GitLab review desk still cannot land a verdict or file an escalation: the write and
  auth paths were never wired to GitLab. `deskpost review`/`security-review`/`comment` (and then
  `ready`) form and read every precondition through a GitHub App-authenticated client, so on a
  GitLab-resolved repo they fail closed with `could-not-check: deskpost has no gitlab write
  backend … exit 6`; and `deskfile check` / `desktoken reviewer` die in `no App ID for App
  "reviewer-app"` because they attempt a GitHub App mint instead of resolving the provisioned
  GitLab reviewer PAT. This brief closes both — the review desk's verdict-and-escalation path
  runs on GitLab through the typed Forge surface, using the role PAT, with parity proven. It is
  the head of the field-check critical path (#795 §1 and §2).
wave: 4
depends: ["forge-gitlab/02", "forge-gitlab/08"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: ["#795"]
schema: brief-v2
authored: 2026-09-10 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §1 (the two disclosed CE degradations — approvals advisory on CE, §1(2)), §3 (governing security-parity requirement; no third degradation without a human ruling), §6 (concept mapping — approval ↔ MR approve endpoint, review-at-head ↔ approvals + notes filtered by head SHA; freeze rule)"
  - "docs/streams/forge-gitlab/edition-matrix.md — table A rows A5/A6/A7 (PostComment, PostReview approve, ReviewsAtHead all Free) and rows B3/B4 (required/no-self-approval Premium → CE advisory)"
  - "docs/streams/forge-gitlab/brief-02-gitlab-forge-impl.md — the GitLab backend the write ops land on (PostReview/PostComment/MarkReadyForReview + the note-body head-SHA pin ReviewsAtHead reads on CE)"
  - "docs/streams/forge-gitlab/brief-07-github-forge-go-gh.md — the GitHub backend + minted-token auth binding whose parity the GitLab write path must match (refuse-if-unminted, no ambient fallback)"
  - "docs/streams/forge-gitlab/brief-08-close-the-forge-surface.md — the enumerated/closed Forge surface and the shell-exec ban this brief works within (no `glab mr approve` passthrough)"
  - "docs/streams/forge-gitlab/brief-03-gitlab-token-custody.md — the custody file contract `<config>/gitlab-<role>.token` the reviewer PAT resolves from"
  - "field-check #795 §1 (deskpost has no gitlab write backend) and §2 (deskfile/desktoken reviewer-role auth on GitLab) — the two head-of-critical-path gaps this brief scopes; §3 (pin-column reader) and §4 (queue labels) are handled in a separate PR"
  - "freshness-checked 2026-09-10 — brief 02 shipped PostReview(APPROVE)/PostComment/MarkReadyForReview + ReviewsAtHead on the GitLab backend and its verifier run confirms the note-body SHA pin; deskflip already passes `app-token` on the GitLab PAT and correctly stops at `reviewer-approved` pending a verdict, so the verdict WRITE is the one missing piece on the desk-post side"
exec-tier: strong
exec-tier-why: "the deliverable IS a security control — the reviewer verdict gate on GitLab. A plausible-but-wrong mapping (an approve that posts a note but never becomes visible to ReviewsAtHead, a request-changes that silently no-ops, or a PAT resolver that falls back to a GitHub App mint) survives happy-path tests but breaks the parity requirement and the verdict-before-ready gate (spec §3; questions b and c)."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the reviewer write ops on the Forge surface — approve/request-changes mapping — and their GitLab backend impl)"
  - "tools/desk/cmd/deskpost: fixed-here (review/security-review/comment/ready wiring routes through the resolved Forge write path instead of a GitHub-App-only client)"
  - "tools/desk/cmd/deskfile, tools/desk/cmd/desktoken: fixed-here (reviewer-role auth resolves the GitLab reviewer PAT on a GitLab-resolved repo, not REVIEWER_APP_ID)"
version: 1
id: fdf0682e-24b8-48fc-93de-becea3aedf5a
---

# Brief 09 — GitLab reviewer write path

## Context

The read side is done; the write and auth sides are not. On a GitLab-resolved repo a review
desk can READ (brief 02's backend answers `ReviewsAtHead`, `ChecksAtHead`, `ListChangedFiles`,
`RepoVisibility`) and `deskflip` already authenticates on the GitLab PAT (`app-token`) and
correctly halts at `reviewer-approved` pending a verdict — but it cannot WRITE the verdict, the
comment, or the escalation, and the reviewer role cannot even authenticate for `deskfile`.

**§1 — deskpost has no GitLab write backend.** `deskpost review` / `security-review` / `comment`
(and the downstream `ready`) form and read every precondition — reviews at head, the label
timeline, CI rollups, the changed-file diff, the trust read — through a GitHub App-authenticated
client. On a GitLab-resolved repo the whole verb fails closed:
`could-not-check: deskpost has no gitlab write backend … exit 6`. The desk therefore cannot land
a verdict on GitLab, so nothing ever reaches `reviewer-approved` for `deskflip` to act on. The
fix is a GitLab write path that (a) resolves preconditions through the typed Forge surface rather
than a GitHub-only client, and (b) maps `approve` / `request-changes` onto GitLab approvals + a
verdict note so `ReviewsAtHead` sees the verdict at head. It must go through the enumerated Forge
surface — **not** by shelling `glab mr approve`: brief 08's shell-exec ban forbids that, and the
no-passthrough test would fail on any arbitrary-endpoint escape hatch added to get around it.

**§2 — deskfile / desktoken reviewer-role auth on GitLab.** On a GitLab-resolved repo,
`deskfile check` and `desktoken reviewer` die with
`desktoken reviewer: exit 6 — no App ID for App "reviewer-app": set REVIEWER_APP_ID …` — a GitHub
App mint attempted where no App exists. They must instead resolve the provisioned GitLab reviewer
PAT from the custody file contract brief 03 established (`<config>/gitlab-reviewer.token`), never
touching `REVIEWER_APP_ID`. Minimum proof: a `deskfile check` on a GitLab repo that completes
without ever calling the GitHub App mint path.

files:
- `tools/desk/internal/deskkit/forge.go` — the reviewer verdict-write op(s) deskpost needs
  (`PostReview` with an `APPROVE` / `REQUEST_CHANGES` verdict, `PostComment`, `MarkReadyForReview`
  where deskpost drives the ready flip). Brief 02 shipped `PostReview(APPROVE)` /`PostComment` /
  `MarkReadyForReview`; the addition here is the **request-changes verdict** on the same typed op
  (no new generic method, spec §6 freeze rule — the op that lands it carries a consuming call
  site in deskpost in this same change).
- `tools/desk/internal/deskkit/forge_gitlab.go` — implement the request-changes verdict on the
  GitLab backend: GitLab has no native "request changes" review state, so it maps to
  **unapprove + a verdict note carrying the verdict and the head SHA**, exactly the surface
  `ReviewsAtHead` already reads (brief 02's note-body SHA pin). This is a *mapping*, not a new
  degradation — see `## Edition`.
- `tools/desk/internal/deskkit/forge_github.go` — the GitHub backend keeps its existing
  `REQUEST_CHANGES` semantics (go-gh review event); the typed op is symmetric across backends.
- `tools/desk/cmd/deskpost/*.go` — `review` / `security-review` / `comment` / `ready` resolve the
  Forge for the repo (GitHub or GitLab) and read preconditions + write the verdict/comment/ready
  through it; the `could-not-check: deskpost has no gitlab write backend … exit 6` fail-closed is
  removed once the GitLab path exists. Budgets/rate-limits/breakers/body-secret checks WRAP the
  interface (spec §6) and stay put — this brief does not pull them into a backend.
- `tools/desk/cmd/desktoken/*.go`, `tools/desk/cmd/deskfile/*.go` — reviewer-role auth on a
  GitLab-resolved repo resolves `<config>/gitlab-reviewer.token` (brief 03 contract) instead of
  minting a GitHub App; the `REVIEWER_APP_ID` lookup is not reached on the GitLab path.

single-point-of-failure: the reviewer verdict WRITE that `ReviewsAtHead` can see at head is the
one control the ready-gate stands on — if an approve/request-changes lands somewhere the read
path cannot see (wrong note format, missing head SHA, an approval with no note), the desk either
flips ready with no real verdict or never flips at all. It is backed by TWO independent layers:
(1) `deskflip` already refuses to flip ready without an at-head verdict and stops at
`reviewer-approved` — a different tool, a different signal — so a mis-landed verdict fails safe
(no flip) rather than falsely ready; and (2) on CE the verdict note carries the head SHA in its
body (brief 02), so at-head reading does not depend on the server pinning the approval. Two
mechanisms, two components.

facts:
- Spec §6 freeze rule governs every op touched: the verdict-write op stays part of the frozen
  enumerated surface; the request-changes verdict extends an EXISTING typed op, and lands with
  its deskpost consuming call site in the same change — no speculative or generic method.
- Brief 08's shell-exec ban and no-passthrough test are in force: no `glab mr approve`, no
  arbitrary-endpoint method may be added to reach GitLab. The verdict write goes through the
  typed Forge op or it does not ship (`TestNoForgeCLIShellout` / `TestForgeNoPassthrough` must
  stay green).
- Auth parity with brief 07's GitHub binding: the GitLab write path authenticates from the
  resolved reviewer PAT value (`PRIVATE-TOKEN` header, value read from the custody file — never a
  token embedded in a URL) and MUST NOT fall back to an ambient `glab`/keyring identity; the
  reviewer role's auth resolves the PAT or refuses, mirroring deskpr's refuse-if-unminted guard.
- `deskflip` is already correct on GitLab (`app-token` on the PAT, halts at `reviewer-approved`);
  this brief supplies only the verdict WRITE that produces the `reviewer-approved` state. No
  deskflip change is in scope.
- Error mapping stays three-state (brief 02): a tier/permission failure on a write is
  `could-not-check`, distinct from a clean write and from an empty read — a Premium-gated 403 is
  never reported as a landed verdict.
- Related but OUT OF SCOPE (#795 §3 and §4 — a separate PR): the pin-column reader and the queue
  labels. This brief scopes only §1 (deskpost write backend) and §2 (reviewer-role PAT auth).

## Edition
Minimum GitLab tier: **free** (Community Edition). Every write this brief lands is Free-tier:
notes (edition-matrix.md row A5), the MR approve endpoint (row A6), and the draft→ready title
edit (row A2); `ReviewsAtHead` reads at head on Free with the note-body SHA pin (row A7). The
reviewer PAT auth is the Free custody mechanism (brief 03).

On CE, **approvals are advisory** — this is the SECOND disclosed degradation the spec already
names (spec §1(2); matrix rows B3/B4), not a new one. The desk's own gate stands: no ready-flip
without an at-head verdict, human-merge-only. The request-changes mapping (unapprove + a verdict
note) is a *mapping across concepts*, not a weakening: GitLab has no native request-changes
state, and the note is the same at-head surface the profile already reads, so no new degradation
is introduced. Spec §3 forbids naming a third degradation without a recorded human ruling; this
brief names none.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the request-changes verdict to the existing typed reviewer-write op on `forge.go`
   (`PostReview` carrying `APPROVE` / `REQUEST_CHANGES`); confirm `PostComment` and
   `MarkReadyForReview` already cover the comment and ready-flip writes deskpost needs.
2. Implement the request-changes verdict on `forge_gitlab.go` as unapprove + a verdict note
   carrying the verdict and head SHA (the surface `ReviewsAtHead` reads); keep the GitHub backend
   symmetric on its native review event. No `glab` shell, no arbitrary-endpoint method.
3. Wire `deskpost review` / `security-review` / `comment` / `ready` to resolve the Forge for the
   repo and read preconditions + write the verdict/comment/ready through it; remove the
   `could-not-check: deskpost has no gitlab write backend … exit 6` fail-closed once the GitLab
   path exists. Preserve the three-state error surface on writes.
4. Resolve reviewer-role auth in `desktoken` / `deskfile` from `<config>/gitlab-reviewer.token`
   on a GitLab-resolved repo, never reaching `REVIEWER_APP_ID`; refuse (do not fall back to an
   ambient identity) if the PAT is absent.
5. Keep every affected tool's existing tests green unmodified; keep brief 08's ban/no-passthrough
   tests green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `deskpost review --dry-run` against a GitLab-resolved repo fixture forms an APPROVE verdict through the Forge write path | exit 0; output shows a verdict formed via the GitLab backend and does NOT contain `deskpost has no gitlab write backend` (the exit-6 fail-closed is gone) |
| 3 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabRequestChanges -v` | exit 0; `PASS` — a request-changes verdict lands as unapprove + a head-SHA verdict note and is then read back by `ReviewsAtHead` at that head (approve↔request-changes both visible to the read path) |
| 4 | `deskfile check` against a GitLab-resolved repo fixture, with `REVIEWER_APP_ID` unset | exit 0; completes without ever calling the GitHub App mint path — a trace/test assertion shows the reviewer PAT (`gitlab-reviewer.token`) resolved and no `no App ID for App "reviewer-app"` error |
| 5 | `go test ./tools/desk/cmd/desktoken/ -run TestReviewerAuthGitlabPAT -v` | exit 0; `PASS` — `desktoken reviewer` on a GitLab repo resolves the custody-file PAT and refuses (does not fall back to ambient identity) when it is absent; `REVIEWER_APP_ID` is not read on the GitLab path |
| 6 | `go test ./tools/desk/... -run TestNoForgeCLIShellout -v && go test ./tools/desk/internal/deskkit/ -run TestForgeNoPassthrough -v` | exit 0 on both; `PASS` — the GitLab write path adds no `glab` shell-out and no arbitrary-endpoint passthrough method |
| 7 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabWriteTierErrors -v` | exit 0; a 403 on a write fixture surfaces `could-not-check`, distinct from a landed verdict — a Premium-gated failure is never reported as a clean write |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). This brief's deliverable touches the reviewer write/auth path —
a security-parity control — so its PR requires a SEPARATE `Security-Review:` review in addition
to the correctness review (public-repo discipline; matches briefs 07/08). Reviewer records the
verdict + date in the stream README table.

Reviewer answers the defense-in-depth questions: the ReviewsAtHead-visible verdict write is the
upper layer — does `deskflip`'s independent refusal-to-flip-without-an-at-head-verdict catch a
mis-landed verdict (fail-safe: no flip) rather than the desk falsely flipping ready? And is the
GitLab reviewer-PAT auth provably refuse-not-fallback — i.e. is the ambient-identity fallback
closed on the write path, not merely unused on the happy path — matching brief 07's GitHub
auth-binding guarantee?
