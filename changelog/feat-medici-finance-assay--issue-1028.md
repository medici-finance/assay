### Fixed
- `fanoutloop plan` no longer offers `Awaiting implementer rework` board rows that have
  already moved on. The rework lane now cross-checks each row against its own stream
  README Status cell (read from the same `origin/main` ref the board is read from) and
  drops any row that has left the awaiting-rework state — the rework already landed
  (`done`) or the deliverable was reset (`todo`) — the same rendered-board lag the Next-up
  lane already guards against.
- `fanoutloop plan`'s already-represented exclusion (a brief that already has an open or
  merged pull request, matched on the PR's `Brief:` trailer rather than a derived branch
  name) now also covers `Awaiting implementer rework` rows, so a rework row whose
  deliverable already merged is not offered for a fresh dispatch. Orphan resumes and
  durable repair obligations stay exempt, since a representing PR is expected there rather
  than a phantom.
