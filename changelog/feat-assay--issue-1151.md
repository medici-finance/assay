### Fixed
- `deskdispatch` claim-acquire now runs the claim child as the dispatching role: it mints (or reuses) that role's App token through the same seam its model-stamp step uses and hands it over as `--token-file <0600 path>` to `deskclaim-ref` or as `GH_TOKEN` in the child environment for the legacy `tools/dispatch-claim.sh`. An exported `GH_TOKEN` still wins; a mint refusal is exit 6 with no claim attempted — the claim tool is never run on the ambient `gh` login (#1151).

### Changed
- `deskdispatch` prefers `deskclaim-ref` on PATH over the legacy `tools/dispatch-claim.sh` when both resolve; the script stays the fallback for a tree that predates the binary, and the `claim-acquire OK` line names which tool ran and how it authenticated (#1151).
