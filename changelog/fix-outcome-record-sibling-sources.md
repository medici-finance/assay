### Fixed
- `deskevidence --outcome-record` now writes a record for a brief whose declared files span
more than one sibling repository (#4876). The verifier attestation binds one source commit per
declared sibling (`SiblingSources`, taken from that sibling's configured root at its fetched
`origin/main`), and the record carries one `repo-source:` key per sibling. Coverage is still
checked per path against that path's own sibling, so a path in a sibling with no bound source is
still refused, and a bound sibling's delivery PRs are compared with that sibling's own commit.
`wake_schema` does not change; older wake readers read the extra keys as could-not-check. Older
attestation readers refuse a body carrying the new field, so upgrade readers before dispatchers.
