### Fixed
- `writeguard` now checks `gh` output filters on every `gh` read in a desk session and refuses filters written out in the call that read beyond the JSON output. Ordinary selection and formatting filters keep working. `docs/write-policy.md` states the limits.
