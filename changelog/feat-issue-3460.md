### Fixed
- `deskpathguard` recognizes descriptive Verify headings, preserves complete escaped-pipe cells during re-derivation, and reports unreadable or malformed tables as could-not-check.
- Re-derivation skips auxiliary detection maps while strictly validating every Command/Expect table, and resolves base-path existence independently of the Git working directory.
- Ambiguous auxiliary tables containing later Command/Expect cells and subdirectory checkout roots now refuse before any re-derivation result, preserving the original-row comparison.
