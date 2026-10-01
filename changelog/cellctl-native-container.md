### Added
- `cellctl` manages local Docker cells directly in Go through `CELL_CONTAINER_CONFIG`, including registration, preflight, status and shutdown. Native cells no longer need an external host launcher.

### Fixed
- A missing container console can be recreated and attached to the same verified running container. Repeated matching model arguments reconnect, failed Docker inspection remains an error, and startup failures retain their console diagnostics.
- `status` and `down` act on the running container's own harness and model, so a per-launch override is reported and stopped, and one role's refusal no longer skips the others. `check` reports every role's file and Docker check failures together.
- Native containers are checked for more runtime settings (entrypoint, extra environment, groups, ports, user/IPC namespaces), the cell network must carry the cell label, and directory mounts are validated against protected paths and the Docker socket; `/` is refused. Model pins with control characters are refused, and the console runner is started without a shell.
- Directory-mount containment compares file identity when a plan is built and before launch, so another spelling of a protected directory (symlink, case or normalization variant, filesystem alias) is refused. The operator's home directory and the cell's own directory are protected too, and a container with device requests (such as GPUs) is refused on reconnect.
