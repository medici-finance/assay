### Changed
- Managed Go caches: the docs now say why final consumer enrollment, and the `cellctl cache-run` pane supervisor that passes through it, refuse a stale, unstamped, future-dated or missing measurement instead of re-measuring it, and a new test pins that refusal (no child starts, no clean runs, `report.json` unchanged).
