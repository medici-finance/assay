### Fixed
- `deskpr edit --force-scan-override` now applies the override it records to the edit's own forge write, so an audited exception no longer ends in a forge-level refusal with no edit applied. Every `deskpr` verb registers the flag through the shared outbound-override helper (issue #5914).
