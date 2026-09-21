### Added
- Persistent review findings survive agent replacement and restarts. A reviewer
  verdict or worker reply may now carry a versioned `review-finding/v1` block
  (embedded additively in the forge body, invisible to a legacy reader), and
  `reviewloop` derives the outstanding findings, disputed responses and per-class
  round counts from those durable records rather than from an agent's memory —
  identical after a replacement or restart because the ledger is a pure function
  of the records.
- `reviewloop plan --records <thread.json>` renders the derived finding ledger for
  one PR: outstanding blocking findings, per-class rounds against the existing
  cap, the single arbiter packet at the cap, and every could-not-check reason.

### Changed
- `deskpost review` and `deskreply` validate an embedded finding block before any
  network call: a worker reply cannot author a reviewer's resolution of a blocking
  finding or hand-assert the arbitration cap, and a blocking finding must carry a
  concrete reproduction or evidence-based explanation. Bodies with no block are
  unaffected.
