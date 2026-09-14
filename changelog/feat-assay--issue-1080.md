### Fixed
- `cellctl desk`'s claude-harness boot no longer prints a misleading "could not enable
  assay@assay — reinstall it" NOTICE when `claude plugin enable` fails only because the
  plugin is already enabled; the NOTICE is still printed for a genuine enable failure.
