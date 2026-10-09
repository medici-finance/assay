### Fixed
- Make `cellctl check` preserve operator configuration paths inside launched desk environments, accept valid config symlink aliases, and continue rejecting missing or misdirected configuration.
- Retain an independent operator config target across command composition and nested launches, and keep config-link validation separate from the active roster override.
- Leave the operator context unset when the composing environment is already cell-scoped, so `cellctl check` refuses a misdirected cell config link instead of recording the link's target as the expectation.
- Recognise a cell config link by resolving each path element as the OS does, so an alias whose stored target ends in a separator or a dot element, a long alias chain, or a not-yet-created path inside a cell home no longer records a misdirected link as the operator expectation; `cellctl check` applies the same test to its expected target.
- Keep `CLAUDE_CONFIG_DIR` out of the Codex process environment; only its command environment carries the resolved Claude config directory.
