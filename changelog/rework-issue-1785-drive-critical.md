### Fixed
- statusgen drives: the critical tier's high-unblocks arm now counts only reciprocated `depends:` edges (the target must also list the dependent in `unblocks:`), so one-sided edges that pass `--lint` at NOTICE tier can no longer lift a brief into the tier that ranks above every score. The ordinary Next-up score still counts every declared edge.

### Added
- statusgen `--next-up`: while a drive is active, the drive worker-pool floor (`driveWorkerCap`, 6) binds the dispatch queue. It offers at most the cap minus the drive work already in flight (claimed items the drive covers). It withholds the rest and counts them in `heldByDriveWorkerCap`, and it offers no drive pick when claims could not be read (`driveWorkerUnknown`). With no active drive the payload is unchanged.
- `deskboard dispatch` applies the same floor across every configured root, summing in-flight drive work, and names the floor in its held-back line.
