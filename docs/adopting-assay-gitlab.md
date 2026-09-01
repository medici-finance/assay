# Adopting Assay on GitLab — the GitLab-profile install runbook

This is the GitLab-side companion to [`docs/adopting-assay.md`](adopting-assay.md) (the
GitHub runbook). It does not repeat CORE Assay concepts (briefs, registers, lifecycle,
board, statusgen) — those are forge-agnostic and unchanged. It covers only what is
GitLab-shaped: the identity model, the provisioning script, the ci-config-project
runbook, and token custody. The accepted design this doc implements is
[`docs/streams/forge-gitlab/spec.md`](streams/forge-gitlab/spec.md) — read it first if
anything here seems to assert a control without justifying it; the spec carries the
per-control parity table this doc only points at.

## 0. Tier ladder — read this before provisioning anything

> The GitLab profile MUST be **at least as secure as the existing GitHub controls**, even
> where the mechanism is completely different. "Weaker but disclosed" is non-conforming.
> — spec.md, governing requirement

- **Premium** is the floor. Below Premium there is no protected-branch push-access list
  granular enough to name a single bypass identity, no project-level approval-prevention
  settings, and no service-accounts feature — the identity model in §1 below cannot be
  built at all.
- **Ultimate is REQUIRED for public or risk-classed work.** Per-resource permission
  parity with a GitHub App only exists at Ultimate custom roles; below that, a service
  account's role grant is coarser than an App's per-resource permission matrix (spec.md
  §3, row 1).
- **GitLab Free / Community Edition (CE) is NON-CONFORMING for this profile.** It cannot
  meet the parity requirement — there is no protected-branch granular push-access list, no
  service accounts, and no project-level approval-prevention settings on Free/CE. Do not
  run the Assay fleet's write path against a Free/CE group and claim GitHub-equivalent
  guarantees; there are none to claim.

This statement is carried verbatim from spec.md §1 — no softening. If your group is
Free/CE, stop here; there is no provisioning path this doc can honestly hand you.

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
`--out-dir` controls where minted token files land (default: a fresh `mktemp -d`). Run
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

Every endpoint above is reachable at the **Premium** tier — nothing the script calls
requires Ultimate.

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
