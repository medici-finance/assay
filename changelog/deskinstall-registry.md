### Fixed
- `deskinstall` is registered in the canonical tool-key registry (`deskkit.canonicalToolKeys`), so its writes count against its own audit budget instead of refusing as an unregistered tool, and `TestRegistryCoversCmdBinaries` is green on main again.
