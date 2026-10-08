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
  code. A line that worked before still does what it did; the lines that now run and used to be
  refused are listed under "What differs" and "Beyond the ruled list".
- **`--cells-root`.** Still accepted before the verb. It is now also accepted after it.
- **Raw entrypoints.** `model-policy`, `cache-run` and `container-run` receive the tokens after
  the verb verbatim; a flag-shaped token after one of these verbs belongs to the verb. The
  cells-root selector in front of the verb is stripped first, in any of its spellings (separate
  value, `=` form, single or double dash), and then the verb sees what follows. The hook verb keeps
  its own exit codes (a parse failure is 2, never the usage code 3).
- **`scratch ... run` command boundary.** Everything after `scratch <cell> <action>` and the
  verb's own flags is the command, with or without `--`: the first word that is not a flag or a
  flag's value starts it, and nothing after that word is parsed as a `scratch` flag, `--help`,
  `--version` or `--cells-root`. A command that itself begins with `-` needs `--` before it, as
  it always did. `TestScratchRunArgvBoundary` runs a child whose arguments spell those flags and
  checks the argv it receives.
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

## Beyond the ruled list

The migration also widens what the parser accepts in four ways. Each was found in review, each is
pinned by a recorded transcript of the pre-migration binary in the same parity test, and the
recorded decision does not list them: they are stated here so that no reader takes the table above
for the whole surface, and a maintainer who prefers the narrower surface can have them refused.
None of them can reach a refusal in the domain code, which still runs after the parse.

| Case key | Before | Now |
|---|---|---|
| `flags-before-positionals/0`, `flags-before-positionals/1`, `flags-before-positionals/2`, `flags-before-positionals/3` | a flag ahead of, or between, positionals (`scratch <cell> --apply sweep`, `cadence <cell> --confirm-stopped recover <role>`, `desk <cell> --model m <role>`, `show --harness h <cell>`) was refused as an unknown operation, role or cell, exit 3 | the line runs and does what its reordered form does |
| `single-dash-outside-scratch/0`, `single-dash-outside-scratch/1` | `-model x` and `-harness x` after the positionals were taken as an extra positional (a config-directory path, or an unexpected argument), exit 3 | the single-dash spelling of a long flag is that flag on every non-raw verb |
| `single-dash-outside-scratch/2`, `single-dash-outside-scratch/3` | `-version` and `version extra` were unknown verbs, exit 3 | `-version` prints the version, and `version` ignores extra words |
| `cells-root-spellings/0`, `cells-root-spellings/1`, `cells-root-spellings/2`, `cells-root-spellings/3` | only the separated `--cells-root <abs>` before the verb was a selector; `--cells-root=<abs>` and `-cells-root <abs>` were unknown verbs, exit 3 | both select the registry on every verb; a raw verb (`cache-run`, `model-policy`, `container-run`) strips the `=` form exactly like the separated one and then sees the tokens after it, so the hook's own checks run (a relative directory is refused by the hook, exit 2) |
| `cells-root-spellings/4` | `-cells-root relative model-policy hook ...` was an unknown verb, exit 3 | the selector is applied like the double-dash form (a relative path is refused with the absolute-path message), and on the hook verb every failure exits 2 |
| `completion-entrypoints/0`, `completion-entrypoints/1`, `completion-entrypoints/2` | `__complete` and `__completeNoDesc` were unknown verbs after the roster echo, exit 3 | the same refusal as any unknown verb (`unknown command "__complete" for "cellctl"`, exit 3, no echo); the parser's hidden completion entrypoint is replaced so it never answers |

One ordering is also held: the `desk` role is validated before any flag, as the old parser did, so
a bad role is refused first whatever flags follow (`refusal-order/0` replays it unchanged).

## Other deliberate differences

- `--flag=value` and a bare `--` terminator are now accepted by every non-raw verb.
- `-h` means help. A single-dash token that is not a known long flag is an error rather than a
  config-directory positional; a single-dash token that is a known long flag is that flag
  (`single-dash-outside-scratch`, above).
- No roster echo is printed for `help`, `-h`, `--help`, `--version`, `version` or a parse failure.
- `cadence recover` without a role is refused.
- `show` ignores `DESK_MODEL_OVERRIDE` in the environment, exactly as before: it reports pins, not
  one-run overrides.
- Environment names match the platform's own rule: case-insensitive on Windows, exact elsewhere.

## Co-execution notes for the human gate

Nothing in this migration changes who may do what, which credentials a launched window sees, or
what reaches a forge. The decision recorded for the migration concerns only the operator-visible
differences above.
