### Changed
- Every Go module now declares `go 1.27.0`, and the container builds, staged workflows and workflow proposals pin the one Go toolchain, 1.27.2, with the reviewed go.dev/dl digest. The go-analysis toolchains stay where they are until their own baseline migration.
- Every `actions/upload-artifact` step uses v7.0.2. This is the version the repository already uses elsewhere.

### Added
- A single-version guard (`TestGoSingleVersion` in desk-tools) goes red when any go.mod directive, workflow `GO_VERSION` or literal `go-version`, Go tarball digest, `golang:` base tag, Dockerfile `GO_VERSION` ARG or upload-artifact pin disagrees with the fleet version. Its exception list fails on a stale entry, so the list can only get shorter.
