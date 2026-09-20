### Added
- `docs/streams/desktools-v2/inventory.md`: a frozen, file:line-accurate inventory of every
  site that reaches past the `Forge` seam (`tools/desk/internal/deskkit/forge.go`) with a
  GitHub-specific fact or an ambient credential — 69 sites across `statusgen/**`,
  `tools/desk/**`, `tools/cellctl/**`, `plugins/assay/` skill scripts, and
  `.github/workflows/**` — reconciled against the existing `forgeban` permit register, routed
  to the migrating brief that owns each site, and flagging the largest undocumented finding: a
  17-site hand-rolled GitHub REST+GraphQL client living in `deskpost/github.go` outside both
  sanctioned backends. No code changed; this is the `desktools-v2` stream's audit brief.
