### Fixed

- `cellctl up` on a house cell without `DESKD=1` no longer opens an idle placeholder `cell` window. It opens only the role windows (plus comms when configured); on tmux the first role window creates the session. The "no deskd" note is printed once on `up`'s stderr.
