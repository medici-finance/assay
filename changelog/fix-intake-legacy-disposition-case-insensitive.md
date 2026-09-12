### Fixed
- `statusgen`'s monolithic intake register parser (`parseIntakeLegacy`) now matches the `Disposition:` key case-insensitively and tolerates surrounding whitespace, preventing lowercase or mixed-case disposition lines (e.g. `disposition: accepted`) from being silently dropped and falling back to the untriaged `new` state (#915).
