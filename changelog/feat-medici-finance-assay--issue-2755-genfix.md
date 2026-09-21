### Fixed
- `statusgen --scan-issues` now quotes issue labels correctly in the generated
  `placeholder-v1` frontmatter. A label containing a YAML flow-indicator character
  (for example a trailing `?`) was previously emitted unquoted inside the
  `labels: [...]` flow sequence, producing frontmatter that failed to parse
  (`did not find expected ',' or ']'`) and reddening lint on every scan. The label
  list is now rendered through the YAML encoder, so each element is quoted exactly
  when — and only when — YAML requires it.
