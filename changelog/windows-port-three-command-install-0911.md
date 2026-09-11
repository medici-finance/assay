### Added
- `windows-port` stream extended with four briefs (06-09) covering the **three-command Windows
  install**: a manifest-driven PowerShell bootstrap that resolves its own pinned tag + sha256 and
  writes PATH (06); a `deskinstall --harness cursor` mode that places the skills/references tree
  and the `AGENTS.md` bindings idempotently, with a `--check` drift report (07); a Go-native GitLab
  fleet-provisioning verb replacing the bash + curl + jq script so native Windows needs no
  Git-Bash/WSL (08, `gate: human` — it mints live access tokens); and the docs/scope delta that
  widens the install skill's Windows scope from acquisition-only to the whole install, collapses
  the adopter walkthrough, and corrects the stale "CI leg is staged" claim (09).
