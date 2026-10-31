### Fixed
- `writeguard` now checks `gh` output filters on every `gh` read in a desk session and refuses filters it finds written out in the call that read beyond the JSON output. Ordinary selection and formatting filters keep working. It is not a complete control: it reads no shell grammar, so it can falsely refuse some text and silently miss some. `docs/write-policy.md` states the limits.
