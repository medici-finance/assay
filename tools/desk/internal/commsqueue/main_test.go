package commsqueue

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskfilestub"
)

// The quarantine filer runs deskfile from PATH: no test here may reach the
// real one.
func TestMain(m *testing.M) { os.Exit(deskfilestub.Run(m)) }
