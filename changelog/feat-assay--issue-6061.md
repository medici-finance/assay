### Fixed

- `deskclose triage --disposition human-decided --decision` now accepts the ruling comment link of the forge serving the issue: a GitHub `#issuecomment-<id>` permalink or a GitLab `/-/issues/<N>#note_<id>` link. The link's forge, host, project and issue number must match the issue being closed. The comment is still fetched, and its author must be the blessing authority (login and numeric user id). A thread link, an unknown anchor, a link with a port, and a failed fetch all refuse.
- The shared comment fetch behind `deskclose`'s ruling, manifest and human-decided gates now refuses a comment link whose forge or host is not the one serving the named project. Before, a github.com link naming a project served by a GitLab backend was fetched from GitLab by its id.
