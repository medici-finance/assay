### Fixed
- `deskevidence --outcome-record` writes one `file:<dir>/` content digest for a declared directory instead of a `file:` key per member, so a record for a directory holding files such as `secret-mount.fixture` no longer trips the secret pattern sweep; the scanner configuration is unchanged, and any member edit, addition, removal or rename still wakes the held work (#4865).
