### Added
- `deskevidence --outcome-record` lands a verify outcome as its own new file under
  `docs/streams/verify-outcomes/<stream>/`, ending the class of Evidence PRs going
  `CONFLICTING` whenever a sibling landed (every Evidence PR used to append one line to a
  single shared `docs/streams/verify-outcomes.jsonl`, which the forge merges server-side with
  no `merge=union` driver).
- The writer additionally refuses a `verify-wake-v1` receipt whose `inputs` omits a declared
  deliverable, whose `blocker_ref` is not a real issue/PR/run reference, or whose brief hash is
  not the brief as it lands — three recurring defects reviewers previously caught by hand.
- `statusgen outcomes split --root <dir> [--check]` migrates the legacy log to the new
  per-file layout, idempotently.

### Changed
- `deskevidence` refuses (exit 5) any write to a shared appended `docs/streams/*.jsonl` log,
  by shape, not just the retired verify-outcomes name.
- verify-desk and pr-review-desk: sibling Evidence PRs writing ONLY the new per-file layout no
  longer conflict on outcomes; a `CONFLICTING` verdict on such a PR now means a real content
  conflict. Both skills add a TRANSITION-WINDOW clause: a PR still appending to the shared
  `docs/streams/verify-outcomes.jsonl` log still conflicts with every sibling PR that also
  touches it, exactly as before, until #1802 retires the log — resolve that case by merging main
  locally (the `merge=union` driver still applies to a local merge).

### Fixed
- The verify-outcomes append-only sidecar's raised write-size cap (#1338) is retired along
  with the file it existed for; a single outcome record is nowhere near the general cap.
- **(round 2, #1803 review)** `--outcome-record` refuses a record whose `ts` is more than 5
  minutes ahead of the writer's own clock, and `LatestPerBrief`/`LatestPerBriefAt` never let a
  future-dated record on disk win the newest-`ts` comparison — it is reported could-not-check
  instead. Before this, an unbounded future `ts` could permanently shadow every later, genuine
  outcome for its brief.
- **(round 2, #1803 review)** The writer now re-checks the RESOLVED record path against
  `docs/streams/verify-outcomes/<stream>/<file>.json`'s exact shape after `RecordName` computes
  it — an independent, defense-in-depth guard sharing no code with the name/regex validation
  that already existed, so a regression in that validation alone cannot silently escape the
  sandbox.
- **(round 2, #1803 review)** Verify row 13's fixture is corrected: it previously could not fail
  even when the deliverable check read the brief from the wrong tree (HEAD instead of the
  record's `sha`), because the discriminating file was absent at the record's `sha` under either
  read. A second, planted file closes that gap.
