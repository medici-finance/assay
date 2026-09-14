### Fixed
- `deskkit.Guard()`'s disarm-transition check (`lastResultWas`) now reads only the last
  line of `audit.jsonl` via a bounded seek-from-end scan (`deskkit.LastEntry`), instead
  of `LoadEntries()` parsing the whole file on every desk verb. On a fleet-scale,
  never-rotated audit log this call was the single largest fixed overhead in the
  desk-tool substrate; it is now independent of the log's size. (medici-finance/assay#1035)
