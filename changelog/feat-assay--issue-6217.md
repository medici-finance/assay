### Fixed
- `cellctl` no longer carries the last run's verb into later in-process cell loads, so a direct load after a run cannot take a cell's admission lock and leave a `.reset.lock` behind. This fixes a test-order failure in the cellctl tests that appeared when other test groups ran first.
