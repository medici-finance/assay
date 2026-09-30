### Fixed
- The reactive-activation refusal names the component by its declared id (`could-not-check: assay/desk-tools inactive — …`) instead of doubling the namespace (`assay/assay/desk-tools`).
- Manifest discovery (desk-verb activation and `deskmanifest lint`) no longer reads manifests from a nested clone, linked worktree or submodule under the checkout, so a stray nested copy can no longer change the parent tree's activation or surface as duplicate component ids.
- Activation picks between duplicate component ids deterministically (first root-relative path wins; the rest are reported as skipped) instead of depending on walk and sort order.
