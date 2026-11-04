### Fixed
- The public-repo write gate's refusal no longer says the allowed-repos authorization "covers every write verb". It names the commands that run the gate (`deskpr`, `deskpost`, `deskreply`, `deskevidence`, `deskrelease`) and says tracker writes such as `deskfile` do not run it. A test fails if a command starts or stops calling the gate without the list following it.
- Code comments and the desk-tools README that described the gate as authorizing only a listed `:public` entry now name `:public` or `:internal`, matching the live visibility.
