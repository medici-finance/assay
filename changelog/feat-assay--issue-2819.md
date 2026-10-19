### Fixed
- Reject multiple physical lines in a tick summary candidate, preventing valid records from hiding forbidden outcomes or count combinations.
- Refuse a line break in a tick summary candidate with the same fixed reason the Go port gives, without echoing the candidate, so a rejected candidate can no longer put a grammar-matching record on stderr.
