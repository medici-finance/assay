# tools/desk/internal/acp — spike findings

This records what the live spike found, so the native-dispatch design starts from
measured protocol facts rather than the spec's assumptions.

## Answer to the dispatch spec §7.1 — auth/billing mode

**Subscription-login billing, not `ANTHROPIC_API_KEY` pay-per-token billing.**

`ANTHROPIC_API_KEY` was unset in the spawned process's environment for both live runs.
`initialize`'s response carried `"authMethods":[]` — the adapter considered itself
already authenticated via the machine's existing `claude` CLI credential store and
offered no login step. Usage from the prompt turn was reported (on a `session/update`
`usage_update` notification, `_meta._claude/rateLimit`) against a **five-hour
subscription rate-limit window**, with per-request overage explicitly disabled at the
org level, not metered per-call:

```
"_claude/rateLimit":{"status":"allowed","resetsAt":1786505400,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"org_level_disabled","isUsingOverage":false}
```

A dollar `cost` figure is also reported per turn (`"cost":{"amount":0.219605,"currency":"USD"}`)
but this reads as an *observability* figure attached to subscription usage, not an
API-key line-item charge — there is no API key in the picture at all.

**Billing implication for tier→runner config:** a dispatched worker running
under this adapter, with no `ANTHROPIC_API_KEY` configured, draws from the *same*
5-hour subscription window as the operator model's own usage and everything else
sharing that machine's `claude` login. Concurrent dispatches contend for one shared
window, not N independent per-worker API budgets. For isolated,
per-worker billing, a runner must set `ANTHROPIC_API_KEY` in each runner's `Opts.Env`
explicitly — the adapter will use it instead (it's one of the adapter's own
credential-routing env vars) — and that path was not exercised live here.

**Caveat:** this was run from inside an already-authenticated Claude Code sandbox (a
worker session of the harness itself), so the ambient credential store observed is the
same one the harness already uses — this is not an unusual or contrived environment,
but it does mean the *unauthenticated* path (a bare machine with no prior `claude`
login) was not exercised. In that case `initialize`'s `authMethods` would instead list
`claude-ai-login` / `console-login` / `claude-login` entries (gated by
`clientCapabilities.auth.terminal`), and a caller must complete a `claude auth login`
step out of band before `session/new` — this package does not implement that flow.

## Versions pinned during the spike

| What | Value |
|---|---|
| Adapter | `@agentclientprotocol/claude-agent-acp@0.66.0` (`npx -y`, no version pin recorded by this spike — real pinning is left to the consumer) |
| Adapter's ACP SDK dep | `@agentclientprotocol/sdk@1.3.0` |
| Adapter's Claude Agent SDK dep | `@anthropic-ai/claude-agent-sdk@0.3.220` |
| Negotiated protocol version | **1** (integer; the adapter's `initialize` handler returns `protocolVersion: 1` unconditionally — it does not actually negotiate against the client's requested value, per its own source) |
| Node | v26.7.0 on the dev machine (a freshness note recorded v22.22.3 present; the adapter ran fine on v26, engines floor not separately checked) |

## Protocol surprises

1. **Framing is newline-delimited JSON-RPC 2.0, not LSP-style Content-Length
   framing.** One JSON object per line on stdin/stdout, confirmed against the
   adapter's own `@agentclientprotocol/sdk` `LineBuffer` source and by driving it
   directly.

2. **The default session mode ("auto") never calls `session/request_permission`
   at all.** `session/new`'s response lists the available modes; the default,
   `auto`, resolves every tool-call permission decision via the adapter's own
   internal model classifier without ever asking the ACP client. In a first spike
   run under this mode, a file-write tool call went straight to `completed` with
   **no** `session/request_permission` round trip. Re-running with
   `session/set_mode` → `modeId: "default"` ("Manual", *"Standard behavior,
   prompts for dangerous operations"*) immediately produced the expected
   `session/request_permission` request, and denying it (client selects the
   `reject_once` option) correctly stopped the agent before the write and produced
   `stopReason: "refusal"` with no file written.

   **This is the single most important finding.** The dispatch spec
   §4.2(4) says "Permission policy: default-deny" — but that policy is *inert*
   unless the native-dispatch adapter calls `SetMode` to `"default"` (or
   `"dontAsk"`) immediately after `NewSession`, before the first `Prompt`. Skipping
   that call does not weaken the permission gate, it **removes it entirely**: every
   tool call the agent's own classifier approves happens with zero visibility to
   the Go client. `Client.SetMode` in this package exists specifically so 15 cannot
   miss this step by omission.

3. **`fs/read_text_file` / `fs/write_text_file` were never called by the adapter
   in either live run**, even with `clientCapabilities.fs.{readTextFile,writeTextFile}`
   advertised as `true`. The adapter performed the file write itself (visible via
   `tool_call` / `tool_call_update` notifications carrying
   `_meta.claudeCode.toolName: "Write"`) and gated it exclusively through
   `session/request_permission` on that tool call. Treat this package's `fs/*`
   handlers (`Opts.FileAccessPolicy` / `Opts.FSRoot`) as defense in depth for
   agents that *do* route through them, not as the primary enforcement point —
   **`session/request_permission` is the real gate**, and per finding 2 it only
   fires at all when the session mode makes it fire.

4. **`session/new`'s response and the subsequent `available_commands_update`
   notification echo this environment's own configuration back** — model/effort/
   agent-persona `configOptions`, and (in `available_commands_update`) the full
   slash-command/skill roster of the *host* Claude Code session the adapter's SDK
   is running inside of. That payload is environment-specific; a native-dispatch
   consumer must not assume its shape is stable or agent-neutral across runner
   changes (relevant to dual-track Claude/Codex/Gemini runner rows).

5. **Lines can be large** (tens of KB — `available_commands_update` was the
   largest observed). This package's reader uses `bufio.Reader.ReadBytes`, not a
   size-capped `bufio.Scanner`, for exactly this reason.

## What this package does and does not cover

- Core flow: `initialize` → `session/new` → `session/set_mode` → `session/prompt`
  (blocks for turn completion) → `session/update` stream (concurrent, buffered
  channel) → `session/cancel`. `Close` kills the process.
- `session/request_permission` and `fs/read_text_file` / `fs/write_text_file` are
  served via `PermissionPolicy` / `FileAccessPolicy` callbacks, both fail-closed by
  default (`DefaultRefusePermission`; FS refuses everything outside `Opts.FSRoot`,
  and everything if `FSRoot` is empty).
- `terminal/*` and `mcp/*` requests from the agent are refused with a JSON-RPC
  "method not found" error — not implemented, consistent with this client
  advertising `clientCapabilities.terminal: false`.
- Protocol-version handling fails closed: `Initialize` returns
  `*ErrUnsupportedProtocolVersion` and the client refuses every later call
  (`ErrNotInitialized`) if the agent negotiates a version outside
  `Opts.SupportedProtocolVersions` (default `[]int{1}`, the only version observed).

Public surface is spike-grade (`Spawn`, `Initialize`, `NewSession`, `SetMode`,
`Prompt`, `Cancel`, `Close`, `Updates()`) and is expected to be revised once a real
consumer is built on top of it.
