### Fixed
- `docs/streams/desk-containers/brief-01-base-image.md`'s Verify row 1 build command now
  uses the working root-context form (`docker build -f containers/base/Dockerfile -t
  assay-desk-base:dev .`, run from the repo root) instead of the broken
  `containers/base`-context form, which failed because the Dockerfile `COPY`s
  `plugins/assay/` from the context root. `containers/README.md` also gains a note that a
  plain (non-buildx) arm64-host build needs `--build-arg TARGETARCH=arm64`.
