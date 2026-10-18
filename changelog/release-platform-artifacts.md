### Changed
- Transfer prepared release artifacts in separate platform bundles so the publisher can download them concurrently. Disable outer ZIP compression for these bundles to avoid recompressing the packaged archives; preserve all binary contents, version stamps and checksum verification.
