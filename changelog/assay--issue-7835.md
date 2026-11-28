### Fixed

- Desk tests that bind a Unix socket no longer fail under a long managed `TMPDIR` (the darwin 104-byte socket path limit): they take their socket directory from one shared helper that falls back to `/tmp`, or skips as could-not-check when no short directory is available.
