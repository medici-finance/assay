### Changed
- `brief-v4` records now require the top-level `reader-requirements` envelope, and the envelope must require `syntax` support for `brief-v4` so a reader advertising only `brief-v1`..`brief-v3` refuses the record. A record without it was previously accepted, and a reader that checks only envelopes would have read it as a legacy brief. `schemas/brief-v4.json` and `spec/brief-v1.md` §3.5 state the requirement.

### Fixed
- A `brief-v4` typed reference whose own root cannot be resolved now reports the underlying resolution error. The error was previously discarded, which surfaced a misleading "names brief work in another root" refusal.
- The cross-root refusal for same-root brief references now has a test and a mutation row in the brief-04 mutation manifest.
