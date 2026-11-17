### Fixed
- The `deskdispatch` tests that plant a fake claim child now run on a PATH built from scratch (the fixture directory plus the system tool directories), so an installed desk-tools on the host no longer answers for `deskdecision` or `statusgen` and the regression floor stays green on hosts with desk-tools installed.
