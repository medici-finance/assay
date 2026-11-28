### Fixed

- The statusgen autonomy gate-share tests now pass when they run as an admitted Verify row. Their fake `gh` fixture removes the row's git config pins for the test's duration, so `ghGuard` no longer refuses the fake. `ghGuard` itself is unchanged.
