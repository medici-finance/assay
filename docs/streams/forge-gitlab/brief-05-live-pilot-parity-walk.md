---
brief: forge-gitlab/05
title: Live pilot — one brief round-tripped on a real GitLab group, security-parity table walked
why: >-
  Nothing may claim GitLab support from fixtures: the profile's promise is per-control
  security parity verified on a live deployment. The pilot boots the fleet on a real
  Premium/Ultimate group, drives one brief todo→done end-to-end through the GitLab
  identities and gates, and walks the spec's parity table against the group's live
  settings — the conformance gate for every published claim downstream.
wave: 4
depends: ["forge-gitlab/04"]
unblocks: ["forge-gitlab/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The pilot requires a real GitLab group, a group-owner credential, and live service
  account tokens (sensitive-data: yes), and its deliverable is a security-parity
  verdict — the human confirms the group choice, provides/authorizes the credentials,
  and signs the walked parity table, which is a security judgment no model
  self-certifies.
decision-trigger: start
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (the parity table), §7 (conformance gate)"
  - "docs/streams/forge-gitlab/brief-04-provisioning-and-adopter-doc.md — the script and runbook this executes"
exec-tier: strong
exec-tier-why: "end-to-end cross-component verification on a live system (question b); parity judgments require reasoning beyond the runbook."
domain: complex
---

# Brief 05 — live pilot + parity walk

## Context
files:
- `docs/streams/forge-gitlab/pilot-report.md` (planned) — the walked parity table with
  per-control evidence (settings screenshots/API reads, MR/approval/board artifacts).
- `docs/streams/forge-gitlab/README.md` — status updates.

single-point-of-failure: the pilot group's fidelity to the runbook is the one control —
backed by the parity walk reading LIVE settings via API (not the runbook's claims), so
a mis-provisioned group fails the walk rather than passing on paper.

facts:
- Pilot scope: boot the fleet via brief-04's script on the human-chosen group; create a
  minimal tracking root; drive ONE real brief todo→done: worker MR (Draft:), reviewer
  approval, human merge, verifier Evidence commit, board regeneration by the
  single-writer identity.
- Parity walk: every row of spec §3 checked against live group/project settings via
  API reads recorded in the report; any row that cannot be satisfied at the group's
  tier is recorded as failed-at-tier with the Ultimate remediation named — never
  waved through.
- Rotate-on-mint verified live: mint twice, prove the first token is rejected.

## Human decision
The GitLab pilot needs a real group and an owner credential. Decide which to use.

Options:
1. **A fresh dedicated group on gitlab.com (paid tier trial or subscription)** — clean
   room, no blast radius, costs a subscription or uses a 30-day Ultimate trial; the
   trial clock bounds when the pilot must run.
2. **A self-managed EE instance we control** — closest to the enterprise adopter
   reality, more setup, needs the instance admin to set token-lifetime policy.
3. **A design partner's group** — realest signal, but schedules the pilot on someone
   else's calendar and their settings may not be ours to change.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands outside the pilot
  group. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Provision per brief-04 on the chosen group; record every deviation the script could
   not automate.
2. Drive one brief todo→done through the full role chain on GitLab.
3. Walk the spec §3 parity table against live settings; write pilot-report.md with
   per-row evidence and an explicit overall verdict.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c '^[\|]' docs/streams/forge-gitlab/pilot-report.md` | ≥ 12 — one walked table row per spec §3 control, plus header |
| 2 | `glab api "projects/:id/merge_requests/:iid/approvals"` (ids from the report) | approval by the reviewer service account, author ≠ approver (dereference: live system confirms the report's claim) |
| 3 | mint token twice via `desktoken --forge gitlab worker`; `curl -s -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: <first>" <api>/user` | `401` — rotated-out token rejected live |
| 4 | `git log --format='%an' -1 -- STATUS.md` in the pilot tracking repo | the board-writer service account, no other identity |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: human (from frontmatter) — the human signs the parity verdict; the reviewer
additionally records verdict + date in the stream README table.
