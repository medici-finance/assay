package contract_test

import (
	"testing"

	"github.com/medici-finance/assay/deskd/internal/purity"
)

// TestContractIsPlatformFree extends the domain-purity lint (design §3.4/§11,
// spec §1.5) to the wire contract, as the service contract Task 5 requires: the
// types that cross the client↔service seam must be as platform-clean as the
// domain they carry, so `issue.number` never becomes a wire field. It runs the
// same purity.CheckDir the domain guards itself with, so `go test ./...` fails
// on a leak in internal/contract without anyone remembering to run the binary.
//
// The lint's load-bearing-ness is proven elsewhere: internal/purity's positive
// control feeds it the exact field spec §1.5 names, and the service contract Verify
// 10 is the captured red — `go run ./cmd/puritylint ./internal/contract` goes
// exit 1 the moment a contract type grows an `IssueNumber` field.
func TestContractIsPlatformFree(t *testing.T) {
	findings, err := purity.CheckDir(".")
	if err != nil {
		t.Fatalf("could-not-check: the purity lint did not run: %v", err)
	}
	for _, f := range findings {
		t.Errorf("%s", f)
	}
	if len(findings) > 0 {
		t.Fatalf("%d platform-shaped identifier(s) in internal/contract — the wire carries an "+
			"adapter-minted EscalationID and platform deep-links, never issue.number (spec §1.5)", len(findings))
	}
}
