# Cell model policy

For shared provider defaults with per-cell exceptions, see
[Shared provider defaults](cellctl-provider-defaults.md). A complete
`CELL_MODEL_POLICY` remains authoritative when set.

`CELL_MODEL_POLICY` selects one operator-owned JSON file for role/provider/model/effort
configuration. Paths are absolute or relative to the cell directory. Start from
[`model-policy.json`](../tools/cellctl/examples/model-policy.json), copy it into the cell,
then set `CELL_MODEL_POLICY=model-policy.json` with `cellctl set`.

The example routes review and verification to Anthropic Opus 4.8/high, coordination to
Fable 5.1/high, workers to GLM 5.3 Flash/high and intake to Codex Terra/medium. These are
editable assignments, not a quality benchmark or a claim that each account has access.
Kimi K3 is configured as another provider option. Existing provider credential variables
and endpoint overrides still apply; the JSON contains no secrets.

## Resolution

Each `roles` entry names a `provider` and `tier`. Each provider names its `harness` and
four tiers: `top`, `strong`, `mid`, `fast`. Every tier specifies an exact requested model
ID, an `effort`, and a nonempty `supported_efforts` list. No implicit model or effort
fallback is permitted. Unknown fields, missing roles/tiers, floating tier targets, denied
models and unsupported effort values refuse the launch. The resolver validates the whole
file, including providers a particular role does not currently use.

With a policy enabled, it replaces legacy `DESK_MODEL_*`, `CODEX_MODEL_*`, tier defaults,
`CELL_PROVIDER` and `CELL_HARNESS` for role selection. Explicit `--provider`, `--harness`
and `--model` remain per-run requests validated against the policy. A harness override
must match the chosen provider; `--harness codex` selects the `codex` provider. Use
`--provider anthropic --harness claude` to override a Codex role back to native Claude.
`--set` on `desk`/`up` is refused in policy mode because persisting a legacy pin would not
change the policy. Edit the JSON to make a permanent change. A plain `cellctl set` can
still set the policy path; legacy model keys are ignored while that path is active.

Model requests `fable`, `opus`, `sonnet`, `haiku` resolve to `top`, `strong`, `mid`, `fast`
within the selected provider. The tier names themselves are accepted too. Direct model IDs
must occur exactly in that provider's map. If an ID occurs in multiple tiers with different
efforts, request a tier to disambiguate. Context suffixes must already be pinned; an override
cannot silently add a larger context window.

Opus 5.0 IDs are prohibited by the policy resolver even if `deny` is omitted — `claude-opus-5`
and its `[1m]` / gateway / `Opus5` / `-5-0` / `-5.0` spellings. The prohibition is anchored at
end-of-token, so Opus 5.5+ (`claude-opus-5-5`) — a valid top tier — is NOT prohibited: it ends in
`opus-5-5`, not `opus-5`. `deny` adds further case-insensitive glob patterns. The example pins the
Opus alias to `claude-opus-4-8[1m]`. The coordinator rule is a VERSION FLOOR: `the-desk` on Claude
requires a top model at or above **Opus 5.5**, so Opus 5.5, 5.6, 6.0 and any later tier are
accepted while the bare `opus` alias (no version), Opus 5.0 and older tiers (e.g. Opus 4.8) stay
refused. Because it is a floor rather than a fixed allowlist, a future Opus tier auto-qualifies
with no code edit — a deliberate, documented trade (the "Opus 5.0 was a bad tier despite its
number" lesson makes auto-adopting a future Opus a choice, reversible by raising the floor).
Without `CELL_MODEL_POLICY`, existing launcher behavior is unchanged; rollout must enable
it on each cell that needs the prohibition.

## Harness adapters and child agents

Claude receives `--model`, `--effort`, `CLAUDE_CODE_EFFORT_LEVEL`, the four
`ANTHROPIC_DEFAULT_*_MODEL` mappings and `CLAUDE_CODE_SUBAGENT_MODEL`. Child agents default
to their desk's model and inherit its effort. Explicit child model aliases use the provider
map; a hook rejects unmapped/denied requests and incompatible inherited effort. A tier's
effort selects the desk at boot; an explicit child model does not separately change effort.
An inherited `MAX_THINKING_TOKENS` is removed for policy launches so it cannot cap the chosen
effort with a hidden fixed budget. Native Anthropic pins its base URL; GLM/Kimi use the
existing credential adapter. No credential value is printed in the launch plan.

Claude also receives an `availableModels` allowlist and a `PreModelSwitch` hook that checks
the actual target ID. Claude Code >=2.1.251 is required. Before launch, cellctl refuses local
user/project/managed-file allowlists that widen the policy, and any local `modelOverrides`.
The worktree is rechecked after creation/merge. These checks preserve other settings and
hooks rather than replacing the operator's full configuration. Both hooks call back into the
launcher itself (`cellctl model-policy hook <cell-dir> <role> <provider> <requested> <harness>
<policy-sha256>`, the event on stdin), which re-resolves the cell's policy for the launched
route. The hook refuses a policy whose SHA-256 differs from the one the window launched with,
whether the file was edited after launch or the hook's inherited environment points
`CELL_MODEL_POLICY`, `CELL_PROVIDER_DEFAULTS` or `CELL_PROVIDER_OVERRIDES` somewhere else. Until
the window is restarted, every model switch and child dispatch is blocked. Every refusal exits
2, Claude Code's blocking status. That includes a cell or policy that can no longer be loaded.
The hook command line ends in `|| exit 2`, so a hook binary that has been removed or is no
longer executable also blocks; on its own the shell would exit 127 or 126, which Claude Code
does not treat as blocking.

The hook is stricter than the shell launcher in one place: it refuses an Agent/Task event with
no `tool_input` object instead of reading it as empty. A model ID pinned at two tiers, such as
the example policy's `claude-sonnet-5` at mid and fast, appears in `availableModels`. A switch
to that exact ID is still refused, because it does not name a single tier and effort.

Codex receives `model_provider="openai"`, the exact model, `model_reasoning_effort`,
`agents.default_subagent_model` and `agents.default_subagent_reasoning_effort` as CLI
configuration overrides. Explicit Codex child-agent arguments can override those defaults;
this increment does not install a Codex child-model enforcement hook. It supports OpenAI
through Codex and Anthropic-compatible providers through Claude, not arbitrary Codex gateways.

Effort names are provider capabilities, not interchangeable measures of reasoning or cost.
The resolver restricts GLM 5.3 and Kimi K3 to low/high/max and rejects Codex `max`. Other
model capabilities are declared by the operator's map. A declaration is not a backend probe.
In particular, third-party endpoint handling of Claude's effort request still needs an
operator-observed smoke check before rollout; local tests prove argv/environment, not inference.

## Inspect, launch, and verify adoption

`cellctl show <cell>` reports each role's provider, harness, model, effort and policy SHA-256.
`cellctl check <cell>` checks policy routes and the required provider credential names. It
also reports one `model policy: <role>` row per role for the preflight `up` runs: credential,
harness on PATH, Claude version floor and the settings conflict scan.
`cellctl up <cell>` preflights every selected role before opening windows, so a missing
credential does not start half a cell. `DRY_RUN=1 cellctl desk <cell> <role>` prints the
resolved launch. A real boot prints the same values. No automatic restart is performed.

This increment supports house and k8s launch paths. Container, scrubbed and Orca automation
launches explicitly refuse policy mode until their composed launch contracts carry it.
Normal tmux/Herdr/Orca terminal launches use the shared `desk` path.

Release/install the reviewed cellctl revision before enabling the JSON path, then restart
one role and inspect both its launch record and actual transcript model/effort. Repeat for
one child agent before expanding to all roles. An old launcher may ignore the new key.
Source merge, a dry run, and a model name in a prompt do not establish live adoption.

The launcher is an operator configuration tool, not a security boundary against agents with
shell access. Managed/cloud or host policies can outrank local settings; administrators must
reconcile those settings too. Hooks alone do not cover automatic Claude fallback. The
allowlist is the fallback control, subject to those higher settings. Use transcript/API
usage records to prove the served model, and stop rollout if they disagree with the pin.
Restore the prior reviewed JSON to roll back assignments; do not remove policy mode to
roll back a model while the Opus 5 prohibition is required. Already-running sessions retain
their launch settings and must be restarted deliberately.

## Offline checks and upstream contracts

Run `python3 tools/cellctl/tests/model-policy.test.py` plus the existing provider, harness,
model-namespace, model-override and cell-set shell suites. The model-policy suite uses only
local Git fixtures and recording harness stubs. No inference call, production endpoint or
real credential is involved. The shell launcher needs Python 3.9+ for policy preflight; the Go
`cellctl` binary needs no Python. Its policy tests run against the built binary with the same
kind of fixtures: `go -C tools/desk test ./cmd/cellctl/ -run 'Policy|Hook|Settings|Preflight'`.

Sources checked 2026-09-20:

- [Claude model configuration](https://code.claude.com/docs/en/model-config): versioned
  aliases, effort, child defaults, allowlists, fallback and settings merge behavior.
- [Claude hooks](https://code.claude.com/docs/en/hooks): `PreModelSwitch` and `PreToolUse`.
- [Codex configuration](https://developers.openai.com/codex/config-reference/): model,
  reasoning effort and default subagent configuration; explicit spawn overrides win.
- [GLM parameters](https://docs.z.ai/guides/overview/concept-param) and
  [Kimi K3 vendor tests](https://github.com/MoonshotAI/Kimi-Vendor-Verifier/blob/main/tests/k3_features/test_thinking_effort.py): provider effort values.
