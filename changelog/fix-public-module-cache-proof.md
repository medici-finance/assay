### Fixed

- The host verification bracket can admit modules from the public Assay repository
  after an unauthenticated check of its fixed repository identity and public
  visibility. Exact committed module versions and both checksums are recorded in
  the receipt and rechecked at container admission and close. Operator private
  patterns, other owner denials and offline execution remain enforced; missing or
  unavailable proof refuses before download.
