### Fixed
- `tools/desk/cmd/cellctl` (the shipped Go binary) now honours `CELL_MODEL_POLICY`: `set`/`show`
  accept and display the key, the policy JSON is schema-validated (harness/tier shape, exact
  model IDs, per-harness effort levels, deny list), per-role provider/model/effort resolution
  drives `desk` and `DRY_RUN=1` dry-run output (including the policy file's sha256), effort
  propagates into the Claude/Codex launch env and argv, a denied model (e.g. `*opus-5*`) refuses
  the launch outright, and a child-model request is resolved and effort-checked against the same
  provider's tiers. Previously the Go binary silently ignored the key entirely (assay#1390).
