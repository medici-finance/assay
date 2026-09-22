### Changed
- The model-capability floor is now **RISK-CONDITIONAL on an unstamped PR** for a review
  verdict. An unstamped PR (no dispatch stamp, a `dispatched-tier:any` stamp, or a stamp
  that has aged out) still proceeds with a NOTICE on a NON-risk PR, exactly as before — a
  human-driven or unattested lane is not bricked. But a review verdict is a
  security-review-bearing write, so on a **risk-classed** PR (every public-repo PR, or a diff
  touching a security path) an unstamped verdict now **REFUSES**: a security-review-bearing
  verdict must carry a trustable attestation of the tier that produced it, and a stamp anyone
  could self-apply — or the absence of one — is not attestation. This closes the hole where the
  floor was strict against an honest below-tier stamp yet permissive against no stamp at all.
  The review lane's own trustable-stamp path (`deskdispatch --kit review`) is what lets a
  correctly-run risk-classed review clear the floor; the risk determination reuses the same
  signal the ready-flip's security-review gate reads, not a second scheme. The strong, `any`,
  aged-out and override cases are otherwise untouched, and the loud incident-recovery override
  still bypasses the floor for an unstamped risk-classed verdict.
