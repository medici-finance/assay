---
brief: assay:assay:windows-port:14
title: The Windows CI leg proves the desk-role runtime paths — pollers, tick, inbox, hooks — and retires the bash oracles it can
why: >-
  Brief 04's Windows leg proves `statusgen --lint` and a `--version` smoke; nothing on Windows
  has ever run a desk role's actual runtime path — the inbound poll, the PR poll, the tick
  emitter, the inbox walk, the push-guard hook. Briefs 11–13 make those paths native; without a
  leg that exercises them on windows-latest the "runs on Windows" claim for the desk roles rests
  on parity fixtures recorded on a Mac. This brief adds that leg, and for each bash oracle whose
  verb the leg proves, retires the script — the only point at which retirement is honest.
wave: 5
depends: ["windows-port/04", "windows-port/11", "windows-port/12", "windows-port/13"]
unblocks: []
effort: M
gate: human
gate-why: >-
  Adds jobs under `.github/workflows/` — a security-classified path an App credential cannot push
  (the same reason brief 04 and brief 10 are human-gated): a maintainer lands the workflow hunk
  from the staged copy. `irreversible: yes` records the workflow-path rule as brief 04 did; the
  other three answers are honestly no — the leg reads no secret beyond the read-only token the
  existing leg already uses. decision-trigger start: the exact staged-copy vs direct landing
  question (the same tension brief 10 named) is best framed when the implementer has the leg.
decision-trigger: start
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-21 by the-desk (Bob) — windows-port authoring session, driver ask 2026-09-21
sources:
  - ".github/workflows/windows-ci-leg.yml (live; brief 04 done) — jobs windows-smoke, windows-bootstrap-smoke, arm64-native-smoke (held `if: false`): the leg this brief extends"
  - "ci/staged-workflows/ — the staged-copy landing pattern brief 04/06 used and brief 10 (PR #1432, not yet merged) names as a tension; the same landing question applies here"
  - "windows-port/11 (deskmonitor, desktick), /12 (hook-install, de-POSIX prose), /13 (deskinbox) — the verbs this leg proves; each keeps its .sh oracle until this brief"
  - "plugins/assay/scripts/{inbound-monitor,pr-monitor,tick-summary,assay-inbox}.sh — the oracles; `pdfingest.sh` (71 lines, curl+python3) is OUT of scope: it needs python3/docling regardless of shell and stays a documented-workaround row"
  - "docs/adopting-assay.md § Windows adopters (brief 05/09) — the doc that states what the leg proves; gains one sentence per proven path"
  - "freshness-checked 2026-09-21 @ 56491ce (origin/main): the live leg has three jobs, none exercising a desk verb beyond --version"
exec-tier: strong
exec-tier-why: >-
  Question (b): the leg must exercise four verbs against a live read on a runner with no bash
  in PATH by construction (the job removes Git-Bash from PATH), and its green must be
  attributable per verb; a leg that goes green because a step silently found bash proves nothing.
domain: complicated
consumers:
  - ".github/workflows/windows-ci-leg.yml (new job desk-role-paths; landed by a human from ci/staged-workflows/windows-ci-leg.yml): follow-up windows-port/14 (this brief)"
  - "plugins/assay/scripts/{inbound-monitor,pr-monitor,tick-summary,assay-inbox}.sh (retired once the leg is green at the head that proves each): follow-up windows-port/14 (this brief)"
  - "plugins/assay/skills/{pr-review-desk,intake-desk,inbox,ask-decision}/SKILL.md + references/tick-contract.md (drop the script fallback wording): follow-up windows-port/14 (this brief)"
  - "docs/adopting-assay.md § Windows adopters: follow-up windows-port/14 (this brief)"
  - "tools/desk/cmd/scanloop/monitor.go parity mode (removed with the oracle): follow-up windows-port/14 (this brief)"
version: 1
id: ea048fcd-b19e-466e-bbb5-7c768c57770d
---

# Brief 14 — The Windows leg proves the desk-role paths; retire the oracles

## Context
files: ci/staged-workflows/windows-ci-leg.yml (staged copy; the human promotes the hunk), .github/workflows/windows-ci-leg.yml (human-landed), plugins/assay/scripts/{inbound-monitor,pr-monitor,tick-summary,assay-inbox}.sh (deleted), the four skill bodies + references/tick-contract.md, tools/desk/cmd/scanloop/monitor.go, docs/adopting-assay.md, changelog/windows-port-14-leg.md
facts:
- new job `desk-role-paths` on `windows-latest`, PowerShell steps only, PATH scrubbed of Git-Bash (`$env:PATH = ($env:PATH -split ';' | Where-Object { $_ -notmatch 'Git' }) -join ';'`) and asserted (`Get-Command bash` must FAIL); then: `deskmonitor inbound --once --repos medici-finance/assay` (read-only, public repo, no token), `deskmonitor pr --once …`, `desktick regexp`, `deskinbox flow medici-finance/assay`, `deskpushguard hook-install --dry-run`; each step's exit code and a one-line stdout hash uploaded as the job summary
- attribution: one step per verb, `continue-on-error: false`, no shared step — a red names the verb
- retirement rule: a script is deleted in this brief ONLY if (a) its verb's step is green on the leg at the head of this PR and (b) the parity test in 11/13 was green at that head; otherwise the script stays and the reason is recorded in the PR body
- landing: the workflow hunk goes to `ci/staged-workflows/` in this PR; the human promotes it (the `## Human decision` the executor authors at pickup frames staged-copy vs direct)
single-point-of-failure: the ONE control is the PATH-scrub assertion (without it a green leg could have used bash). Independent layer: 11/13's parity tests on a bash-less Go test run (`-tags nobash` or a runner without bash) — a different runner, different signal.

## Human decision
<!-- decision-trigger: start — the executor authors this at pickup: the landing mechanism for
     the workflow hunk (staged copy promoted by a maintainer vs direct), the same question
     brief 10 raised; then files it via tools/decision-issue.sh ensure … --at start. -->

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Never delete a script whose verb the leg has not proven at this PR's head.
- The leg reads only public data with no token; a step that needs a token is out of scope.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Author the `desk-role-paths` job in the staged copy per `facts:`.
2. Retire each oracle whose two conditions hold; drop the fallback wording in the skills and
   tick-contract.md; remove scanloop's parity mode when inbound-monitor.sh goes.
3. docs/adopting-assay.md: one sentence per proven path in the Windows section.
4. Author `## Human decision` at pickup; file the decision issue; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | Staged job present: `grep -c 'desk-role-paths' ci/staged-workflows/windows-ci-leg.yml` | `>= 1` | `check` |
| 2 | **PATH-scrub assertion exists**: `grep -c 'Get-Command bash' ci/staged-workflows/windows-ci-leg.yml` | `>= 1`, and the step expects failure | `check` |
| 3 | One step per verb: `grep -c -e 'deskmonitor inbound' -e 'deskmonitor pr' -e 'desktick regexp' -e 'deskinbox flow' -e 'hook-install' ci/staged-workflows/windows-ci-leg.yml` | `5` | `check` |
| 4 | **DEREFERENCE — the leg ran green on this head** (after the human promotes it): `gh run list --workflow windows-ci-leg.yml --branch <this PR branch> --json conclusion,headSha --jq '.[0]'` | `success` at this PR's head; could-not-check with reason before promotion | `check +dereference` |
| 5 | Retirement honest: for each deleted script, the PR body cites the green step + the parity test; `git diff --name-status origin/main -- plugins/assay/scripts/ \| grep -c '^D'` | equals the number of scripts the body claims retired | `check` |
| 6 | No fallback wording left for retired scripts: `grep -c -e 'inbound-monitor.sh' -e 'pr-monitor.sh' -e 'tick-summary.sh' -e 'assay-inbox.sh' plugins/assay/skills/*/SKILL.md plugins/assay/references/*.md` | `0` for each retired script | `check` |
| 7 | **Flow — a desk boots without bash**: on a Windows host (or the leg): `deskboot intake-desk --dry-run` with bash absent from PATH | exit 0 and the inbound surface reported as armed | `check +flow` |
| 8 | Decision issue: `gh issue list -R medici-finance/assay --label needs-decision --search 'decision-gate: windows-port/14' --json number --jq length` | `1` | `check` |
| 9 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/14; echo $?` | `0` | `check` |
| 10 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Row 4 and 7 depend on the
     human promoting the workflow hunk. -->

## Review
Gate: **human** (workflow path). Reviewer's questions: (1) row 2 — would the leg go red if bash
were on PATH? (2) is every retired script backed by BOTH a green step and a green parity test at
this head? (3) does any step need a token the leg does not have?
