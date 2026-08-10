# Main-commit actor routing

Deliverable of [brief-06 — desk App cutover](./brief-06-desk-app-cutover.md). Enumerated 2026-07-24
from `git log origin/main --format='%ae %an' | sort | uniq -c | sort -rn`. Every current `main`
committer is enumerated below; none unlisted (class-sweep rule). 8 unique name/email pairs, matching
`git log origin/main --format='%ae %an' | sort -u | wc -l` = 8.

Each current actor routes to exactly one of:
`desk-app` / `verifier-app` (desk-apps/04) / `worker-app via PR` (desk-apps/05) /
`stays-the-org` (reason) / `stays-CI` ([skip-status-regen], Flux) / `human:<name>`.

## Routing table

| Current actor | Email | Count | Route | Notes |
|---|---|---|---|---|
| shared-agent | `none@example.com` | 1471 | **desk-app** | the-org identity (unverified-email variant). Coordinator doc/brief-row/k8s/maintenance commits. Future commits: `assay-desk-app[bot]`. |
| shared-agent | `the-org@users.noreply.github.com` | 1024 | **desk-app** | Same the-org identity (GitHub-verified email variant). Merged into desk-app per row above. |
| Kryton | `20959+human:<name>@users.noreply.github.com` | 433 | **human:<name>** | human:<name>'s personal account. Merge commits, PR approvals, human decisions. The human gate. |
| github-actions[bot] | `41898282+github-actions[bot]@users.noreply.github.com` | 413 | **stays-CI** | status-regen (`[skip-status-regen]`), harvest, CI workflows. Automated, deterministic generators. |
| Router Engine | `the-org@users.noreply.github.com` | 307 | **desk-app** | Same the-org user ID, different git `user.name`. Merged into desk-app. |
| assay-worker-app[bot] | `306480234+assay-worker-app[bot]@users.noreply.github.com` | 23 | **worker-app via PR** | Already the correct App identity. Workers author PRs/commits as worker-app (brief 05). |
| assay-verifier-app[bot] | `4331323+assay-verifier-app[bot]@users.noreply.github.com` | 20 | **verifier-app** | Already the correct App identity. Un-forgeable Evidence authorship (brief 04). |
| dependabot[bot] | `49699333+dependabot[bot]@users.noreply.github.com` | 8 | **stays-CI** | Automated dependency version bumps. Not an agent-actor loop; stays as-is. |

## Routing summary

| Destination | Actors | Count |
|---|---|---|
| **desk-app** (`assay-desk-app[bot]`) | shared-agent (3 email/name variants) | 1 distinct identity (the-org) |
| **human:<name>** | Kryton | 1 |
| **stays-CI** | github-actions[bot], dependabot[bot] | 2 |
| **worker-app via PR** | assay-worker-app[bot] | 1 |
| **verifier-app** | assay-verifier-app[bot] | 1 |
| **stays-the-org** | (none) | 0 |

**6 distinct identities across 8 name/email pairs.** The the-org identity (3 variants) is the only
one transitioning — to desk-app. All other actors already route to their correct destination or stay
unchanged.

## Desk-app commit identity

For coordinator main-commits (the desk-app destination), the git identity to use:

```bash
export GIT_AUTHOR_NAME="assay-desk-app[bot]"
export GIT_AUTHOR_EMAIL="4331346+assay-desk-app[bot]@users.noreply.github.com"
export GIT_COMMITTER_NAME="assay-desk-app[bot]"
export GIT_COMMITTER_EMAIL="4331346+assay-desk-app[bot]@users.noreply.github.com"
```

Push authentication via `desktoken desk` (App installation token):

```bash
export GH_TOKEN="$(cat "$(desktoken desk --repo oit)")"
```

See `../oit/.claude/skills/the-desk/SKILL.md` for the full coordinator commit flow.
