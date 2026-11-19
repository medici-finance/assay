### Fixed
- The verifier can finish preparing its fresh module cache after the public Go proxy interrupts a download. It retries up to twice within the same open-bracket run and still refuses incomplete or changed inputs.
