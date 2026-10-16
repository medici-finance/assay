### Fixed
- The cellctl shell suites no longer depend on the host having tmux or herdr installed: each suite now prepends its stub directory through one `private_path` helper that stubs both cockpits, so `check` rows and the auto cockpit read the same on a clean runner as on a workstation. A regression test fails any suite that prepends to PATH another way.
