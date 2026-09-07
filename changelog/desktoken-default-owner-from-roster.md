### Fixed
- `desktoken <role>` with no `--repo` now resolves the installation owner from the
  configured allowed-repo roster (`ASSAY_ALLOWED_REPOS`), the same source the other
  desk verbs use, instead of the shipped `example-org` topology placeholder. A single
  configured owner resolves automatically; an unconfigured or ambiguous (multi-owner)
  set now fails closed with a clear could-not-check message that names the placeholder
  and tells you to pass `--repo` — replacing the bare `rc=6` that read like an auth
  failure when the real cause was an unresolved owner.

### Changed
- `desktoken` now declares its tool class and echoes its effective config (the P3
  `assay-config:` lines) on stderr like every other roster-reading verb; its stdout
  remains the token path alone.
