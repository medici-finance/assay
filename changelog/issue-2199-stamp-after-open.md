### Fixed
- Add `deskdispatch --stamp-only` so the coordinator can attest the worker model and tier selected by the original dispatcher after its PR opens without allocating another dispatch. Shared stamp writes now verify the standing label pair and appliers before reporting success.

- Add dispatcher-owned pre-work verifier attestation with exact run/source/brief/model binding, fail-closed execution and Evidence admission, and recovery of the same record.
- Document and test the worker-desk to coordinator post-open handoff without granting child workers stamp authority.
