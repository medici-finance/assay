### Fixed
- `harness-portability/07`'s Verify row 6 (`docs/streams/harness-portability/brief-07-adoption-live-smoke.md`)
  read the RELEASE-NOTES version to match from `plugins/assay/.claude-plugin/plugin.json`'s
  `.version` (the umbrella/plugin-manifest version), which drifted out of step with
  `RELEASE-NOTES.md`'s bundle-content version headings and made the row fail as written on
  merged main with nothing wrong in the release notes themselves. Row 6 now reads
  `plugins/assay/SOURCES.yaml`'s `bundle-version` — the same authoritative source row 5
  already uses — so both rows key off one version scheme.
