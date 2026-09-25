### Added
- `tools/desk/internal/weight`: a Go test package that counts the desk tools' verbs,
  flags, refusal-constructor call sites and shared skill/reference rule-text lines, and
  ratchets them against a committed `ceiling.txt` (currently `# mode: advisory`, so growth
  is logged as `GROWTH-NOTICE` rather than failing CI). `go test ./internal/weight/` runs
  as part of the existing `go test ./...` CI step — no workflow change needed.
