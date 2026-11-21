### Fixed

- `TestCellopCompletionRecord` now accepts either correct refusal from the one-step close of desk-containers/15 (the FAIL-verdict check or the held/could-not-check check), so a later BLOCKED Evidence entry on that brief no longer breaks the statusgen test shard. A nil error still fails the test.
