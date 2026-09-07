### Added
- `deskavatar` — a new desk tool that generates the deterministic, on-brand
  avatar set an adopter uploads for their Assay Apps. It composes an
  octagon-framed SVG per App, rasterises it offline to PNG (pure-Go, no cgo), and
  runs a 20 px legibility proof (CIELAB ΔE + glyph-silhouette IoU) over the set
  before writing anything — refusing (exit 5) and naming the pair if two tiles
  would be indistinguishable in a PR timeline. Same org, same tier, same bytes.
  Importable as `internal/avatar.Generate` for the installer to call in-process.
