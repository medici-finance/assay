### Changed

- **worker-desk skill and Claude Code reference: how a session below strong tier launches a
  strong-tier worker.** The skill said a strong item "goes only to session-tier", which left a
  worker window running at a provider's mid tier with no stated way to dispatch one: its default
  launches landed on the mid model and stopped at the kit's pickup check. The skill now says such
  a session names the strong tier explicitly in the launch and stamps the model the worker was
  launched on; the Claude Code reference states which `model` alias resolves to which provider
  tier under a `cellctl` provider launch. No tool, kit text or gate changes.
