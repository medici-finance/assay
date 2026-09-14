### Fixed
- statusgen's human-stamp sole-permitted-writer check no longer false-positives on Windows: `relPath` now normalizes backslash separators before building git pathspecs, so a legitimately gate-written `human:<name>` sign-off stamp reads clean on `windows-smoke` the same way it already did on Linux/macOS (#1007).
