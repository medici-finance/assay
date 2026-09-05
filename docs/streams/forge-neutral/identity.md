# Forge-qualified identity — the grammar, the renderings, the corroboration rule

This is the ONE place the forge-qualified identity model is written down. Briefs 07, 08, 09
and 11 cite this file rather than restating any of it, so the grammar, the per-forge
rendering set, the per-forge commit address, and the corroboration rule cannot drift between
the desk verbs, the preflight, `statusgen`, and the install/adopt skills.

Authored for `forge-neutral/02`. Freshness-checked against
`docs/streams/forge-gitlab/pilot-report.md` (the 2026-09-02 live pilot) and
`docs/streams/forge-gitlab/edition-matrix.md` (row A7).

## The grammar

A trusted-bot roster entry (`ASSAY_TRUSTED_BOT_SLUGS`) now speaks:

```
[role=]<forge>:<slug-or-login>[:<numeric id>]
```

- `<forge>` is `github` or `gitlab`.
- An entry with **no** `<forge>` segment is read as `github` — the backward-compatibility
  rule, so every roster deployed before this brief keeps working unchanged on a GitHub repo.
- The inferred default is **recorded** (an inferred github is distinguishable from an explicit
  one): a caller can therefore tell a legacy entry from a deliberate `github:` one, which is
  what lets the forge-agreement rule below exempt the legacy case without exempting an
  explicit mismatch.
- The `<numeric id>`, when present, must be a positive number; a typo'd id fails the whole
  roster closed rather than degrading to login-only trust.

Examples:

```
reviewer=github:assay-reviewer-app:300000004     # explicit github
reviewer=gitlab:assay-reviewer-bot:41987965      # explicit gitlab
reviewer=assay-reviewer-app:300000004            # legacy — read as github, flagged INFERRED
```

## Per-forge renderings

The bare GitHub App slug is **never** an accepted login on any forge — App slugs and ordinary
usernames share no GitHub namespace, so a plain account named after a slug must not be able to
spoof a desk identity. Each forge accepts only its own genuine rendering(s), and the commit
author noreply address is derived from the entry's forge and never falls back to the other's.

| Forge | Accepted login rendering(s) | Commit author noreply address | Address is |
|-------|-----------------------------|-------------------------------|------------|
| github | `<slug>[bot]` (REST `user.login`) and `app/<slug>` (`gh --json author`) | `<bot-user-id>+<slug>[bot]@users.noreply.github.com` | **derivable** — built from slug + bot USER id (the id is the bot USER id, not the App id — a commit under the App id lands account-UNLINKED, `#638`) |
| gitlab | the service account's username, verbatim | `service_account_group_<group-id>_<per-account-suffix>@noreply.<host>` | **validated by shape only** — see below |

### Why the GitLab commit address is validated, not constructed

A GitLab service account commits under
`service_account_group_<group-id>_<per-account-suffix>@noreply.<host>` (live-read on the pilot,
`pilot-report.md` §0 and §3 row 13: the group is id `9619193`, each of the seven role accounts
has a distinct `service_account_group_9619193_*@noreply.gitlab.com` commit email).

Neither the **group id** nor the **per-account suffix** is derivable from a roster entry: the
entry carries the account's username and its numeric **user** id (`41987965…`), and the group
id (`9619193`) is a different number from every user id. So a check must **not** hard-code an
underivable value. The preflight therefore matches the GitLab address by **shape** — it is the
tightest available check — and a tool that must *stamp* a commit identity (`deskwt`) **refuses**
a GitLab entry rather than inventing the address or falling back to the GitHub shape. The exact
service-account address is a provisioning fact, supplied by the forge-gitlab custody path, not
by the roster.

The shape check is still a real gate: a GitHub noreply address presented for a `gitlab:` entry
**fails** (it does not match the service-account shape), and a GitLab address presented for a
`github:` entry **fails** (it is not the exact GitHub string). Neither forge accepts the other's
address.

## Forge agreement is enforced, not assumed

An entry whose forge does not match the forge that `ForgeFor` resolves for the repo being acted
on is **refused** (exit 5) naming both forges, before any credential is read — silently ignoring
a mismatched entry is exactly how a same-named account on the wrong forge would become trusted.

The one exemption is the backward-compatibility case: a **legacy unqualified** (inferred-github)
entry is not refused, because breaking every deployed roster is not an acceptable way to add a
field. An **explicit** `github:`/`gitlab:` mismatch is always refused.

## Corroboration — what counts as a review verdict at head, per forge

No code in `forge-neutral/02` consumes this rule; briefs 07 and 08 do. It is recorded here so
they cite one definition.

| Forge | A verdict is "at head" when… | What carries the at-head property | Reset-on-push |
|-------|------------------------------|-----------------------------------|---------------|
| github | a review by an accepted reviewer identity whose `commit_id` equals the PR head SHA | the review's own `commit_id` | server resets stale reviews implicitly via `commit_id` |
| gitlab (CE) | an **approval** by an accepted reviewer identity **plus a note by that identity pinning the head SHA** | the note body's pinned SHA — **not** the approval flag | **none** on CE (`reset_approvals_on_push` is Premium and read `false` on the pilot) |

The GitLab form is **weaker** where approvals do not reset on push. On GitHub the forge itself
ties the verdict to a commit; on GitLab Community Edition the approval flag persists across a
new push (edition-matrix row A7; `pilot-report.md` steps A9 and B4), so the approval flag alone
is **not** proof of an at-head verdict. The desk writes the head SHA into the verdict **note**
body, and that pinned SHA — read back and compared to the current head — is what carries the
at-head property on CE. A reader that trusted the approval flag on CE would treat a verdict
recorded against an older head as current; the note pin is what closes that gap.
