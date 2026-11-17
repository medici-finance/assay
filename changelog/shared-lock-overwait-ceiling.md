### Fixed

- deskkit: the shared lock helper's bounded wait is now pinned by a fake-clock test that proves a lock which never frees is waited on for exactly the granted time and then refused, so a dropped wait ceiling or a deadline read from the wrong clock reading fails a test instead of passing.
