# forge-gitlab — live pilot report

**Brief:** forge-gitlab/05 · **Run date:** 2026-09-02 (all timestamps UTC, from the API
responses quoted below) · **Substrate:** gitlab.com SaaS, GitLab `19.4.0-pre`, **Free tier**.

**What this file is.** The conformance record spec.md §7 asks for: every claim below cites an
endpoint, an id, a SHA or an HTTP status read off the live deployment on the run date. Nothing
here is stated from the runbook, from the edition matrix, or from memory. Where the instrument
did not look, or could not, the row says `COULD-NOT-CHECK` — never `PASS`.

**Naming.** The pilot group and project are named by their numeric ids throughout, and the
service accounts by role plus numeric user id. Ids are the citation Verify row 2 dereferences
(`projects/:id/merge_requests/:iid/approvals`), and they carry no private name into a public
repo.

## 0. The substrate

| Thing | Live read | Value |
|---|---|---|
| Group | `GET /groups/9619193` | id `9619193`, `visibility: private`, `require_two_factor_authentication: false`, `max_personal_access_token_lifetime: null`, `service_access_tokens_expiration_enforced: true` |
| Tracking project | `GET /projects/86032201` | id `86032201`, `visibility: private`, `default_branch: main`, `merge_method: merge`, `ci_config_path: ""` |
| Human owner | `GET /projects/86032201/members/all` | one user at `access_level: 50` (Owner) |
| Service accounts | `GET /projects/86032201/members/all` | 7 bots: reviewer `41987965`, worker `41987966`, verifier `41987969`, desk `41987971`, board-writer `41987978` at `access_level: 30` (Developer); issue-loop `41987973`, intake-loop `41987976` at `access_level: 20` (Reporter). Every one returns `"bot": true` on `GET /user` |
| Protected `main` | `GET /projects/86032201/protected_branches` | rule id `312839985`: `push_access_levels: [{access_level: 0, "No one", user_id: null, group_id: null}]`, `merge_access_levels: [{access_level: 30, "Developers + Maintainers"}]`, `allow_force_push: false`, `unprotect_access_levels: [{access_level: 40}]` |

Provisioning (task 1) was run by the desk before this session; its two defects are filed as
`#346` and are not re-litigated here. Two consequences of that filing are load-bearing for
the walk below and are recorded, not fixed: the protect step's failure path left `main` to be
re-protected **by hand** with `merge_access_level=30` rather than the script's intended `40`,
and the script exits before its `only_allow_merge_if_pipeline_succeeds` step, so that setting
was never applied.

## 1. The round trip — phase A (tracking-root seed)

Every write below is a role service account's; the group-owner credential was used for
**read-only** settings walks and for nothing else.

| Step | Actor (id) | Mechanism | Artifact | Timestamp (UTC) |
|---|---|---|---|---|
| A1 · clone | desk `41987971` | `git clone` over HTTPS, `oauth2:<PAT>` via a credential helper reading the 0600 token file | base commit `40dc329` on `main` | 2026-09-02 |
| A2 · scaffold | desk `41987971` | `statusgen init` (binary `v0.23.0`), example stream replaced by a `pilot` stream carrying one `brief-v1`, `gate: model` brief whose single Verify row is `test -f docs/streams/pilot/hello.txt` | `.assay-versions` pinned to `v0.23.0` with the three per-platform sha256 digests read from that release's `checksums.txt` | 2026-09-02 |
| A3 · lint + board | desk `41987971` | `statusgen --root <root> --lint` → `LINT: PASS`, exit `0`; `statusgen --root <root>` → wrote the board | STATUS.md generated | 2026-09-02 |
| A4 · commit | desk `41987971` | `git -c user.name=… -c user.email=…` (inline identity, never `git config`) | commit `52879f61050c433276836afe61a066e1ce0e507f` | 2026-09-02 |
| A5 · push | desk `41987971` | push to `seed/assay-tracking-root`; **not** to `main`, which is push=No one | branch created | 2026-09-02 |
| A6 · open MR | desk `41987971` | `POST /projects/86032201/merge_requests` → `HTTP 201` | MR `!1`, id `527281070`, head `52879f61050c433276836afe61a066e1ce0e507f` | 2026-09-02T20:28:21.621Z |
| A7 · **B4 probe** | desk `41987971` | `POST …/merge_requests/1/approve` **as the MR's own author** → `HTTP 201`, `approved_by: [41987971]` | see §3, row "No self-approval" | 2026-09-02T20:28:44.201Z |
| A8 · withdraw probe | desk `41987971` | `POST …/merge_requests/1/unapprove` → `HTTP 201` | probe approval removed | 2026-09-02 |
| A9 · verdict note | reviewer `41987965` | `POST …/merge_requests/1/notes` → `HTTP 201`; body carries `Verdict: APPROVE` and pins the head SHA in text (the CE at-head pin, edition matrix row A7) | note id `3777865539` | 2026-09-02T20:29:03.378Z |
| A10 · approve | reviewer `41987965` | `POST …/merge_requests/1/approve` → `HTTP 201` | `approved_by: [{username: <reviewer>, approved_at: 2026-09-02T20:29:03.870Z}]`, `approved: true` | 2026-09-02T20:29:03.870Z |
| A11 · **human merge** | — | **not performed by this session.** `main` merge is the human's act | pending | — |

Author `41987971` ≠ approver `41987965` at MR `!1` — the dereference Verify row 2 asks for,
readable at `GET /projects/86032201/merge_requests/1/approvals`.

### Phase B — the brief round trip

Phase B (worker `Draft:` MR for `pilot/01`, reviewer approval, human merge, verifier Evidence
commit, board regeneration by the board-writer identity) **had not started when this report was
written**: it is gated on the human merging MR `!1`, which is step A11. This report is
therefore published at the end of phase A. Rows in §3 that phase B would have exercised
(nothing — phase B exercises the same controls a second time, on a second MR) are unaffected;
Verify rows 2 and 3 are already satisfied by phase A and by §4 respectively, and **Verify row 4
is `COULD-NOT-CHECK` until phase B lands**, recorded as such in §4 rather than assumed.

One parity deviation is already visible from phase B's design and is recorded here rather than
discovered later: **on GitHub the verifier lands its Evidence row straight to `main`** (the sole
carve-out from the branch-and-PR rule). On this GitLab project `main` is push = No one for every
identity — `GET /projects/86032201/repository/branches/main` returns `can_push: false` read on
each of the six credentials in turn, the group owner's included — so the verifier's Evidence row
must itself travel as a merge request and wait on a human merge. That is a **structural
difference in the verify lane, not a weakening**: it removes the one identity that could write
`main` unreviewed. It does mean the verify desk's single-actor Evidence landing has no GitLab
equivalent and the loop gains a human hop.

## 2. What the tooling could not do

Every write in §1 was a hand-built `curl` call against REST v4 or a raw `git` push. The desk
verbs the GitHub lane uses — `deskpr`, `deskpost`, `deskflip`, `deskfile`, `deskevidence` — have
no GitLab backend, so none of them was reachable for any step. The only fleet tool that ran
against GitLab was `desktoken --forge gitlab`, which worked (§4). This is expected at this point
in the stream — the `Forge` seam is forge-gitlab/02's scope and its GitHub half is still
`gh`-shelled (`#274`) — but it is what a pilot is for, so it is recorded as measurement
rather than assumed from the plan.

`statusgen` ran cleanly against the GitLab tracking root and emitted a correct board, with two
GitHub-shaped degradations named in §5.

## 3. Security parity walk — spec §3, per control, against live settings

Verdicts: **PASS** · **FAILED-AT-TIER** (with the tier that remediates it) · **COULD-NOT-CHECK**.
A `FAILED-AT-TIER` whose remediation tier is `free` is a provisioning gap, not a tier ceiling,
and is called out as such in the row.

| # | GitHub control (spec §3) | GitLab mechanism | Live read — endpoint and the fields that decide it | Verdict |
|---|---|---|---|---|
| 1 | Per-resource App permissions | role + token-scope narrowing + protected branches; Ultimate custom roles | `GET /groups/9619193/member_roles` → `HTTP 403` (Ultimate). `GET /personal_access_tokens/self` on each of the 7 role PATs → scopes are `["api"]` (desk, reviewer, issue-loop, intake-loop) or `["api","write_repository"]` (worker, verifier, board-writer). `api` is a whole-account scope: it carries every project the account can reach, at that account's role, with no per-resource narrowing — the desk PAT pushed a branch (step A5) holding `["api"]` alone | **FAILED-AT-TIER** (Ultimate for custom roles; and even there the *token* scope stays coarse — see Deviation D-5) |
| 2 | Ruleset bypass = a single board-writer | protected-branch `allowed_to_push` naming exactly the board-writer | `GET /projects/86032201/protected_branches` → `push_access_levels[0] = {access_level: 0 "No one", user_id: null, group_id: null}`. Identity-granular push is unexpressible: `#346` records the `allowed_to_*` POST returning `HTTP 400` on this tier | **FAILED-AT-TIER** (Premium) — the first disclosed degradation, spec §1(1). CE posture is in force: push = No one, every write via MR |
| 3 | No self-approval | approval settings: prevent-author + prevent-committers | `GET /projects/86032201/approvals` → `merge_requests_author_approval: false`, `merge_requests_disable_committers_approval: false`. **Live probe (step A7):** the MR's own author called `POST /projects/86032201/merge_requests/1/approve` and got `HTTP 201` with `approved_by: [<the author>]` — the setting is stored and **not enforced**. Same response: `merge_request_approvers_available: false` | **FAILED-AT-TIER** (Premium) — the second disclosed degradation, spec §1(2), now proved live rather than read off a docs badge |
| 4 | Required CI checks before merge | "pipelines must succeed" + required approvals; Ultimate external status checks | `GET /projects/86032201` → `only_allow_merge_if_pipeline_succeeds: false` and `only_allow_merge_if_all_discussions_are_resolved: false`. `GET /projects/86032201/approvals` → `approvals_required: 0`; `GET …/approval_rules` → one `any_approver` rule with `approvals_required: 0`. `GET /projects/86032201/external_status_checks` → `HTTP 401` (Ultimate). No CI exists to gate on: `GET …/pipelines` → `[]`, `GET …/repository/files/.gitlab-ci.yml?ref=main` → `HTTP 404` | **FAILED-AT-TIER** — split verdict. The required-approvals half is Premium (§1(2)). The pipeline half is **Free and simply unset**: remediation tier `free`, a provisioning gap (`#346`'s early exit) |
| 5 | `workflows` permission guarding CI | locked ci-config project + external CI config path | `GET /projects/86032201` → `ci_config_path: ""`. `GET /groups/9619193/projects` → 3 projects, none of them a ci-config project. The provisioner names this a human-only remainder, so nothing was attempted | **COULD-NOT-CHECK** — the mechanism spec §3 calls *stronger* than GitHub's was not provisioned on this pilot, so the pilot neither confirms nor refutes it |
| 6 | Human-gated workflow promotion | human-merged MR into the ci-config project | no ci-config project exists (row 5) | **COULD-NOT-CHECK** |
| 7 | Short-lived minted tokens | rotate-on-mint + short expiry policy (§5) | Two consecutive `desktoken --forge gitlab worker` mints; the first token, captured before the second mint, then returns `HTTP 401 {"error":"invalid_token","error_description":"Token was revoked."}` on `GET /user`, while the current one returns `HTTP 200`. `GET /personal_access_tokens/self` → `expires_at: "2026-09-09"`, i.e. 7 days, `active: true`, `revoked: false`. Full record in §4 | **PASS** — and stronger than the GitHub control on the single-valid-credential property: at most one credential per role is ever live |
| 7b | Expiry backstop (§5's other half) | group/instance max-token-lifetime policy | `GET /groups/9619193` → `max_personal_access_token_lifetime: null` (the policy is Ultimate). The backstop is supplied tool-side instead: every rotated token above carries `expires_at` 7 days out, and the group *does* report `service_access_tokens_expiration_enforced: true` | **FAILED-AT-TIER** (Ultimate) for the server-side policy; the tool-side backstop is **PASS** and is what actually holds |
| 8 | Secret push protection | push rules secret checks (Premium) + Secret Detection (Ultimate); house leak sweep in CI regardless | `GET /projects/86032201/push_rule` → `HTTP 404` (Premium). `GET /projects/86032201/security_settings` → `HTTP 200`, `secret_push_protection_enabled: false`, `validity_checks_enabled: false`. The compensating layer spec §3 leans on — the leak sweep in CI — is also absent: no `.gitlab-ci.yml`, no pipelines (row 4) | **FAILED-AT-TIER** (Premium for push rules, Ultimate for secret push protection) **and the free-tier compensator is unbuilt** — remediation for the compensator is tier `free` |
| 9 | Immutable release integrity | protected tags + audit events + sha256 pins in `.assay-versions` | `GET /projects/86032201/protected_tags` → `[]` — no protected tag rule at all; role-level protected tags are **Free**, so this is unset, not unavailable. `GET /groups/9619193/audit_events` and `GET /projects/86032201/audit_events` → `HTTP 403` (Premium). The pin discipline **is** in place: `.assay-versions` on MR `!1` pins `statusgen v0.23.0` with three per-platform sha256 digests taken from that release's `checksums.txt`, no placeholder token left | **FAILED-AT-TIER** — split. Audit events: Premium. Protected tags: remediation tier `free`, a provisioning gap. The pin half, which spec §3 says the control actually leans on: **PASS** |
| 10 | Merge is always the human's | `Allowed to merge` = humans only | `GET /projects/86032201/protected_branches` → `merge_access_levels[0] = {access_level: 30, "Developers + Maintainers"}`. Read on each credential in turn, `GET /projects/86032201/merge_requests/1` returns `user.can_merge: true` for **all five Developer service accounts** as well as for the owner. Nothing server-side stops a bot merging MR `!1` into `main` | **FAILED-AT-TIER**, remediation tier **`free`** — this is a provisioning defect, not a tier ceiling. Role-level `merge_access_level=40` is Free and is what the provisioner script intends; the hand-repair recorded in `#346` applied `30`. **The most serious finding in this walk** |
| 11 | Direct push to `main` closed (matrix B1, the floor under rows 2 and 10) | protected branch, role-level | `GET /projects/86032201/repository/branches/main` read on the owner and on all five Developer bots → `protected: true`, `can_push: false`, `developers_can_push: false`, `allow_force_push: false` on every one | **PASS** |
| 12 | Reviewer identity that can approve but cannot push (matrix B9) | Ultimate custom roles | `GET /groups/9619193/member_roles` → `HTTP 403` (Ultimate). **Live probe:** the reviewer service account created a throwaway feature branch (`POST /projects/86032201/repository/branches` → `HTTP 201`) and deleted it (`DELETE …` → `HTTP 204`). A Developer-role reviewer can write feature branches | **FAILED-AT-TIER** (Ultimate) — the degradation the edition matrix predicts at B9, confirmed live |
| 13 | Role identities are seatless bots (matrix C1) | service accounts, Free from 18.11 | `GET /user` on each of the 7 role PATs → `"bot": true`, `"state": "active"`, distinct numeric ids, distinct `service_account_group_9619193_*@noreply.gitlab.com` commit emails | **PASS** |

### Overall verdict

**The pilot does not clear spec §3's governing requirement on this deployment, and the reason is
not only the tier.** Of the fourteen rows walked: three **PASS** outright (rotate-on-mint's
single-valid-credential property, direct-push closure on `main`, seatless role identities), two
are **COULD-NOT-CHECK** because the CI-isolation half of the profile was never provisioned on
this pilot, and nine are **FAILED-AT-TIER**.

Those nine split into two very different populations, and conflating them would be the easiest
way to read this report wrongly:

- **Most are genuine tier ceilings** and are exactly the ones the 2026-08-30 ruling anticipates:
  identity-granular protected branches (row 2, Premium), enforced approval rules (rows 3 and the
  approvals half of 4, Premium), audit events (row 9, Premium), push rules and secret push
  protection (row 8, Premium/Ultimate), custom roles (rows 1 and 12, Ultimate), and the
  server-side token-lifetime policy (row 7b, Ultimate). Rows 2 and 3 are the two **disclosed
  degradations** spec §1 names, and the CE posture they require is in force — push = No one,
  merge via MR, the reviewer's verdict pinned to a head SHA in the note body. They are recorded
  as failed-at-tier with the remediation named, which is what the brief asks for and is not a
  pilot failure. Row 3 is now stronger evidence than it was: it is no longer a docs badge but a
  live `HTTP 201` on an author approving its own merge request.
- **Four of the nine are free-tier controls that were simply never applied** — row 10 (`Allowed to merge` =
  Developers, so every bot can merge into `main`), the pipeline half of row 4, protected tags in
  row 9, and the missing leak-sweep CI in row 8. None of these needs a licence. Row 10 is the
  one that matters: it defeats the single control the whole CE posture rests on. The CE story is
  "identity-granular push is unavailable, so *every* write to `main` lands as a merge request a
  human merges." On this deployment a human does not have to be the merger — a worker service
  account holding an `api` PAT can merge its own approved MR. With that, the CE fallback for rows
  2, 3 and 4 collapses at once, because all three of them discharge onto human-merge-only.

So the honest verdict is: **the tooling lane is proved and the tier story holds; this
*deployment* is misconfigured in one specific, free-to-fix way that invalidates the CE posture
until it is corrected.** The correction is a single call setting `merge_access_level` to `40`
(Maintainers) on the `main` protection rule, after which the only Maintainer-or-above member is
the human owner. Rows 4, 8 and 9's free-tier halves should be applied in the same pass. Until
then, no downstream claim of GitLab security parity may cite this group.

Nothing in this walk contradicts the edition matrix. Every tier boundary it predicted was
observed at the predicted tier, by the predicted status code.

## 4. Brief Verify rows

| Row | Command | Result | Citation |
|---|---|---|---|
| 1 | `grep -c '^[\|]' docs/streams/forge-gitlab/pilot-report.md` ≥ 12 | **checked-clean** | `grep -c '^[\|]'` over this file returns `42`; the §3 walk table alone contributes 16 of them (14 control rows + header + separator) |
| 2 | `…/merge_requests/:iid/approvals` shows approval by the reviewer account, author ≠ approver | **checked-clean** | `GET /projects/86032201/merge_requests/1/approvals` → `approved: true`, `approved_by[0].user.id = 41987965` (reviewer), MR `author.id = 41987971` (desk). Reproducible against the live system from the ids in this row |
| 3 | mint twice, first token rejected `401` | **checked-clean** | `desktoken --forge gitlab worker` run twice, 2026-09-02 ~20:29Z (exit `0` both times; it prints the token *path*, never the value). The first token was captured to a 0600 scratch file from the token file between the mints, returned `HTTP 200` on `GET /user` before the second mint and `HTTP 401 {"error":"invalid_token","error_description":"Token was revoked. You have to re-authorize from the user."}` after it; the scratch file was deleted immediately after the check. The replacement token: `GET /personal_access_tokens/self` → id `27157268`, `expires_at "2026-09-09"`, `active: true` |
| 4 | `git log --format='%an' -1 -- STATUS.md` is the board-writer account | **could-not-check** | Board regeneration is phase B, which is gated on the human merge of MR `!1` (step A11). Not run, not inferred. At the time of writing STATUS.md's only commit is `52879f61050c433276836afe61a066e1ce0e507f`, authored by the **desk** account as part of the seed — so this row would read `checked-failed` if run now, and is honestly `could-not-check` for its actual subject, which is the post-round-trip board |

## 5. Deviations

**D-1 — the desk verbs have no GitLab backend, so every write in the round trip was hand-built.**
`deskpr`, `deskpost`, `deskflip`, `deskfile` and `deskevidence` speak GitHub only. Steps A6–A10
(open MR, approve, note) were `curl` against REST v4, and the commits were raw `git` with an
inline identity. `desktoken --forge gitlab` is the one fleet tool that already has a GitLab path,
and it worked. Expected at this point in the stream (the `Forge` seam is forge-gitlab/02's, and
its GitHub half is still `gh`-shelled per `#274`) — recorded because a pilot measures rather
than assumes. Filed as a comment on `#274`.

**D-2 — `statusgen init` scaffolds a GitHub-only CI half.** The scaffold writes
`.github/workflows/assay-statusgen.yml` and nothing else, so a GitLab adopter's tracking root
arrives with an inert single-writer board CI and no `.gitlab-ci.yml`. Separately,
`statusgen --lint` on a GitLab remote emits `NOTICE: dead-claim decay unavailable — gh pr list:
… none of the git remotes configured for this repository point to a known GitHub host`: the
claim-decay pass shells `gh`. Lint still exits `0`, so this is a silently degraded check, not a
refusal. Filed as `#349`.

**D-3 — the trust roster is GitHub-shaped, so the Evidence-actor defense is unavailable on
GitLab.** `roster.env` keys identities as `<login>:<numeric GitHub user id>` and
`<app-slug>:<GitHub App id>`. Pointed at a GitLab-only home, `statusgen --lint` reports
`could-not-check: Evidence-actor (desk-apps/07, F-verify-self-attest) did not run — the trust
roster … is absent or invalid, so no accepted verifier identity could be resolved`. The check
that stops an implementer self-attesting its own Evidence has no GitLab expression today. Filed
as a comment on `#349` (the session filing budget for this repo was spent on `#349` itself).

**D-4 — the provisioner never attempts two free-tier §3 controls.** `create-fleet-gitlab.sh`
sets neither protected tags (spec §3 "immutable release integrity"; role-level protected tags are
Free) nor `only_allow_merge_if_all_discussions_are_resolved` (matrix B6, Free). Live reads:
`GET /projects/86032201/protected_tags` → `[]`;
`GET /projects/86032201` → `only_allow_merge_if_all_discussions_are_resolved: false`. Filed as a
comment on `#346`, whose provisioning-script scope this shares.

**D-5 — GitLab PAT scope is coarser than a GitHub App installation token, and Ultimate does not
fix it.** Spec §3 row 1 offers custom roles as the Ultimate parity route, but custom roles narrow
the *role*, not the *token*: an `api`-scoped PAT still reaches every project its account can see,
at that account's role. The desk account pushed a branch (step A5) holding `["api"]` alone, with
no `write_repository`. A GitHub App installation token is scoped per repository and per
permission; nothing at any GitLab tier reproduces that shape for a PAT. Recorded here as a
qualification on row 1's remediation, and as a comment on `#274`.

**D-6 — post-repair merge access on `main` admits every bot.** Covered as §3 row 10 and in the
overall verdict; the provisioning history behind it belongs to `#346` and is filed there as
a comment rather than as a fresh issue.

**D-7 — token custody filenames.** The provisioner writes `<prefix>-<role>-bot.token` while
`desktoken --forge gitlab` reads `gitlab-<role>.token` from the credential search path. Already
documented on `#348`; on this pilot the gap was bridged with symlinks before the run, which
is why §4 row 3 could execute at all. Cited, not re-filed.

**D-8 — the verifier's Evidence lane gains a human hop on GitLab.** Recorded in §1 under
"Phase B": with `main` at push = No one for every identity, the GitHub carve-out that lets the
verifier commit an Evidence row straight to `main` has no GitLab equivalent. Structural, and on
balance a tightening rather than a weakening — but it changes the verify desk's loop shape and
is not a detail the profile had written down.

## 6. Token hygiene during this run

No token value was printed, passed in an argv, or written to any file in a repository. Per-role
`curl` config files carrying `header = "PRIVATE-TOKEN: …"` were built 0600 outside every
checkout by redirecting from the 0600 token file, and `git` reached GitLab through a credential
helper that reads that same file. The rotate-on-mint check in §4 row 3 required holding the
superseded token for one call; it was held in a 0600 scratch file and deleted in the same step
that read it. The group-owner credential was used for reads only.
