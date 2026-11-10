### Added

- desk-tools briefs 61 to 64 scope the class fix for the shared desk secret scan's false positives on benign shapes. Brief 61 moves the scan's per-shape exceptions into one registry, so the docs, tests and mutation checks read the same list. Brief 62 reports a branch-diff refusal as a file, a line and a side. Brief 63 adds the digest and armor shapes behind a word-salad entropy cap. Brief 64 puts the choice of which diff lines are scanned to a human as options. Each brief that changes what the scan refuses carries a threat model.
