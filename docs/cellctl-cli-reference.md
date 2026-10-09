# `cellctl` command reference

<!-- GENERATED from the command tree by TestCLIReferenceDoc; do not edit by hand. -->

Every command takes the cell as its first argument. The shape is
`cellctl [--cells-root <abs>] <verb> <positionals...> [flags]`: the registry selector is the
separated `--cells-root <abs>`, and only as the first word. Single-dash spellings of long flags
(`-apply`) are accepted by `scratch` only. Help (`-h`, `--help`, `help <command>`) and version
(`--version`, `version`) read no cell and no credential file, and read the roster only when the
line carries a relative `--cells-root`, which every line refuses after the roster echo. How
this differs from the pre-Cobra surface is in [cellctl-cli-compat.md](cellctl-cli-compat.md).

## `cellctl cache <cell> status|clean|recover`

report or clean a cell's managed Go caches

```
  --confirm-stopped                   recover only: confirm every cache consumer has stopped
```

## `cellctl cadence <cell> status|stop|resume|recover [role]`

inspect or control a house cell's role cadence

```
  --confirm-stopped                   recover only: confirm the prior harness and all its children have stopped
```

## `cellctl check <cell> [CLAUDE_CONFIG_DIR]`

audit the preconditions of a cell

```
  (no flags)
```

## `cellctl comms <cell> check|run|recover`

check, run or recover a cell's interim comms service

```
  --confirm-stopped                   recover only: confirm the prior gateway, drain and every owned child have stopped
```

## `cellctl defaults init|print`

print or create the machine-wide cell defaults file

```
  (no flags)
```

## `cellctl desk <cell> <role> [CLAUDE_CONFIG_DIR]`

boot one role window

```
  --cadence                  string   role cadence: a duration, or off
  --cockpit                  string   cockpit surface (auto|tmux|herdr|orca)
  --harness                  string   harness for this run (claude|codex|cursor)
  --kind                     string   cell kind for this run (k8s|house|container|scrubbed)
  --model                    string   model for THIS run only, passed through verbatim to the harness
  --provider                 string   provider for this run (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)
  --set                               persist every override given into cell.env
  --tick-budget              string   per-pass tick budget (a duration)
```

## `cellctl deskd <cell>`

stand the cell's persistent deskd

```
  (no flags)
```

## `cellctl down <cell>`

tear a cell's session down

```
  --cockpit                  string   cockpit surface (auto|tmux|herdr|orca)
  --keep-deskd                        leave the cell's deskd running
```

## `cellctl ls`

list the cells under the registry

```
  (no flags)
```

## `cellctl new <cell>`

scaffold a cell

```
  --cells-yaml               string   this cell's slice of cells.yaml
  --container-config         string   absolute JSON file for the native container runtime
  --deskd-app-id-var         string   github: apps.env variable holding the App id
  --deskd-app-pem            string   github: the deskd App PEM
  --forge                    string   github|gitlab
  --gitlab-api-base          string   gitlab: API base URL
  --gitlab-token-store       string   gitlab: role token store directory
  --group                    string   gitlab: group
  --kind                     string   k8s|house|container|scrubbed
  --launcher                 string   absolute operator-owned container launcher
  --orgs                     string   github: comma-separated orgs
  --port                     string   house: deskd port
  --repo                     string   checkout path (or repo id for a container cell)
  --repo-slug                string   scrubbed: <owner>/<repo>
  --roles                    string   space-separated roles
  --roots                    string   the DESK_ROOTS map: '<owner>/<repo>=<abs path>,...'
```

## `cellctl providers init`

create the shared providers.json

```
  (no flags)
```

## `cellctl scratch <cell> run|ack|sweep|inventory [flags] [-- command [args]]`

managed task scratch and evidence handoff

```
  --apply                             apply cleanup; default is dry-run
  --id                       string   owned task id
  --input                    stringArray explicit extra source-relative file needed by this task; repeatable
  --max-age                  duration diagnostic retention age (default from the cell's scratch policy)
  --max-bytes                int      retained diagnostic byte budget (default from the cell's scratch policy)
  --path                     string   legacy root for read-only inventory
  --receipt                  string   canonical evidence destination, after verified handoff
  --resumable                         retain for resumption (inactive task only)
  --revision                 string   source revision
  --session                  string   session identity
  --snapshot                          materialize all tracked files, never working-directory copies
  --snapshot-bytes           int      snapshot plus declared input byte limit
  --source                   string   source Git checkout (required for run)
  --task                     string   task identity
```

## `cellctl set <cell> KEY=VALUE... | <cell> <role> --model <m> | <cell> [flags]`

persist a change into a cell's cell.env

```
  --cockpit                  string   CELL_COCKPIT (auto|tmux|herdr|orca)
  --force                             write a KEY that is not a known cell.env key
  --harness                  string   harness (claude|codex|cursor)
  --kind                     string   CELL_KIND (k8s|house|container|scrubbed)
  --model                    string   model pin (needs a role)
  --provider                 string   CELL_PROVIDER
```

## `cellctl show <cell>`

print the effective per-run choices of a cell

```
  --cockpit                  string   cockpit surface (auto|tmux|herdr|orca)
  --harness                  string   harness (claude|codex|cursor)
  --kind                     string   cell kind (k8s|house|container|scrubbed)
  --model                    string   model
  --provider                 string   provider
```

## `cellctl smoke <cell>`

one-shot readiness probe of a scrubbed cell

```
  --harness                  string   harness to probe (claude|codex)
  --model                    string   model to probe with
```

## `cellctl status <cell>`

report a cell's session state

```
  (no flags)
```

## `cellctl up <cell> [CLAUDE_CONFIG_DIR]`

open one window per role in the resolved cockpit

```
  --automate                 string   orca only: schedule one automation per role (a 5-field cron string or a preset)
  --cadence                  string   role cadence: a duration, or off
  --cockpit                  string   cockpit surface (auto|tmux|herdr|orca)
  --harness                  string   harness for this run (claude|codex|cursor)
  --kind                     string   cell kind for this run (k8s|house|container|scrubbed)
  --model                    string   model for THIS run only, passed through verbatim to the harness
  --no-attach                         do not attach to the session after opening it
  --no-the-desk                       do not open the the-desk window
  --provider                 string   provider for this run (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)
  --set                               persist every override given into cell.env
  --tick-budget              string   per-pass tick budget (a duration)
  --with-the-desk                     open the the-desk window (the default; accepted for compatibility)
```

## `cellctl version`

print the release tag this copy ships at

```
  (no flags)
```

## Global flag

```
  --cells-root               string   absolute path of the cell registry to use for this run (overrides CELLS_ROOT)
```

## Internal entrypoints

These verbs are invoked by cellctl's own generated command lines and hooks. They take their
argv verbatim (no flag parsing by the command layer) and are not part of the operator surface.

- `cache-run`
- `container-run`
- `model-policy`
