### Fixed
- `statusgen --consumers`'s `fixed-here` gate no longer wrongly DISPROVES a site token written
  with Markdown emphasis — a path wrapped in backticks or `**bold**` — around it: the wrapping
  delimiters are now stripped before the token is resolved as a path, so a backticked entry
  corroborates exactly like its bare equivalent. (#1077)
- A `fixed-here` claim that is genuinely DISPROVED now names WHICH check actually failed —
  `tree-lookup` (does the path resolve under the root?) vs `diff-lookup`/`diff-deletion-lookup`
  (does the diff touch it, or name it as a delete/rename?) — instead of one conflated sentence
  that left a reader unable to tell which predicate returned false. (#1077)
