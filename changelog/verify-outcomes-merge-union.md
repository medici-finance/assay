### Fixed
- **The verifier Evidence log no longer serially re-conflicts.** Every verifier Evidence PR
  appends one self-contained JSON row to `docs/streams/verify-outcomes.jsonl`, so with several
  open at once each landing made the rest CONFLICTING on that append-only file (add/add), stalling
  the auto-merge lane and forcing a serial merge-main into each survivor. `.gitattributes` now
  marks the log `merge=union`, so a base-side append and a branch-side append both survive with no
  conflict markers. The rows are read by key (brief/ts/sha), never by position, so the interleaved
  order a union merge can produce is harmless; a consumer wanting chronological order sorts by
  `ts`. (#588)
