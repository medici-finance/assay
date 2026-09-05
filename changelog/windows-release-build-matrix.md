### Added
- The umbrella release now cross-compiles Windows binaries. `release.yml` builds
  `statusgen-windows-amd64.exe` / `statusgen-windows-arm64.exe` and packages
  `desk-tools-windows-amd64.tar.gz` / `desk-tools-windows-arm64.tar.gz` (each `cmd/*`
  binary suffixed `.exe` on the windows legs only), all on the existing Linux release
  runner — Go cross-compiles the Windows targets natively, no Windows host. `checksums.txt`
  now covers all ten assets, so consumers can pin each Windows platform by sha256 in
  `.assay-versions`; the adopter-scaffold example gained illustrative
  `statusgen-windows-amd64` / `statusgen-windows-arm64` pin lines (placeholder hashes —
  the real ones are harvested from the published release). Unblocks the Windows install
  path, CI leg, and adopter-doc work (windows-port/03–05).
