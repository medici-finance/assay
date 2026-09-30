### Fixed
- `cellctl`'s built-in Opus 5.0 model ban now matches by version rather than by fixed spelling, so a 5.0 model ID carrying a trailing date, provider tail or variant name (`claude-opus-5-20260101`, `claude-opus-5@20260101`) is refused like `claude-opus-5`; the-desk's Opus 5.5 floor no longer reads such a date as a minor version. `claude-opus-5-5` and the other 5.x IDs stay allowed.
