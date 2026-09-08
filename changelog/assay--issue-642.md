### Fixed
- Preflight cold-mint now inherits the platform's home-defining variables
  (`USERPROFILE`, `HOMEDRIVE`, `HOMEPATH` alongside `HOME`) into the scrubbed
  child environment, so a Windows child mint can resolve `os.UserHomeDir()` and
  find `roster.env`. Previously the child reported the roster absent on an intact
  envelope (`%userprofile% is not defined`), because the scrub kept only `HOME`.
