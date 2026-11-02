### Fixed
- The worker-operations vitals brief's Verify table now fails on a tree without the vitals delivery: its go test rows run one anchored test and require its `--- PASS:` line, the status rows build into a temporary directory, the schema-count row asks for more than the pre-delivery count, and the consumers row is pinned to the delivery's base.
