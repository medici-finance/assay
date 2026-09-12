### Fixed
- The shared `deskkit` secret scan no longer refuses a path that has a `+` immediately in
  front of it. `+` is in the base64 alphabet, so a regex quantifier written against a path
  (`grep -cE -e '^FRESH +plugins/assay/…/claude-code\.md'`, the Verify-row idiom) or
  a unified-diff add marker was read as the path's first character, and the path rule refuses
  any run containing `+` outright. Because `deskevidence` scans the whole merged brief before
  appending its Evidence row, a brief carrying such a Verify row could not receive an Evidence
  append through the sanctioned tool at all. The exemption is earned by the remainder being
  path-like — exactly one `+` is stripped, and a `+` on opaque token material still refuses.
