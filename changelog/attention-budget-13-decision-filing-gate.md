### Added
- `deskfile new` requires a `### Fork test` block on every `needs-decision` filing (the
  workable options, the default, the human-held gate that catches a wrong guess, and the
  search proving the question was not already ruled), refusing a filing with fewer than two
  workable options and naming three `--no-fork` re-routes
  (`brief-contradicts-artifact` | `wrong-repo` | `tool-false-positive`) instead.
- Two workable options plus a gate the driver still holds now files on a NOTICE LANE
  (`desk-decided`, off the driver's queue, with the shared `desk-r3-decision v1` marker) rather
  than `needs-decision` — unless a one-way term (merge, release, security, secrets, money,
  identity, publication) is present, which always keeps the item on the driver's queue.
  `deskdigest` lists notice-lane items in its desk-decisions section with their veto date.

### Changed
- The R-3 human-only keyword list moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`) so `deskfile`'s new one-way override and
  `deskdigest`'s classifier consult exactly one definition.
