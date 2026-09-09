### Added
- `deskclaim-ref` — a pure-Go port of the consumer dispatch-claim script (verbs
  `acquire`/`progress`/`release`/`steal`/`show`/`list`). It speaks the same durable,
  cross-machine claim protocol (the `refs/dispatch/<id>` ref namespace, holder encoding, and
  0/5/6 exit codes) with no shebang and no bash — the only claim path that runs native on a
  Windows adopter.

### Fixed
- `deskdispatch` now dispatches on a freshly adopted tree that carries **no**
  `tools/dispatch-claim.sh`: its claim-acquire step falls back to the pure-Go `deskclaim-ref`
  binary on PATH when the consumer script is absent, so a green-field (including
  native-Windows) adopter dispatches without the consumer script on disk and without a tribal
  `--claim-root`. A repo that still carries the script keeps using it unchanged, so a Go and a
  bash dispatcher collide on the same claim ref and never double-dispatch during the
  transition.
