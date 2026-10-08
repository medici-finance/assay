### Added
- `docs/brittle-investigation-template.md`: the task template for a brittle-marked module. A strong-tier session reads the module's original intent (brief, decision record, `S-` row) and what has happened since (class instances, fix commits, findings), picks one divergence (`drifted`, `intent-changed`, `intent-right, implementation-wrong`, or `none`) and one recommendation (`reconcile` with a deletion bundle, `redesign`, `accept`, or `clear`), and gives each conclusion the evidence it rests on, so a later policy change shows which conclusions need revalidation. Includes a worked example.
- `docs/investigations/`: where investigations land, one dated file per module.

### Changed
- worker-desk: a class issue labelled `brittle` dispatches at strong tier with the investigation template as its deliverable; a design brief follows only on `redesign`.
