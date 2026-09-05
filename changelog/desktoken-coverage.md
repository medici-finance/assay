### Added
- `desktoken coverage <role> [--repo <slug>] [--json]` — a read-only verb that lists the repositories each of a role's App installations can see. Tokens are minted into memory only (no cache, no token or JWT printed); a repo page that cannot be read is exit 6 rather than a silently-short list, and `--repo` returns exit 0/5 for seen/not-seen.
