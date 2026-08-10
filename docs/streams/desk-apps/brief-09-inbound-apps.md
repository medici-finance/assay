---
brief: desk-apps/09
title: INBOUND Apps — issue-loop + intake-loop (the two inbound lanes)
why: >-
  INBOUND is one loop with two lanes (issues = work-shaped, intake = idea-shaped; the routing test
  decides — decks/loops/deck.md slide 3). Each lane is a distinct agent actor that files/routes its
  own items, so each gets its own App: issue-loop-app for the issues lane, intake-loop-app for the
  intake lane. This completes the per-role App family across every agent-actor loop (REVIEW, VERIFY,
  DISPATCH, COORDINATE, INBOUND×2); METRICS is zero-AI and RETRO is human, so they are deliberately
  non-App.
wave: 3
depends: ["desk-apps/03", "desk-apps/02"]
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
decision-issue: 741
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md); inbound-lane coverage 2026-07-12)
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (issue-loop app — distinct enforcement actor)", "decks/loops/deck.md slide 3 (INBOUND — one front door, two lanes; the routing test)", "docs/streams/issue-loop/README.md § 'One inbound loop, two lanes'", "INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) (issues workstream)", "INTAKE I-loops-reference"]
gate-why: >-
  Creating public GitHub Apps is an outward, public, irreversible act (new identities on GitHub
  installed across accounts) — human:<name>'s, not a model's. The brief scopes both inbound-lane Apps; human:<name>
  decides when to activate them (lower priority than reviewer/verifier/worker/desk).
---

# Brief 09 — INBOUND Apps (issue-loop + intake-loop)

**The INBOUND loop's two lanes each get an App.** Lower priority than reviewer/verifier/worker/desk
(briefs 04–06); activated when the inbound desk wants its own distinct actors. The icons already
ship with the family (brief 01) so the visual set is complete now.

## Context
files: TBD at activation (a small inbound-desk tool per lane + the App creation via guide 02).
facts:
- **Both Apps provisioned 2026-07-18** (issue #404 pass): `assay-issue-loop-app` = App **4331385**
  (the-org install 147395450, medici-finance install 147395610); `assay-intake-loop-app` = App
  **4331405** (the-org install 147396055, medici-finance install 147396073). Canonical record:
  desk-apps README [Provisioned Apps](./README.md#provisioned-apps-2026-07-18).
- **Permissions refined at activation:** both inbound Apps got `pull_requests: write` +
  `issues: write` + `contents: write` (`metadata: read`) — **not** the originally-proposed
  issues-only (issue-loop) / issues+contents (intake-loop). Reason: the inbound lanes act via **PRs**
  — issue-loop opens **close-PRs** (issue-loop/10: agents close issues via a `bugs/<N>.md` PR with
  `Closes #<N>`, never a direct close), and intake-loop opens **entry PRs** for INTAKE/FINDINGS
  entries — so both need `pull_requests: write` + `contents: write`, not just issue writes.
- **`assay-issue-loop-app[bot]`** — the issues lane (work-shaped items filed as GitHub issues,
  routed hand-to-a-worker-as-is; opens close-PRs). Avatar:
  `assay-toolkit/docs/brand/issue-loop-app.svg` (octagonal stamp + ticket glyph).
- **`assay-intake-loop-app[bot]`** — the intake lane (idea-shaped; needs judgment → routed to an
  INTAKE entry in `docs/streams/intake/`; opens entry PRs). Avatar:
  `assay-toolkit/docs/brand/intake-loop-app.svg` (octagonal stamp + funnel glyph).
- Both public, dual-install (per guide 02). Role = key custody; `desktoken issue-loop` /
  `desktoken intake-loop` mint their tokens.
- **Routing test stays human/coordinator-owned** (decks/loops slide 3): the App owns the lane's
  filing/editing voice, not the judgment of which lane an item belongs in.

## Ground rules
- NEVER git push / trigger workflows / create the Apps. human:<name>'s acts.
- Stop at `implemented` — you do not set verified/done.

## Task
(At activation) define each lane's verbs the App owns, create both Apps via guide 02, wire the
tools. **Activated 2026-07-18 (issue #404):** human:<name> created both Apps (IDs/installs in facts above);
`desktoken` now accepts the `intake-loop` role (alongside `issue-loop`), so both lanes can mint
tokens. Permissions were refined from the proposal (see facts). Per-lane inbound-tool verbs remain
defined incrementally as each lane's tooling lands.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verifier run — VERIFY: PASS (row 1); held `implemented` for human sign-off (glm-5.2-verifier, merged main `c2311679`, 2026-07-18)

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 — advisory NOTICEs only |

The irreversible act this brief gates — creating the two public inbound Apps — is **already done**
(2026-07-18, issue #404): `assay-issue-loop-app[bot]` (App 4331385, installs 147395450 / 147395610) and
`assay-intake-loop-app[bot]` (App 4331405, installs 147396055 / 147396073), public + dual-install
(`the-org` + `medici-finance`), permissions refined to `pull_requests` + `issues` + `contents: write`
(both lanes act via PRs — close-PRs / entry-PRs — not issue writes alone). `desktoken` accepts both the
`issue-loop` and `intake-loop` roles. Row 1's lint PASS + the provisioned-App record satisfy the Verify table.

**No flip.** `gate: human` + `irreversible: yes` ⇒ statusgen `brieffile.go:709` forbids a model-only flip
to `verified` (an irreversible brief needs a `human:<name>` in Reviewed before it may be marked verified
or done). Held at `implemented`; sign-off decision issue
**[#741](https://github.com/example-org/oit/issues/741)**.

### Resolved — signed off → verified (2026-07-19)

human:<name> (`human:<name>`) closed decision issue [#741](https://github.com/example-org/oit/issues/741) on **2026-07-18 with "A"** (Option A = sign off — accept). The human:<name> close is the real human authorization artifact for the irreversible floor (`brieffile.go:709`); Reviewed = `2026-07-18 human:ian`. With the Verify table already PASS (row 1, lint) + the human sign-off recorded, the brief advances `implemented → verified`. (The state-tracking gap that delayed this — verify-desk not checking decision-issue close state — is filed as [#869](https://github.com/example-org/oit/issues/869).)

## Review
Gate: human (gate-why above — public App creation). `/security-review` at activation (auth/identity).
