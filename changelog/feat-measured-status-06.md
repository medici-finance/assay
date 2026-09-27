### Added
- `ASSAY_STAMP_TRUSTED_LOGINS` roster key: an explicit, fail-closed stamp-authority allowance — the trusted human logins whose `dispatched-*` label applications the model-floor's actor check honours in addition to the bound dispatcher slugs (#336). Unset vouches for nobody beyond the dispatcher slugs; entries must already be trusted logins; an unconfigured roster vouches for nobody.
- `deskrestamp`: first-class re-stamp verb that removes a foreign-applied `dispatched-*` stamp and re-applies the same (model, tier) pair under the session's dispatcher App, preserving content and posting a per-PR record comment naming both the original applier(s) and the re-stamp actor.

### Changed
- The model-capability floor's actor check (`deskflip`, `deskpost`, `deskautolane`, and the `deskdispatch` stamp step) now reads `IsStampAuthorityLogin` — the bound dispatcher slugs plus the roster-configured allowance — so a legitimately dispatched PR stamped by an allowed trusted login is no longer refused a verdict.
