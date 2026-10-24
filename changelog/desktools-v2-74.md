### Added
- added `statusgen verifyrun --rows <id>[,<id>...]`, which runs and records only
the named Verify rows under the standard witness header and reports every other
row `not selected`, so a verifier can witness a mixed table with a container
bracket over the container-witnessable rows and a host run of the host-only
rows. Unknown, empty, repeated or shared IDs, a table with no `#` column, and
`--rows` with `--check` or a bracket step are refused before any row runs.
`--check`, last-wins and the container-row credit rule are unchanged.
