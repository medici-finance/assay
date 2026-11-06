### Fixed
- `statusgen verifyrun --fail-first` now accepts a deliverable-repository `--base` on a cross-repository (version 2 receipt) run on both lanes. On the host lane the brief-held check is skipped and the ancestry check still applies. In the container lane `--history-pack` imports history for a brief that the receipt binds outside the home.
