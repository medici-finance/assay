### Fixed
- `statusgen verifyflip` and the reconcile fold no longer skip a double-quoted hold inside a verifyrun witness row (the held scan's rule 7). The witness check credits such a row by the first state word in its Result cell, so a Result cell reading `"<hold>" pass exit=0` flipped where it had been refused before rule 7; it is refused again on every read.
