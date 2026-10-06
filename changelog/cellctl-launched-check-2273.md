### Fixed
- Make `cellctl check` preserve operator configuration paths inside launched desk environments, accept valid config symlink aliases, and continue rejecting missing or misdirected configuration.
- Retain an independent operator config target across command composition and nested launches, and keep config-link validation separate from the active roster override.
