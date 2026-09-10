### Fixed
- **`deskboard`'s drift check now recognises a channel-D `desk-tools-source` pin, so a
  GitLab / native-Windows consumer sweep is no longer permanently STALE-UNKNOWN.** An
  adopter on channel D pins the desk-tools SOURCE line (`desk-tools-source <tag>
  <40-hex-commit>`, the shape `desksourceguard` already reads) rather than a `desk-tools`
  release line, and its consumer checkout carries no in-tree `tools/desk` ref either — so
  `staleState` fell straight through to could-not-check, which pinned `reviewloop`'s idle
  gate at COULD-NOT-CHECK on every tick and made a healthy board look stale forever. It now
  binds the running binary's stamped `sourceSHA` to the pinned commit the same way
  `desksourceguard`'s third agreement does (the stamp is a short SHA, so the full pinned
  commit must have it as a prefix): a match is `in-sync`, a mismatch is a MEASURED `drift`,
  and only a genuinely unreadable source line (a non-40-hex digest) with no in-tree ref
  falls back to could-not-check. This completes the fallback #185 opened — treating a real
  source pin as absent was that fallback landing short.
