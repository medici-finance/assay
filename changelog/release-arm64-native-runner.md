### Fixed
- Build and check the arm64 verify-runner image on a native ARM64 runner, then transfer its image archive and receipt to the release publisher. The publisher validates the release metadata, archive checksum, tag, platform and image identity before publishing the multi-platform index. Release jobs no longer require kernel emulator registration on protected runners.
- The release publisher takes the native ARM64 artifact into a directory of its own and admits only the exact archive and receipt, so a file in that artifact can no longer replace a checksum-verified release input.
