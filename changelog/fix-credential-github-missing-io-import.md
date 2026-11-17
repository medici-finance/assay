### Fixed
- `tools/desk` builds again: the credential provider's GitHub arm had lost its `io` import in a merge, so `go build ./...` failed on `main`.
