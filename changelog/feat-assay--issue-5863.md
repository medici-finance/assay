### Fixed
- deskd's OpenAPI generator recognises `json.RawMessage` by type identity again. From Go 1.27 that type is an alias of `jsontext.Value`, so the old package-path-and-name match stopped firing and the contract's `Event.data` field was emitted as a base64 string. The committed `openapi.yaml` is regenerated to the correct form.
- The `cmd/deskd` tests no longer leave `deskd-index.db` (and its `-shm`/`-wal` files) in the module tree. Every test that runs the binary goes through one helper that points the index into a temp dir, and the package fails if a run leaves a state file behind.
