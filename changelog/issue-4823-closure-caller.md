### Fixed

- Verifier outcome instructions now explicitly permit fresh approval corroboration for the full landed closure tree. Permission and credential custody remain local to each invocation, including concurrent calls. Approval and recorded human-ruling audits use the landing's minted GitHub verifier credential, with ambient credentials removed and foreign-forge credentials withheld; without explicit opt-in, audits cannot fall back to a stored login and a decision gate needing a recorded ruling remains held. Unknown repository authority and refuted approvals remain held or refused.
