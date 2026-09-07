### Added
- **`tools/claimguard`** — a heuristic lint for the "named third-party product,
  no citation at all" shape of unresolved outward claim (a CamelCase or
  ALL-CAPS-acronym-shaped token with no resolving URL within a token window,
  and no markdown-link anchor around it). Complements a separate
  link-resolution check, which covers a *present-but-dead* citation; this one
  covers the case where nothing was ever cited to resolve in the first place.
  Standalone Go module (`go run ./tools/claimguard <path>...`), hermetic
  (no network calls), unit-tested with a fail-first case and a passing case.
  Not wired into any CI gate yet — see `tools/claimguard/README.md` for scope
  and known limitations.
