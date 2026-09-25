---
brief: assay:assay:desk-supervision:04
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
schema: brief-v2
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
version: 1
id: 9896223f-e63f-4fe6-a7c9-f7e935a2e884
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

### Verification — 2026-09-25 (assay-verifier-app[bot], on-behalf-of human:ian) — VERIFY: FAIL — 6/9 pass, 3 fail (check-definition)

Non-implementer run at merged main `89042b8fcc7e777a38903b588e0403866c606a41`, darwin/arm64, offline
(`KUBECONFIG=/dev/null`). The table below is the execution witness written by `statusgen verifyrun`
(built from this tree), landed verbatim on a clean tree. No Verify row is `check:ci`, so no
network-isolated run applies; every row is decided on its direct result. The three witness
failures are check-definition defects: each row's Expect, read as written, is met (supplementary
rows below), but the witness cannot derive that verdict from the row as authored.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Hook' -count=1` | pass exit=0 | sha256:8d02dd622e4a | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHooksIgnoreItemTreeFile -v -count=1` | pass exit=0 | sha256:44e03377edf0 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHookTimeoutKills -v -count=1` | pass exit=0 | sha256:219953b34d67 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestHookEnvScrubsSecrets -v -count=1` | pass exit=0 | sha256:b31ac4dc3a4a | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run TestBeforeRunFailureAbortsAndReleases -v -count=1` | pass exit=0 | sha256:b8dcb65f157e | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOWORK=off go test ./cmd/deskwt/ -run 'TestAfterCreateRunsOnceForNewPath\|TestBeforeRemoveFailureStillRemoves' -v -count=1` | pass exit=0 | sha256:762bed05b872 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 7 | `test -f tools/desk/hooks.example.yaml && grep -c 'KUBECONFIG' tools/desk/hooks.example.yaml` | fail exit=0 | sha256:7de1555df0c2 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -rn -- '--hooks' tools/desk/cmd tools/desk/internal/deskkit/hooks.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/04` | fail exit=2 | sha256:860e09ac35c0 | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (on-behalf-of human:ian) (forge-identity) |

Supplementary hand-run rows for the three witness failures (same tree, same sha):

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---|---|---|---|---|
| 7 | `test -f tools/desk/hooks.example.yaml && grep -c 'KUBECONFIG' tools/desk/hooks.example.yaml` | output is `1` or more | exit 0; prints `4`. The Expect ("1 or more") is met; the witness reads the Expect as the literal line `1` and fails the row (no output line equals "1"). Check-definition defect, not an implementation one. | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `grep -rn -- '--hooks' tools/desk/cmd tools/desk/internal/deskkit/hooks.go \| wc -l` | output is `0` | exit 1 under `bash -o pipefail` (the witness shell), exit 0 without it; prints `0` both ways. grep exits 1 on zero matches, which is the passing condition here, and pipefail carries that exit out of the pipeline. The Expect (output `0`, no `--hooks` path argument) is met; the witness infers "exit 0" and fails the row. Check-definition defect. | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/04` | exit 0; no `DISPROVED` (run on the implementing branch) | On merged main with a clean tree: exit 2, COULD-NOT-CHECK — the brief is not in the diff against the merge base, so the run carries no evidence (the Expect itself says to run on the implementing branch). Re-run on the implementing branch head 1b72110b40cd (second parent of merge 56694aeee, PR #449) with `--base 0b5c75f9368f` (their merge base): exit 0; summary: 4 corroborated, 0 disproved, 2 unchecked (the common-clauses KUBECONFIG clause and the worker-desk skill residue, both unchanged in that diff). No DISPROVED. | 2026-09-25 | assay-verifier-app[bot] @ 89042b8fcc7e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Notes for the human gate (observations, not Verify rows):
- The shipped `after_create` default sets only `extensions.worktreeConfig`; the commit identity lines are comments (deployment-specific), and no credential helper is set, although the brief's facts and the file's own comment say it sets the credential helper and identity.
- The shipped `after_run` default records the run's end time, not its duration (the file says so).
- The shipped `before_run` default refuses only when KUBECONFIG names a readable file other than /dev/null. An unset KUBECONFIG passes, and kubectl then falls back to its default config path, so the hook does not enforce the offline envelope in that case.
- `statusgen --lint` raises a risk-files-crossread NOTICE on this brief: all four risk answers are no, but its declared path tools/desk/internal/deskkit/hooks.go matches the security-path trigger tools/desk/internal/deskkit/. The gate is already human, so this affects the recorded risk answers, not the gate.

Risk-bearing values. The brief carries risk metadata (all four answers no, irreversible no) and is gate: human. The enumeration covers the implementing diff (hooks.go, hookprocess_unix.go, the deskdispatch before_run step, hooks.example.yaml) plus the literals named in the brief's facts:
1. `isSecretEnvName` scrub shapes: prefix "GH_" @ tools/desk/internal/deskkit/hooks.go:304; substrings "TOKEN", "SECRET", "PEM" @ hooks.go:307-309 (case-insensitive).
2. `hooksFileName = "hooks.yaml"` @ hooks.go:147, joined only to `StateDir()`, which resolves `filepath.Join(home, ".config", "assay")` @ tools/desk/internal/deskkit/killswitch.go:72.
3. `HookFatalOnFailure`: fatal = after_create, before_run @ hooks.go:143.
4. `ExitUnverifiable = 6` @ tools/desk/internal/deskkit/exitcodes.go:31 (the before_run abort exit; pre-existing constant).
5. SIGKILL to the hook's process group @ tools/desk/internal/deskkit/hookprocess_unix.go:38.
6. `maxHookOutputBytes = 2000` @ hooks.go:71.
7. `DefaultHookTimeoutMS = 60000` @ hooks.go:66; non-positive falls back to the default @ hooks.go:112; `timeout_ms: 60000` @ tools/desk/hooks.example.yaml:37.
8. `hookWaitDelay = 2 * time.Second` @ hooks.go:77.
9. Shell `"/bin/sh"` @ hooks.go:232; fixed env names ASSAY_RUN_KEY / ASSAY_WORKTREE / ASSAY_REPO / ASSAY_ROLE / ASSAY_HOOK @ hooks.go:290-294.
10. before_run default refusal test `[ -r "$KUBECONFIG" ] && [ "$KUBECONFIG" != /dev/null ]` and `exit 1` @ hooks.example.yaml:52,54.

Ranked by irreversibility: 1 (a credential that reaches a hook's log or child process is a disclosure; an edit does not undo it, the credential has to be rotated), then 2 (the source rule decides whose shell runs under the desk's credentials), then 3 and 4 (failure classes; reversible by edit), then 5, 6, 10 (reversible), then 7, 8, 9 (operational knobs, reversible, no derivation needed).

RISK-VALUE: NAMED, NOT DERIVED — isSecretEnvName shapes = {prefix "GH_"; contains "TOKEN", "SECRET", "PEM"} @ tools/desk/internal/deskkit/hooks.go:304-309 — the set matches the brief's facts exactly, but nothing derives that these four shapes cover every credential a desk process carries. Names such as `*_KEY` (an API key variable), `*PASSWORD*`, `*CREDENTIAL*`, or a `GIT_CONFIG_*` pair that carries a credential helper pass through to the hook unscrubbed. The scrub is also name-only: hook stdout and stderr go to the audit detail with control characters stripped and no content scrub. Open question for the human: is the four-shape set the intended complete control, or should it widen (or become an allow-list of passed-through names) before sign-off? I could not derive it because the full set of credential-bearing variable names in a desk process is deployment configuration that this tree does not enumerate.
RISK-VALUE: DERIVED — hooksFileName = "hooks.yaml" @ tools/desk/internal/deskkit/hooks.go:147 under StateDir = $HOME/.config/assay @ tools/desk/internal/deskkit/killswitch.go:72 — this is the kill switch's trust root: $HOME-owned and not relocatable by env or flag (the only override is an unexported test variable). The item's tree is untrusted content, so the one directory an agent or PR head cannot write through the tools is the right home. The loader takes no path argument (row 8 prints 0), and row 2's negative control shows a hooks.yaml in cwd, worktree or repo root is never read.
RISK-VALUE: DERIVED — HookFatalOnFailure = after_create, before_run @ tools/desk/internal/deskkit/hooks.go:143 — mirrors Symphony §5.3.4 exactly: the two pre-work moments abort because the work has not started (a failed setup must not run an agent in a bad envelope), and the two post-work moments only log because the outcome already stands and a cleanup failure must not strand a worktree. Row 5 shows the before_run abort releases the claim.
RISK-VALUE: DERIVED — ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:31 — the brief's Task fixes exit 6, and 6 is the toolset's existing could-not-proceed class, so the before_run abort reuses the one exit that callers already treat as "no dispatch happened" rather than adding a new code.

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
