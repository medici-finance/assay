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
- The doc carries the tier ladder and the non-conforming Free/CE statement verbatim
  from spec §1 — no softening.
- All REST; requires a group-owner PAT supplied by the operator at run time (never
  stored by the script).

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
| 3 | `grep -n 'Free' docs/adopting-assay-gitlab.md \| grep -ci 'non-conforming'` | ≥ 1 — the tier honesty statement is present |
| 4 | For each REST endpoint named in the script: `grep -oE 'api/v4/[a-z_/:{}.-]+' tools/create-fleet-gitlab.sh \| sort -u` checked against the GitLab REST v4 docs | every endpoint exists in current docs (dereference: reviewer resolves each against docs.gitlab.com and records the doc URL per endpoint) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table.
