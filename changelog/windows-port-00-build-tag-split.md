### Added
- `windows-port/00` — a new wave-0 brief for the `_unix.go`/`_windows.go` build-tag split of the eight unix-only `syscall` sites in `statusgen/` and `tools/desk/` (a process-group kill, two `Stat_t` roster-owner checks and five `flock` copies), each Windows variant required to degrade explicitly and visibly rather than silently.

### Changed
- The `windows-port` stream's "the Go binaries are already portable, no source change needed" premise is retired: it was measured on 2026-08-07 as a harness claim, not a `GOOS=windows` one, and neither module cross-compiles for Windows today. `windows-port/01` (release build matrix) keeps its two-file scope but now depends on `00`, and the stream's critical path becomes `00 → 01 → 03 → 05`. Planning docs only — no tool behavior changes yet.
