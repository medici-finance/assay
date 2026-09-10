### Fixed
- **The SessionStart banner no longer hard-codes a stale plugin version.** It said
  `assay plugin v0.1.0` long after `plugins/assay/.claude-plugin/plugin.json` moved to `1.0.0`.
  `resident-rules.md`'s Header now carries a `{{VERSION}}` token that `harnessgen resident`/
  `harnessgen cursor` resolve from the plugin manifest at generation time — never a literal a
  human can forget to bump — and `--check` reddens if a manifest bump lands without
  regenerating. `inject-resident-rules.sh` now reads the generated payload file instead of
  carrying its own duplicate copy of the rules text, which had also silently drifted from the
  single source on rule 8's wording.
