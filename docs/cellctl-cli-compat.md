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
  code. A line that worked before still does what it did. A line the old parser refused is still
  refused, by the shape check below when the new parser would otherwise have accepted it. The
  exceptions are the recorded decision entries, under "What differs" and "Other deliberate
  differences".
- **Command-line shape.** `cellctl [--cells-root <abs>] <verb> <positionals...> [flags]`: the
  selector is the separated `--cells-root <abs>`, and only as the first word; a verb's fixed
  positionals (its cell, role or action) come before its flags; the single-dash spelling of a long
  flag (`-apply`, `-max-age 1h`) is accepted by `scratch` only, which read its flags with the Go
  flag package. A line of any other shape is refused before anything runs: exit 3, the parser's
  wording, nothing on stdout and no roster echo (rows under "Refused shapes"). Every verb declares
  how many fixed positionals it takes, and a test plants a verb without that declaration and
  requires the guard to name it.
- **A relative selector** (`--cells-root <relative>`) is refused before anything else, on every
  line, help and version included: the roster echo, then
  `cellctl: --cells-root requires an absolute registry path and a command`, exit 3, as before. It
  is the one help-shaped line that reads the roster, and the selector is never applied.
- **An empty verb word, `-h` or `--help` in the verb's place** prints the usage, exit 0, and runs
  nothing, whatever follows (`-h --bogus`, `--help up <cell>`), as before.
- **Raw entrypoints.** `model-policy`, `cache-run` and `container-run` receive the tokens after
  the verb verbatim; a flag-shaped token after one of these verbs belongs to the verb, including a
  `--cells-root` (it reaches the child and is never applied as the selector). Only the separated
  selector in front of the verb is stripped. The hook verb keeps its own exit codes (a parse
  failure is 2, never the usage code 3). A line that is `model-policy hook ...` after the
  selector, the only form a launch installs, exits 2 whatever refuses it (decision entry 7); any
  other `model-policy` line keeps the exit it had.
- **Words read as words.** `cadence`, `cache` and `comms` compare the words after the verb
  exactly, as before: `--confirm-stopped` confirms only as the one word after the action (or
  `cadence`'s role), so a repeated, misplaced or `--`-escaped confirmation is refused with the
  verb's own message and nothing recorded. `check` and `deskd` parse no flags: `check` takes the
  cell, then a `CLAUDE_CONFIG_DIR`, and ignores the rest; `deskd` takes the cell and ignores the
  rest, a flag-shaped word included. A flag ahead of their cell is still refused.
- **A repeated `--kind`** on `desk`, `up` or `show` keeps its first value; a later one is skipped
  unread, as the old pre-scan did, so a word appended after a pinned kind never re-kinds a launch.
- **`scratch ... run` command boundary.** Everything after `scratch <cell> <action>` and the
  verb's own flags is the command, with or without `--`: the first word that is not a flag or a
  flag's value starts it, and nothing after that word is parsed as a `scratch` flag, `--help`,
  `--version` or `--cells-root`. A command that itself begins with `-` needs `--` before it, as
  it always did. A flag ahead of the action or the cell is refused, so it cannot move where the
  command starts. `TestScratchRunArgvBoundary` and `TestScratchRunFlagsBeforeVerb` run a child and
  check the argv it receives; the shared adapter also refuses any line whose opaque-argv command it
  did not reach by its own walk.
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

## Refused shapes

The old parser refused each of these lines (exit 3) and the new parser on its own would have run
them. They are refused again, exit 3 and nothing run, by the shape check; what changed is the
wording and the missing roster echo, which decision entry 10 accepts for refusals of this kind and
entry 1 for the echo. The parity test replays each against the recorded transcript of the
pre-migration binary.

| Case key | Line | Before (exit 3, after the echo) | Now (exit 3, no echo) |
|---|---|---|---|
| `flags-before-positionals/0` | `scratch <cell> --apply sweep` | `unknown scratch operation "--apply"` | `scratch: flag "--apply" comes before the command's positional arguments; flags follow them` |
| `flags-before-positionals/1` | `cadence <cell> --confirm-stopped recover <role>` | `cadence: unknown role "recover"` | the same "comes before" refusal |
| `flags-before-positionals/2` | `desk <cell> --model m <role>` | `unknown role '--model'` | the same "comes before" refusal |
| `flags-before-positionals/3` | `show --harness h <cell>` | `no cell '--harness' ...` | the same "comes before" refusal |
| `single-dash-outside-scratch/0` | `desk <cell> <role> -model x` | `CLAUDE_CONFIG_DIR not a directory: ...` | `desk: single-dash flag "-model" (only scratch takes that spelling; write --model)` |
| `single-dash-outside-scratch/1` | `show <cell> -harness x` | `show: unexpected argument '-harness'` | the same single-dash refusal |
| `single-dash-outside-scratch/2` | `-version` | `unknown verb '-version' (try --help)` | `unknown command "-version" for "cellctl"` |
| `single-dash-outside-scratch/3` | `version extra` | `unknown verb 'version'` | `unknown command "extra" for "cellctl version"` |
| `cells-root-spellings/0`, `cells-root-spellings/1`, `cells-root-spellings/2` | `--cells-root=<abs> <verb>`, `-cells-root <abs> <verb>` | `unknown verb '--cells-root=...'` / `unknown verb '-cells-root'` | `unknown command "..." for "cellctl"` |
| `cells-root-spellings/3`, `cells-root-spellings/4` | the same spellings in front of `model-policy hook ...` | the same unknown verb, exit 3 | the same refusal, but exit **2**: a refused line that names the hook verb exits with the hook's blocking code (decision entry 7, below) |
| `completion-entrypoints/0`, `completion-entrypoints/1`, `completion-entrypoints/2` | `__complete ...`, `__completeNoDesc ...` | `unknown verb '__complete'` | `unknown command "__complete" for "cellctl"`; the parser's hidden completion entrypoint is replaced so it never answers |

Also refused, with no recorded case of its own: a `--cells-root` anywhere after the verb (`ls
--cells-root <abs>`), a flag or a lone `-` in the verb's place (`-- ls`, `--bogus`, `- ls`), a
flag ahead of the cell of `check` or `deskd` (`check --bogus <cell>`), `help` with more than one
word or with a hidden or unknown verb (`help __complete`, `help completion`), and `version` with
any word but `-h`/`--help`. `TestCLILegacyShapeRefusals` and `TestCLILegacyWordsRestored` pin
these and the lines under "What did not change" that read as they did.

One ordering is also held: the `desk` role is validated before any flag, as the old parser did, so
a bad role is refused first whatever flags follow (`refusal-order/0` replays it unchanged).

## Other deliberate differences

- `--flag=value` and a bare `--` terminator are now accepted by every non-raw verb (decision
  entry 4): `cadence <cell> recover <role> --confirm-stopped=true` confirms, and
  `cadence <cell> status <role> --` and `check <cell> --` run. A `--confirm-stopped=<value>` in any
  other place, or with a false value, is refused as before.
- `-h` and `--help` after the verb mean help (decision entry 9), `check <cell> --help` and
  `deskd <cell> -h` included. On a verb that parses flags, a single-dash token that is not a known
  long flag is an error (decision entry 4); a single-dash long flag outside `scratch` is refused
  (above). `check` and `deskd` parse no flags, so such a word after their cell is still theirs.
- No roster echo is printed for `help`, `-h`, `--help`, `--version`, `version` or a parse failure
  (decision entry 1).
- `cadence recover` without a role is refused (decision entry 4).
- `show` ignores `DESK_MODEL_OVERRIDE` in the environment, exactly as before: it reports pins, not
  one-run overrides.
- Environment names match the platform's own rule: case-insensitive on Windows, exact elsewhere.

These seven cannot be restored by a refusal. Each is one of decision entries 5 to 11, which the
driver's second answer on #2240 accepted (`1 — DR-cellctl-cobra`, accepting all seven items of
the question put to it). Entry 10's "Before" was misstated when it was put to the driver; the
corrected before for `desk` (exit 1, not 3) was put again and the driver's third answer on #2240
accepted exit 3 (`1 — DR-cellctl-cobra`).

| Decision entry | Line | Before | Now |
|---|---|---|---|
| 5 | `ls --bogus`, `status <cell> --bogus` | exit 0, the word ignored | exit 3, `unknown flag` |
| 6 | `ls --cells-root <abs>`, `ls --cells-root=<abs>` | exit 0, lists the default registry and ignores the words | exit 3, refused |
| 7 | a refused or relative selector in front of `model-policy hook ...` (`--cells-root relative`, `--cells-root=<abs>`, `-cells-root <abs>`, a repeated selector) | exit 3, which the hook's caller reads as non-blocking | exit 2, the hook's blocking code (fail-closed) |
| 8 | `help`, `help <verb>` | `unknown verb 'help'`, exit 3 | the usage or the verb's help, exit 0 |
| 9 | `<verb> --help`, `<verb> -h` (for example `desk --help`, `ls -h`) | the word taken as a cell or role, or ignored | the verb's help, exit 0 |
| 10 | `-help`, `cadence <cell> status --bogus` and similar refusals | exit 3 with the tool's own wording after the echo | exit 3 with the parser's wording and no echo |
| 10 | `desk -help`, `desk --bogus`, `desk -x` (a flag-shaped word where `desk` takes its cell) | exit **1**, the `desk` usage line after the echo | exit **3**, the parser's wording, no echo |
| 11 | `scratch <cell> <action> --nosuch` and other `scratch` flag-parse failures except `--max-age bogus` | exit 2 (Go flag package) | exit 3 |

One residual difference has no entry of its own. `scratch <cell> <action> -h` (or `--help`,
`-help`) still loads the cell and its scratch policy first and exits 2 with nothing on stdout, as
before, but the usage it prints on stderr is the verb's help, not the Go flag package's flag list,
and the roster echo is not printed before it (decision entry 1). A help flag before the action
(`scratch <cell> -h`) is the verb's help, exit 0 (decision entry 9).

## Co-execution notes for the human gate

Nothing in this migration changes who may do what, which credentials a launched window sees, or
what reaches a forge. The decision recorded for the migration concerns only the operator-visible
differences above.
