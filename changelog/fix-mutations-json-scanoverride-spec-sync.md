### Fixed
- `internal/deskkit/mutations.json`: re-synced the "stop the refusal advertising
  the override" mutation's `old`/`new` text to the current
  `internal/deskkit/scanoverride.go` line (`RefusedFinding(scanErr.Error()+OverrideHint(), f)`),
  which had drifted after an earlier refusal-shape change left the mutation's
  recorded `old` text unmatched. `muhar` now KILLS every mutation in the spec
  with none `COULD_NOT_MUTATE`.
