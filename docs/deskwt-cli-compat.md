# `deskwt` command-line compatibility

`deskwt` parses its command line with Cobra through the shared desk adapter
(`tools/desk/internal/cli`). This page pins the surface the port started from, lists every
observable difference the port made, and names the lifecycle decisions it deliberately left
to their own owner. The contract it follows is
[cli-contract.md](streams/desktools-v2/cli-contract.md).

The rule the port held to: a command line an existing caller uses still parses and reaches
the same handler with the same values; every domain refusal still refuses in the handler,
with its old exit code; anything else that differs is listed under "What changed".

## Baseline

Source: `8cba3584f4a782cd1b54251a78ec9776d692db1b` (`origin/main` at pickup, 2026-10-25).
The old parser was one Go `flag.FlagSet` per verb behind a manual `switch` in `main.go`,
with `deskkit.TakeTraceFlag` stripping a global `--trace` first and `deskkit.HelpOnly`
short-circuiting single-token help.

**#2525 disposition.** Merged on 2026-10-20 as merge commit
`6c79ace7be7e77a6648d34d9174afd6453970c31`, so its command tree is in the baseline:
`maintain objects` (offline common-store inventory and bounded compaction), the prune-side
daily maintenance pass (`--no-maintenance`) and the temporary-ref custody. Its temp-ref
entrypoints live in `deskmerge` (`gitmaintenance.BeginTempRef`), not in `deskwt`; `deskwt
maintain --apply` takes `assay-maintenance.lock` and then `assay-maintenance-temprefs.lock`
in the common Git directory, in that order. The port keeps both locks in the handler, after
parsing; nested help never opens either (row-10 test, and a mutation that moves the lock to
parse time).

### Verbs

| Command | Positionals | Handler |
|---|---|---|
| `deskwt add <name>` | exactly 1 (`--` admits a dash-leading name, which the handler then refuses) | `runAdd` |
| `deskwt remove <path>` | exactly 1 | `runRemove` |
| `deskwt prune` | none | `runPrune` |
| `deskwt maintain` | none | `runMaintain` |
| `deskwt maintain objects` | none (`objects` must be the first word after `maintain`; only the root's persistent flags, `--trace` and the `--perf-*` flags below, with the value word a `--perf-*` flag takes, may stand between them) | `runObjectMaintenance` |
| `deskwt role-init [<role>]` | 0 or 1 | `runRoleInit` |
| `deskwt role-clean [<role>]` | 0 or 1 | `runRoleClean` |

There are no hidden or internal entrypoints. Cobra's `completion` command is disabled, and
`__complete` / `__completeNoDesc` are refused as unknown verbs (exit 2).

### Flags and defaults

Every verb setting in the table below is flag > literal default. There is no config file,
and no environment binding for any verb flag or for `--trace`; the precedence matrix in
`TestCLIConfigFlow` proves that a same-named environment variable or config key never
supplies a value, that an explicit empty string beats the default, and that an unset flag
keeps the baseline default. Scalar repeats keep the last value.

The one exception is the set of eight persistent `--perf-*` flags, which the shared adapter
binds on the root for every verb (cli-contract section 8, "Performance context"). They carry
optional local performance-attribution context only. Each resolves flag > named environment
variable > unset, and an explicitly empty flag clears an inherited environment value. They
never change what a verb does, which role or credential it runs under, or its exit code,
and an invalid value is not echoed. Attribution is recorded only when the opt-in
`ASSAY_TELEMETRY=1` is set and a profile (`tool` or `workflow`) is selected; with no profile
the recorder behaves as it did before.

| Command | Flag | Type | Default |
|---|---|---|---|
| all | `--trace` | bool, any position | false; absent leaves `DESK_TRACE` in charge, explicit wins over it |
| all | `--perf-profile` | string, persistent | unset; env `ASSAY_PERF_PROFILE` (`tool`, `workflow`; `off` or empty disables) |
| all | `--perf-session-id` | string, persistent | unset; env `ASSAY_PERF_SESSION_ID` |
| all | `--perf-agent-id` | string, persistent | unset; env `ASSAY_PERF_AGENT_ID` |
| all | `--perf-parent-agent-id` | string, persistent | unset; env `ASSAY_PERF_PARENT_AGENT_ID` |
| all | `--perf-parent-invocation-id` | string, persistent | unset; env `ASSAY_PERF_PARENT_INVOCATION_ID` |
| all | `--perf-loop-role` | string, persistent | unset; env `DESK_LOOP` (not an `ASSAY_PERF_*` name) |
| all | `--perf-role` | string, persistent | unset; env `ASSAY_PERF_ROLE` |
| all | `--perf-work` | string, persistent, repeatable | unset; env `ASSAY_PERF_WORK` (a JSON array); repeated flags replace the whole environment list |
| add | `--branch` | string | `""` (the worktree name) |
| add | `--detach` | bool | false |
| add | `--base` | string | `origin/main` |
| add | `--upstream` | string | `""` |
| add | `--role` | string | `""` |
| add | `--dry-run` | bool | false |
| add | `--lock-session` | string | `""` |
| add | `--adopt-run` | string | `""` |
| remove | `--dry-run` | bool | false |
| prune | `--repo` | string | `""` (current directory) |
| prune | `--interval` | duration text | `""` (one sweep) |
| prune | `--reclaim-stale-locks` | bool | false |
| prune | `--lock-ttl` | duration text | `0` |
| prune | `--reap-dead-sessions` | bool | false |
| prune | `--dry-run` | bool | false |
| prune | `--singleton-ttl` | duration text | `10m0s` |
| prune | `--no-singleton` | bool | false |
| prune | `--no-branches` | bool | false |
| prune | `--branches-only` | bool | false |
| prune | `--no-maintenance` | bool | false |
| prune | `--no-cache` | bool | false |
| prune | `--cells-root` | string | `""` (`$CELLS_ROOT`, then the per-user data directory) |
| prune | `--min-idle` | duration text | `""` (1h) |
| maintain | `--repo` | string | `""` |
| maintain | `--apply` | bool | false |
| maintain | `--min-interval` | duration | `6h` |
| maintain | `--timeout` | duration | `30s` |
| maintain objects | `--repo` | string | `""` |
| maintain objects | `--apply` | bool | false |
| maintain objects | `--dry-run` | bool | false |
| maintain objects | `--json` | bool | false |
| maintain objects | `--min-interval` | duration | `24h` |
| maintain objects | `--timeout` | duration | `2m` |
| role-init, role-clean | `--role` | string | `""` |
| role-init, role-clean | `--session` | string | `""` (`DESK_SESSION`, then `CLAUDE_SESSION_ID`) |
| role-init, role-clean | `--repo-root` | string | `""` (current directory) |
| role-init | `--no-fetch` | bool | false |

Prune's duration settings stay text at the parser: `runPrune` parses them and keeps every
bound and combination refusal (`--interval soon`, `--lock-ttl` without an opt-in,
`--interval` with `--dry-run`, both reclamation opt-ins together), still exit 5.
`maintain`'s and `maintain objects`' durations were typed Go-flag durations before and
are typed now; their range checks stay in the handlers (exit 5).

### Environment the domain code reads

Unchanged and not bound by the parser: `DESK_TRACE`, the kill switch and audit settings
read by `deskkit`, `CELLS_ROOT` and `XDG_DATA_HOME` (prune cache pass), `DESK_SESSION` and
`CLAUDE_SESSION_ID` (role session fallback), and the `ASSAY_TELEMETRY` opt-in. The
`ASSAY_PERF_*` variables and `DESK_LOOP` named in the flag table are read by the shared
performance resolver as the fallback for the `--perf-*` flags, not by `deskwt`'s own
bindings.

### Exit codes and output

Baseline: 0 ok, 3 disabled, 5 refused (including every parse error and unknown verb),
6 unverifiable. Now the same, plus 2 for usage errors under the shared contract. Success
output, refusal wording from the handlers, the audit row on execution, and the
`role-init` last-line path contract are unchanged.

### Callers replayed

`TestCLILegacyForms` replays each of these and checks the typed values that reach the
handler:

- `cellctl` (`cmd/cellctl/git.go`): `deskwt role-init --help` as a support probe (exit 0),
  then `deskwt role-init <role> --session <s>`.
- `deskboot`: `deskwt prune --repo <root>`.
- `deskdispatch`: `deskwt add <name> [--detach | --branch B] [--upstream ... --adopt-run KEY]
  --base <base> --role <role>`.
- Documented forms in `tools/desk/README.md` and the desk skills: every flag above in
  `--f v`, `--f=v` and single-dash Go-flag spellings (`-repo /r`, `-branch=b1`), `--`
  termination, repeated flags, and `--trace` before or after the verb, including between
  `maintain` and `objects` (`maintain --trace objects`, `maintain --perf-role worker objects`).
- Path values are passed through untouched, so Windows and POSIX paths reach the handler
  as given (`GOOS=windows go vet ./cmd/deskwt` keeps the package compiling there).

## What changed

Each row is a CLI-contract change, not a lifecycle change.

| Line | Before | Now |
|---|---|---|
| Unknown flag, malformed value, missing value, wrong positional count (`add`, `add a b`, `add a --detach=maybe`, `remove --force x`, `prune stray`, `maintain --timeout bogus`, `role-init a b`) | exit 5 after the kill switch, one `refused: bad flags` audit row | exit 2 before the kill switch and before any audit row, read or child process |
| Unknown verb (`deskwt frobnicate`) | exit 5, usage on stderr | exit 2, Cobra's unknown-command message |
| `maintain --apply objects` (the word after a flag) | exit 5 (`maintain requires no positional args …`) | exit 2: `objects` is a sub-verb only as the first word after `maintain`, not counting the root's persistent flags, `--trace` and `--perf-*` (`maintain --trace objects` runs the object pass, as before) |
| `--perf-profile`, `--perf-session-id`, `--perf-agent-id`, `--perf-parent-agent-id`, `--perf-parent-invocation-id`, `--perf-loop-role`, `--perf-role`, `--perf-work` before or after any verb (`deskwt --perf-role worker prune --dry-run`, `deskwt maintain --perf-role worker objects`) | not recognised: exit 5 (`unknown subcommand` before the verb, `flag provided but not defined` after it) | accepted on every verb, with the `ASSAY_PERF_*` / `DESK_LOOP` environment fallback in the flag table; attribution only, no effect on the verb's behaviour or exit code |
| `deskwt --version=true` | exit 5 (only the bare `--version` token answered) | exit 0, prints the version, like `--version` (`--version=false` still exits 5) |
| `deskwt <verb> --help`, `-h`, `-help`, `deskwt help <verb>` | the whole usage text on stderr, exit 0 | that verb's generated help on stdout, exit 0 |
| `deskwt --help`, `-h`, `help` | usage on stderr, exit 0 | generated root help on stdout, exit 0 |
| Help and version | ran after the roster echo and verb-activation check | run before either, with no config, roster, credential or network read, lock, worktree or child process |
| Activation check failure | the activation refusal on stderr, exit 6 | the same refusal, then `deskwt activation could not be established`, exit 6 |
| `--trace=true` / `--trace=false` | not recognised (only the bare `--trace` / `-trace` token was stripped) | accepted; an explicit `--trace=false` turns off an inherited `DESK_TRACE` |
| `--` | the Go flag package stopped at it per verb | ends flag parsing for every verb; `add -- -name` reaches the handler, which refuses the name (exit 5) |
| `add -x` (a dash-leading name without `--`) | exit 5 | exit 2 (unknown flag) |
| Bare `deskwt` | usage on stderr, exit 5 | root help on stderr, exit 5 (unchanged code) |
| A word beside `--version` (`deskwt --version extra`, `deskwt extra --version`) | exit 5 (`--version` answered only as the sole argument; otherwise an unknown verb) | exit 2 (unknown command), nothing on stdout; `--trace` beside it still prints the version |

## Left to the lifecycle owner

The port changes parsing only. These policy items belong to the worktree and repository
lifecycle owner set out in
[non-CI architecture proposal 14](proposals/non-ci-architecture/14-storage-forge-graph.md)
(merged in #2601), and each needs its own reviewed implementation:

- **Allocation records** — who allocated a worktree and for which run. Today `add
  --lock-session` and `--adopt-run` are the only hooks; the port passes them through.
- **Class-aware review retirement** — retiring review worktrees by class rather than by the
  dead-session and clean-and-pushed rules prune applies now.
- **Forge-bound branch eligibility** — the local-branch GC still uses its existing
  compare-and-delete protections; eligibility from forge state is not added here.
- **Lock retirement.** The approved target uses a lock's age only to *trigger* a holder
  check; age never proves the holder gone. The baseline `--lock-ttl` still treats a lock
  older than the TTL as stale when an opt-in is given. The port keeps that behaviour
  byte-for-byte in `lockreclaim.go`, does not reinterpret `--lock-ttl` in the binding layer,
  and does not claim the legacy behaviour satisfies the approved target.
- **Hook policy** — `after_create` (fatal) and `before_remove` (logged) keep their current
  semantics ([hooks.md](desk-tools/hooks.md)); hooks do not become a second pruner.
- **Deployment profiles** — sweep scheduling (`--interval`, the singleton and its TTL)
  stays in `runPrune`; nothing about scheduling moved into parsing or configuration.
