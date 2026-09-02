### Added
- **`clusterguard`** — an exec-boundary shim for cluster CLIs. Installed as a directory of
  symlinks (`kubectl`, `flux`, `helm`, `talosctl`, `k9s`) on the front of a session's `PATH`, it
  refuses every shimmed CLI by default with exit `5`, records both verdicts to
  `<config-home>/clusterguard.log`, and execs the real CLI only when an operator shell exported
  `ASSAY_ALLOW_CLUSTER` — `=1` for read-only verbs, `=mutate` for everything, any other value
  refused rather than guessed. This catches what a command-text permission rule cannot: a cluster
  call made from inside a committed script never matches a text rule, but it still resolves the
  CLI name on `PATH`. Read-only classification is a per-CLI **allowlist**, so an unclassified verb
  is treated as mutating; `k9s` has no read-only lane at all, being an interactive TUI that can
  mutate from inside the session. A stop flag can only make the guard stricter — an armed kill
  switch refuses (exit `3`) rather than making a refusal-guard stop intercepting, which would fail
  open. Its limits are stated rather than implied: an absolute-path invocation is never
  intercepted (there is a test asserting that bypass exists), and the guard is not a network
  boundary. Contract, verdict table and limits: `tools/desk/README.md`; install notes:
  `docs/adopting-assay.md`.
