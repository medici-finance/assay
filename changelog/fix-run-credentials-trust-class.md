### Fixed
- `ASSAY_RUN_CREDENTIALS` is now a roster **trust** key, not an extension key: it chooses which
  credential a desk write runs as, so a malformed entry now refuses the whole roster (every desk
  tool refuses, naming the key) instead of disabling only `deskrun`. Unset and valid values
  behave as before. `ResolveRunCredential` also refuses on its own when the roster is refused,
  whether or not the caller checked first, and a class guard pins the extension catalogue to a
  committed allow-list, so no trust key can land in it again.
