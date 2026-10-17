### Changed
- Use one protected publication job for all nine release images, tag creation and GitHub release assets, so an umbrella release needs one release-environment approval instead of two. Preserve image/version checks, immutable references and the normalized-version publication lock; keep changelog rolling independently retryable.
- Use the board-writer App client ID for changelog rolling, replacing the deprecated numeric App ID input.
