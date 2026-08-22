---
brief: desk-containers/07
title: multi-desk control layer — tmux/tmuxinator evaluation + cross-platform config
wave: 4
depends: ["desk-containers/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [64]
schema: brief-v1
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#64 — the request (tmux and equivalents as a cheap way to control all the desks; win32 as well as mac; the aim is to fire up the docker/k8s pods)"
  - "docs/streams/desk-containers/spec.md — multi-desk control layer section + open question 8"
  - "containers/desk-run.sh (brief 04) — the per-desk launch path each pane invokes"
  - "freshness-checked 2026-08-22 @ b3a2067 — no control/multiplexer config exists in the tree"
why: >-
  Five desks means five terminals to start, find, and re-attach by hand. One
  multiplexer session naming a pane per desk turns "fire up the fleet" into a single
  command and gives one place to see every desk — cheaply, on macOS and Windows alike.
---

# Brief 07 — multi-desk control layer

## Context

files:
- `containers/control/EVALUATION.md` (planned) — the tmux/tmuxinator vs equivalents
  evaluation, with ONE recommendation per platform.
- `containers/control/desks.yaml` (planned) — declarative session definition
  (tmuxinator-style: one pane per desk running the launch script), or the recommended
  tool's equivalent layout file.
- `containers/control/desk-grid.sh` (planned) — fallback plain-tmux launcher (no
  tmuxinator dependency): builds the session with a pane per desk; `--dry-run` prints
  the per-pane commands.
- Win32 deliverable per the evaluation's recommendation (planned): a native config
  (Windows Terminal `wt` fragment / wezterm Lua / zellij layout) or a documented WSL2
  recipe in `containers/control/EVALUATION.md` (planned) — one committed, working answer.
- `docs/docker.md` — add the "control all desks at once" section.

facts:
- Candidates to evaluate (minimum): tmux, tmux+tmuxinator, zellij, wezterm, Windows
  Terminal panes. tmux does not run natively on win32; WSL2 changes that — the
  evaluation must state which host model (native win32 vs WSL2 + docker) the
  recommendation assumes (spec open question 8) and give the answer for BOTH mac and
  win32, choosing exactly one recommendation each.
- Every pane runs the brief-04 launch path (`containers/desk-run.sh <desk-name>` or its
  re-attach mode) — the control layer NEVER composes docker flags itself and NEVER
  reads or holds a credential; it inherits fail-closed behaviour from the script.
- Pane/window names are exactly the five desk names: intake-desk, worker-desk,
  pr-review-desk, verify-desk, the-desk.
- Detach/re-attach of the whole fleet must survive a terminal close (multiplexer
  session persistence) — that property is part of why a multiplexer beats five
  terminal tabs, and the evaluation must verify it per recommended tool.
- The evaluation is a factual deliverable: each claim about a tool (platform support,
  licence, config mechanism) carries the tool's version checked and the command used to
  check it, so a reviewer can re-run the check rather than trust the prose.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `containers/control/EVALUATION.md` (planned): the candidate table (platforms,
   install cost, config style, session persistence, licence), the mac recommendation,
   the win32 recommendation (native tool or WSL2 recipe), and the host-model assumption
   — each factual claim with its version-checked command.
2. Implement the recommended mac path: `containers/control/desks.yaml` (planned) +
   `containers/control/desk-grid.sh` (planned) fallback, one pane per desk invoking the
   brief-04 launch script, with `--dry-run` support in the script path.
3. Implement the committed win32 answer per the recommendation (config file or
   documented recipe).
4. Document the fleet workflow in `docs/docker.md`: start all, detach, re-attach, kill.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'win32' containers/control/EVALUATION.md` | exit 0; count ≥ 1 — the win32 answer is present |
| 2 | `for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do grep -q "$d" containers/control/desks.yaml \|\| exit 1; done` | exit 0 — every desk has a pane in the session definition |
| 3 | `shellcheck containers/control/desk-grid.sh` | exit 0 |
| 4 | `sh containers/control/desk-grid.sh --dry-run` | exit 0; prints exactly five launch lines, each containing `desk-run.sh` and one desk name (dereferencing row: the config drives the real launch path, not a parallel one) |
| 5 | `sh containers/control/desk-grid.sh --dry-run \| grep -ci -e pem -e 'sk-ant-' -e ghs_; test $? -eq 1` | exit 0 — the control layer surfaces no credential material; injection stays inside the launch script |
| 6 | `tmux -V` (or the recommended tool's version command, per EVALUATION.md) | exit 0; version printed matches the version recorded in EVALUATION.md (dereferencing row for the evaluation's platform claims on the runner's own platform) |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- One recommendation per platform (macOS, win32), committed and working — not a list of
  maybes; the host-model assumption stated.
- The control layer holds no credential and composes no docker flags — every pane goes
  through the brief-04 launch script, so **no secret in any image layer** and the
  brief-02 runtime-injection contract remain untouched by this layer (row 5).
- `docs/docker.md` regenerated with the fleet-control section (docs-regen item).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a layout/orchestration layer over the existing
launch script, holding no credentials and adding no privilege). Reviewer re-runs the
evaluation's version-check commands on at least one platform and confirms rows 4-5.
