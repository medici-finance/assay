### Added
- `scripts/bootstrap-windows.ps1` resolves the pinned tag + sha256 for the detected
  `windows-<arch>` from the committed `plugins/assay/paired-versions.yaml` manifest instead of
  demanding the sha256 as a mandatory, hand-transcribed parameter; `-Sha256` is now an optional
  override that must agree with the manifest or refuse. The script also writes the resolved
  install directory onto the current user's `PATH` (idempotent, segment-guarded) and additionally
  places the verified binary as `statusgen.exe`, so `statusgen --version` resolves by bare name
  in a new shell after step 1 of the Windows install.
- `scripts/windows-bootstrap-hashcheck-smoke.ps1` gained manifest-tamper and absent-platform-line
  negative-path assertions (with their own non-vacuity controls), alongside its existing
  hash-mismatch and override-disagreement checks.
