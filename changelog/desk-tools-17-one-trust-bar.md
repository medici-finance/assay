### Changed
- **`deskboard` now applies the same author-trust bar on every repo, private or public**
  (desk-tools/17, #808). The board's PR classifier previously swapped in a STRICTER bar
  (`role App or mapped human only`) on any public/internal/unknown repo, diverging from
  `deskpost`'s gate — a trusted shared automation login authoring a PR on a public repo was
  invisible to the review loop even though `deskpost` would happily post a verdict on it.
  Both tools now answer "may this PR enter the review loop?" with the same predicate
  (`TrustedAuthor`); an unlisted author is still quarantined unless blessed, and merge
  authority is unchanged — a human still merges every PR.
