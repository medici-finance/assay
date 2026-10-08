# `cellctl` command-line compatibility

`cellctl` parses its command line with Cobra and resolves its per-run settings (`--model`,
`--cadence`, `--tick-budget`, `--session`, `--id`) through the shared desk adapter. This page lists
every way the operator-visible surface differs from the hand-written parser it replaced. The
generated command list is [cellctl-cli-reference.md](cellctl-cli-reference.md).

The rule the migration held to: a command line that worked before still works and does the same
thing, every refusal still refuses (in the domain code, below the parser), and anything that
differs is listed here. The non-help parity test replays a recorded transcript of the pre-migration
binary over every verb and refusal and fails on any difference that is not one of the rows below;
it also fails when a listed row stops differing, so this table cannot go stale in either direction.

## What did not change

- **Verbs, arguments and flags.** Every verb and flag keeps its name, its meaning and its exit
  code. `-flag value` (the Go single-dash spelling) is still accepted everywhere a long flag is.
- **`--cells-root`.** Still accepted before the verb. It is now also accepted after it.
- **Raw entrypoints.** `model-policy`, `cache-run` and `container-run` receive their argv verbatim;
  a flag-shaped token after one of these verbs belongs to the verb. The hook verb keeps its own
  exit codes (a parse failure is 2, never the usage code 3).
- **Setting precedence.** For `--model`, `--cadence` and `--tick-budget`: the flag, then the
  environment (the process environment with `cell.env` overlaid, so `cell.env` wins over the
  process), then the cell's own pin or default. An empty environment value counts as unset; an
  explicitly empty flag value is refused with the same message as before.
- **No new settings sources.** No setting reads a config file or an environment name it did not
  read before; a test fails if a binding opens one.
- **Admission.** The Opus refusal for `the-desk`, the role, harness and cockpit checks, the
  model-policy and `--set` refusals, and the `down` and `up` container refusals are unchanged and
  still live in the domain code. The parser grants nothing: a test calls the handlers with the
  parser bypassed and requires the same refusals.
- **`up` output.** The per-role command lines `up` prints still re-execute and reproduce the plan.

## What differs

| Case key | Before | Now | Why |
|---|---|---|---|
| `version-flag/0` | `--version` printed the version after the effective-configuration (roster) echo on stderr | `--version` prints the version and nothing else | Help and version are introspection: they read no cell, no credential and no roster. |
| `version-verb/0` | `version` printed the same echo first | `version` prints the version only | Same. |
| `unknown-verb/0` | an unknown verb printed `cellctl: unknown verb 'x' (try --help)`, exit 3, after the roster echo | Cobra's wording (`unknown command "x" for "cellctl"` and a pointer to `cellctl --help`), exit 3, no roster echo | Parse failures happen before any effect, so nothing is read or echoed. |
| `show-refusals/3` | `show <cell> --model` (no value) died with `cellctl: --model needs a value` after the echo, exit 3 | Cobra's `flag needs an argument: --model`, exit 3, no echo | A missing flag value is a parse failure. |
| `desk-refusals/6` | `desk <cell> <role> --model` (no value): same | same change | same |
| `set-refusals/9` | `set <cell> --harness` (no value): same | same change | same |
| `scratch/10` | `scratch <cell> sweep --max-age bogus` exited **2** (Go flag package, which also printed the whole flag list) | Cobra's `invalid argument "bogus" for "--max-age" flag: ...`, exit **3** | One usage exit code for the whole command; the only verb that used 2 for a parse failure was `scratch`. The hook verb keeps 2. |
| `cells-root/3` | `--cells-root` with no value: `cellctl: --cells-root requires an absolute registry path and a command` after the echo, exit 3 | Cobra's `flag needs an argument: --cells-root`, exit 3, no echo | Parse failure. |

## Other deliberate differences

- `--flag=value` and a bare `--` terminator are now accepted by every non-raw verb.
- `-h` means help. A single-dash token that is not a known long flag is an error rather than a
  config-directory positional.
- No roster echo is printed for `help`, `-h`, `--help`, `--version`, `version` or a parse failure.
- A `scratch ... run` command that begins with `-` needs a `--` before it.
- `cadence recover` without a role is refused.
- `show` ignores `DESK_MODEL_OVERRIDE` in the environment, exactly as before: it reports pins, not
  one-run overrides.
- Environment names match the platform's own rule: case-insensitive on Windows, exact elsewhere.

## Co-execution notes for the human gate

Nothing in this migration changes who may do what, which credentials a launched window sees, or
what reaches a forge. The decision recorded for the migration concerns only the operator-visible
differences above.
