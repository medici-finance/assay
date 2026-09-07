### Added
- Windows install path (`windows-port/03`): a Go-native `deskinstall` command that
  mirrors the Unix acquire→verify→place flow — detects `windows-amd64`/`windows-arm64`,
  resolves the pinned tag + per-platform sha256 from `paired-versions.yaml` (never a
  floating ref), downloads the `statusgen-windows-<arch>.exe` and
  `desk-tools-windows-<arch>.tar.gz` assets, and **verifies each sha256, refusing on any
  mismatch before anything is placed** (nothing is installed on a bad hash).
- `scripts/bootstrap-windows.ps1`: a minimal PowerShell first-install bootstrap that
  fetches only `statusgen` and hash-verifies it before executing, keeping the
  security-critical hash-verify in one tested Go implementation.
