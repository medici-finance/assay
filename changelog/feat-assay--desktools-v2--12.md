### Changed
- `cellctl` resolves the operator home in one place: `USERPROFILE` first on Windows (the home every desk tool it launches reads through Go's `os.UserHomeDir`), `HOME` first elsewhere, the other as fallback — and refuses with a named reason when neither is set instead of deriving a relative or root-anchored config, gh, Claude or Codex path. Explicit overrides (`ASSAY_CONFIG_HOME`, `GH_CONFIG_DIR`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME`) still resolve with no home, and `%APPDATA%\GitHub CLI` is the gh default on Windows. A class guard fails any new raw `HOME`/`USERPROFILE` read outside the resolver.

### Fixed
- The public-repo self-containment scan now recognises Windows absolute machine paths — a drive-letter path under the Users root and a UNC path naming a host and share, each confirmed with `IsAbsFor("windows", …)` — as the same "absolute machine path" refusal the POSIX roots already raise; bare roots and placeholders stay tolerated.
