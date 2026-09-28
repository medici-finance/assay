---
brief: assay:assay:forge-gitlab:05
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
schema: brief-v2
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (the parity table), §7 (conformance gate)"
  - "docs/streams/forge-gitlab/brief-04-provisioning-and-adopter-doc.md — the script and runbook this executes"
exec-tier: strong
exec-tier-why: "end-to-end cross-component verification on a live system (question b); parity judgments require reasoning beyond the runbook."
domain: complex
tier: free
version: 1
id: 2a213e5a-0741-4ef0-b89b-eab26b33ede6
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c '^[\|]' docs/streams/forge-gitlab/pilot-report.md` | pass exit=0 | sha256:13c1dc569ae4 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `glab api "projects/:id/merge_requests/:iid/approvals"` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:8bc771b87f87 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `desktoken --forge gitlab worker` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:44619f9310aa | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git log --format='%an' -1 -- STATUS.md` | pass exit=0 | sha256:31ee4deda595 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |

**Non-implementer verifier re-run 2026-09-27 — opus-5.5 verifier (verify-desk dispatch), merged main e70bc86474f94b6e241d12857a10dcbd8136d556 — gate:human, sensitive-data:yes: Evidence only, status stays implemented.** Offline envelope (KUBECONFIG=/dev/null). The witness above was produced by the pinned statusgen v1.0.27 binary (sha256 matches the release pin) inside an OS network-deny sandbox with a PATH carrying only the base system tools, so no row could reach a live forge or rotate a live credential. Implementing change: PR #353 (merged 2026-09-02), report commits through 631aca15e; the report's section 7 appendix was added later by forge-gitlab/16 and is not this brief's deliverable.

Per-row notes (what each witness row actually proves):
- Row 1 — PASS, genuine: `grep -c` returns 92 (>= 12). The count grew from 68 because the section 7 appendix (forge-gitlab/16) added tables, so the whole-file count is a loose instrument. Grounded check: spec section 3 lists 10 controls; the report's section 3 walk table carries all 10 plus rows 7b, 11, 12, 13 (16 table lines with header and separator), so the table-per-control property holds on its own.
- Row 2 — could-not-check (live gitlab.com MR approvals read; needs a live GitLab credential). The witness exit 127 is the envelope withholding the forge CLI, not a missing tool. Exact probe, not run: `glab api "projects/<pilot-project-id>/merge_requests/<iid>/approvals"` for iid 1..4, ids from report section 4 row 2. Phase-0 record present and well-formed (report section 4 row 2: all four MRs approved by the reviewer service account, author differs from approver on each).
- Row 3 — could-not-check (mint rotates a live GitLab PAT; offline envelope forbids it). Exit 127 = envelope, as above. Exact probe, not run: `desktoken --forge gitlab worker` twice, then `curl -s -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: <first>" https://gitlab.com/api/v4/user`, expect 401. Phase-0 record present (report section 4 row 3: 200 before, 401 "Token was revoked" after; replacement expires 7 days out).
- Row 4 — witness pass is VACUOUS: verifyrun runs rows at this repo's root, so the command read this repo's STATUS.md (author: this repo's board-writer App), not the pilot tracking project's. could-not-check against the pilot project, which is private and not present here. Exact probe, not run: `git clone <pilot-tracking-project> && git -C <clone> log --format='%an' -1 -- STATUS.md`. Phase-0 record present (report section 4 row 4: board-writer service account only, landed via a human-merged MR).
- Stale-shaped, not defects: row 1's whole-file grep is inflated by a later brief's appendix; row 4's command has no repo selector, so any root-run witness is vacuous; row 3's command cell holds a multi-step recipe, so the witness can only lift its first code span.

Risk-bearing value enumeration (diff scope: pilot-report.md sections 0-6 as landed by PR #353, plus the brief's own facts and Verify expectations). Every value is an observed live setting recorded in the report, not a code constant:
- merge_access_level = 40 (Maintainers) @ docs/streams/forge-gitlab/pilot-report.md:25 (post-repair; the pre-repair read of 30 is @ :24)
- push_access_level = 0 (No one) @ docs/streams/forge-gitlab/pilot-report.md:25
- allow_force_push = false, unprotect_access_levels = 40 @ docs/streams/forge-gitlab/pilot-report.md:25
- service-account access_level = 30 (Developer, five bots) and 20 (Reporter, two bots); owner = 50 @ docs/streams/forge-gitlab/pilot-report.md:22-23
- PAT scopes = ["api"] (desk, reviewer, issue-loop, intake-loop) and ["api","write_repository"] (worker, verifier, board-writer) @ docs/streams/forge-gitlab/pilot-report.md:154
- rotated-token expires_at = 7 days (2026-09-09 from 2026-09-02) @ docs/streams/forge-gitlab/pilot-report.md:160; group max PAT lifetime = null @ :161
- approvals_required = 0, merge_requests_author_approval = false, pipelines-must-succeed = false @ docs/streams/forge-gitlab/pilot-report.md:156-157 (observed CE state, recorded failed-at-tier / provisioning gap)
- Verify expectations: >= 12 table lines (row 1), HTTP 401 on the rotated-out token (row 3)

Ranking by irreversibility: (1) merge_access_level — if wrong, any bot merges its own approved MR and every CE fallback collapses (report D-6); undoable by edit, but a merge that already landed is not. (2) PAT scope ["api"] — a leaked token reaches every project the account can see, at its role; a push or exfiltration is not undone by rotation. (3) push_access_level = 0 — floor under direct-push closure. (4) bot access_level 30 < 40 — what makes 40 exclude bots. (5) TTL 7 days, approvals_required, pipeline flag — reversible knobs or recorded tier gaps; rank last.

RISK-VALUE: DERIVED — merge_access_level = 40 @ docs/streams/forge-gitlab/pilot-report.md:25 — the brief's single-point-of-failure control "merge is always the human's": every service account sits at 30 (:23) and only the human owner at 50 (:22), so a threshold of 40 is the unique GitLab level that admits the owner and excludes every bot; 30 (the pre-repair read, :24) admits all five Developer bots. Derivation is of the value; whether the live group still holds 40 today is could-not-check offline (the report itself flags a hand-repaired control as able to regress).
RISK-VALUE: DERIVED — push_access_level = 0 @ docs/streams/forge-gitlab/pilot-report.md:25 — CE cannot name a single board-writer in allowed-to-push (Premium), so "No one" is the only CE setting that keeps every write to main behind a human-merged MR; any non-zero role level would let a bot push main unreviewed.
RISK-VALUE: NAMED, NOT DERIVED — PAT scope = ["api"] for the reviewer, desk, issue-loop and intake-loop accounts @ docs/streams/forge-gitlab/pilot-report.md:154 — missing: a per-role derivation of the minimal scope each role's duties need, and a ruling on whether whole-account `api` (which let the desk account push a branch and the reviewer create and delete one, rows 1 and 12) is an acceptable custody posture. It could not be derived here: the report shows no GitLab tier offers a narrower token shape (D-5), so the answer is a security-acceptance judgment for the human gate, not arithmetic.
RISK-VALUE: DERIVED (ranks last, reversible) — rotated-token TTL = 7 days @ docs/streams/forge-gitlab/pilot-report.md:160 — matches spec section 5's "7 days RECOMMENDED" expiry backstop; reversible by re-mint.

OPEN QUESTION for the human gate (carried verbatim from the NAMED, NOT DERIVED line): PAT scope = ["api"] for the reviewer, desk, issue-loop and intake-loop accounts — is whole-account `api` scope an acceptable custody posture for these roles, given no GitLab tier offers a narrower token shape? Also still owed by the human: sign the section 3 parity verdict (4 PASS, 8 FAILED-AT-TIER of which 3 are free-tier provisioning gaps, 2 COULD-NOT-CHECK), and decide whether rows 2-4 need a live re-probe against the pilot group or the Phase-0 records suffice.

VERIFY: BLOCKED — row 1 PASS (genuine); rows 2 and 3 could-not-check (live forge / live credential, offline envelope); row 4 witness pass vacuous (wrong repo), could-not-check against the pilot project. No FAIL observed. gate:human, sensitive-data:yes — status stays implemented; a model does not sign the live-pilot security-parity verdict.

## Review
Gate: human (from frontmatter) — the human signs the parity verdict; the reviewer
additionally records verdict + date in the stream README table.
