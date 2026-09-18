### Fixed
- `cellctl gen_shims`: a shimmed desk verb's `gh` subprocess now authenticates. gh's ambient
  credential (keychain on macOS, hosts.yml-adjacent elsewhere) is keyed to the REAL `HOME`, so it
  is resolved *before* the shim swaps `HOME` to the cell home, then threaded through as `GH_TOKEN`
  — the verb's own config/state stays isolated to the cell exactly as before. An explicit
  `GH_TOKEN`/`GH_ENTERPRISE_TOKEN` already set by the caller is never overridden.
