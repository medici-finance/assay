### Fixed
- Serialize shared roster-beacon updates across processes and publish complete files atomically, preserving receipts, resource vitals and concurrent work changes.
- Keep malformed beacon data intact on refusal and document recovery and the need to upgrade every writer sharing the state directory.
