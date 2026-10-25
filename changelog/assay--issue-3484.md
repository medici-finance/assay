### Fixed
- The brief-flow acceptance runner's mutation mode now copies every local `replace` target of the statusgen module into each scratch copy, derived from `go mod edit -json` rather than a fixed list, so mutants build and are observed red again; a new local module can no longer silently break the audit (#3484).
