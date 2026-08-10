---
brief: methodology/06
title: Implementer deny-hooks — mechanical enforcement of the two hard prohibitions
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §3 enforcement layer (c) — explicitly assigned here so it doesn't fall through", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)"]
---

# Brief 06 — Implementer deny-hooks (enforcement layer c)

## Context
files: .claude/settings.json (this repo); report-only note for ~/.claude/settings.json (user-level is the human's call)
facts:
- Spec §3 prohibitions: (1) no mutating kubectl (apply/delete/edit/scale/rollout, flux triggers); (2) no git commit/push/workflow triggers without explicit permission. Layers (a) engineering-standards doc and (b) brief Ground-rules block exist; layer (c) mechanical deny is NOT shipped.
- Harness mechanism: `permissions.deny` entries in `.claude/settings.json` (Bash tool patterns). Scope: the deny list must block the mutating patterns while leaving read-only kubectl (get/logs/describe) and git status/diff/log usable.
- Constraint: this session-shared settings file affects ALL sessions in the repo, including the coordinator that legitimately commits. `git commit` therefore CANNOT go in the repo deny list — it stays enforced by layers (a)/(b) + review; the deny list covers `git push`, `gh workflow run`, `gh run rerun`, `flux reconcile`, and mutating kubectl. Document exactly this boundary and why.
- AMENDED 2026-07-09 ([I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) supersession, desk decision): [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) made agent-run branch push + draft PR the sanctioned review loop, so plain `git push` moved to the same layer as `git commit` (process + review, not mechanical deny); only force-push is denied mechanically. Verify row 2 updated to match.
- AMENDED 2026-07-09 (PR #114 desk review, same wave — Verify table unchanged): (i) force-push deny also covers the args-after-refspec shape (`git push * --force*` / `git push * -f*`); (ii) `kubectl rollout*` narrowed to the four mutating verbs (restart/undo/pause/resume) so read-only `rollout status`/`history` stay usable; (iii) coverage extended to `kubectl create/replace/annotate/label` per spec §3. `kubectl exec` is deliberately NOT denied: the medici-admin debug pod (`kubectl exec deploy/medici-admin -- <script>`) is the sanctioned read-only diagnostic path (docs/debugging-guide.md) — denying exec breaks it; exec misuse stays on layers (a)/(b). Review finding 4 (any deny-glob is defeated by `sh -c`/env-prefix/abs-path wrapping) is a conscious accept: layer (c) guards against accidental direct invocation, not a sandbox.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add deny patterns to .claude/settings.json permissions for: force-push in both arg orders (`git push --force*`, `git push -f*`, `git push * --force*`, `git push * -f*`), `gh workflow run*`, `gh run rerun*`, `flux reconcile*`, `flux suspend*`, `flux resume*`, and mutating kubectl (`apply`, `create`, `replace`, `delete`, `edit`, `scale`, `annotate`, `label`, `patch`, plus the four mutating rollout verbs `rollout restart/undo/pause/resume`) — 22 entries. Plain `git push` (non-force) is deliberately NOT in this list (see AMENDED fact lines above) — it stays process-governed per layers (a)/(b) + the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) review loop, exactly like `git commit`; `kubectl exec` stays available for the medici-admin debug pod. The human can allowlist per-invocation when they intend a denied operation.
2. Preserve every existing settings key untouched; JSON must stay valid.
3. Document the layer-(c) boundary (why git commit is not denied repo-wide) in engineering-standards when that doc is authored — for now add one line to this stream README's conventions.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `jq -e '.permissions.deny | length >= 9' .claude/settings.json` | exit 0 |
| 2 | `jq -e '.permissions.deny | map(select(startswith("Bash(git push --force"))) | length >= 1' .claude/settings.json` | exit 0 |
| 3 | `jq . .claude/settings.json > /dev/null && echo valid` | prints `valid` |
| 4 | `jq -r '.permissions.deny[]' .claude/settings.json \| grep -c kubectl` | ≥6 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `jq -e '.permissions.deny \| length >= 9' .claude/settings.json` | 0 | `true` (13 deny entries added) | 2026-07-09 | implementer (sonnet) |
| 2 | `jq -e '.permissions.deny \| map(select(startswith("Bash(git push --force"))) \| length >= 1' .claude/settings.json` | 0 | `true` (`Bash(git push --force*)` present; plain `git push` deliberately absent per [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) amendment) | 2026-07-09 | implementer (sonnet) |
| 3 | `jq . .claude/settings.json > /dev/null && echo valid` | 0 | `valid` | 2026-07-09 | implementer (sonnet) |
| 4 | `jq -r '.permissions.deny[]' .claude/settings.json \| grep -c kubectl` | 0 | `6` (apply/delete/edit/scale/rollout/patch) | 2026-07-09 | implementer (sonnet) |

`go run ./tools/statusgen --root . --lint` also run: exit 0, clean.

Review-response run (post PR #114 desk review; deny list now 22 entries):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `jq -e '.permissions.deny \| length >= 9' .claude/settings.json` | 0 | `true` (22 entries) | 2026-07-09 | implementer (Fable, review response) |
| 2 | `jq -e '.permissions.deny \| map(select(startswith("Bash(git push --force"))) \| length >= 1' .claude/settings.json` | 0 | `true` (+ args-after-refspec shapes) | 2026-07-09 | implementer (Fable, review response) |
| 3 | `jq . .claude/settings.json > /dev/null && echo valid` | 0 | `valid` | 2026-07-09 | implementer (Fable, review response) |
| 4 | `jq -r '.permissions.deny[]' .claude/settings.json \| grep -c kubectl` | 0 | `13` (≥6) | 2026-07-09 | implementer (Fable, review response) |

Independent verification (non-implementer opus re-run on merged main 37c0eab2, 2026-07-09):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `jq -e '.permissions.deny \| length >= 9' .claude/settings.json` | 0 | `true` (deny list = 22 entries) | 2026-07-09 | independent (opus-verifier) |
| 2 | `jq -e '.permissions.deny \| map(select(startswith("Bash(git push --force"))) \| length >= 1' .claude/settings.json` | 0 | `true` (force-push denied in both arg orders) | 2026-07-09 | independent (opus-verifier) |
| 3 | `jq . .claude/settings.json > /dev/null && echo valid` | 0 | `valid` | 2026-07-09 | independent (opus-verifier) |
| 4 | `jq -r '.permissions.deny[]' .claude/settings.json \| grep -c kubectl` | 0 | `13` (≥6) | 2026-07-09 | independent (opus-verifier) |
| + | `go run ./tools/statusgen --root . --lint` | 0 | clean | 2026-07-09 | independent (opus-verifier) |

Lockout sweep (the Review section's coordinator concern, checked category by category):
plain `git push`/`git commit` NOT denied (per [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) amendment); read-only kubectl
(get/describe/logs/top/wait/config) and read-only flux (events/get/logs) NOT denied;
`kubectl rollout status` NOT denied (narrowing held); `kubectl exec` NOT denied
(medici-admin debug path preserved); zero allow/deny contradictions; both PostToolUse and
PreToolUse hook guards intact.

Review verdict (model:opus, 2026-07-09): **PASS — closed to `done`.** Confirmed gate:model
with all-no risk in frontmatter (closeable by a model). Read `.claude/settings.json`: the
deny list is exactly the 22 entries the contract specifies — force-push in all four arg
shapes (`--force*`, `-f*`, and the args-after-refspec `git push * --force*`/`-f*`),
`gh workflow run*`, `gh run rerun*`, `flux reconcile/suspend/resume*`, and the mutating
kubectl verbs `apply/create/replace/delete/edit/scale/annotate/label/patch` + the four
mutating `rollout {restart,undo,pause,resume}`. Ran the coordinator-lockout sanity check the
Review section demands: **plain `git push` and `git commit` are NOT denied** (only force-push
variants match — per the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) amendment they stay process-governed), **`kubectl exec` is NOT
denied** (grep count 0 — the medici-admin debug path is preserved), and `kubectl rollout
status/history` is not caught by the four-verb narrowing. JSON valid. The conscious-accept
that any glob is defeatable by `sh -c`/env-prefix wrapping (layer-c guards accidents, not a
sandbox) is documented and correct. No defect.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes. Reviewer must ALSO sanity-
check the deny list doesn't lock the coordinator out of sanctioned operations.
