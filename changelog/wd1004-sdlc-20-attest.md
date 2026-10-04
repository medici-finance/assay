### Added
- `deskinstall` now verifies each release asset's build attestation before placing anything: after the sha256 pin check it fetches `<asset>.sigstore.json` from the same release and refuses unless the Sigstore bundle proves SLSA provenance for those exact bytes, signed by this repository's release workflow at the pinned tag and recorded in the transparency log. The check is mandatory, so there is no `--no-attest` or other opt-out. Assets that have no bundle are refused.
- The worker-desk skill adds a rule for new dependencies, plugins and MCP servers: before install, file an issue with an agent-generated risk summary covering source, permissions and persistence. The minimum release age is 7 days, and a younger release also needs a driver `bless`.

### Changed
- **Merge ordering:** a `deskinstall` built from this change refuses every release that does not publish attestation bundles. Releases must ship `<asset>.sigstore.json` beside each asset before adopters re-pin to a `deskinstall` that includes this change.
