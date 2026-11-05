# Cell-owned providers and Herdr on scrubbed cells

A scrubbed cell can use Kimi through the Claude harness while retaining its composed
environment, private configuration, single-session lock and `KUBECONFIG=/dev/null`.
Herdr can display that same private tmux session. It does not launch a second harness
or receive provider credentials.

## Provisioning

The operator provisions the token as a regular mode-0600 file at
`<cell>/home/.config/assay/providers/kimi.token`. Each directory from `<cell>/home`
through `providers` must be a real mode-0700 directory. Symlinks, including links
on intermediate directories, are refused. The file contains one nonempty token
line and an optional final newline, bounded to 16 KiB. Do not put tokens in
`cell.env`, command arguments, issue bodies, or shell history.

Endpoint/model overrides for this kind must come from its own `cell.env`, never
the operator environment or machine-wide provider defaults. The controller never
imports `KIMI_API_KEY`, native Anthropic credentials, or a
host `CLAUDE_CONFIG_DIR` for this kind. A missing private token remains a refusal
even if a host token is available. The cell also needs its own Claude configuration
and Assay skills installed; a Codex login does not provision the Claude harness.

Once provisioned, select the provider and display with:

```sh
cellctl set example --harness claude --provider kimi --cockpit herdr
cellctl set example DESK_MODEL_the_desk='k3[1m]'
cellctl check example
DRY_RUN=1 cellctl desk example the-desk
```

The provider's preset supplies the default endpoint and model. A per-role model
pin or `--model` still takes precedence, so replace any existing Anthropic-only
role pins with the chosen provider model. Other roles without such pins use the
provider preset. The provider model also supplies the three subagent tier aliases.
`CODEX_MODEL_*` pins remain available for a deliberate future switch to Codex;
Kimi with the Codex harness is refused.

`show` names the token file rather than reporting host token availability. `check`
checks custody without calling the provider. A dry run prints the internal
credential-loading entrypoint and token-file location, never token contents.
The live entrypoint rechecks custody after `env -i`, then passes the token through
the harness process environment. Tokens are absent from tmux commands, argv,
Herdr pane commands and saved receipts. `smoke` uses the same custody path and
provider model; invoke it explicitly only when a real provider call is authorized.
Before first contact it prints the resolved provider endpoint.

The token is in the harness environment, so harness tool and MCP subprocesses can
inherit it. The harness also has access to its cell-owned token file. This is the
same capability boundary as the existing host-provider launch; this change does
not isolate the credential from code running inside the cell. Credential-helper
delivery would require a separate protocol and lifecycle design.

Provider runtime entry preserves the outer launch's validated performance context,
including its parent invocation. It uses the shared performance-context resolver
and serializer; unknown `ASSAY_PERF_*` variables are not copied through.

## Herdr lifecycle

`CELL_COCKPIT=herdr` is explicit opt-in. An absent setting or `auto` preserves the
historical private tmux surface. `orca` is refused for scrubbed cells. Herdr and
tmux must both be installed.
Scrubbed `auto` resolves directly to private tmux. Neither launch nor readiness
checks contact Herdr or probe the orca desktop service to make that choice.

The private tmux session remains the sole process owner and retains the existing
atomic cell lock. Herdr opens an attachment in a workspace owned by this cell,
using the existing workspace receipt and metadata ownership checks. The child
still uses plain git worktrees (`ASSAY_COCKPIT=tmux` for provider/Herdr launches);
it receives no host Herdr socket or configuration. A failed Herdr attachment is
reported as a failure, with the private session left inspectable via `status` and
`down`. It never starts a second harness or silently switches displays.

`down` stops the private session and closes only the workspace whose receipt and
live ownership metadata match this cell. A recycled workspace ID is refused.
Worktrees and provider credentials remain in place. Existing running sessions do
not change provider or display when configuration is edited; restart separately
after readiness and review.

## Consumer and sibling sweep

The provider selection flows through `desk`/`up`, `show`, `check`, dry-run plans,
the internal runtime entrypoint and `smoke`. Host provider behavior remains on its
existing credential path. Container providers, model policies and host config
directories remain refused on scrubbed cells where previously refused.

The display selection flows through `desk`/`up`, `check`, private tmux launch,
Herdr attachment and `down`. The existing status/session-lock mechanism remains
authoritative. This is controller support for a new scrubbed-cell feature, not
a change to the historical shell oracle's compatibility matrix.

Offline verification uses the built controller with stub harnesses, synthetic
credentials and stub Herdr/tmux commands. No test calls a model, forge, or cluster.

The guard tests have a reproducible mutation manifest. From `tools/desk`, run:

```sh
go run ./cmd/muhar -j 2 -spec cmd/cellctl/scrubbed-provider-mutations.json
```

The existing harness requires a green pristine baseline and a failing positive
control before reporting mutants. It uses isolated module copies and self-contained fixtures. The manifest
plants host credential fallback, each custody refusal defect, missing runtime
credential/performance handoff, token-in-argv, missing session ownership, unsafe
cockpit selection/probing and hidden smoke-target defects.
