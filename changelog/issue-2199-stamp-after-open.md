### Fixed
- Add `deskdispatch --stamp-only` so the original dispatcher can attest a worker's model and tier after its PR opens without allocating another dispatch. Shared stamp writes now verify the standing label pair and appliers before reporting success.
