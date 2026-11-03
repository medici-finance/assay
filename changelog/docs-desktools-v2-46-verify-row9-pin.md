### Fixed
- Verify row 9 of the release and provenance CLI migration brief (desktools-v2/46) now pins the delivering merge range and asserts the exact `statusgen --consumers` summary, so it no longer reports "nothing to corroborate" on merged main. Its Expect records that all 20 declared consumers are routed to the brief itself and pass by self-reference, not by the delivery diff.
