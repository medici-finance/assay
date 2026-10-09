### Changed

- **worker-desk skill and Claude Code reference: how a session below strong tier launches a
  strong-tier worker.** The skill said a strong item "goes only to session-tier", which left a
  worker window running at a provider's mid tier with no stated way to dispatch one: its default
  launches landed on the mid model and stopped at the kit's pickup check. The skill now says such
  a session names the strong tier explicitly in the launch, confirms before the dispatch that the
  slot it names is pinned to a strong-tier model, stamps that pinned id, and holds the item when it
  cannot confirm; the Claude Code reference states that a `model` alias is a slot whose model the
  launch pins and carries no tier claim by itself, where the session reads the pin, the form the
  stamp takes, and which documents own the launcher's maps. No tool, kit text or gate changes.
