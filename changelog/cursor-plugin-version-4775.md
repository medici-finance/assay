### Fixed
- Cursor installs now report the release they hold. `harnessgen cursor` generates a Cursor plugin manifest (`plugins/assay/.cursor-plugin/plugin.json`) and a root Cursor marketplace (`.cursor-plugin/marketplace.json`). The release stamp, check and gate keep the manifest's `version` equal to the tag; the marketplace entry carries no version. A Cursor team-marketplace import follows the branch it tracks, not the tag.
- The Cursor copy-skills inventory (`.cursor/assay-managed.json`) now records the placed plugin `version`, and `upgrade-assay --harness cursor` prints it beside the selected release. An older `deskinstall` refuses an inventory carrying the new field rather than guessing; place with the matching release's binaries.

### Changed
- The Cursor reference and adoption guide describe both Cursor routes: a versioned copy-skills install that pins and upgrades by umbrella tag, and a Cursor plugin import that follows a branch. They no longer say Cursor has no marketplace.
