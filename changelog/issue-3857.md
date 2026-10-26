### Changed
- The desk role images now create the `desk` account with UID/GID pinned to 999 (the value the Kubernetes manifests run as) and fail the image build if it resolves to anything else; a desk-tools test holds the Dockerfile and all five role manifests to that one value on every PR.

### Added
- The Kubernetes desk package keeps its release's own `release-images.json` byte for byte, and `containers/k8s/check.sh --base` requires it to match the sha256 pinned in the check, validates it with the release image contract and rejects any role image that is not exactly the reference that release build recorded for that role.
