### Fixed
- `statusgen` verify-gate cards now treat a brief-v1 `<stream>/<NN>` key and a
  brief-v2 `<cell>:<repo>:<stream>:<NN>` key as ONE identity. The idempotency
  marker renders and matches in the canonical `<stream>/<NN>` form, and
  `--close-verify` accepts either form — so a v1→v2 migration no longer re-files a
  duplicate sign-off card for every already-carded brief.
