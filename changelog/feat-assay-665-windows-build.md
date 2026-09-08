### Added
- `scripts/build-windows.ps1`: a PowerShell counterpart to the root `Makefile` that
  builds and installs the desk-tools on Windows (`.exe` outputs, a per-user
  `%LOCALAPPDATA%\Assay\bin` install, `Get-FileHash` manifests) with no `nmake`,
  Visual Studio build tools, or `make` required — only PowerShell and the Go toolchain.
- `tools/winparity`: a fail-closed, three-state parity guard that asserts the Windows
  build script's target set equals the Makefile's `.PHONY` set, so the Windows build
  cannot silently fall behind (or run ahead of) the Unix target set. The Windows script
  runs it as a preflight; a staged `ci/staged-workflows/winparity.yml` is the Linux-CI half.
