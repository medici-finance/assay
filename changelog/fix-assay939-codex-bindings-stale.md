### Fixed
- `plugins/assay/references/codex.md`'s `capability:isolate-workspace` row (and the two skill rows
  that derive from it, `pr-shepherd` and `worker-desk`) re-measured on **codex-cli 0.154.0**: the
  CLI now ships a managed-worktree mechanism (`codex exec --worktree`), so the prior `§3.9 absent`
  reading is stale. The floor behaviour is unchanged — still refuses under `workspace-write`,
  still runs under `danger-full-access` — only the mechanism classification updates. (#939)
- `docs/adopting-assay.md` §3's `multi_agent_v2` could-not-check is narrowed: `codex features list`
  on 0.154.0 confirms the key exists (`stable`, defaults `false`); only the V2 tool-name behaviour
  itself remains unexercised. (#939)
- `docs/adopting-assay.md` §1's install arm A (plugin/marketplace) is recorded as demonstrated, not
  merely documented: a clean Codex home on 0.154.0 ran
  `codex plugin marketplace add` → `codex plugin add` → `codex plugin list` end to end against this
  repo's legacy manifest, with all twelve skills discoverable namespaced `assay:<name>`. (#939)
