### Added
- `cellctl` now ships inside `desk-tools-<platform>.tar.gz` (every platform gets the same file —
  it is a shell script, not a per-platform Go build) and `make desk-install`, sha-pinned by the
  umbrella `checksums.txt` like every other desk-tools asset. No more hand-copied script.
- `cellctl --version` (also `cellctl version`) reports the umbrella release tag a packaged copy
  ships at, stamped into the tarball's copy at release time; a source checkout keeps reporting
  `dev`, honestly.
