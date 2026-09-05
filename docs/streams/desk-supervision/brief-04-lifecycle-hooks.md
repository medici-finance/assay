---
brief: desk-supervision/04
title: Lifecycle hooks — after-create / before-run / after-run / before-remove from config home
why: >-
  The per-run envelope is prose residue spread across a prompt kit, a skill body and a
  role-init verb: export KUBECONFIG=/dev/null, set the worktree-local credential helper,
  set the inline commit identity, clean up on removal. Every dispatched agent is asked to
  remember it, and nothing runs at run-end at all. Symphony puts the same four moments in a
  declarative hook block with a timeout and clear failure semantics; doing that here makes
  the envelope a checked configuration instead of a sentence, and gives brief 01's observer
  a run-end moment to fire.
wave: 1
depends: ["desk-supervision/01"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief adds a new execution surface: operator-configured shell that runs under the
  desk's credential envelope at four lifecycle points. The four risk answers are no, but
  the source rule (state directory only, never the item's tree, no path override) and the
  env scrub are controls a human should confirm before they exist, not after.
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §5.3.4 (hooks: after_create, before_run, after_run, before_remove; timeout_ms default 60000; after_create/before_run failure aborts, after_run/before_remove failure is logged) and §15.4 (hook script safety) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md line ~57 — the KUBECONFIG=/dev/null rule is prompt prose today"
  - "tools/desk/cmd/deskwt/main.go — role-init sets the worktree-local App commit identity; add/remove/prune are the worktree lifecycle verbs; no hook seam exists"
  - "tools/desk/internal/deskkit/killswitch.go — the state directory is resolved from $HOME only and is deliberately not operator-relocatable, because the kill switch lives there; hooks inherit that property by living there too"
  - "freshness-checked 2026-09-02 @ 30c9934"
exec-tier: strong
exec-tier-why: >-
  (c): hooks execute shell under the desk's credential envelope. The source rule (state
  directory only, never the item's tree) and the failure semantics are safety plumbing a
  subtle slip would silently weaken while every test still passes.
consumers:
  - "tools/desk/cmd/deskwt/deskwt.go add / remove: fixed-here (after_create on add; before_remove on remove — the verbs live in deskwt.go, main.go is only the router)"
  - "tools/desk/cmd/deskwt/prune.go: fixed-here (before_remove on each prune removal)"
  - "tools/desk/cmd/deskdispatch/dispatch.go: fixed-here (before_run between worktree-create and prompt-emit — the dispatch flow lives in dispatch.go, not main.go)"
  - "tools/desk/cmd/desksupervise/actions.go: fixed-here (after_run when a claim is released or landed by the observer — the runAction seam lives in actions.go, not main.go)"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md KUBECONFIG clause: fixed-here (the clause stays as the agent-facing rule; the shipped before_run hook makes it checked, not remembered)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Boot 'two residues': follow-up desk-supervision/04 (the residue paragraph shrinks to a pointer at the hooks file in the implementation PR, after the hooks are proven)"
---

# Brief 04 — Lifecycle hooks

## Context

files:
- `tools/desk/internal/deskkit/hooks.go` (new) — `LoadHooks()` from `<StateDir>/hooks.yaml`
  only; `RunHook(name, env)` with timeout and the per-hook failure class.
- `tools/desk/internal/deskkit/hooks_test.go` (new).
- `tools/desk/cmd/deskwt/main.go` — `add` runs `after_create` on a newly created path
  only; `remove` and `prune` run `before_remove` before deletion.
- `tools/desk/cmd/deskdispatch/main.go` — new step between worktree-create and
  prompt-emit: `before_run`; failure ⇒ exit 6, no prompt emitted, claim released.
- `tools/desk/cmd/desksupervise/main.go` (planned) — `after_run` on release / land.
- `tools/desk/hooks.example.yaml` (new) — the shipped defaults, documented.
- `docs/desk-tools/` — a short hooks page; `tools/desk/README.md` row updates.

single-point-of-failure: the source rule — hooks load from the state directory and from
nowhere else. Behind it: the loader refuses any path argument at all (there is no
`--hooks FILE`), and `deskkit` tests assert that a `hooks.yaml` placed inside a worktree
or repo root is never read (the negative control is the layer).

facts:
- Hook names and semantics mirror Symphony exactly: `after_create` (new worktree only;
  failure aborts creation), `before_run` (each attempt, after worktree preparation;
  failure aborts the attempt), `after_run` (each attempt end, any outcome; failure logged),
  `before_remove` (before deletion; failure logged, deletion proceeds). `timeout_ms`
  default 60000 applies to all; non-positive ⇒ default.
- Hooks run via the shell with cwd = the worktree and a fixed env: `ASSAY_RUN_KEY`,
  `ASSAY_WORKTREE`, `ASSAY_REPO`, `ASSAY_ROLE`, `ASSAY_HOOK`, plus the caller's env
  minus any variable whose name matches `*TOKEN*`, `*SECRET*`, `*PEM*`, `GH_*`. Hook stdout
  and stderr go to the audit line's detail (truncated), never to the agent prompt.
- Shipped defaults (`hooks.example.yaml`): `after_create` — worktree-local credential
  helper and App commit identity (what `deskwt role-init` does today, expressed once);
  `before_run` — refuse when `KUBECONFIG` names a readable file (`exit 1` with reason);
  `after_run` — record run duration to the audit line; `before_remove` — nothing (a
  documented empty slot).
- Absent `hooks.yaml` ⇒ every hook is a no-op; the tools behave exactly as today.
- Reload: the file is read at each invocation (these are one-shot verbs), so an edit
  applies to the next run with no restart; `desksupervise run --interval` re-reads per
  tick.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `hooks.go`: schema `{after_create, before_run, after_run, before_remove: string;
   timeout_ms: int}`; unknown keys ignored; loader resolves `<StateDir>/hooks.yaml` only.
   `RunHook` returns `(ran bool, err error)`; the caller applies the per-hook failure
   class. Env scrubbing per the facts.
2. Wire the four call sites (deskwt add/remove/prune, deskdispatch before_run,
   desksupervise after_run). `--dry-run` on each prints `HOOK <name>: would run` /
   `HOOK <name>: none`.
3. Ship `hooks.example.yaml` with the defaults and a header stating the source rule.
4. Tests: loader ignores a `hooks.yaml` in cwd/worktree/repo root (negative control);
   timeout kills a sleeping hook and reports it; before_run failure aborts deskdispatch
   with exit 6 and releases the claim; after_run failure is logged and exit stays 0; env
   scrub drops a `GH_TOKEN` set in the caller's env.
5. Docs page + README rows.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Hook' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHooksIgnoreItemTreeFile -v -count=1` | exit 0; output contains `--- PASS: TestHooksIgnoreItemTreeFile` |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHookTimeoutKills -v -count=1` | exit 0; output contains `--- PASS: TestHookTimeoutKills` |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHookEnvScrubsSecrets -v -count=1` | exit 0; output contains `--- PASS: TestHookEnvScrubsSecrets` |
| 5 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run TestBeforeRunFailureAbortsAndReleases -v -count=1` | exit 0; output contains `--- PASS: TestBeforeRunFailureAbortsAndReleases` |
| 6 | `cd tools/desk && GOWORK=off go test ./cmd/deskwt/ -run 'TestAfterCreateRunsOnceForNewPath\|TestBeforeRemoveFailureStillRemoves' -v -count=1` | exit 0; output contains two `--- PASS:` lines |
| 7 | `test -f tools/desk/hooks.example.yaml && grep -c 'KUBECONFIG' tools/desk/hooks.example.yaml` | output is `1` or more |
| 8 | `grep -rn -- '--hooks' tools/desk/cmd tools/desk/internal/deskkit/hooks.go \| wc -l` | output is `0` (no path argument exists — the source rule has no override) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/04` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "an untrusted head ships a hooks.yaml and it runs" → rows 2, 8;
"a hung hook wedges the dispatcher" → row 3; "a hook leaks the App token into a log" →
row 4; "before_run fails and the claim is left held" → row 5; "after_create re-runs on an
existing worktree and clobbers identity" → row 6.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
