### Fixed
- `aggregate.py credits` now credits a fragment to the pull request that
  actually landed it: the adding commit's own `(#N)` / `Merge pull request #N`
  subject is read FIRST, and a merge is only accepted when its second parent
  contains the adding commit. Previously the resolver took the oldest merge on
  the ancestry path to `HEAD`, so a squash-landed fragment — and a fragment that
  never landed on a pull request at all — was credited to whatever unrelated
  pull request merged above it, collapsing unrelated fragments onto the same few
  recent numbers.
