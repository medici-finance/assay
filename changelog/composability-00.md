### Added
- Component manifests (`component.yaml`) for every unit of the `docs/adopting-assay.md` §2 inventory — the skills, hooks, desk tools, statusgen, and the forge/scaffold-side units — each declaring the keys it `provides` and `injects` and its ordered `apply` steps (composability/00, #624).
- `components/KEYS.md` — the authoritative namespaced-key catalogue, with each key's providing component and meaning.
- `deskmanifest lint` (`tools/desk/cmd/deskmanifest`): discovers every `component.yaml`, resolves every `inject.required` key to a `provides`, checks version ranges, and reports dependency cycles from the declarations alone — three-state (exit 0 clean / 1 problems / 2 could-not-check). Wired into the board-lint CI job so an undeclared dependency is a CI failure instead of an outage.

### Changed
- `component-model.md` §3 now links to `components/KEYS.md` for the key catalogue instead of carrying an inline table.
