### Added
- `cellctl` manages local Docker cells directly in Go through `CELL_CONTAINER_CONFIG`, including registration, preflight, status and shutdown. Native cells no longer need an external host launcher.

### Fixed
- A missing container console can be recreated and attached to the same verified running container. Repeated matching model arguments reconnect, failed Docker inspection remains an error, and startup failures retain their console diagnostics.
