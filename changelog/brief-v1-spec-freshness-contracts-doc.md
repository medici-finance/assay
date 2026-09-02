### Changed
- `spec/brief-v1.md` is brought current with the reference validator: the "Describes reference implementation" header now names the released `statusgen` version instead of a long-stale one, and the five optional frontmatter fields the validator has grown but the spec never documented — `domain`, `blocked-by`, `homed-in`, `measures`, and `parallel-streams` — are now specified with their exact value sets and flagging rules.

### Added
- `docs/contracts.md` documents the three-part schema-first contract pattern (versioned machine-readable artifact + source-side coverage gate + consumer-side conformance run), written over the brief-v1 frontmatter contract and parameterized for the consumer install seam as a second instance.
