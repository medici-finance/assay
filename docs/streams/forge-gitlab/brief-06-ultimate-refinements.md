---
brief: forge-gitlab/06
title: Ultimate refinements — custom reviewer role + external-status-check verdict lane
why: >-
  Premium parity leans on protected branches and token scopes; Ultimate can do
  structurally better: a custom reviewer role that cannot push restores the per-resource
  granularity GitHub Apps have, and external status checks give verdict lanes a
  required-check surface with zero repo write access. These are the two refinements the
  parity table names for risk-classed work — landing them turns "Ultimate required" from
  a floor statement into shipped configuration.
wave: 5
depends: ["forge-gitlab/05"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (Ultimate rows of the parity table)"
  - "docs/streams/forge-gitlab/brief-05-live-pilot-parity-walk.md — pilot findings scope this brief"
exec-tier: any
domain: complicated
tier: ultimate
---

# Brief 06 — Ultimate refinements

## Context
files:
- `tools/create-fleet-gitlab.sh` (planned) — created by forge-gitlab/04; add the Ultimate configuration section (custom role
  creation, external status check registration) behind a `--tier ultimate` flag.
- `docs/adopting-assay-gitlab.md` (planned) — created by forge-gitlab/04; Ultimate section moves from "human-only checklist"
  to scripted-with-verification.
- `tools/desk/internal/deskkit/forge_gitlab.go` (planned) — created by forge-gitlab/02; post external status check as the
  verdict-lane surface where the tier supports it; three-state fallback at Premium.

single-point-of-failure: tier detection is the one control routing verdict posting —
backed by the API's own 403 on the Ultimate endpoint, surfaced as could-not-check
(never silently downgraded to a note).

facts:
- Custom reviewer role: member role with MR read/approve permissions and no
  push — created via the member-roles API, assigned to the reviewer service account.
- External status checks: registered per project; the lane service posts pass/fail
  against the MR head SHA; merge requires the check per project settings.
- Scope is bounded by pilot findings: anything brief-05's report marked
  failed-at-tier with an Ultimate remediation is in; new capabilities are not.
- **2026-08-30 — paid-tier hardening consolidated here (medici-finance/assay#219).** The
  edition matrix (edition-matrix.md) established, per docs citation, that every core-lane
  operation is Free-tier and that only *guarantees* are tier-gated. The tier-gated guarantees
  the core lane was implicitly assuming now live in this brief's territory, so 01-05, 07 and
  08 stay CE-clean with the named fallbacks rather than carrying a licence prerequisite:
  identity-granular protected-branch allowlists (Premium — matrix row B2; core-lane fallback:
  `Allowed to push and merge` = No one, all writes via MR), required approvals and
  prevent-approval-by-author (Premium — rows B3/B4; fallback: humans-only merge plus the
  desk's at-head verdict refusal), group and project audit events (Premium — row C4;
  fallback: sign-in events plus the desk's own records), push rules and secret push protection
  (Premium/Ultimate — row C5; fallback: the house leak sweep in CI), and pipeline execution
  policy (Ultimate — row C7; fallback: the locked ci-config project, itself Free). The two
  refinements this brief was authored around — external status checks (row B8) and custom
  roles (row B9) — are unchanged and remain Ultimate. The Task and Verify tables are as
  authored; the Premium rows above are recorded scope. **Ruled the same day:** they are
  optional hardening, not required for the parity claim — spec §1 makes CE conforming for the
  core lane with the B2 and B3/B4 degradations disclosed, so Premium is what converts those two
  into server-enforced controls and Ultimate stays refinement.

## Edition
Minimum GitLab tier: **ultimate**. This is the paid-tier brief by construction — external
status checks are `Tier: Ultimate`
(https://docs.gitlab.com/user/project/merge_requests/status_checks/) and custom roles are
`Tier: Ultimate` (https://docs.gitlab.com/user/custom_roles/), and the note above adds the
Premium hardening rows. Nothing here is a prerequisite for the core lane: on CE every item
degrades to the fallback named in its matrix row, and the tier detection this brief already
makes its single point of failure is what keeps the degradation honest — a 403 on an Ultimate
endpoint surfaces as could-not-check, never as a silent downgrade.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per
  the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Script the custom reviewer role + assignment; verify the role cannot push (negative
   test against a scratch project).
2. Script external-status-check registration; extend the gitlab forge impl to post
   lane verdicts through it at Ultimate, falling back three-state at Premium.
3. Update the adopter doc's Ultimate section with the scripted path + verification
   commands.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `bash tools/create-fleet-gitlab.sh --dry-run --tier ultimate --group example --prefix myorg 2>&1 \| grep -cE -e 'custom role' -e 'status check'` | ≥ 2 |
| 2 | push attempt to a scratch project branch as the custom-role reviewer token | rejected by GitLab (dereference: live negative-path proof the role cannot push) |
| 3 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabStatusCheckFallback -v` | exit 0; Premium fixture yields could-not-check, Ultimate fixture posts the check |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table.
