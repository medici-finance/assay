### Fixed

- Include Bash in the combined desk-tools image and trust its explicit `/work`
  checkout mount for Git attribution while retaining the nonroot runtime user.
- Refuse new verified outcome receipts until the target branch contains the
  verified/done row, dated verifier stamp, and passing execution witnesses, and
  the same checkout passes lint. Evidence-only landings retain implemented status
  without recording a completed verification; failure receipts remain available.
- Require explicit shell selection when authoring native Windows Verify commands;
  prefer POSIX commands and forward-slash paths for portable checks.
