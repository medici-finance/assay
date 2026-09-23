### Added
- Dispatch claims are now kept in whatever store the roster names: a new `ASSAY_CLAIM_STORE` roster key (`file` or `service`), with `ASSAY_CLAIM_DIR` and `ASSAY_CLAIM_SINGLE_HOST`. `deskclaim-ref` and `deskdispatch` both ask one resolver, and no flag selects the store. A configured store that cannot be used is refused (exit 6) before any worktree is cut or credential minted, and is never replaced by another store. The `file` and `service` stores are not in this release, so setting either is refused and the message names the brief that ships it.

### Changed
- With `ASSAY_CLAIM_STORE` unset, claims stay on the forge exactly as before. Every `deskclaim-ref` and `deskdispatch` run now prints a NOTICE that this default will be removed, and names the release it goes in. Setting `forge-ref` explicitly is refused. The `claim-acquire OK` line names the store (`store forge-ref (legacy)`).
