### Fixed
- House desk launches now own task scratch, reclaim acknowledged completed output, retain bounded failure diagnostics, and conservatively recover abandoned tasks. New scratch commands provide bounded source snapshots, explicit evidence handoff, cleanup reports, and legacy inventory.
- Scratch sources now require a working-tree root and share admission across revision lookup, snapshots, and declared inputs. Inputs remain within that source repository.
