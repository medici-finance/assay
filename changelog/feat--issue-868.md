### Fixed
- `deskpost comment` can now annotate a **verify-gate sign-off card**. Those cards
  are filed by the repo's own `verify-gate-open` workflow, so their author is
  `github-actions[bot]` — an identity the trust gate refuses — and the refusal meant
  no desk could mark a card an inert duplicate or warn that closing it will not flip
  the brief's row, leaving the human closing it with no signal. The carve-out is the
  narrowest read that fixes that: the `comment` verb, on an **issue**, authored by the
  forge's Actions identity, carrying the **`verify-gate`** label. `review`,
  `security-review` and `ready` stay refused on such issues, an Actions-authored issue
  *without* the label stays refused for `comment` too, and the label admits nothing on
  an issue anyone else authored. Every other comment-path protection — the body size
  cap and secret scan, the repo gate, the write budget, the audit line — is unchanged.
