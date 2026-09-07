### Added
- Windows adopter walkthrough (`windows-port/05`): `docs/adopting-assay.md` gains a
  **Windows adopters** section — the native-Windows arm of the install step every scenario
  references. It documents the pinned, sha256-verify-or-refuse install path (the
  `scripts/bootstrap-windows.ps1` first-install bootstrap + the Go-native `deskinstall`
  command), the `statusgen-windows-<arch>.exe` / `desk-tools-windows-<arch>.tar.gz` pins for
  `.assay-versions`, and the native-not-WSL claim (WSL noted only as a local-dev fallback).

### Changed
- The two Windows "deferred fast-follow" deferrals are retired: `docs/adopting-assay.md`'s
  Prerequisites and `plugins/assay/skills/install/SKILL.md` §Scope now point at the real
  Windows install path instead of stopping at Windows.

### Notes
- The walkthrough mirrors the delivered state honestly, not aspirational parity: it lifts the
  SessionStart-hooks `documented-workaround` (install Git-Bash for `bash`+`jq`) verbatim from
  the portability audit, states the Windows CI leg as **staged (PR #569), pending a
  maintainer's promotion into `.github/workflows/`** rather than a live green check, and marks
  the native `windows/arm64` smoke **BLOCKED** pending an arm64 Windows runner (the arm64
  asset still ships cross-compiled + checksummed).
