### Fixed
- `cellctl up`, `down` and `reset` no longer wedge on a Herdr workspace receipt left without an ID by a create that never finished: `up` recreates the workspace under a fresh owner token and `down`/`reset` drop the receipt, with a stderr note that a prior attempt may have left an untagged workspace labelled with the cell name. The ownership check against a live workspace is unchanged.
