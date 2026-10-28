### Added
- The `verify-runner` release image is now multi-arch (`linux/amd64` and `linux/arm64`), built from a new `statusgen-linux-arm64` release asset, so arm64 hosts run `go test` natively instead of under emulation (#4323). `release-images.json` and `verify-runner-image.txt` pin the image index digest, which covers both platforms; records from older releases stay valid.
