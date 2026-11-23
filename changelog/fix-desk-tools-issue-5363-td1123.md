### Fixed

- `deskevidence --outcome-record` can now bind a cross-repository verification's outcome to the Evidence commit it names. The Evidence for such a run lands through the forge, so the verifier's tracking clone never held that commit, and every cross-repository blocked or verify-fail outcome was refused with "cannot read the outcome record's Evidence commit". The landing now fetches the tracking repository's main and the landing branch into private refs before reading the commit, and stays could-not-check when neither holds it. Same-repository outcomes are unchanged.
