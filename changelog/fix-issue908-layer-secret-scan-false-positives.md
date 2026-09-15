### Fixed
- `containers/scripts/layer-secret-scan.sh` no longer flags toolchain material
  it never wrote as a secret: Go's own stdlib test fixtures, npm's bundled
  docs, and PEM-shaped strings compiled into `gpgv`/`libssh2`/`libgnutls` are
  now excluded by a narrow, commented path allowlist, the generic `sk-` key
  shape is gated to text-shaped content (no longer checked against compiled
  binaries), and each layer-filesystem hit is reported once instead of twice.
  Verified clean (`exit 0`) against a real build of the desk base image, with
  a new mutation-test fixture proving no bypass for a real secret at an
  ordinary path.
