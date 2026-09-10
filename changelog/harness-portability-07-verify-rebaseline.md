### Fixed
- **harness-portability/07 Verify rows 5 and 7 re-baselined to the current tree, so they
  test what they claim.** Row 5 read the bundle version from `.version` on the plugin
  manifest — plugin.json in the plugins/assay/.claude-plugin directory — and compared it
  to a live `origin/main` ref; in the public tree that field tracks the plugin-manifest /
  umbrella release (now `1.0.0`), not the bundle content version, and a live-ref compare
  can never discriminate on merged main. It now reads the authoritative `bundle-version`
  from `plugins/assay/SOURCES.yaml` and asserts it is at least `0.3.0`, the version that
  records Assay's second first-class harness — still discriminating (a regression exits
  `1`), with no moving anchor. Row 7's run-log check anchored `Result:` at column 0, but
  the run-log skeleton in `docs/codex-smoke-protocol.md` writes each verdict as an
  INDENTED `  Result:` line, so a complete run log would have counted zero and falsely
  FAILed the stream's acceptance row; the anchor is now `^[[:space:]]*Result:`. Row 7
  stays BLOCKED pending the live Codex run and the brief stays `implemented` — no status
  advance.
