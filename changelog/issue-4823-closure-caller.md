### Fixed

- Verifier outcome instructions now explicitly permit fresh approval corroboration for the full landed closure tree. Approval audits use the landing's minted GitHub verifier credential, with ambient credentials removed and foreign-forge credentials withheld; other audits cannot fall back to a stored login. Unknown repository authority and refuted approvals remain held or refused.
