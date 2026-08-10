---
brief: desk-apps/08
title: "[F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) server-side main-push ruleset ({verifier-app, desk-app, human:<name>})"
why: >-
  The `ASSAY_MAIN_COMMIT_OK` client-side hook ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)) only stops faithful agents — a determined one
  exports the env var. A GitHub repo ruleset restricting main pushes to {verifier-app, desk-app,
  human:<name>} makes the boundary server-side: the governance of who can land main stops being honor-system.
wave: 3
depends: ["desk-apps/04", "desk-apps/06"]
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) goes server-side — repo ruleset restricting main pushes to {verifier-app, desk-app, human:<name>})", "CLAUDE.md [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) main-commit backstop (the client hook being replaced)"]
gate-why: >-
  A main-push ruleset is an outward security-control change across repos that redefines who can land
  on the deploy trigger. Reverting is possible but the boundary change itself — and any window it
  blocks legitimate pushes or allows illegitimate ones — is exactly the kind of governance act that
  is human:<name>'s: signing off who can push main is signing off the integrity boundary (same shape as
  assay-dogfood/01's permission-matrix gate).
---

# Brief 08 — [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) server-side main-push ruleset

**The act is human:<name>'s (human-gate, outward GitHub settings).** The brief prepares the ruleset spec +
the client-hook removal plan; human:<name> executes the settings change.

## Context
files: ruleset spec at `docs/streams/desk-apps/main-push-ruleset.md` (new — the JSON ruleset human:<name>
uploads); CLAUDE.md [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) note updated to reference the server-side ruleset.
facts:
- GitHub repo ruleset (per repo: oit, agents, examples, + the medici-finance report repos):
  "restrict pushes" to `main` → allowed actors `{assay-verifier-app[bot], assay-desk-app[bot],
  ianholsman}` (verify the exact human:<name> handle). Everyone else (incl. the-org PAT) → rejected.
- Replaces the client-side `core.hooksPath .githooks` + `ASSAY_MAIN_COMMIT_OK` env-var hook ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)).
  The client hook can stay as defense-in-depth but is no longer the primary gate.
- Depends on briefs 04 + 06: the verifier + desk Apps must exist + be installed before the ruleset
  names them, else the desk/verifier can't push main once it locks.
- **BLOCKER — the ruleset API is plan-locked, so Verify rows 3+4 cannot run today.** Verified
  2026-08-02: `GET /repos/medici-finance/assay-toolkit/rulesets` and
  `…/branches/main/protection` both return **HTTP 403** *"Upgrade to GitHub Pro or make this
  repository public to enable this feature"*; identically for
  `oit`. The `medici-finance` org is on the **free** plan and
  both repos are **private** — that combination offers neither rulesets nor branch protection.
  Rows 1+2 (the spec doc + lint) are runnable; the live half needs a **plan upgrade or a
  visibility change** first, which is human:<name>'s call and is not in this brief's scope. Context:
  enforcement-model.md.

## Ground rules
- NEVER git push / trigger workflows / change GitHub settings. The ruleset upload is human:<name>'s. Leave
  commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. Write the ruleset spec (the JSON + the per-repo apply steps) to `main-push-ruleset.md`.
2. Update CLAUDE.md [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) to note the server-side ruleset as primary, client hook as defense-in-depth.
3. Hand human:<name> the apply runbook (settings clicks per repo); record his execution in Evidence.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/desk-apps/main-push-ruleset.md && grep -ciE 'verifier-app|desk-app' docs/streams/desk-apps/main-push-ruleset.md` | ≥2 |
| 2 | `statusgen --root . --lint; echo $?` | 0 |
| 3 | **(live, human:<name>-applied)** a push to main from a non-sanctioned actor (e.g. the-org PAT) | rejected by the ruleset |
| 4 | **(live, human:<name>-applied)** verifier-app or desk-app push to main | allowed |

## Evidence
<!-- appended at implementation time; human:<name>'s apply record lands here. -->

## Review
Gate: human (gate-why above — human:<name> signs the main-push boundary). `/security-review` required
(auth/identity boundary). human:<name> confirms the sanctioned actor set + that legitimate flows still land.
