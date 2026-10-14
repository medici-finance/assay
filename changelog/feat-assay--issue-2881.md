### Fixed

- `cellctl up` on a cell with no deskd (a house cell without `DESKD=1`) no longer opens an idle placeholder `cell` window. It opens only the role windows (plus comms when configured); on tmux the comms window or else the first role window creates the session. The "no deskd" note is printed once on `up`'s stderr.
