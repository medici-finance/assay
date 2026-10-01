### Added
- `cellctl` manages local Docker cells directly in Go through `CELL_CONTAINER_CONFIG`, including registration, preflight, status and shutdown. Native cells no longer need an external host launcher.

### Fixed
- A missing container console can be recreated and attached to the same verified running container. Repeated matching model arguments reconnect, failed Docker inspection remains an error, and startup failures retain their console diagnostics.
- `status` and `down` act on the running container's own harness and model, so a per-launch override is reported and stopped, and one role's refusal no longer skips the others. `check` reports every role's failures together.
- Native containers are checked for more runtime settings (entrypoint, extra environment, groups, ports, user/IPC namespaces), the cell network must carry the cell label, and directory mounts are validated against protected paths and the Docker socket; `/` is refused. Model pins with control characters are refused, and the console runner is started without a shell.
