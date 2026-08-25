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
