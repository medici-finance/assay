# Adopting Assay on GitLab — the GitLab-profile install runbook

This is the GitLab-side companion to [`docs/adopting-assay.md`](adopting-assay.md) (the
GitHub runbook). It does not repeat CORE Assay concepts (briefs, registers, lifecycle,
board, statusgen) — those are forge-agnostic and unchanged. It covers only what is
GitLab-shaped: the identity model, the provisioning script, the ci-config-project
runbook, and token custody. The accepted design this doc implements is
[`docs/streams/forge-gitlab/spec.md`](streams/forge-gitlab/spec.md) — read it first if
anything here seems to assert a control without justifying it; the spec carries the
per-control parity table this doc only points at.

**Windows + Cursor.** Native Windows still follows the Windows arm of
[`adopting-assay.md`](adopting-assay.md) (config home, PATH, channel D vs E). Cursor still
copies skills (no `/plugin`). This file does not make those into GitHub App installs.

## 0. Tier ladder — read this before provisioning anything

> The GitLab profile MUST be **at least as secure as the existing GitHub controls**, even
> where the mechanism is completely different. "Weaker but disclosed" is non-conforming.
> — spec.md, governing requirement

**Parity vs provisioning are different claims.** GitHub-equivalent *controls* still need
Premium/Ultimate for several rows (push allowlist, approval-prevention, custom roles). The
*core lane* (human-merged MRs, service-account role fleet, desk verbs) can run on Free /
Community Edition with the degradations in §0.1 **declared**. Do not refuse to provision
because the first sentence of an older draft said "Premium is the floor"; do not present a
Free/CE fleet as GitHub-equivalent either.

- **Premium** is the floor for **GitHub-equivalent** protected-branch push-access lists,
  project-level approval-prevention, and some policy backstops.
- **Ultimate is REQUIRED for public or risk-classed work** that needs per-resource permission
  parity with a GitHub App (custom roles; spec.md §3, row 1).
- **GitLab Free / Community Edition (CE) is non-conforming for GitHub-equivalent controls.**
  It **is** a documented core-lane target (ruling #219, §0.1). Do not run the write path on
  Free/CE and claim GitHub-equivalent guarantees; do run the lane there if the disclosed
  degradations are acceptable.

**Self-managed is not gitlab.com Free.** A live gitlab.com Free run (GitLab 19.4, 2026-09-02)
is the evidence behind the table below. A self-managed CE/EE instance can differ: approval
settings may **404** instead of `201` with a silent no-op; protected-branch read-back may
show Maintainer (`40`) push/merge rather than "push = No one." **Always print and keep the
script's read-back.** Script exit `1` with accounts created is a degraded provision, not a
rolled-back one. Do not treat a different HTTP status as the gitlab.com row.

This statement is carried from spec.md §1, with the **provisioning** claim measured rather
than assumed:

### 0.1 Free tier / CE — conforming for the core lane, with the degradations disclosed below (ruling #219; measured live 2026-09-02)

The edition ruling (#219, carried in `docs/streams/forge-gitlab/edition-matrix.md`) stands:
**Community Edition is conforming for the core lane** (Free on gitlab.com is the same
edition): the Maintainer role set is the allowlist for pushes to `main`,
approvals are advisory, and every write reaches `main` through a merge request a human
merges. What the pilot changed is the *size of the disclosure*, not
the stance: a live run of the §2 script against a fresh free-tier gitlab.com top-level group
(GitLab 19.4) provisioned the whole §1 identity model — seven service accounts (free and
seatless since GitLab 18.11, edition-matrix row C1), their group memberships and their
7-day PATs — and then drove one brief through desk → worker → reviewer → verifier →
board-writer with the human as the only merger. The round trip is the proof the core lane
runs on Free (the forge-gitlab/05 pilot report carries every read).

The disclosed degradations, each observed live and each a row the parity walk records as
*failed-at-tier* with its remediation — an adopter on Free runs with all of them and says so:

| Control | Free-tier behaviour observed | What stands in for it on Free | Remediation (tier) |
|---|---|---|---|
| single board-writer push allowlist on `main` (B2) | `allowed_to_*` arrays rejected (HTTP 400); only `push_access_level` / `merge_access_level` / `allow_force_push` apply | push = **No one**, merge = **Maintainers**; every write, the board regeneration included, travels as an MR the human merges | Premium: name the board-writer in `allowed_to_push` |
| required approvals (B3) and prevent-author/committer approval (B4) | gitlab.com Free: `POST /projects/:id/approvals` returns 201 and **silently keeps** `approvals_before_merge: 0`; some self-managed CE: **HTTP 404**. An MR's own author can `/approve` itself where the setting never applied | the reviewer service account holds the only reviewer credential; the desk refuses an author-authored verdict and never flips ready without an at-head verdict; the human merges | Premium |
| group token-expiry policy (§5 backstop) | not available | the 7-day PAT expiry set at mint is the only backstop; rotate-on-mint still invalidates the previous token live | Premium |
| audit events | not available | the MR history itself — every content commit a distinct role account, every merge the human | Premium |
| protected tags | none set by the provisioner | release tags are a human act on Free; treat any bot tag as unauthorised | Premium (tag allowlist) |
| pipeline execution gate / merge-time CI | no pipeline configured → GitLab offers "merge unverified changes" and lets the human proceed | the human declines that prompt until a `.gitlab-ci.yml` board-writer + lint half exists (a stated gap, not a silent pass) | Free once the CI half is scaffolded; Ultimate for an *enforced* external status check |
| reviewer that can approve but cannot push (B9) | Developer can push to feature branches | `main` is push = No one; feature-branch pushes remain possible | Ultimate (custom role) |

Read that table as the profile's own honesty rule applied to a tier: the core lane runs
and conforms on Free *with these degradations declared*; a deployment's conformance is a
separate question that only a live per-control walk answers, so walk it and file the rows.
Do not present a Free deployment as GitHub-equivalent on the rows above; do run the lane
there, and pay for the tier that closes a row only when that row's remediation is what you
need.

## 0.2 Group, not a personal namespace

`create-fleet-gitlab.sh` provisions **group-owned** service accounts. A project under a
personal namespace cannot host that fleet. Create (or use) a top-level **group**, put the
adopter project in it, then pass `--group` / `--project` as `group/project`. Moving a
personal project into a group is a human GitLab act, not something the script infers.

## 1. Identity model

GitLab has no App-manifest analog (no manifest flow, no per-resource permission matrix,
no JWT-minted installation tokens). The role fleet maps to **service accounts** — seatless
bot users owned by the top-level group, one per role, each with its own personal access
token (PAT):

| Assay role | GitLab identity | Access level | Token scopes | Mechanism |
|---|---|---|---|---|
| reviewer | service account | Developer (30) | `api` | MR notes + approvals; Ultimate: custom role without push |
| worker | service account | Developer (30) | `api`, `write_repository` | branches + `Draft:` MRs |
| verifier | service account | Developer (30) | `api`, `write_repository` | commits Evidence; excluded from approval eligibility by approval rules |
| desk | service account | Developer (30) | `api` | coordination via MRs |
| issue-loop | service account | Reporter (20) | `api` | files/triages issues |
| intake-loop | service account | Reporter (20) | `api` | files/triages issues |
| board-writer | service account | Developer (30) + allowed-to-push entry on protected `main` | `api`, `write_repository` | the ruleset-bypass analog |
| promote | usually **no identity at all** — see §3 | — | — | workflow promotion is a human-merged MR into the ci-config project, not a bot act |

Attribution separation holds exactly as on GitHub: notes/approvals/commits carry the
service-account identity, which the PR/MR author's own token cannot produce — the same
honest limit as the GitHub profile (separation of attribution, not proof of diligence).

**Three credential classes — do not collapse them.**

| Class | Who | Lives in config-home as | Used for |
|---|---|---|---|
| Bless / merge human | the Owner who may merge protected `main` | roster `ASSAY_BLESS_LOGIN` / `ASSAY_TRUSTED_LOGINS` (humans only) | human gates, merge, tags |
| Session actor | the account the agent/`glab` session authenticates as | e.g. `gitlab-second-human.token` | interactive API; **must not** merge protected `main` if it is only Developer |
| Role fleet | one SA per Assay role | `gitlab-<role>.token` (see §2 copies) | minted desk writes |

Never put a `*-bot` login in `ASSAY_TRUSTED_LOGINS` — the roster loader refuses that mix.
Role tokens are not the session git identity.

**Commit identity vs role identity.** `deskroster preflight --role worker` may require the
worktree `user.email` to be the **worker service-account noreply** form
(`service_account_group_<group-id>_<suffix>@noreply.<host>`). That can disagree with a
session-actor email the rest of the GitLab profile documents. Until the check matches the
two-identity model, either set the worktree email to the worker SA noreply **or** expect
`commit-identity=checked-failed` while using a session actor. `deskroster preflight` is now
forge-aware (#655): on a GitLab adopter `token-mint-cold` verifies the
`desktoken --forge gitlab <role>` custody path READ-ONLY and its remediation names that path,
not GitHub App PEMs. Any older PEM-oriented remediation you still see is GitHub-path leftover —
a GitLab deployment has no PEMs to install.

## 2. Provisioning script — `tools/create-fleet-gitlab.sh`

The script is idempotent bash + curl + jq, run by a human holding a **group-owner PAT**
(supplied only via the `GITLAB_TOKEN` environment variable — never a flag, never
committed, never stored by the script). It creates the seven service accounts above,
their group memberships, and their PATs; when `--project` is given, it also configures
that project's protected `main` branch and MR-approval settings. It never touches
Ultimate-only settings, the group token-expiry policy, or ci-config project creation —
those are the human-only remainder it prints at the end (§4, §5).

```
GITLAB_TOKEN=<group-owner PAT> tools/create-fleet-gitlab.sh \
  --group mygroup --prefix myorg --project mygroup/myproject
```

Always dry-run first — it makes zero network calls and enumerates every account, scope,
and setting the real run would touch:

```
tools/create-fleet-gitlab.sh --dry-run --group mygroup --prefix myorg
```

Flags: `--group` and `--prefix` are required; `--project` is optional (omitting it skips
the protected-branch/approval steps with a printed NOTICE, not silently); `--gitlab-url`
defaults to `https://gitlab.com` (point it at your self-managed instance otherwise);
`--pat-expiry-days` defaults to 7, the RECOMMENDED backstop from spec.md §5;
`--out-dir` controls where minted token files land (default: a fresh `mktemp -d`);
`--avatars-dir`, `--no-avatars` and `--avatars-only` control the avatar step below. Run
`tools/create-fleet-gitlab.sh --help` for the full reference.

**Idempotency.** A re-run against an already-provisioned group is a set of named no-ops,
never duplicates — the script checks for an existing service account by username before
creating one (GitLab's own username-uniqueness constraint is the backstop this leans on),
checks group membership before adding it, and — deliberately — mints a PAT **only** for
an account it just created. Re-running against an existing account prints a NOTICE
instead of silently minting a second live credential; get a fresh one via the group
service-accounts rotate endpoint (`api/v4/groups/:id/service_accounts/:user_id/personal_access_tokens/:token_id/rotate`),
which is what rotate-on-mint (§5) actually calls at operation time.

**Token custody.** Every minted token is written to a `0600` file under `--out-dir`; the
script prints the file's **path**, never the token value, on stdout or in argv. Move the
files out of the default `mktemp` location into your role-token store immediately — the
directory is not cleaned up for you, by design, so a run's tokens survive the script
exiting.

**Where the role-token store is, and what the files must be called.** The desk verbs
resolve credentials from the config-home (`$HOME/.config/assay/` on Unix;
`%USERPROFILE%\.config\assay` on Windows). A cell that spawns verbs with a stripped
environment (no `HOME` / `USERPROFILE`) will not see this store. Point `--out-dir`
straight at it. The script names each file `<prefix>-<role>-bot.token`
(the service account's username); `desktoken --forge gitlab <role>` — the rotate-on-mint
custody in §5 — looks for **`gitlab-<role>.token`**. Until the two agree, link **or copy**
them once after provisioning.

Unix:

```
cd "$HOME/.config/assay" && for r in reviewer worker verifier desk issue-loop intake-loop board-writer; do
  ln -s "<prefix>-$r-bot.token" "gitlab-$r.token"
done
```

Windows (no `ln -s` required): copy each `<prefix>-<role>-bot.token` to `gitlab-<role>.token`
in the same directory. Keep both files `0600`-equivalent (owner-only ACL).

The script itself is **bash + curl + jq**. On native Windows run it from Git-Bash or WSL,
not from PowerShell.

**`GITLAB_API_BASE` — required before the next boot, and it is not a `roster.env` key.**
Every GitLab-side token operation — the read-only custody check `deskboot` / `deskroster
preflight` runs at boot (`token-mint-cold`, GitLab arm), and the rotate-on-mint
`desktoken --forge gitlab <role>` it is proving the preconditions of — needs the deployment's
REST v4 base and refuses before any network contact without it. There is **no fallback**:
unlike GitHub's fixed `api.github.com`, GitLab is commonly self-hosted, so a default host
would risk sending a role's live PAT to a guessed target the first time this variable was
left unset. Set it to:

```
https://gitlab.example.com/api/v4   # self-hosted
https://gitlab.com/api/v4           # gitlab.com SaaS
```

It is a plain **environment variable**, read directly from the process environment at call
time (`os.Getenv("GITLAB_API_BASE")`) — never a `roster.env` key. `roster.env` only recognises
a fixed, namespaced allowlist of keys (`ASSAY_*`); an unrecognised key in that namespace fails
the whole shared file closed, and `GITLAB_API_BASE` does not follow that namespace, so writing
it into `roster.env` does nothing — the tools would not read it there. Export it in the
environment of whatever shell or session-bootstrap actually invokes the desk verbs (the same
place that would export e.g. `ASSAY_CONFIG_HOME`), before the next `deskboot` /
`deskroster preflight` / `desktoken --forge gitlab` call: the GitLab custody check reads the
variable in-process, with no child process and no scrubbed environment in between, so once it
is exported in that process's own environment there is nothing further to propagate. A value
set only in an interactive shell that later exits, or in a config file nothing sources into
that process's environment, does not count as set.

*Should this become a `roster.env` key instead of an environment variable?* Weighing it: the
value is deployment-wide (one REST base per GitLab instance, not per-role like the token
files), which is roster-shaped; but `gitlabAPIBase()` is deliberately read at call time rather
than cached, specifically so a value exported or changed after process start still takes
effect (`tools/desk/cmd/desktoken/gitlab.go:52-54`) — a property a `roster.env` key would lose,
since that file is parsed once through `rosterconfig.go`'s fixed `ASSAY_*` allowlist, and
adding a new key there means registering it in that allowlist plus threading a second read
path everywhere `GITLAB_API_BASE` is currently read. That is a real design question, not a
docs fix — it is not decided here, and this doc does not anticipate the outcome.

**The owner PAT.** Use a **legacy** personal access token with scope `api` (and only
`api`), issued by a group Owner, expiring in 30–90 days, stored `0600` in the same
config-home (for example `gitlab-owner.token`) and exported into `GITLAB_TOKEN` only for
the duration of the run. GitLab's fine-grained tokens are not yet proven against this
script — it needs owner-level calls on four surfaces (service accounts and their token
minting, group members, protected branches, approval settings) and a missing granular
permission fails mid-run with a half-provisioned fleet. Proving the script on a
fine-grained token is an open follow-up, not a supported path.

**Bot avatars.** Each service account sets its **own** avatar, at the moment its PAT is
minted — a group Owner cannot do it for them (`PUT /users/:id` is admin-only and answers
`403` on gitlab.com, and the service-account endpoints carry no avatar field), so
`PUT /user/avatar` under the account's own credential is the only path. By default the
script fetches the public role icons from `https://assay.guide/assets/app-icon-<role>.png`
— the same set the GitHub Apps wear — and uploads one per role; `--avatars-dir <dir>` uses
your own `<role>.png` files instead, and `--no-avatars` skips the step. An icon that is
missing or cannot be fetched is a NOTICE, never a failed run. Because a PAT is minted only
for an account the script just created, a fleet that already exists is re-skinned with
`--avatars-only --prefix <prefix> --out-dir <dir holding the role token files>`, which
uploads and mints nothing. Verify with `GET /groups/:id/members/all`: every
`<prefix>-<role>-bot` should carry an `avatar_url` under `/uploads/`, none under
`gravatar.com`.

**The protect step is tier-aware and never leaves `main` open.** It reads the group's
`plan`, sends the Premium `allowed_to_*` arrays only where they are available and the three
free-tier fields (`push_access_level=0`, `merge_access_level=40`, `allow_force_push=false`)
otherwise — printing the omitted push allowlist as `failed-at-tier, remediation: Premium` —
and it never removes the existing rule before the replacement is known to apply: an
already-correct rule is a no-op, a force-push-only difference is a `PATCH`, and where a
delete-and-recreate is unavoidable a refused re-create immediately re-applies the rule that
was read. It then reads all three fields back and prints them, so a wrong rule (a repair at
`merge_access_level=30` lets every Developer bot merge) is visible at provisioning time.
On some self-managed instances the intended "push = No one" fields do not stick and
read-back stays Maintainer (`40`); treat **that printed read-back** as the live control,
not the script's request body. Approval settings are read back the same way, because on
gitlab.com Free the write can return 201 and change nothing, and on some CE instances
`POST /projects/:id/approvals` returns **404**. A failed step no longer aborts the steps
after it: every step runs, the failures are listed under the HUMAN-ONLY REMAINDER, and the
script exits non-zero.

## 2a. Runners and job tags — the executor that will actually pick up the pipeline

`statusgen init --forge gitlab` scaffolds the CI *file* (`.gitlab-ci.yml`, the two-half
single-writer shape). It does **not**, and will not, give you a **runner** — the executor
that actually runs the jobs. Registering or reconfiguring a GitLab Runner is instance-admin
work, and it is an explicit non-goal: Assay never ships a `gitlab-runner register`, never
touches your fleet, and never guesses your instance's tag set. This section is the human
post-install step that closes that gap.

**Why this is not automatic — and why an untagged job can hang forever.** GitHub Actions has
a hosted `ubuntu-latest`; a self-hosted GitLab instance has no equivalent default. The
scaffolded jobs are **untagged** (they carry no `tags:`), so only a runner configured to take
untagged jobs will run them. GitLab matches a job to a runner by tags: a runner with
`run_untagged = false` (a common default on self-hosted fleets) will **never** take an
untagged job. When nothing eligible is online, the pipeline is still *created* and the YAML
still *parses* — the job simply sits in `pending`, and after the instance's stuck timeout the
default-branch job fails with `failure_reason = stuck_pending_no_matching_runners`. This is
not "CI disabled" and not a missing workflow file; pipelines fire, but nothing picks them up.

**What you need — an executor and one of two matching strategies:**

- A **Linux Docker** (or **Kubernetes**) executor online for the fleet project (or its group),
  AND
- **either** a runner with **`run_untagged = true`** — it will take the untagged scaffold jobs
  as-is —
- **or** the instance's required tag(s) put on the jobs: uncomment the `tags:` line under each
  job in `.gitlab-ci.yml` (the scaffold ships a commented `ADOPTER: runner` placeholder there)
  and list the tag(s) your executor advertises. Do **not** copy a tag from another instance or
  from this doc — the required tag set is local to your instance.

**Executor type matters.** The scaffold's install step downloads the pinned `statusgen`
release binary and verifies its sha256, then uses `git` to push the regenerated board — so
the executor must provide `curl`, `sha256sum`, `install`, and `git`. A Docker or Kubernetes
executor gives you a clean image with these present; a bare **shell** executor without a
container image will run only what happens to be installed on that host, and cannot pull an
image for you. Prefer a Docker/Kubernetes executor.

**Prove-the-install — a `pending` job is could-not-check, never "installed".** Do not report
CI as installed while a job is still `pending` / `stuck_pending_no_matching_runners`. The
install is proven only once a job has **left pending** — it reached `running`, or a **terminal
non-stuck** result. Note the ordering: an unset `STATUSGEN_PUSH_TOKEN` makes the regen job
fail, but that is a *later* red — the job ran, so the runner match is proven and only the push
credential is missing (see §2, token custody). A job that never leaves `pending` proves
nothing about either; it is a runner-match gap, and the fix is a runner, not a token.

This is the CI half of the Free-tier "pipeline execution gate" degradation in §0.1: that row
assumes a runner exists once the CI file is scaffolded. It does not exist until you provide
one here.

## 3. By-hand table — what the script does, if you'd rather read the REST calls

For the reviewer verifying this script, or an operator without shell access, the table
below mirrors exactly what `create-fleet-gitlab.sh` does — each row names the GitLab API
v4 endpoint used (paths dereferenced against docs.gitlab.com; see the script's own header
comment for the full endpoint list):

| Step | Endpoint | Notes |
|---|---|---|
| Resolve the group | `GET api/v4/groups/:id` | `:id` accepts a URL-encoded full path |
| Check for an existing service account | `GET api/v4/groups/:id/service_accounts` | filtered client-side by username |
| Create a service account | `POST api/v4/groups/:id/service_accounts` | one call per role not already present |
| Check group membership | `GET api/v4/groups/:id/members/:user_id` | 404 means "not a member yet" |
| Add group membership | `POST api/v4/groups/:id/members` | `access_level` per the §1 table |
| Mint a PAT | `POST api/v4/groups/:id/service_accounts/:user_id/personal_access_tokens` | only for a freshly created account; `expires_at` = today + `--pat-expiry-days` |
| Resolve the project | `GET api/v4/projects/:id` | requires `--project` |
| Read/clear existing branch protection | `GET`/`DELETE api/v4/projects/:id/protected_branches/:name` | recreate rather than PATCH, for portability across GitLab versions |
| Protect `main` | `POST api/v4/projects/:id/protected_branches` | `allowed_to_push=[{user_id: <board-writer>}]`, `allowed_to_merge=[{access_level: 40}]` (Maintainer role) |
| Set approval settings | `POST api/v4/projects/:id/approvals` | `merge_requests_author_approval: false`, `merge_requests_disable_committers_approval: true` — the prevent-author / prevent-committers pair |
| Require green pipelines before merge | `PUT api/v4/projects/:id` | `only_allow_merge_if_pipeline_succeeds: true` |
| Create the desk labels | `POST api/v4/projects/:id/labels` | one call per label, idempotent (a duplicate name answers 409, or 400 "already exists"); the queue-legibility pair `authorization-needed` / `approval-needed`, the `review-request` dispatch token, and one `raised-by:<role>` per filing role |

Every endpoint above is reachable at the **Premium** tier — nothing the script calls
requires Ultimate.

The label set is the GitLab twin of the GitHub **`create-labels`** primitive
(`docs/adopting-assay.md`) — the SAME names, colors and descriptions, so the two
adoption profiles are label-parity. It matters because a label that is absent when a
desk verb reaches for it degrades **silently**: `deskflip`'s `authorization-needed` →
`approval-needed` queue swap fails, and `deskfile --raised-by <role>` drops the
provenance stamp (assay#774). The script creates them under `--project`; colors are
sent with the leading `#` GitLab requires.

## 4. The ci-config-project runbook (human-only)

GitLab has no `workflows`-permission class the way GitHub Apps do — `.gitlab-ci.yml` is
an ordinary file — and does not need one: a project's CI/CD configuration can point at a
file in a **different project** (all tiers), and Ultimate groups can **enforce** the
injected pipeline by policy. The profile uses one locked ci-config project per group; this
step is human-only because it is a one-time, irreversible-shaped act (project creation +
membership) that the provisioning script deliberately does not attempt:

1. Create a new project in the top-level group — e.g. `mygroup/ci-config` — with
   **Maintainer-humans-only** membership. Do not add any of the §1 service accounts to
   it; that absence is the control (spec.md §4 — "Bot identities are simply never members
   of the ci-config project").
2. Protect its `main` branch (allowed-to-push = Maintainer humans only) and turn on the
   same approval rules as §2 (prevent-author, prevent-committers).
3. Commit the shared `.gitlab-ci.yml` there.
4. For each fleet project, set **Settings > CI/CD > General pipelines > CI/CD
   configuration file** to:

   ```
   <path>/.gitlab-ci.yml@mygroup/ci-config
   ```

5. Verify no bot service account can read or write the ci-config project: `GET
   api/v4/projects/:id/members/all` on the ci-config project should list only human
   Maintainer logins.
6. On Ultimate, additionally create a pipeline execution policy that pins the injected
   pipeline — this makes CI-definition tampering structurally unreachable rather than
   merely un-permissioned (spec.md §3, row 5: "**stronger** — CI definition lives outside
   the writable repo").

Workflow promotion — changing what CI runs — collapses to an ordinary human-merged MR
into the ci-config project. No bot identity is ever in a position to promote its own
workflow change; that is the whole control.

## 5. Token custody rules

Carried verbatim in spirit from spec.md §5 — this doc does not relax any of it:

- **Rotate-on-mint.** Every token mint (beyond the provisioning script's one-time initial
  mint) calls the group service-accounts rotate endpoint
  (`POST api/v4/groups/:id/service_accounts/:user_id/personal_access_tokens/:token_id/rotate`),
  which returns a fresh token and atomically invalidates the old one — at most ONE valid
  credential per role at any moment, and any captured token dies at the next mint. Roles
  are single-window by convention already; parallel actors get per-actor service
  accounts, never a shared token.
- **Expiry backstop.** Set the group (or instance) token-lifetime policy short — **7 days
  RECOMMENDED** — so an idle fleet leaves no live credential. This is a human-only setting
  (§4's checklist item 2); the script cannot set it via a group-scoped PAT.
- **File custody unchanged.** `0600` token files, path-only printing, never in an
  environment variable or a command's argv.
- **Audit events** (Premium+) should be reviewed periodically for rotation/use anomalies.

## 6. Parity statement

The per-control security-parity table — what GitHub control maps to what GitLab
mechanism, and the verdict for each — is maintained in one place, not duplicated here:
[`docs/streams/forge-gitlab/spec.md` §3](streams/forge-gitlab/spec.md#3-security-parity--the-per-control-table).
Read it before asserting GitLab parity to anyone; this doc's job is provisioning
mechanics, not the parity argument.

**No claim of GitLab support is valid before the live pilot.** Per spec.md §7, this
profile is not to be described as supported until one brief has round-tripped
todo→done on a real GitLab group with the §3 table walked and recorded as Evidence
(`forge-gitlab/05`, human-gated). Provisioning a group with this doc does not itself
constitute that pilot.
