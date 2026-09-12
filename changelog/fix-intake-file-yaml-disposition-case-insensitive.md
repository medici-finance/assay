### Fixed
- `statusgen`'s per-entry file parser (`parseIntakeFile`) now matches the YAML frontmatter `disposition` key case-insensitively, preventing title-cased or uppercase keys (such as `Disposition: accepted` or `DISPOSITION: rejected`) from being silently ignored by `gopkg.in/yaml.v3` struct-tag matching and falling back to the untriaged `new` state (#931).
