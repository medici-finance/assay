### Added
- `writeguard` refuses a Bash call that runs `gh auth token` (or asks `gh auth status` to print the token) while `DESK_LOOP` or `DESK_SESSION` is set in the hook's environment, naming `desktoken <role>` as the only sanctioned credential path. Outside a desk session the hook stays silent, other `gh` use is untouched, and malformed hook input still fails open.
