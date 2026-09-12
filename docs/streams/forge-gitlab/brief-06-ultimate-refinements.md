---
brief: assay:assay:forge-gitlab:06
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
schema: brief-v2
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (Ultimate rows of the parity table)"
  - "docs/streams/forge-gitlab/brief-05-live-pilot-parity-walk.md — pilot findings scope this brief"
exec-tier: any
domain: complicated
tier: ultimate
version: 1
id: 45c382c0-2232-4c6d-bb4c-435b98cebdc8
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
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestForgeGitlabStatusCheckFallback -v` | exit 0; Premium fixture yields could-not-check, Ultimate fixture posts the check |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->
### Verify run — 2026-09-11, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — VERDICT: PARTIAL/held at `implemented` (live row-2 proof pending)

Target: merged `origin/main` @ `fc9001a7ab48ee9c859dd7e52f7543dec5f86c50` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer. gate: model, risk all=no.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `bash tools/create-fleet-gitlab.sh --dry-run --tier ultimate --group example --prefix myorg 2>&1 \| grep -cE -e 'custom role' -e 'status check'` | 0 | count `5` (≥2): "would create custom role 'myorg-reviewer-role' (base=Reporter/20, admin_merge_request=true, no push) via POST /groups/:id/member_roles"; "would register external status check 'assay-verdict'"; "Custom reviewer role + external status check: CONFIGURED" | PASS |
| 2 | push attempt to a scratch branch as the custom-role reviewer token → rejected by GitLab (LIVE — not executed) | — | offline envelope forbids; needs a live Ultimate instance + reviewer token. brief `## Evidence` empty; the only related pilot probe (`pilot-report.md:166`, parity row 12) records the OPPOSITE (pilot not Ultimate → role never provisionable; a Developer SA pushed successfully — the B9 failed-at-tier degradation). NO well-formed Phase-0 record of the custom-role token being rejected | COULD-NOT-CHECK (live, no Phase-0 record) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabStatusCheckFallback -v` | 0 | PASS — `ultimate_posts_the_check` PASS; `premium_yields_could_not_check` PASS (Premium 403 → `could-not-check: GET /projects/.../external_status_checks — permission or tier gate (HTTP 403)`). Both branches | PASS |

**Risk-bearing value (ENUMERATE → RANK → DERIVE):**
- `RISK-VALUE: TOP-RANKED (security control, reviewer-role-cannot-push) — offline-derivable half: custom-role permission literals in tools/create-fleet-gitlab.sh — base_access_level: 20 (Reporter → cannot push to any branch, line 115/521) + admin_merge_request: true (approve without push, line 521), echoed in dry-run line 494. This is the role DEFINITION as not-push-capable; its irreversible/live PROOF is row 2 (GitLab-enforced push rejection), which is UNCONFIRMED — the offline surface shows the role is defined not-push-capable, not that GitLab enforces it.`
- `RISK-VALUE: DERIVED — tier-gated status-check behavior @ forge_gitlab.go: Premium (no external-status-check API) → 403 → could-not-check (carries the literal 'could-not-check', never a silent downgrade); Ultimate → posts the check.` Confirmed both branches (row 3). Reversible relative to the push restriction, ranks below it.

**Live row-2 hand-off (exact probe, for an Ultimate instance):** create the role (`POST /groups/:id/member_roles base_access_level:20 admin_merge_request:true`), bind it to the reviewer SA (`PUT /groups/:id/members/:user_id member_role_id`), mint that reviewer token, then `POST /projects/:id/repository/branches` (or `git push`) to a scratch branch as that token → expect HTTP 403 / rejection. Record endpoint + project + token role + HTTP status.

**Scope-traceability:** every offline-runnable row maps 1:1 to a deliverable; row 2 being a live probe is inherent to the brief (Ultimate-by-construction), not a gap in the work.

**VERDICT: PARTIAL/BLOCKED** — offline surface (rows 1 & 3) PASS; row 2 COULD-NOT-CHECK (live, no Phase-0 record). NOT a FAIL (no failing observation). **Held at `implemented`** — row 2 is the live proof of the brief's single-point-of-failure security control (reviewer-role-cannot-push), so the offline surface is not flipped alone. Decision filed `#838`: run the live push-rejection probe on an Ultimate instance (record as Phase-0 Evidence), OR a recorded ruling to accept the offline role-definition + row-3 tier-fallback with row 2 deferred (mirrors fg/05's live-pilot human-gate treatment).

### Human ruling — 2026-09-11 (relayed from the driver, Ian; `#838`)

**Answer: B — accept the offline surface, defer the live row-2 proof** ("I don't have an ultimate instance to test it on"). This human sign-off accepts rows 1 & 3 (offline, PASS) with row 2 (the live custom-reviewer-role-cannot-push proof) DEFERRED — recorded COULD-NOT-CHECK, not disproven. On that basis the row flips **implemented → verified**.

**Deferred live proof still owed** (not lost): the enforced "custom reviewer role cannot push" guarantee is proven only at role-DEFINITION level (offline: `base_access_level: 20` + `admin_merge_request:true` + no push), not live-enforced. When an Ultimate GitLab instance is available, run the push-rejection probe (provision the role `POST /groups/:id/member_roles base_access_level:20 admin_merge_request:true`, bind to the reviewer SA, mint that token, `POST …/repository/branches` → expect HTTP 403) and append the Phase-0 record. Tracked on `#838`.

**VERDICT (post-ruling): VERIFIED** — offline surface PASS + human sign-off (Ian, #838) accepting the deferred live row 2.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table.
