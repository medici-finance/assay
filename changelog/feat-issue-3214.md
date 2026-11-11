### Fixed

- The `cellctl` two-session comms hello round-trip test now puts its cell directory, and so the gateway's Unix socket, in a fresh owner-only directory whose socket path fits the platform's socket-address limit. It prefers `TMPDIR`, falls back to a fresh private directory under `/tmp` when a long `TMPDIR` (such as a managed task scratch) would overflow macOS's 104-byte limit, and skips with an explicit could-not-check when neither fits, instead of failing with `bind: invalid argument`. Part of #3214.
