package domain_test

import (
	"testing"

	"github.com/medici-finance/assay/deskd/internal/purity"
)

// TestDomainIsPlatformFree runs the domain-purity lint over this package, so
// `go test ./...` fails on a leak without anyone remembering to run a
// separate binary (design §11, spec §1.5).
//
// The sibling positive control lives in internal/purity: it proves this lint
// CAN fail, by feeding it a package with the exact field spec §1.5 names.
// A guard with no captured red run is a guard nobody has seen work.
func TestDomainIsPlatformFree(t *testing.T) {
	findings, err := purity.CheckDir(".")
	if err != nil {
		t.Fatalf("could-not-check: the purity lint did not run: %v", err)
	}
	for _, f := range findings {
		t.Errorf("%s", f)
	}
	if len(findings) > 0 {
		t.Fatalf("%d platform-shaped identifier(s) in internal/domain — the domain sees only "+
			"an adapter-minted EscalationID; the platform deep-link is attached at the serving "+
			"edge as links.platform", len(findings))
	}
}
