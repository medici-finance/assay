### Fixed
- `deskevidence flip` excludes already-published Evidence using the same library branch-tip snapshot as `deskpr update`, allowing Evidence and its verified flip in one PR while publication still checks the live destination and new commits retain both identity checks.
- A publish-identity refusal over a widened range (missing, badly spelled or divergent tracking ref) names the unused tip and its remedy (usually `git fetch origin` and retry), and no longer offers `git commit --amend --reset-author`, which would rewrite a commit the remote may already hold.
