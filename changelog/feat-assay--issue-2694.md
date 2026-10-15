### Fixed
- `statusgen reconcile` no longer holds every witnessed move on a driven stream for `drive-region-drift`: the lint hold now judges the README edit together with the drive-snapshot region it re-renders, reports that region under the row's `generated`, and `--apply` writes both. The read-only form still writes nothing, and a hand-edited region is never re-rendered.
