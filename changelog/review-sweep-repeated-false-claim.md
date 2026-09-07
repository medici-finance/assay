### Added
- Review kit (`review-prompt.md`): a "claim is false" finding now requires the reviewer to
  sweep the whole diff (and, where cheap, the repository) for every other instance of the
  same claim before signing off the fix, instead of checking only the cited file:line.
