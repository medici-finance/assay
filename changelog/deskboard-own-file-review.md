### Fixed
- Deskboard requests another review for non-merge commits that touch a PR’s own files after its reviewed head, including fixes next to keep-current merges and edits later reverted. Own-file conflict resolutions in merges also retain their re-review trigger. Incomplete history degrades to another review.
