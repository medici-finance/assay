### Changed
- Retain quiet intake observations across polling advances, restarts and later source closure.
- Add an explicitly negotiated intake handoff companion for issue and register lanes while preserving intake-exit-v1.
- Require an accountable disposition before scanner repair scheduling; raw and legacy placeholders remain classification-needed. Trust, claims and independent verification remain separate.
- A recorded `observed` or `external-wait` judgment on an issue's latest event ends its replay, and so does an intake-exit-v1 `rejected-watching` record decided by judgment for that event (a mechanical exit never does). A definite not-found in a cleanly read repository, or a clean untrusted read, records a terminal `gone` or `quarantined` disposition for that event, and a newer event is read again. Retained replay shares the poller's per-repository burst bound; the excess is reported as deferred and is presented once an earlier retained event is discharged or retired.
- `scanloop land` reports when a retry names a different exit than the one already recorded; the first exit stays immutable.
- Rollout effect: existing scanner placeholders leave ordinary repair scheduling (Next-up) until a reviewed change publishes their `handoff:` companion with a repair disposition. Nothing writes those pointers automatically.
