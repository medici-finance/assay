### Fixed

- Desk tests whose Unix socket directory followed `TMPDIR` no longer fail under a long managed `TMPDIR` (the darwin 104-byte socket path limit): they now take that directory from one shared helper that falls back to `/tmp`, or skips as could-not-check when no short directory is available.
