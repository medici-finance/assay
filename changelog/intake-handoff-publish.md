### Added

- `statusgen --scan-issues` now publishes the recorded intake judgment for each placeholder it owns. Given `--judgments <log>` and `--scan-target <owner/repo>`, it writes the latest recorded handoff beside the placeholder and sets the placeholder's `handoff:` and `handoff-opened:` keys, so a judged issue reaches the board without a hand edit. Only the latest revision of the latest event the log holds for an issue is published; a judgment for an older event, or one a reactivation already spent, is left out. A second scan with no change writes nothing. A log line that does not decode, or has no RFC 3339 event time, is skipped, named on stderr and counted as unpublished; the scan still exits 0. A log file that exists but cannot be opened or read fails the scan (exit 2).
- Free text in a judgment (evidence that is not a typed reference, a required action, a wake condition) is published only when a target was supplied, the checkout's origin push destination names that same repository, and the roster marks it private. Under any other root a judgment is published only when every string field is a closed value, a bounded typed reference or an opaque id and the free-text fields are empty; each judgment left out is named with its field and counted as unpublished.
- `scanloop`'s scan lane passes the judgment log and the drain's resolved target repository to the scan.

### Changed

- `scanloop land --exit bug` and `--exit placeholder` are refused (exit 5) without `--handoff` and `--handoff-reader`. The `finding`, `needs-decision` and `rejected-watching` exits are unchanged.
