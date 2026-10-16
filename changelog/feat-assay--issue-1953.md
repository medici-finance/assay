### Fixed
- `deskrebaseline` and the `verifyloop` runner no longer cut a Verify row at an escaped pipe (`\|`): the whole pipeline is parsed and run, as `statusgen` does.
- `deskrebaseline` re-runs a renamed row against the renamed file before calling it `safe:rename`, and judges the behaviour probe against the row's Expect cell instead of treating any non-zero exit as a changed result. An Expect it cannot decide (prose only, or a "non-zero" exit) is never proof: the row is `refused:unclassified`.
