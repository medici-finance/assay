### Fixed

- Cross-repository verifier dispatch now resolves the tracking repository and its forge from the same checkout. A conflicting repository identity holds the run before it takes a claim. If the planned and admitted destinations differ, the refusal names both valid repository slugs.
- The verifier's generated publication command preserves paths containing spaces or apostrophes.
