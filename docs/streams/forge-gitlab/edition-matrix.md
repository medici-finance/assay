# forge-gitlab — GitLab edition / tier matrix

**Question this answers:** do we need a licensed GitLab, or can the Assay fleet run on
Community Edition? What genuinely needs Premium or Ultimate, and what can be done on CE?

**Answer, ruled 2026-08-30:** CE. Community Edition is conforming for the core lane (01-05,
07, 08) with the two disclosed degradations recorded under "Residual gaps" below; Premium is
the hardening that makes both server-enforced, and Ultimate is refinement (brief 06). Premium
and Ultimate features are opt-in, never prerequisites for the core lane. The binding wording
is spec.md section 1; this file is its evidence.

**Method.** Every tier in this file was read off the GitLab documentation page cited in its
row — each page carries a `Tier:` badge and an `Offering:` line near the top, and some
sections carry their own badge that overrides the page header. Nothing here is stated from
memory. Where a page could not be read, the row says `could-not-check` and names the URL, per
the three-state rule: an instrument that did not look has not cleared anything.

Read date for every citation below: **2026-08-30**.

## What "CE" means here

| Term | Meaning | Citation |
|---|---|---|
| Community Edition (CE) | the MIT-licensed build; the Free feature set | https://docs.gitlab.com/development/ee_features/ |
| Enterprise Edition (EE), unlicensed | same feature set as CE — "GitLab Enterprise Edition works like GitLab Community Edition when no license is active"; "When you install a new GitLab instance without a license, only Free features are enabled" | https://docs.gitlab.com/development/ee_features/ , https://docs.gitlab.com/administration/license/ |
| Free / Premium / Ultimate | the tier badges the docs use; a `Tier: Free, Premium, Ultimate` badge means the feature is in CE | (per-row pages below) |

So "runs on CE" and "runs on an unlicensed EE binary" are the same claim, and both are read
off the `Tier: Free, ...` badge. That equivalence is why every row below cites the tier badge
rather than the edition.

## A. Forge interface operations

One row per method of `deskkit.Forge` (the frozen 14, plus the typed ops brief 08 adds).
Every one of them is Free-tier.

| # | Operation (`Forge` method) | GitLab feature | Minimum tier | Docs citation (tier badge read there) | CE fallback | Owning brief |
|---|---|---|---|---|---|---|
| 1 | `CreateDraftChange` — open MR as draft | Merge requests API; `Draft:` title prefix | Free | https://docs.gitlab.com/api/merge_requests/ (`Tier: Free, Premium, Ultimate`); https://docs.gitlab.com/user/project/merge_requests/drafts/ (`Tier: Free, Premium, Ultimate`) | none needed | 02 |
| 2 | `MarkReadyForReview` — draft to ready flip | edit MR title, removing the `Draft:` prefix | Free | https://docs.gitlab.com/user/project/merge_requests/drafts/ | none needed | 02 |
| 3 | `GetPullRequest` — read change | `GET /projects/:id/merge_requests/:iid` | Free | https://docs.gitlab.com/api/merge_requests/ | none needed | 02 |
| 4 | `ListChangedFiles` — read change files (risk gate) | MR diffs endpoint | Free | https://docs.gitlab.com/api/merge_requests/ | none needed | 02 |
| 5 | `PostComment` — notes | Notes API (MR + issue notes) | Free | https://docs.gitlab.com/api/notes/ (`Tier: Free, Premium, Ultimate`) | none needed | 02 |
| 6 | `PostReview` (APPROVE) — record a verdict | MR approve endpoint | Free | https://docs.gitlab.com/user/project/merge_requests/approvals/ — "GitLab Free allows all users with at least the Developer role to approve merge requests. These approvals are optional and don't prevent merging without approval."; https://docs.gitlab.com/api/merge_request_approvals/ (page header `Tier: Free, Premium, Ultimate`; the *rules* sections carry `Tier: Premium, Ultimate`) | the approve call itself works; what CE lacks is making it *required* — see row B3 | 02 |
| 7 | `ReviewsAtHead` — read verdicts pinned to head | approvals + notes | Free to read; the head pin leans on "Remove all approvals when commits are added to the source branch" | https://docs.gitlab.com/api/merge_request_approvals/ ; reset-on-push: https://docs.gitlab.com/user/project/merge_requests/approvals/settings/ (`Tier: Premium, Ultimate`) | tool-side pin: the desk already writes the head SHA into the verdict note body, so at-head reading does not depend on the server resetting approvals. On CE, treat the approval flag as unpinned and the note's SHA as the pin | 02 |
| 8 | `ChecksAtHead` — read checks at head | pipelines at SHA + commit statuses | Free | https://docs.gitlab.com/api/pipelines/ (`Tier: Free, Premium, Ultimate`); https://docs.gitlab.com/api/commits/ (`Tier: Free, Premium, Ultimate`, no separate badge on set/list commit status) | none needed | 02 |
| 9 | `IssueReactions` — award-emoji admission gate | emoji reactions (award emoji) | Free | https://docs.gitlab.com/user/emoji_reactions/ (`Tier: Free, Premium, Ultimate`). The API page https://docs.gitlab.com/api/award_emoji/ is `could-not-check` — three fetches returned a 302 to an auth host, so its badge was not read; the user-facing page above is the tier source | none needed | 02 |
| 10 | `RepoVisibility` — visibility gate | `GET /projects/:id` `.visibility` | Free | https://docs.gitlab.com/api/projects/ (`Tier: Free, Premium, Ultimate`) | none needed | 02 |
| 11 | `GetIssue` / `FileIssue` / `CloseIssue` | Issues API | Free | https://docs.gitlab.com/api/issues/ (`Tier: Free, Premium, Ultimate`) | none needed | 02 |
| 12 | `PushTransportHint` | `oauth2:<PAT>` over HTTPS | Free | https://docs.gitlab.com/api/personal_access_tokens/ (`Tier: Free, Premium, Ultimate`) | none needed | 02, 03 |
| 13 | `DeleteRef` (brief 08's typed op replacing a CLI passthrough) | Branches API | Free | https://docs.gitlab.com/api/branches/ (`Tier: Free, Premium, Ultimate`) | none needed | 08 |
| 14 | reviewer assignment | MR reviewers | Free | https://docs.gitlab.com/user/project/merge_requests/reviews/ (`Tier: Free, Premium, Ultimate`; the "find reviewers who fulfill approval rules" and "prevent merge when you request changes" sections carry `Tier: Premium, Ultimate`) | assign reviewers normally; the two Premium sub-features are conveniences the fleet does not consume | 02 |

## B. Merge gates and branch protection — the security-parity profile

This is where the tier question actually lives. These rows are not operations the tools
perform; they are the *guarantees* spec section 3 promises.

| # | Control | GitLab feature | Minimum tier | Docs citation | CE fallback | Owning brief |
|---|---|---|---|---|---|---|
| B1 | protected `main` — nobody force-pushes, direct push closed | protected branches, role-level | Free | https://docs.gitlab.com/user/project/repository/branches/protected/ (`Tier: Free, Premium, Ultimate`) | none needed | 04 |
| B2 | **single board-writer identity** (the GitHub ruleset-bypass analog: exactly ONE identity may push to `main`) | protected branch `allowed_to_push` naming a user or group | **Premium** | https://docs.gitlab.com/api/protected_branches/ — "`user_id`, `group_id`, and `access_level` are Premium and Ultimate only"; https://docs.gitlab.com/user/project/repository/branches/protected/ — "In GitLab Premium and Ultimate, you can also add groups or individual users to **Allowed to merge** and **Allowed to push and merge**." | set `Allowed to push and merge` = **No one** and route *every* write, board regeneration included, through an MR. The allowlist becomes the project's Maintainer membership rather than a per-identity branch rule: role granularity, not identity granularity. **Disclosed degradation** — see "Residual gaps" | 04, hardened in 06 |
| B3 | **required approvals before merge** (a verdict must exist) | MR approval rules / `approvals_before_merge` | **Premium** | https://docs.gitlab.com/user/project/merge_requests/approvals/rules/ (`Tier: Premium, Ultimate`); https://docs.gitlab.com/api/projects/ — `approvals_before_merge` is annotated "Premium and Ultimate only" | merge is human-only on both forges anyway (`Allowed to merge` = humans). The desk supplies the gate it already supplies on GitHub: no ready-flip without an at-head reviewer verdict. Server-side enforcement drops to human discipline plus tool refusal | 04, hardened in 06 |
| B4 | **no self-approval** (author and committers cannot approve) | approval settings: prevent approval by author / by committers | **Premium** | https://docs.gitlab.com/user/project/merge_requests/approvals/settings/ (`Tier: Premium, Ultimate`) | the fleet's attribution separation already makes this structural — a worker service account holds no reviewer credential, so it cannot mint a reviewer approval. The desk additionally refuses author-authored verdicts. **Disclosed degradation**: on CE a *human* with both roles is not stopped by the server | 04, hardened in 06 |
| B5 | pipelines must succeed before merge | project setting `only_allow_merge_if_pipeline_succeeds` | Free | https://docs.gitlab.com/api/projects/ — "Whether merges are allowed only if the pipeline succeeds.", carrying no tier annotation on a `Tier: Free, Premium, Ultimate` page | none needed | 04 |
| B6 | all threads resolved before merge | project setting `only_allow_merge_if_all_discussions_are_resolved` | Free | https://docs.gitlab.com/api/projects/ — "Whether merges are allowed only if all discussions are resolved.", no tier annotation | none needed | 04 |
| B7 | auto-merge / merge when pipeline succeeds | auto-merge | Free | https://docs.gitlab.com/user/project/merge_requests/auto_merge/ (`Tier: Free, Premium, Ultimate`) | none needed | 04 |
| B8 | external status checks as the verdict-lane surface | external status checks | **Ultimate** | https://docs.gitlab.com/user/project/merge_requests/status_checks/ (`Tier: Ultimate`) | verdicts land as notes plus a commit status (row A8's surface, Free). The fleet already reads verdicts from notes on GitHub | 06 |
| B9 | reviewer identity that can approve but cannot push | custom roles / member roles | **Ultimate** | https://docs.gitlab.com/user/custom_roles/ (`Tier: Ultimate`) | reviewer service account at Developer, with `main` protected so it cannot push there. Push to *feature* branches remains possible — **disclosed degradation** vs a GitHub App's per-resource permissions | 06 |
| B10 | merge trains | merge trains | Premium | https://docs.gitlab.com/ci/pipelines/merge_trains/ (`Tier: Premium, Ultimate`) | not consumed by the profile — listed so the matrix is exhaustive, not because anything needs it | none |
| B11 | CODEOWNERS-based required review | Code Owners | Premium | https://docs.gitlab.com/user/project/codeowners/ (`Tier: Premium, Ultimate`) | not consumed by the profile; the desk routes reviewers itself | none |
| B12 | protected tags (release integrity) | protected tags, role-level | Free (role-level); Premium to name users or groups | https://docs.gitlab.com/user/project/protected_tags/ (`Tier: Free, Premium, Ultimate`; "In GitLab Premium and Ultimate, you can also add groups or individual users to **Allowed to create**.") | role-level protection plus the `.assay-versions` sha256 pin discipline, which is the control spec section 3 actually leans on | 04 |

## C. Identity, custody, CI isolation, provisioning

| # | Control | GitLab feature | Minimum tier | Docs citation | CE fallback | Owning brief |
|---|---|---|---|---|---|---|
| C1 | **role identities** (one seatless bot per desk role) | service accounts | Free, from GitLab 18.11 | https://docs.gitlab.com/user/profile/service_accounts/ — `Tier: Free, Premium, Ultimate`; version history: "Service accounts on Free tier: Introduced in GitLab 18.10 with a feature flag named `service_accounts_available_on_free_or_unlicensed`. Disabled by default." / "Generally available in GitLab 18.11. Feature flag removed." Also "They do not use a seat." | below 18.11: ordinary bot users, which on self-managed CE cost nothing. Note for brief 04: on GitLab Self-Managed "only administrators can create either type of service account" by default, so provisioning needs an admin credential, not a group-owner PAT | 04 |
| C2 | rotate-on-mint custody | `POST /personal_access_tokens/self/rotate` | Free | https://docs.gitlab.com/api/personal_access_tokens/ (`Tier: Free, Premium, Ultimate`; the rotation section carries no separate badge) | none needed — the single-valid-credential property is CE-native | 03 |
| C3 | **expiry backstop** (an idle fleet leaves no live credential) | instance/group maximum access-token lifetime policy | **Ultimate** | https://docs.gitlab.com/administration/settings/account_and_limit_settings/ — page `Tier: Free, Premium, Ultimate`, but the access-token-lifetime section carries `Tier: Ultimate` | `desktoken` supplies the backstop itself: rotation takes `expires_at` — "If the token requires an expiration date, defaults to 1 week" — so the rotated token expires in a week whether or not a policy exists. Tool-enforced instead of instance-enforced, and the default is already the 7 days spec section 5 recommends | 03 |
| C4 | audit trail on rotation / use | audit events | Free for sign-in events only; **Premium** for group and project audit events | https://docs.gitlab.com/user/compliance/audit_events/ — page `Tier: Free, Premium, Ultimate`; "Successful sign-in events are the only audit events available at all tiers."; group audit events and project audit events each `Tier: Premium, Ultimate` | none server-side. Sign-in events remain, and the desk's own advisory/claim records are the local trail. **Disclosed degradation**; opt-in refinement | 06 |
| C5 | secret push protection | push rules secret checks / Secret Push Protection | **Premium** (push rules) / **Ultimate** (secret push protection) | https://docs.gitlab.com/user/project/repository/push_rules/ (`Tier: Premium, Ultimate`); https://docs.gitlab.com/user/application_security/secret_detection/secret_push_protection/ (`Tier: Ultimate`) | the house leak sweep runs in CI regardless — spec section 3 already names CI as the layer that does not depend on tier | 06 |
| C6 | **CI isolation** — CI definition outside the writable repo | custom CI/CD configuration file path, including a file in a different project | Free | https://docs.gitlab.com/ci/pipelines/settings/ (`Tier: Free, Premium, Ultimate`; the "Specify a custom CI/CD configuration file" section carries no separate badge and documents referencing a file in a different project) | none needed. This is the row where the profile is *stronger* than the GitHub controls it must match, and it is stronger for free | 04 |
| C7 | enforced injected pipeline | pipeline execution policy | **Ultimate** | https://docs.gitlab.com/user/application_security/policies/pipeline_execution_policies/ (`Tier: Ultimate`) | the locked ci-config project (C6) already keeps the CI definition out of bot-writable space; the policy is belt-and-braces | 06 |
| C8 | group and project provisioning | Groups API, Projects API, Members API | Free | https://docs.gitlab.com/api/groups/ , https://docs.gitlab.com/api/projects/ , https://docs.gitlab.com/user/project/members/ (all `Tier: Free, Premium, Ultimate`) | none needed | 04 |
| C9 | webhooks / events | webhooks | Free | https://docs.gitlab.com/user/project/integrations/webhooks/ (`Tier: Free, Premium, Ultimate`) | none needed | 04 |

## could-not-check

| Page | What was wanted | What happened |
|---|---|---|
| https://docs.gitlab.com/api/award_emoji/ | the API page's own tier badge for row A9 | three fetches returned `302` to an auth host and the page body was never read. Row A9's tier comes from https://docs.gitlab.com/user/emoji_reactions/ instead |
| https://docs.gitlab.com/user/project/merge_requests/checks/ | a single page enumerating merge checks with per-check badges | two fetches returned the same `302`. Rows B5 and B6 take their tier from the Projects API attribute descriptions instead, which is a stronger citation for the fleet's purposes since the provisioning script sets those attributes through that API |

Neither gap changes a verdict: both rows are sourced from a page that WAS read, and both
resolve to Free.

## Verdict

**The core lane runs on Community Edition.** Every operation the `Forge` interface performs
maps to a Free-tier GitLab feature: merge requests and the `Draft:` prefix, notes, award
emoji, the approve endpoint, pipeline and commit-status reads, issues, branches, project and
group provisioning, webhooks, and `POST /personal_access_tokens/self/rotate` all carry
`Tier: Free, Premium, Ultimate` on the pages cited above. Service accounts joined them in
GitLab 18.11, so even the identity model is CE-clean on a current instance, with ordinary bot
users as the fallback below that version. Briefs 01, 02, 03, 07 and 08 need no licence at all,
and 04 and 05 need one only for the hardening they choose to configure. What needs a paid tier
is not an operation but a *guarantee*, and there are exactly three drivers. **First,
identity-granular protected branches** — `allowed_to_push` and `allowed_to_merge` accept a
`user_id` or `group_id` only on Premium — which is the direct analog of the GitHub ruleset
bypass naming exactly one board-writer App; on CE the branch rule is role-granular, so
"exactly one identity may push to `main`" degrades to "the Maintainer set is the allowlist".
**Second, merge-request approval rules** — required approvals and prevent-approval-by-author
or by-committer are both Premium — which is the server-side half of no-self-approval and of
verdict-before-merge; on CE approvals are advisory by design and the gate becomes human-only
merge plus the desk's own refusal to flip a change ready without an at-head verdict. **Third,
the Ultimate set** — external status checks, custom roles, the instance token-lifetime policy,
pipeline execution policy, and secret push protection — which is already brief 06's scope, and
whose one genuinely load-bearing member, the token-lifetime backstop, `desktoken` supplies
itself by sending `expires_at` on every rotation. So: build the tooling, provision, and pilot
on CE; treat Premium as the hardening that converts two disclosed degradations into
server-enforced controls, and Ultimate as refinement — never as prerequisites for the core
lane.

## Residual gaps — the two disclosed degradations

Two CE fallbacks are honest degradations, not equivalences. They are the two the ruling below
names, and spec.md section 1 carries them as the profile's disclosed degradations:

1. **B2 (single board-writer).** On CE the set of identities that may write `main` is a role
   membership, not a one-name allowlist. The mitigation (all writes via MR, `Allowed to push`
   = No one) is arguably a *different* strong control rather than a weaker one, but it is not
   the same control.
2. **B3 + B4 (enforced approval rules).** Required approvals are Premium (B3), and so is
   prevent-approval-by-author or by-committer (B4), so on CE approvals are advisory: the
   server neither requires a verdict before merge nor stops a human who holds both roles. The
   fleet's own attribution separation covers the bot case structurally; the rest is
   humans-only merge, the desk's refusal to flip ready without an at-head verdict, and
   discipline.

**Ruled 2026-08-30 — CE is conforming for the core lane** (briefs 01–05, 07, 08) with exactly
these two degradations disclosed; Premium is the hardening that makes both server-enforced,
and Ultimate is refinement (brief 06). The ruling and the wording that binds live in
[spec.md](spec.md) section 1, with section 3 scoping the carve-out to these two rows and no
others; this file is the evidence behind that ruling, not a second statement of it. Brief 04's
Verify row 3 was re-baselined onto the amended sentence in the same change.
