package web

import (
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"os"
	"testing"
)

// Existing synthetic fixtures run under an explicit process-start policy.
func TestMain(m *testing.M) {
	if os.Getenv("DESKD_TEST_POLICY_DISABLED") != "1" {
		if err := humanid.Configure("operator"); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
