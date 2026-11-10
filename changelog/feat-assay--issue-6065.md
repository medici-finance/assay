### Added
- Verify tables may carry an optional `Env` column naming where each row runs (`tree`, `host`, `service:<name>` for a named non-production instance, `cluster:<name>`). `statusgen --lint` reddens a `check:ci` row that names anything but `tree`, an explicitly classed env-bound row that names no environment, and an unknown Env value. Tables without the column are unaffected.
- Brief `desk-containers/34` (human-gated, todo) designs how project secrets reach dispatched worker and verifier windows, never a reviewer slot or an untrusted head.

### Changed
- The review kit and the verify-desk and pr-review-desk skills say that a check needing a project secret or a live service is never proven by the implementer's own report; until the channel above lands, such a row is witnessed after merge by an operator from the Environment-blocked queue.
