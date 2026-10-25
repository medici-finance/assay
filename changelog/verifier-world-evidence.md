### Added
- The verifier run now carries a run-bound prerequisite record: each root's envelope preflight admits the root revision, desk-tools build against its pin, host runtime and App capability evidence, and every queued item is admitted on top of it with the sibling checkouts its brief declares and the pinned instruments its Verify rows call. All of it is read from local materialized state; nothing is fetched, minted or probed live.

### Changed
- `verifyloop plan` holds an item whose prerequisites are missing as `environment could-not-check` (or `environment hold` when an input contradicts its pin) before any Verify row is spent, keeps both distinct from a code FAIL and from a successful empty scan, and skips a root with unreadable prerequisites with that exact reason while healthy roots keep their coverage.
- Dispatch and landing recheck the admitted record: a source, desk-tools build, runtime, sibling or instrument that moved after admission refuses the run, and landing writes no Evidence and files no bug for it. A run prepared in its own home persists the record with the attestation, so the executor's own admission check consumes it too.
