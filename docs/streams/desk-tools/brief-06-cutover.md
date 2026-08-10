---
brief: desk-tools/06
title: Cutover — human install, allowlist swap, skill wiring, zero-prompt + kill-switch drills
wave: 2
depends: ["desk-tools/02", "desk-tools/03", "desk-tools/04", "desk-tools/05", "desk-tools/07", "desk-tools/08"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [209, 210]
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md (rollout + TM-1 accepted residual)", "freshness-checked 2026-07-10 @ b98e1e84"]
gate-why: >-
  This is the step that actually removes per-action human prompts from outward-facing GitHub
  writes (bot reviews, ready-flips, PR creation) for EVERY session in the repo — permission
  rules cannot distinguish the review window from a worker. human:<name> is confirming the trade
  prompt-per-action → policy-in-code + audit + human merge gate, and signing off the TM-1
  accepted residual: the review gate stays tamper-evident from GitHub's side but becomes
  honor-system locally. sensitive-data flag: the App private key handling moves into the
  installed toolchain.
why: >-
  Briefs 01-05 build the tools; nothing goes zero-prompt until the binaries are installed, the
  allowlist swaps to them, and the loops actually call them. This brief is the switch — and the
  place where the design's drills (zero-prompt cycle, kill switch) prove the whole stream did
  what it claimed.
---

# Brief 06 — Cutover

## Context
files: `.claude/settings.json` (permission allowlist), `~/.claude/skills/pr-review-desk/SKILL.md`,
`~/.claude/skills/verify-desk/SKILL.md`, `~/.claude/skills/batch-fanout/SKILL.md`,
`~/.claude/skills/the-desk/SKILL.md` (the coordinator quotes ready-lists too — #209 names it;
all four out-of-repo: edits land live but can't ride this PR — record diffs in the PR body),
`Makefile` (desk-install from brief 01), `docs/streams/desk-tools/scoping.md` (the
accepted-residual text to sign)
facts:
- Allowlist changes (exact): ADD `Bash(/opt/desk-tools/bin/deskboard *)`,
  `Bash(/opt/desk-tools/bin/deskpost *)`, `Bash(/opt/desk-tools/bin/deskpr *)`,
  `Bash(/opt/desk-tools/bin/deskwt *)`, `Bash(/opt/desk-tools/bin/deskreply *)`, plus the
  worker-local rules `Bash(git commit *)` and `Bash(git merge origin/main*)` (local-only
  mutations — scoping "write/read line"). REMOVE
  `Bash(go run /Users/iholsman/.claude/skills/pr-review-desk/deskboard.go*)` (the TM-3
  source-path trap rule). Do NOT remove statusgen (recorded TM-3 exemption) or other rules.
- **settings.local.json purge (deliverable, not a footnote):** the shared checkout's untracked
  `.claude/settings.local.json` holds ~1,179 accreted rules incl. `Bash(gh *)`,
  `Bash(git push *)`, `Bash(go run *)` — wider than everything this stream builds. Enumerate
  it, remove wide execution/write rules, keep narrow reads; paste the before/after rule counts
  and the removed-rules list into the PR body. Without this the TM-1 sign-off is theater.
- **Rollback (pre-written):** BEFORE the swap, commit `../assay-toolkit/tools/desk/rollback.patch` — re-adds the
  removed rule(s) and reverts the three skill edits in one `git apply`. Also record the
  worktree-propagation caveat: pre-cutover worktrees keep the old checked-in rules until they
  merge main — note it in the PR body and merge main into long-lived open branches at cutover.
- `sudo make desk-install` is run BY IAN (C-1): the sudo password IS the manual permission
  gate; binaries land root-owned in /opt/desk-tools/bin and no agent can replace them. The brief's implementer prepares everything and STOPS before
  install; the Verify drills below are executed WITH human:<name> at the gate.
- Skill wiring: pr-review-desk uses `deskboard`/`deskpost` for board reads (incl. reviewer
  diff/files reads), review/comment posting, and the ready-flip; batch-fanout workers use
  `deskpr create`/`update` and `deskreply` for findings replies; verify-desk uses `deskwt`
  (its documented `../verify-desk-main` location is amended to a sanctioned prefix); all three
  boot steps check deskboard's STALE banner (C-1 drift). Two rules land in the same skill
  edits: (#209) any message naming PRs "ready for merge" must come from a deskboard/gh run in
  THAT turn — never repeated from session context (measure at mention time); this rule lands in
  pr-review-desk AND the-desk (the coordinator quotes ready-lists too), phrased with the
  greppable marker "same-turn" so Verify row 7 can gate on it; (#210, strengthened) verify-desk
  + its verifier-dispatch template: ALL cluster-touching Verify rows run EXCLUSIVELY via the
  medici-admin pod (probe-canton.sh / verify-user.sh / check-reconciler.sh + the documented
  recipes, docs/debugging-guide.md) — hand-rolled curl/token chains are prohibited; a missing
  probe = file an issue + add the probe script to the pod ConfigMap, never bypass. Local-artifact
  rows (go test / dpm test / lint / greps) stay local — the pod holds the admin token and must
  not become a CI runner (full trade-off analysis on #210). NOTE: verify-desk SKILL.md today
  SANCTIONS "read-only curl https://canton.dev... GETs" — that clause is removed/replaced by
  this rule, not supplemented. (#212 is CLOSED by methodology/32, pulled forward 2026-07-10 —
  this brief only re-verifies the worker mergeable-watch rule survived the skill rewiring and
  wires deskboard `prs` mergeState as the preferred sensor post-cutover.) Each skill keeps the
  RAW-command fallback
  documented for when a tool exits 3/5/6 (fail-closed routes back to the prompted path — C-10).
- Drills (the proof the stream delivered):
  1. **Zero-prompt cycle:** one full review-loop pass — board read → review posted by App →
     fix pushed (worker) → re-review → ready-flip — with ZERO permission prompts observed.
  2. **Kill-switch drill:** `touch ~/.claude/desk-tools/DISABLED` → every tool exits 3;
     remove the file → tools work again. Timed: disarm-to-dead under 5 seconds.
  3. **Refusal spot-check:** deskpost with a token-shaped body → exit 5, nothing posted.
  4. **Binary-manifest check:** `shasum -a 256 -c tools/desk/MANIFEST.sha256` against
     /opt/desk-tools/bin → all OK (C-1 drift detection works).
  5. **Settings-diff check:** `git diff` of `.claude/settings.json` + the local.json before/after
     counts match the PR body exactly (TM-1b: the allowlist itself is honor-system — the drill
     makes tampering visible).
  6. **App parity check:** verify per-repo (all SEVEN — the the-org three plus the
     medici-finance report repos assay-toolkit/reconciler/decks/proposals, [F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md)) that the
     reviewer App installation covers it AND that the ready-flip actor works there (C-4:
     App-installation parity is verified, not assumed — the medici-finance org needs its
     own App installation) — record which identity performs the flip if the App cannot.
- TM-1 accepted-residual sign-off: human:<name> records (in this brief's Evidence + the verify-gate
  issue this brief will get at `verified`) that he accepts local forgeability of the review
  gate as a knowing trade, with branch protection named as the escalation if it bites.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. Do NOT run `make desk-install` (human act) and do NOT edit
  `.claude/settings.json`'s deny rules.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Prepare the settings.json allowlist edit exactly per facts (add 5 desk-tool rules + 2
   worker-local git rules, remove 1) — as the diff in this brief's PR (the file is
   repo-tracked, so it CAN ride the PR).
2. Update the three skills per facts; since they live under `~/.claude/` (out-of-repo), apply
   the edits live AND paste each skill's diff into the PR body for the record.
3. Write the tools/desk README operator section: install/upgrade procedure (human),
   version-check (`deskboard --version` vs repo HEAD; mismatched = stale binary warning),
   kill-switch procedure, rollback (delete the 5 desk-tool allow rules + rm the binaries —
   full revert in two steps).
4. Script the drills as a checklist in the PR body; execute them WITH human:<name> post-install and
   record outcomes in Evidence.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `python3 -c "import json;d=json.load(open('.claude/settings.json'));a=d['permissions']['allow'];print(sum(1 for r in a if r.startswith('Bash(/opt/desk-tools/bin/')), sum(1 for r in a if 'deskboard.go' in r))"` | `5 0` |
| 2 | `ls /opt/desk-tools/bin/ \| wc -l` (post-install, human:<name>-run) | ≥5 |
| 3 | zero-prompt drill (facts #1), human:<name> observing | 0 permission prompts in the full cycle |
| 4 | kill-switch drill (facts #2) | all tools exit 3 while armed; recover on disarm |
| 5 | refusal spot-check (facts #3) | exit 5; no comment appears on the target PR |
| 6 | `statusgen --root . --lint; echo $?` | 0 |
| 7 | `grep -l "same-turn" ~/.claude/skills/pr-review-desk/SKILL.md ~/.claude/skills/the-desk/SKILL.md \| wc -l` | `2` (#209 rule present in both) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     Rows 2-5 are executed with human:<name> (post-install drills); his accepted-residual sign-off
     is recorded here alongside the drill outcomes. -->

## Review
Gate: human (see gate-why — this is the prompt-removal switch and the TM-1 residual sign-off).
`/security-review` in addition to `/code-review`. The verify-gate issue for this brief IS the
sign-off surface; closing it is human:<name> accepting the trade.
