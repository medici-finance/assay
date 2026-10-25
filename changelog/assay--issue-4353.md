### Fixed
- The verify-runner image's `stage-source.sh` now accepts real GitHub App installation tokens, which carry `.` and `-`, at both the custody check and the credential helper; each site still refuses any other character and any token longer than 2048 characters (#4353). The fix takes effect once a release rebuilds the image.
