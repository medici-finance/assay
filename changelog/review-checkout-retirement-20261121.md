### Fixed

- Finished review checkouts can retire while their pull request is open. The desk acknowledges reviewer exit after saving the evidence. `deskwt` removes eligible checkouts, retries held ones during pruning, and reports the backlog count, size and oldest age.
