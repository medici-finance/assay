### Fixed
- `statusgen`'s issue scanner no longer silently drops or retypes a GitHub
  label whose text is a YAML type keyword or number (`null`, `~`, `true`,
  `false`, `yes`, `no`, `on`, `off`, `123`, `1.5`) when it writes the
  `labels: [...]` flow list into a generated placeholder's frontmatter. Each
  label is now explicitly string-tagged on write, so it always re-parses back
  as the original string instead of being resolved to `null` (dropped), a
  boolean, or a number. (#1431)
