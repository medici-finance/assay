### Fixed
- The shape stage of the disclosure sweep no longer spells the banned word-boundary escape in its own comments or test source, and `TestS2ShapeNoWordBoundary` now scans every byte of the stage's `leakshape*` files (comments, raw strings and fixtures included), not only the compiled rule patterns, so a reintroduced escape fails the test.
