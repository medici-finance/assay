### Added
- `tools/desk/README.md` gains a "Hotfix cut" runbook beside the hotfix overlay: an urgent fix ships as the next plain patch release through the existing release workflow (dry run, then the real cut, each approved on the `release` environment), followed by one re-pin change, the install, and `cellctl hotfix clear`.
