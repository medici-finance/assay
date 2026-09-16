---
brief: assay:assay:forge-gitlab:17
title: An enforceable merge gate on GitLab Free — the unresolved review thread
why: >-
  On GitLab the review desk's verdict-before-merge gate has nothing server-side behind it on the
  Free tier: the `Draft:` prefix is a title string any Developer can strip, and required
  approvals are Premium — on 2026-09-14 a GitLab adopter cell watched every review-state read
  abort because the project approval-configuration route answered 403, leaving the verdict verb
  unable to post, the board blind and the flip refusing. GitLab does enforce two merge conditions
  on every tier, as project settings: pipelines must succeed (already provisioned) and all
  discussion threads must be resolved. This brief turns the second into the desk's gate — a marker
  thread opened with every merge request, resolved only by the reviewer's approve verdict at the
  current head, re-opened by a request-changes verdict or a new head — so the merge button is
  blocked from the first second on Free, the flip keys on a read that cannot 403, and the human
  merge stays the outer gate it already is.
wave: 5
depends: ["forge-gitlab/09", "forge-gitlab/14"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-14 by forge-gitlab authoring session (fanout worker)
sources:
  - "#1091 — the field evidence, 2026-09-14, from a GitLab adopter cell on a gitlab.com Free-tier project: `GET /projects/:id/approvals` answers 403 (not the 404 the tree degrades on), so the review-state read fails closed, `deskpost review` / `security-review` abort before posting, `deskboard actions` exits 6 and `deskflip` cannot flip; verdicts were recorded off-forge for the day. Its fix (treat 403 like 404 on that one route) is a SIBLING of this brief, not a prerequisite — this brief removes the flip's dependence on that route altogether"
  - "docs/streams/forge-gitlab/edition-matrix.md rows B5 and B6 — `only_allow_merge_if_pipeline_succeeds` and `only_allow_merge_if_all_discussions_are_resolved` are both Free, both project settings on the Projects API, both owned by brief 04; and row B3 — required approvals are Premium, with the CE fallback being human-only merge plus the desk's refusal to flip without an at-head verdict. This brief gives that fallback a server-side layer"
  - "docs/streams/forge-gitlab/pilot-report.md row B3 (a `Draft:` MR opens with `draft: true` on Free), row B6 (the ready flip is a title edit stripping the prefix, `detailed_merge_status: mergeable` afterwards, on Free) and D-4 (the 2026-09-02 pilot project had `only_allow_merge_if_all_discussions_are_resolved: false` because the provisioner did not set it then; the provisioner sets it now, the runbook's by-hand table still does not list it)"
  - "docs/streams/forge-gitlab/spec.md §1 (CE is conforming with two disclosed degradations, the second being advisory approvals), §3 (parity per control) and §6 (the Forge interface is FROZEN at the operations a shipping tool consumes — additions require a consuming tool in the same change; this brief adds a merge-hold op set and its three consuming verbs in the same change)"
  - "docs/streams/forge-gitlab/brief-09-gitlab-reviewer-write-path.md — the verdict write this brief hangs the resolve on: `PostReview` on the GitLab backend posts a `Verdict: approve` note and calls the approve endpoint with the head sha; request-changes calls unapprove"
  - "docs/streams/forge-gitlab/brief-14-public-repo-gate-on-the-resolved-forge.md — the rule this brief inherits: every gate read goes through the backend already resolved for the repo, never a second client"
  - "https://docs.gitlab.com/api/projects/ — `only_allow_merge_if_all_discussions_are_resolved` (boolean, no tier badge on a Free page); https://docs.gitlab.com/api/discussions/ — merge-request discussions: create (`POST /projects/:id/merge_requests/:iid/discussions`), reply (`POST …/discussions/:discussion_id/notes`), resolve/unresolve (`PUT …/discussions/:discussion_id?resolved=true|false`), all Free; https://docs.gitlab.com/api/merge_requests/ — `detailed_merge_status` values include `discussions_not_resolved` and `draft_status`; https://docs.gitlab.com/user/project/merge_requests/drafts/ — the prefix is removable by anyone who can edit the title"
  - "freshness-checked 2026-09-14 @ a864af26 — `tools/create-fleet-gitlab.sh` already PUTs both merge checks and reads them back (its merge-checks step, issue #346); `docs/adopting-assay-gitlab.md` §3's by-hand table lists only the pipeline check; the Forge interface carries no discussion operation; `forge_gitlab.go`'s `gitlabMergeableState` maps `discussions_not_resolved` and `draft_status` to UNKNOWN and `deskflip`'s `mergeable` condition refuses UNKNOWN; `deskflip`'s `reviewer-approved` condition reads `ReviewsAtHead`, which on GitLab reads the project approval-configuration route first; `deskpr create` calls `CreateDraftChange` and nothing after it on GitLab; the client library (`gitlab.com/gitlab-org/api/client-go` v1.46.0) exposes `Discussions.CreateMergeRequestDiscussion`, `GetMergeRequestDiscussion`, `ResolveMergeRequestDiscussion` and `AddMergeRequestDiscussionNote`"
exec-tier: strong
exec-tier-why: "the deliverable IS a merge-gate control on a new forge, spread across a seam addition and three verbs (question b); a plausible-but-wrong shape — a thread the author can resolve and the flip accepts, a resolve that never records the head so a stale resolve reads as current, or a flip that still consults the Premium route on the side and 403s — survives every happy-path test and reopens exactly the blindness #1091 describes (question c)."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit/forge.go: follow-up forge-gitlab/17 (this brief — the merge-hold op set joins the frozen interface with its consumers in the same change; flips to fixed-here when the implementation lands)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: follow-up forge-gitlab/17 (this brief — the discussions-backed implementation, and the flip-side reading of `draft_status` / `discussions_not_resolved`)"
  - "tools/desk/internal/deskkit/forge_github.go: follow-up forge-gitlab/17 (this brief — a typed not-applicable implementation; the GitHub twin of this control is branch protection's required reviewer-App review, already server-side)"
  - "tools/desk/cmd/deskpr/deskpr.go: follow-up forge-gitlab/17 (this brief — opens the marker thread after `CreateDraftChange` on a GitLab-resolved repo)"
  - "tools/desk/cmd/deskpost/forgeclient.go: follow-up forge-gitlab/17 (this brief — approve at head resolves, request-changes and stale-head re-open)"
  - "tools/desk/cmd/deskflip/flip.go: follow-up forge-gitlab/17 (this brief — the GitLab `reviewer-approved` condition keys on the resolved marker thread at the current head; the `mergeable` condition stops refusing the two policy holds the flip itself is about to release)"
  - "docs/adopting-assay-gitlab.md: follow-up forge-gitlab/17 (this brief — the by-hand table gains the discussions-resolved row and §0.1 gains the paragraph naming the gate)"
  - "plugins/assay/references/desk-shell.md: out-of-scope (it documents shell and transport mechanics and names no forge gate; the gate paragraph lives in the GitLab runbook, which is where an adopter provisioning the setting reads)"
  - "tools/create-fleet-gitlab.sh: out-of-scope (it already sets and reads back `only_allow_merge_if_all_discussions_are_resolved`; this brief adds a Verify row against that read-back and changes no line of it)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the status row, the tier column, the wave entry and the issue map row)"
version: 1
id: 20ae7950-c280-453e-af9b-46e633d528e9
---

# Brief 17 — An enforceable merge gate on GitLab Free: the unresolved review thread

## Context

On GitHub the desk's verdict-before-merge gate has a server-side layer behind it: branch
protection requires the reviewer App's approval at head, and a draft PR cannot be merged. On
GitLab Free neither half exists. The `Draft:` prefix is a title string — any Developer can strip
it, the server enforces nothing about who did — and required approvals are a Premium feature, so
on Free the reviewer's approve is advisory (spec §1's second disclosed degradation). Until
2026-09-14 that degradation was disclosed on paper. On that day a GitLab adopter cell on a Free
project found the desk did not merely lack the server-side half — it lost the tool-side half too:
the review-state read consults the Premium approval-configuration route first, gitlab.com Free
answers it 403 rather than the 404 the tree degrades on, and the whole read fails closed. The
verdict verb aborted before posting, the board exited 6, the flip refused, and the day's verdicts
were recorded off-forge (#1091).

GitLab does enforce two merge conditions on every tier, as plain project settings the Projects
API exposes: `only_allow_merge_if_pipeline_succeeds` (matrix row B5, already provisioned) and
`only_allow_merge_if_all_discussions_are_resolved` (row B6, provisioned by the script since #346
but absent from the runbook's by-hand table). The second is a gate the desk can drive: a
resolvable discussion thread on the merge request blocks the merge button while it is unresolved,
and the desk controls when it is opened and when it is resolved. This brief makes that thread the
desk's merge hold — opened with the merge request, released only by the reviewer's approve at the
current head, re-armed by a request-changes verdict or a new head — and points `deskflip`'s
GitLab gate at it. The `Draft:` prefix is kept as the human-facing signal (pilot row B6 proved
the flip works on Free); this gate is additive to it. The human merge stays the outer gate.

files:
- `tools/desk/internal/deskkit/forge.go` — the merge-hold op set on the `Forge` interface (one
  open, one read, one release/re-arm), with its consumers in the same change (spec §6).
- `tools/desk/internal/deskkit/forge_gitlab.go` — the implementation over the discussions API;
  the flip-side reading of `draft_status` and `discussions_not_resolved`.
- `tools/desk/internal/deskkit/forge_github.go` — a typed not-applicable implementation.
- `tools/desk/internal/deskkit/forge-gitlab-mutations.json` and the golden tests beside it — the
  recorded fixtures for open, read, release and re-arm.
- `tools/desk/cmd/deskpr/deskpr.go` — open the hold after `CreateDraftChange` on GitLab.
- `tools/desk/cmd/deskpost/forgeclient.go` — release on approve at head; re-arm on
  request-changes and on any verdict at a head the hold was not released at.
- `tools/desk/cmd/deskflip/flip.go` — the GitLab `reviewer-approved` and `mergeable` conditions.
- `docs/adopting-assay-gitlab.md` — the by-hand table row and the §0.1 paragraph naming the gate.
- `docs/streams/forge-gitlab/README.md` — the status row.
- `CHANGELOG.md`, the v1.0.9 section — the fragment (changelog/forge-gitlab-17-resolved-threads-gate.md)
  this authoring change carried was consumed by the v1.0.9 changelog roll, so the fragment file no
  longer exists and the section is its record; the implementation change adds its own fragment.

single-point-of-failure: the project setting `only_allow_merge_if_all_discussions_are_resolved`
is the ONE server-side control — a Maintainer who flips it off, or a provisioner that never set
it, removes the merge block without touching the desk. Behind it: (a) `deskflip`'s own read —
the flip refuses unless the marker thread is resolved BY the reviewer identity with a resolving
note naming the CURRENT head, so a thread resolved by hand, by the author, or at a stale head
never flips ready, and that refusal fires in the desk tool on a different signal (note author +
head) than the server's (thread state); (b) the human merge, which on GitLab Free is the outer
gate by ruling and which this brief neither replaces nor weakens. The layers are independent by
the rule-10 test: the server refuses the merge button, the tool refuses the flip, the human
refuses the merge, and each fails for a different reason in a different component.

facts:
- Field evidence (#1091, 2026-09-14, a GitLab adopter cell, gitlab.com Free): project
  `/approvals` → 403; per-MR `/merge_requests/:iid/approvals` → 200 with `approved_by`. The
  tree's review read fails the WHOLE read closed on anything but 404 from the project route, so
  `deskpost review` / `security-review` abort, `deskboard actions` exits 6, `deskflip` refuses.
- On Free: `Draft:` is a title prefix any Developer can remove (GitLab drafts doc); required
  approvals, prevent-author and reset-approvals-on-push are Premium (matrix B3, B4, A7). The
  approve endpoint itself works on Free and the desk already pins the head in the verdict note
  body (brief 09) — this brief does not change the verdict write, it adds a merge hold beside it.
- Both merge checks are Free project settings: `PUT /projects/:id` with
  `only_allow_merge_if_pipeline_succeeds` and `only_allow_merge_if_all_discussions_are_resolved`;
  `GET /projects/:id` reads them back. `tools/create-fleet-gitlab.sh` sets both and reads both
  back (measured 2026-09-14 @ a864af26); the runbook's by-hand table in
  `docs/adopting-assay-gitlab.md` §3 lists only the pipeline one.
- Discussions API (Free): `POST /projects/:id/merge_requests/:iid/discussions` with a `body`
  creates a resolvable thread (`notes[0].resolvable: true`, `resolved: false`);
  `POST …/discussions/:discussion_id/notes` replies; `PUT …/discussions/:discussion_id` with
  `resolved=true` or `resolved=false` resolves or re-opens and records `resolved_by` on the
  note. Any Developer on the project can resolve any thread — which is why layer (a) reads WHO
  resolved and at WHICH head, not just whether.
- `detailed_merge_status` (Free) reports `discussions_not_resolved` while a resolvable thread is
  open and `draft_status` while the title carries the prefix; GitLab reports ONE value and does
  not document which of the two wins when both hold — could-not-check without a live project,
  and the flip must not depend on the answer. The tree maps both to UNKNOWN
  (`gitlabMergeableState`), and `deskflip`'s `mergeable` condition refuses UNKNOWN; a GitLab
  draft therefore currently reads UNKNOWN at the flip on every MR, which the field has not yet
  reached because the `reviewer-approved` condition ahead of it refuses first (#1091).
- Marker thread body, fixed text the implementation must use verbatim as its first line so the
  read can find the desk's own thread among human ones:
  `assay-merge-hold: review pending — released by the reviewer's approve verdict at the current head`.
  The resolving reply carries `assay-merge-hold: released` on its first line and `Head: <full sha>`
  on its second; a re-arm reply carries `assay-merge-hold: re-armed` and the reason
  (`request-changes` or `new head <full sha>`).
- Identity: `deskpr create` runs under the worker credential (Developer) and opens the thread;
  `deskpost review` runs under the reviewer credential and resolves it; `deskflip` runs under
  the desk credential and reads it. The reviewer login the flip compares `resolved_by` against
  is the one bound to the reviewer role in the adopter's roster — the same login the
  `reviewer-approved` condition already resolves; nothing new is configured.
- No hook runs on push. A new head does NOT re-arm the server-side layer by itself (the same gap
  Free has for approvals, matrix A7); it is re-armed by the next desk verb that touches the MR —
  `deskpost review` at the new head before it evaluates, or `deskflip` as part of its stale-head
  refusal. Between the push and that verb the server-side layer is down and layers (a) and (b)
  hold. Disclosed, not absorbed.
- Environment for the live Verify rows: `GL_API` = the instance's `/api/v4` base, `GL_TOKEN` = a
  read-capable PAT for the project, `GL_PROJECT` = the URL-encoded project path or numeric id,
  `MR` = the merge request iid. Rows that need them are could-not-check without a live project
  and say so in their class.

## Edition
Minimum GitLab tier: **free**. Every call this brief adds — project-setting read-back,
discussion create, reply, resolve, unresolve, `detailed_merge_status` read — carries no tier
badge on a `Tier: Free, Premium, Ultimate` page (sources). The gate exists precisely so the
Free-tier desk has a server-enforced merge condition that is not the Premium approval-rules
route; on Premium and Ultimate it is additive to the rules the hardening (brief 06) provisions,
and nothing in it consults those rules.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new operation on the `Forge` interface without a consuming verb in the same change (spec
  §6), and no arbitrary-endpoint passthrough on either backend (brief 08's closed surface).
- No gate read on GitLab may consult `GET /projects/:id/approvals` (the Premium
  approval-configuration route). The per-MR approvals read may be LOGGED as information; it
  gates nothing in this brief.
- Public-tree wording only in every file touched: "a GitLab adopter cell"; no cell names,
  session ids, host names or private aliases.

## Task
1. **Seam.** Add to `Forge` a merge-hold op set: open a hold on a change (returns the hold's
   opaque id), read the hold at a change (returns present / resolved / resolved-by login /
   released-at head, or a typed not-applicable), and release or re-arm a hold at a head with a
   reply body. GitLab implements them over the discussions API using the fixed marker text in
   `facts:` and finds its own thread by that first line. GitHub implements all three as a typed
   not-applicable the callers skip — the GitHub twin is server-side branch protection, which
   this brief does not restate. Record fixtures for open, read (unresolved, resolved-at-head,
   resolved-at-stale-head, resolved-by-non-reviewer), release and re-arm in the GitLab golden
   set; the surface test's enumerated-operation list grows by exactly the ops added.
2. **`deskpr create`.** On a GitLab-resolved repo, after `CreateDraftChange` succeeds, open the
   hold on the new MR. Failure to open is a loud non-zero exit naming the MR that now exists
   without its hold (the change is created; the gate is not armed; the operator must know) —
   never a silent success.
3. **`deskpost review`.** On a GitLab-resolved repo, after the verdict write succeeds:
   `approve` at the current head releases the hold with the `released` reply naming that head;
   `request-changes` re-arms it with the `request-changes` reason; any verdict whose head differs
   from the head a resolved hold names re-arms first, then applies the verdict's own release or
   re-arm. A verdict that posts but whose hold write fails exits non-zero and says which half
   landed. The security-review verb touches the hold not at all — the hold is the correctness
   verdict's, and the security lane keeps its own gate (`deskflip`'s `security-verdict`).
4. **`deskflip`.** On a GitLab-resolved repo: (4a) the `reviewer-approved` condition is
   satisfied when the hold is present, resolved, `resolved_by` is the reviewer login, and the
   resolving reply's `Head:` equals the current head; any other state is a refusal naming which
   part failed. A hold resolved at a STALE head is re-armed as part of that refusal so the
   server-side layer is back up at the next tick. The condition consults no approval route; the
   per-MR approvals read, if made, is printed as information. (4b) the `mergeable` condition on
   GitLab refuses `conflict` and `broken_status` as before, stays could-not-check on `checking`,
   `unchecked` and the empty string, and treats `draft_status` and `discussions_not_resolved` as
   NON-blocking at the flip only when (4a) has already passed — those are the two holds the flip
   itself is about to release, and refusing on them would make every GitLab draft unflippable.
   Every other value stays UNKNOWN → refusal, exactly as `gitlabMergeableState` documents.
5. **Docs.** `docs/adopting-assay-gitlab.md`: the §3 by-hand table gains the row `Require all
   threads resolved before merge — PUT api/v4/projects/:id —
   only_allow_merge_if_all_discussions_are_resolved: true`, and §0.1 gains one paragraph naming
   the gate: what the marker thread is, who resolves it, that the `Draft:` prefix stays as the
   signal, and that the human merge stays the outer gate.
6. **Fragment.** Add the implementation's own `changelog/<slug>.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `curl -sf -H "PRIVATE-TOKEN: $GL_TOKEN" "$GL_API/projects/$GL_PROJECT"` | exit 0; the JSON field `only_allow_merge_if_all_discussions_are_resolved` is `true` and `only_allow_merge_if_pipeline_succeeds` is `true` — the provisioning flag read back from the project, not from the script's log. Could-not-check without a live project | gate:human +dereference |
| 2 | `deskpr create` on a GitLab-resolved repo, then `curl -sf -H "PRIVATE-TOKEN: $GL_TOKEN" "$GL_API/projects/$GL_PROJECT/merge_requests/$MR/discussions"` | exit 0; exactly one discussion whose `notes[0].body` begins `assay-merge-hold: review pending` with `notes[0].resolvable` `true` and `notes[0].resolved` `false`, authored by the worker login. Could-not-check without a live project | gate:human +flow |
| 3 | with row 2's thread unresolved: `curl -s -o /dev/null -w "%{http_code}" -X PUT -H "PRIVATE-TOKEN: $GL_TOKEN" "$GL_API/projects/$GL_PROJECT/merge_requests/$MR/merge"` | prints `405` — the server refuses the merge while the hold is unresolved (GitLab answers 405 when the merge request is not mergeable); and `curl -sf … "$GL_API/projects/$GL_PROJECT/merge_requests/$MR"` reports `detailed_merge_status` as `discussions_not_resolved` or `draft_status`, never `mergeable`. Could-not-check without a live project and a token allowed to merge | gate:human +mutation |
| 4 | `deskpost review --verdict approve` at the current head, then row 2's discussions read | exit 0; the same discussion now has `notes[0].resolved` `true`, `notes[0].resolved_by.username` equal to the reviewer login, and a reply note whose body begins `assay-merge-hold: released` and whose `Head:` line equals the MR's `sha`. Could-not-check without a live project | gate:human +flow |
| 5 | push one further commit to the MR's source branch, then `deskflip --pr $MR` | non-zero exit; the refusal names condition `reviewer-approved`, the head the hold was released at and the current head; row 2's discussions read afterwards shows `notes[0].resolved` `false` and a reply beginning `assay-merge-hold: re-armed` with `new head`. Could-not-check without a live project | gate:human +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskflip/ -run TestFlipGitLabMergeHold -v -timeout 300s` | exit 0; output contains `PASS`; the recorded fixture asserts the flip refuses on each of: hold absent, hold unresolved, hold resolved by a non-reviewer login, hold resolved at a stale head — and that NO request in the flip's GitLab path is addressed to the project approval-configuration route | check +mutation |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v -timeout 300s` | exit 0; output contains `PASS`; the golden set carries the open, read, release and re-arm recordings, and `gitlabMergeableState` still maps `discussions_not_resolved` and `draft_status` to UNKNOWN (the flip-side allowance lives in `deskflip`, not in the mapping) | check |
| 8 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestForgeGithubGolden' -v -timeout 300s` | exit 0; output contains `PASS` — the GitHub backend's recorded behaviour is unchanged by the seam addition | check +neighbour |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSurface -v -timeout 300s` | exit 0; output contains `PASS`; the enumerated-operation count grew by exactly the merge-hold ops and no passthrough method appeared | check |
| 10 | `grep -c 'only_allow_merge_if_all_discussions_are_resolved' docs/adopting-assay-gitlab.md` | prints `2` or more — the by-hand table row and the §0.1 paragraph both name the setting | check |
| 11 | `statusgen --root . --consumers` | exit 0 | check |
| 12 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The thread is created but not resolvable (a plain note), so the server never blocks the merge | row 2 (`resolvable: true`) and row 3 (the 405) |
| The provisioner sets the flag on a fresh project but the adopter's existing project never had it, so every layer below is armed and the server-side one is not | row 1 (read-back from the project itself) |
| The reviewer's approve resolves the thread without recording the head, so a resolve at an old head reads as current after a push | row 4 (`Head:` equals `sha`) and row 5 (the stale-head refusal + re-arm) |
| The author, who is a Developer, resolves the thread by hand and the flip accepts it | row 6 (resolved-by-non-reviewer fixture refuses) |
| The flip still consults `GET /projects/:id/approvals` on the side and 403s on Free — #1091 all over again | row 6's no-request assertion against that route |
| The flip's `mergeable` condition refuses `draft_status` on every GitLab draft, so the gate is correct and nothing can ever flip | row 6 (the flip passes on the resolved-at-head fixture, whose MR fixture reports `draft_status`) |
| The seam addition changes GitHub's recorded behaviour | row 8 |
| Which of `draft_status` / `discussions_not_resolved` GitLab reports when both hold | **no row** — could-not-check without a live instance, and the design does not depend on the answer (4b treats both the same); row 3 accepts either. Review-only |
| A verdict posts, the hold write fails, and the exit code says success | **no row** that runs it — the fixture set in row 7 records the failure shape; whether the verb's exit reflects it is a reviewer read of step 3. Review-only |

### Dispatch checklist
```
[x] 1. Rows discriminate — row 3 goes red on a non-resolvable note; row 5 goes red on a resolve that never named the head; row 6 goes red on a flip that accepts a hand-resolved thread or still touches the Premium route.
[x] 2. Facts dated — the field evidence is #1091 (2026-09-14); the tree state is measured 2026-09-14 @ a864af26 and recorded in sources:.
[x] 3. Self-contained — the marker text, the API calls, the identities, the two policy-hold values and the env vars for the live rows are all here.
[x] 4. Risk answers match files: — desk tooling and a runbook; no regulated surface, no customer-facing behaviour, nothing irreversible (a thread can be re-opened, a setting flipped back), no sensitive data.
[x] 5. gate-why — not required: gate model, all four answers no.
[x] 6. Effort honest — one op set on the seam with two backends and fixtures, three call sites, one doc: M. Not L because the verdict write, the flip's condition list and the provisioner are all already in place and this brief hangs one read and one write on each.
[x] 7. Shared value — the marker text is a wire format three verbs agree on; consumers: enumerates every site; rows 2, 4 and 5 are the flow rows.
[x] 8. Pre-mortem run; the two review-only items are named.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — 2026-09-15 sonnet-5-verifier (verify-desk dispatch), offline — **VERIFY: PASS**

Pin: `medici-finance/assay` main `fe734253c1862774f4fa7ab018779945a108bf9d` (worktree cut from `refs/remotes/origin/main`). Isolated worktree, offline throughout (`KUBECONFIG=/dev/null`), no live GitLab session or credential opened. `gate: model`, `risk {all no}`, `irreversible: no`.

| # | Result |
|---|---|
| 1 | COULD-NOT-CHECK — needs a live GitLab project; this pass is offline-only by design (brief's own Expect column: "Could-not-check without a live project") |
| 2 | COULD-NOT-CHECK — needs a live GitLab merge request; same reason |
| 3 | COULD-NOT-CHECK — needs a live GitLab merge request and a merge-capable token; same reason |
| 4 | COULD-NOT-CHECK — needs a live GitLab merge request; same reason |
| 5 | COULD-NOT-CHECK — needs a live GitLab merge request; same reason |
| 6 | PASS — `go test ./cmd/deskflip/ -run TestFlipGitLabMergeHold -v -timeout 300s` exit 0, `PASS`. All 10 subtests pass: hold_absent_refuses, hold_unresolved_refuses, resolved_by_non_reviewer_refuses, resolved_at_stale_head_refuses_and_rearms, resolved_at_current_head_by_reviewer_passes, mergeable_condition_allows_draft_status_and_discussions_not_resolved, mergeable_condition_still_refuses_every_other_unknown_reason, mergeable_condition_still_refuses_conflicting, mergeable_condition_unaffected_on_github, touches_no_other_forge_op. The last one is structural, not a request-count assertion: `mergeHoldFake` embeds a nil `deskkit.Forge`, so a call to `ReviewsAtHead` or the approvals route would panic — none of the fixture cases panicked, which is only possible if `checkMergeHoldApproved` never reaches that route |
| 7 | PASS — `go test ./internal/deskkit/ -run TestForgeGitlabGolden -v -timeout 300s` exit 0, `PASS`. Includes 11 merge-hold golden subtests (open_merge_hold, open_merge_hold_not_resolvable_refused, read_merge_hold_absent/unresolved/resolved_at_head/ignores_non_released_reply_head_line/ignores_released_reply_from_wrong_author/resolved_by_non_reviewer, set_merge_hold_release/rearm/absent_refused). Traced `gitlabMergeableState` (forge_gitlab.go) directly: `discussions_not_resolved` and `draft_status` still fall through its `default` case to `MergeableUnknown` — the flip-side allowance lives only in `deskflip`, confirmed at task 4b below |
| 8 | PASS — `go test ./internal/deskkit/ -run TestForgeGithubGolden -v -timeout 300s` exit 0, `PASS`. 34 GitHub golden subtests plus `TestForgeGithubGoldenCount` pinning 50 operations, unchanged by the seam addition |
| 9 | **Stale test name in the brief's own row, not an implementation defect.** `go test ./internal/deskkit/ -run TestForgeSurface -v -timeout 300s` exits 0 and prints `PASS`, but the `-run` filter matches 0 tests (`[no tests to run]`) — there is no `TestForgeSurface` in this tree. The property the row describes (enumerated-operation count + no passthrough) is actually implemented as `TestForgeNoPassthrough` in `forge_surface_test.go`. Ran that directly as supplementary evidence: exit 0, `PASS`, including subtest `method_set_equals_the_committed_inventory`, which logs "the frozen surface is 45 operations, every one tabulated in ../../../../docs/streams/forge-gitlab/inventory.md" and `neither_backend_exports_a_method_outside_the_interface` (both backends checked). This substantively confirms row 9's claim; flagging the row's own test-name drift as a small follow-up for whoever authors the next forge-gitlab brief revision |
| 10 | PASS — `grep -c 'only_allow_merge_if_all_discussions_are_resolved' docs/adopting-assay-gitlab.md` exit 0, prints `2` |
| 11 | PASS — `statusgen --root . --consumers` exit 0. Output: "consumers: no brief files in the diff against fe734253c1862774f4fa7ab018779945a108bf9d — nothing to corroborate" (expected: this pass runs on merged main with no open brief diff) |
| 12 | PASS — `statusgen --root . --lint` exit 0, output contains `LINT: PASS` (repo-wide pre-existing NOTICE lines present, none naming this brief or its files) |

**Substance checks (traced code, not just test names).**
1. Marker-text wire format traced at source: `mergeHoldMarkerBody` (forge_gitlab.go:2318), `mergeHoldReleasedMarker` / `mergeHoldRearmedMarker` (forge_gitlab.go:2324-2325) are the exact literals `ReadMergeHold`/`SetMergeHold` share — a single defined constant per marker line, not independently retyped strings that could drift.
2. `checkMergeHoldApproved` (flip.go:820-859) reads `hold.State`, `hold.ResolvedBy` (via `deskkit.SameActor` against the reviewer login) and `hold.Head` against the current head — never a verdict-note lane and never an approvals route call, matching task 4a exactly. A stale-head resolve re-arms via `SetMergeHold` as part of the refusal (flip.go:839-852).
3. `checkMergeableCondition` (flip.go:787-807) is the ONLY place `draft_status`/`discussions_not_resolved` are treated as non-blocking; `gitlabMergeableState` (forge_gitlab.go:284-293) is untouched and still maps both to `MergeableUnknown` — confirmed by direct read, not inference.
4. Call-site check: `checkMergeableCondition` runs before `checkMergeHoldApproved`/`checkReviewerApproved` in `runFlip` (flip.go:254 vs 285/290), but the flip only reports ready when EVERY condition passes — so the leniency at flip.go:787 is sound regardless of call order, exactly as the code's own comment states (flip.go:780-786).

RISK-VALUE: DERIVED — `mergeableLeniencyValues = {"draft_status","discussions_not_resolved"}` @ tools/desk/cmd/deskflip/flip.go:797 (checked case-insensitively, `strings.ToLower`) — GitLab's `detailed_merge_status` is a closed, documented enum where each value names exactly one distinct block reason (draft-prefix vs. unresolved-discussion vs. CI vs. conflict, etc. — GitLab merge_requests API). Exempting precisely these two named reasons, and no others, is sound because the flip's own `reviewer-approved` condition (`checkMergeHoldApproved`, flip.go:820) independently and unconditionally re-checks the discussion-resolution reason on every run, and the human-facing `Draft:` prefix is the flip's already-established parallel signal (pilot row B6, pre-existing). Every other named reason in the enum (`blocked_status`, `broken_status`/`conflict`, `checking`, `unchecked`, empty) still hits the hard refusal or could-not-check path at flip.go:788-804 — confirmed directly by the `mergeable_condition_still_refuses_every_other_unknown_reason` and `mergeable_condition_still_refuses_conflicting` subtests (row 6). Ranked top because it is the one place in this diff that actively LOOSENS a check rather than tightening or reading one (every marker-text literal only narrows what the read matches); not irreversible — the item's own frontmatter (`irreversible: no`) and single-point-of-failure note already record why (a wrong exemption is fixed by an edit and redeploy, and the human merge stays the outer gate regardless of what the flip reports).

**Flip.** gate:model, risk all no, irreversible no, and all offline-runnable rows (6-12) PASS with rows 1-5 could-not-check strictly by design (live GitLab infrastructure; this pass is offline-only per assignment). README row 17 flipped `implemented` → `verified`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers both questions: (1) what single control stands between a stale or hand-resolved
thread and a ready-flip, and is it acceptable? (2) does any row prove the tool-side layer refuses
with the server-side layer bypassed — row 6's non-reviewer and stale-head fixtures are meant to
be that proof; if they only walk the resolved-at-head happy path, the answer is no.
