### Fixed
- The changelog aggregator reads its release-tag fragment list through `os.popen` again, as its module docstring says, and no longer imports `subprocess`; a ref with shell metacharacters, a missing `git` and an unresolvable ref still fail closed, and a `git` that cannot be run is reported as "cannot run git" from either git call.
