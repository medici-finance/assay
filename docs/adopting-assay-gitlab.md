# Adopting Assay on GitLab — the GitLab-profile install runbook

This is the **GitLab half of one install flow**; [`docs/adopting-assay.md`](adopting-assay.md)
holds the shared half and the GitHub half. It does not repeat CORE Assay concepts (briefs,
registers, lifecycle, board, statusgen) — those are forge-agnostic and unchanged. It covers only
what is GitLab-shaped: the identity model, the provisioning script, the runner and CI/CD
variables, the ci-config-project runbook, and token custody.

**The one flow, in order, for a GitLab adopter:**

1. **§0 tier ladder** — decide what the edition you run can claim, before provisioning.
2. **§2 provisioning** — a human runs `tools/create-fleet-gitlab.sh`: the service accounts (the
   automation principals of the two-principals prerequisite), the protected `main`, and the desk
   labels (the GitLab form of the `create-labels` primitive).
3. **The shared half, from [`docs/adopting-assay.md`](adopting-assay.md)** — the turnkey
   `assay:install` skill on Claude Code, or its CORE primitives by hand. It needs **no `gh` and no
   `glab`**: the pinned binaries are fetched over plain HTTPS and sha256-verified against
   `.assay-versions`, and `statusgen init` writes `.gitlab-ci.yml` for a GitLab `origin`. The
   skill's *CORE primitives per forge* table names which primitives are forge-neutral and how the
   rest are expressed here.
4. **§2a runner, §2c `STATUSGEN_PUSH_TOKEN`, §2e `STATUSGEN_ROSTER_ENV`** — the human acts the
   scaffolded `.gitlab-ci.yml` needs before its first pipeline can prove anything.

`glab` is an optional convenience CLI for a human's own reads. No install step in either file
requires it. The one tool here that drives `glab` is the later PAT renewal (§2g), a human-run
operation that comes after the install.

The accepted design this doc implements is
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

**The unresolved-review-thread merge gate (row B3's server-side layer, every tier).**
Required approvals (B3, above) are advisory on Free — the reviewer service account's
Developer role can approve, but nothing on GitLab itself refuses a merge for lack of one.
GitLab does enforce, on every tier, that a merge request carrying an unresolved discussion
thread cannot be merged (the row this section's table adds above,
`only_allow_merge_if_all_discussions_are_resolved`). The desk turns that into a real gate: a
resolvable "merge-hold" discussion thread opens with every merge request the worker creates,
carrying a fixed marker body; the reviewer's approve verdict resolves it (recording the
approved head on a reply); a request-changes verdict, or a push past the head it was resolved
at, re-opens it. The GitLab merge button is blocked while it stands open — from the first
second, on Free, with no Premium route consulted. The `Draft:` title prefix stays exactly what
it always was: the human-facing "not ready yet" signal, additive to this gate rather than
replaced by it. And the human merge remains the outer gate it already is on this profile —
this control narrows what an accidental or bypassed merge can do, it does not remove the
human from the loop.

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
| issue-loop | service account | Developer (30) | `api`, `write_repository` | files/triages issues AND lands its exits (placeholders, closes) as draft MRs on a branch — GitLab refuses MR creation below Developer |
| intake-loop | service account | Developer (30) | `api`, `write_repository` | files/triages issues AND lands intake entries, specs and brief rows as draft MRs on a branch — GitLab refuses MR creation below Developer |
| board-writer | service account | Developer (30) + allowed-to-push entry on protected `main` | `api`, `write_repository` | the ruleset-bypass analog |
| auditor | service account | Reporter (20) for the `project` and `file` reads; **Maintainer (40)** for the protected-branches, protected-tags, approvals and push-rules reads — see §5a | `read_api` | GET-only hardening reads for `repohardenguard`; no write scope (the `read_api` scope is the forge-enforced read-only boundary whatever the role) |
| cell-issues | not yet mapped on GitLab | — | — | GitHub-only "write-issues" identity today (a narrower, per-purpose issues-filing role, selectable only by name); no GitLab consumer is wired to it yet |
| release-runner | **pipeline trigger token**, not a service account | — (a trigger token carries no project role) | none — it authenticates the trigger endpoint only | `deskrun` starts pipelines through `POST /projects/:id/trigger/pipeline` with it — the narrowest credential that can start a pipeline and nothing else. Custody is `gitlab-release-runner.token` (0600), read, never self-rotated: a trigger token has no self-rotate endpoint, so it is rotated in the project's CI/CD settings and re-provisioned. A trigger token cannot approve a gate or read a pipeline, so on GitLab `deskrun approve`/`status` report could-not-check under it. Bound per repo in `ASSAY_RUN_CREDENTIALS` with the gate shape the project uses (`release-runner+manual-job` or `release-runner+environment`) |
| promote | usually **no identity at all** — see §3 | — | — | workflow promotion is a human-merged MR into the ci-config project, not a bot act |

Attribution separation holds exactly as on GitHub: notes/approvals/commits carry the
service-account identity, which the PR/MR author's own token cannot produce — the same
honest limit as the GitHub profile (separation of attribution, not proof of diligence).

**The ready-flip's protected-branch read — `could-not-check` on this profile.** On GitHub the
reviewer App needs `Administration: Read-only` before `deskflip` can read the required status
checks of a protected branch whose required set is not expressed in a ruleset
(`docs/adopting-assay.md` §3 `setup-reviewer-app`).
GitLab has **no permission toggle of that shape**: the equivalent reads are
`GET api/v4/projects/:id/protected_branches/:name` and, for the merge-gating checks themselves,
`GET api/v4/projects/:id/external_status_checks`, both under the role's plain `api` scope, so the
grant is expressed as an **access level**, not a scope. **Whether Developer (30) — the reviewer
service account's level in the table above — can read either endpoint on your edition is
`could-not-check` here: it has not been measured on a live project, and GitLab has historically
gated protected-branch reads at Maintainer.** Read it back on your own instance before you claim
the flip gate works (the edition + read-back rule in §0 *Tier ladder* applies unchanged);
a 403 there is a **level** decision for a human, and raising the reviewer to Maintainer is not a free swap — it
carries push rights the Developer level deliberately withholds.

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

**On Windows, prefer `deskfleet provision` instead — this section is the fallback.**
`windows-port/08` ships a native Go verb that does the same provisioning natively on every OS,
including Windows, with no bash/curl/jq: `deskfleet provision --group <path> --prefix <name>
--owner-token-file <file> [--project <path>]` — see [`adopting-assay.md`](adopting-assay.md) §
**GitLab fleet provisioning**. It writes each minted token straight to `gitlab-<role>.token`, so
the link/copy custody step below (§2's "Where the role-token store is") does not apply to it. The
script below remains a supported, clearly-labelled fallback (#1646), run from Git-Bash or WSL. It is
never a Windows prerequisite for provisioning. **Renewal is different:** §2g's
`tools/renew-fleet-gitlab-tokens.sh` (bash + `glab`) has no native equivalent yet. `deskfleet` has
no renew verb, and a re-run of `deskfleet provision` mints nothing for accounts that already exist.
A Windows adopter therefore still runs renewal from Git-Bash or WSL until `windows-port/16` ships
`deskfleet renew`.

The script is idempotent bash + curl + jq, run by a human holding a **group-owner PAT**
(supplied only via the `GITLAB_TOKEN` environment variable — never a flag, never
committed, never stored by the script). It creates the seven service accounts above,
their group memberships, and their PATs; when `--project` is given, it also configures
that project's protected `main` branch and MR-approval settings. With `--tier ultimate`
it additionally scripts the two Ultimate refinements — a custom reviewer role that cannot
push, and an external-status-check verdict lane (§2b). The group token-expiry policy, the
ci-config project, and the pipeline execution policy remain the human-only remainder it
prints at the end (§4, §5).

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
which is what rotate-on-mint (§5) actually calls at operation time. To renew every
role's PAT in one run, use `tools/renew-fleet-gitlab-tokens.sh` (§2g).

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

Unix (`auditor` is optional — link it only if you provisioned that service account for
`repohardenguard`, per §1's role table; the fleet script does not provision it):

```
cd "$HOME/.config/assay" && for r in reviewer worker verifier desk issue-loop intake-loop board-writer auditor; do
  ln -s "<prefix>-$r-bot.token" "gitlab-$r.token"
done
```

Windows (no `ln -s` required): copy each `<prefix>-<role>-bot.token` to `gitlab-<role>.token`
in the same directory. Keep both files `0600`-equivalent (owner-only ACL).

**The link survives rotation — and the copy does not.** `desktoken --forge gitlab <role>`
rotates THROUGH the custody path: when `gitlab-<role>.token` is a symlink it resolves the link
and renames the new token onto the link's target, so the link stays a link and
`<prefix>-<role>-bot.token` holds the live credential afterwards. Re-running the `ln -s` loop
above therefore stays a no-op at any time, and a re-issue or re-provisioning pass that re-links
cannot re-point custody at a stale value. The COPY layout has no such property: after the first
rotation `gitlab-<role>.token` holds the live token and the `<prefix>-<role>-bot.token` copy
holds an invalidated one, so on that layout never copy the provisioned file back over
`gitlab-<role>.token` — doing so installs a dead credential, and the next desk verb fails `401`
on its first API read. Prefer the link on any platform that has one. (#1112)

The script itself is **bash + curl + jq** — a labelled **fallback** (#1646), not the Windows
path (see **`deskfleet provision`** at the top of this section). If you do run it on native
Windows, run it from Git-Bash or WSL, never from PowerShell.

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

**`ASSAY_SCAN_REPOS` — required for the issue lane, and (unlike `GITLAB_API_BASE`) it IS a
`roster.env` key.** Once the fleet PATs exist and `deskboot` / `deskroster preflight` goes
green, the write lane works — but the **issue lane** (`issueboard`, `scanloop`,
`deskroster repos --scope scan`) refuses **LOUDLY** with **exit 6, COULD-NOT-CHECK** when
`ASSAY_SCAN_REPOS` is unset or empty (`tools/desk/cmd/issueboard`, `deskkit.ScanRepos`): an
empty sweep is never reported as a clean, empty board, so intake-desk and worker-desk treat
the GitLab issue/orphan lane as could-not-check rather than empty. **It is a DISTINCT key from
the write boundary `ASSAY_ALLOWED_REPOS`** — a green write-lane preflight says nothing about
it, and `ASSAY_ALLOWED_REPOS` already listing the adopter project does not set it.

- **Required value shape.** A comma-separated list of `<group>/<project>` slugs — the scan
  scope. It must contain **at least the adopter project itself** (the same slug that appears
  in `ASSAY_ALLOWED_REPOS`); it may be **wider** than the write boundary, since the scan scope
  covers every repo the desk is the front door for even where the desk is not a write target
  (the desk still posts only where `deskpost` / `deskpr` / `deskreply` gate independently on
  `deskkit.IsAllowedRepo`). Example, for a single-project cell:

  ```
  ASSAY_SCAN_REPOS=mygroup/myproject
  ```

- **Where it lives.** Like the other `ASSAY_*` operator config, it is set in CI (the
  project/group CI/CD variable) **or** the config-home `roster.env` — never compiled in. This
  is the post-fleet-boot checklist item that is easy to miss precisely because the write lane
  goes green without it.

- **Verify.** Run the tool and read the effective value it echoes to **stderr**:

  ```
  issueboard 2>&1 >/dev/null | grep -E 'ASSAY_SCAN_REPOS=|configured='
  ```

  A non-empty `ASSAY_SCAN_REPOS=` listing at least the adopter slug is the pass; `exit 6`
  with an empty value is the silent half-configured state this step exists to close.

**`ASSAY_REPO_FORGES` — required so a GitLab-provisioned repo is not read as the default
forge.** Every desk verb resolves which forge software serves a repo before it does
anything else (`ForgeFor`, `tools/desk/internal/deskkit/forgeresolve.go`): first the
roster's `ASSAY_REPO_FORGES` entry for that repo, then — only when the roster is silent —
the origin remote's host mapped through a short, exact-match table (`github.com` → GitHub,
`gitlab.com` → GitLab). A self-hosted GitLab instance has no host literal that table can
match, so a correctly provisioned self-hosted fleet with this key unset either resolves as
the default forge or refuses could-not-check — the symptom this key exists to close.

- **Required value shape.** A comma-separated list of `<owner>/<name>=gitlab` (or
  `=github`) entries — **full slug only**, a bare basename is refused. This key is held to
  a stricter grammar than the display-only `ASSAY_REPO_ALIASES`: it chooses which minted
  credential a write is performed as, so a malformed entry, an unrecognised forge value, or
  a repo bound twice refuses the WHOLE roster rather than degrading one feature. Example,
  for a single-project cell:

  ```
  ASSAY_REPO_FORGES=mygroup/myproject=gitlab
  ```

- **Where it lives.** Like `ASSAY_SCAN_REPOS` above, the config-home `roster.env` — never
  compiled in.
- **Unset is not itself an error** for a `gitlab.com` / `github.com` project — the
  remote-host fallback (and, failing that, an Unverifiable could-not-check refusal naming
  the repo) is a complete answer on its own there. It is a **self-hosted** GitLab project
  that has no fallback and needs this key to resolve at all.

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

## 2b. Ultimate refinements — scripted, with verification (`--tier ultimate`)

On an **Ultimate** instance, `--tier ultimate` scripts the two refinements the parity
table (§0.1, rows B8/B9) leaves human-only on lower tiers. Both are **optional
hardening**, never a prerequisite for the core lane (ruling #219): each converts a
disclosed CE degradation into a server-enforced control.

```
GITLAB_TOKEN=<group-owner PAT> tools/create-fleet-gitlab.sh \
  --group mygroup --prefix myorg --project mygroup/myproject \
  --tier ultimate --status-check-url https://<your-verdict-lane>/status
```

Dry-run first — it enumerates the custom-role and status-check steps and makes zero
network calls:

```
tools/create-fleet-gitlab.sh --dry-run --tier ultimate --group mygroup --prefix myorg
```

**A. Custom reviewer role that cannot push (row B9).** The reviewer service account is
given a custom member role — a **Reporter** base (which cannot push to any branch) plus
the `admin_merge_request` ability — via `POST /groups/:id/member_roles`, then bound to the
reviewer member. This restores the GitHub-App granularity of "approve MRs but never write
code" that a plain Developer role cannot express. On a non-Ultimate instance the
member-roles endpoint returns **403**; the script records that as `could-not-check` under
the HUMAN-ONLY REMAINDER and exits non-zero — it never silently downgrades.

*Verify the role cannot push* (the negative test — run it against a scratch project, as
the reviewer's own PAT, not the owner PAT):

```
# Expect: remote rejects the push (protected/insufficient permission), non-zero exit.
git -c http.extraHeader="PRIVATE-TOKEN: $(cat <out-dir>/myorg-reviewer-bot.token)" \
  push https://gitlab.com/mygroup/scratch-project HEAD:refs/heads/reviewer-push-probe
echo "exit=$?  # non-zero == the role cannot push, as intended"
```

A push that SUCCEEDS is a finding: the role has write access it must not have — do not
proceed until the push is rejected.

**B. External-status-check verdict lane (row B8).** With `--project` and
`--status-check-url`, the script registers an external status check (default name
`assay-verdict`; override with `--status-check-name`) via
`POST /projects/:id/external_status_checks`. The desk's verdict lane then posts pass/fail
against the MR head SHA through the forge seam
(`postExternalStatusCheckVerdict` in `tools/desk/internal/deskkit/forge_gitlab.go`) — a
required merge check the lane satisfies with **zero repo write access**. Make it a required
check on protected `main` in **Settings > Merge requests > Status checks**. On a
non-Ultimate instance the endpoint returns 403; again recorded as `could-not-check`, never
a silent pass.

*Verify the check is registered and required:*

```
# The check appears in the project's external status checks:
curl -sS -H "PRIVATE-TOKEN: <owner PAT>" \
  "https://gitlab.com/api/v4/projects/<id>/external_status_checks" | jq '.[].name'
# Expect: "assay-verdict" (or your --status-check-name) present.
```

The tier gate is the single point of failure by design: both endpoints are Ultimate-only,
so a 403 there is exactly the signal that routes verdict posting back to the three-state
fallback rather than posting to a surface that does not exist.

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
credential is missing (see §2c, board-push credential). A job that never leaves `pending`
proves nothing about either; it is a runner-match gap, and the fix is a runner, not a token.

This is the CI half of the Free-tier "pipeline execution gate" degradation in §0.1: that row
assumes a runner exists once the CI file is scaffolded. It does not exist until you provide
one here.

## 2c. Board-push credential — `STATUSGEN_PUSH_TOKEN`

The scaffolded `statusgen-regen` job (`.gitlab-ci.yml`, generated by `statusgen init --forge
gitlab`) cannot push the regenerated `STATUS.md` with the job's own `CI_JOB_TOKEN` — GitLab's
default job token has no push scope. The job needs a **separate** masked **and protected**
CI/CD variable holding a real push credential, and refuses with a clear message rather than
guessing when it is unset. The instruction below is dereferenced from the generated file's own
source (`statusgen/init.go`, `initGitlabCI`), whose job script prints, verbatim, on an unset
token:

```
STATUSGEN_PUSH_TOKEN is not set — cannot push the regenerated board.
Create a project access token with the write_repository scope and set it as a masked and
protected CI/CD variable named STATUSGEN_PUSH_TOKEN (protected: the default branch must be a
protected branch). Refusing rather than guessing.
```

- **Token kind.** A **project or group access token** — GitLab's bot-user-backed token
  scoped to one project or its whole group. This is deliberately **not** a human's personal
  credential (no human PAT belongs in a project-level CI/CD variable) and **not** one of the
  §1 role-fleet service-account tokens (`gitlab-<role>.token`): the fleet identities mint and
  rotate through the `desktoken --forge gitlab <role>` custody path documented in §5, which
  is a different credential class from a CI variable a runner reads directly. Do not point
  `STATUSGEN_PUSH_TOKEN` at a role-fleet token file.
- **Scope.** `write_repository` only — the literal scope named in the refusal text above. Do
  not add `api`; the regen job pushes a file, it does not call the REST API.
- **Minimum role under a protected default branch.** A project/group access token's bot user
  is subject to the same protected-branch push check GitLab applies to a human member at the
  same role; the valid `push_access_level` values are `0` ("No one"), `30` Developer, `40`
  Maintainer, and `60` Admin (self-managed only) (docs.gitlab.com, Protected Branches API).
  Mint the token at **Maintainer**: §2's by-hand table names `access_level: 40` (Maintainer)
  as the role the provisioning script's protect step grants an exception to, and a
  self-managed instance's protected-branch read-back has been observed to stay at Maintainer
  (`40`) even when the script requested "No one" (§2, "the protect step is tier-aware").
  **If your instance's own printed read-back genuinely holds `push_access_level = 0`**, no
  token role clears a direct push under it — the regen job cannot push at all, and board
  regeneration has to travel as a human-merged MR instead, the same Free-tier degradation
  §0.1's board-writer row already discloses. Read your instance's protect-step read-back
  before relying on Maintainer being sufficient; this doc does not assert it holds on yours.
- **Variable visibility — masked AND protected, both required.** Create it under
  **Settings > CI/CD > Variables** at the project (or group, if shared across the fleet) that
  owns the pipeline, flagged **Masked** (so a leaked job log never prints it) **and**
  **Protected** (so it is exposed only to pipelines running on protected refs — the default
  branch this job runs on already is one, per §2's protect step). Protected is not optional: a
  masked-only variable is still injected into merge-request pipelines, which run the MR
  branch's own CI file, so any member who can open an MR could read the token and push to the
  default branch past the merge gate. Via the API the same variable is
  `POST api/v4/projects/:id/variables` with `key=STATUSGEN_PUSH_TOKEN`, `masked=true`,
  `protected=true` (a GitLab adopter cell's review found a masked-only variable in the field;
  that variable is now protected).

The runners section above used to send the reader to the per-role fleet credential section
by a wrong number — the section that actually holds the per-role rotation rules is §5, and
its subject is a different credential entirely. Read `STATUSGEN_PUSH_TOKEN` from this
subsection instead.

## 2d. Source-pin lane — GitLab + native Windows (channel D)

A GitLab adopter on **native Windows** installs `statusgen` through the same channel D
"from-source" lane a native-Windows adopter on the other forge uses. This runbook does not
fork a second copy of that lane's grammar — a forked copy is the one that goes stale (#896).
The full lane (cloning this repo at a pinned commit, building `statusgen` and the desk-tool
cmds, and scaffolding with `statusgen init --forge gitlab --root <adopter>`) is documented in
[`adopting-assay.md`](adopting-assay.md), section "Channel D — from-source Windows"; read it
there before writing a `.assay-versions` line on this lane.

**The one rule worth restating, because getting it wrong stalls the board silently.** Channel
D writes **no** `-source` `.assay-versions` pin line at all — the `-source` grammar
(`<artifact> <tag> <40-hex-commit-SHA>`) is a release-provenance mechanism for a
**published** tag, and the literal string `channel-D` is never a field in it. A live install
that wrote a `statusgen-source <sha> channel-D` line anyway then ran `deskboard dispatch` and
got **no statusgen pin found**, reading a stale board as an all-clear (#896). The desk-tools
pin reader now accepts the commit in either field position (#795), so a wrongly-shaped line
can be *accepted* by that one reader and still be refused by the generic pin reader
(`tools/desk/cmd/deskboard/main.go`), which returns fields two and three positionally — the
asymmetry is exactly why the rule is "write no source pin line", not "write it carefully."
Prove the binary that ran from `statusgen --version` / desk-tools' `sourceSHA=<shortsha>`,
never from a fabricated `.assay-versions` entry.

## 2e. Trust roster for CI — `STATUSGEN_ROSTER_ENV`

The scaffolded jobs (`.gitlab-ci.yml`, generated by `statusgen init --forge gitlab`) run
`statusgen` on a runner that has **no trust roster** unless you give it one. There is no
`GITHUB_ACTIONS` on a GitLab runner, so `statusgen` is in its file-only roster class and reads
`$HOME/.config/assay/roster.env` — a file no step wrote on a fresh runner. Before #1110 the regen
job therefore logged

```
no roster configured: /root/.config/assay/roster.env does not exist … role-bindings=(none bound)
```

and **still succeeded**: `STATUS.md` was written and pushed, while every roster-backed check —
the Evidence-actor check on `verified`/`done` rows (which identity committed each brief's
Evidence lines, and is it the bound verifier), and the trust gates — reported could-not-check
on every regen. A could-not-check is not a pass, and nothing in the pipeline result said so.

**The fix the scaffold now carries.** Both jobs (`statusgen-lint` on merge requests and
`statusgen-regen` on the default branch) run a shared `.statusgen-roster` step before their
`statusgen` invocation that materialises the **non-secret half** of the roster from one CI/CD
variable:

- **Name.** `STATUSGEN_ROSTER_ENV`.
- **Contents.** The `roster.env` lines an operator writes to the config-home per
  [`adopting-assay.md`](adopting-assay.md), section `configure-roster` — the five surfaces,
  `ASSAY_BLESS_LOGIN`, `ASSAY_TRUSTED_LOGINS`, `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`,
  `ASSAY_HUMAN_LOGIN_MAP`, plus the GitLab-specific keys §2 names (`ASSAY_REPO_FORGES`,
  `ASSAY_SCAN_REPOS`). For the Evidence-actor check to run at all, `ASSAY_TRUSTED_BOT_SLUGS`
  must carry the `verifier=` binding in the forge-qualified form, for example
  `ASSAY_TRUSTED_BOT_SLUGS=verifier=gitlab:example-verifier-bot,reviewer=gitlab:example-reviewer-bot`.
- **Type.** Either **Variable** (multi-line value: the contents themselves) or **File** (GitLab
  hands the job a path to a temp file holding the contents). The step accepts both — it copies a
  path, and writes a value — so the type you pick cannot silently produce a one-line roster
  holding a temp-file path.
- **Visibility — NOT masked, NOT protected.** It holds logins, numeric ids and role bindings
  only, nothing a job log could leak, and it must reach merge-request pipelines too so the
  `--lint` half has the same roster as the regen half (a protected variable is withheld from
  MR pipelines). This is the opposite of `STATUSGEN_PUSH_TOKEN` (§2c) on purpose: that one is a
  credential, this one is a register.
- **Never a token, key, or password.** The roster is the non-secret half and the step enforces
  it: a line whose key is secret-shaped (`…TOKEN…=`, `…SECRET…=`, `…PASSWORD…=`,
  `…PRIVATE_KEY…=`) makes the job **refuse with exit 1** and delete the file it wrote, before
  `statusgen` runs. Do not put `STATUSGEN_PUSH_TOKEN` or a `gitlab-<role>.token` value in it.
- **Where it lands, and how.** `$HOME/.config/assay/roster.env`, directory `0700`, file
  `0600` — the owner-only mode the loader enforces (a group- or world-writable file or
  directory is refused). Via the API: `POST api/v4/projects/:id/variables` with
  `key=STATUSGEN_ROSTER_ENV`, `masked=false`, `protected=false`, and `variable_type=env_var`
  (or `file`).

**The verifier's display-name gap — `ASSAY_GITLAB_DISPLAY_NAMES` (#1477).** A GitLab account
has a username AND a separate **display name**, and every commit GitLab writes carries the
**display name** in `author_name`, never the username the roster binds. The offline Evidence-actor
check (`statusgen --lint` runs offline on the runner and cannot resolve the account) therefore
could never match the bound verifier's Evidence commit on a deployment whose service account has
an ordinary display name — and, since a new `verified` closure with no accepted actor is a hard
`PROBLEM`, `implemented → verified` was permanently blocked. Declare the bound account's display
name so the offline check has a value to compare against:

```
ASSAY_GITLAB_DISPLAY_NAMES=example-verifier-bot=Assay verifier (fleet bot)
```

- It is a map `<username>=<display name>`, entries separated by `;` or a newline — **not** the
  comma/space list separator every other roster value uses, because a display name contains
  spaces (and only the first `=` splits an entry, so the display name keeps its spaces). The
  username must match the `verifier=gitlab:<username>` slug.
- It is **non-secret** (a public display name) and belongs in `STATUSGEN_ROSTER_ENV` beside the
  other roster lines. It is a statusgen-only key; the desk verbs recognise it and ignore it (they
  resolve the account **online** via the typed forge instead — see below), so a roster carrying it
  does not disturb the desk side.
- Renaming the service account's display name to equal its username is a valid deployment
  workaround, but it silently re-breaks whenever an admin edits the display name; declaring it
  here is the durable fix.
- **Online alternative.** The desk verb that lands Evidence (`deskevidence`) resolves the landed
  commit's GitLab account to its username through the typed forge (`GET /users?search=`) and
  compares that to the verifier binding — so a deployment whose runners can reach the API gets the
  check for free without declaring display names. The offline map is the fallback for the
  network-free `--lint` path; unset, that path is simply could-not-check for the display-name case
  (never a false pass).

A roster-known **human** verifier on GitLab is accepted by the Evidence-actor check via GitLab's
private commit noreply address `<user-id>-<username>@users.noreply.<host>` the way a GitHub human
is via the GitHub form — no extra configuration beyond the human's `ASSAY_TRUSTED_LOGINS` entry.

**Loud when absent.** With the variable unset the step prints, verbatim from the generated
file's own source (`statusgen/init.go`, `initGitlabCI`):

```
NOTICE: STATUSGEN_ROSTER_ENV is not set — no trust roster for this job.
```

followed by what `statusgen` will report without it (could-not-check on every roster-backed
check) and the variable to set. The job **continues** — the board still regenerates, exactly as
before — but the gap is now named in the log rather than inferable only from
`role-bindings=(none bound)`.

**Verify.** In the regen job's log, `assay-config: … configured=true` and a `role-bindings=`
line naming your `verifier=` binding — not `(none bound)`, and no `NOTICE: STATUSGEN_ROSTER_ENV
is not set` line. Locally, the same materialisation can be proven without a runner: extract the
`.statusgen-roster` script from the generated `.gitlab-ci.yml`, run it with `HOME` pointed at an
empty directory and `STATUSGEN_ROSTER_ENV` set to your roster lines, then run
`HOME=<that dir> statusgen --root .` and read the `assay-config:` echo.

**What this does not cover.** The roster names *who* the verifier is; it does not make the
regen job's `git blame` any stronger than the commit metadata it reads (see `statusgen`'s
Evidence-actor check: tamper-evident, not tamper-proof). And the acting desk tools never read
this variable — they are file-only everywhere, and the file they read is the one on the
operator's machine, not the runner's.

## 2f. Dead-claim decay credential — `CI_JOB_TOKEN`, or `STATUSGEN_GITLAB_TOKEN`

statusgen builds its claim set from open `origin` branch heads: a branch that looks like a
brief's branch is treated as in-flight work and subtracts from that stream's dispatch cap.
`git ls-remote` is a pure ref view, so it still reports the head of a branch whose change
already landed. **Dead-claim decay** is the pass that drops those corpses, and to run it
statusgen has to ask the forge one question per branch — has this change already merged or
closed?

On GitLab that question is answered by the project's **merge-request listing over REST v4**
(`GET /projects/:id/merge_requests?state=all`). No `gh` is involved, and none is needed: a
GitLab project has no pull requests to list, which is why an earlier statusgen simply
declined to run this pass on a GitLab remote and a GitLab adopter's claims never decayed at
all (issue #1111).

- **In CI, nothing to configure.** Every GitLab job carries the predefined `CI_API_V4_URL`,
  `CI_PROJECT_ID` and `CI_JOB_TOKEN`, and the scaffolded `statusgen-regen` job uses them as
  they are.
- **Override — `STATUSGEN_GITLAB_TOKEN`.** Some instances do not expose the
  `merge_requests` endpoint to the job token. Where yours does not, create a **project
  access token with the `read_api` scope** and set it as a masked CI/CD variable named
  `STATUSGEN_GITLAB_TOKEN`; statusgen prefers it over the job token. `GITLAB_TOKEN` is
  accepted under the same rule, for a local run outside CI. This is a **read** credential
  and is a different variable from `STATUSGEN_PUSH_TOKEN` (§2c, `write_repository`) — do not
  reuse one for the other.
- **Outside CI**, with neither variable set in the environment, statusgen derives the API
  base and project path from the `origin` remote but still **requires** a token: an
  unauthenticated listing of a private project answers `404`, whose empty body would decode
  as "nothing is dead" and decay nothing while reading exactly like a clean run.

**What an unreadable listing does — could-not-check, not a pass and not a failure.** When
the read cannot happen (no token, a refused endpoint, an API error), the run prints

```
could-not-check: claims not decayed — merge-request state over the GitLab REST v4 API could not be read: <reason>
```

and the generated `STATUS.md` carries the matching banner at the head of its Next-up
section. The job is **not** failed over it. The direction is why: an undecayed claim set is a
*superset of claims*, so the board is a **subset** — briefs held behind already-merged
branches are missing from it. That hides work; it never hands the same brief to two
sessions, which is the failure the claim read's own `--require-claims` flag exists to stop.
Read the banner as *some backlog may be hidden*, and regenerate once the listing is
readable to release it.

## 2g. Renewing every role PAT at once — `tools/renew-fleet-gitlab-tokens.sh`

The provisioner mints a PAT only for an account it creates in that run (§2 *Idempotency*). The
same holds for the native `deskfleet provision`. **On Windows this script has no native
equivalent yet.** It is bash + `glab`, so run it from Git-Bash or WSL. `windows-port/16` ports it as
`deskfleet renew`. The
renewal is the companion for accounts that already exist: one command rotates each configured
role's live PAT, creates one only where the role has none, and replaces each
`gitlab-<role>.token` in the role-token store. It reads the same role table as the provisioner
(`tools/fleet-gitlab-roles.sh` — the one copy both scripts source), so the scopes, the
`assay-<role>-fleet` PAT names and the `<prefix>-<role>-bot` usernames cannot drift between
them.

It drives `glab api`, logged in as an **Owner (access level 50) of the top-level group** named by
`--group <top-level-group>`. That is the one authority model. The calls are the group
service-account endpoints the provisioner already uses:
`groups/:id/service_accounts/:user_id/personal_access_tokens` (list with `?state=active`,
`…/:token_id/rotate`, create). The script never requires instance administrator and never probes
for it: it makes no `application/settings` read and does not use `glab token … --user`, which is
administrator-only and the wrong transport for this.

Every deployment-specific value is an argument; nothing is built in. `--duration` defaults to
the shared `FLEET_PAT_DAYS` in `tools/fleet-gitlab-roles.sh` (7 days — spec.md §5's "7 days
RECOMMENDED" expiry backstop), so leaving it off keeps the backstop; passing a longer value
widens it, which matters most on Free tier, where there is no group token-expiry policy to cap
it independently (§Free-tier degradations above):

```
tools/renew-fleet-gitlab-tokens.sh --dry-run \
  --hostname gitlab.example.com --group mygroup --prefix myorg \
  --out-dir "$HOME/.config/assay"

tools/renew-fleet-gitlab-tokens.sh \
  --hostname gitlab.example.com --group mygroup --prefix myorg \
  --out-dir "$HOME/.config/assay"
```

An installation that does not use the provisioner's naming describes each role explicitly with
`--role ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE]`, repeated once per role. `SCOPES` is
comma-separated, and `FILE` is a bare file name that defaults to `gitlab-<ROLE>.token`. Used
with `--prefix`, a record replaces that role's table row, and a record for any other role (for
example `auditor`) adds it. Run `--help` for the full reference.

What one run does, in order:

1. **Preflight, before any token changes.** The script checks the arguments, `glab` and `jq`,
   the output directory (it must be writable) and each destination file. It also checks that
   the active `glab` identity is an Owner of the group. It resolves each service account from
   the group's own service-account listing and lists each role's active PATs, and each role gets one
   of four outcomes: **rotate** (exactly one active PAT with that name, and it is not
   protected — see below), **create** (none), **skip-in-use** (exactly one active PAT, but its
   `last_used_at` falls inside the in-use window), or **refuse** (more than one active PAT; the
   script will not guess which one is live, so revoke the extras). The script decides a listing
   from its parsed JSON, never its byte count, and refuses an unreadable one instead of
   treating it as "no match", which would otherwise create a second live credential. That
   covers a listing with no JSON value at all (zero bytes, a bare newline, whitespace only), a
   non-array, and a record missing its id, name, `active` flag or `last_used_at` key. A
   multi-page listing is merged across pages. A literal `[]` is a real "no active PAT" and
   leads to a create. Any preflight
   problem aborts the run before any role is rotated. `--dry-run` runs this whole phase and
   prints the plan (`would-rotate` / `would-create` / `would-skip-in-use` for each role), then
   stops. It makes no mutating call and writes no file.
2. **In-use protection.** Rotating a live PAT invalidates it immediately (see below), so a role
   whose active PAT was used inside the in-use window is **skipped by default** and reported as
   `skipped-in-use`, not rotated. Pass `--rotate-in-use` to rotate it anyway — do that only after
   confirming nothing is still relying on that credential (stop the fleet's desk sessions first).
   A PAT that has never been used (`last_used_at` is null) is never in-use, and a PAT last used
   before the window (for example three days ago) is not in-use either. Both rotate normally.
3. **Renewal, one role at a time.** `glab` writes the returned secret straight into a new
   owner-only (`0600`) temp file in the destination's own directory. The secret never goes
   through argv, an environment variable, a log line or the report. The file must hold exactly
   one line that looks like a token. If it does not, the output is malformed: the script
   deletes the temp file, keeps the old file, and stops. Otherwise it renames the temp file
   onto the destination, which replaces the file atomically. If `gitlab-<role>.token` is a
   symlink (§2's link layout), the rename lands on the link's target, so the link survives,
   the same way `desktoken` handles it (§5). A link that resolves outside `--out-dir` is
   refused during preflight.
4. **The report** gives only the role, the destination path and the outcome for each role
   (`rotated`, `created`, `skipped-in-use (…)`, `failed (…)`, `not-attempted`). It never
   includes a secret, a username or a token id. The summary counts renewed roles and skipped
   roles separately, and states the new expiry only for the renewed ones: a skipped role's
   PAT keeps its previous expiry.

**Exit status.** `0`: every selected role was renewed (with `--dry-run`, the plan passed
preflight). `1`: a preflight refusal (nothing changed) or a role failed part-way. `2`: a usage
error. `3`: the run finished, but at least one role was skipped as in-use and **not** renewed.
The summary names those roles and prints the `--only <roles> --rotate-in-use` re-run for once
nothing holds them. A run that skipped a role never exits `0`, so a fleet with one role left
behind cannot look fully renewed.

**A rotation invalidates the old credential immediately.** GitLab revokes a role's previous PAT
as soon as it accepts the rotation. Any process still holding the old value then gets a `401`.
Stop the fleet's desk sessions before you renew. `desktoken`'s per-role rotation lock applies
only between `desktoken` calls, and this script does not take it.

**The operation is not atomic across the fleet.** Each role's file swap is atomic. The fleet as
a whole cannot be, because each role is a separate rotation on the forge. The run stops at the
first role that fails. Roles before that one have been renewed, and roles after it have not
been touched. The script then prints the `--only <role,...>` list that resumes the run: fix the
cause and re-run the same command with that flag. A role that failed during its own rotation
(a `glab` error or malformed output) may already have lost its old credential on the forge. The
resume rotates that role's current active PAT again, or creates one if it has none, so it
recovers without any hand work.

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
| Require all threads resolved before merge | `PUT api/v4/projects/:id` | `only_allow_merge_if_all_discussions_are_resolved: true` — the merge-hold marker thread's server-side half (§0.1); Free tier, read back the same way as the pipeline flag above |
| Create the desk labels | `POST api/v4/projects/:id/labels` | one call per label, idempotent (a duplicate name answers 409, or 400 "already exists"); the queue-legibility pair `authorization-needed` / `approval-needed`, the `review-request` dispatch token, and one `raised-by:<role>` per filing role |

Every endpoint above is reachable at the **Premium** tier — nothing the script calls
requires Ultimate.

The label set is the GitLab twin of the GitHub **`create-labels`** primitive
(`docs/adopting-assay.md`) — the SAME names, colors and descriptions, so the two
adoption profiles are label-parity. It matters because a label that is absent when a
desk verb reaches for it degrades **silently**: `deskflip`'s `authorization-needed` →
`approval-needed` queue swap fails, and `deskfile --raised-by <role>` drops the
provenance stamp (#774). The script creates them under `--project`; colors are
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

## 4a. The CI leak-sweep half — the free-tier disclosure compensator

The live pilot found the disclosure control absent on GitLab: no `.gitlab-ci.yml`, no
pipelines, and `secret_push_protection_enabled: false`
(`docs/streams/forge-gitlab/pilot-report.md` §3 row 8). The leak gate's strong verdict is a
status posted **out of band** by the control-based sweep (it needs the private withheld-token
map and cannot run in an adopter's CI), so on GitLab a change can carry a green pipeline with
the leak gate never having run. This section closes that gap with the **pipeline-side
leak-sweep job** — the free-tier layer that runs the in-tree (pattern-half) controls in the
change's own pipeline and fails it on a hit.

Add the leak-sweep job to the shared `.gitlab-ci.yml` you commit into the ci-config project
(§4 step 3). Its exact shape — the job, its `rules`, and the three-state property (passed /
failed / absent-is-could-not-check) — is templated by `forge-neutral/08` and specified in
[`docs/streams/forge-neutral/gitlab-ci-half.md`](streams/forge-neutral/gitlab-ci-half.md);
where it sits in the two-layer gate design (external verdict + pipeline-side sweep, and which
layer blocks on which tier) is [`docs/streams/forge-neutral/leak-gate-shape.md`](streams/forge-neutral/leak-gate-shape.md).

The load-bearing points for an adopter:

- **On CE / free tier the sweep job is the merge blocker.** CE cannot express a blocking
  external status check (the tier-gated surface returned `HTTP 401` on the pilot, §0.1), so
  make the `leaksweep` job a **required** pipeline step in the project's merge-request
  settings — a failing required pipeline is what blocks the merge here.
- **On Ultimate the sweep job is the second, independent layer** behind the external status
  check, catching a change on a different signal (its own CI) than the external verdict.
- **An absent sweep is could-not-check, never a pass.** A required pipeline that produced no
  `leaksweep` job reads as a missing required step, not as a clean run — the same three-state
  contract the external verdict honours.

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
- **Custody layout the rotate path expects.** `gitlab-<role>.token` on the config-home search
  path, resolving to a `0600` regular file — either the file itself or a **symlink** at that
  name pointing at one (§2's link step). A rotation resolves the link, writes the new token to
  a temp file in the target's own directory, `fsync`s it, renames it onto the target, `fsync`s
  that directory, and reads the value back THROUGH `gitlab-<role>.token`. Two properties follow,
  and both are what the verbs depend on: the link is never replaced by a regular file, so
  exactly one file holds the live credential whichever name reaches it; and the value is durably
  written before the command prints the path, so the verb's own read after its own rotation
  cannot return the pre-rotation token. A rotation that finds the link gone after its write
  fails at mint time rather than leaving a layout that reads stale later. (#1112)
- **Audit events** (Premium+) should be reviewed periodically for rotation/use anomalies.

## 5a. Hardening checklist — `repohardenguard` on GitLab

`repohardenguard` compares a project's LIVE settings against a checklist document and reports
three states per row — `checked-ok`, `checked-wrong`, `could-not-check` — plus
`not available` for a setting the edition does not offer. It reads through the `auditor`
identity (§1) and ONE enumerated forge operation over a closed kind vocabulary; a checklist
row's Read cell is `read <kind>` or `read file <path>`, never an endpoint. **A checklist is
written per forge**: each GitLab kind is a fixed endpoint returning GitLab's own settings
document, and the GitHub kinds (`repo`, `rulesets`, …) are refused by name on a GitLab
project — a GitHub row copied into a GitLab checklist reads `could-not-check` naming both
forges, never GitLab's nearest document.

### The GitLab kinds, their tier, and the auditor's minimum project role

| Kind | Reads | Tier | Minimum role for the auditor PAT |
|---|---|---|---|
| `project` | `GET /projects/:id` — `.visibility`, `.only_allow_merge_if_pipeline_succeeds`, `.only_allow_merge_if_all_discussions_are_resolved`, `.ci_config_path`; `.ci_allow_fork_pipelines_to_run_in_parent_project` (Owner/admin-visible only); `.secret_push_protection_enabled` (Ultimate) | Free | Reporter (20) |
| `file <path>` | the file's presence on the default branch (SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md) | Free | Reporter (20) |
| `protected-branches` | `GET /projects/:id/protected_branches`, every page, as one list — `[name=main].allow_force_push`, `[name=main].push_access_levels.0.access_level`, `[name=main].merge_access_levels.0.access_level` | Free at role level; `user_id` / `group_id` entries are Premium | Maintainer (40) — see the note below |
| `protected-tags` | `GET /projects/:id/protected_tags`, every page — `[name=v*].create_access_levels.0.access_level` | Free at role level | Maintainer (40) — see the note below |
| `approvals` | `GET /projects/:id/approvals` — `.reset_approvals_on_push`, `.merge_requests_author_approval`, `.merge_requests_disable_committers_approval` | the read answers 200 on gitlab.com Free (404 on some self-managed CE); enforcement is Premium | Maintainer (40) — see the note below |
| `push-rules` | `GET /projects/:id/push_rule` — `.reject_unsigned_commits`, `.prevent_secrets` | **Premium** — on Community Edition the route answers 404/403 | Maintainer (40) — see the note below |

**The Maintainer rows are a stated minimum, not a measured one.** GitLab's API pages for
protected branches, protected tags, push rules and project approvals (dereferenced
2026-09-15) state no minimum role for the GET, and GitLab has historically gated the
protected-branch settings reads at Maintainer. Before you rely on a checklist, read the
document back with the auditor PAT itself — `curl -sS -o /dev/null -w '%{http_code}'
-H "PRIVATE-TOKEN: <auditor PAT>" "$GITLAB_API_BASE/projects/<group>%2F<project>/protected_branches"`
— and treat `200` as the proof and `403` as "raise the auditor's project role", the same
edition + read-back rule §0 applies to everything else on this profile. Raising the auditor to
Maintainer does NOT widen what it can write: the PAT carries only `read_api`, which the forge
enforces on every request regardless of role. The identity table in §1 records both levels.

### How the `Gated` cell reads on GitLab

`Gated` decides what an ABSENT value means. `public`: the document was read and the field is
not there, so the setting is off — `checked-wrong`. `admin`: an absence is indistinguishable
from a permission or tier wall, so it is `could-not-check`, never a pass and never a failure.
On GitLab that makes `admin` the right cell for every row a tier or role can hide:

- the `push-rules` rows — on Community Edition the route is 404/403, and even on Premium a
  project with no push rule answers the literal document `null`;
- the `approvals` rows — some self-managed Community Edition instances answer 404;
- the Owner-visible project field `ci_allow_fork_pipelines_to_run_in_parent_project`, which
  is simply missing from the document at any lower role.

A 403 is `could-not-check` on every row whatever the cell says: a permission wall says nothing
about the value behind it. And the `not available — <tier>` Required cell makes the guard
issue **no request at all** for that row: on a plan that lacks the feature, asking would only
produce a wall-shaped error that muddies the two states that matter. Two independent layers
therefore stand between a Premium endpoint on CE and a false pass — the backend's own
three-state classification of the 403/404, and the checklist's `not available` short-circuit
that never asks.

### The Community Edition template

Copy this block into your project's hardening checklist document (the directive names every
project the document covers; the guard refuses a row naming any other) and point
`repohardenguard --repo <group>/<project> --checklist <that file>` at it. Required cells are
the profile's values for a private project; a public project sets `visibility` to `public`.
Cells that name your CI-config project (§4) are placeholders to replace.

<!-- repohardenguard:repos: example-group/example-project -->
<!-- repohardenguard:rows:begin -->
| ID | Repo | Setting | Gated | Read | Field | Required | Set |
|---|---|---|---|---|---|---|---|
| visibility | example-group/example-project | project visibility | public | `read project` | visibility | private | Settings → General → Visibility, or `PUT /projects/:id` `visibility` |
| merge-pipeline | example-group/example-project | pipelines must succeed before merge (B5) | public | `read project` | only_allow_merge_if_pipeline_succeeds | true | `PUT /projects/:id` `only_allow_merge_if_pipeline_succeeds=true` (§3) |
| merge-threads | example-group/example-project | all threads resolved before merge (B6, the merge-hold gate's server half) | public | `read project` | only_allow_merge_if_all_discussions_are_resolved | true | `PUT /projects/:id` `only_allow_merge_if_all_discussions_are_resolved=true` (§3) |
| ci-config-path | example-group/example-project | CI definition lives outside the writable project (C6) | public | `read project` | ci_config_path | .gitlab-ci.yml@example-group/ci-config | `PUT /projects/:id` `ci_config_path=.gitlab-ci.yml@<group>/<ci-config project>` (§4) |
| fork-pipelines | example-group/example-project | fork pipelines cannot run in the parent project (Owner-visible field) | admin | `read project` | ci_allow_fork_pipelines_to_run_in_parent_project | false | Owner: `PUT /projects/:id` `ci_allow_fork_pipelines_to_run_in_parent_project=false` |
| main-no-force | example-group/example-project | main: force push closed (B1) | public | `read protected-branches` | [name=main].allow_force_push | false | `POST /projects/:id/protected_branches` `name=main&allow_force_push=false` (§3) |
| main-push-no-one | example-group/example-project | main: Allowed to push = No one (B2 — role-level on CE) | public | `read protected-branches` | [name=main].push_access_levels.0.access_level | 0 | `push_access_level=0` on the same call |
| main-merge-maintainers | example-group/example-project | main: Allowed to merge = Maintainers | public | `read protected-branches` | [name=main].merge_access_levels.0.access_level | 40 | `merge_access_level=40` on the same call |
| release-tags | example-group/example-project | v* tags: Allowed to create = Maintainers (B12) | public | `read protected-tags` | [name=v*].create_access_levels.0.access_level | 40 | `POST /projects/:id/protected_tags` `name=v*&create_access_level=40` |
| approvals-reset | example-group/example-project | approvals reset when commits are added (advisory on CE — B3) | admin | `read approvals` | reset_approvals_on_push | true | `POST /projects/:id/approvals` `reset_approvals_on_push=true` |
| approvals-no-author | example-group/example-project | the author cannot approve (advisory on CE — B4) | admin | `read approvals` | merge_requests_author_approval | false | `POST /projects/:id/approvals` `merge_requests_author_approval=false` (§3) |
| approvals-no-committer | example-group/example-project | committers cannot approve (advisory on CE — B4) | admin | `read approvals` | merge_requests_disable_committers_approval | true | `POST /projects/:id/approvals` `merge_requests_disable_committers_approval=true` (§3) |
| push-rules-signed | example-group/example-project | push rules: reject unsigned commits (C5) | admin | `read push-rules` | reject_unsigned_commits | not available — Premium | n/a on Community Edition |
| push-rules-secrets | example-group/example-project | push rules: prevent secrets (C5) | admin | `read push-rules` | prevent_secrets | not available — Premium | n/a on Community Edition; the CI leak sweep (§4a) is the layer that does not depend on tier |
| secret-push-protection | example-group/example-project | secret push protection (C5) | admin | `read project` | secret_push_protection_enabled | not available — Ultimate | n/a below Ultimate |
| doc-security | example-group/example-project | SECURITY.md present | public | `read file SECURITY.md` | - | present | add the file |
| doc-contributing | example-group/example-project | CONTRIBUTING.md present | public | `read file CONTRIBUTING.md` | - | present | add the file |
<!-- repohardenguard:rows:end -->

**On Premium**, swap the two push-rules rows for real requirements — `reject_unsigned_commits`
Required `true` and `prevent_secrets` Required `true`, `Gated` still `admin` — and, if you
provisioned the board-writer as an identity-level push entry (§2, the B2 remediation), require
that entry rather than `access_level` `0`: the identity appears as a `user_id` field in
`[name=main].push_access_levels`, which Community Edition never renders. **Do not require a
`user_id` in the CE template** — it fails every CE project. **On Ultimate**, the
`secret-push-protection` row becomes `secret_push_protection_enabled` Required `true`.

The three `not available` rows above are the profile's existing disclosed degradations
(§0.1: B2's role-level allowlist and B3/B4's advisory approvals, plus C5's tier note) written
as rows the guard records on every run — they add no new degradation, and a run that shows
them is honest, not red.

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
