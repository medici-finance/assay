### Changed
- A test now pins that a clean Result cell naming the default sandbox mode (`sandbox=unshare`) as a detail reads as clean, so a later clean run under that mode still covers an earlier held row. The held-scope mutation entry that drops `unshare` from the neutral-detail set was not caught before and is caught now (#6790).
