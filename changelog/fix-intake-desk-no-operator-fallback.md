### Fixed
- The `intake-desk` skill no longer tells a desk to work as a shared operator account when its role App is not installed on a target repository. That case is now a stop: the desk files a `help wanted` naming the App and the repository it must be installed on, which matches the rule every desk-role skill already states that each forge call runs as the role's own App.
