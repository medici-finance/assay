### Fixed
- `statusgen verifyrun` accepts a relative `--brief` path: the acceptance transition resolves it against the working directory before binding the approval payload, so the documented `--brief docs/streams/<stream>/brief-NN-….md` form verifies instead of refusing with `cannot resolve brief path`.
