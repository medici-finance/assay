### Fixed
- **Restored the two Windows pin lines in `examples/adopter-scaffold/.assay-versions`** dropped
  by a later, unrelated rewrite of that file. windows-port/01 added
  `statusgen-windows-amd64`/`statusgen-windows-arm64` illustrative pin lines; the derived-board/06
  migration rewrote the whole file onto a new schema (umbrella line, fixture-placeholder digests)
  and did not carry the two lines forward, silently regressing the brief's Verify row 7. The two
  lines are re-added at the file's current pin tag, following the ONE TAG, ONE TREE rule and the
  same fixture-placeholder convention every other artifact line in the file already uses.
