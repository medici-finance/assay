# Planted positive control for TestShippedCallersNoRotate

Each line below is a read-only caller written in the rotating form. The guard must flag
every one; if it stops flagging any of them its matcher is broken.

- Hand the helper `--token-file "$(desktoken worker)"`.
- Resolve it with `desktoken <role>
  --repo <slug>` before the read.
- elif ! tokfile="$(desktoken "$role" --repo "$repo")"; then

These forms are allowed and must not be flagged:

- `desktoken --no-rotate worker --repo <owner/name>`
- `desktoken <role> --forge github --repo <slug>`
- `desktoken <role> --forge gitlab` (an explicit rotation)
- `desktoken <role> --fresh`
- `desktoken coverage <role>`
