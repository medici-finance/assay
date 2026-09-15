---
brief: assay:assay:forge-neutral:19
title: Human-only surfaces made server-side — merge, workflow-file pushes, rulesets, variables, App installs
why: >-
  The design principle this whole series follows is two-sided: "every desk action a human
  does through the forge CLI today gets a desk verb with a forge-neutral mapping; the
  human-only ones stay human-only because SERVER-SIDE permissions make them so, not because
  the desk lacks a verb." Briefs 14-17 answer the first half. Nobody has yet stated, surface
  by surface, that the second half is TRUE — and for the loudest surface (merge to a
  protected branch) it currently is not: today's desk Apps hold `contents: write` +
  `pull_requests: write`, which is sufficient to merge a PR outright, so "humans merge"
  is a convention every App honours voluntarily, not a wall the platform enforces. A
  reader of this stream deserves to know which human-only claims are backed by a
  permission the platform itself refuses to grant, and which are backed by nothing but
  every App's own restraint so far.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
exec-tier: strong
exec-tier-why: "getting item 1 wrong in either direction is a security judgment: understating the gap leaves a real merge-bypass unflagged, and a proposed fix that is subtly mis-specified (a required check a bot can also satisfy, a review-login match that accepts the wrong identity class) would read as closed when it is not — this is not a fact a model self-certifies (question c)."
gate-why: >-
  Item 1 documents a real, currently-UNENFORCED gap in who can merge to this repo's
  protected branch — App tokens hold the permission scope to do it today, and only
  convention says they do not. That is squarely release/custody-adjacent: a brief that
  states this plainly and proposes closing it needs a human's sign-off on both the
  characterisation (is the gap real, and is nothing already catching it that this brief
  missed?) and the proposed fix (does `human-approved` actually close it, or trade it for
  a narrower version of the same hole?) before anyone builds against it.
issues: [992]
schema: brief-v2
authored: 2026-09-13 by forge-neutral authoring session
sources:
  - "#992 — the tracking issue for this five-brief series; its tracking comment states the
    design principle quoted above verbatim and names this brief (18) as the series' close"
  - "docs/streams/forge-neutral/identity.md — the forge-qualified roster grammar
    (`[role=]<forge>:<slug-or-login>[:<id>]`) the proposed `human-approved` workflow reads a
    trusted-human login from, described here generically (this file names no private
    roster path)"
  - "docs/streams/forge-neutral/README.md §Purpose claim 2 and its measured matrix — the
    identity model this brief's item 1 fix extends into a NEW surface (who reviewed) rather
    than the surfaces already covered (who authored/committed)"
  - "docs/streams/apps-installer/brief-03-deskapps-install-prove.md:78 — an independent,
    pre-existing rule in this repo, \"never call `gh workflow run`\", reached for a
    permission-widening reason; this brief's item 1 gap is the same `actions: write`
    scope-granularity problem read from the merge side rather than the dispatch side"
  - "tools/desk/cmd/repohardenguard/main.go, checklist.go — the existing GET-only,
    three-state (checked-ok / checked-wrong / could-not-check) hardening-settings reader
    already in this tree; its own doc comment states GitHub renders an admin-gated
    `bypass_actors` omission identically whether the setting is on or off, which is the
    same admin-visibility caveat this brief's audit-script Verify row inherits"
  - "tools/desk/cmd/repohardenguard/repohardenguard_test.go:521-537 — `TestShippedChecklistParses`
    skips when `docs/repo-hardening-checklist.md` (planned) is absent from a checkout; freshness-checked
    2026-09-13 @ cc4f9e22 that the file is in fact absent from this public tree, so
    `repohardenguard` cannot run against `medici-finance/assay` today without a checklist
    supplied via `--checklist`"
  - "freshness-checked 2026-09-13 @ cc4f9e22 (origin/main) — live read, `gh api
    repos/medici-finance/assay/rulesets` and the per-ruleset detail endpoint for each of
    the three ids returned (20301257 `protect-main`, 20872509 `leak-sweep`, 20301270
    `protect-release-tags`): `protect-main` carries a `pull_request` rule with
    `required_approving_review_count: 1` and no `pull_request` restriction on which
    identity may supply that approval; neither branch ruleset's JSON carries a
    `bypass_actors` key at all (repohardenguard's documented absent-means-none-or-gated
    ambiguity applies — this read came back with every other field populated, which is
    the closest a live, non-tool-mediated read gets to ruling out the admin-gated-null
    case, but this brief does not claim admin-level certainty a proper `repohardenguard`
    run would); `leak-sweep` carries the sole `required_status_checks` entry
    (`context: leak-sweep`) and `required_approving_review_count: 0`. This is the CURRENT
    observed state this brief's item 1 facts and Verify row 1 describe — not an invented
    ruleset id"
domain: complicated
consumers:
  - "docs/streams/forge-neutral/brief-19-human-only-surfaces-server-side.md: fixed-here (this document)"
  - "docs/streams/forge-neutral/README.md: fixed-here (brief-table row)"
  - ".github/workflows/human-approved-gate.yml (new): out-of-scope — specified in Task 1
    below (the trigger, the check, the roster read it performs); a follow-on brief authors
    and lands the workflow file itself, gated the same way any change to `.github/workflows/`
    is (a human review, per item 2's own finding)"
  - "the `protect-main` ruleset's required-status-checks list (medici-finance/assay repo
    settings, id 20301257): out-of-scope — adding `human-approved` to it is a repo-admin
    act (item 3's own finding: rulesets are Maintainer/admin-only), performed by a human
    once the workflow above exists, not by this brief or any tool"
  - "tools/desk/cmd/repohardenguard: out-of-scope — already ships the general three-state
    hardening-settings reader this brief's item-4 audit role wants; wiring it to a shipped
    `docs/repo-hardening-checklist.md` (planned) row set for `medici-finance/assay` is a follow-on,
    not this brief (the checklist file itself is not part of this public checkout today,
    per the freshness fact above)"
version: 1
id: 7f1c9a4e-6b3d-4e5a-9c2f-1d8e4a7b0c63
---

# Brief 18 — Human-only surfaces made server-side

## Context

files:
- `.github/workflows/human-approved-gate.yml` (planned) — out-of-scope for this brief,
  specified here, authored by a follow-on — the workflow this brief's item 1 proposes.
- `medici-finance/assay`'s `protect-main` ruleset (repo settings, id `20301257`) — the
  required-status-checks list a human adds `human-approved` to, once the workflow exists.
- `tools/desk/cmd/repohardenguard/` — the existing read-only audit tool this brief's item 4
  points at rather than duplicates.

This is the closing brief of the five-brief series #992 tracks. Briefs 14-17 give the desk a
forge-neutral verb for every action a human currently performs by hand through the forge
CLI — dispatching a run, approving a gate, labelling, closing, reading a run log. This brief
is the other half of the same sentence: for the handful of actions that stay human-only, it
states PLAINLY, surface by surface, whether that is because the platform itself refuses the
permission to a non-human actor, or merely because every desk App has so far chosen not to
use a permission it already holds. Those are not the same guarantee, and conflating them is
how a convention quietly becomes load-bearing without anyone deciding it should be.

single-point-of-failure: for the one surface this brief finds genuinely unenforced (item 1,
merge to a protected branch), the single control today is every desk App's own restraint —
no server-side rule stops an App holding `contents: write` + `pull_requests: write` from
merging a PR it or a colluding App approved. There is NO second layer behind that restraint
today. The proposed `human-approved` required check is this brief's answer to that gap, and
it is deliberately a SERVER-SIDE control (a required status check the ruleset itself
enforces) rather than a second convention, because a second convention would share the exact
failure mode as the first: something a well-behaved actor honours and a compromised or
mis-configured one does not.

facts:

**1. Merge to protected branches — NOT currently server-side-enforced. A real, unenforced
gap, not yet closed.**

The three role Apps this repo's desk verbs act as (`assay-worker-app`, `assay-reviewer-app`,
`assay-verifier-app`, per `identity.md`'s roster) each hold `contents: write` and
`pull_requests: write` on their installation — both scopes exist to let them push branches,
open PRs, and (`deskflip`) flip a PR ready. Nothing about those two scopes is merge-specific,
but together they are SUFFICIENT to merge: `pull_requests: write` covers
`PUT /repos/{o}/{r}/pulls/{n}/merge`, and GitHub does not offer a narrower scope that grants
"open/comment/review a PR" without also granting "merge it". The live ruleset read
(freshness fact above) confirms the gap is not caught by branch protection either:
`protect-main`'s `pull_request` rule requires `required_approving_review_count: 1`, but the
rule carries no restriction on WHICH identity may supply that approval — a review posted by
any accepted account, human or bot, satisfies the count. So today, an App holding
`pull_requests: write` could post an approving review (its own, or a second bot's) and then
merge, and the ruleset would not refuse either call. **Nothing currently stops this except
every App's documented restraint** — `deskflip`'s own gate (`tools/desk/cmd/deskflip/flip.go`) checks
for a human merge decision before it ever proposes readiness, but that is the DESK's
convention, not a permission the platform withholds. Convention-only is not the same claim as
server-side-enforced, and this brief does not round it up to one.

**Proposed fix — a `human-approved` required status check.** A GitHub Actions workflow
(`.github/workflows/human-approved-gate.yml` (planned), out of scope for this brief — see Task 1) triggers on
`pull_request_review`, reads the review's submitting login, and checks it against the
trusted-human roster concept `identity.md` already establishes for this stream (described
here generically: a configured allow-list of human logins this repo trusts to approve merges
— never the private roster file path itself). When the reviewing login is on that list AND
the review is `APPROVED` AND its `commit_id` equals the PR's current head SHA (the same
at-head property `identity.md`'s corroboration table already defines for GitHub), the
workflow posts a `success` commit status named `human-approved`; any other case — a bot
reviewer, an unlisted human, a stale `commit_id` — posts `failure` or posts nothing (an
absent status is itself not a pass, per the three-state discipline this stream already
uses elsewhere). Adding `human-approved` to `protect-main`'s required-status-checks list
(a human, repo-admin act — item 3 below) then makes the ruleset itself refuse a merge that
has not been approved by a listed human at head, closing the gap SERVER-SIDE: the check that
decides is a status the ruleset enforces, not a convention an App chooses to honour.

**2. Workflow-file pushes — GitHub: genuinely server-side-enforced today. GitLab: no
equivalent scope; needs a protected-branch + CODEOWNERS rule instead.**

GitHub's fine-grained permission model carries a dedicated `workflows` scope gating writes
under `.github/workflows/`; a token or App installation without it gets a 403 on any attempt
to create or update a file under that path, independent of whatever `contents` permission it
holds. Freshness-checked: no role App's installation in this repo's roster is granted
`workflows` (the roster names `contents`, `pull_requests`, `issues`, `metadata` per role —
`workflows` appears nowhere), so every write under `.github/workflows/` genuinely requires a
human's own credential today; this is not a convention, it is a scope no App is handed.

GitLab has **no equivalent permission scope** for `.gitlab-ci.yml` — a GitLab access token or
service account with Developer-or-above role and `write_repository` can push a change to that
file exactly as it can push any other tracked file; there is no scope-level lock comparable
to GitHub's `workflows` permission. So on GitLab, the ONLY available control is a
protected-branch rule plus a CODEOWNERS entry that names `.gitlab-ci.yml` (and any file it
`include`s) and requires human approval on any change touching it. This is a real
architectural asymmetry between the two forges' permission models, not an oversight in
either backend: GitHub's fix is a scope no App holds; GitLab's fix is a review-gate rule,
because GitLab's platform genuinely has no scope-level equivalent to hold back.

**3. Rulesets / branch protection settings — server-side-enforced on both forges today.**

GitHub: repository rulesets (the same object type the live read above queried) can only be
created, edited, or deleted by a repository admin or org owner — no fine-grained PAT
permission and no GitHub App permission grants ruleset write access to a non-admin actor;
`current_user_can_bypass: "never"` on all three of this repo's rulesets (freshness fact) is
itself evidence no installed actor, admin or otherwise, currently holds a bypass. GitLab:
protected-branch settings (the CE analogue) are Maintainer-role-and-above only, enforced by
GitLab's own role hierarchy, with no lower-privileged token able to alter them. Both are
genuinely closed today; this brief adds nothing here.

**4. Repo/CI variables — server-side-enforced on both forges today.**

GitHub: repository and organisation Actions secrets/variables are writable only by a
repository admin (repo-level) or org owner (org-level); no App permission scope grants
writing them, and reading a secret's VALUE back is never possible through the API on either
forge — only overwriting is. GitLab: CI/CD variables at the project or group level are
Maintainer-role-and-above only, same as protected-branch settings. Both are genuinely closed
today.

**5. App/OAuth installation — server-side-enforced on both forges today.**

GitHub: installing a GitHub App (or authorizing an OAuth App) onto an organisation's
repositories is an org-owner-only action; a repository admin who is not an org owner cannot
grant an App installation across the org, and no App can install another App (there is no
API for an App to self-install or install a peer). GitLab: installing an integration or
granting an application access at the group/project level is admin-only, matching GitHub's
shape. Both are genuinely closed today.

**Audit-script role — what already exists, and what this brief does not build.**
`tools/desk/cmd/repohardenguard` is already the general instrument for turning claims 3-5
above into a checked, three-state fact instead of a claim someone re-derives by hand every
time: it reads a repo's live settings against a checklist document and reports
`checked-ok` / `checked-wrong` / `could-not-check`, with the same admin-visibility honesty
this brief's own live read above inherits (an admin-gated field absent from a non-admin
token's response is indistinguishable from that field being genuinely off, and the tool
refuses to round that ambiguity to a pass). `docs/repo-hardening-checklist.md` (planned) — the document
`repohardenguard` reads its expected values from — is not part of this public checkout today
(`TestShippedChecklistParses` skips on its absence rather than failing), so running
`repohardenguard --repo medici-finance/assay` right now would refuse for want of a covering
row, not because the tool is missing. Wiring a shipped checklist for this repo is a follow-on
this brief names but does not take on; the Verify table below uses a direct `gh api` read
instead, which is executable today with no new tooling.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. This applies doubly to item 1's fixture Verify row: the merge-refusal
  demonstration runs against an isolated FIXTURE repo carrying the same ruleset shape,
  never against this repo's own `main` — a successful merge attempt there would BE the
  incident this brief exists to prevent, not a test of it.
- Stop at `implemented` — you do not set verified/done (and this brief's own Task section
  is scoped to documentation + a proposed workflow spec; the workflow file and the ruleset
  edit are named follow-on work, per the `consumers:` list above).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not invent a ruleset id, a permission-scope name, or a live setting value. Every
  concrete number and id in this brief (`20301257`, `20872509`, `20301270`, the
  `required_approving_review_count` and `required_status_checks` values) came from a live
  `gh api` read against `medici-finance/assay` at the freshness-checked commit; re-run the
  same read rather than trusting a stale copy if this brief is revisited later.
- The `.github/workflows/human-approved-gate.yml` (planned) workflow this brief specifies is itself a file under
  `.github/workflows/` — per item 2's own finding, its own first push is a human-reviewed
  act, and it is authored by a follow-on brief, not landed here.

## Task
1. **Specify `.github/workflows/human-approved-gate.yml` (planned)** (the follow-on implements it; this brief fixes its
   contract). Trigger: `pull_request_review` (type `submitted`). Steps: (a) read the review's
   `user.login` and compare it against the repo's configured trusted-human login list (the
   `identity.md` roster concept, read via whatever repo-variable or config file the follow-on
   chooses — this brief does not mandate a storage shape, only that it be a repo-scoped,
   human-editable allow-list, never a hard-coded literal in the workflow file itself); (b)
   confirm `review.state == "APPROVED"`; (c) confirm `review.commit_id` equals the PR's
   current head SHA (`identity.md`'s at-head corroboration rule — a review against a
   superseded head must not satisfy the check); (d) on all three passing, `POST
   /repos/{o}/{r}/statuses/{sha}` with `state: "success"`, `context: "human-approved"`; on any
   failing, post `state: "failure"` naming which condition failed (never post nothing on a
   reviewed-but-failing case — an absent status must stay distinguishable from a checked
   failure). The workflow's own token needs only `statuses: write` and `pull-requests: read` —
   name this explicitly in the follow-on so nobody over-grants it `contents: write` "to be
   safe".
2. **Name the ruleset edit as a human, repo-admin, one-time act.** Once the workflow exists
   and has run green at least once, a human adds `human-approved` to `protect-main`'s (id
   `20301257`) `required_status_checks` list, alongside the existing `leak-sweep` entry from
   the `leak-sweep` ruleset (id `20872509`) — two separate rulesets, both required, is the
   current shape and this brief does not propose merging them. This brief does not perform
   the edit; it names who does (a human) and what closes (the item-1 gap) when they do.
3. **State the do-nothing verdict for items 2-5 explicitly**, so a future reader does not
   re-derive it: GitHub's `workflows` permission scope (item 2), rulesets (item 3), Actions
   variables/secrets (item 4), and App/OAuth installation (item 5) are already genuinely
   server-side-enforced on GitHub; items 3-5 are equally enforced on GitLab via its
   Maintainer-role hierarchy; item 2 on GitLab has no scope-level equivalent and must be a
   protected-branch + CODEOWNERS rule over `.gitlab-ci.yml` instead — named here as the
   GitLab-side follow-on for whichever brief next templates a GitLab adopter's protected-file
   settings (this brief does not author that CODEOWNERS template; `gitlab-ci-half.md` is the
   sibling doc it would live alongside).
4. **Name the ruleset-audit instrument** rather than build a new one: `repohardenguard`
   already exists for exactly this class of check (claims 3-5, plus item 1 once
   `human-approved` lands) and already carries the admin-visibility honesty this brief's own
   facts rely on. Landing a `docs/repo-hardening-checklist.md` (planned) row set for
   `medici-finance/assay` — the one thing standing between "the tool exists" and "the tool
   runs here" — is named as a follow-on, not performed by this brief.

## Verify (executable — no prose-only DoD items)

This brief follows brief 13's Class-column convention, using only the classes `statusgen
--lint` recognises: `check:ci` (hermetic, network-off), `check` (env-bound, runner-executed —
rows 3 and 4 are `check` rows whose COMMAND text, not the class token, restricts them to a
throwaway fixture repo, never a live shared one), `+dereference` (resolves a claim rather
than counting presence) and `+flow` (row 3 exercises the cross-component path end to end: a
branch push, a review post, and a merge attempt against the ruleset's own enforcement, not
one component in isolation).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check +dereference | `gh api repos/medici-finance/assay/rulesets` then `gh api repos/medici-finance/assay/rulesets/<id>` for each returned id | `protect-main` (20301257) carries a `pull_request` rule with `required_approving_review_count: 1` and NO per-identity restriction on the approver; NEITHER branch ruleset's JSON carries a `bypass_actors` key; `leak-sweep` (20872509) carries `required_status_checks: [{"context":"leak-sweep"}]` and `required_approving_review_count: 0`. Confirms this brief's item-1 facts against LIVE state, not a stale description — re-run this row rather than trust the frozen numbers if time has passed |
| 2 | check | `grep -n -A5 '^var requiredDuties = \[\]Duty{' tools/desk/internal/deskkit/preflight.go` — reads the fixed duty set every role App's installation grant is checked against (`checkAppScopes`, same file) | the block lists exactly three `Duty` entries — `pull_requests`, `issues`, `contents` — and no `workflows` entry appears anywhere in it: `workflows` is not among the three permissions every role App is gated on, so no role App installation is required (or checked) to carry it — confirms item 2's GitHub half is genuinely closed, not merely undocumented |
| 3 | check +flow | **FIXTURE REPO ONLY — never `medici-finance/assay` (see Ground rules).** On a throwaway fixture repo carrying the SAME ruleset shape as `protect-main` + `leak-sweep` **plus** the proposed `human-approved` required check already wired to a stub workflow that only ever posts `failure`: push a branch as a worker-App token, post an APPROVING review as a second bot-App token (simulating collusion), then attempt `gh api repos/<fixture>/pulls/<n>/merge -X PUT` as either token | **REFUSED** (422/405, "Required status check \"human-approved\" is expected") — proves the proposed fix closes the collusion path the item-1 facts describe, entirely on a fixture; this row MUST NEVER be pointed at `medici-finance/assay` itself, since a SUCCESSFUL merge in that variant would be the exact incident this brief exists to prevent, not a demonstration of the fix |
| 4 | check | **FIXTURE REPO ONLY — never `medici-finance/assay` (see Ground rules).** Same fixture, same stub workflow now posting `success` for a review from a login on the fixture's own trusted-human list at the PR's head SHA | merge is **NOT blocked by `human-approved`** (the other required checks may still gate it) — the POSITIVE control proving the check is a real gate a legitimate human approval clears, not a check that always fails closed regardless of who approved |
| 5 | check | `gh api repos/medici-finance/assay/rulesets/20301257 --jq '.rules[] \| select(.type=="pull_request")'` after item-2's ruleset edit lands (future re-run, not at authoring time) | `required_status_checks` (via the paired `leak-sweep`-style ruleset) lists `human-approved` alongside `leak-sweep` — the audit confirms the proposed fix was actually applied, not merely proposed |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| This brief overstates item 1 as already closed, when it is not | row 1 — the live read is the check, run against real state, not this brief's prose |
| This brief understates items 2-5, missing a real gap in one of them | Task 3's explicit per-item statement + row 2 (item 2's GitHub half re-verified live, not assumed) |
| The proposed `human-approved` check can itself be satisfied by a bot posing as a trusted human (a compromised or misconfigured login match) | out of scope for THIS brief's Verify table (no implementation exists yet to test) — named as the first thing the follow-on's own Verify table must cover, alongside a negative row for a bot-authored review that happens to share a login string with a trusted human |
| The fixture demonstration (row 3) is quietly run against a real, shared repo instead of a throwaway one | Ground rules + Review — a Review-time check of which repo the command in row 3's evidence actually names, not a Verify row (there is no code-level guard against a human typing the wrong `--repo`) |
| `repohardenguard` is treated as already covering this repo when its checklist is absent | the `sources:` freshness fact + Task 4 naming the checklist gap explicitly, so a reader does not assume coverage that is not there |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; this brief documents and proposes
closing a live merge-to-protected-branch gap). Reviewer records verdict + date in the stream
README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between an App-held credential and an unreviewed merge to
   `main` today? (Every App's own restraint — `deskflip`'s convention, not a permission the
   platform withholds.) Is that acceptable alone? (No — this brief's own claim; that is why
   it proposes `human-approved` as a second, server-side layer rather than describing the
   restraint as sufficient.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed?
   (Row 3, once the follow-on implements the workflow: a colluding pair of App tokens that
   bypasses every desk-side convention still cannot clear the ruleset's required check on a
   fixture repo. Today, before that follow-on lands, the honest answer is NO row proves
   this yet — which is exactly the gap this brief states rather than papers over.)
3. Is the characterisation of items 2-5 as "already closed" correct, or does one of them hide
   the same convention-only gap item 1 has? (This is the judgment the gate exists for — a
   model does not self-certify that a permission scope genuinely has no bypass path GitHub's
   or GitLab's documentation omits.)
