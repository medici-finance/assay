### Fixed
- Verify row 10 of the worker usage-counts brief (desk-supervision/34) now pins the delivering merge range and asserts the exact `statusgen --consumers` summary, so it no longer reports "nothing to corroborate" on merged main. Its Expect records 12 corroborated `fixed-here` routings and 5 UNCHECKED `out-of-scope` routings, which stay review obligations, not passes.
