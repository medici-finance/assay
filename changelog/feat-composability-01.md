### Added
- `tools/desk/internal/deskkit`: component ACTIVATION (`activation.go`) — a component is
  ACTIVE iff every declared `inject.required` key resolves; an INACTIVE component's owning
  verb refuses with a three-state `could-not-check: assay/<component> inactive — <key>
  <reason>` before doing any work. Wired into every desk command's shared entrypoint
  (`deskkit.CheckVerbActivation`, called right after `EchoEffectiveConfig`).

### Changed
- The roster loader now splits TRUST-surface validation (unchanged, fail-closed) from
  EXTENSION-key validation: a malformed `ASSAY_REPO_ALIASES`, `ASSAY_REPO_FORGES`,
  `ASSAY_RISK_CALLOUT`, `ASSAY_WRITEGUARD_CALLOUT`, `ASSAY_RELEASE_REPO`, or
  `ASSAY_SCAN_REPOS` value no longer collapses the WHOLE configuration to unconfigured — it
  is recorded per-key on the new `Config.Ext` map (`ExtKeyResult`), the affected field falls
  back to its own shipped default, and only a component that actually requires that
  extension key goes INACTIVE. The five trust surfaces (`ASSAY_BLESS_LOGIN`,
  `ASSAY_TRUSTED_LOGINS`, `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`,
  `ASSAY_HUMAN_LOGIN_MAP`) are unchanged: unset or malformed still refuses every trust-gated
  verb.
