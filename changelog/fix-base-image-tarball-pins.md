### Changed
- The desk base image now pins every base image by tag and digest, and checks the Go, `gh` and Node tarballs against pinned per-architecture sha256s (`sha256sum -c`, before unpacking), as it already did for git. A static test fails if any pin or check is dropped or masked, and `containers/README.md` lists every pin with its bump procedure (#2320).
