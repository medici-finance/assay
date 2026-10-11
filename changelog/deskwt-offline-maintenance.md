### Added
- `deskwt maintain` plans and applies bounded offline ref packing and commit-graph updates, with a shared-repository lock, attempt cadence, and before/after inventory. Object reclamation remains held until borrowed-object ownership can be established.
- Maintenance requires native Git hook inventory and holds repositories with listed relevant entries or unverifiable configuration, preserving existing hooks. Unrelated events remain usable.
