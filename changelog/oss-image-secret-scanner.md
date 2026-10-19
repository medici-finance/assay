### Changed
- Replace the image gate's custom credential regexes and compiler-packing exceptions with digest-pinned Gitleaks. Source scanning remains primary; packaging checks every layer's text and image metadata using normal OSS binary handling.
- Keep every detection rule enabled. A reviewed upstream baseline is bound to whole-file contents and image paths, so changed files cannot inherit clearance. Scanner errors and missing detection controls fail closed.
