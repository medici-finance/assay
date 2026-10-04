# CLI contract — Cobra and Viper across the tool suite

This is the shared contract every CLI migration brief in this stream (desktools-v2/16 and
desktools-v2/18–54) builds to, and the contract desktools-v2/17 enforces. desktools-v2/15
delivered it with the adapter (`tools/desk/internal/cli`), the routing registry
([cli-migration.json](cli-migration.json)) and the routing check
(`tools/desk/internal/clicontract`). Spec background: [spec.md](spec.md) §9.

## 1. Command contract

Every migrated command, in the desk module or a standalone module:

| Rule | Requirement |
|---|---|
| Help first | `-h`, `--help`, `help`, `help <verb>` and `<verb> --help` exit 0 before any config, roster, credential or network read, lock, worktree or child process, with config absent or malformed. Required-argument checks apply to execution, never to help. |
| Version first | `--version` on the root exits 0 under the same zero-effect rule. No `-v` shorthand (no legacy tool gave `-v` that meaning). A tool that printed a bare version keeps it through `VersionTemplate`. |
| Value spellings | `--flag=value` and `--flag value` both work. Values are delivered byte-identical: the adapter never cleans, splits, expands or re-separates a value, so Windows drive, UNC and relative paths reach the handler unchanged on every platform. |
| Go-flag spellings | `-flag value`, `-flag=value`, `-help` and `-version` are accepted only when the command opts into `GoFlagCompat` because its callers use them. The normaliser leaves shorthand clusters, values and everything after `--` untouched; a token that is the value of a preceding long flag is never rewritten. |
| Termination | `--` ends option parsing; everything after it is positional. |
| Repeated flags | A list-typed flag accumulates repeats in order. A scalar flag keeps the last value, as Go's flag package did. |
| Dash-leading values | A value starting with `-` is delivered intact in the `--flag=value` form; the fixtures also exercise it in the separate-argument form. |
| Global flags | A persistent flag works before or after the verb it applies to. |
| Unknown input | An unknown flag, a malformed value or a refused positional count exits 2 (`ExitUsage`) before any handler runs, and prints the error once with a pointer to `--help`. A flag marked secret never reaches argv, so a parse error cannot echo one. |
| Handler errors | A handler error keeps its own exit code (an `ExitCode() int` method, or the command's `ExitCode` mapping); otherwise it exits 1. Parse errors and handler errors never share a code by accident. |
| Completion | Cobra's completion command is disabled unless a brief adds it deliberately, with tests and docs. |
| One parser | The old parser is removed in the same change; two parsers never run. |

## 2. Configuration contract

| Rule | Requirement |
|---|---|
| Allowlist | A setting is resolved only from the sources its binding names: a flag spelling, explicit environment variable names, the config map the command's existing parser produced, a default. |
| Banned | `AutomaticEnv`, config-file search, remote config, the global Viper instance, and credential values on argv. A binding marked secret cannot declare a flag. |
| Fresh per call | Each invocation builds a new command tree and resolves through a new Viper instance; nothing is package-level mutable. |
| Order | Default order is flag > env > config > default. A key that needs a different order states it per key; an order that omits or repeats a declared source is refused at declaration. |
| Unset vs empty | A source present with an empty value resolves as set-to-empty from that source and beats every lower source; an absent source falls through. Handlers can ask which source won. |
| Undeclared keys | Keys in the config map that no binding declares are left to the command's own parser and ignored by the adapter. A declared key whose binding does not allow config is an error. |
| Environment lookup | A nil lookup means the process environment; only the names a binding lists are ever read. Windows environment names fold case, POSIX names do not; tests assert with `EnvFoldsCase`, never one platform's assumption. |
| Errors | Name the key and the source, never the value. |
| Authority | A value the adapter resolved is not thereby admissible. Roster, custody, cell and domain validators keep deciding; the adapter reads no file, credential or cell itself. |

A tool with no configuration binds only its declared flags and defaults.

For a legacy **first non-empty** environment fallback, declare each variable as
a separate binding and retain the existing fallback in the typed handler. An
`Env` list selects the first **present** variable, even when empty; it does not
implement a non-empty fallback. For example, bind `GH_TOKEN` and `GITHUB_TOKEN`
separately as secrets when preserving a command that skips an empty first token.
The owner's compatibility tests must cover both absent and present-empty values.

## 3. Fixture cases every migrated package copies

The reusable tables live in `tools/desk/internal/clicontract/fixtures.go`. Desk-module
packages import them; standalone modules cannot import the desk module, so they copy these
cases into their own tests, unchanged:

- **Help forms:** `-h`; `--help`; `help`; `help <verb>`; `<verb> -h`; `<verb> --help`. With
  Go-flag compatibility add `-help` and `<verb> -help`.
- **Version forms:** `--version`; with Go-flag compatibility add `-version`.
- **Malformed configs** that help and version must survive: absent; a non key-value file
  containing a NUL byte; an unterminated quote.
- **Path values** delivered byte-identical through every value spelling: POSIX absolute,
  POSIX with a space, POSIX relative with `..`; Windows drive, drive with spaces, UNC,
  backslash-relative, forward-slash drive; a dash-leading value; a value with `=` and `,`;
  a non-ASCII path.
- **Option forms:** long-equals, long-space; go-equals and go-space only with Go-flag
  compatibility.
- **Precedence matrix** for a key declared from all four sources (default `dflt`): nothing
  set → default; config only → config; env over config; flag over all; empty config beats
  the default; empty env beats config; empty flag beats env; unset env falls through to
  config.

## 4. Owner tests and completion

Each migrated package declares, in its own `cli_test.go`:

| Test | Proves |
|---|---|
| TestCLIHelpOffline | every help and version form under every malformed config, with effect counters at zero and no config read |
| TestCLIConfigFlow | each precedence-matrix row and unset-versus-empty case reaches the typed handler with the expected value and source |
| TestCLILegacyForms | every legacy spelling the consumers use, the path values, unknown-flag exits and `--` termination |
| TestCLIAdmissionBoundary | human-gated briefs only: refused inputs stay refused through Cobra AND when the domain check is called directly with the adapter bypassed |

`CLI_OWNER=desktools-v2/NN go test -run '^TestCLIOwnerMigrated$' ./internal/clicontract`
(from `tools/desk`) is each child's completion check: it fails unless every row the brief owns
is migrated, the registry validates, and the first three owner tests named above actually
run and pass in each migrated package's own module. Each child also carries a mutation row:
a muhar spec that plants a broken guard (help after a config read, an undeclared key
binding, a dropped legacy form, and for human-gated briefs a refusal moved into the
adapter) and requires every plant to redden.

## 5. Routing registry

[cli-migration.json](cli-migration.json), schema `cli-migration/v1`, routes every discovered
entrypoint. Discovery is independent of the registry: every Go `main` package directory and
every script launcher (`.sh`, `.bash`, `.ps1`, `.py`, `.mjs`, `.js`, or an extensionless file
with a `#!` line), skipping `.git`, `node_modules`, `vendor` and hidden directories other than
`.github`.

| Field | Meaning |
|---|---|
| executable, path, kind, module | what is invoked, where it lives, `go` or `script`, its Go module directory or interpreter |
| release | how it ships (release archive, plugin bundle, image, CI-only, or not shipped) |
| parser | the hand-rolled parser it uses today |
| complexity, basis | `simple` or `complex`, with the measured LOC, verbs and flags behind it |
| consumers | the files that mention the entrypoint most at the measured commit, up to six. A starting list, not an exhaustive one: re-enumerate every caller at pickup |
| owner, state | the owning brief, and `pending`, `migrated`, `retired` or `excluded` |
| class, reason | excluded rows only: `test`, `fixture`, `library`, `vendor` or `demo`, and why |
| migrated_to | migrated scripts only: the inventoried Go entrypoint the script now delegates to |

**States.** `pending` — owned, not migrated. `migrated` — the owner tests pass on the migrated
command; a Go row imports Cobra or the adapter, and a script row names a Go entrypoint that the
script itself still invokes. `retired` — removed, with its consumers, by the owner's merged
change. `excluded` — not an operator entrypoint.

**Exclusion rules** carry a class, a pattern and a reason; a rule that matches nothing is stale.
A shipped entrypoint (anything in the plugin bundle, a desk command the Makefile packages, or
a path the release workflow or Makefile names) can never be excluded. Test-named suites
(`x.test.sh`, `x_test.sh`) are tests even when a release gate runs them.

**Complexity.** A Go entrypoint is complex at 4000 LOC or more, 8 or more verbs, or 20 or more
flags; a script is complex at 1000 lines or more. Everything else is simple.

**Budget.** One owning brief takes one complex entrypoint alone, or at most five simple ones.
Every owner depends on desktools-v2/16, the reference adapter migration, and
desktools-v2/17 depends on every owner.

TestCLIInventory fails on an unrouted entrypoint, a stale or duplicate row, a stale rule, an
orphan owner, a missing edge, an over-budget owner, or an owner whose Verify rows select no
TestCLI test or use an empty `-run` selector.

## 6. Deliberate help changes (not legacy compatibility)

These are improvements every migration adopts. They are listed apart from compatibility so a
reviewer never mistakes them for regressions:

- Help is generated from the command tree, not hand-maintained usage strings.
- `<verb> --help` exits 0 with that verb's help, where some tools used to exit 1 or report an
  unknown flag.
- Help and version never read configuration or credentials first.
- Unknown-flag errors name the flag and point to `--help`.

Everything else a caller can observe (flag spellings, exit codes, output text and files,
refusals) stays as it was, unless the owning brief's compatibility report lists the change
and, for a human-gated brief, the sign-off approves it.
