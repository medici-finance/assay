### Changed
- `cellctl` (Go launcher): the seeded Kimi provider defaults and the example model policy now pin the **strong** and **mid** tiers to `k3-256k` (same K3 weights, fixed 256K context, lower quota rate); **top** and **fast** keep `k3[1m]`. Existing `providers.json` files are never overwritten, so this reaches new installs only.
- `cellctl desk` (Go launcher, host Claude launches) now exports `CLAUDE_CODE_AUTO_COMPACT_WINDOW=200000` so a long-running desk session compacts before its context grows unbounded; a value in `cell.env` or the launching environment overrides it.

### Fixed
- Model-policy validation now applies the Kimi K3 low/high/max effort restriction to `k3-256k` as well as `k3`; the 256K model ID previously escaped the check.
