### Fixed
- `cellctl cycle` now shows its plan and takes the typed confirm code before it pauses, drains, stops or restarts any role, on every path (`--all-at-once`, `--force-after-timeout`, every role down). A wrong or absent code changes nothing. The fixture walk in `TestEveryStateChangingInvocationRequiresTTY` reaches `cycle`'s apply path again.
