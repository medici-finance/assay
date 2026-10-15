### Fixed
- `deskdispatch` prints claim commands that run as printed: when the pure-Go claim binary is in use, every release, progress and steal line it emits (in the agent prompt and in its own hints) carries `--token-file "$(desktoken --no-rotate <role> --repo <repo>)"`, so it no longer exits 6 for want of a forge token.
- The review assignment names `deskpost finish` (post, confirm and release in one step) for the lane its claim key names; the separate release line stays as the fallback.
- The review and worker kits let a packet larger than one read be read whole in ranges sent together; the review kit states the `wrote-to-the-test` approve obligation and how to read `deskpathguard check` (the `state=` line, not the exit code) and a `statusgen verifyrun` identity refusal.

### Changed
- `deskpathguard --help` and `docs/protected-paths.md` say that a labelled (`checked-failed`) check exits 0 and that the `state=` line is the result.
