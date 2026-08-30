---
brief: forge-gitlab/04
title: Fleet provisioning script + adopter doc + ci-config-project runbook
why: >-
  GitLab has no App-manifest flow, so the eight-click GitHub onboarding becomes API
  calls — which is an opportunity: one idempotent script can create the service
  accounts, memberships, tokens, protected-branch and approval-rule settings, and print
  the human-only remaining acts. Without it the profile exists only for experts; with
  it a GitLab group boots the fleet in minutes, which is the adoption story.
wave: 3
depends: ["forge-gitlab/02", "forge-gitlab/03"]
unblocks: ["forge-gitlab/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §2 (identity model), §4 (ci-config project), §5 (custody)"
  - "docs/adopting-assay.md — the GitHub-side adopter doc this parallels"
  - "freshness-checked 2026-08-24 @ 5c4a67d — no gitlab provisioning or adopter doc exists"
exec-tier: any
domain: complicated
tier: free
consumers:
  - "docs/adopting-assay.md: fixed-here (one cross-link line to the GitLab doc)"
---

# Brief 04 — provisioning + adopter doc + ci-config runbook

## Context
files:
- `tools/create-fleet-gitlab.sh` (planned) — idempotent provisioning script.
- `docs/adopting-assay-gitlab.md` (planned) — the adopter walkthrough.
- `docs/adopting-assay.md` — add one cross-link line.

single-point-of-failure: the script's idempotency check is the one control against
double-provisioning — backed by GitLab's own uniqueness errors (username collision), so
a re-run degrades to named no-ops, never duplicates.

facts:
- Script creates per role (spec §2): service account, group membership at the role's
  access level, PAT with the role's scopes; then configures protected `main`
  (allowed-to-push = board-writer only, allowed-to-merge = maintainer humans),
  approval rules (prevent-author, prevent-committers), and prints the HUMAN-ONLY
  remainder: Ultimate settings (custom roles, external status checks, pipeline
  execution policy), group token-expiry policy, ci-config project creation.
- ci-config runbook section: create the locked project, set consumer projects'
  CI config path to it, verify no bot membership.
- The doc carries the tier ladder and, verbatim from spec §1, the CE statement: Community
  Edition is conforming for the core lane with two named, disclosed degradations
  (identity-granular protected branches — the Maintainer role set is the allowlist; enforced
  approval rules — approvals are advisory, so verdict-before-merge is human-merge-only plus
  the desk's refusal to flip without an at-head verdict). No softening in either direction:
  the degradations are stated as degradations.
- All REST; requires a group-owner PAT supplied by the operator at run time (never
  stored by the script).

re-baselined: 2026-08-30 — Verify row 3 previously grepped the adopter doc for spec §1's
"Free/CE non-conforming" sentence. That sentence was replaced by the tier ruling of
2026-08-30 (medici-finance/assay#219, evidence in edition-matrix.md): CE conforms for the
core lane with the two disclosed degradations, Premium is the hardening, Ultimate is
refinement. Row 3 now greps the replacement sentence and both named degradations. No other
Verify row changed, and no task changed.

## Edition
Minimum GitLab tier: **free** (Community Edition) for everything the script must do. Service
accounts are `Tier: Free, Premium, Ultimate` from GitLab 18.11 ("Generally available in GitLab
18.11. Feature flag removed."), below which ordinary bot users are the fallback; group,
project and member provisioning, protected branches at role level, protected tags at role
level, "pipelines must succeed", "all threads resolved", webhooks, and the external CI config
path (the locked ci-config project — the one place this profile is *stronger* than the GitHub
controls) are all Free. Citations per row in edition-matrix.md tables B and C.

Two credential facts the script must respect on CE and everywhere else: on GitLab
Self-Managed "only administrators can create either type of service account" by default, so
service-account creation needs an admin credential rather than the group-owner PAT the rest of
the script uses; and service accounts "do not use a seat"
(https://docs.gitlab.com/user/profile/service_accounts/).

What degrades on CE — the script prints these as human-only, tier-gated items rather than
configuring them:

- **Single board-writer on `main`.** `allowed_to_push`/`allowed_to_merge` take a `user_id` or
  `group_id` only on Premium ("`user_id`, `group_id`, and `access_level` are Premium and
  Ultimate only", https://docs.gitlab.com/api/protected_branches/). CE fallback: `Allowed to
  push and merge` = **No one**, every write including board regeneration lands as an MR, and
  the allowlist becomes Maintainer membership — role granularity, not identity granularity.
- **Required approvals** and **prevent approval by author/committer**: both Premium
  (https://docs.gitlab.com/user/project/merge_requests/approvals/rules/ ,
  https://docs.gitlab.com/user/project/merge_requests/approvals/settings/). CE fallback:
  humans-only merge plus the desk's own refusal to flip ready without an at-head verdict; the
  bot case is already structural because a worker holds no reviewer credential.
- **Push rules** (Premium) and **secret push protection** (Ultimate): the house leak sweep in
  CI is the tier-independent layer.
- **Group/project audit events** (Premium): only sign-in events exist at Free.

Settled 2026-08-30 (medici-finance/assay#219): the open point this brief carried — whether the
adopter doc must repeat a "Free/CE non-conforming" statement the matrix's per-feature tier
reads did not support — is ruled. CE is conforming for the core lane with the two disclosed
degradations above; Premium is the hardening that makes them server-enforced; Ultimate is
refinement. Task 2 and Verify row 3 now carry the amended spec section 1 sentence, and the
degradations above are the doc's tier-honesty statement.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per
  the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `tools/create-fleet-gitlab.sh` (planned) (bash + curl + jq; same dependency posture as
   the existing exchange script) — idempotent, dry-run flag, plain-text summary ending
   with the human-only checklist.
2. Write `docs/adopting-assay-gitlab.md` (planned): prerequisites/tier ladder, script usage,
   by-hand table (mirroring the script), ci-config-project runbook, custody rules,
   parity statement pointing at the stream spec.
3. Cross-link from `docs/adopting-assay.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `bash -n tools/create-fleet-gitlab.sh && shellcheck tools/create-fleet-gitlab.sh` | exit 0 |
| 2 | `bash tools/create-fleet-gitlab.sh --dry-run --group example --prefix myorg 2>&1 \| grep -c 'service account'` | ≥ 7 — dry-run enumerates every role it would create |
| 3 | `grep -ci 'Community Edition is conforming for the core lane' docs/adopting-assay-gitlab.md && grep -ci 'Maintainer role set is the allowlist' docs/adopting-assay-gitlab.md && grep -ci 'approvals are advisory' docs/adopting-assay-gitlab.md` | each ≥ 1, chain exits 0 — the doc carries the amended spec §1 statement: CE conforms for the core lane, plus both named degradations verbatim |
| 4 | For each REST endpoint named in the script: `grep -oE 'api/v4/[a-z_/:{}.-]+' tools/create-fleet-gitlab.sh \| sort -u` checked against the GitLab REST v4 docs | every endpoint exists in current docs (dereference: reviewer resolves each against docs.gitlab.com and records the doc URL per endpoint) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table.
