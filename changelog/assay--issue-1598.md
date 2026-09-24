### Changed
- `statusgen --close-verify` now runs the same PASS/HELD contradiction check on the `verified → done` close path that it already ran on `implemented → done`, and also refuses a `verified` brief whose most recent Evidence verdict is `VERIFY: FAIL`. A brief flipped to `verified` over an un-deferred HELD/could-not-check row can no longer be closed to `done` with that hold still in place.
- The verify-desk skill states that a PASS whose Evidence still carries an un-deferred HELD/could-not-check row is not a flip signal: run or formally defer the row first.
