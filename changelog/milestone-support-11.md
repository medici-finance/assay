### Added
- `stream-view/v2` adds a mandatory milestone section that keeps supported scope, work remaining, acceptance, availability and holds distinct and lists every unknown; `stream-view/v1` documents and non-opted-in streams are unchanged.
- Opted-in stream READMEs can carry a separately marked, generated `statusgen:milestones` region (scope, prerequisites, evidence, reverse membership, coverage); `--regen` rewrites it and `--lint` reports a hand edit or markers on a stream that did not opt in.
- `deskboard milestones <stream-view.json>...` renders the projection in process, shows zero work with acceptance outstanding truthfully, collapses aliases, quotes untrusted text, withholds inaccessible locators and refuses a contract version it cannot decode.
