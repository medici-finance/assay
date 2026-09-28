### Added
- statusgen drives critical tier, main-red arm: a new `--main-health` input (`green`, or `red:<owner/repo#N>[,...]` naming the issues that track the red main) lifts a main-red fix into the tier. A main-red fix is an issue-loop placeholder for a tracking issue, or a brief whose `issues:` lists one. statusgen still reads no live CI. When no input is given and a drive is active, the board and `--next-up` (`mainHealth`) report could-not-check instead of a silent green.
- statusgen drives critical tier, stamped-security arm: the ratified authority set is read from the new roster key `ASSAY_CRITICAL_STAMP_AUTHORITIES` instead of a compiled-in placeholder. When the key is unset, the arm grants nothing, the effective-config echo shows it as unset, and `--lint` names every stamp it cannot honour. The desk tools recognise the key.

### Fixed
- statusgen drives critical tier, reviewer-finding arm: the remediation that an unresolved finding names in `control:` now reaches the tier. Before, the arm keyed only on `affects:`, and those briefs are excluded from Next-up by design (StaleRef).
