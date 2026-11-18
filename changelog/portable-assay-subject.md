### Fixed

- For a brief tracked in one repository and delivered in another, `deskdispatch` now emits the verifier’s outcome command for the tracking repository and its forge. The verifier still runs in the admitted delivery worktree. A mismatch between the tracking checkout and its registry entry holds dispatch before it acquires a claim.
