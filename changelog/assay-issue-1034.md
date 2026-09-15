### Fixed
- `desktoken --forge gitlab <role>` now **serialises** rotate-on-mint per role. Two mints for one
  role previously both called the GitLab self-rotation endpoint, which invalidates the token the
  caller presents — the loser got `401 invalid_token`, and the custody file could be left holding a
  revoked value with no live successor, recoverable only by a group owner re-issuing the PAT.
  Overlapping mints now queue on a per-role advisory lock held across read-current → rotate →
  write-verify, so each rotates from the value its predecessor persisted.
- A mint that cannot take that lock **refuses before a second rotation is in flight** (exit 6) and
  names the recovery path, instead of rotating unserialised. A custody directory that cannot be
  written is now detected *before* the rotation rather than after it, so the role's existing token
  survives a misconfiguration instead of being spent on a rotation that could never have been saved.

### Added
- `desktoken --no-rotate` — a read-only credential lookup that makes the same custody checks and
  prints the same path, but performs no rotation and no network contact. `deskfile check`, a dry run
  that files nothing, now uses it, so a parallel sweep of checks no longer drives one destructive
  rotation per call. Rotate-on-mint is unchanged for verbs that write.
