### Fixed
- Roster-permission check now validates Windows file security via ACLs instead of POSIX mode bits. On Windows `os.FileMode` is synthetic, so the old group/world-writable mode test misfired; the check now reads the roster's owner SID and DACL and refuses a roster the invoking user does not own or that grants write to any principal beyond the owner, SYSTEM, or Administrators. Unix keeps its existing mode-bit + owning-uid guarantee. Applies to both `statusgen` and the desk-tools `deskkit` roster loaders.

### Added
- A platform-independent roster-ACL decision function (`evaluateRosterACL`) with unit tests that inject ACL data, so the Windows security logic is exercised on every CI platform even though there is no Windows CI runner.
