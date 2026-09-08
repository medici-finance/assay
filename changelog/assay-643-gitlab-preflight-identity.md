### Fixed
- `deskroster preflight` on GitLab no longer fails `commit-identity` when the worktree
  commits as the documented session / implementer identity (a real GitLab user such as
  `ih-bot`) rather than as the role service account. The check now distinguishes the two
  identities: it accepts a commit email that is an explicitly trusted session address —
  listed in the new `ASSAY_GITLAB_SESSION_EMAILS` roster allowlist — in addition to the
  service-account noreply shape used when the worktree commits *as* the service account.

### Added
- `ASSAY_GITLAB_SESSION_EMAILS` — an exact-match, roster-only allowlist of the GitLab
  commit-author addresses accepted as a session / implementer identity. It is additive
  and fail-closed: unset means the service-account noreply shape stays the only accepted
  GitLab commit email (unchanged behaviour), an unlisted ordinary address still fails,
  the cross-forge rejection is unchanged, and it is never consulted on a GitHub identity
  (the bot-USER-id guarantee is untouched). Echoed in the effective-config run output.
