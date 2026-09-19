### Fixed
- `statusgen --corroborate` no longer reads the on-behalf-of attribution form (`on-behalf-of human:<login>` in a Runner cell or prose, and the `On-behalf-of:` trailer) as a `human:<name>` sign-off stamp or an acceptance citation; only sign-off vocabulary is judged, so App-authored Evidence rows carrying the principal the attribution lint requires no longer red the checker (#1335).
