### Fixed
- The board regen job on the default branch now checks out full history (`fetch-depth: 0`), like the lint job. On a shallow checkout a changelog fragment consumed by a release read as a missing path, so the default-branch board regen failed while PR lint passed.
