### Fixed

- Verifier outcome instructions now explicitly permit fresh approval corroboration for the full landed closure tree. Approval audits use the landing's minted verifier credential, with ambient credentials removed; other audits cannot fall back to a stored login. Unknown repository authority and refuted approvals remain held or refused.
