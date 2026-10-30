- Move stale-FAIL merge-range graph reads into the shared Git library, preserving native abbreviation-shadowing profiles and fail-closed graph errors.
- Reuse a per-run indexed ancestry graph and preserve symbolic replacement references in ordinary native-profile selection.

- Move literal last-touch and board heartbeat history selection plus fresh reference snapshots into gitcore, retaining native short-hash and committer-date formatting and unsupported profiles. Cover merge simplification, symbolic references, mode/deletion changes, linked-worktree shadowing and encoding choices.

- Reuse immutable commit and tree facts across metadata queries while re-reading HEAD and reference snapshots; retain failures as uncertainty.

- Qualify reference-only snapshots without an extra history-policy namespace scan; retain repository-selection checks and all history-specific replacement/graft checks.

- Keep last-touch and heartbeat logs native when effective configuration sets log.follow, log.showSignature or an i18n log encoding (one config read per root per run), or when a pathspec environment is set; cap decoded trees retained by metadata queries.
