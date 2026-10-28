### Fixed
- The release job registers arm64 emulation itself, from a digest-pinned `tonistiigi/binfmt` installer that runs before the registry login, and proves a `linux/arm64` container runs before building images. The multi-arch verify-runner build no longer fails with `exec format error` on a runner without emulation.
