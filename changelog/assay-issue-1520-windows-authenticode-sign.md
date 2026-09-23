### Added
- `scripts/build-windows.ps1` gains an **opt-in** `-Sign` switch that Authenticode-signs the
  built Windows PE files with a local self-signed code-signing certificate, before
  `desk-manifest` hashes them. Off by default (unsigned builds are unchanged); resolves the cert
  from `-CertThumbprint`, `$env:ASSAY_CODESIGN_THUMBPRINT`, or subject `CN=Assay local tools`, and
  **fails closed** with a setup snippet when `-Sign` is set but no code-signing cert is found.
- `docs/adopting-assay.md` documents the from-source Windows signing loop, the tightly-scoped
  Trusted-Root import (code-signing EKU, CurrentUser, this-machine-only) and how to remove it, the
  honest AV limit (signing names the publisher but does not clear ML/heuristic verdicts — a
  per-machine folder exception on the install dir does), and notes signed **release** assets as a
  documented follow-on that needs a real certificate.

### Changed
- `desk-build` now removes-then-writes each `dist\*.exe` so a sign → rebuild → re-sign loop works
  where recent Go's `go build -o <existing.exe>` refuses to overwrite a signed PE.
