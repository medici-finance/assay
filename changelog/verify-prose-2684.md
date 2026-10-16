### Fixed

- `statusgen`: a Verify Command cell whose lifted command is only a shell comment (an inline `## Coverage` heading in a procedure) is now flagged `prose-led-command` and recorded could-not-run instead of a pass with empty output; the check:ci re-execution lane refuses it, an old pass witness is not audited as pass, and the `verifyloop` and `deskrebaseline` executors no longer run it (#2684).
