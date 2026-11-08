### Fixed
- The changelog aggregator reads its release-tag fragment list through `os.popen` again, as its module docstring says, and no longer imports `subprocess`; a ref with shell metacharacters, a missing `git` and an unresolvable ref still fail closed with the same refusal.
