### Changed
- The CI jobs that needed a native Windows runner are retired (#4106): the Windows CI leg
  (desk suite, `statusgen --lint`/`--version` smoke, bootstrap hash-check smoke, the held arm64
  smoke and the in-container witness job), the native cellctl launch-contract workflow, the
  Windows git-maintenance workflow and the bootstrap hash-check smoke script. No CI job now runs
  anything on native Windows, including the bootstrap's runtime sha256 refusal of a tampered
  download. The Windows release cross-compiles, `scripts/bootstrap-windows.ps1`,
  `scripts/build-windows.ps1` and the Linux-side Windows build checks are unchanged.
