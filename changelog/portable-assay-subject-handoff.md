### Fixed

- Cross-repository verifier dispatch now resolves the tracking repository and its forge from the same checkout. A conflicting repository identity holds the run before it takes a claim. If the planned and admitted destinations differ, the refusal names both valid repository slugs.
- The verifier's generated publication command preserves paths containing spaces or apostrophes.
- When a witness names a read-set file missing from the landing checkout, the Evidence writer now tells the operator where the verifier recorded the bytes and how to include them with the rows in one Evidence PR commit.
