### Added
- `deskclose self-withdraw` — the authoring App closes its OWN open draft (`--because abandoned`, or `--because superseded --by <ref>` recorded not verified), pinned by login AND roster bot id; refuses a non-draft, another author's change, a login-only match, an unpinned roster id, and anything carrying `needs-decision`. Cites no ruling and consults no disposition record: an author's own withdrawal, nothing wider.
- `deskclose verify-gate-refire` — the verifier session reopens, comments on, and re-closes a CLOSED issue carrying `verify-gate` (`--reason` mandatory) so the card's close event fires again; refuses every other role by name, an unlabelled item, and a pull request. Explicitly not the human sign-off: a bot's close of a verify-gate issue is reopened by the repository's verify-gate close workflow regardless.
- `Forge.ReopenIssue(repo, number)` on both backends (inventory op 46) — `CloseIssue`'s inverse, one request on the issue endpoint, no state reason; golden-pinned on GitHub and GitLab.

### Changed
- `deskclose manifest` is documented as the sanctioned human-ruled BATCH lane: the human's own ruling comment is the manifest's `authorized-by`, and the digest binds it to exactly the rows they saw. No behaviour change.
