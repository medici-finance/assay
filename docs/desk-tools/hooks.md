# Lifecycle hooks — `after_create` / `before_run` / `after_run` / `before_remove`

The per-run envelope every dispatched agent is asked to remember — `export
KUBECONFIG=/dev/null`, set the worktree-local credential helper and commit identity, clean
up on removal — used to be prose spread across a prompt kit, a skill body and a role-init
verb, and nothing ran at run-*end* at all. Lifecycle hooks make those four moments a
declarative, **checked** configuration (mirroring OpenAI Symphony's hook block) instead of a
sentence.

## The source rule (the single point of failure)

Hooks are read from **one** place: the desk-tools **state directory**,
`~/.config/assay/hooks.yaml` — the same `$HOME`-anchored directory the kill switch and the
audit log live in. They are read from **nowhere else**:

- there is **no** hooks-path flag on any tool, and `LoadHooks` takes no path argument;
- a `hooks.yaml` placed inside a worktree or a repo root is **never** read.

This is deliberate. The state directory is not operator-relocatable (relocating it would
move the kill switch too), and the item's own tree is **untrusted** content — an agent, or
an untrusted PR head, that could drop a `hooks.yaml` the tools would execute is exactly the
surface this rule closes. The negative control — a `hooks.yaml` in the item tree is inert —
is the layer behind the rule, pinned by `deskkit`'s `TestHooksIgnoreItemTreeFile`.

Absent the file, **every hook is a no-op** and the tools behave exactly as before hooks
existed. A file the loader cannot read or parse is a fail-closed refusal (exit 6), never
silently treated as empty.

## The four moments and their failure classes

| Hook | Fires at | Tool | Failure class |
|------|----------|------|---------------|
| `after_create` | a **new** worktree was created | `deskwt add` | **fatal** — aborts creation; the worktree is rolled back |
| `before_run` | after the worktree is prepared, before the agent prompt is emitted | `deskdispatch` | **fatal** — aborts the attempt (exit 6), emits no prompt, **releases the claim** |
| `after_run` | an attempt ended (claim released or landed) | `desksupervise` | **logged** — the run's result stands |
| `before_remove` | before a worktree is deleted | `deskwt remove` / `prune` | **logged** — the deletion proceeds |

`before_run` releases the claim on failure because it is the one lifecycle point *after* the
durable claim: a failed hook that left the claim held would wedge the item behind a
dispatcher that never dispatched.

## Schema

```yaml
# ~/.config/assay/hooks.yaml
timeout_ms: 60000        # per-hook wall-clock budget; non-positive ⇒ 60000 (Symphony default)
after_create:  "…shell…"
before_run:    "…shell…"
after_run:     "…shell…"
before_remove: "…shell…"
```

Unknown keys are ignored (a newer file will not break an older binary). An empty or absent
field is a no-op. The file is read fresh on **every** invocation — the desk verbs are
one-shot, so an edit applies to the next run with no restart, and `desksupervise run
--interval` re-reads per tick.

## Environment

Every hook runs via `/bin/sh -c` with **cwd = the worktree**, and sees a fixed set of
variables on top of the caller's environment:

| Variable | Value |
|----------|-------|
| `ASSAY_RUN_KEY` | the item / claim key this run is for |
| `ASSAY_WORKTREE` | the worktree path (also the cwd) |
| `ASSAY_REPO` | `owner/name` |
| `ASSAY_ROLE` | the dispatched agent class / desk role |
| `ASSAY_HOOK` | the hook name that fired (so one script can dispatch on `$ASSAY_HOOK`) |

The caller's environment is passed through **minus every secret-shaped variable**: any name
containing `TOKEN`, `SECRET`, or `PEM`, or beginning `GH_`, is scrubbed. A hook is operator
shell, but it must never be the path an App token leaks into a log or a child process.
Pinned by `deskkit`'s `TestHookEnvScrubsSecrets`.

Hook stdout and stderr land (truncated) in the **audit line's detail** and **never** in an
agent's prompt.

## Dry run

`deskwt add`, `deskwt remove`, `deskwt prune`, `deskdispatch` and `desksupervise` all accept
`--dry-run`, which reports the hook plan — `HOOK <name>: would run` / `HOOK <name>: none` —
and touches nothing.

## Defaults

The shipped defaults live in [`tools/desk/hooks.example.yaml`](../../tools/desk/hooks.example.yaml).
To adopt them:

```sh
cp tools/desk/hooks.example.yaml ~/.config/assay/hooks.yaml
# then edit for your deployment — the commit identity is deployment-specific
```

They express: `after_create` — the worktree-local credential helper and App commit identity
(what `deskwt role-init` sets today, expressed once); `before_run` — refuse when
`KUBECONFIG` names a readable file (the checked form of the `KUBECONFIG=/dev/null` rule);
`after_run` — record the run's end to the audit line; `before_remove` — a documented empty
slot.
