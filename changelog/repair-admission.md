### Added
- `deskdispatch` can now ENFORCE the worker pool's repair (`rework`) reservation at the dispatch
  boundary, opt-in via `ASSAY_REPAIR_ADMISSION=on` (recorded policy `repair-admission-v1`). When
  on, a **fresh** dispatch that would drop the free slots to or below the reserved floor while a
  repair obligation is runnable is refused (exit 5, naming the waiting repair); the repair itself is
  admitted. Previously the reservation was only PRINTED by `fanoutloop plan`, so a caller could
  ignore it and fill every reserved slot with fresh work while a repair waited.
- The gate serialises admission across dispatchers with a compare-and-swap lease in the existing
  claim backend (so two hosts cannot both admit into the last slot), resolves an item's class from
  the authoritative repair-obligation store (a caller cannot relabel fresh work as a repair), and
  treats a waiting-external repair as non-runnable so it never idles a usable slot. Unreadable
  occupancy or demand is a visible could-not-check (exit 6), never a fabricated free slot.

### Changed
- Ships **OFF**: with `ASSAY_REPAIR_ADMISSION` unset, `deskdispatch` behaves exactly as before —
  the reservation stays advisory and no dispatch is held. That unset state is also the rollback.
  Repair-obligation records stay additive and readable by an older reader.
