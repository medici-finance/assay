### Added
- A new stdlib-only Go module, `forgeread`, is the importable fact reader: one narrow `Reader` API for the open issues of an explicit repository set, with the three-state result (read, known empty, unavailable) made a type. A repository appears exactly once, as data or as unavailable; a provider that keeps paginating past the walk ceiling is reported as truncated rather than as a short list; an envelope with an unknown schema is refused. Offline and frozen-result readers live in the root package, which imports no HTTP client, no process launch and no subpackage. The Go API and the JSON envelope are versioned separately.
- `deskread issues` marks a truncated repository with an optional `class` field on its unavailable entry (additive; the envelope schema number is unchanged).

### Changed
- The open-issue paging and mapping for GitHub and GitLab now live once, in `forgeread/adapters`, and desk-tools' forge methods delegate to it over their own authenticated transport. Credential resolution and host and repository validation stay in desk-tools; the delegated reader refuses any repository other than the one a credential was admitted for before it builds a request. Behaviour is unchanged.
- statusgen reads the shared types: its default forge reader is the library's offline reader, and its online reads still run the `deskread` verb.
- `forge-ban.sh` counts `forgeread/` as its own tree (`forgeread sites: N`) and exits non-zero when it is above zero; the desk and statusgen columns stay advisory.
- The container build files copy `forgeread/` beside the modules that resolve it.
