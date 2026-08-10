---
brief: desk-apps/06
title: desk App cutover — coordinator main-landing identity
why: >-
  The coordinator (Bob) lands small main commits today as the-org — the same login workers and
  verify-desk use — so "the desk said so" is indistinguishable from "a worker said so." A dedicated
  desk-app[bot] gives the coordinator an tamper-evident main-landing identity, and it is one of only
  the three actors the brief-08 ruleset sanctions on main, so it must exist + be wired before that
  ruleset locks main down.
wave: 2
depends: ["desk-apps/03", "desk-apps/02"]
unblocks: ["desk-apps/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) ({verifier-app, desk-app, human:<name>} may land main)", "CLAUDE.md (the coordinator 'small doc commits' exception; ASSAY_MAIN_COMMIT_OK client hook)", "memory verify-desk-operating-mode + bob-the-desk (the coordinator persona)"]
---

# Brief 06 — desk App cutover (coordinator main-landing identity)

**BLOCKED-ON-HUMAN:** requires the desk App to exist (created by human:<name> via guide 02).

## Context
files: the coordinator commit path (the tools/skills the desk uses to land main commits today) +
a routing table deliverable at `docs/streams/desk-apps/main-commit-actor-routing.md` (new).
facts:
- [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md): only `{verifier-app, desk-app, human:<name>}` may land `main`. The desk App is the coordinator's
  identity for the main commits that are NOT Evidence (those are verifier-app, brief 04) and NOT
  worker PRs (those are worker-app, brief 05).
- **Shared-value change (rule 6) — WHO commits to main.** Enumerate the current main-committers:
  `git log origin/main --format='%ae %an' | sort | uniq -c | sort -rn`. Route each to exactly one of
  {desk-app / verifier-app (brief 04) / worker-app via PR (brief 05) / stays-the-org (why) /
  stays-CI ([skip-status-regen], Flux) / human:<name>}. Known today: verify-desk Evidence/status (→verifier-app,
  brief 04); status-regen (stays CI); Flux (stays); coordinator doc/brief-row commits (→desk-app).
- The client-side `ASSAY_MAIN_COMMIT_OK` hook ([F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)) currently gates main pushes; it is REPLACED by
  the server-side ruleset (brief 08) that sanctions desk-app. This brief wires the desk's commit path
  to desk-app[bot]; brief 08 enforces it server-side.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. Grep the main-commit actors; write the routing table to `main-commit-actor-routing.md` (every
  actor routed, none unlisted — the class-sweep rule).
2. Wire the coordinator's main-commit path (the desk skill's small-doc-commit flow) to use
   `desktoken desk` → commits attribute to `assay-desk-app[bot]`.
3. Tests/Verify: a coordinator main-commit attributes to desk-app[bot]; non-coordinator tools still
   cannot land main.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/desk-apps/main-commit-actor-routing.md` | exit 0 |
| 2 | `git log origin/main --format='%ae %an' | sort -u | wc -l` matches the routing table's enumerated actor count | counts agree (every actor routed) |
| 3 | **(live, blocked-on-human)** a coordinator main-commit; `gh api …/commits/<sha>` actor | `assay-desk-app[bot]` |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer. -->

### Non-implementer verify — VERIFY: FAIL (rows 2 & 3) — glm-5.2-verifier, merged main `422a8732`, 2026-07-26

**Filed as #1356.** Row 3's failure is load-bearing — it defeats the brief's "tamper-evident main-landing identity" why. Rows 1 & 4 PASS.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f docs/streams/desk-apps/main-commit-actor-routing.md` | 0 | PASS — file present (3403 B) | 2026-07-26 | glm-5.2-verifier |
| 2 | `git log origin/main --format='%ae %an' \| sort -u \| wc -l` | 0 | **FAIL** — 12 vs the table's 8 pairs / 6 identities; 4 actors unlisted (incl. `assay-desk-app[bot]` itself — 5 commits, brief-06's own success) | 2026-07-26 | glm-5.2-verifier |
| 3 | `gh api …/commits/<sha>` actor linkage | 0 | **FAIL** — `assay-desk-app[bot]` commits have `.author.login = null` / `.author.type = null`; git-level name attribution works but the tamper-evident GitHub-side linkage is not achieved | 2026-07-26 | glm-5.2-verifier |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | PASS — 0 ERROR / 0 WARN / 362 NOTICE | 2026-07-26 | glm-5.2-verifier |

**Root cause (#1356):** the App-commit email prefix uses the **App ID** (`4331346` desk / `4331323` verifier) but GitHub links bot commits via the **Bot User ID** (`306483193` desk / `306482097` verifier) — pattern-proven against `assay-worker-app[bot]` (`306480234+…`) and `assay-issue-loop-app[bot]` (`306484374+…`) which link correctly. **Verify-desk independently confirmed this on its own commits** (`e27ede60` → `gh_login: null`). Fix = swap the prefix to the bot user id (skill/mint-script wiring — the-desk/human:<name>'s; verify-desk does not edit skills). Brief stays `implemented`. RISK-VALUE: DERIVED.

## Review
Gate: model + `/security-review`. Reviewer confirms the routing table is complete (no unlisted
main-committer) and desk-app can't be minted by non-coordinator paths.

### Non-implementer verifier run — VERIFY: FAIL (rows 2 & 3 substantive) — k3-verifier (verify-desk dispatch), 2026-07-27, merged main `26766ba6`

Row 1 PASS (routing table present); row 4 PASS (no new lint problems attributable). **Row 2 FAIL:** 13 unique `%ae %an` pairs vs the table's 8 — 5 unrouted actors incl. `306482097+assay-verifier-app[bot]` (the corrected linking prefix, 19 commits) and `4331346+assay-desk-app[bot]` (desk-app itself, 5). **Row 3 FAIL (observable NOW, not blocked-on-human):** desk-app coordinator main-commits are GitHub-unlinked (`author.login=null`) — `../oit/.claude/skills/the-desk/SKILL.md` + routing table still prescribe the App-ID prefix `4331346+` instead of bot-user-id `306483193+` (#1356 OPEN; contrast: corrected verifier commits link as Bot). RISK-VALUE verdicts: desk-app prefix REFUTED by live evidence; verifier row mislabeled "correct"; worker row correct. **Filed as #1466.** Brief stays `implemented`.
