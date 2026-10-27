### Changed
- `statusgen verifyrun`: an admitted run's Verify rows no longer inherit the forge credential variables `GH_TOKEN`, `GITHUB_TOKEN`, `GH_ENTERPRISE_TOKEN` and `GITHUB_ENTERPRISE_TOKEN` from the invoking environment; a row sees a forge credential only when its own command supplies one. The invoking process keeps them, so verifyrun's own forge reads are unchanged (#4612).
