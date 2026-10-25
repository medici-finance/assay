### Fixed
- `deskevidence flip` excludes already-published Evidence using the same library branch-tip snapshot as `deskpr update`, allowing Evidence and its verified flip in one PR while publication still checks the live destination and new commits retain both identity checks.
