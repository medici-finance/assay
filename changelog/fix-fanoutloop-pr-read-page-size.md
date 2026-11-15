### Fixed
- The open+merged changes read behind `fanoutloop plan` and `deskdispatch`'s phantom check now pages 25 changes at a time through the shared pager and retries a page cut off mid-transfer at half the size, so a repo with a large PR history no longer holds the fresh and rework lanes on every run. A read that still cannot complete stays could-not-check and holds the lanes (#6475).
