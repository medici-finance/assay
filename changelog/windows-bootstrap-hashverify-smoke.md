### Added
- **Windows-runtime hash-verify smoke for the PowerShell bootstrap (windows-port/03, decision
  #508).** A new `scripts/windows-bootstrap-hashcheck-smoke.ps1` exercises
  `scripts/bootstrap-windows.ps1`'s sha256 verify on `windows-latest`: a tampered checksum must
  REFUSE (throws on the mismatch, nothing installed), the pinned checksum installs, and a
  check-removed copy installs the tampered asset — proving the refusal is non-vacuous. It runs in
  a dedicated `windows-bootstrap-smoke` job (staged in `ci/staged-workflows/windows-ci-leg.yml`,
  maintainer-promoted), kept SEPARATE from the offline `windows-smoke` job because the bootstrap
  downloads a release asset — the sanctioned decision-#508 online exception, so `windows-smoke`'s
  offline envelope stays intact. Recorded as Verify row 8 on windows-port/03. (#595)
