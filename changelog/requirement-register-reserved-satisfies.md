### Added
- **REQUIREMENTS register** — a third append-only register recording what the product was
  asked to do: the ask, who asked, an ordered `impact` axis, acceptance criteria a Verify
  row could be written against, and a `proposed → accepted → satisfied → withdrawn`
  lifecycle. Entries are per-entry files under `docs/streams/requirements/<slug>.md` with
  `REQ-<slug>` ids. Specified in `spec/registers-v1.md` §6; `statusgen --lint` parses and
  shape-validates every entry, and flags a missing or out-of-vocabulary `impact` by value.
- **`satisfies:` on a brief** — an optional `brief-v1` frontmatter key citing the
  requirements a brief was written against (`REQ-<slug>`, or `<alias>:REQ-<slug>` through
  the existing repo-alias registry). It rides the existing `brief-v1` schema, so no pinned
  consumer has to be upgraded to keep linting a tree that uses it.

### Changed
- `statusgen --lint` now says out loud that requirement traceability is **reserved, not
  gating**: it emits a NOTICE naming what is parsed and what is deliberately not checked.
  An absent `satisfies:` is never flagged, a citation naming a requirement that does not
  exist is not an error, and no exit code changes on either — the enforcing checks are a
  separate change.
