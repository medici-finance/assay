### Fixed
- `deskflip` and `deskpost ready` refuse to flip a PR whose body names a `Worker-Attestation:` record until it carries the matching `authored-model:` / `authored-tier:` pair (condition `authoring-stamp`). The refusal prints the exact `deskdispatch --stamp-only` line built from the record. A PR with no attestation line is unaffected.

### Added
- `deskboard prs` lists each open attested PR that lacks its authoring stamp (`authoringStamp`, `stampOnly`), and the coordinator's loop runs that listing on every sweep, so a dropped worker hand-off is stamped before merge.
