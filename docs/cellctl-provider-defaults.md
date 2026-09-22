# Shared provider defaults

The Go `cellctl` launcher can read one editable provider catalog for every local
`house` or `k8s` cell. It contains model IDs, effort levels and desk assignments;
credentials remain in the existing provider credential environment.

```sh
cellctl providers init
```

This creates `$CELLS_ROOT/providers.json` with Anthropic, GLM, Kimi and Codex
profiles. When `CELLS_ROOT` is unset, the file is under
`${XDG_DATA_HOME:-$HOME/.local/share}/assay/cells/`. Initialization refuses to
replace an existing file. Installing a newer binary does not overwrite it.
The checked-in seed is [providers.json](../tools/desk/cmd/cellctl/providers.json).

Once this file exists, cells without a complete `CELL_MODEL_POLICY` inherit it.
Without either file, existing legacy cell configuration keeps its behavior. The
shell implementation does not support the shared catalog; use the shipped Go binary.
Container and scrubbed cells never inherit host provider defaults.

## One model edit, all inheriting desks

The catalog uses the existing four-tier model-policy ladder:

| Tier | Claude alias environment variable |
|---|---|
| `top` | `ANTHROPIC_DEFAULT_FABLE_MODEL` |
| `strong` | `ANTHROPIC_DEFAULT_OPUS_MODEL` |
| `mid` | `ANTHROPIC_DEFAULT_SONNET_MODEL` |
| `fast` | `ANTHROPIC_DEFAULT_HAIKU_MODEL` |

Each provider declares its harness and four `tiers`. Each tier has `model`,
`effort` and `supported_efforts`, validated by the existing
[model policy](cellctl-model-policy.md). Its `desks` map assigns all five roles
to a tier, with an optional effort override. For example, this is the shape of
one desk entry inside `providers.anthropic.desks`:

```json
"pr-review-desk": {"tier": "strong", "effort": "high"}
```

Changing `providers.anthropic.tiers.strong.model` updates every desk assigned to
that tier that has not overridden that model in its cell. The seed pins the Opus
alias to `claude-opus-4-8[1m]`. GLM's strong tier uses its full model and its mid
and fast tiers use its flash model; Kimi's tiers use K3. These are editable
operator defaults, not a promise of backend availability.

Changing a tier's effort affects desks without their own effort field. Changing
one desk's effort affects only that desk on that provider. Unsupported efforts,
floating model IDs, unknown fields/roles and denied models refuse the launch.
The entire catalog is validated, including currently unused providers and desks.

`default_provider` selects the default provider. `CELL_PROVIDER` selects another
provider for a cell, or `CELL_HARNESS=codex` selects Codex when no provider is set.
The catalog's optional `roles` map selects providers for individual desks:

```json
"roles": {"worker-desk": "glm", "intake-desk": "codex"}
```

Per-desk provider selections take precedence over the cell-wide provider. An
explicit `--provider` wins for that invocation. `--model` selects an allowed
model/tier using its tier effort, just as with `CELL_MODEL_POLICY`; without it,
the selected provider's desk effort applies.

## Cell overrides contain only exceptions

Place a partial catalog at `$CELLS_ROOT/<cell>/providers.json`. For example:

```json
{
  "roles": {"worker-desk": "glm"},
  "providers": {
    "anthropic": {
      "desks": {
        "pr-review-desk": {"effort": "xhigh"}
      }
    },
    "glm": {
      "tiers": {
        "mid": {"model": "glm-5.3[1m]"}
      }
    }
  }
}
```

This cell runs its worker on GLM's full model and its Anthropic reviewer at
`xhigh`, while continuing to inherit all other entries. Objects merge by key;
scalars and lists replace the corresponding value. `deny` patterns accumulate,
and the built-in Opus 5 prohibition remains. Null values are refused, not treated
as deletion. Removing an override key restores inheritance on the next launch.
A malformed shared file cannot be repaired or hidden by a cell override.

Alternative paths are optional:

```sh
cellctl set example CELL_PROVIDER_DEFAULTS=team-providers.json
cellctl set example CELL_PROVIDER_OVERRIDES=local-providers.json
```

A relative shared path resolves under `CELLS_ROOT`; a relative override path
resolves under the cell directory. Explicitly configured missing files refuse,
as does an override with no shared catalog.

An existing `CELL_MODEL_POLICY` remains a complete, authoritative policy and
wins over the catalog and local overrides. To adopt inheritance, migrate only
that cell's exceptions into its `providers.json`, then clear `CELL_MODEL_POLICY`
with `cellctl set example CELL_MODEL_POLICY=`. Inspect before restarting desks:

```sh
cellctl show example
DRY_RUN=1 cellctl desk example pr-review-desk
```

The output names the shared file and any cell override, the effective policy
hash, and each role's model, provider and effort. `check` uses the same resolver
for its role rows; `up` validates every requested role before opening windows.
Legacy `DESK_MODEL_*` / `TIER_MODEL_*` model pins are superseded while the catalog
is active; put cell exceptions in the override file instead. With an active
catalog, `desk/up --set` refuses; use `cellctl set` or edit the files directly.
`up --automate` also refuses because that launch path cannot propagate the policy;
use live desk windows.

## Launch and verification

For Claude, the selected provider's four tiers populate all four alias variables.
The desk's selected model populates `ANTHROPIC_MODEL`, `--model` and
`CLAUDE_CODE_SUBAGENT_MODEL`; effort populates `--effort` and
`CLAUDE_CODE_EFFORT_LEVEL`. Provider mappings replace stale inherited alias values.
GLM and Kimi retain the existing endpoint/token adapters. Codex receives its
existing model and reasoning-effort CLI settings instead of Claude aliases.

Shared edits are read on each launcher invocation. Running agents retain the
environment with which they started; restart those desks deliberately to adopt
changed models. A shown pin is requested configuration: use response/usage
metadata to prove the served model. Alias mapping does not itself enforce a ban
on an explicit runtime model switch; the remaining runtime enforcement work is
tracked in #1392.

The offline tests run the built Go binary with a temporary local Git origin and
stub harnesses. They check inherited edits, local exceptions, strict rejection,
legacy-policy precedence and actual launch environment/argv without provider
credentials or requests.
