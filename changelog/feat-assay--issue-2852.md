### Fixed

- The derived-board reconcile-derivation brief's mutation-harness Verify row no longer pins an exact mutant count. It now requires `0 NOT CAUGHT, 0 could-not-mutate` and at least 41 caught, so growing the fold mutation corpus no longer fails the row while every mutant is still caught.
