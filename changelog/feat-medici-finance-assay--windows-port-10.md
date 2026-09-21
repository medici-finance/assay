### Added
- `statusgen verifyrun --in-container` runs a brief's Verify rows inside the pinned harness
  container instead of on the host — the supported execution-witness runner on Windows, where a
  native `pipefail` bash is unreliable (the WSL-launcher case that made verifyrun record
  could-not-run for a whole table). It reads the harness image from the new `harness:` block of
  `plugins/assay/paired-versions.yaml`, resolves it by its sha256 **digest** (never a floating
  `latest`), bind-mounts the checkout at `/work`, maps `--user` to the host uid:gid on POSIX so the
  Evidence the container writes lands host-owned, and forwards credentials only as an `--env-file`
  path (never baked or logged). It **refuses fail-closed** on a `latest` tag or an absent/placeholder
  digest — an un-digest-pinned image is never run.
- A `verify-in-container` Windows CI leg (staged, pending maintainer promotion) proves the execution
  witness actually lands through the container; a held `windows-verify-in-container` job records the
  native-Windows-host proof as BLOCKED pending a Windows runner with a Linux-container Docker backend.
