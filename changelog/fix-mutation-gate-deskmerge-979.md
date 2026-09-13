### Fixed
- `cmd/deskmerge`'s mutation-gate spec (`cmd/deskmerge/mutations.json`) is repointed at the
  current `gitcore`-based parent-check code, so `muhar` can plant its positive control and
  mutations again instead of exiting 2 before any verdict prints. (#979)
- Pairs with #983, which closes the remaining fast-forward mutation-coverage gap; the two ship
  together.
