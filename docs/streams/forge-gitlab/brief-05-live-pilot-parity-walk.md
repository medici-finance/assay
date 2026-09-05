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
tier: free
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

## Edition
Minimum GitLab tier: **free** (Community Edition) to run the pilot. The whole round trip the
pilot drives — worker `Draft:` MR, reviewer approval, human merge, verifier Evidence commit,
board regeneration by the single-writer identity, rotate-on-mint proved live — uses Free-tier
operations only (edition-matrix.md table A). A CE pilot is therefore the cheapest way to prove
the tooling, and it no longer needs a paid trial clock.

What degrades on CE, and what the pilot must record rather than wave through: the parity walk
rows that are tier-gated — single board-writer on `main` (Premium), required approvals and
prevent-author approval (Premium), audit events (Premium), external status checks and custom
roles (Ultimate). On a CE pilot each of those is recorded as failed-at-tier with the named
remediation, exactly as the brief already requires; that is not a pilot failure, it is the
pilot producing the evidence the ruling in edition-matrix.md's "Residual gaps" needs.

Consequence for the Human decision below: option 1's paid-trial framing is no longer forced. A
CE or unlicensed-EE instance is a legitimate fourth option that proves the tooling lane and
leaves the two Premium parity rows as recorded gaps.

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

**NON-IMPLEMENTER VERIFY 2026-09-02 — gate:human, sensitive-data:yes → status stays `implemented`; a model cannot sign a live-pilot security-parity table.** Runner: opus-4.8[1m]-verifier, offline (`KUBECONFIG=/dev/null`), public-assay merged HEAD `ecf722d068e0fa3c6273eff68931ba6c1fb96e84`. Deliverable present: `docs/streams/forge-gitlab/pilot-report.md`.

| # | Command | Exit | Key observed output |
|---|---------|------|---------------------|
| 1 | `grep -c '^[\|]' docs/streams/forge-gitlab/pilot-report.md` | 0 | `68` (≥12) — cross-checks the report's own §4 row-1 claim of 68 exactly. PASS (offline-runnable) |
| 2 | `glab api projects/:id/merge_requests/:iid/approvals` | — | could-not-check (offline envelope; `glab` not installed; live gitlab.com). Well-formed implementer Phase-0 record EXISTS: report §4 row 2 / §1 — MRs iid 1..4 each `approved:true`, `approved_by[0].user.id=41987965` (reviewer SA) vs authors 41987971/66/69/78 — author ≠ approver on all four. |
| 3 | mint token twice via `desktoken --forge gitlab worker`; curl old token vs `/user` | — | could-not-check (offline; minting hits live forge). Phase-0 record EXISTS: report §4 row 3 / §3 — first token `200` pre-mint, `401 "Token was revoked."` post-mint; replacement `expires_at 2026-09-09` (7d). |
| 4 | `git log --format='%an' -1 -- STATUS.md` in the pilot repo | — | could-not-check (offline; live pilot project id 86032201 not in this worktree). Phase-0 record EXISTS: report §4 row 4 — fresh clone returns board-writer SA (41987978) only, commit `bfd01ac`, landed via MR `!4` merged by the human owner; no other identity touched STATUS.md. |

Phase-0 attestation: for the three live rows the implementer's records are well-formed conformance records (endpoint + numeric ids + HTTP status + timestamps + commit SHAs), internally consistent — I attest they EXIST and are well-formed; I did not (could not, offline) re-execute them.

RISK-VALUE: DERIVED — `merge_access_level = 40` ("Maintainers") @ `docs/streams/forge-gitlab/pilot-report.md:164` — the brief's single-point-of-failure control ("merge is always the human's"); only the human owner is a member at ≥40, so 40 ⇒ humans-only merge; the report proves `can_merge:false` for all five Developer(30) bots. `push_access_level = 0` ("No one") @ :165 — floor under rows 2/10, correct for no-direct-push. Token TTL 7d ranks last (reversible). `approvals_required:0` is an OBSERVED CE limitation recorded failed-at-tier, not a value the brief pins.

Verdict: BLOCKED (offline) — 1/4 rows offline-runnable and it PASSes; rows 2/3/4 could-not-check, each backed by an attested well-formed Phase-0 record. No FAIL observed. **Status stays `implemented` — the human gate (Ian) confirms the group, authorizes credentials, and signs the §3 parity verdict; the report self-records 8 failed-at-tier rows + a mid-run D-6 hand-repair as the substance to weigh (pilot report on #353).**

### Non-implementer verifier run — VERIFY: BLOCKED (offline; row 1 PASS, rows 2-4 could-not-check w/ Phase-0 records) — HELD at implemented (gate:human, sensitive-data:yes) — 2026-09-05 opus-4.8[1m]-verifier (verify-desk dispatch), medici-finance/assay merged main 203dac5

Runner != implementer. Offline envelope (KUBECONFIG=/dev/null). gate: human; risk {regulatory:no, customer:no, irreversible:no, sensitive-data:yes}; gate-why = the deliverable is a security-parity verdict signing the walked parity table — a security judgment no model self-certifies. A live/externally-authenticated probe brief: the live rows are Phase-0 implementer records; the non-implementer confirms they EXIST and are well-formed, never re-runs a live/billed probe.

| # | command | expected | observed (exit + key line) | date · runner |
|---|---------|----------|----------------------------|---------------|
| 1 | grep -c table rows in docs/streams/forge-gitlab/pilot-report.md | >= 12 | exit 0 — 68 (>= 12); matches the report section-4 self-claim of 68. PASS (offline-runnable) | 2026-09-05 · opus-4.8[1m]-verifier |
| 2 | glab api merge_requests approvals (author != approver) | approval by reviewer SA, author != approver | could-not-check — glab absent + live gitlab.com (offline). Phase-0 record present + well-formed: MR iid 1..4 each approved:true, approved_by user id 41987965 (reviewer SA) vs distinct author ids; author != approver on all four | 2026-09-05 · opus-4.8[1m]-verifier |
| 3 | desktoken --forge gitlab worker (mint x2); curl old token vs /user | 401 rotated-out | could-not-check — desktoken present but mint hits the live forge (offline). Phase-0 record present + well-formed: first token 200 pre-mint, 401 "Token was revoked" post-mint; replacement expires_at 2026-09-09 (7d) | 2026-09-05 · opus-4.8[1m]-verifier |
| 4 | git log author of STATUS.md in the pilot project | board-writer SA only | could-not-check — pilot project 86032201 not in this worktree (offline). Phase-0 record present + well-formed: pilot fresh clone returns board-writer SA 41987978 only, commit bfd01ac, landed via MR !4 merged by the human owner | 2026-09-05 · opus-4.8[1m]-verifier |

**VERIFY: BLOCKED (offline) — HELD at implemented.** Row 1 (offline parity-table presence) PASSES; rows 2/3/4 are live/externally-authenticated probes the offline desk cannot re-run, each backed by an attested well-formed Phase-0 record (endpoint + numeric ids + HTTP status + timestamps + commit SHAs, internally consistent). No FAIL. gate:human + sensitive-data:yes and a risk-bearing live probe ⇒ a model does not sign the live-pilot security-parity verdict; the human gate (owner) confirms the group, authorizes credentials, and signs the parity table (which self-records 8 failed-at-tier rows, all Premium/Ultimate-gated with named remediation, plus the mid-run merge-access hand-repair). Corroborates the prior 2026-09-02 run (report commits are ancestors of 203dac5; literals unchanged).

RISK-VALUE: DERIVED — merge_access_level = 40 (Maintainers) @ docs/streams/forge-gitlab/pilot-report.md:164 — the single-point-of-failure control "merge is always the human's": only the human owner is a member at >=40, so 40 ⇒ humans-only merge; the report proves can_merge:false for all five Developer(30) bots and true only for the owner.
RISK-VALUE: DERIVED — push_access_level = 0 (No one) @ docs/streams/forge-gitlab/pilot-report.md:164 — the floor under the no-direct-push parity rows; every write travels via MR. Correct for the CE posture the spec requires. (Token TTL 7d ranks last — reversible knob; approvals_required:0 is an observed CE limitation recorded failed-at-tier, not a brief-pinned value.)

## Review
Gate: human (from frontmatter) — the human signs the parity verdict; the reviewer
additionally records verdict + date in the stream README table.
