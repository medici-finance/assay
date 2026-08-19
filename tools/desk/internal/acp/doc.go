// Package acp is a stdlib-only, spike-grade Go client for the Agent Client
// Protocol (ACP): JSON-RPC 2.0 over an agent subprocess's stdin/stdout,
// speaking the core flow initialize -> session/new -> session/prompt ->
// session/update notifications -> turn completion, plus session/cancel.
//
// Built to answer the two facts the dispatch spec §7.1 needs before any wider
// rollout: whether a thin Go client can drive the official adapter
// (`npx @agentclientprotocol/claude-agent-acp`) through a full session, and
// which auth/billing mode that adapter actually uses. See README.md for the
// findings -- both are answered there in detail, not restated here.
//
// # Scope and non-goals
//
//   - This package owns protocol mechanics and the permission/fs policy gate
//     ONLY. It must not import the drain-engine package -- the dependency
//     points the other way (the engine adapter imports this package, per the
//     dispatch spec §4.1). A reverse-import grep enforces this mechanically.
//   - Spike-grade: the public surface (Spawn/Initialize/NewSession/Prompt/
//     Cancel/Close, the update channel, PermissionPolicy/FileAccessPolicy) is
//     expected to be revised once a real consumer exists.
//   - Terminal (`terminal/*`) and MCP (`mcp/*`) callbacks from the agent are
//     refused (JSON-RPC "method not found"), not implemented. This client
//     advertises `terminal: false` in its `clientCapabilities`, so a
//     spec-conformant agent should not call them in the first place.
//
// # Fail-closed defaults
//
//   - DefaultRefusePermission refuses every session/request_permission
//     unless the caller supplies its own PermissionPolicy.
//   - The default FileAccessPolicy refuses every fs/read_text_file and
//     fs/write_text_file call outside Opts.FSRoot (and refuses everything if
//     FSRoot is empty).
//   - Initialize returns *ErrUnsupportedProtocolVersion, and the client
//     refuses to proceed to session/new or session/prompt (ErrNotInitialized),
//     if the agent negotiates a protocol version outside
//     Opts.SupportedProtocolVersions.
//
// See README.md for the live-spike findings, including a protocol surprise
// that matters more than any of the above: the agent's default session mode
// ("auto") never calls session/request_permission at all -- a consumer that
// wants this package's permission gate to mean anything MUST call
// session/set_mode to "default" (or "dontAsk") right after NewSession.
package acp
