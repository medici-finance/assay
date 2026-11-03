package contract

// The OpenAPI document is generated FROM the types in this package, never hand
// written, so it cannot drift from the shipped wire shapes. `go generate` here
// runs cmd/openapigen (which imports this package); the committed openapi.yaml
// must match its output, asserted by the service contract Verify 6:
//
//	go generate ./internal/contract/ && git diff --exit-code -- internal/contract/openapi.yaml
//
// cmd/openapigen is a SEPARATE main package, so it is not in this package's
// import graph — `go list -deps ./internal/contract` (TestLeafImports) never
// sees it, and the contract stays a leaf both binaries import.
//
//go:generate go run github.com/medici-finance/assay/deskd/cmd/openapigen -out openapi.yaml
